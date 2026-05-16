package main

import (
	"fmt"
	"os"

	"mirage/internal/app"
)

func main() {
	if err := app.RunServer(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
