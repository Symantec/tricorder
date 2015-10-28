package tricorder

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

const (
	browseMetricsUrl = "/tricorder"
	htmlTemplateStr  = `
	<html>
	<head>
	  <link rel="stylesheet" type="text/css" href="/tricorderstatic/theme.css">
	</head>
	<body>
	{{with $top := .}}
	  {{range .List}}
	    {{if .Directory}}
	      <a href="{{$top.Link .Directory}}">{{.Directory.AbsPath}}</a><br>
            {{else}}
	      {{if $top.IsDistribution .Metric.Value.Type}}
	        {{.Metric.AbsPath}} (distribution: {{.Metric.Description}})<br>
	        {{with .Metric.Value.AsDistribution.Snapshot}}
		  <table>
	          {{range .Breakdown}}
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
		  </table>
	          {{if .Count}}
	            <span class="summary"> min: {{.Min}} max: {{.Max}} avg: {{.Average}} count: {{.Count}}</span><br><br>
	          {{end}}
		{{end}}
	      {{else}}
	        {{.Metric.AbsPath}} {{$top.AsHtmlString .Metric.Value}} ({{.Metric.Value.Type}}: {{.Metric.Description}})<br>
	      {{end}}
	    {{end}}
	  {{end}}
	{{end}}
	</body>
	</html>
	  `

	themeCss = `
	.summary {color:#999999; font-style: italic;}
	  `
)

var (
	htmlTemplate = template.Must(template.New("browser").Parse(htmlTemplateStr))
	errLog       *log.Logger
	appStartTime time.Time
)

type view struct {
	*directory
}

func (v *view) Link(d *directory) string {
	return browseMetricsUrl + d.AbsPath()
}

func (v *view) IsDistribution(t valueType) bool {
	return t == Dist
}

func (v *view) AsHtmlString(val value) string {
	str, err := val.AsHtmlString()
	if err != nil {
		return err.Error()
	}
	return str
}

func emitDirectoryAsHtml(d *directory, w io.Writer) error {
	v := &view{d}
	if err := htmlTemplate.Execute(w, v); err != nil {
		return err
	}
	return nil
}

func browseFunc(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	d := root.GetDirectory(path)
	if d == nil {
		fmt.Fprintf(w, "Path does not exist.")
		return
	}
	if err := emitDirectoryAsHtml(d, w); err != nil {
		fmt.Fprintln(w, "Error in template.")
		errLog.Printf("Error in template: %v\n", err)
	}
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

func init() {
	appStartTime = time.Now()
	http.Handle(browseMetricsUrl+"/", http.StripPrefix(browseMetricsUrl, http.HandlerFunc(browseFunc)))
	http.Handle("/tricorderstatic/", http.StripPrefix("/tricorderstatic", newStatic()))
	errLog = log.New(os.Stderr, "", log.LstdFlags|log.Lmicroseconds)
}
