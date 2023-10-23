package main

import (
	"archive/zip"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
)

const prefixString = `
package main

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"io"
	"os"
	"path/filepath"
)

const zipdata = `

const suffixString = `

// Unzip extracts the zip data to the file system.
func Unzip(path string) error {
	// Decode the zip data.
	data, err := base64.StdEncoding.DecodeString(zipdata)
	if err != nil {
		return err
	}

	// Open the zip archive.
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return err
	}

	// Extract the files in the archive.
	for _, f := range r.File {
		if err := extractFile(f, path); err != nil {
			return err
		}
	}

	return nil
}

// extractFile extracts a single file from the zip archive.
func extractFile(f *zip.File, path string) error {
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

// Main function accepts a directory or file name from the command line argument,
// and creates a zip-encoded buffer that can be written to a file as a Go constant
// expression.
func main() {
	var (
		path   string
		output = "zip.go"
		size   int
	)

	for index := 1; index < len(os.Args); index++ {
		arg := os.Args[index]

		switch arg {
		case "-o", "--output":
			index++
			if index >= len(os.Args) {
				fmt.Println("Missing output file name")
				os.Exit(1)
			}

			arg = os.Args[index]
			ext := filepath.Ext(arg)

			if ext == "" {
				arg = arg + ".go"
			} else {
				if ext != ".go" {
					fmt.Println("Output file must have .go extension")
					os.Exit(1)
				}
			}

			output = arg

		default:
			path = arg
		}
	}

	if path == "" {
		fmt.Println("Usage: zipgo <path>")
		os.Exit(1)
	}

	// Make a buffer to hold the zip-encoded data.
	buf := new(bytes.Buffer)

	// Create a new zip archive.
	w := zip.NewWriter(buf)

	// Add files to the archive.
	if err := addFiles(w, path, ""); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// Close the zip archive.
	if err := w.Close(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// Open the output text file and write the header from the constant.
	f, err := os.Create(output)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	defer f.Close()

	// Write the header for the constant.
	if n, err := f.WriteString(prefixString); err != nil {
		fmt.Println(err)
		os.Exit(1)
	} else {
		size += n
	}

	// Encode the zip data as a Go string constant.
	if encodedSize, err := f.WriteString(encode(buf.Bytes())); err != nil {
		fmt.Println(err)
		os.Exit(1)
	} else {
		size += encodedSize
	}

	// Close out the string constant.
	if n, err := f.WriteString("\n\n"); err != nil {
		fmt.Println(err)
		os.Exit(1)
	} else {
		size += n
	}

	// Write the function that unpacks the zip data back to the file system.
	if n, err := f.WriteString(suffixString); err != nil {
		fmt.Println(err)
		os.Exit(1)
	} else {
		size += n
	}

	// All done, close the file
	if err := f.Close(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	fmt.Println("Wrote zip data to", output, "(", size, "bytes)")
}

// addFiles walks the file tree rooted at path and adds each file or directory
// to the zip archive.
func addFiles(w *zip.Writer, path, prefix string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}

	if info.IsDir() {
		return addDir(w, path, prefix)
	}

	return addFile(w, path, prefix)
}

// addDir adds the files in a directory to the zip archive.
func addDir(w *zip.Writer, path, prefix string) error {
	// Open the directory.
	dir, err := os.Open(path)
	if err != nil {
		return err
	}

	defer dir.Close()

	// Get list of files in the directory.
	files, err := dir.Readdir(-1)
	if err != nil {
		return err
	}

	// Recursively add files in the subdirectories.
	for _, file := range files {
		if file.IsDir() {
			if err := addDir(w, filepath.Join(path, file.Name()), filepath.Join(prefix, file.Name())); err != nil {
				return err
			}
		} else {
			if err := addFile(w, filepath.Join(path, file.Name()), prefix); err != nil {
				return err
			}
		}
	}

	return nil
}

// addDir adds a single file to the zip archive.
func addFile(w *zip.Writer, path, prefix string) error {
	// Open the file.
	file, err := os.Open(path)
	if err != nil {
		return err
	}

	defer file.Close()

	zipfile := filepath.Join(prefix, file.Name())

	zf, err := w.Create(zipfile)
	if err != nil {
		return err
	}

	// Get file contents and write to the zip archive
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	if _, err := zf.Write(data); err != nil {
		return err
	}

	return nil
}

// encode converts a byte slice to a Go string constant.
func encode(data []byte) string {
	var (
		buf        bytes.Buffer
		lineLength = 0
		n          int
	)

	for _, b := range data {
		switch b {
		case '\n':
			n, _ = buf.WriteString("\\n")
		case '\r':
			n, _ = buf.WriteString("\\r")
		case '\t':
			n, _ = buf.WriteString("\\t")
		case '"':
			n, _ = buf.WriteString("\\\"")
		case '\\':
			n, _ = buf.WriteString("\\\\")
		default:
			if b < 32 || b > 126 {
				n, _ = fmt.Fprintf(&buf, "\\x%02x", b)
			} else {
				_ = buf.WriteByte(b)
				n = 1
			}
		}

		lineLength += n
		if lineLength >= 80 {
			buf.WriteByte('\n')

			lineLength = 0
		}
	}

	return "`" + buf.String() + "`"
}