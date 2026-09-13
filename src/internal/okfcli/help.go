package okfcli

const helpText = `okf — Knowledge・ADR・Ruleの操作
使い方: okf ACTION [引数] --space SPACE [--project-dir ROOT]
操作: rules search show check create update
各操作: okf ACTION --help
版: okf --version
終了code: 0 成功、2 不正入力または競合、1 操作失敗。
`

func Help(args []string) (string, bool) {
	if len(args) == 0 || (len(args) == 1 && (args[0] == "help" || args[0] == "--help")) {
		return helpText, true
	}
	if len(args) != 2 {
		return "", false
	}
	action := ""
	if args[0] == "help" {
		action = args[1]
	} else if args[1] == "help" || args[1] == "--help" {
		action = args[0]
	} else {
		return "", false
	}
	if action == "create" || action == "update" {
		return memoryWriteHelp(action), true
	}
	usage, ok := map[string]string{"rules": "--space SPACE", "check": "--space SPACE", "search": "[QUERY] --space SPACE [--intent-id ID]", "show": "CONCEPT-ID --space SPACE"}[action]
	if !ok {
		return "", false
	}
	return "使い方: okf " + action + " " + usage + " [--project-dir ROOT]\nConcept IDは拡張子なし。例 codekb/authentication。showは原文contentと現在hashを返す。searchはqueryのAND検索、intent-idは32桁小文字16進数で完全一致。rulesは必須Rule全文、checkはSpaceのOKF検査。\n", true
}
func memoryWriteHelp(action string) string {
	text := "Knowledge・ADR・Ruleの本文とmetadataを保存する。\n使い方: okf " + action + " CONCEPT-ID --space SPACE --body-file BODY.md --actor ACTOR"
	if action == "create" {
		text += " --type TYPE --title TITLE --description DESCRIPTION\n作成時はtype/title/descriptionの非空文字列が必須。"
	} else {
		text += " --expect HASH\n更新時はshowの現在hashが必須。省略metadataと未知拡張項目を保持する。"
	}
	return text + `
Concept IDは拡張子なし。例 codekb/authentication、adr/authentication、rules/project。
--body-file: 本文だけのUTF-8ファイル（256 KiB以内、文書全体も同上）。先頭frontmatterは禁止。本文途中の水平線は可。
本文ファイルは対象project内の通常file。symlink・project外pathは拒否する。--project-dir ROOTでprojectを明示できる。
--type: 自由な非空文字列。Design / adr / Rule は例であり列挙型ではない。
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
  okf create codekb/authentication --space default --type Design --title '認証の仕様' --description '現行の方式' --tag authentication --status stable --actor process:codex --body-file body.md
  okf show codekb/authentication --space default
  okf update codekb/authentication --space default --body-file body.md --actor process:codex --expect HASH
`
}
