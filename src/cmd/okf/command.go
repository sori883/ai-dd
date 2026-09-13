package main

import (
	"github.com/sori883/ai-dd/src/internal/okfapp"
	"github.com/sori883/ai-dd/src/internal/okfcli"
	"github.com/sori883/ai-dd/src/internal/projectroot"
	"os"
	"path/filepath"
)

func executeCommand(r okfcli.CommandRequest) ([]byte, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	root, err := projectroot.Resolve(r.ProjectDir, cwd, false)
	if err != nil {
		return nil, err
	}
	if r.BodyFile != "" {
		absolute, err := filepath.Abs(r.BodyFile)
		if err != nil {
			return nil, err
		}
		parent, err := filepath.EvalSymlinks(filepath.Dir(absolute))
		if err != nil {
			return nil, err
		}
		r.BodyFile = filepath.Join(parent, filepath.Base(absolute))
	}
	return (okfapp.Service{Root: root}).Execute(r)
}
