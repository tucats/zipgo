package main

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	version = "1.0.2"

	helpText = `
Create a Go source file that can be used to unzip a file or directory tree.

Usage: zipgo [options] <path>

Options:
  -d, --data			Write only the zip data to the output file.
  -h, --help            Print this help text and exit
  -l, --log 			Log the files as they are added to the zip archive.
  -o, --output <file>   Write output to <file> (default: unzip.go)
  -p, --package <name>  Specify Go package name (default: main)
  -v, --version         Print version and exit

`
	shortPrefixString = `
package %s

const zipdata = `

	prefixString = `
package %s

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"io"
	"os"
	"path/filepath"
)

const zipdata = `

	suffixString = `

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
)

var (
	data bool
	log  bool
)

// Main function accepts a directory or file name from the command line argument,
// and creates a zip-encoded buffer that can be written to a file as a Go constant
// expression.
func main() {
	var (
		path   string
		output = "unzip.go"
		pkg    = "main"
		size   int
	)

	for index := 1; index < len(os.Args); index++ {
		arg := os.Args[index]

		switch arg {
		case "-d", "--data":
			data = true

		case "-p", "--package":
			index++
			if index >= len(os.Args) {
				fmt.Println("Missing package name")
				os.Exit(1)
			}

			pkg = os.Args[index]

		case "-l", "--log":
			log = true

		case "-h", "--help":
			fmt.Print(helpText)
			os.Exit(0)

		case "-v", "--version":
			fmt.Println("zipgo", version)
			os.Exit(0)

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

	// Write the header for the constant. The header is different if we are writing
	// out only the data part of the zip encoding, as opposed to the function that
	// also handles the unzip operation.
	header := prefixString

	if data {
		header = shortPrefixString
	}

	if n, err := f.WriteString(fmt.Sprintf(header, pkg)); err != nil {
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

	// Write the function that unpacks the zip data back to the file system, if we
	// are not writing out only the data part of the zip encoding.
	if !data {
		if n, err := f.WriteString(suffixString); err != nil {
			fmt.Println(err)
			os.Exit(1)
		} else {
			size += n
		}
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
	if log {
		fmt.Println(path + "/")
	}

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

	if log {
		fmt.Println(path)
	}

	defer file.Close()

	zf, err := w.Create(path)
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

// encode converts a byte slice to a Go string constant containing a base64 representation
// of the data.
func encode(data []byte) string {
	text := base64.StdEncoding.EncodeToString(data)

	b := strings.Builder{}

	b.WriteRune('`')

	for _, ch := range text {
		if b.Len()%60 == 0 {
			b.WriteString("\n")
		}

		b.WriteRune(ch)
	}

	b.WriteRune('`')

	return b.String()
}
