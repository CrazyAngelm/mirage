package serverhost

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Detector func() (string, error)

func Normalize(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "https://")
	value = strings.TrimPrefix(value, "http://")
	value = strings.TrimSuffix(value, "/")
	return value
}

func Resolve(explicit string, detector Detector) (string, error) {
	if host := Normalize(explicit); host != "" {
		return host, nil
	}
	if detector == nil {
		detector = DetectPublicIP
	}
	host, err := detector()
	if err != nil {
		return "", err
	}
	host = Normalize(host)
	if host == "" {
		return "", fmt.Errorf("public host detection returned empty value")
	}
	return host, nil
}

func DetectPublicIP() (string, error) {
	client := http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("https://api.ipify.org")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return "", fmt.Errorf("public host detection failed: %s", resp.Status)
	}
	payload, err := io.ReadAll(io.LimitReader(resp.Body, 128))
	if err != nil {
		return "", err
	}
	return Normalize(string(payload)), nil
}
