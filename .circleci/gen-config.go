package main

import (
	"html/template"
	"os"

	"github.com/golang/glog"
)

func main() {
	tmpl := template.Must(template.ParseFiles("config.yml.tmpl"))
	v := map[string][]string{
		"GoVersions": []string{"1.8", "1.9", "1.10rc"},
		"Binary":     []string{"controller", "speaker", "test-bgp-router"},
		"Arch":       []string{"amd64", "arm", "arm64", "ppc64le", "s390x"},
	}
	if err := tmpl.Execute(os.Stdout, v); err != nil {
		glog.Fatalf("Error executing template: %s", err)
	}
}
