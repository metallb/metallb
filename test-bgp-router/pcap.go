package main

import (
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/golang/glog"
)

func writePcap(w http.ResponseWriter, r *http.Request) {
	f, err := os.Open("/run/tcpdump/pcap")
	if err != nil {
		http.Error(w, fmt.Sprintf("opening pcap file: %s", err), http.StatusInternalServerError)
	}
	defer f.Close()
	w.Header().Set("Content-Type", "application/octet-stream")
	if _, err := io.Copy(w, f); err != nil {
		glog.Fatalf("Failed to write pcap: %s", err)
	}
}
