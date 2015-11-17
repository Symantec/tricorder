package tricorder

import (
	"fmt"
	"github.com/Symantec/tricorder/go/tricorder/types"
	"github.com/Symantec/tricorder/go/tricorder/units"
	"html/template"
	"io"
	"net/http"
	"strconv"
	"strings"
)

const (
	htmlUrl         = "/metrics"
	htmlTemplateStr = `
	{{define "METRIC"}} \
	  {{with $top := .}} \
            {{if .IsDistribution}} \
	      {{.Metric.AbsPath}} <span class="parens">(distribution: {{.Metric.Description}}{{if .HasUnit}}; unit: {{.Metric.Unit}}{{end}})</span><br>
	      {{with .Metric.AsDistribution.Snapshot}}
	        <table>
	        {{range .Breakdown}} \
	          {{if .Count}} \
	            <tr>
  	            {{if .First}} \
	              <td align="right">&lt;{{$top.ToFloat32 .End}}:</td><td align="right">{{.Count}}</td>
	            {{else if .Last}} \
	              <td align="right">&gt;={{$top.ToFloat32 .Start}}:</td><td align="right"> {{.Count}}</td>
	            {{else}} \
	              <td align="right">{{$top.ToFloat32 .Start}}-{{$top.ToFloat32 .End}}:</td> <td align="right">{{.Count}}</td>
	            {{end}} \
		    </tr>
		  {{end}} \
		{{end}} \
		</table>
	        {{if .Count}} \
		<span class="summary"> min: {{$top.ToFloat32 .Min}} max: {{$top.ToFloat32 .Max}} avg: {{$top.ToFloat32 .Average}} &#126;median: {{$top.ToFloat32 .Median}} sum: {{$top.ToFloat32 .Sum}} count: {{.Count}}</span><br><br>
	        {{end}} \
	      {{end}} \
	    {{else}} \
	      {{.Metric.AbsPath}} {{.AsHtmlString}} <span class="parens">({{.Metric.Type}}{{if .Metric.Bits}}{{.Metric.Bits}}{{end}}: {{.Metric.Description}}{{if .HasUnit}}; unit: {{.Metric.Unit}}{{end}})</span><br>
	    {{end}} \
	  {{end}} \
	{{end}} \
	<html>
	<head>
	  <link rel="stylesheet" type="text/css" href="/metricsstatic/theme.css">
	</head>
	<body>
	{{with $top := .}} \
	  {{if .Directory}} \
	    {{range .Directory.List}} \
	      {{if .Directory}} \
	        <a href="{{$top.Link .Directory}}">{{.Directory.AbsPath}}</a><br>
              {{else}} \
	        {{template "METRIC" $top.AsMetricView .Metric}} \
	      {{end}} \
	    {{end}} \
	  {{else}} \
	    {{template "METRIC" .}} \
	  {{end}} \
	{{end}} \
	</body>
	</html>
	  `

	themeCss = `
	.summary {color:#999999; font-style: italic;}
	.parens {color:#999999;}
	  `
)

var (
	htmlTemplate = template.Must(template.New("browser").Parse(strings.Replace(htmlTemplateStr, " \\\n", "", -1)))
)

type htmlView struct {
	Directory *directory
	Metric    *metric
	Session   *session
}

func (v *htmlView) AsMetricView(m *metric) *htmlView {
	return &htmlView{Metric: m, Session: v.Session}
}

func (v *htmlView) AsHtmlString() string {
	return v.Metric.AsHtmlString(v.Session)
}

func (v *htmlView) IsDistribution() bool {
	return v.Metric.Type() == types.Dist
}

func (v *htmlView) HasUnit() bool {
	return v.Metric.Unit != units.None
}

func (v *htmlView) Link(d *directory) string {
	return htmlUrl + d.AbsPath()
}

func (v *htmlView) ToFloat32(f float64) float32 {
	return float32(f)
}

func htmlEmitMetric(m *metric, s *session, w io.Writer) error {
	v := &htmlView{Metric: m, Session: s}
	if err := htmlTemplate.Execute(w, v); err != nil {
		return err
	}
	return nil
}

func htmlEmitDirectory(d *directory, s *session, w io.Writer) error {
	v := &htmlView{Directory: d, Session: s}
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
	s := newSession()
	defer s.Close()
	if m == nil {
		return htmlEmitDirectory(d, s, w)
	}
	return htmlEmitMetric(m, s, w)
}

type textCollector struct {
	W io.Writer
}

func (c *textCollector) Collect(m *metric, s *session) (err error) {
	if _, err = fmt.Fprintf(c.W, "%s ", m.AbsPath()); err != nil {
		return
	}
	return textEmitMetric(m, s, c.W)
}

func textEmitDistribution(s *snapshot, w io.Writer) error {
	_, err := fmt.Fprintf(
		w,
		"{min:%s;max:%s;avg:%s;median:%s;sum:%s;count:%d",
		strconv.FormatFloat(s.Min, 'f', -1, 32),
		strconv.FormatFloat(s.Max, 'f', -1, 32),
		strconv.FormatFloat(s.Average, 'f', -1, 32),
		strconv.FormatFloat(s.Median, 'f', -1, 32),
		strconv.FormatFloat(s.Sum, 'f', -1, 32),
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

func textEmitMetric(m *metric, s *session, w io.Writer) error {
	if m.Type() == types.Dist {
		return textEmitDistribution(m.AsDistribution().Snapshot(), w)
	}
	_, err := fmt.Fprintf(w, "%s\n", m.AsTextString(s))
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
		return d.GetAllMetrics(&textCollector{W: w}, nil)
	}
	return textEmitMetric(m, nil, w)
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

func newStatic() http.Handler {
	result := http.NewServeMux()
	addStatic(result, "/theme.css", themeCss)
	return result
}

func initHtmlHandlers() {
	http.Handle(
		htmlUrl+"/",
		http.StripPrefix(
			htmlUrl, http.HandlerFunc(htmlAndTextHandlerFunc)))
	http.Handle(
		"/metricsstatic/",
		http.StripPrefix("/metricsstatic", newStatic()))
}
