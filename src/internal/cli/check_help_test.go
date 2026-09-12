package cli_test

import (
	"bytes"
	"errors"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/sori883/ai-dd/src/internal/buildinfo"
	"github.com/sori883/ai-dd/src/internal/cli"
)

func TestCheckHelpStageBoundaries(t *testing.T) {
	text := runCheckHelp(t, []string{"intent", "check", "--help"})
	cases := []struct {
		name  string
		start []string
		end   []string
	}{
		{
			name:  "initialization",
			start: []string{"Rule", "固定workflow", "3skill", "aidlc", "aidlc-cli", "aidlc-okf", "hooks", "有効なJSON"},
			end:   []string{"開始入力", "宣言出力"},
		},
		{
			name:  "discovery",
			start: []string{"Rule", "共有分析", "現在版", "任意"},
			end:   []string{"Requirements", "目的", "範囲", "受入条件", "阻害事項", "現在の集合SHA", "資材", "資材なし理由", "ADR要否"},
		},
		{
			name:  "architecture-analysis",
			start: []string{"受入済みRequirements", "共有分析", "任意"},
			end:   []string{"CurrentAnalysis", "Architecture", "各1件"},
		},
		{
			name:  "planning",
			start: []string{"受入済みRequirements", "共有分析", "任意"},
			end:   []string{"ImplementationPlan", "実装手順", "検証方法", "Unit計画"},
		},
		{
			name:  "tdd",
			start: []string{"受入済みRequirements", "先行planning", "受入済みImplementationPlan"},
			end:   []string{"直接実装の全体検証", "Unit", "内容照合", "反映", "現在回", "成功記録"},
		},
		{
			name:  "integration",
			start: []string{"受入済みRequirements", "先行planning", "ImplementationPlan", "先行tdd", "受入済み証拠"},
			end:   []string{"現在の集合SHA", "成功記録", "宣言文書"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			section := checkHelpSection(t, text, tc.name+":")
			start, end, ok := strings.Cut(section, "終了:")
			if !ok || !strings.Contains(start, "開始:") {
				t.Fatalf("開始と終了の条件が区別されていない: %s", section)
			}
			requireCheckHelpText(t, start, tc.start...)
			requireCheckHelpText(t, end, tc.end...)
		})
	}
}

func TestCheckHelpEvidenceContract(t *testing.T) {
	text := runCheckHelp(t, []string{"intent", "check", "--help"})
	t.Run("全段階の入出力", func(t *testing.T) {
		common := checkHelpSection(t, text, "全段階:")
		requireCheckHelpText(t, common,
			"開始", "固定workflow", "Rule", "宣言入力", "資材",
			"終了", "begin", "現在のstep_id", "宣言出力",
		)
	})
	t.Run("終了時の共通条件", func(t *testing.T) {
		common := checkHelpSection(t, text, "initialization以外の終了:")
		requireCheckHelpText(t, common,
			"目的", "範囲", "受入条件", "阻害事項", "現在の集合SHA", "資材", "資材なし理由", "ADR要否",
			"必要", "adr", "宣言",
		)
	})
	t.Run("実測結果と文書の区別", func(t *testing.T) {
		evidence := checkHelpSection(t, text, "実測証拠:")
		requireCheckHelpText(t, evidence,
			"tdd", "integration", "現在回", "step_id", "stage", "runs",
			"unit_id", "command", "verification_sha256", "整数exit_code", "output_path", "非空", "実測",
			"必要command", "成功", "コード", "テストコード", "commit", "文書outputsへ登録しない",
			"Sensor", "独立レビュー", "RED/GREEN", "意味", "真正性",
		)
	})
}

func TestCheckHelpDocumentContracts(t *testing.T) {
	text := runCheckHelp(t, []string{"intent", "check", "--help"})
	t.Run("共通metadataと本文", func(t *testing.T) {
		common := checkHelpSection(t, text, "文書の共通条件:")
		requireCheckHelpText(t, common,
			"metadata", "本文とは別", "識別", "検索", "type", "用途", "title", "description", "非空", "本文",
			"正確なH2", "各節", "非空", "memory CLI", "日時", "生成",
			"Space", "knowledge root", "相対path",
		)
	})
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
			extra:    []string{"宣言"},
		},
		{
			name: "adr", path: "adr/",
			extra: []string{"宣言", "固定の必須H2はない", "新規出力", "現在intent_id", "必要"},
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
	versions := checkHelpSection(t, text, "文書の版と省略:")
	requireCheckHelpText(t, versions,
		"Requirements", "先行planning", "ImplementationPlan", "受入済みpath", "内容hash", "照合",
		"planning省略時", "受入済みImplementationPlanを一律要求しない",
		"CurrentAnalysis", "Architecture", "共有現在版", "任意入力",
		"architecture-analysis終了", "各1件", "intent_id", "日時", "形式だけのために更新しない",
	)
	outputs := checkHelpSection(t, text, "文書outputs:")
	requireCheckHelpText(t, outputs,
		"initialization", "tdd", "integration", "一律の既定文書出力はない", "必要な文書", "宣言", "空",
	)
}

func TestCheckHelpWorkflowAndProcedure(t *testing.T) {
	text := runCheckHelp(t, []string{"intent", "check", "--help"})
	t.Run("検査と完了", func(t *testing.T) {
		workflow := checkHelpSection(t, text, "検査と完了:")
		requireCheckHelpText(t, workflow,
			"読取り専用", "記録済みの証拠", "テストcommand自体は実行しない",
			"--boundary start|end", "省略end", "passだけでは段階は完了しない",
			"begin", "開始入力版", "再試行では差し替えない", "一般作業", "Unit claim",
			"提示後", "実回答", "人間", "成果承認",
			"plan-approval", "実行計画", "approval", "成果", "別",
		)
		order := "開始Sensor → begin → 終了Sensor → 独立レビュー → 成果承認 → finish"
		requireCheckHelpText(t, workflow, order)
	})
	t.Run("現在の具体的な入出力", func(t *testing.T) {
		procedure := checkHelpSection(t, text, "入出力の確認:")
		requireCheckHelpText(t, procedure,
			"aidlc intent procedure ID --space SPACE", "JSON", "現在step_id", "段階", "定義hash", "手順全文",
			"入力条件", "実path", "hash", "診断", "出力", "保存先", "metadata",
			"段階変更", "再開後", "取り直す", "Sensor内部の全必須H2一覧を返す操作ではない",
			"memory CLI", "本文", "intent documents", "宣言",
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

func TestCheckHelpOutputFailure(t *testing.T) {
	var stderr bytes.Buffer
	code := cli.Run(
		[]string{"intent", "check", "--help"},
		checkHelpFailingWriter{},
		&stderr,
		buildinfo.Info{},
		cli.Dependencies{},
	)
	if code != 1 || !strings.Contains(stderr.String(), "aidlc: write stdout: check help write failure") {
		t.Errorf("stdout失敗: exit=%d stderr=%q", code, &stderr)
	}
}

func TestCheckHelpPreservesBeginHelp(t *testing.T) {
	text := runCheckHelp(t, []string{"intent", "begin", "--help"})
	requireCheckHelpText(t, text,
		"aidlc intent begin ID --space SPACE --expect REVISION",
		"beginは開始入力版を保存し同段階の再試行では差し替えない",
		"一般作業/Unit claim前にbeginが必要",
	)
	if strings.Contains(text, "段階ごとの開始・終了条件") {
		t.Error("check専用の詳細説明がbegin helpにも追加された")
	}
}

type checkHelpFailingWriter struct{}

func (checkHelpFailingWriter) Write([]byte) (int, error) {
	return 0, errors.New("check help write failure")
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
