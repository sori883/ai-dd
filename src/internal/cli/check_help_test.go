package cli_test

import (
	"bytes"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/sori883/ai-dd/src/internal/buildinfo"
	"github.com/sori883/ai-dd/src/internal/cli"
)

func TestCheckHelpEvidenceContract(t *testing.T) {
	text := runCheckHelp(t, []string{"intent", "check", "--help"})
	evidence := checkHelpSection(t, text, "実測証拠:")
	requireCheckHelpText(t, evidence, "コード", "テストコード", "commit", "文書outputsへ登録しない", "Sensor", "真正性")
}

func TestCheckHelpDocumentContracts(t *testing.T) {
	text := runCheckHelp(t, []string{"intent", "check", "--help"})
	cases := []struct {
		name     string
		path     string
		headings []string
		extra    []string
	}{
		{
			name: "Rule", path: "rules/rule.md",
			extra: []string{"固定の必須H2はない"},
		},
		{
			name: "Requirements", path: "design/<intent_id>/requirements.md",
			headings: []string{"目的", "範囲", "要件", "受入条件", "未確定事項"},
			extra:    []string{"現在intent_id", "必要"},
		},
		{
			name: "CurrentAnalysis", path: "codekb/current-analysis.md",
			headings: []string{"現状", "構成・動作", "根拠", "未確認事項"},
		},
		{
			name: "Architecture", path: "codekb/architecture.md",
			headings: []string{"構成図", "構成要素", "データフロー"},
			extra:    []string{"構成図節", "非空のMermaidコードブロック", "必要"},
		},
		{
			name: "ImplementationPlan", path: "design/<intent_id>/implementation-plan.md",
			headings: []string{"変更箇所", "実装手順", "検証方法"},
			extra:    []string{"現在intent_id", "必要"},
		},
		{
			name: "Knowledge", path: "codekb/",
			headings: []string{"機能", "利用手順", "制約"},
			extra:    nil,
		},
		{
			name: "adr", path: "adr/",
			extra: []string{"固定の必須H2はない", "現在intent_id", "必要"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			section := checkHelpSection(t, text, tc.name+":")
			requireCheckHelpText(t, section, tc.path)
			headings := []string{}
			for _, match := range regexp.MustCompile(`## ([^、。\r\n]+)`).FindAllStringSubmatch(section, -1) {
				headings = append(headings, strings.TrimSpace(match[1]))
			}
			if !slices.Equal(headings, tc.headings) {
				t.Errorf("%sの必須H2: got %q, want %q", tc.name, headings, tc.headings)
			}
			requireCheckHelpText(t, section, tc.extra...)
		})
	}
}

func TestCheckHelpDocumentVersionsAndOptionalOutputs(t *testing.T) {
	text := runCheckHelp(t, []string{"intent", "check", "--help"})
	requireCheckHelpText(t, checkHelpSection(t, text, "文書の版と省略:"), "受入済みpath", "内容hash", "planning省略時", "受入済みImplementationPlanを一律要求しない", "共有現在版", "任意入力")
	requireCheckHelpText(t, checkHelpSection(t, text, "文書outputs:"), "一律の既定文書出力はない")
}

func TestCheckHelpWorkflowAndProcedure(t *testing.T) {
	text := runCheckHelp(t, []string{"intent", "check", "--help"})
	t.Run("検査と完了", func(t *testing.T) {
		workflow := checkHelpSection(t, text, "検査と完了:")
		requireCheckHelpText(t, workflow,
			"読取り専用", "テストcommand自体は実行しない",
			"passだけでは段階は完了しない",
			"begin", "開始入力版", "再試行では差し替えない", "一般作業",
			"提示後", "実回答", "人間", "成果承認",
			"plan-approval", "実行計画", "approval", "成果", "別",
		)
		order := "開始Sensor → begin → 終了Sensor → 独立レビュー → 成果承認 → finish"
		requireCheckHelpText(t, workflow, order)
	})
	t.Run("現在の具体的な入出力", func(t *testing.T) {
		procedure := checkHelpSection(t, text, "入出力の確認:")
		requireCheckHelpText(t, procedure,
			"aidlc intent procedure ID --space SPACE",
			"段階変更", "再開後", "取り直す", "Sensor内部の全必須H2一覧を返す操作ではない",
		)
		if strings.Contains(text, "intent procedureの必須型/節") {
			t.Error("procedureが必須見出しを全て返すと読める旧案内が残っている")
		}
	})
}

func TestCheckHelpForms(t *testing.T) {
	forms := [][]string{
		{"intent", "check", "--help"},
		{"help", "intent", "check"},
		{"intent", "check", "help"},
		{"intent", "help", "check"},
	}
	want := runCheckHelp(t, forms[0])
	for _, args := range forms {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			if got := runCheckHelp(t, args); got != want {
				t.Errorf("help形式 %v のstdoutが異なる", args)
			}
		})
	}
}

func TestCheckHelpRejectsExecutionArguments(t *testing.T) {
	forms := [][]string{
		{"intent", "check", "0123456789abcdef0123456789abcdef", "--help"},
		{"help", "intent", "check", "0123456789abcdef0123456789abcdef"},
		{"intent", "check", "--help", "--space", "default"},
		{"intent", "help", "check", "0123456789abcdef0123456789abcdef"},
	}
	for _, args := range forms {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := cli.Run(
				args,
				&stdout,
				&stderr,
				buildinfo.Info{},
				cli.Dependencies{Execute: func(cli.CommandRequest) ([]byte, error) {
					t.Fatal("不正なhelp引数で実操作が呼ばれた")
					return nil, nil
				}},
			)
			if code != 2 || stdout.Len() != 0 || stderr.Len() == 0 {
				t.Errorf("不正なhelp: exit=%d stdout=%q stderr=%q", code, &stdout, &stderr)
			}
		})
	}
}

func TestCheckHelpPreservesBeginHelp(t *testing.T) {
	text := runCheckHelp(t, []string{"intent", "begin", "--help"})
	requireCheckHelpText(t, text,
		"aidlc intent begin ID --space SPACE --expect REVISION",
		"beginは開始入力版を保存し同段階の再試行では差し替えない",
		"一般作業/Unit claim前にbeginが必要",
	)
}

func runCheckHelp(t *testing.T, args []string) string {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := cli.Run(
		args,
		&stdout,
		&stderr,
		buildinfo.Info{},
		cli.Dependencies{
			Execute: func(cli.CommandRequest) ([]byte, error) {
				t.Fatal("helpから実操作が呼ばれた")
				return nil, nil
			},
			PrepareOutput: func() { t.Fatal("helpから作業環境の準備が呼ばれた") },
		},
	)
	if code != 0 || stderr.Len() != 0 || stdout.Len() == 0 {
		t.Fatalf("help %v: exit=%d stdout=%q stderr=%q", args, code, &stdout, &stderr)
	}
	return stdout.String()
}

// 各案内の段落内を検査し、別の段階に書かれた語で条件を満たさないようにする。
func checkHelpSection(t *testing.T, text, heading string) string {
	t.Helper()
	_, rest, ok := strings.Cut(text, heading)
	if !ok {
		t.Fatalf("helpに案内 %q がない", heading)
	}
	section, _, _ := strings.Cut(strings.TrimLeft(rest, "\r\n"), "\n\n")
	return section
}

func requireCheckHelpText(t *testing.T, text string, wants ...string) {
	t.Helper()
	text = strings.Join(strings.Fields(text), " ")
	for _, want := range wants {
		if !strings.Contains(text, want) {
			t.Errorf("案内に %q がない: %s", want, text)
		}
	}
}
