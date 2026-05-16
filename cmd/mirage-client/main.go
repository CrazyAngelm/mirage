package main

import (
	"fmt"
	"os"

	"mirage/internal/app"
	"mirage/internal/gui"
)

func main() {
	if len(os.Args) == 1 {
		// No arguments: launch GUI mode
		gui.RunDesktop()
		return
	}
	if err := app.RunClient(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
