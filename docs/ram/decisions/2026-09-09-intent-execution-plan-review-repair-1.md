# Intent実行計画の独立review修復1

Issue153、work_unit_id=intent-execution-plan-review-repair-1、verification_mode=loop。
直接承認済み計画内の3findingを親が一括修復として許可した。単独writer、元checkout不変更、外部module追加なし。
開始HEAD: `1b03b760e70ffcca6071c271a2f347e9d862a771`。

## 修復結果

R1: 公開PLANにreopen_step_idだけ付けてlogを作る案、由来なしinitialization、再初期化後にdiscoveryがない案を拒否する。
新ID/pending/対象由来を必須とし、初期s01からの正規再実行連鎖をOriginsへ保持する。
早期の初期化再実行、未完了現在回の再々実行、完了prefix後の再初期化＋新discoveryを許可する。

R2: HistoryRevisionをheadの確定版anchorとして追加した。通常mutationと回答source保存前に最新recordを読み、
hash・UTF-8・厳密schema/state・identity・anchor revisionを検査する。履歴列挙はrevisionの単調減少と全列の実在を検査する。
通常のconfig更新で履歴を増やさず、最新headだけを検査する方針を維持する。保存途中のretryも同じanchorを使う。
全操作auditや、stateと履歴の両方を書換えられる者への改竄耐性・本人認証は追加しない。

R3: aidlc / help / --helpのトップ案内を6段階、plan/plan-approval/approval/finish/historyとreopen --stepへ更新した。

## TDD証拠

- R1 exact: `go test -count=1 ./src/internal/flow ./src/internal/minimal -run '^TestExecutionPlanReviewReopen'`。
  runnable RED 7af1b1 exit 1（Storeと公開CLIのlog-only/後続discovery欠落/由来なし受理）。GREEN a77358 exit 0。
  正規の完了後再初期化も加えて463519 exit 0。再々実行と完了後ケースは先行修復でALREADY_GREEN。
- R2 exact: `go test -count=1 ./src/internal/flow -run '^TestExecutionPlanReviewHistory'`。
  runnable RED a5cc48 exit 1（欠落/hash不一致/古いheadでconfigure・pause・source保存が成功、revision非単調列受理、anchorなし）。
  GREEN 56b9ba exit 0。厳密state enumはALREADY_GREENとして追加確認。
  同commandの追加UTF-8回帰は8bef1f exit 1（不正bytesがJSONで置換されmutationを許可）、GREEN f2bb48 exit 0。
- R3 exact: `go test -count=1 ./src/internal/cli -run '^TestExecutionPlanReviewHelp'`。
  runnable RED dad34f exit 1（3入口で新操作なし/旧文法あり）。GREEN 98e492 exit 0。

旧復旧回帰 `go test -count=1 ./src/internal/flow -run '^TestExecutionPlanReopenHistory'` もexit 0。
最初の末尾確認中にUTF-8追加回帰を導入したため、その末尾実測は最終証拠に使わず、修正後に指定群を取り直す。

既存cmdのトップhelp snapshotが旧advanceを要求していたため、新しく承認されたトップ操作一覧へ追従させた。旧ルート拒否のassertionは保持した。

## 修復work unit末尾の実測

全command exit 0。integrationタグはDistribution unitだけを選択し、実機/journey/E2E本体は未実行。

`go test -count=1 ./src/internal/flow ./src/internal/minimal -run '^TestExecutionPlanReviewReopen'` — exit 0。log: `/tmp/intent-review-repair-boundary-review-reopen.log`

```text
ok  	github.com/sori883/ai-dd/src/internal/flow	4.158s
ok  	github.com/sori883/ai-dd/src/internal/minimal	0.545s
```

`go test -count=1 ./src/internal/flow -run '^TestExecutionPlanReviewHistory'` — exit 0。log: `/tmp/intent-review-repair-boundary-review-history.log`

```text
ok  	github.com/sori883/ai-dd/src/internal/flow	1.081s
```

`go test -count=1 ./src/internal/cli -run '^TestExecutionPlanReviewHelp'` — exit 0。log: `/tmp/intent-review-repair-boundary-review-help.log`

```text
ok  	github.com/sori883/ai-dd/src/internal/cli	0.162s
```

`go test -count=1 ./src/internal/workflow ./src/internal/flow -run '^TestExecutionPlanSchema'` — exit 0。log: `/tmp/intent-review-repair-boundary-schema.log`

```text
ok  	github.com/sori883/ai-dd/src/internal/workflow	0.161s
ok  	github.com/sori883/ai-dd/src/internal/flow	0.326s
```

`go test -count=1 ./src/internal/flow -run '^TestExecutionPlanBootstrap'` — exit 0。log: `/tmp/intent-review-repair-boundary-bootstrap.log`

```text
ok  	github.com/sori883/ai-dd/src/internal/flow	0.299s
```

`go test -count=1 ./src/internal/flow -run '^TestExecutionPlanDraft'` — exit 0。log: `/tmp/intent-review-repair-boundary-draft.log`

```text
ok  	github.com/sori883/ai-dd/src/internal/flow	0.434s
```

`go test -count=1 ./src/internal/flow -run '^TestExecutionPlanEvidence'` — exit 0。log: `/tmp/intent-review-repair-boundary-evidence.log`

```text
ok  	github.com/sori883/ai-dd/src/internal/flow	1.747s
```

`go test -count=1 ./src/internal/flow -run '^TestExecutionPlanApproval'` — exit 0。log: `/tmp/intent-review-repair-boundary-approval.log`

```text
ok  	github.com/sori883/ai-dd/src/internal/flow	5.103s
```

`go test -count=1 ./src/internal/flow -run '^TestExecutionPlanReopenHistory'` — exit 0。log: `/tmp/intent-review-repair-boundary-history.log`

```text
ok  	github.com/sori883/ai-dd/src/internal/flow	3.322s
```

`go test -count=1 ./src/internal/cli ./src/internal/minimal -run '^TestExecutionPlanCLI'` — exit 0。log: `/tmp/intent-review-repair-boundary-cli.log`

```text
ok  	github.com/sori883/ai-dd/src/internal/cli	0.182s
ok  	github.com/sori883/ai-dd/src/internal/minimal	0.597s
```

`go test -count=1 ./src/internal/install ./src/cmd/aidlc -run '^TestExecutionPlanDistribution'` — exit 0。log: `/tmp/intent-review-repair-boundary-distribution.log`

```text
ok  	github.com/sori883/ai-dd/src/internal/install	0.177s
ok  	github.com/sori883/ai-dd/src/cmd/aidlc	0.294s
```

`go test -tags=integration -count=1 ./src/cmd/aidlc -run '^TestExecutionPlanDistribution'` — exit 0。log: `/tmp/intent-review-repair-boundary-fixture-compile.log`

```text
ok  	github.com/sori883/ai-dd/src/cmd/aidlc	0.378s
```

`go test -count=1 ./src/internal/workflow ./src/internal/flow ./src/internal/cli ./src/internal/minimal ./src/internal/install ./src/internal/workspace ./src/cmd/aidlc` — exit 0。log: `/tmp/intent-review-repair-boundary-affected.log`

```text
ok  	github.com/sori883/ai-dd/src/internal/workflow	0.296s
ok  	github.com/sori883/ai-dd/src/internal/flow	57.707s
ok  	github.com/sori883/ai-dd/src/internal/cli	0.470s
ok  	github.com/sori883/ai-dd/src/internal/minimal	3.591s
ok  	github.com/sori883/ai-dd/src/internal/install	1.127s
ok  	github.com/sori883/ai-dd/src/internal/workspace	0.999s
ok  	github.com/sori883/ai-dd/src/cmd/aidlc	1.717s
```

`git diff --check` — exit 0。log: `/tmp/intent-review-repair-boundary-diff.log`

```text
(outputなし)
```

変更Goファイルのgofmt -lは空、最終git diff --checkはexit 0。sourceのSHA256一覧:

```text
08b333c2bfd4609c94bacc62a8f53a8085f6658d94aa3d39b80637aca23497a2  src/cmd/aidlc/flow_command_test.go
82509d9321a11b62a1092e759584c881bf217636bc0441270a44a4e3e5d5bd29  src/internal/cli/cli.go
b18cb355886e2d23f75a389a91fd073620fbfd9467ee2913305e61b95b798b17  src/internal/cli/cli_test.go
70fffedd62156743ba8a5a02d9492f3c4cf3e89ac994ca296377143e0bef4205  src/internal/cli/execution_plan_review_test.go
4517e95b7a6341deb66158c01a6001d7d860cd39a5fc6f7f819569bb934a0ac7  src/internal/flow/approval.go
5227796a6a44d358bee70a1999f882eb101f810c8714f5db3a35620ff48cf8e7  src/internal/flow/execution_plan.go
a967a626cc17d1e3e85df560099da9e962d9fb042d003ee59ff7ca4238a5f554  src/internal/flow/execution_plan_review_test.go
8b663f28d47be7d62b987281e7eb35d4dab62f8442af2cdab658c791cd651fd7  src/internal/flow/execution_reopen.go
121d489d2a1d90e98c7c65efceaef87f91ae73b4428db763f59a742b0bd3870a  src/internal/flow/history.go
105530d7f2d574ccf220d38fce8a375b699df527a86956610cf90c86244fd9b0  src/internal/flow/history_review_test.go
a0ed11baae1ea9f148f521663be703801e238eaa02a04d8b3766c277a070e6f0  src/internal/flow/reassign.go
327d2b6caf9ee0c274e8c276f5387888066d46da6beea22d23386896b075d74c  src/internal/flow/store.go
00588da08d8e2d50d643e1c3b7ca698abe29ae04a9059bc85baddf6957f01d6a  src/internal/minimal/execution_plan_review_test.go
```

終了HEADはこの記録を含む修復commitとして親へ報告する。独立再review・finalは親担当。全package/race/vet/cross-build/E2E/liveはloopで未実行。
