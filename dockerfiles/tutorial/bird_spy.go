package main

import (
	"net/http"
	"os/exec"
)

func main() {
	http.HandleFunc("/", status)
	http.ListenAndServe(":8080", nil)
}

func status(w http.ResponseWriter, r *http.Request) {
	if !writeCmd(w, "/usr/sbin/birdc", "show", "protocol", "minikube") {
		return
	}
	if !writeCmd(w, "/usr/sbin/birdc", "show", "route", "all", "protocol", "minikube") {
		return
	}
}

func writeCmd(w http.ResponseWriter, cmd string, args ...string) bool {
	c := exec.Command(cmd, args...)
	bs, err := c.CombinedOutput()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return false
	}
	w.Write(bs)
	return true
}
