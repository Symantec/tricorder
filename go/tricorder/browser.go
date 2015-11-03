package tricorder

import (
	"bytes"
	"fmt"
	"github.com/Symantec/tricorder/go/tricorder/types"
	"github.com/Symantec/tricorder/go/tricorder/units"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	browseMetricsUrl = "/metrics"
	restUrl          = "/metricsapi"
	htmlTemplateStr  = `
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

type view struct {
	Directory *directory
	Metric    *metric
}

func (v *view) AsMetricView(m *metric) *view {
	return &view{Metric: m}
}

func (v *view) Link(d *directory) string {
	return browseMetricsUrl + d.AbsPath()
}

func (v *view) IsDistribution(t types.Type) bool {
	return t == types.Dist
}

func (v *view) HasUnit(u units.Unit) bool {
	return u != units.None
}

func (v *view) ToFloat32(f float64) float32 {
	return float32(f)
}

func emitMetricAsHtml(m *metric, w io.Writer) error {
	v := &view{Metric: m}
	if err := htmlTemplate.Execute(w, v); err != nil {
		return err
	}
	return nil
}

func emitDirectoryAsHtml(d *directory, w io.Writer) error {
	v := &view{Directory: d}
	if err := htmlTemplate.Execute(w, v); err != nil {
		return err
	}
	return nil
}

func emitDirectoryAsText(d *directory, w io.Writer) error {
	for _, entry := range d.List() {
		if entry.Directory != nil {
			err := emitDirectoryAsText(entry.Directory, w)
			if err != nil {
				return err
			}
		} else {
			_, err := fmt.Fprintf(w, "%s ", entry.Metric.AbsPath())
			if err != nil {
				return err
			}
			err = emitMetricAsText(entry.Metric, w)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func emitDistributionAsText(s *snapshot, w io.Writer) error {
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

func emitMetricAsText(m *metric, w io.Writer) error {
	if m.Value.Type() == types.Dist {
		return emitDistributionAsText(m.Value.AsDistribution().Snapshot(), w)
	}
	_, err := fmt.Fprintf(w, "%s\n", m.Value.AsTextString())
	return err
}

func doTextFormatting(
	d *directory, m *metric, w http.ResponseWriter) error {
	if d == nil && m == nil {
		fmt.Fprintf(w, "*Path does not exist.*")
		return nil
	}
	if m == nil {
		return emitDirectoryAsText(d, w)
	}
	return emitMetricAsText(m, w)
}

func doHtmlFormatting(
	d *directory, m *metric, w http.ResponseWriter) error {
	if d == nil && m == nil {
		fmt.Fprintf(w, "Path does not exist.")
		return nil
	}
	if m == nil {
		return emitDirectoryAsHtml(d, w)
	}
	return emitMetricAsHtml(m, w)
}

func handleError(w http.ResponseWriter, err error) {
	fmt.Fprintln(w, "Error in template.")
	errLog.Printf("Error in template: %v\n", err)
}

func browseFunc(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	path := r.URL.Path
	d, m := root.GetDirectoryOrMetric(path)
	var err error
	if r.Form.Get("format") == "text" {
		err = doTextFormatting(d, m, w)
	} else {
		err = doHtmlFormatting(d, m, w)
	}
	if err != nil {
		handleError(w, err)
	}
}

/*
type jsonListingGroup map[string]*JsonListing

func (j jsonListing) Contains(path string) bool {
	result := j[path]
	return result != nil
}

func (j jsonListingGroup) Add(l *JsonListing) {
	j[l.Path] = l
}

func restFunc(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	path := r.URL.Path
	d, m := root.GetDirectoryOrMetric(path)
	listingGroup := make(jsonListingGroup)
	if m != nil {
		listingGroup.Add(asJsonListing(m))
	}

	var err error
}
*/

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

func registerDefaultMetrics() {
	RegisterMetric("/name", &os.Args[0], units.None, "Program name")
	RegisterMetric("/args", getProgramArgs, units.None, "Program args")
	RegisterMetric("/start-time", &appStartTime, units.None, "Program start time")
}

func initHttpFramework() {
	appStartTime = time.Now()
	errLog = log.New(os.Stderr, "", log.LstdFlags|log.Lmicroseconds)
}

func registerBrowserHandlers() {
	http.Handle(browseMetricsUrl+"/", http.StripPrefix(browseMetricsUrl, http.HandlerFunc(browseFunc)))
	//	http.Handle(restUrl+"/", http.StripPrefix(restUrl, http.HandlerFunc(restFunc)))
	http.Handle("/metricsstatic/", http.StripPrefix("/metricsstatic", newStatic()))
}

func init() {
	registerDefaultMetrics()
	initHttpFramework()
	registerBrowserHandlers()

}
