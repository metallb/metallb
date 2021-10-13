// SPDX-License-Identifier:Apache-2.0

package main

import (
	"fmt"
	"net"
	"os"
)

func main() {
	ln1, err := net.Listen("tcp", ":8080")
	if err != nil {
		panic(err)
	}
	ln2, err := net.Listen("tcp", ":8081")
	if err != nil {
		panic(err)
	}
	ln3, err := net.Listen("tcp", ":8082")
	if err != nil {
		panic(err)
	}
	go loop(ln1)
	go loop(ln2)
	loop(ln3)
}

func loop(ln net.Listener) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			continue
		}
		go respond(conn)
	}
}

func respond(conn net.Conn) {
	defer conn.Close()
	host, _, err := net.SplitHostPort(conn.RemoteAddr().String())
	if err != nil {
		return
	}
	fmt.Fprintf(conn, "%s\n%s\n%s\n", os.Getenv("NODE_NAME"), os.Getenv("POD_UID"), host)
}
