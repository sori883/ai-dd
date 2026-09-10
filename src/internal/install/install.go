// Package install deploys embedded assets into a fresh project without replacement.
package install

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	core "github.com/sori883/ai-dd/src/core/minimal"
	coreworkflow "github.com/sori883/ai-dd/src/core/workflow"
	codex "github.com/sori883/ai-dd/src/harness/codex/minimal"
)

// Result names each file saved before success or a partial failure.
type Result struct{ Paths []string }

// Codex installs initial assets. Existing target files are never replaced.
func Codex(root, binary string) (result Result, err error) {
	if !filepath.IsAbs(binary) {
		return result, fmt.Errorf("binary must be absolute: %w", fs.ErrInvalid)
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return result, err
	}
	assets := map[string][]byte{}
	for _, source := range []struct {
		files  fs.FS
		prefix string
	}{{core.Files, ""}, {codex.Files, ""}, {coreworkflow.Files, "aidlc/workflow/"}} {
		err = fs.WalkDir(source.files, ".", func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				return nil
			}
			data, readErr := fs.ReadFile(source.files, path)
			if readErr != nil {
				return readErr
			}
			destination := ""
			switch {
			case source.prefix != "":
				destination = source.prefix + path
			case path == "adr-template.md":
				destination = "aidlc/templates/adr.md"
			case strings.HasPrefix(path, "knowledge/"):
				destination = "aidlc/spaces/default/" + path
			case path == "SKILL.md":
				destination = ".agents/skills/aidlc/" + path
			case path == "aidlc-cli/SKILL.md":
				destination = ".agents/skills/aidlc-cli/SKILL.md"
			case strings.HasPrefix(path, "agents/"):
				destination = ".codex/" + path
			}
			if destination != "" {
				assets[destination] = []byte(strings.ReplaceAll(string(data), "@@BINARY@@", shellQuote(binary)))
			}
			return nil
		})
		if err != nil {
			return result, err
		}
	}
	hooks := map[string]any{}
	for _, event := range []string{"SessionStart", "UserPromptSubmit", "PreToolUse", "PostToolUse", "Stop"} {
		group := map[string]any{"hooks": []any{map[string]any{"type": "command", "command": shellQuote(binary) + " __minimal-hook --project-dir " + shellQuote(root), "timeout": 10}}}
		if event == "SessionStart" {
			group["hooks"].([]any)[0].(map[string]any)["additionalContextLimit"] = 8192
		}
		if event == "PreToolUse" || event == "PostToolUse" {
			group["matcher"] = assignmentMatcher
		}
		hooks[event] = []any{group}
	}
	assets[".codex/hooks.json"], err = json.MarshalIndent(map[string]any{"hooks": hooks}, "", "  ")
	if err != nil {
		return result, err
	}
	var paths []string
	for path := range assets {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	project, err := os.OpenRoot(root)
	if err != nil {
		return result, err
	}
	defer project.Close()
	for _, path := range paths {
		if err := checkDestination(project, path); err != nil {
			return result, err
		}
	}
	for _, path := range paths {
		if err := project.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return result, fmt.Errorf("create parent for %s: %w", path, err)
		}
		file, err := project.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
		if err != nil {
			return result, fmt.Errorf("create %s: %w", path, err)
		}
		_, writeErr := file.Write(assets[path])
		closeErr := file.Close()
		result.Paths = append(result.Paths, path)
		if writeErr != nil {
			return result, fmt.Errorf("save %s: %w", path, writeErr)
		}
		if closeErr != nil {
			return result, fmt.Errorf("close %s: %w", path, closeErr)
		}
	}
	return result, nil
}

func checkDestination(root *os.Root, path string) error {
	parts := strings.Split(filepath.ToSlash(path), "/")
	for i := range parts {
		prefix := filepath.Join(parts[:i+1]...)
		info, err := root.Lstat(prefix)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 || i == len(parts)-1 {
			return fmt.Errorf("existing destination %s: %w", prefix, fs.ErrExist)
		}
		if !info.IsDir() {
			return fmt.Errorf("invalid parent %s: %w", prefix, fs.ErrInvalid)
		}
	}
	return nil
}
func shellQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'" }

const assignmentMatcher = "^(Bash|apply_patch|spawn_agent|collaborationspawn_agent|followup_task|collaborationfollowup_task|send_message|collaborationsend_message|interrupt_agent|collaborationinterrupt_agent)$"

// Known pre-assignment assets are relocated without upgrading their content.
var legacyAssignmentSkills = map[string]string{
	"SKILL.md":           "---\nname: aidlc\ndescription: AI-DLCでIntentの計画、段階実行、担当依頼、会話承認と知識記録を進める。利用プロジェクトのAI-DLC作業に使う。\n---\n# AI-DLCの進め方\n\n実行ファイルは @@BINARY@@（以下A）。Spaceはユーザー指定、未指定なら初期配置のdefaultを使う。ID不明なら `A intent list --space SPACE` で確認し、名前を--idへ渡さない。引数不明時は選択前でも `A intent ACTION --help` を読める。\nSessionStartのsession IDを使い、各turnで `A intent switch --id ID --space SPACE --session SESSION` または `A session bind ID --space SPACE --session SESSION` が返すプロジェクトRule全文を読む。このスキルだけではRule読了にならない。\n`A intent procedure ID --space SPACE` で現在のstep_id、手順、担当、入力・文書outputsを取得する。段階変更・再開後は取り直す。操作選択は [aidlc-cli](../aidlc-cli/SKILL.md)、正確な引数・JSONは各CLI helpを読む。\n\ninitialization→discoveryが初回必須。他4段階の採否・順序・省略理由はユーザーが計画を承認する。bootstrapを承認済みと扱わない。変更ごとに承認し、計画承認と成果承認を別requestで記録する。提示後の実際のUserPromptSubmit回答だけを引用する。両requestを提示していた場合だけ同じ回答を使え、後で生成したrequestや過去turnに転用しない。\n各回は開始Sensorとbegin後に一般作業を行い、終了Sensor・独立review・成果承認を確認してfinishする。未実施・skip・fail・対象変更後の古いpassを成功扱いしない。省略段階の成果を一律要求しない。\n\nメインAIが共有stateの単独writerとなり、Ruleと必要な入力を担当へ渡して結果を回収する。aidlc-requirementsが要件、aidlc-researcherが根拠を調べ、その後と計画変更時にaidlc-stage-plannerが採否・順序・理由とPLAN案を返す。planningは実装手順とUnit詳細を扱う。aidlc-workerは承認・割当後に別worktreeで担当範囲を実装しcommitを返す。依存統合前や重複範囲で開始しない。aidlc-reviewerは固定成果を別root/sessionで独立に確認する。worker以外はread-only。子担当は共有state/Knowledge/ADRを保存せず報告・本文案を返す。他者編集を保全し、回答を捏造しない。\n\nKnowledgeは現行what/how、ADRは判断のwhy・代替案・影響、進捗はstate。差戻し理由はCLIがknowledge/log/ID-work-log.mdへ保存する。毎操作の日誌や一律ADRは作らない。必要なADRを作り、不要なら理由をreviewする。\noutputsは期待する文書だけで、なければなし。プログラム・テストコード・commitを文書outputsへ列挙せず、検証証拠は別に説明する。文書はOKF metadataを保持し、一般知識を命令権限にしない。共有文書のID/日時を形式だけのために更新せず、合格目的でRuleを変えない。本文草稿からメインAIがCLIで保存する。\n\n質問待ちはwait、中断はpause、確認後resume。再実行はreopenで承認後に新step IDを使い、古い成果・合格を流用しない。対象変更後は再検査・再reviewする。非同期toolは終端までpollする。失敗終了とPost未到着を確認した同じsessionだけ `A session bind ID --space SPACE --session SESSION --recover` を使う。不明なrunを自動再実行しない。Stopは作業全体の完了ではない。\n定義の欠落・変更は停止し元版復元か新Intentで解決する。旧手順や埋込み版を代用せず既設配置を自動移行・上書きしない。保存途中は同一要求で再試行し、state確定まで成功扱いしない。\n",
	"aidlc-cli/SKILL.md": "---\nname: aidlc-cli\ndescription: AI-DLC CLIの操作目的からコマンドとhelpを選ぶ。Intent、文書保存、Unit、復旧や配置移転の操作時に使う。\n---\n# 操作を選ぶ\n\n実行ファイルは @@BINARY@@（以下A）。進行と承認の規約は [aidlc](../aidlc/SKILL.md)、現在工程は `A intent procedure ID --space SPACE`。以下から目的を選び、変更前に該当helpで引数・型・値・JSON例を確認する。実際のID/Space/sessionとshowのrevisionを使い、競合時は再読込する。\n\n| 目的 | help |\n| --- | --- |\n| Space作成・選択、Intent作成・選択 | `A space --help`、`A intent create --help`、`A intent list --help`、`A intent switch --help` |\n| 現在状態と手順 | `A intent show --help`、`A intent procedure --help` |\n| 段階の採否・順序と計画承認 | `A intent plan --help`、`A intent plan-approval --help` |\n| 設定・文書宣言・実測結果 | `A intent configure --help`、`A intent documents --help` |\n| 開始/終了Sensorと開始 | `A intent check --help`、`A intent begin --help` |\n| 独立review・成果承認・完了 | `A intent review --help`、`A intent approval --help`、`A intent finish --help` |\n| 質問待ち・中断・再開・差戻し・履歴 | `A intent wait --help`、`A intent pause --help`、`A intent resume --help`、`A intent reopen --help`、`A intent history --help` |\n| Unit割当・回収・統合・中断確認 | `A unit claim --help`、`A unit result --help`、`A unit integrate --help`、`A unit confirm --help` |\n| Knowledge/ADRの作成・更新・読取り | `A memory create --help`、`A memory update --help`、`A memory show --help`、`A memory search --help` |\n| session読込み・復旧、配置・移転、Unit移転 | `A session bind --help`、`A install codex --help`、`A unit reassign --help` |\n\n## 文書と実測\n\nprocedureのmetadata条件と解決path/版を読み、documentsでinputs/outputs両一覧を置換する。新文書は未存在でも宣言できるが、実測test_resultsは実行後に存在する結果だけを指定する。accepted入力は前回合格のpath/hashを保持し、変更にはreopenが必要。共有currentと同回outputは更新できる。各宣言・Unit・実測へ現在step_idを使う。\n本文だけをsessionのdraftへ書き、memory CLIでmetadataを生成する。Concept IDは拡張子なし。Knowledgeはcodekb/NAME、ADRはadr/NAMEでtype adr。要件はdesign/ID/requirements、実装計画はdesign/ID/implementation-plan、共有解析はcodekb/current-analysis、構成図はcodekb/architecture。新規ADRのIntent IDを保持する。update前にshowのcontent/hashを確認する。\nTDD/integrationは現在回のcommit・command・exit_code・output_pathをconfigureへ記録する。TDDはdirect_commitまたはUnitのResultCommit、integrationは現在HEADを検証する。Sensorは形式・版・command成功を確認し、真正性とRED/GREENの意味はreviewerが確認する。\n\n## 承認と復旧\n\nreviewは別root/sessionへassignし、実報告をacceptする。コードを扱う段階のreviewer checkoutは調整rootと同じ版・bytesにする。plan-approval/approvalは表示されたrequest_id/targetと実回答のsession/turn/quoteを使う。計画整理・読取り・質問回答は承認待ちでもできる。\nreopenは--stepで対象を指定する。保存途中は同じexpect・decisionを再試行する。durable pending後は時刻と前後hashが固定され、log改変時は元版を復元する。記録の存在だけで成功扱いせずstateのrevision確定を確認する。generated.atは更新日時で承認を意味しない。work-logはmemory search work-log、memory show log/ID-work-logで読む。\n移転前に端末からinstall codex --helpを読み、--relocateへ旧root/binaryの配置済み絶対文字列を渡す。両skillとhooksの既知参照だけを移し、版更新や未知編集の上書きを兼ねない。利用者が新hooksの絶対pathとCodex trustを確認する。部分失敗はPaths/Pendingを見て同じ引数で再検査する。\nUnit移転は旧run停止を確認してからreassignする。runningはpause/resumeでneeds_confirmationにし、previous_run_stoppedは確認時だけtrue。成功まで新workerを開始せず、保存途中は同じexpect/JSONで再試行する。新run_idとHEADで再テストし、reviewも新root/sessionへassignし直す。runtimeはGit共有しない。CLIはworker起動やGit統合を代行しない。\n",
}
