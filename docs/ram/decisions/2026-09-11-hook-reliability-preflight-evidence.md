# Hook修正前の証拠fixture

日付: 2026-09-11。Issue #169。work unit: `hook-reliability-preflight`、検証mode: `loop`。

[直接承認](2026-09-11-hook-reliability-repair-approved.md)と[修正計画](../../design/hook-reliability-repair-plan.md)のG0に基づき、実製品hookへ入力を中継するtest-only fixtureを追加した。製品修正と実機での修復確認は未完了である。

`src/cmd/aidlc/hook_reliability_probe_test.go` は実製品binaryを起動し、raw入力、stdout、stderr、exit、sessionの保存前後を採取する。正常Post、重複Post、保存ロック競合、壊れた入力について、別の製品直接起動と結果を比較した。ロック競合時の製品はexit 0でも `continue:false` を返し、Toolを保持する。これは製品内部の競合経路の観測であり、過去の実案件の原因確定ではない。

証拠判定はPre、同session／turn／tool IDの `CommandExecution` 終端、Post、保存前後を照合する。保存失敗・残存は「観測完了、未修正」、欠落・ID相違・未終端・子の最終回答だけ・hook一覧だけは「不完全、不明」とする。重複Postは保存bytesが不変なら冪等として扱う。終端の形は固定Codex 0.153.4で親が採取した構造化eventを根拠とし、outer call IDや自然文から同一性を推測しない。

`hook_reliability_probe_integration_test.go` はprepareとrunを分離する。prepareは既存hooksの候補資材をGit外に生成し、配置済みrootを変更しない。runは候補hash、実製品・helperのhash、固定OS／Codex、親の準備完了入力を確認し、通常のCodex CLIを起動する。各ケース5分、全体25分。trust台帳の変更、trust bypass、assignment初期化、人間承認の代筆は行わない。

## TDD証拠

| 項目 | command | RED | GREEN |
| --- | --- | --- | --- |
| G0-1 | `go test -count=1 ./src/cmd/aidlc -run '^TestHookReliabilityProbeProtocol$'` | exit 1。空scaffoldがraw入力・製品応答を保持しない。次のprocess入口scaffoldも同じcommandでexit 1 | exit 0。4ケースで製品直接起動とwrapperの結果が一致 |
| G0-2 | `go test -count=1 ./src/cmd/aidlc -run '^TestHookReliabilityProbeEvidence$'` | exit 1。空判定が期待する分類を返さない。prepareの空scaffoldも同じcommandで候補file欠落のexit 1 | exit 0。分類と既存登録を保持するprepareを確認 |

追加の `go test -tags=integration -count=1 ./src/cmd/aidlc -run '^TestHookReliabilityProbeCollectedEvidence$'` は `ALREADY_GREEN`（exit 0）。独立の構造化終端入力とhook記録を対応付け、最終回答だけへ置換した場合は不完全になることを確認した。この補足collector testを新しいREDの証拠とはしない。integration tagでG0-2もcompile・実行済み。全package、race、vet、配布E2E、liveは未実行。

## 候補資材と未完了の条件

候補は `/tmp/aidlc-hook-reliability-169.Ivx0Nq/evidence/`、実行helperは隣接する `probe.test`。prepareだけを実行した。rootは親の用意した `/Users/const/sori883/ai-dd-validation/hook-reliability-169/probe-project`、製品binaryは同専用領域の `baseline/aidlc` である。

| 資材 | SHA-256 |
| --- | --- |
| `manifest.json` | `2bbcdf26b02ee5c6f219d72628e408c31a826435f9b3f5ba1d9ef75214305005` |
| `hooks.candidate.json` | `2486afe9467cc713541b9e9853d8b3841e33a6b03977ba9558e6229826a7a68b` |
| `wrapper.sh` | `fb428ac84df18aad69fd57fcd17d06f547a3b15e0b83907edbb369de3d7dad65` |
| `probe.test` | `b689a260b620394556a73790b6c1b4b168bfd9aa3bc59c5e907997f66183766d` |
| baseline製品 | `9175e0d29260d032191316ad88326632a2c8fcffb8f5e01cbb204b1fa811f9da` |

親が候補配置・通常trust・初回割当確認を済ませ、配置後の新しいIntentを用意してからrunへ進む。`AIDLC_HOOK_RELIABILITY_ROOT`、`BINARY`、`HELPER_BINARY`、`EVIDENCE`（いずれも同prefix）、`SPACE`、`INTENT`、`READY=1`、`LIVE=1`を明示する。入口は `go test -tags=integration -count=1 -timeout=30m -v ./src/cmd/aidlc -run '^TestHookReliabilityProbeLive$'`。prepareには同じpath入力と `PREPARE=1` を使う。helperは事前に `go test -c -tags=integration` で永続する一時pathへbuildする。

現fixtureの自動終端判定は確認済みの `CommandExecution` に限定する。失敗patchや子の途中報告はraw入力を保持するが、未確認の終端形式や親の実受信を合格に置き換えない。prepの成功、CLIのexit 0、一覧表示は製品合格ではない。実機のrunning→poll、patch、並列要求、子報告のケース別評価とwrapperを外した確認は、親の実機検証に残る。観測自体がタイミングへ影響し得る。

開始・終了HEADはともに `cf5631b6a26f7f307c70f7e8df3547594a4c1da6`。commit・Issue／PR操作は親へ委ねる。Serena toolsは実装担当のcallable一覧になく、組み込みreadを使用した。親側でのactivate・初期指示確認は完了済みと連絡を受けた。
