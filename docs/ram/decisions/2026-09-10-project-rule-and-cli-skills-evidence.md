# Issue #157 RuleとCLIスキル分離の実装証拠

状態: loop完了。work_unit_id=project-rules-and-aidlc-skills、verification_mode=loop。開始HEADは5caaf433f902488c551ad249f9577fec43e37227、branchはcodex/project-rule-cli-skills。直接承認と範囲は[承認RAM](2026-09-10-project-rule-and-cli-skill-approved.md)、[計画](../../design/project-rule-and-cli-skills-plan.md)、Issue #157に従った。単独writerで実装し、原本checkoutは編集していない。親が途中に追記したparser接続の計画/RAM/索引も保全して同時にcommitする。

## 順序付きTDD

| slice / exact command | 初回実測 | 最小実装修正後 | 境界再確認 |
| --- | --- | --- | --- |
| 1 `go test -count=1 ./src/internal/install -run '^TestRuleSkillSeparationAssets'` | RED exit 1、3cc312: CLI skill欠落、旧WORKFLOW配置/参照 | GREEN exit 0、0f1091 | exit 0、78468c |
| 2 `go test -count=1 ./src/internal/flow ./src/internal/minimal ./src/internal/workspace ./src/internal/workflow -run '^TestRuleSkillSeparationRule'` | RED exit 1、17025c: 初期制約混在と利用者titleでselector失敗。追加parser testは0b3f9aで正規固定Rule入力拒否 | GREEN exit 0、24462e | exit 0、1fde7a |
| 3 `go test -count=1 ./src/internal/minimal ./src/internal/flow -run '^TestRuleSkillSeparationHook'` | RED exit 1、423e1f: 両skill読取り拒否、CLI skill欠落をbeginが許可 | GREEN exit 0、f26b9c | exit 0、5e1240 |
| 4 `go test -count=1 ./src/internal/install -run '^TestRuleSkillSeparationRelocate'` | RED exit 1、8ff24e: 新skill未移転、欠落/編集/旧配置/symlinkでも2ファイルを保存 | GREEN exit 0、e5bdee | exit 0、cf72cd |
| 5 `go test -count=1 ./src/internal/cli ./src/cmd/aidlc -run '^TestRuleSkillSeparationHelp'` | help索引到達とcmdはALREADY_GREEN。749972 exit 1は旧Rule selector/旧移転案内の不一致 | GREEN exit 0、42fddf | exit 0、0db371 |

chunkはtool実行出力識別子。文書文言のために既存実装を壊した人工REDはない。slice 2の初期commandにはworkflow packageを含めず、途中で親が所有を追加して上記へ拡張した。minimalのRule全文/hash、workspaceのRuleコピー・不正拒否は初回からALREADY_GREEN（335a43）で製品実装を変更しなかった。

初回Rule fixtureはexecutionFixtureがinitializationを空入力へ置換するため、配置原稿のtitle問題を検査できなかった。fresh installそのものを使うfixtureへ補正し、17025cで正規のREDを再測定した。固定path入力の既存parser制限で一度停止し、親が計画へ限定許可を記録して再開した。slice 3のwaiting_for_answerという誤fixture statusはREDに数えず、既存のwaitingへ補正して423e1fを測定した。既存WORKFLOW/旧Rule文言/移転件数の期待を新配置へ追従した際の0c694fも新機能のREDには数えない。

## 実装と検査の境界

rule.mdはtype Rule、利用プロジェクト共通ルールのtitle/descriptionと非空の「追加ルールはありません」。全stageは固定pathとmetadata type Rule、version currentを参照する。入力parserはこの組合せだけを追加し、他path/type/versionやmatch/count/role/accepted_at混在を拒否する。既存metadata検証と一般selector検査は保持する。

AI-DLCの進行/承認/記録規約をaidlcへ、操作目的からhelpへの索引をaidlc-cliへ移し、新配布からWORKFLOWを廃止した。aidlcは原稿3616 bytesで4 KiB以内。TDD手順はstage/worker、独立reviewは担当/共通規約、引数/JSONはhelpへ接続した。旧全文コマンド例の配布testはhelp参照先を検査するよう追従し、操作の内容検査を維持した。

installは両skillを明示mappingし、全件事前検査と既設無上書きを維持する。relocateは両skillとhooksの3ファイルを事前検査し、既知参照だけを変更する。途中失敗Paths/Pending、同一retry、未知編集/欠落/旧配置/symlinkの無変更拒否を確認した。旧利用先へのupgradeやRule上書きはしない。

hookの限定catは両skillへ接続し、未読Rule・変更hash・in-flight・待機状態・危険shell構文・任意path・symlinkの境界を維持する。承認待ちのskill読取りを追加確認し、一般書込みは従来どおり拒否する。initializationは新CLI skill欠落を拒否する。Sensor/承認/state schemaを変更していない。

## 関連回帰と整形

以下は境界で全てexit 0。

- `go test -count=1 ./src/internal/install -run '^(TestFlowInstall|TestInstallMemoryCommandGuidance|TestMemory|TestOKFWorkLogInstalled|TestCodeKBGuidance|TestDocumentDistribution|TestStagePlanner|TestExecutionPlanDistribution|TestRelocate|TestInstallRecovery)'` — b1c8f2
- `go test -count=1 ./src/internal/minimal -run '^(TestFlowWorkflow|TestFlowInactive|TestRulesFullText|TestSessionStart|TestExecutionPlanCLIPendingHook)'` — a46b2a
- `go test -count=1 ./src/internal/flow -run '^(TestExecutionPlanBootstrap|TestProcedureBoundary|TestStagePlanner)'` — 75f7b5
- `go test -count=1 ./src/internal/workflow -run '^(TestDefinition|TestStagePlanner)'` — f979d0
- `go test -count=1 ./src/internal/cli -run '^(TestMemoryHelp|TestRelocationCLI|TestProcedureGrammar|TestOKFWorkLogHelp|TestConfigureHelp)'` — 615951

変更Goファイルへgofmt、git diff --checkはexit 0（eb2d60）。全体test/race/vet/build/E2E/live/skill validatorは実行していない。

## 親finalへの引継ぎ

memory live promptは現行codekb/live-noteへ補正し、Intent選択前のmemory create help、その後のプロジェクトRule・両skill明示cat・procedure、CLI create/updateとmetadata保持の順序を要求する。親のjsonlでtool_callの両skill catコマンド、対応tool_resultの本文、Rule bind全文/hash、procedure結果、create/update実行結果を突き合わせる。SessionStartのaidlc配信だけをaidlc-cli実読込と数えない。Ruleに製品進行規約を戻すmarkerは加えていない。

human approval liveは旧WORKFLOW/Rule本文を固定せず既存配布を読むため原稿追従の変更は不要だった。親の `/tmp/ai-dd-rule-skills-final.py` で全体・配布・限定liveと新配布両skillのquick_validateを行う。実読込と回答品質、実機の承認待ち非迂回はloopから保証しない。

## 変更ファイルhash

以下は整形後SHA-256。証拠自身と索引は自己参照を避けて除く。

- `docs/design/project-rule-and-cli-skills-plan.md`: `ebba0fd2e0a1e615b73e2f9e4c1922631718ec37306e197349c574361649e8df`
- `docs/development.md`: `2f80fdf1109ba6fdfec2d11c842ad88c28c11aaa26ab2be62a2dd75f045b124f`
- `docs/ram/decisions/2026-09-10-project-rule-and-cli-skill-approved.md`: `71272ee5187ea5d3719e3d36941015aa3dec59118bd948fc4107dee0797b1b32`
- `src/cmd/aidlc/main_unix_test.go`: `87b8e90e37f54c8e6c6c8b6e48f675b8bf714e5423e253ff5e5538c34e90593e`
- `src/cmd/aidlc/memory_live_integration_test.go`: `46b804c13239fdce9ad7e7e5e54295db7d924a491f27f4c3e8f0df26f332c196`
- `src/cmd/aidlc/rule_skill_separation_test.go`: `4cd903625846110110590e60675a845cfdb2ba6562275e53c4d81b7789bf5612`
- `src/core/minimal/knowledge/rules/entry.md`: `c087051500207624c811cba3490fe1f660ab51049669953bdb0717808ac3af08`
- `src/core/minimal/knowledge/rules/rule.md`: `19426899ca50179799fa9716e282025ae399354857d11a81f1885bbbb410a725`
- `src/core/workflow/stages/architecture-analysis.md`: `64b952932525ae58e58d584303f4483710fcf36515f7d97c3f5b45305304ce06`
- `src/core/workflow/stages/discovery.md`: `795a36725c4e16c7fc271f3b0f47e50738b4e56f98e6dc390e8eb8ffd6958e7d`
- `src/core/workflow/stages/initialization.md`: `b7c143ef3778c89b37ad3c182fd23663e528afc7167de1966d9e425a60e7e297`
- `src/core/workflow/stages/integration.md`: `b299f1148c113470e42a845e9c127f9cbf278801011b7413c5028ca18b4859bb`
- `src/core/workflow/stages/planning.md`: `5ae71c2b70b0820d7831e0d4c8c82da4a9222cdf67edf060167473bc8f2960da`
- `src/core/workflow/stages/tdd.md`: `8a198265f8b504d7d916b2ec5571ce55dd7f01e6dd1c36f22c13cdbc01e55265`
- `src/harness/codex/minimal/SKILL.md`: `08f7f3f4f1d3c901fcc07a74b68da1f579edfefabe044627ff9360e3fb3c012a`
- `src/harness/codex/minimal/WORKFLOW.md`: `deleted`
- `src/harness/codex/minimal/aidlc-cli/SKILL.md`: `5482c653651c21df6dbacaac414f7798c29d19c7978c44a299cce2cccb03dba1`
- `src/harness/codex/minimal/assets.go`: `bccce44659c9345ad4cf705b5aa78b5e77b19e6327e0333f6f2623d3f0a28bed`
- `src/internal/cli/help.go`: `e235039bf726d8b8e119e22757248703c17d2eda7be70acb932020ea979e60cc`
- `src/internal/cli/rule_skill_separation_test.go`: `61ac8fea4f1a83acbd6ab091345d2be0d42361dc22d1e5372a6286ec79eed0c1`
- `src/internal/flow/procedure.go`: `bceccc4b77bb3214a679cb13aed7a0c1f1b47a9ef6d9ea372b96a92c10e6d385`
- `src/internal/flow/rule_skill_separation_test.go`: `a9ba3401cca2357a0bd09313e7d73eb24f190fdd460fc5676f19d2009de350db`
- `src/internal/install/codekb_test.go`: `bb5f346cee285c6704767af7dbb3fb30ff9afa243cf4390075d22c420944c607`
- `src/internal/install/documents_test.go`: `b9b76508d60f88507a57882843846d8893a204cb5988964e457e58093aad749b`
- `src/internal/install/execution_plan_test.go`: `e224e74a617ab2aa5861d1889af362b3690122fe2c6e39b2554110553a8ee1c1`
- `src/internal/install/flow_test.go`: `8d48e394339a6a6dc48d98a65741244c602a3c559088e51e9f94bd1d969a30c8`
- `src/internal/install/install.go`: `99bbcec047556882d9a77b093b33f3b2547e2c754d611e66b4f54211a5e068d0`
- `src/internal/install/install_test.go`: `379100b54a2e4d519f365df203c5d9ce5030984f66673826cc5178ee348394ff`
- `src/internal/install/relocate.go`: `81c8a17a551cd6a2b3ac2d1cdf40bcc78012070d812692ac8f4db48216c19c99`
- `src/internal/install/relocate_test.go`: `991c010f312427ae5134b14f2bfb396237bf0ba87beafe29d59686f262487aec`
- `src/internal/install/rule_skill_separation_test.go`: `f2b9c8b2445fdd411cc46d073efd32d71a5008d13ffaf57b93aebd37e77c3ada`
- `src/internal/install/stage_planner_test.go`: `30b8d5e998f5fefcfe9994e1d13e942f9326108b56ddfa0b46fa45dc12e8757a`
- `src/internal/minimal/execution_plan_test.go`: `620974d7c659192a8ac0f29b4209c30c47866f2977999f29b9f5b2c6ff96ff63`
- `src/internal/minimal/flow_test.go`: `e7c40a1c5c30a1948b57f2d9d9c28e52022f0973c96a0fb0b67d909af317259c`
- `src/internal/minimal/hook.go`: `89170a5f5755d6264c52d118464a84855678ca2cc2074d0af668c4004d3175cf`
- `src/internal/minimal/rule_skill_separation_test.go`: `2d4f0ed67fc747e631fe38b084055afe3a8c5a47e87a42917a2386b9b29c1cce`
- `src/internal/minimal/session_test.go`: `e8c86cd05a3b97c6eb9771ce0901f2ef377646f0a085bd713f592650e05a0d40`
- `src/internal/workflow/definition.go`: `ce7e84dd7949f1c4a350152b7c6d7d49802828b7bcaa794740f4a0ee2674eb0c`
- `src/internal/workflow/rule_skill_separation_test.go`: `6aec1794abcf5a0c77729e6e2ffccb86b22bb87187beb822a32b6eba8ce2ae5d`
- `src/internal/workspace/rule_skill_separation_test.go`: `80a856fd0b909236f5c45a0282963c37c6dd5d3c8c33faefde9146caf2663407`

## Review修復1

work_unit_id=project-rules-skills-review-repair-1、verification_mode=loop。開始HEADはc4d9ded8e3015b3791ad7d3ec51a13e6630eda23。

P1: 固定path正本契約に追従し、無関連の同型Rule追加が正規選択を変えずCheckWork可能、正規Rule本文変更と正規path欠落は拒否されることをTestSelectedDocumentsStartAndIdentityで確認した。製品実装は変更せず人工REDなし。テスト編集時の余分な閉じ括弧によるgofmt失敗は補正し、REDとして数えていない。

P2: 固定Rule入力のtype以外のtitle/description/status/tags/intent_idの各指定を拒否するtestを先行追加。`go test -count=1 ./src/internal/workflow -run '^TestRuleSkillSeparationRule'` はexit 1（ceb140）、全5属性が誤って受理されるrunnable REDを観測した。許可分岐へ全5属性nilを追加し、正規type-only受理とmetadata既存検証を維持した。

末尾確認はすべてexit 0（396a48）:

- `go test -count=1 ./src/internal/workflow -run '^TestRuleSkillSeparationRule'`
- `go test -count=1 ./src/internal/flow -run '^(TestSelectedDocuments|TestRuleSkillSeparationRule)'`
- `go test -count=1 ./src/internal/okfmemory -run '^TestDocumentSelectorExactAndCount$'`
- 変更Goへのgofmtと`git diff --check`。

一般metadata selectorの件数・完全一致・曖昧重複拒否は既存okfmemory回帰で維持を確認。全体finalは実行していない。上記hash一覧のworkflow定義/関連testは本修復で更新され、確定内容は修復commitを参照する。
