package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) != 2 {
		usage()
	}

	var f func() error
	switch os.Args[1] {
	case "image":
		f = buildImage
	case "build":
		f = buildUniverse
	case "run":
		f = runCluster
	default:
		f = usage
	}

	if err := f(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func usage() error {
	fmt.Println("need 1 argument, either 'build' or 'run'")
	os.Exit(1)
	panic("unreachable")
}
