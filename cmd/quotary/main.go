package main

import (
	"fmt"
	"os"

	"github.com/juanrubio/quotary/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "quotary:", err)
		os.Exit(1)
	}
}
