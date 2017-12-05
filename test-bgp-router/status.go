package main

import (
	"html/template"
	"net"
	"net/http"
)

var tmpl = template.Must(template.New("").Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <title>BGP Router Status</title>
</head>

<body>
  <h1 align="center">Router status</h1>
<table style="margin: auto; border-collapse: collapse">
  <thead><tr>
    {{ range . }}
    <th>{{ .Name }}</th>
    {{ end }}
  </tr></thead>
  <tr>
    {{ range . }}
    <td style="border-style: double; border-width: 5px; padding: 15px">

{{ if .Connected }}
<pre><code>
    __________               ╔═════════════╗               _____________
   |          |              ║             ║      BGP     |             |
   | Internet |--------------║ This router ║<-------------| Minikube VM |     
   |__________|              ║             ║              |_____________|
                             ╚═════════════╝
</pre></code>
<p><b>MetalLB is connected!</b><p>
{{ if .Prefixes }}
<p>MetalLB is giving me routes to these destinations:</p>
<ul>
{{ range .Prefixes }}
<li>{{.}}</li>
{{ end }}
</ul>
{{ else }}
<p>MetalLB is not telling me about any IPs yet. Try defining a LoadBalancer service?</p>
{{ end }}
{{ else }}
<pre><code>
    __________               ╔═════════════╗               _____________
   |          |              ║             ║              |             |
   | Internet |--------------║ This router ║      ~?~     | Minikube VM |
   |__________|              ║             ║              |_____________|
                             ╚═════════════╝
</pre></code>
<p><b>MetalLB is not connected</b>, I don't know anything about services in the cluster.</p>
{{ end }}
  <h2>Raw status from the BGP router</h2>
  <pre><code>{{.ProtocolStatus}}
{{.Routes}}</code></pre>
    </td>
    {{ end }}
  </tr>
</table>
</body>
</html>`))

type values struct {
	Name           string
	Connected      bool
	Prefixes       []*net.IPNet
	ProtocolStatus string
	Routes         string
}

func status(w http.ResponseWriter, r *http.Request) {
	bStat, err := birdStatus()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	qStat, err := quaggaStatus()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	tmpl.Execute(w, []*values{bStat, qStat})
}
