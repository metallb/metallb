package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"testing"
)

func TestMain(m *testing.M) {
	flag.Parse()

	if testing.Short() {
		fmt.Println("Skipping e2e tests because short testing was requested.")
		os.Exit(0)
	}

	cmd := exec.Command(
		"go", "run", "make.go",
		"-a", "build,image",
		"-b", "controller,speaker,e2etest-mirror-server",
		"--tag", "e2e",
		"--registry", "metallb",
	)
	cmd.Dir = ".."
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("Failed to build MetalLB images: %v", err)
		os.Exit(1)
	}

	os.Exit(m.Run())
}
