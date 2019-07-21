package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "%s\n%s\n%s\n", os.Getenv("NODE_NAME"), os.Getenv("POD_UID"), strings.Split(r.RemoteAddr, ":")[0])
	})
	go func() { log.Fatal(http.ListenAndServe(":8080", nil)) }()
	go func() { log.Fatal(http.ListenAndServe(":8081", nil)) }()
	log.Fatal(http.ListenAndServe(":8082", nil))
}
