package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"sync"
)

func main() {
	var readerCount int
	var outputDir string

	flag.IntVar(&readerCount, "readers", runtime.NumCPU(),
		"Number of reading Goroutines to start")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(),
			"Usage: %s [OPTIONS] <output-dir> <input-file...>\n\nOptions:\n",
			os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()

	if readerCount < 1 {
		readerCount = 1
	}

	if len(flag.Args()) < 2 {
		flag.Usage()
		os.Exit(2)
	}

	outputDir = flag.Arg(0)

	wgMessages := sync.WaitGroup{}
	errorChan := make(chan error, 4)
	messageCollector := outputMessageCollector{}

	wgWriter := sync.WaitGroup{}
	objectChan := make(chan *k8Object, 16)
	writer := newOutputFileWriter(outputDir)

	wgReader := sync.WaitGroup{}
	paths := make(chan string, 4)

	// Collect errors
	wgMessages.Add(1)
	go func() {
		defer wgMessages.Done()

		for i := range errorChan {
			messageCollector.AppendError(i)
		}
	}()

	// Create writer
	wgWriter.Add(1)
	go func() {
		defer wgWriter.Done()

		for obj := range objectChan {
			if err := writer.Write(obj); err != nil {
				errorChan <- err

				// Abort on first write error
				break
			}
		}
	}()

	go func() {
		wgWriter.Wait()

		if err := writer.Close(); err != nil {
			errorChan <- err
		}

		close(errorChan)
	}()

	// Create readers
	wgReader.Add(readerCount)
	for i := 0; i < readerCount; i++ {
		go func() {
			defer wgReader.Done()

			decodeInputFiles(paths, objectChan, errorChan)
		}()
	}

	go func() {
		wgReader.Wait()

		close(objectChan)
	}()

	for _, i := range flag.Args()[1:] {
		paths <- i
	}

	close(paths)

	wgMessages.Wait()

	os.Exit(messageCollector.Print())
}
