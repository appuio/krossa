package main

import (
	"bufio"
	"errors"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/francoispqt/gojay"
)

const (
	outputBufferSize = 128 * 1024
	outputFileHeader = `{
  "kind": "List",
  "apiVersion": "v1",
  "metadata": {
    "annotations": {
      "krossa.appuio.ch/comment": "Object order is unpredictable"
    }
  },
  "items": [
`
	outputFileFooter = `
]}`
)

// Compute list of path components for output files for a given object.
func outputFileComponents(obj *k8Object) [][]string {
	// Sanitize a name for use in a filename.
	//
	// K8s object and namespace names should already use a limited alphabet, but
	// it's better to make sure when names are used as part of a path.
	ns := url.PathEscape(obj.Namespace())
	kind := url.PathEscape(obj.Kind())

	return [][]string{
		{"__all__.json"},
		{ns, "__all__.json"},
		{ns, kind + ".json"},
	}
}

type outputFile struct {
	path  string
	fh    io.WriteCloser
	bufw  *bufio.Writer
	count int
}

var _ io.Closer = &outputFile{}

// Open a new output file; any existing file is truncated. Prepares the file
// for writing a Kubernetes list object.
func newOutputFile(path string) (*outputFile, error) {
	fh, err := os.OpenFile(path, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0600)
	if err != nil {
		return nil, err
	}

	bufw := bufio.NewWriterSize(fh, outputBufferSize)

	if _, err := bufw.WriteString(outputFileHeader); err != nil {
		return nil, err
	}

	return &outputFile{
		fh:   fh,
		bufw: bufw,
	}, nil
}

// Write an object to the file.
func (f *outputFile) WriteObject(obj *k8Object) error {
	if f.count > 0 {
		if _, err := f.bufw.WriteString(",\n"); err != nil {
			return err
		}
	}

	f.count++

	enc := gojay.BorrowEncoder(f.bufw)
	defer enc.Release()

	if err := enc.EncodeEmbeddedJSON(&obj.raw); err != nil {
		return err
	}

	return nil
}

// Close file after flushing buffers.
func (f *outputFile) Close() error {
	if _, err := f.bufw.WriteString(outputFileFooter); err != nil {
		return err
	}

	if err := f.bufw.Flush(); err != nil {
		return err
	}

	if err := f.fh.Close(); err != nil {
		return err
	}

	return nil
}

type outputFileWriter struct {
	baseDir string
	files   map[string]*outputFile
}

var _ io.Closer = &outputFileWriter{}

func newOutputFileWriter(baseDir string) *outputFileWriter {
	return &outputFileWriter{
		baseDir: baseDir,
		files:   map[string]*outputFile{},
	}
}

// Ensure all directories in the given components exist beneath the base
// directory. The last component is assumed to be a filename.
func (w *outputFileWriter) makeOutputDir(components []string) error {
	path := []string{
		w.baseDir,
	}

	// Skip last component as it's a filename
	for i := 0; i < (len(components) - 1); i++ {
		if len(components[i]) == 0 {
			return errors.New("Empty output path component")
		}

		path = append(path, components[i])

		if err := os.Mkdir(filepath.Join(path...), 0700); err != nil && !os.IsExist(err) {
			return err
		}
	}

	return nil
}

// Open an output file for the given path components. Once opened files are
// always kept open.
func (w *outputFileWriter) openOutputFile(components []string) (*outputFile, error) {
	key := strings.Join(components, "\u0000")

	if file, ok := w.files[key]; ok {
		return file, nil
	}

	if err := w.makeOutputDir(components); err != nil {
		return nil, err
	}

	path := filepath.Join(append([]string{w.baseDir}, components...)...)
	file, err := newOutputFile(path)

	if err != nil {
		return nil, err
	}

	w.files[key] = file

	return file, nil
}

// Write an object to applicable output files
func (w *outputFileWriter) Write(obj *k8Object) error {
	for _, components := range outputFileComponents(obj) {
		file, err := w.openOutputFile(components)
		if err != nil {
			return err
		}

		if err = file.WriteObject(obj); err != nil {
			return err
		}
	}

	return nil
}

// Close all files and flush buffers
func (w *outputFileWriter) Close() error {
	for _, i := range w.files {
		if err := i.Close(); err != nil {
			return err
		}
	}

	return nil
}
