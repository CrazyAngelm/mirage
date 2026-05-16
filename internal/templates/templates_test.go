package templates

import (
	"strings"
	"testing"
)

func TestWebIndexLooksNormal(t *testing.T) {
	if !strings.Contains(DefaultIndexHTML(), "<!doctype html>") {
		t.Fatal("index must be normal HTML")
	}
}
