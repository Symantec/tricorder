package tricorder

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"github.com/Symantec/tricorder/go/tricorder/messages"
	"github.com/Symantec/tricorder/go/tricorder/types"
	"github.com/Symantec/tricorder/go/tricorder/units"
	"html/template"
	"io"
	"log"
	"net/http"
	"net/rpc"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	htmlUrl         = "/metrics"
	jsonUrl         = "/metricsapi"
	htmlTemplateStr = `
	{{define "METRIC"}}
	  {{with $top := .}}
            {{if $top.IsDistribution .Metric.Value.Type}}
	      {{.Metric.AbsPath}} <span class="parens">(distribution: {{.Metric.Description}}{{if $top.HasUnit .Metric.Unit}}; unit: {{.Metric.Unit}}{{end}})</span><br>
	      {{with .Metric.Value.AsDistribution.Snapshot}}
	        <table>
	        {{range .Breakdown}}
	          {{if .Count}}
	            <tr>
  	            {{if .First}}
	              <td align="right">&lt;{{.End}}:</td><td align="right">{{.Count}}</td>
	            {{else if .Last}}
	              <td align="right">&gt;={{.Start}}:</td><td align="right"> {{.Count}}</td>
	            {{else}}
	              <td align="right">{{.Start}}-{{.End}}:</td> <td align="right">{{.Count}}</td>
	            {{end}}
		    </tr>
		  {{end}}
		{{end}}
		</table>
	        {{if .Count}}
		  <span class="summary"> min: {{.Min}} max: {{.Max}} avg: {{$top.ToFloat32 .Average}} &#126;median: {{$top.ToFloat32 .Median}} count: {{.Count}}</span><br><br>
	        {{end}}
	      {{end}}
	    {{else}}
	      {{.Metric.AbsPath}} {{.Metric.Value.AsHtmlString}} <span class="parens">({{.Metric.Value.Type}}: {{.Metric.Description}}{{if $top.HasUnit .Metric.Unit}}; unit: {{.Metric.Unit}}{{end}})</span><br>
	    {{end}}
	  {{end}}
	{{end}}
	<html>
	<head>
	  <link rel="stylesheet" type="text/css" href="/metricsstatic/theme.css">
	</head>
	<body>
	{{with $top := .}}
	  {{if .Directory}}
	    {{range .Directory.List}}
	      {{if .Directory}}
	        <a href="{{$top.Link .Directory}}">{{.Directory.AbsPath}}</a><br>
              {{else}}
	        {{template "METRIC" $top.AsMetricView .Metric}}
	      {{end}}
	    {{end}}
	  {{else}}
	    {{template "METRIC" .}}
	  {{end}}
	{{end}}
	</body>
	</html>
	  `

	themeCss = `
	.summary {color:#999999; font-style: italic;}
	.parens {color:#999999;}
	  `
)

var (
	htmlTemplate = template.Must(template.New("browser").Parse(htmlTemplateStr))
	errLog       *log.Logger
	appStartTime time.Time
)

type htmlView struct {
	Directory *directory
	Metric    *metric
}

func (v *htmlView) AsMetricView(m *metric) *htmlView {
	return &htmlView{Metric: m}
}

func (v *htmlView) Link(d *directory) string {
	return htmlUrl + d.AbsPath()
}

func (v *htmlView) IsDistribution(t types.Type) bool {
	return t == types.Dist
}

func (v *htmlView) HasUnit(u units.Unit) bool {
	return u != units.None
}

func (v *htmlView) ToFloat32(f float64) float32 {
	return float32(f)
}

func htmlEmitMetric(m *metric, w io.Writer) error {
	v := &htmlView{Metric: m}
	if err := htmlTemplate.Execute(w, v); err != nil {
		return err
	}
	return nil
}

func htmlEmitDirectory(d *directory, w io.Writer) error {
	v := &htmlView{Directory: d}
	if err := htmlTemplate.Execute(w, v); err != nil {
		return err
	}
	return nil
}

func htmlEmitDirectoryOrMetric(
	path string, w http.ResponseWriter) error {
	d, m := root.GetDirectoryOrMetric(path)
	if d == nil && m == nil {
		fmt.Fprintf(w, "Path does not exist.")
		return nil
	}
	if m == nil {
		return htmlEmitDirectory(d, w)
	}
	return htmlEmitMetric(m, w)
}

func rpcAsMetric(m *metric) *messages.Metric {
	return &messages.Metric{
		Path:        m.AbsPath(),
		Description: m.Description,
		Unit:        m.Unit,
		Value:       m.Value.AsRPCValue()}
}

type rpcMetricsCollector messages.Metrics

func (c *rpcMetricsCollector) Collect(m *metric) (err error) {
	*c = append(*c, rpcAsMetric(m))
	return nil
}

type textCollector struct {
	W io.Writer
}

func (c textCollector) Collect(m *metric) (err error) {
	if _, err = fmt.Fprintf(c.W, "%s ", m.AbsPath()); err != nil {
		return
	}
	return textEmitMetric(m, c.W)
}

func textEmitDistribution(s *snapshot, w io.Writer) error {
	_, err := fmt.Fprintf(
		w,
		"{min:%s;max:%s;avg:%s;median:%s;count:%d",
		strconv.FormatFloat(s.Min, 'f', -1, 32),
		strconv.FormatFloat(s.Max, 'f', -1, 32),
		strconv.FormatFloat(s.Average, 'f', -1, 32),
		strconv.FormatFloat(s.Median, 'f', -1, 32),
		s.Count)
	if err != nil {
		return err
	}
	for _, piece := range s.Breakdown {
		if piece.Count == 0 {
			continue
		}
		if piece.First {
			_, err := fmt.Fprintf(
				w,
				";[-inf,%s):%d",
				strconv.FormatFloat(piece.End, 'f', -1, 32),
				piece.Count)
			if err != nil {
				return err
			}
		} else if piece.Last {
			_, err := fmt.Fprintf(
				w,
				";[%s,inf):%d",
				strconv.FormatFloat(piece.Start, 'f', -1, 32),
				piece.Count)
			if err != nil {
				return err
			}
		} else {
			_, err := fmt.Fprintf(
				w,
				";[%s,%s):%d",
				strconv.FormatFloat(piece.Start, 'f', -1, 32),
				strconv.FormatFloat(piece.End, 'f', -1, 32),
				piece.Count)
			if err != nil {
				return err
			}
		}
	}
	_, err = fmt.Fprintf(w, "}\n")
	return err
}

func textEmitMetric(m *metric, w io.Writer) error {
	if m.Value.Type() == types.Dist {
		return textEmitDistribution(m.Value.AsDistribution().Snapshot(), w)
	}
	_, err := fmt.Fprintf(w, "%s\n", m.Value.AsTextString())
	return err
}

func textEmitDirectoryOrMetric(
	path string, w http.ResponseWriter) error {
	d, m := root.GetDirectoryOrMetric(path)
	if d == nil && m == nil {
		fmt.Fprintf(w, "*Path does not exist.*")
		return nil
	}
	if m == nil {
		return d.GetAllMetrics(textCollector{W: w})
	}
	return textEmitMetric(m, w)
}

func handleError(w http.ResponseWriter, err error) {
	fmt.Fprintln(w, "Error in template.")
	errLog.Printf("Error in template: %v\n", err)
}

func htmlAndTextHandlerFunc(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	path := r.URL.Path
	var err error
	if r.Form.Get("format") == "text" {
		err = textEmitDirectoryOrMetric(path, w)
	} else {
		err = htmlEmitDirectoryOrMetric(path, w)
	}
	if err != nil {
		handleError(w, err)
	}
}

func jsonSetUpHeaders(h http.Header) {
	h.Set("Content-Type", "text/plain")
	h.Set("X-Tricorder-Media-Type", "tricorder.v1")
}

func jsonHandlerFunc(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	jsonSetUpHeaders(w.Header())
	path := r.URL.Path
	var collector rpcMetricsCollector
	root.GetAllMetricsByPath(path, &collector)
	var buffer bytes.Buffer
	content, err := json.Marshal(collector)
	if err != nil {
		handleError(w, err)
		return
	}
	json.Indent(&buffer, content, "", "\t")
	buffer.WriteTo(w)
}

type rpcType int

func (t *rpcType) ListMetrics(path string, response *messages.Metrics) error {
	return root.GetAllMetricsByPath(path, (*rpcMetricsCollector)(response))
}

func newStatic() http.Handler {
	result := http.NewServeMux()
	addStatic(result, "/theme.css", themeCss)
	return result
}

func addStatic(mux *http.ServeMux, path, content string) {
	addStaticBinary(mux, path, []byte(content))
}

func addStaticBinary(mux *http.ServeMux, path string, content []byte) {
	mux.Handle(
		path,
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.ServeContent(
				w,
				r,
				path,
				appStartTime,
				bytes.NewReader(content))
		}))
}

func getProgramArgs() string {
	return strings.Join(os.Args[1:], " ")
}

func initDefaultMetrics() {
	RegisterMetric("/name", &os.Args[0], units.None, "Program name")
	RegisterMetric("/args", getProgramArgs, units.None, "Program args")
	RegisterMetric("/start-time", &appStartTime, units.None, "Program start time")
}

func initHttpFramework() {
	appStartTime = time.Now()
	errLog = log.New(os.Stderr, "", log.LstdFlags|log.Lmicroseconds)
}

type gzipResponseWriter struct {
	http.ResponseWriter
	W io.Writer
}

func (w *gzipResponseWriter) Write(b []byte) (int, error) {
	return w.W.Write(b)
}

type gzipHandler struct {
	H http.Handler
}

func (h gzipHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
		h.H.ServeHTTP(w, r)
		return
	}
	w.Header().Set("Content-Encoding", "gzip")
	gz := gzip.NewWriter(w)
	defer gz.Close()
	gzr := &gzipResponseWriter{ResponseWriter: w, W: gz}
	h.H.ServeHTTP(gzr, r)
}

func initHttpHandlers() {
	http.Handle(htmlUrl+"/", http.StripPrefix(htmlUrl, http.HandlerFunc(htmlAndTextHandlerFunc)))
	http.Handle(jsonUrl+"/", http.StripPrefix(jsonUrl, gzipHandler{http.HandlerFunc(jsonHandlerFunc)}))
	http.Handle("/metricsstatic/", http.StripPrefix("/metricsstatic", newStatic()))
}

func initRpcHandlers() {
	rpc.RegisterName("MetricsServer", new(rpcType))
	rpc.HandleHTTP()
}

func init() {
	initDefaultMetrics()
	initHttpFramework()
	initHttpHandlers()
	initRpcHandlers()
}
