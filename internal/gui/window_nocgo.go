//go:build !cgo

package gui

import "fmt"

type DesktopGUI struct{}

func (g *DesktopGUI) Show() {}

func RunDesktop() {
	fmt.Println("Mirage desktop GUI requires CGO-enabled build.")
	RunTray(NewController(getSupervisor()))
}
