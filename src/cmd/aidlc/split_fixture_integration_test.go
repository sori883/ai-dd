//go:build integration

package main

import (
	"encoding/json"
	"fmt"
	"github.com/sori883/ai-dd/src/internal/install"
	"path/filepath"
	"strings"
)

// Legacy journey fixtures exercise runtime state with the internal installer.
// ReleaseCandidateNative separately exercises the real version-selecting installer.
func fixtureInstall(binary, root string, args []string) (operationsResult, bool) {
	if len(args) < 2 || args[0] != "install" || args[1] != "codex" {
		return operationsResult{}, false
	}
	value := func(name string) string {
		for i := 2; i+1 < len(args); i++ {
			if args[i] == name {
				return args[i+1]
			}
		}
		return ""
	}
	if v := value("--project-dir"); v != "" {
		root = v
	}
	var r any
	var err error
	if strings.Contains(strings.Join(args, " "), "--relocate") {
		r, err = install.Relocate(root, binary, value("--from-project-dir"), value("--from-binary"))
	} else {
		r, err = install.Codex(root, binary)
	}
	raw, _ := json.Marshal(r)
	if err != nil {
		return operationsResult{out: raw, stderr: []byte(fmt.Sprintln(err)), code: 2}, true
	}
	return operationsResult{out: raw}, true
}
func fixtureProduct(binary string, args []string) (string, []string) {
	if len(args) > 0 && args[0] == "memory" {
		return filepath.Join(filepath.Dir(binary), "okf"), args[1:]
	}
	if len(args) > 0 && args[0] == "__hook" {
		args = append(append([]string{}, args...), "--okf-binary", filepath.Join(filepath.Dir(binary), "okf"))
	}
	return binary, args
}
