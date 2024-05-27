// SPDX-License-Identifier:Apache-2.0

package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func CopyFile(src, dst string) error {
	srcFile := filepath.Clean(src)
	destFile := filepath.Clean(dst)
	source, err := os.Open(srcFile)
	if err != nil {
		err = fmt.Errorf("failed to open source file %s: %w", srcFile, err)
		return err
	}
	defer source.Close()

	srcStat, err := os.Stat(srcFile)
	if err != nil {
		err = fmt.Errorf("failed to check stats for %s: %w", srcFile, err)
		return err
	}

	destination, err := os.Create(destFile)
	if err != nil {
		err = fmt.Errorf("failed to create destination file: %s: %w", destFile, err)
		return err
	}
	defer destination.Close()

	_, err = io.Copy(destination, source)
	if err != nil {
		err = fmt.Errorf("failed to copy %s to %s: %w", srcFile, destFile, err)
		return err
	}

	err = os.Chmod(destFile, srcStat.Mode())
	if err != nil {
		err = fmt.Errorf("failed to apply permission on %s", destFile)
		return err
	}

	return nil
}

func main() {
	if len(os.Args) != 3 {
		fmt.Printf("Please provide two command line arguments.")
		return
	}

	sourceFile, destinationFile := os.Args[1], os.Args[2]

	err := CopyFile(sourceFile, destinationFile)
	if err != nil {
		fmt.Println(err)
		return
	}
}
