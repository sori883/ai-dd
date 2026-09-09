package cli

import "strings"

var publicActions = map[string]string{
	"install": "codex", "space": "create list switch", "intent": "create list switch show procedure configure check begin review advance pause resume reopen wait cancel", "unit": "claim result integrate confirm reassign", "memory": "create update show search rules check", "session": "bind inspect",
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
		"intent/procedure": "ID --space SPACE", "intent/show": "ID --space SPACE", "intent/check": "ID --space SPACE [--boundary start|end]", "intent/begin": "ID --space SPACE --expect REVISION", "intent/configure": "ID --space SPACE --expect REVISION --file CONFIG.json",
		"intent/review": "ID --space SPACE --expect REVISION --file REVIEW.json", "intent/advance": "ID --space SPACE --expect REVISION",
		"intent/pause": "ID --space SPACE --expect REVISION --reason TEXT", "intent/resume": "ID --space SPACE --expect REVISION --reason TEXT", "intent/cancel": "ID --space SPACE --expect REVISION --reason TEXT",
		"intent/wait": "ID --space SPACE --expect REVISION --reason TEXT --resume-condition TEXT", "intent/reopen": "ID --space SPACE --expect REVISION --reason TEXT --stage STAGE",
		"unit/reassign": "ID --space SPACE --expect REVISION --file REQUEST.json", "unit/claim": "ID --space SPACE --expect REVISION --file REQUEST.json", "unit/result": "ID --space SPACE --expect REVISION --file REQUEST.json", "unit/integrate": "ID --space SPACE --expect REVISION --file REQUEST.json", "unit/confirm": "ID --space SPACE --expect REVISION --file REQUEST.json",
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
	if key == "install/codex" {
		text += "移転: aidlc install codex --relocate --project-dir NEW_ROOT --from-project-dir OLD_ROOT --from-binary OLD_BINARY\n新binaryは実行中のaidlc。旧pathは絶対参照文字列で存在不要。移転先AI開始前に端末から実行する。既知Skillと製品hook参照だけを更新し、独自hookやWORKFLOW、Knowledgeを保持する。部分失敗はPathsが更新済み、Pendingが未処理。全件再検査する同じ引数の再試行で復旧する。未知編集は自動上書きしない。新ROOT/.codex/hooks.jsonの絶対pathを確認し、Codexのhook trustを利用者が確認する。trust/認証設定は変更しない。\n"
	}
	if key == "unit/reassign" {
		text += "active/tddのneeds_confirmationだけ再割当できる。旧worker終了を確認し、previous_run_stoppedをtrueにする。runningなら先にpause/resumeする。成功まで新workerを開始しない。\nREQUEST.json例: {\"unit\":\"a\",\"session\":\"new-worker\",\"root\":\"/new/worker\",\"commit\":\"<40桁の現在HEAD>\",\"reason\":\"旧処理終了と成果を確認\",\"previous_run_stopped\":true}\nunit/session/root/commit/reasonは文字列、停止確認は真偽値。run_idは指定せず新規発行する。HEAD・base履歴・依存統合・scope・他担当との衝突を検査する。state保存途中は当該Intentのconfigureや他Unit操作を拒否してrevisionを保持する。同じexpectとJSONで再試行し、異なる残存要求は現在割当を確認する。既にrunningならshowとassignmentで成功済みを確認する。新runで再テスト後result/integrateし、reviewは新root/sessionへassignして現在targetを受け直す。\n"
	}
	if key == "intent/review" {
		text += "REVIEW.json: actionはassign / accept。assignはcoordinator_session、別session、別rootを指定。acceptは同じsession/root/targetと、実報告のstatus pass / fail、summaryを指定する。古いtargetや未割当結果は拒否する。\n"
	}
	if key == "intent/reopen" {
		text += "graphの現在段階または祖先だけへ差し戻す。理由はUTC日時・元/先・要求revisionとwork-log.mdへ記録し、戻り先以降の合格を無効化する。途中保存時は同じexpect/from/to/reasonの要求だけretryできる。異なる操作は停止する。記録の改変・欠落は元版を復元する。成功後の古いexpectはrevision conflict。\n"
	}
	if key == "intent/procedure" {
		text += "現在段階・definition_hash・procedureのpath/frontmatter/本文全文・advance・reopen候補をJSONで返す読取り専用操作。IDは32桁小文字16進数、SPACEはSpace名。段階変更や再開後に取り直す。任意path/stageは指定不可。定義変更時は元版を復元するか新Intentを作る。\n"
	}
	if key == "intent/check" || key == "intent/begin" {
		text += "checkは読取り専用。--boundary start|end（省略end）。beginは開始入力版を保存し同段階の再試行では差し替えない。一般作業/Unit claim前にbeginが必要。文書不足はintent procedureの必須型/節に従いmemory CLIで修復する。\n"
	}
	if key == "intent/configure" {
		text += "新規stateはschema_version=3。旧schemaは保持して明示エラー。material_sources/feature_knowledge/test_resultsはrepository相対pathの文字列配列、no_materials_reasonは文字列。material_sourcesは明示UTF-8ファイル/ディレクトリ。空ならdiscovery終了までにno_materials_reasonが必要。変更時はdiscoveryのbeginを無効化し、後段ではdiscoveryへのreopenが必要。entry/acceptedはCLI所有でconfigure不可。feature_knowledgeは統合終了で最低1件、共有解析/図とは別のknowledge/<機能名>.md。test_resultsはstrict JSONのstage(tdd|integration)/runs配列。現在存在する結果だけを指定し、次段階の結果は作成後に追記する。TDDはUnitごとのcommandとResultCommitの成功、integrationは全Unit統合後の現在HEADで計画command全ての成功を要求する。各runはcommand、実成果commit、必須整数exit_code、非空出力ファイルoutput_path。intent procedureの本文節と作成例に従う。\n"
		text += "CONFIG.jsonはconfig全体。objective、scope配列、acceptance配列、unknowns配列、plan、code_revision、tests配列、direct_commit、adr、artifacts配列、units配列。adrはrequired/reason/refs。artifact kindはKnowledge / ADR / test、stageは上記4値。既存fieldとUnit進捗を保持する。\n"
		text += "\n型: objective/plan/code_revision/direct_commitは文字列。scope/acceptance/unknowns/testsは文字列配列。adrはobjectでrequiredは真偽値、reasonは文字列、refsは文字列配列。artifacts/unitsはobject配列。artifactのpath/kind/stageは文字列。Unitのid/bolt/base_commit/status/result_commit/integrated_commitは文字列、depends_on/scope/testsは文字列配列。\n例中の<CURRENT_HEAD>を対象Gitの現在HEAD（40桁commit）へ置換する。Knowledgeのpath/本文、scope、受入条件、テストcommandは実projectに合わせて用意・置換する。例のコピーだけではSensor合格や検証済みを意味しない。\nUnitなし（直接実装）:\n```json\n{\n  \"no_materials_reason\": \"新規fixture。既存資材があればmaterial_sourcesを指定する\",\n  \"objective\": \"加算を提供する\",\n  \"scope\": [\n    \"add.go\"\n  ],\n  \"acceptance\": [\n    \"Add(2,3)は5を返す\"\n  ],\n  \"unknowns\": [],\n  \"plan\": \"失敗するテストを書き、最小実装と検証を行う\",\n  \"code_revision\": \"<CURRENT_HEAD>\",\n  \"tests\": [\n    \"go test -run TestAdd\"\n  ],\n  \"direct_commit\": \"\",\n  \"adr\": {\n    \"required\": false,\n    \"reason\": \"既存方式に沿うため追加判断なし\",\n    \"refs\": []\n  },\n  \"artifacts\": [\n    {\n      \"path\": \"aidlc/spaces/default/knowledge/knowledge/current.md\",\n      \"kind\": \"Knowledge\",\n      \"stage\": \"discovery\"\n    }\n  ],\n  \"units\": []\n}\n```\nUnitあり（新規Unitの初期値）:\n```json\n{\n  \"no_materials_reason\": \"新規fixture。既存資材があればmaterial_sourcesを指定する\",\n  \"objective\": \"加算を提供する\",\n  \"scope\": [\n    \"add.go\"\n  ],\n  \"acceptance\": [\n    \"Add(2,3)は5を返す\"\n  ],\n  \"unknowns\": [],\n  \"plan\": \"失敗するテストを書き、最小実装と検証を行う\",\n  \"code_revision\": \"<CURRENT_HEAD>\",\n  \"tests\": [\n    \"go test -run TestAdd\"\n  ],\n  \"direct_commit\": \"\",\n  \"adr\": {\n    \"required\": false,\n    \"reason\": \"既存方式に沿うため追加判断なし\",\n    \"refs\": []\n  },\n  \"artifacts\": [\n    {\n      \"path\": \"aidlc/spaces/default/knowledge/knowledge/current.md\",\n      \"kind\": \"Knowledge\",\n      \"stage\": \"discovery\"\n    }\n  ],\n  \"units\": [\n    {\n      \"id\": \"addition\",\n      \"bolt\": \"bolt-1\",\n      \"base_commit\": \"<CURRENT_HEAD>\",\n      \"depends_on\": [],\n      \"scope\": [\n        \"add.go\"\n      ],\n      \"tests\": [\n        \"go test -run TestAdd\"\n      ],\n      \"status\": \"pending\",\n      \"result_commit\": \"\",\n      \"integrated_commit\": \"\"\n    }\n  ]\n}\n```\n新規Unitはpending、result_commit/integrated_commitは空文字列。Unitのidは英数字から始まる英数字・_・-の1〜80文字。既存configの更新はintent showを読み、既存fieldとUnit進捗を保持する。実行中Unitを例のpendingや空commitへ戻してはならない。direct_commitは直接実装の成果commitを検証後に指定する。\n"
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
