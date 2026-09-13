package app

import (
	"github.com/sori883/ai-dd/src/internal/cli"
	"github.com/sori883/ai-dd/src/internal/okfcli"
	"path/filepath"
)

func (s Service) productBinary(binary string) bool {
	return sameBinary(binary, s.Binary) || (filepath.IsAbs(s.OKFBinary) && sameBinary(binary, s.OKFBinary))
}
func (s Service) productHelp(argv []string) bool {
	if len(argv) < 2 {
		return false
	}
	if sameBinary(argv[0], s.Binary) {
		_, ok := cli.Help(argv[1:])
		return ok
	}
	if filepath.IsAbs(s.OKFBinary) && sameBinary(argv[0], s.OKFBinary) {
		_, ok := okfcli.Help(argv[1:])
		return ok
	}
	return false
}
func (s Service) productCommand(argv []string) (cli.CommandRequest, error) {
	if len(argv) < 2 {
		return cli.CommandRequest{}, invalid("missing product command")
	}
	if sameBinary(argv[0], s.Binary) {
		return cli.ParseCommand(argv[1:])
	}
	if !filepath.IsAbs(s.OKFBinary) || !sameBinary(argv[0], s.OKFBinary) {
		return cli.CommandRequest{}, invalid("unconfigured product binary")
	}
	r, err := okfcli.ParseCommand(argv[1:])
	if err != nil {
		return cli.CommandRequest{}, err
	}
	return cli.CommandRequest{Command: "memory", Action: r.Action, Target: r.Target, Space: r.Space, ProjectDir: r.ProjectDir, BodyFile: r.BodyFile, Actor: r.Actor, Expect: r.Expect, IntentID: r.IntentID, Metadata: r.Metadata}, nil
}
