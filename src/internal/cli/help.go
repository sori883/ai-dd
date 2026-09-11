package cli

import "strings"

var publicActions = map[string]string{
	"assignment": "init list show reserve check release reset",
	"install":    "codex", "space": "create list switch", "intent": "create list switch show hash procedure documents configure check begin review plan plan-approval approval finish history advance pause resume reopen wait cancel", "unit": "claim result integrate confirm reassign", "memory": "create update show search rules check", "session": "bind inspect",
}

// Help recognizes only complete public help requests, without execution arguments.
func Help(args []string) (string, bool) {
	var target []string
	switch {
	case len(args) == 0:
		target = nil
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
		return helpText + "\n  assignment init/list/show/reserve/check/release/reset — native worker作業場所の登録と明示解放（各ACTION --help）\n", true
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
	if target[0] == "assignment" {
		return assignmentHelp(target[1]), true
	}
	if key == "memory/create" || key == "memory/update" {
		return memoryWriteHelp(target[1]), true
	}
	usage := map[string]string{
		"install/codex": "[--project-dir ROOT]",
		"space/create":  "NAME [--project-dir ROOT]", "space/list": "[--json] [--project-dir ROOT]", "space/switch": "NAME [--project-dir ROOT]",
		"intent/create": "NAME --space SPACE", "intent/list": "--space SPACE", "intent/switch": "NAME|--id ID --space SPACE --session SESSION",
		"intent/hash":          "ID --space SPACE [--unit UNIT --root ROOT]",
		"intent/plan":          "ID --space SPACE [--expect REVISION --file PLAN.json]",
		"intent/plan-approval": "ID --space SPACE --expect REVISION --file DECISION.json",
		"intent/approval":      "ID --space SPACE --expect REVISION --file DECISION.json",
		"intent/finish":        "ID --space SPACE --expect REVISION",
		"intent/history":       "ID --space SPACE",
		"intent/documents":     "ID --space SPACE [--expect REVISION --file DOCUMENTS.json]",
		"intent/procedure":     "ID --space SPACE", "intent/show": "ID --space SPACE", "intent/check": "ID --space SPACE [--boundary start|end]", "intent/begin": "ID --space SPACE --expect REVISION", "intent/configure": "ID --space SPACE --expect REVISION --file CONFIG.json",
		"intent/review": "ID --space SPACE --expect REVISION --file REVIEW.json", "intent/advance": "ID --space SPACE --expect REVISION",
		"intent/pause": "ID --space SPACE --expect REVISION --reason TEXT", "intent/resume": "ID --space SPACE --expect REVISION --reason TEXT", "intent/cancel": "ID --space SPACE --expect REVISION --reason TEXT",
		"intent/wait": "ID --space SPACE --expect REVISION --reason TEXT --resume-condition TEXT", "intent/reopen": "ID --space SPACE --expect REVISION --reason TEXT --step STEP_ID",
		"unit/reassign": "ID --space SPACE --expect REVISION --file REQUEST.json", "unit/claim": "ID --space SPACE --expect REVISION --file REQUEST.json", "unit/result": "ID --space SPACE --expect REVISION --file REQUEST.json", "unit/integrate": "ID --space SPACE --expect REVISION --file REQUEST.json", "unit/confirm": "ID --space SPACE --expect REVISION --file REQUEST.json",
		"memory/show": "CONCEPT-ID --space SPACE", "memory/search": "[QUERY] --space SPACE [--intent-id ID]", "memory/rules": "--space SPACE", "memory/check": "--space SPACE",
		"session/bind": "ID --space SPACE --session SESSION [--recover]", "session/inspect": "--session SESSION",
	}
	text := "使い方: aidlc " + target[0] + " " + target[1] + " " + usage[key] + "\n--project-dir ROOT で対象プロジェクトを明示できる。\n"
	switch target[0] {
	case "intent":
		text += "Intentのstage: initialization → discoveryが必須。その後はarchitecture-analysis / planning / tdd / integrationの採否と順序を計画する。status: active / waiting / paused / completed / cancelled。状態は専用操作で変更する。\n--expect は現在stateの正のrevision。競合時はshowで再読込する。finishはSensor・独立review・人間の成果承認が必要。advanceはfinishへの案内付きエラー。resumeは待機/中断から、reopenは指定実行回の再実行計画を提示する。\n"
	case "unit":
		text += "claim/reassignはregistry_epoch、request_id、coordinator_sessionを必須指定する。assignment initで管理を開始し、assignment list/showでtask_nameを取得してメインAIがnative spawnする。reported後も明示releaseまで保持する。\n"
		text += "Unitのstatus: pending / running / needs_confirmation / reported / integrated。現在step_idを指定する。claimはunit/session/root、resultとconfirmはunit/session/root/run_id/verification_sha256、integrateはunitを指定する。resultは登録runの現在SHAとunit結果JSONを照合し、integrateは管理rootの内容がresult_sha256と同じと確認する。reportedは有効な同runで明示再提出できる。後続変更だけで過去integratedを取り消さず、最新全体SHAで最終検証する。\n"
	case "memory":
		text += "Concept IDは拡張子なし（例 codekb/authentication、adr/authentication、rules/project）。showは原文contentと現在hashを返す。searchはqueryのAND検索、intent-idは32桁の小文字16進数で完全一致。rulesは必須Rule全文、checkはSpaceのOKF検査。\n"
	case "session":
		text += "bindは現在stateと必須Rule全文を読む。--recoverは同じsession/Space/Intentで、失敗toolの終了を確認した後だけ使用する。実行中と推測して解除しない。inspectは読取りのみ。\n"
	}
	if key == "install/codex" {
		text += "移転: aidlc install codex --relocate --project-dir NEW_ROOT --from-project-dir OLD_ROOT --from-binary OLD_BINARY\n新binaryは実行中のaidlc。旧pathは絶対参照文字列で存在不要。移転先AI開始前に端末から実行する。既知のaidlc/aidlc-cli両skillと製品hooksの3ファイルを事前検査し参照だけを更新する。独自hookや旧WORKFLOW、Knowledgeを保持し、旧版のupgradeを兼ねない。部分失敗はPathsが更新済み、Pendingが未処理。全件再検査する同じ引数の再試行で復旧する。未知編集は自動上書きしない。新ROOT/.codex/hooks.jsonの絶対pathを確認し、Codexのhook trustを利用者が確認する。trust/認証設定は変更しない。\n"
	}
	if key == "unit/claim" {
		text += `REQUEST.json例: {"registry_epoch":"initのepoch","request_id":"一意の要求ID","coordinator_session":"メイン会話ID","step_id":"現在step_id","unit":"a","session":"worker-a","root":"/project"}
`
	}
	if key == "unit/reassign" {
		text += "active/tddのneeds_confirmationだけ再割当できる。旧worker終了を確認し、previous_run_stoppedをtrueにする。REQUEST.jsonはregistry_epoch/request_id/coordinator_session/step_id/unit/session/root/reasonとprevious_run_stoppedを指定し、run_idを指定しない。依存、scope、担当競合を検査する。保存失敗は同じexpect/JSONで再試行し、成功まで新workerを開始しない。\n"
	}
	if key == "intent/review" {
		text += "REVIEW.json: actionはassign / accept。assignはcoordinator_session、別session、rootを指定。同じrootを使える。別rootではIntentの検証対象集合SHAの一致を確認する。acceptは同じsession/root/targetと、実報告のstatus pass / fail、summaryを指定する。古いtargetや未割当結果は拒否する。\n"
	}
	if strings.HasPrefix(key, "intent/") {
		text += "実行回step_idはCLIが採番する。計画承認と各実行の成果承認を分け、finishはSensor・独立review・成果承認を確認する。\n"
	}
	if key == "intent/plan" {
		text += "PLANはreason(string)、steps(array:既存{id,stage}または新規{stage})、omitted(array:{stage,reason})。任意段階の採否と省略理由を毎回承認する。例: {\"reason\":\"調査のみ\",\"steps\":[{\"id\":\"s01\",\"stage\":\"initialization\"},{\"id\":\"s02\",\"stage\":\"discovery\"}],\"omitted\":[{\"stage\":\"architecture-analysis\",\"reason\":\"不要\"},{\"stage\":\"planning\",\"reason\":\"実装なし\"},{\"stage\":\"tdd\",\"reason\":\"実装なし\"},{\"stage\":\"integration\",\"reason\":\"実装なし\"}]}\n"
	}
	if key == "intent/approval" || key == "intent/plan-approval" {
		text += "DECISIONはrequest_id,target,decision(approve/reject),session,turn,quoteの文字列。提示後の実際のUserPromptSubmit回答から引用する。例: {\"request_id\":\"表示されたID\",\"target\":\"表示されたhash\",\"decision\":\"approve\",\"session\":\"session ID\",\"turn\":\"turn ID\",\"quote\":\"承認します\"}。AI自身が回答を作らない。\n"
	}
	if key == "intent/reopen" {
		text += "既知step_idを指定して再実行の変更案を作り、plan-approvalで承認して適用する。完了実績を保持し、未完了の現在回も新IDへ置換する。理由はUTC日時・元/先・要求revisionとaidlc/spaces/<space>/knowledge/log/<intent_id>-work-log.mdへOKF（type: work-log）で記録し、新回へ過去合格を流用しない。承認途中の保存失敗は同じexpectと承認JSONだけretryできる。異なる操作は停止する。記録の改変・欠落は元版を復元する。成功後の古いexpectはrevision conflict。stateのrevisionが要求revisionを超えて確定するまで、記録の存在だけで成功扱いしない。generated.atはCLIが生成する内容更新時のUTC日時であり、承認や検証済みを意味しない。\n検索: aidlc memory search work-log --space SPACE --intent-id ID\n本文: aidlc memory show log/ID-work-log --space SPACE\n"
	}
	if key == "intent/documents" {
		text += "DOCUMENTS.json例: {\"inputs\":[],\"outputs\":[{\"step_id\":\"s05\",\"stage\":\"integration\",\"path\":\"aidlc/spaces/default/knowledge/codekb/feature.md\",\"metadata\":{\"type\":\"Knowledge\",\"title\":\"Feature\",\"description\":\"Current behavior\"}}]}\nstatusはdraft/stable/deprecated、tagsは文字列配列、intent_idは32桁小文字16進数。Requirements/ImplementationPlanと新規adrの登録結果のintent_idをmemory create --intent-idへ渡す。generated日時はmemory CLIが生成する。\n"

		text += "inputs/outputs配列を一括置換。各要素はstage、Space内Markdownのpath、metadata(type/title/description必須、status/tags/intent_id任意)。outputsは未存在pathも宣言できる。test_resultsは実行後に存在する結果だけをconfigureへ登録する。受入済み段階の変更はreopenが必要。metadataは完全一致、tagsは順序なし集合。新規adrはknowledge/adr/へ作り現在Intent IDを保持する。\n"
	}
	if key == "intent/procedure" {
		text += "現在step_id・計画revision・現在段階・definition_hash・procedureのpath/frontmatter/本文全文・解決inputs・具体outputs・診断・次回ID・reopen候補をJSONで返す読取り専用操作。IDは32桁小文字16進数、SPACEはSpace名。段階変更や再開後に取り直す。任意path/stageは指定不可。配布Ruleは固定path rules/rule.mdとmetadata type Ruleで特定する。titleは利用者が変更でき、title/description/本文の検証は維持する。他のRuleや入口を削除しない。定義変更時は元版を復元するか新Intentを作る。\n"
	}
	if key == "intent/begin" {
		text += "checkは読取り専用。--boundary start|end（省略end）。beginは開始入力版を保存し同段階の再試行では差し替えない。一般作業/Unit claim前にbeginが必要。文書不足はintent procedureの必須型/節に従いmemory CLIで修復する。\n"
	}
	if key == "intent/check" {
		text += checkHelp()
	}
	if key == "intent/configure" {
		text += "新規stateはschema_version=6、担当registryはschema_version=2。CONFIG.jsonはconfig全体を指定する。objective/plan/no_materials_reasonは文字列、scope/verification_paths/acceptance/unknowns/tests/material_sources/test_resultsは文字列配列。scopeは編集の担当範囲、verification_pathsは検証に影響するソース・テスト・設定・共通部品のproject相対パス集合。TDD/integrationは非空のverification_pathsが必須。Unitの実効範囲はIntentの範囲へ含める。Unitはstep_id/id/bolt/status/result_sha256、depends_on/scope/tests、任意verification_pathsを持つ。新Unitはpendingでresult_sha256を空にし、既存進捗はUnit操作だけで変更する。adrはrequired/reason、artifactsはpath/kind/stage。文書宣言はintent documents専用。資材を使わないdiscoveryにはno_materials_reasonを記す。\n結果はaidlc/evidence等の通常ファイルへ保存する。結果JSONはstep_id、stage、verification_scope(intent|unit)、verification_sha256、runs。runはunit_id（直接実装では省略）、command、整数exit_code、非空出力のoutput_path。unit結果にはトップレベルのunit_id/run_idも必要で、各runのUnitと一致させる。現在全体SHAで各unit_id+commandの成功が終了条件。hash→テスト→hashが一致した結果だけを登録する。\nadr.requiredは真偽値。result_sha256は64桁のSHAで初期値は空。<CURRENT_STEP>は現在のstep_idで置換する。\nUnitなし\n```json\n{\"objective\":\"加算を提供する\",\"scope\":[\"src\"],\"verification_paths\":[\"src\",\"go.mod\",\"go.sum\"],\"acceptance\":[\"加算が正しい\"],\"unknowns\":[],\"plan\":\"テスト先行で確認する\",\"tests\":[\"go test ./src/...\"],\"no_materials_reason\":\"新規開発\",\"adr\":{\"required\":false,\"reason\":\"既存方式\"},\"artifacts\":[],\"units\":[]}\n```\nUnitあり\n```json\n{\"objective\":\"加算を提供する\",\"scope\":[\"src\"],\"verification_paths\":[\"src\",\"go.mod\",\"go.sum\"],\"acceptance\":[\"加算が正しい\"],\"unknowns\":[],\"plan\":\"テスト先行で確認する\",\"tests\":[\"go test ./src/...\"],\"no_materials_reason\":\"新規開発\",\"adr\":{\"required\":false,\"reason\":\"既存方式\"},\"artifacts\":[],\"units\":[{\"id\":\"add\",\"step_id\":\"<CURRENT_STEP>\",\"bolt\":\"one\",\"depends_on\":[],\"scope\":[\"src\"],\"verification_paths\":[\"src\",\"go.mod\",\"go.sum\"],\"tests\":[\"go test ./src/...\"],\"status\":\"pending\",\"result_sha256\":\"\"}]}\n```\n"
	}
	if key == "intent/hash" {
		text += "verification_sha256とversion、intent_id、step_id、任意unit_id、verification_paths、files、bytes、missing_pathsを返す読取り専用操作。--rootは登録済みUnit rootだけを使える。root直下aidlcと全深さ.gitを除外し、symlink/特殊file/範囲外参照を拒否。上限10,000ファイル・256 MiB、二回の読込一致を確認する。mtimeや絶対rootはSHAを変えない。承認待ち中も読取りできる。\n"
	}
	return text, true
}

func checkHelp() string {
	return `
Intentは一つの案件、Sensorは段階の開始・終了条件を調べる検査。

検査と完了:
  checkは読取り専用で、記録済みの証拠を検査する。テストcommand自体は実行しない。
  --boundary start|end（省略end）。passだけでは段階は完了しない。
  開始Sensor → begin → 終了Sensor → 独立レビュー → 成果承認 → finish の順に進める。
  beginは開始入力版を保存し、同段階の再試行では差し替えない。一般作業やUnit claim前に必要。
  成果承認は、成果と独立レビューの提示後に受けた人間の実回答を使う。
  plan-approvalは実行計画、approvalは現在回の成果の承認。別の要求として扱い、過去の回答を流用しない。

入出力の確認:
  aidlc intent procedure ID --space SPACE
  JSONで現在step_id・段階・定義hash・手順全文、入力条件・実path・hash・診断、出力の保存先・metadataを確認する。
  段階変更や再開後は取り直す。procedureはSensor内部の全必須H2一覧を返す操作ではない。
  文書不足は以下の文書条件と現在のprocedureを照合し、memory CLIで本文・metadataを保存して修復する。
  intent documentsによる入出力の宣言と、memory CLIによる本文保存は別の操作。

段階ごとの開始・終了条件

全段階:
  開始: 固定workflow、Rule、宣言入力と資材を確認する。
  終了: beginで保存した開始入力との整合と現在の入力、現在のstep_idに結び付けた宣言出力を確認する。

initialization:
  開始: Rule、固定workflow、配置2skill（aidlc・aidlc-cli）、hooks設定の有効なJSONを確認する。
  終了: 開始入力との整合と宣言出力を確認する。

discovery:
  開始: Rule、共有分析文書があれば現在版を任意入力として確認する。
  終了: 目的・範囲・受入条件、阻害事項なし、現在の集合SHA、資材または資材なし理由、ADR要否、Requirementsを確認する。

architecture-analysis:
  開始: 受入済みRequirements、共有分析文書の任意入力を確認する。
  終了: CurrentAnalysisとArchitectureを各1件、本文と構成図を確認する。

planning:
  開始: 受入済みRequirements、共有分析文書の任意入力を確認する。
  終了: ImplementationPlan、実装手順・検証方法、必要なUnit計画を確認する。

tdd:
  開始: 受入済みRequirements、先行planningがある場合は受入済みImplementationPlanを確認する。
  終了: 直接実装の全体検証、またはUnitの内容照合と反映、現在回の必要commandの成功記録を確認する。

integration:
  開始: 受入済みRequirements、先行planningがあれば受入済みImplementationPlan、先行tddがあれば受入済み証拠を確認する。
  終了: 現在の集合SHAに対応する必要commandの成功記録と宣言文書を確認する。

initialization以外の終了:
  目的・範囲・受入条件、阻害事項なし、現在の集合SHA、資材または資材なし理由、ADR要否も共通に確認する。
  設計判断の記録が必要なら、採用するadr文書を入力または出力へ宣言する。
  検証方法やUnit詳細はplanning以降の該当段階で登録する。

実測証拠:
  tddとintegrationの結果JSONは現在回のstep_id・stage・runsを持つ。
  各runには実測のunit_id・command・整数exit_code・非空の出力ファイルを指すoutput_pathを記録する。
  必要commandの成功（exit_code=0）を対応するverification_sha256で確認する。結果は実行後にtest_resultsへ登録する。
  コード・テストコード・commit・実測証拠は文書outputsへ登録しない。
  Sensorは成功記録を確認し、独立レビューはRED/GREENの意味と記録の真正性を確認する。

文書の共通条件:
  metadataは本文とは別に文書を識別・検索する情報。typeは文書の用途を識別し、段階で要求する型と一致させる。
  全文書に正しいtype、非空のtitle・description・本文が必要。日時はmemory CLIが生成する。
  以下の保存先は当該Spaceのknowledge rootからの相対path。
  必須見出しは正確なH2（## 見出し）で記載し、各節の本文も非空にする。

Rule:
  rules/rule.md。固定の必須H2はない。共通のmetadata・本文条件は必要。

Requirements:
  design/<intent_id>/requirements.md。現在intent_idが必要。
  必須H2: ## 目的、## 範囲、## 要件、## 受入条件、## 未確定事項。

CurrentAnalysis:
  codekb/current-analysis.md。
  必須H2: ## 現状、## 構成・動作、## 根拠、## 未確認事項。

Architecture:
  codekb/architecture.md。
  必須H2: ## 構成図、## 構成要素、## データフロー。
  構成図節には非空のMermaidコードブロックが必要。

ImplementationPlan:
  design/<intent_id>/implementation-plan.md。現在intent_idが必要。
  必須H2: ## 変更箇所、## 実装手順、## 検証方法。

Knowledge:
  現行仕様・利用方法をcodekb/配下へ宣言する。
  必須H2: ## 機能、## 利用手順、## 制約。

adr:
  設計判断が必要な場合にadr/配下へ宣言する。固定の必須H2はない。
  共通のmetadata・本文条件に加え、新規出力には現在intent_idが必要。

文書の版と省略:
  後続で使うRequirementsと先行planningのImplementationPlanは、受入済みpathと内容hashを照合する。
  planning省略時は、その受入済みImplementationPlanを一律要求しない。
  CurrentAnalysis・Architectureは共有現在版の任意入力。ただしarchitecture-analysis終了では各1件が必要。
  共有文書のintent_idや日時を形式だけのために更新しない。

文書outputs:
  initialization・tdd・integrationには一律の既定文書出力はない。必要な文書だけを現在回へ宣言する。
  必要な文書がなければ空にできるが、段階の他の条件や実測証拠は引き続き必要。

`
}

func memoryWriteHelp(action string) string {
	text := "Knowledge・ADR・Ruleの本文とmetadataを保存する。\n使い方: aidlc memory " + action + " CONCEPT-ID --space SPACE --body-file BODY.md --actor ACTOR"
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
  aidlc memory create codekb/authentication --space default --type Design --title '認証の仕様' --description '現行の方式' --tag authentication --status stable --actor process:codex --body-file body.md
  aidlc memory show codekb/authentication --space default
  aidlc memory update codekb/authentication --space default --body-file body.md --actor process:codex --expect HASH
`
}

func assignmentHelp(action string) string {
	usage := map[string]string{"init": "--file INIT.json", "reset": "--file RESET.json", "list": "", "show": "ASSIGNMENT", "check": "ASSIGNMENT", "reserve": "ID --space SPACE --session MAIN --expect REVISION --file RESERVE.json", "release": "ASSIGNMENT --session MAIN --expect ENTRY_REVISION --file RELEASE.json"}
	return "使い方: aidlc assignment " + action + " " + usage[action] + ` [--project-dir ROOT]
管理CLIはagentを起動しない。メインAIが予約のtask_nameとagentをnative spawn_agentへ渡す。worker以外は現在procedureのread-only担当。実child rootや全process停止の証明ではない。
init: {"request_id":"一意の要求ID","human_confirmed":true,"reason":"人間が既知の作業停止・成果回収を確認した回答と理由"}。初回も人間確認が必要。既存registryは上書きしない。
reserve: {"registry_epoch":"initのepoch","request_id":"一意のID","step_id":"現在step","agent":"aidlc-worker","root":"/project","session":"worker-a"}。--expectは現在Intent revision。通常ディレクトリを使い、管理rootと同じ場所を許可する。同一・親子rootのworkerは順次実行する。同じ管理rootの全Space/Intent/sessionで占有を検査する。
show/listでassignment_id、entry_revision、task_name、状態を確認する。checkは現在step・担当・Unit・開始条件の再検査。追加依頼は応答確認済みの相対task_nameを使う。Post欠落時は不明として保持し同名再spawnをしない。
release: {"registry_epoch":"epoch","request_id":"一意の解放ID","previous_run_stopped":true,"no_more_requests":true,"reason":"既知処理終了と成果・残件回収を確認"}。--sessionは予約したメイン会話、--expectはentry_revision。追加依頼終了、既知コマンド/background終了、成果回収をメインAIが確認する。不明なら保持して人間へ確認する。Post/Stop/TTLやunit resultでは解放しない。
同じepoch/request_idと同一内容でretryする。解放後の古いreserve要求はreleasedを返し再占有しない。保存や応答の喪失時はshowして同一要求を再試行する。
reset: {"request_id":"一意のID","registry_epoch":"元epoch","registry_hash":"元ファイルSHA-256","human_confirmed":true,"reason":"人間確認の回答と理由"}。全件解放後に世代を更新し旧記録を保管する。復元できる原本を優先。復元不能時のみdiagnosisにunrecoverable（読取可）/missing（欠落）/corrupt（破損）を明示。missingは元epoch/hashなし、corruptは元hash必須。旧epoch要求は拒否する。reset自体はworkerを停止しない。
長さ上限: request/session/step/agentは160byte、理由2048byte、registry256KiB。容量不足は新規要求を止め、受理済みPostと解放の余裕を残す。人間確認・session文字列は申告であり本人認証ではない。
Unitありはunit claim/reassign --helpを参照。旧Unitへ予約を後付けせず、停止・成果確認後に新Intentへ新規claimする。
`
}
