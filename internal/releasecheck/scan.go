package releasecheck

import "strings"

type Finding struct {
	Path   string
	Reason string
}

func ScanText(path string, text string) []Finding {
	text = strings.ReplaceAll(text, "mirage://example", "")
	text = strings.ReplaceAll(text, "203.0.113.", "")
	text = strings.ReplaceAll(text, "198.51.100.", "")
	checks := []struct{ needle, reason string }{
		{"mirage://", "full mirage link"},
		{"client_private_key", "client private key"},
		{"private_key:", "private key"},
		{"PRIVATE KEY", "private key block"},
		{"ssh-key-", "ssh key path"},
		{"default.link", "generated link file"},
		{"profile.link", "generated profile link"},
	}
	var findings []Finding
	for _, check := range checks {
		if strings.Contains(text, check.needle) {
			findings = append(findings, Finding{Path: path, Reason: check.reason})
		}
	}
	return findings
}
