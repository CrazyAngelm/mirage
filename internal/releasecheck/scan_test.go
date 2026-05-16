package releasecheck

import "testing"

func TestScanCatchesSecrets(t *testing.T) {
	findings := ScanText("state/default.link", "mirage://abc\nclient_private_key: secret\n")
	if len(findings) < 2 {
		t.Fatalf("findings = %#v", findings)
	}
}

func TestScanAllowsFakeDocumentationValues(t *testing.T) {
	findings := ScanText("docs/example.md", "host 203.0.113.10\nmirage://example\n")
	if len(findings) != 0 {
		t.Fatalf("findings = %#v", findings)
	}
}
