package cli

import "strings"

var publicActions = map[string]string{
	"install": "codex", "space": "create list switch", "intent": "create list switch show configure check review advance pause resume reopen wait cancel", "unit": "claim result integrate confirm", "memory": "create update show search rules check", "session": "bind inspect",
}

// Help recognizes only complete public help requests, without execution arguments.
func Help(args []string) (string, bool) {
	var target []string
	switch {
	case len(args) > 0 && args[0] == "help":
		target = args[1:]
	case len(args) > 0 && (args[len(args)-1] == "--help" || args[len(args)-1] == "help"):
		target = args[:len(args)-1]
	case len(args) == 3 && args[1] == "help":
		target = []string{args[0], args[2]}
	default:
		return "", false
	}
	if len(target) == 0 {
		return helpText, true
	}
	if len(target) > 2 {
		return "", false
	}
	actions, ok := publicActions[target[0]]
	if !ok {
		return "", false
	}
	if len(target) == 1 {
		return "aidlc " + target[0] + " — 公開操作: " + actions + "\n各操作は aidlc " + target[0] + " ACTION --help または aidlc help " + target[0] + " ACTION で確認する。\n", true
	}
	if !strings.Contains(" "+actions+" ", " "+target[1]+" ") {
		return "", false
	}
	key := target[0] + "/" + target[1]
	if key == "memory/create" || key == "memory/update" {
		return memoryWriteHelp(target[1]), true
	}
	usage := map[string]string{
		"install/codex": "--project-dir ROOT",
		"space/create":  "NAME [--project-dir ROOT]", "space/list": "[--json] [--project-dir ROOT]", "space/switch": "NAME [--project-dir ROOT]",
		"intent/create": "NAME --space SPACE", "intent/list": "--space SPACE", "intent/switch": "NAME|--id ID --space SPACE --session SESSION",
		"intent/show": "ID --space SPACE", "intent/check": "ID --space SPACE", "intent/configure": "ID --space SPACE --expect REVISION --file CONFIG.json",
		"intent/review": "ID --space SPACE --expect REVISION --file REVIEW.json", "intent/advance": "ID --space SPACE --expect REVISION",
		"intent/pause": "ID --space SPACE --expect REVISION --reason TEXT", "intent/resume": "ID --space SPACE --expect REVISION --reason TEXT", "intent/cancel": "ID --space SPACE --expect REVISION --reason TEXT",
		"intent/wait": "ID --space SPACE --expect REVISION --reason TEXT --resume-condition TEXT", "intent/reopen": "ID --space SPACE --expect REVISION --reason TEXT --stage STAGE",
		"unit/claim": "ID --space SPACE --expect REVISION --file REQUEST.json", "unit/result": "ID --space SPACE --expect REVISION --file REQUEST.json", "unit/integrate": "ID --space SPACE --expect REVISION --file REQUEST.json", "unit/confirm": "ID --space SPACE --expect REVISION --file REQUEST.json",
		"memory/show": "CONCEPT-ID --space SPACE", "memory/search": "[QUERY] --space SPACE [--intent-id ID]", "memory/rules": "--space SPACE", "memory/check": "--space SPACE",
		"session/bind": "ID --space SPACE --session SESSION [--recover]", "session/inspect": "--session SESSION",
	}
	text := "使い方: aidlc " + target[0] + " " + target[1] + " " + usage[key] + "\n--project-dir ROOT で対象プロジェクトを明示できる。\n"
	switch target[0] {
	case "intent":
		text += "Intentのstage: discovery → planning → tdd → integration。status: active / waiting / paused / completed / cancelled。状態は専用操作で変更する。\n--expect は現在stateの正のrevision。競合時はshowで再読込する。advanceは現在Sensorと独立reviewのpassが必要。resumeは待機/中断から、reopenは同じ段階または前の段階へ再開する。\n"
	case "unit":
		text += "Unitのstatus: pending / running / needs_confirmation / reported / integrated。JSONはclaimでunit/session/root、resultでunit/session/root/run_id/commit、integrateでunit/commit、confirmでunit/session/root/run_id/commitを指定する。resultとconfirmのcommitは現在のworker HEADと一致する40桁のcommitが必須。claimで割当、resultで成果commit、integrateで統合commit、confirmで既存runを確認する。statusを入力して進捗を偽装しない。\n"
	case "memory":
		text += "Concept IDは拡張子なし（例 knowledge/authentication、ADR/authentication、rules/project）。showは原文contentと現在hashを返す。searchはqueryのAND検索、intent-idは32桁の小文字16進数で完全一致。rulesは必須Rule全文、checkはSpaceのOKF検査。\n"
	case "session":
		text += "bindは現在stateと必須Rule全文を読む。--recoverは同じsession/Space/Intentで、失敗toolの終了を確認した後だけ使用する。実行中と推測して解除しない。inspectは読取りのみ。\n"
	}
	if key == "intent/review" {
		text += "REVIEW.json: actionはassign / accept。assignはcoordinator_session、別session、別rootを指定。acceptは同じsession/root/targetと、実報告のstatus pass / fail、summaryを指定する。古いtargetや未割当結果は拒否する。\n"
	}
	if key == "intent/configure" {
		text += "CONFIG.jsonはconfig全体。objective、scope配列、acceptance配列、unknowns配列、plan、code_revision、tests配列、direct_commit、adr、artifacts配列、units配列。adrはrequired/reason/refs。artifact kindはKnowledge / ADR / test、stageは上記4値。既存fieldとUnit進捗を保持する。\n"
	}
	return text, true
}

func memoryWriteHelp(action string) string {
	text := "Knowledge・ADR・Ruleの本文とmetadataを保存する。\n使い方: aidlc memory " + action + " CONCEPT-ID --space SPACE --body-file BODY.md --actor ACTOR"
	if action == "create" {
		text += " --type TYPE --title TITLE --description DESCRIPTION\n作成時はtype/title/descriptionの非空文字列が必須。"
	} else {
		text += " --expect HASH\n更新時はshowの現在hashが必須。省略metadataと未知拡張項目を保持する。"
	}
	return text + `
Concept IDは拡張子なし。例 knowledge/authentication、ADR/authentication、rules/project。
--body-file: 本文だけのUTF-8ファイル（256 KiB以内、文書全体も同上）。先頭frontmatterは禁止。本文途中の水平線は可。
本文ファイルは対象project内の通常file。symlink・project外pathは拒否する。--project-dir ROOTでprojectを明示できる。
--type: 自由な非空文字列。Design / ADR / Rule は例であり列挙型ではない。
--title / --description: 非空文字列。改行や引用符はCLIが安全なYAMLへ変換する。
--actor: 必須。generated.byへ保存。OKFの識別形式は producer/version、human:id、process:id（例 process:codex）。
generated.atは内容または明示metadataを変更した保存時の現在UTC日時。generatedを手入力しない。
--tag TEXT: 繰返し指定して配列にする。作成省略は空、更新省略は保持、指定時は全置換。
--clear-tags: 更新時だけtags消去。--tagと同時指定しない。
--status: draft / stable / deprecated。作成省略はstable、更新省略は保持。工程完了や検証合格ではない。
--intent-id: 32桁の小文字16進数。作成省略は未設定、更新省略は保持。共有Ruleへ一律付与しない。
--resource: 文字列。--stale-after: UTC offset付きISO 8601日時（例 2026-10-01T00:00:00Z）。省略は保持。
--sources-json: sources配列のJSON。各要素にresource文字列が必須。id/title/authorは任意文字列、usage_countは整数、last_modifiedは日時、usage_windowはfrom/to日時のobject。
  例 '[{"resource":"src/auth.go","last_modified":"2026-09-08T00:00:00Z"}]'
--verified-json: 検証済みのbyとatを持つobjectまたは配列のJSON。確認を実施した場合だけ指定する。
  例 '{"by":"human:alice","at":"2026-09-08T00:00:00Z"}'。未指定なら検証済みにしない。
--metadata-json: 未知拡張keyのobject JSON。標準key・generated・intent_idは禁止。指定keyだけ置換し、他のkeyは保持。
  例 '{"audience":"maintainers"}'。JSONの重複key・不正型・末尾dataは拒否する。
出典last_modified、verified.at、stale_afterは実際の意味に合う日時を指定し、自動で現在時刻へ置換しない。
更新は本文が同じでも明示metadata変更があれば可。本文とmetadataが全く同じならno-opとして拒否。
--expect HASHは更新時必須。競合時は保存せずshowで再読込。文書保存後のindex/log失敗は保存済みpath/hashと診断を返す。
旧--fileは使用不可。Intent/UnitのJSON入力用--fileとは別。helpには書込み引数を混ぜない。

例:
  aidlc memory create knowledge/authentication --space default --type Design --title '認証の仕様' --description '現行の方式' --tag authentication --status stable --actor process:codex --body-file body.md
  aidlc memory show knowledge/authentication --space default
  aidlc memory update knowledge/authentication --space default --body-file body.md --actor process:codex --expect HASH
`
}
