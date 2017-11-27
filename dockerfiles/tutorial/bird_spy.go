package main

import (
	"html/template"
	"net"
	"net/http"
	"os/exec"
	"strings"
)

func main() {
	http.HandleFunc("/", status)
	http.ListenAndServe(":8080", nil)
}

var tmpl = template.Must(template.New("").Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <title>BGP Spy</title>
</head>

<body>
  <h1>Router status</h1>
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
  <h2>Raw status from the BIRD BGP router</h2>
  <pre><code>{{.Proto}}
{{.Route}}</code></pre>
</body>
</html>`))

func status(w http.ResponseWriter, r *http.Request) {
	proto, err := bird("show protocol all minikube")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	routes, err := bird("show route all protocol minikube")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	summary, err := bird("show route protocol minikube")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var cidrs []*net.IPNet
	// Quick and dirty parser to extract the prefixes from the route
	// dump.
	for _, l := range strings.Split(summary, "\n") {
		fs := strings.Split(l, " ")
		if len(fs) < 1 {
			continue
		}
		_, n, err := net.ParseCIDR(fs[0])
		if err != nil {
			continue
		}
		cidrs = append(cidrs, n)
	}

	v := map[string]interface{}{
		"Connected": strings.Contains(proto, "Established"),
		"Prefixes":  cidrs,

		"Proto": proto,
		"Route": routes,
	}
	tmpl.Execute(w, v)
}

func bird(cmd string) (string, error) {
	c := exec.Command("/usr/sbin/birdc", strings.Split(cmd, " ")...)
	bs, err := c.CombinedOutput()
	if err != nil {
		return "", err
	}
	return string(bs), nil
}
