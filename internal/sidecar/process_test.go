package sidecar

import (
	"context"
	"testing"
)

func TestCommand(t *testing.T) {
	p := Process{Name: "xray", Path: "xray", Args: []string{"run", "-config", "xray.json"}}
	cmd := p.Command(context.Background())
	if cmd.Path != "xray" {
		t.Fatalf("Path = %q", cmd.Path)
	}
	if len(cmd.Args) != 4 {
		t.Fatalf("Args = %#v", cmd.Args)
	}
}
