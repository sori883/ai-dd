package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestFiveCLIContract(t *testing.T) {
	var out, err bytes.Buffer
	code := run([]string{"--version"}, &out, &err)
	if code != 0 || !strings.HasPrefix(out.String(), "okf ") {
		t.Fatalf("%d %s %s", code, &out, &err)
	}
}
