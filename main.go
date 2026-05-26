package main

import (
	"flag"
	"fmt"
	"os"

	"epub-reader/store"
	"epub-reader/ui"
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] [file.epub]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nA terminal EPUB reader.\n")
	}
	flag.Parse()

	s, err := store.New()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing store: %v\n", err)
		os.Exit(1)
	}

	app := ui.NewApp(s)
	if err := app.Run(flag.Args()); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
