package diagnostics

import "testing"

func TestCheckResultSummary(t *testing.T) {
	result := Result{Name: "sing-box", OK: false, Fix: "place sing-box.exe in bin directory"}
	if result.Line() != "sing-box: FAIL - place sing-box.exe in bin directory" {
		t.Fatalf("line = %q", result.Line())
	}
}

func TestCheckResultSummaryIncludesOKDetail(t *testing.T) {
	result := Result{Name: "sing-box.exe", OK: true, Fix: `C:\Mirage\bin\sing-box.exe`}
	if result.Line() != `sing-box.exe: OK - C:\Mirage\bin\sing-box.exe` {
		t.Fatalf("line = %q", result.Line())
	}
}
