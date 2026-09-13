# 共通手順とCodex設定を分離しClaude未完了作業を破棄する

2026-09-13。ユーザーは、共通手順を一か所に置き、ハーネスには環境固有の違いを置く具体計画へ「はい、じゃあそれで、うんうん、まあお願いします。」と回答した。これは[実装計画](../../design/common-skills-codex-plan.md)全体への直接承認である。旧33 Stageや3環境対応の包括承認を流用しない。

## 採用する構成

共通の15 skillを `src/core/skills/`、5担当の役割本文を `src/core/agents/` にまとめる。承認・単独writer・入力等の共通部品を一か所に置き、必須境界は生成物へ含める。長い方法論はskillを必要時に読む。`src/harness/codex/` はCodex固有のtool・event・権限・形式・配置処理を持ち、既存のManifestを使って配置する。原稿・参照資料・ライセンスを移す。生成済み本文を第二の正本にしない。

Go単一バイナリ、外部module数、CLI名、配置先、state、Sensor、承認・担当管理を維持する。配置済み工程MDはbytesとdefinition hashを維持する。通常のソース整理であり、本家2.6.123の参照済み範囲へ新しい製品挙動差分を加えない。未確認のupstream全体との一致を主張しない。

## Claude作業の置換と削除範囲

[Claude未完了保留](2026-09-13-claude-adapter-paused-incomplete.md)の「実装を保持して再開する」を置換する。Issue #185「CodexとClaude Codeを環境別アダプターとして新規配置する」、local branch `codex/claude-adapter`、専用worktree `/Users/const/sori883/ai-dd-naming` の削除を承認済み。調査時点で同branchのremoteとPRはない。D1の共通Manifestはmainへmerge済みなので保持し、未commitのD2コード・runtime schema変更を今回へ持ち込まない。

削除前に未保存だった15件のRAMと[Claude接続計画](../../design/claude-code-connection-plan.md)を内容を変えず回収し、索引へ追加した。これらは当時の判断・実測の履歴である。Claudeの部分成功やCredit balance too lowを全機能の成功・無料プランの制限と説明しない。Claude再開はCodex確認後の新計画で決める。Copilot保留・fresh配置・単一CLI配布の関連判断も回収した。

READMEのAI-DD見出しと既存RAM、他のworktreeを保持する。README見出し修正だけの「Issue不要」はその修正に適用する。今回の共通化はリポジトリの通常Issue／PR運用で管理する。新規Release・tag公開はこの承認に含めない。

## 検証と完了

単独実装担当が順序付きTDDを行い、独立review後にread-only finalを行う。配置・relocate・hook読取り・梱包・既存工程の回帰と固定Codexの実読込みを確認する。自動fixtureのtrustと通常trustを区別し、未実施・skipを成功にしない。実装結果・削除結果・検証証拠は本記録の後続へ追記する。

## loop実装の結果

Issue #196、work unit `common-skills-codex-composition`。開始HEADは`3ecd40a74de2ee9765a69cf4549bc9a037d34769`。
単独writerが15 skillと5 roleを共通正本へ移し、Codexの接続・sandbox差分を通常installで合成した。
共通の担当入力・共有保存・単独writer・承認・skill注意事項・工程操作をincludeで直接展開し、長い方法論は各skillに保持する。
役割名は共通file名、役割説明は共通Markdownの見出しから取得する。共通側からharnessをimportしない。
Codexの具体tool・event・配置pathはhost差込みとし、未使用の差分部品は残さない。

旧69資材のhash fixtureは書き換えていない。aidlc-cliと5 agentの本文構成変更は権限と完成本文のtestで分け、
他63資材は旧bytes/hashを維持した。6工程とgraphの7資材は別の固定hash照合でも一致した。
入口の4 KiB上限、最大3fileのcat、未知path・共有部品・template・symlink拒否、必須Rule、worker予約と承認境界を維持した。
relocateは完成Distributionの3 skillとhooksだけを照合し、編集済み本文の拒否・部分失敗retryを維持する。
日本語補助CLIのREADMEと5ライセンスはcoreの同じ原稿から梱包する。

以下は実際に実行したloopの証拠であり、final・実Codex・CIの成功を意味しない。各GREENは同じcommandでexit 0。

| slice | exact command | RED／ALREADY_GREEN |
| --- | --- | --- |
| content | `go test -count=1 ./src/core -run '^TestContent'` | exit 1。共有本文がworkerへ反映されず、欠落・不正path・循環・host欠落・不正template・未展開を受理。最小scaffoldから実装してGREEN |
| codex | `go test -count=1 ./src/harness/codex -run '^Test(Distribution\|Content)'` | exit 1。15 skill／5 agentが0件、変更した共通契約が5担当へ届かない。後の役割説明正本化も変更した共通見出しがdescriptionへ届かないREDを観測しGREEN |
| install | `go test -count=1 ./src/internal/install -run '^Test(CodexManifest\|StageSkills\|NaturalJapaneseSkill\|ProductAgentAssets\|WorkflowDefinitionFresh\|Bootstrap)'` | ALREADY_GREEN、exit 0。配置側のproduction変更は不要。生成TOMLの共通契約・権限、既存衝突・symlink・上限に加え、全skill Markdownの相対リンク解決も成功 |
| relocate | `go test -count=1 ./src/internal/install -run '^Test(Relocate\|OKFSkillRelocate)'` | exit 1。旧原稿SKILL.mdがなく、既知配置の移転・部分失敗retry・同一参照が失敗。完成資材への接続でGREEN。編集済み共通契約の拒否testも追加 |
| read | `go test -count=1 ./src/internal/app -run '^Test(StageSkillsRead\|OKFSkillRead\|RuleSkillSeparationHook)'` | exit 1。all／single／before beginで既知完成skillが誤拒否。完成配布集合との照合でGREEN。内部部品・template・任意配置skillの拒否を追加 |
| workflow | `go test -count=1 ./src/internal/flow -run '^Test(RuleSkillSeparation\|OKFSkillInitialization\|ProcedureBoundary)'` | exit 1。scaffoldで7資材hashが不一致、共通操作の変更が0工程へ反映。6工程への伝播・部品欠落拒否・旧hash一致でGREEN |
| package | `go test -count=1 ./src/cmd/aidlc-dist -run '^Test(Archive\|Product)'` | exit 1。Linux／Windows archiveで旧原稿参照が失敗。core参照でGREEN、README本文・7entryも確認 |
| package evidence | `go test -tags=integration -count=1 ./src/cmd/aidlc -run '^TestStageSkillsEvidence$'` | exit 1。valid fixtureが旧原稿参照で失敗。完成配布参照でGREEN |

表の`\|`はMarkdown表の区切りを避ける表記で、実行したshell引数は通常の正規表現`|`である。
package GREEN作業中のimport aliasのcompile errorは修正し、RED証拠には数えていない。

所有範囲へ追加された旧原稿参照のtest-only追従は`src/internal/cli/rule_skill_separation_test.go`、
workflowを配置する`graph_test.go`、`procedure_test.go`、`declaration_test.go`、`workflow_test.go`、
および`memory_live_integration_test.go`。既存の検査内容を維持して完成原稿へ接続した。
その限定確認として以下もexit 0を確認した。

- `go test -count=1 ./src/internal/cli -run '^TestRuleSkillSeparationHelp$'`
- `go test -count=1 ./src/internal/workflow -run '^TestDocumentDeclarationDefault$'`
- `go test -count=1 ./src/internal/app -run '^TestProcedureRead'`
- `go test -count=1 ./src/internal/flow -run '^TestGraphTransition'`
- `go test -tags=integration -count=1 ./src/cmd/aidlc -run '^TestMemoryMetadataCommandEvidence$'`

独立review、read-only final、完成skillのvalidator、TOML parser／Codex設定ロード、固定Codexの実機、対象PRのCIは親の後続gateとして残る。
全体test・race・vet・E2E・実機はloopでは実行していない。Go module・CLI/state仕様・公開Releaseは変更していない。

## Claude整理の完了

親が回収した16文書がcommit `3ecd40a`に元bytesで保存されていることを再照合した。
専用treeで稼働していたgoplsだけを停止した後、`git worktree remove --force /Users/const/sori883/ai-dd-naming`、
`git branch -D codex/claude-adapter`、`gh issue delete 185 --repo sori883/ai-dd --yes`がすべてexit 0で完了した。
直前にもremote branch・PRは存在しなかった。これらは親から受領した実測結果で、実装担当はGitHubや他treeを操作していない。

loop末尾で表の8 commandをすべて再実行しexit 0を確認した。変更Go file 26件へgofmtを適用し、
`git diff --check`はexit 0。共通15 skill・5 role、未使用host fragmentが0件であることを確認した。
開始時の作業treeに未commit差分はなく、終了HEADも開始HEADと同じである。実装担当はcommitしていない。
