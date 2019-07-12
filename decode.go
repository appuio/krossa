package main

import (
	"fmt"
	"os"

	"github.com/francoispqt/gojay"
)

func decodeInputFile(path string) (*k8List, error) {
	fh, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer fh.Close()

	list := &k8List{}

	dec := gojay.BorrowDecoder(fh)
	defer dec.Release()

	if err := dec.DecodeObject(list); err != nil {
		return nil, err
	}

	return list, nil
}

func decodeInputFiles(paths <-chan string, objects chan<- *k8Object, errors chan<- error) {
	for i := range paths {
		if list, err := decodeInputFile(i); err != nil {
			errors <- fmt.Errorf("%s: %s", i, err)
		} else {
			for _, i := range list.items {
				objects <- i
			}
		}
	}
}
