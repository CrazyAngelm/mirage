package downloader

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type Entry struct {
	Name       string
	TargetName string
	URL        string
	EnvURL     string
	SHA256     string
	Archive    string
	ArchiveBin string
}

type Manifest struct {
	Entries []Entry
}

const (
	XrayVersion     = "v26.3.27"
	HysteriaVersion = "app/v2.9.1"
)

func DefaultManifest(goos string, goarch string) Manifest {
	if goos != "linux" {
		return Manifest{}
	}
	switch goarch {
	case "amd64":
		return Manifest{Entries: []Entry{
			{Name: "xray", TargetName: "xray", EnvURL: "XRAY_URL", URL: "https://github.com/XTLS/Xray-core/releases/download/" + XrayVersion + "/Xray-linux-64.zip", Archive: "zip", ArchiveBin: "xray", SHA256: "23cd9af937744d97776ee35ecad4972cf4b2109d1e0fe6be9930467608f7c8ae"},
			{Name: "hysteria", TargetName: "hysteria", EnvURL: "HYSTERIA_URL", URL: "https://github.com/apernet/hysteria/releases/download/" + HysteriaVersion + "/hysteria-linux-amd64", SHA256: "0524001e171f543624124e0b61ad4db568f6113d0543d7d9f2e080f1a699c065"},
			{Name: "amneziawg", TargetName: "amneziawg", EnvURL: "AMNEZIAWG_URL"},
		}}
	case "arm64":
		return Manifest{Entries: []Entry{
			{Name: "xray", TargetName: "xray", EnvURL: "XRAY_URL", URL: "https://github.com/XTLS/Xray-core/releases/download/" + XrayVersion + "/Xray-linux-arm64-v8a.zip", Archive: "zip", ArchiveBin: "xray"},
			{Name: "hysteria", TargetName: "hysteria", EnvURL: "HYSTERIA_URL", URL: "https://github.com/apernet/hysteria/releases/download/" + HysteriaVersion + "/hysteria-linux-arm64"},
			{Name: "amneziawg", TargetName: "amneziawg", EnvURL: "AMNEZIAWG_URL"},
		}}
	default:
		return Manifest{}
	}
}

func RuntimeManifest() Manifest {
	return DefaultManifest(runtime.GOOS, runtime.GOARCH)
}

func (m Manifest) Entry(name string) (Entry, bool) {
	for _, entry := range m.Entries {
		if entry.Name == name {
			return entry, true
		}
	}
	return Entry{}, false
}

func DownloadAll(ctx context.Context, manifest Manifest, targetDir string) error {
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return err
	}
	for _, entry := range manifest.Entries {
		if err := Download(ctx, entry, targetDir); err != nil {
			return err
		}
	}
	return nil
}

func Download(ctx context.Context, entry Entry, targetDir string) error {
	url := os.Getenv(entry.EnvURL)
	if url == "" {
		url = entry.URL
	}
	if url == "" {
		return fmt.Errorf("%s URL missing; set %s", entry.Name, entry.EnvURL)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("download %s failed: HTTP %d", entry.Name, resp.StatusCode)
	}
	tmp, err := os.CreateTemp("", "mirage-download-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	h := sha256.New()
	if _, err := io.Copy(io.MultiWriter(tmp, h), resp.Body); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if entry.SHA256 != "" && !strings.EqualFold(hex.EncodeToString(h.Sum(nil)), entry.SHA256) {
		return fmt.Errorf("%s checksum mismatch", entry.Name)
	}
	if entry.Archive == "zip" {
		return extractZipBinary(tmpPath, entry.ArchiveBin, filepath.Join(targetDir, entry.TargetName))
	}
	return installFile(tmpPath, filepath.Join(targetDir, entry.TargetName))
}

func installFile(src string, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func extractZipBinary(zipPath string, binName string, dst string) error {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer zr.Close()
	for _, file := range zr.File {
		if filepath.Base(file.Name) != binName {
			continue
		}
		r, err := file.Open()
		if err != nil {
			return err
		}
		defer r.Close()
		out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, r); err != nil {
			out.Close()
			return err
		}
		return out.Close()
	}
	return fmt.Errorf("%s not found in archive", binName)
}
