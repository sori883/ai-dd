package cli

import (
	"strings"
	"testing"
)

func TestGitIndependentInstall(t *testing.T) {
	for _, args := range [][]string{{"intent", "configure", "--help"}, {"unit", "result", "--help"}, {"intent", "hash", "--help"}} {
		text, ok := Help(args)
		if !ok || !strings.Contains(text, "verification_sha256") {
			t.Errorf("missing hash contract: %v", args)
		}
		for _, old := range []string{"code_revision", "direct_commit", "base_commit", "result_commit", "integrated_commit"} {
			if strings.Contains(text, old) {
				t.Errorf("old field %s in %v", old, args)
			}
		}
	}
}

func TestGitIndependentInstallReviewHelp(t *testing.T) {
	text, ok := Help([]string{"intent", "review", "--help"})
	if !ok {
		t.Fatal("missing review help")
	}
	for _, contract := range []string{"別session", "同じrootを使える", "別rootではIntentの検証対象集合SHAの一致", "同じsession/root/target"} {
		if !strings.Contains(text, contract) {
			t.Errorf("review help omits %q", contract)
		}
	}
	if strings.Contains(text, "別session、別rootを指定") {
		t.Error("review help requires a separate root")
	}
}
