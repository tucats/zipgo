package main

const (
	shortPrologString = `
package %s

const zipdata = `

	fullPrologString = `package %s

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
)

var zipdata = `

	epilogString = `// Unzip extracts the zip data to the file system. The path specifies the
// directory to extract the files to. If replace is true, existing files are
// replaced in the output directory.
func Unzip(path string, replace bool) error {
	// Open the zip archive.
	r, err := zip.NewReader(bytes.NewReader(zipdata), int64(len(zipdata)))
	if err != nil {
		return err
	}

	// Extract the files in the archive.
	for _, f := range r.File {
		if err := extractFile(f, path, replace); err != nil {
			return err
		}
	}

	return nil
}

// extractFile extracts a single file from the zip archive.
func extractFile(f *zip.File, path string, replace bool) error {
	// Open the file in the archive.
	rc, err := f.Open()
	if err != nil {
		return err
	}

	defer rc.Close()

	// Create the file in the file system.
	path = filepath.Join(path, f.Name)
	if f.FileInfo().IsDir() {
		if err := os.MkdirAll(path, 0755); err != nil {
			return err
		}
	} else {
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}

		// If the file exists and we are not replacing, do nothing.
		if _, err := os.Stat(path); !replace && err == nil {
			return nil
		}

		f, err := os.Create(path)
		if err != nil {
			return err
		}
		defer f.Close()

		// Copy the file contents.
		if _, err := io.Copy(f, rc); err != nil {
			return err
		}
	}

	return nil
}

`
)
