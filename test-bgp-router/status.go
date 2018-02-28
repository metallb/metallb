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
  <style>
td {
  border-style: solid; border-width: 2px; padding: 15px; vertical-align: top;
}

h2 { text-align: center; margin: 0; padding: 0; }
h3 { text-align: center; margin: 0; padding: 0; }
  </style>
  <h1 align="center">Router status</h1>
<table style="margin: auto; border-collapse: collapse">
  <tr><td colspan="{{len .}}"><h2>Router Status</h2></td></tr>
  <tr>{{range .}}<td><h3>{{.Name}}</h3></td>{{end}}</tr>
  <tr>
    {{ range . }}
    <td>
      {{ if .Connected }}
      <pre><code>
    __________               ╔═════════════╗               _____________
   |          |              ║             ║      BGP     |             |
   | Internet |--------------║ This router ║<-------------| Minikube VM |     
   |__________|              ║             ║              |_____________|
                             ╚═════════════╝
      </code></pre>
      <p><b>MetalLB is connected!</b><p>
      {{ if .Prefixes }}
        <p>MetalLB is giving me routes to these destinations:</p>
        <ul>
          {{ range .Prefixes }}<li>{{.}}</li>{{ end }}
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
      </code></pre>
      <p><b>MetalLB is not connected</b>, I don't know anything about services in the cluster.</p>
      {{ end }}
    </td>
    {{ end }}
  </tr>

  <tr><td colspan="{{len .}}"><h2>Raw Router Status</h2></td></tr>
  <tr>{{range .}}<td><h3>{{.Name}}</h3></td>{{end}}</tr>

  <tr>
  {{ range . }}
  <td>
    <pre><code>{{.ProtocolStatus}}
    {{.Routes}}
    </code></pre>
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
	var vals []*values

	if hasBird() {
		bStat, err := birdStatus()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		vals = append(vals, bStat)
	}
	if hasQuagga() {
		qStat, err := quaggaStatus()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		vals = append(vals, qStat)
	}

	if err := tmpl.Execute(w, vals); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
