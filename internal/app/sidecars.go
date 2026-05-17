package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

func resolveSidecar(exeDir string, base string, name string) (string, bool) {
	candidates := []string{
		filepath.Join(exeDir, "bin", name),
		filepath.Join(base, "bin", name),
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, true
		}
	}
	return candidates[0], false
}

func clientExeDir() string {
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(exe)
}

func missingSidecarMessage(name string) string {
	return fmt.Sprintf("%s missing; download mirage-windows-amd64.zip, extract it, and run mirage-client.exe from the extracted folder", name)
}

func singBoxConfigLooksStale(payload []byte) (bool, string) {
	var decoded struct {
		Outbounds []map[string]any `json:"outbounds"`
	}
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return false, ""
	}
	for _, outbound := range decoded.Outbounds {
		if outbound["tag"] == "xray_ws_tls" {
			return true, "legacy xray_ws_tls outbound found"
		}
		if outbound["type"] == "vless" {
			return true, "native vless outbound found; current Mirage uses xray sidecar SOCKS"
		}
	}
	return false, ""
}
