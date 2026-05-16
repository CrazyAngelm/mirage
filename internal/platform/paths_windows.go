//go:build windows

package platform

import "os"

func ServerBaseDir() string { return `C:\ProgramData\MirageServer` }

func ClientBaseDir() string {
	if v := os.Getenv("ProgramData"); v != "" {
		return v + `\Mirage`
	}
	return `C:\ProgramData\Mirage`
}
