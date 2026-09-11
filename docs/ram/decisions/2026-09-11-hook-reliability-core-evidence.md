# 終了通知の保存と子の親宛報告の修正

日付: 2026-09-11。Issue #169、work unit `hook-reliability-core`、`verification_mode=loop`。開始HEADは `0019a809a1af0bee777cd8c47d2f487dd51b1700`。

[①②の直接承認](2026-09-11-hook-reliability-repair-approved.md)、[計画](../../design/hook-reliability-repair-plan.md)、[固定Codex契約](2026-09-11-fixed-codex-hook-reliability-contract.md)に基づく実装証拠である。過去の全残存原因の確定や実Codex上の修復成功とは区別する。

## 実装した結果

Hookに限り、sessionロック取得の一時競合を最大2秒再試行する。非競合エラーは即時返却し、既存ロックを奪取しない。解除は同sessionで保持中のTool IDと一致したPostだけとし、pollが別turnへ跨ぐ場合も維持する。重複・古いID・別session・Stopで新しいToolを消さない。取得したロックの解放失敗も診断へ返す。CLIの通常ロック、sessionの6項目形式、一般Toolの単一枠は維持する。

子のsend_messageは現在工程・role資格を確認し、一回の検証済みregistry snapshot内の同contextのbound記録から親の正式task名を導出する。全roleで親pathが一意、同roleが最低一件あり、targetが完全一致する場合だけ許可する。workerは同snapshotの予約owner・context・未解放を照合する。Canonicalは固定PostSpawnと同じ名前空間・正規形・末尾TaskName・512byte上限を検査してからpath.Dirへ渡す。

初回の適格pendingだけは最大2秒、ロックを保持せず再読取りする。再読取りではbinding・工程資格・定義版・registry epochを検査する。不正・uncertain・親矛盾は即時拒否し、期限切れは同じ子の報告再送を案内する。再spawn・自動解放はしない。報告Pre/Postが親の保存bytesを変更しないことを確認した。これは現在roleと宛先の制限で、agent_idと特定予約の本人認証ではない。

復旧案内は成功・失敗を問わず終了を確認したTool残存に揃えた。同じSpace・Intent・sessionでメインAIが既存の `session bind ... --recover` を明示実施する。worker解放は別であり、失敗patchのPostなしを架空の通知で補わない。

## TDDと境界確認

次の各commandをRED先行またはALREADY_GREENで実行し、変更後にGREENを確認した。末尾にも対象command群を再確認する。

| 項目 | exact targeted command | 初回の観測 | 変更後 |
| --- | --- | --- | --- |
| C0分類 | `go test -count=1 ./src/cmd/aidlc -run '^TestHookReliabilityProbeEvidence$'` | exit 1。failed終端を拒否し、completed＋非0を誤認 | exit 0 |
| C0対象選別 | `go test -tags=integration -count=1 ./src/cmd/aidlc -run '^TestHookReliabilityProbeCollectedEvidence$'` | exit 1。準備操作を試験対象に混ぜ、failed終端を拒否 | exit 0 |
| C0 wrapper補足 | `go test -count=1 ./src/cmd/aidlc -run '^TestHookReliabilityProbeProtocol$'` | exit 1。mktemp失敗で製品が未実行、stderr混入 | exit 0 |
| C1 | `go test -count=1 ./src/internal/minimal -run '^TestHookSessionContention$'` | exit 1。競合時に待機せず届いたPostを失う | exit 0 |
| C2 | `go test -count=1 ./src/internal/minimal -run '^TestHookTerminalPersistence$'` | release_failureだけexit 1。他の既存保持条件と保存callback失敗後の再試行はALREADY_GREEN | exit 0 |
| C3既存境界 | `go test -count=1 ./src/internal/minimal -run '^TestFlowHookSelectionRulesAndRecovery$'` | ALREADY_GREEN、exit 0 | exit 0 |
| C3案内 | `go test -count=1 ./src/internal/install -run '^TestInstallRecoveryGuidanceAndContextLimit$'` | exit 1。配置済み案内に成功終了・同contextの条件がない | exit 0 |
| C4 | `go test -count=1 ./src/internal/minimal -run '^TestChildReportToParent$'` | exit 1。適格な子の親宛報告を拒否 | exit 0 |
| C5 | `go test -count=1 ./src/internal/minimal -run '^(TestChildReportBoundary\|TestChildHookNotifications\|TestChildHookEligibility\|TestChildHookCommandBoundary\|TestAssignmentDispatch)$'` | exit 1。bound併存のuncertainを許可し、pending待機を行わない | exit 0 |

C5のworker active/released/pending中release/owner不一致は補足testがALREADY_GREEN。個々の子の本人認証を保証するtestは追加していない。時計はServiceごとのprivate依存であり、共有globalを差し替えていない。

C0ではユーザー所有hookの保持、子報告依頼文、固定形式の構造化受信照合も同じEvidence commandでRED→GREENを確認した。`MESSAGE` のauthor・recipient・本文を完全一致で検査し、FINAL_ANSWER、別sender、別nonce、2要素の暗号化content等は受信成功にしない。直前のmetadata隣接は固定writerの保証がないため必須にしない。

影響package検査 `go test -count=1 ./src/internal/minimal ./src/internal/install` は両packageともexit 0。変更Goのgofmt、差分check、5agentのTOML parseを確認。標準python3にはtomllibがなく最初のparseは環境エラーだったが、既設Python 3.13で成功し、外部tool/moduleは追加していない。全package、race、vet、cross-build、E2E、liveは実行していない。

## 実機入口と未実施の判定

修正後のtest-only helperとbaseline用の候補資材は `/tmp/aidlc-hook-reliability-169-core.f6XhO3/` に生成した。`evidence/manifest.json` がroot・binary・helper・hashを保持する。旧 `/tmp/aidlc-hook-reliability-169.Ivx0Nq/` の候補はstaleなので採用しない。

| 資材 | SHA-256 |
| --- | --- |
| `probe.test` | `bee1b7acde77ec50841e71002b4b42dfe8849154f674d6d0e2672cfce4ef78a8` |
| `evidence/manifest.json` | `1ccd86fb23f8d4a625d64b68c747e3660a09744ea0ff55b499f97d8ef959f56a` |
| `evidence/hooks.candidate.json` | `cbc68721e863c099a3ba24b40bd19cdd2fd7ad018ec6e74b7a6d3485a6a9a02a` |
| `evidence/wrapper.sh` | `cdfc6abb62872393d92efd6029f58a6b0ec376c97b535ff6cfff237a78990344` |

この候補は親が用意したbaseline binaryを参照する。core製品の実機確認には、新binaryとその配置済みcommandに一致する別候補をprepareし、通常hook review/trustを行う。prepareは完全一致する製品hookだけを置換し、未知・ユーザーhookを保持する。記録先の作成失敗では記録なしの実製品直接execへ移り、mktempのstderrを混ぜない。観測欠落を成功にしない。

`TestHookReliabilityProbeLive` の通常モードは正常／非0の単発・poll付きBashの4ケース。`AIDLC_HOOK_RELIABILITY_EXTRA=1` は失敗patchと並列読取りの追加収集で、未対応終端や拒否を自動passへ混ぜず、明示assessmentを要求する。Bash等の準備条件は資材・通常trust・Intentのみである。

`AIDLC_HOOK_RELIABILITY_CHILD=1` は子の途中報告と兄弟宛拒否の対照収集で、さらに `ASSIGNMENT_READY=1`、`PARENT_TASK`、`CHILD_ROLE`（全て同prefix）を要求する。子自身がnonceを作り、同nonceで拒否対照と親宛報告を行う有限手順を渡す。実行後はraw証拠を保持して未判定とし、自己申告を製品合格にしない。

親は子の生成結果と送信hookを照合してnonceと正式senderを確定した後、read-onlyの `TestHookReliabilityProbeReceipt` へ `RECEIPT_FILE`（親rollout）、`PARENT_TASK`、`CHILD_TASK`、`NONCE`（全て同prefix）を渡せる。成功した構造化MESSAGE行のhashを出力する。この照合は試験だけで、製品の権限判定には使用しない。根拠は固定 `rust-v0.153.4` の [inter_agent_message.rs](https://github.com/openai/codex/blob/rust-v0.153.4/codex-rs/core/src/context/inter_agent_message.rs)、[protocol.rs](https://github.com/openai/codex/blob/rust-v0.153.4/codex-rs/protocol/src/protocol.rs#L881-L912)、[multi_agents_v2.rs](https://github.com/openai/codex/blob/rust-v0.153.4/codex-rs/core/src/tools/handlers/multi_agents_v2.rs#L58-L84)である。実機の途中MESSAGE成功はまだ未観測。

各入口はintegration tagと明示envが必要。実装担当はprepareだけを実行し、live・assignment init・人間承認・trust操作はしていない。親から、通常folder trust後に製品5eventが専用rootのhooks.jsonから列挙され、候補前のため全てhook未信頼と連絡を受けた。これは実機修復成功ではない。

③は固定sourceの主checkout参照と配置上の制約の記録で今回の調査を完了する。環境変換・探索仕様修正・Codex更新は実施していない。独立reviewとread-only final、実機の送受信・復旧・unwrapped確認、commit／Issue／PR運用は親が管理する。

## 親境界で検出した実hook案内の補修

work unit `hook-reliability-core-guidance-fix`。親の確認で、実際のPre拒否／Stopに出る `recoveryHint` が編集失敗限定のまま残っていた。`go test -count=1 ./src/internal/minimal -run '^TestHook.*RecoveryGuidance$'` を追加し、実 `Service.Hook` の両応答が成功終了・同Space／Intent／session・メインAI・worker解放との区別を欠くRED（exit 1）を確認した。

終了の成功・失敗を問わずメインAIが実終端を確認し、同contextで明示recoverする案内へ修正した。未終端はpoll、終了不明なら復旧や再実行をしない。新testはGREEN（exit 0）、既存 `go test -count=1 ./src/internal/minimal -run '^TestFlowHookSelectionRulesAndRecovery$'` もexit 0。応答によるsession変更はない。gofmt・diffcheckを確認し、既存20fileの差分は保持した。開始／終了HEADは `0019a809a1af0bee777cd8c47d2f487dd51b1700` のままである。

親はcoreの全差分を読み、Protocol/Evidence、integrationのCollectedEvidence、minimalのC1・C2・C3・C4・C5群、installのRecoveryGuidanceを各一回再確認し、全てexit 0だった。案内補修後は変更された案内と既存recoverの2testだけを再確認し、exit 0だった。全体検査や実機成功をこの限定確認で代用しない。次は固定commitの独立review、その後にread-only finalを行う。

## 独立reviewの4findingを修正

work unit `hook-reliability-review-fixes`、開始・終了HEAD `389a3287e1a215d2fea29f13c20c5feb992402de`。独立reviewの4点を一つのloopで修正した。

1. 別roleがboundでも送信roleに適格pendingがあり、一意の親とtargetが一致すれば待機を続ける。`TestChildReportBoundary/bound_other_role` で即時拒否のRED（exit 1）を確認し、条件修正後GREEN（exit 0）。別targetは待機なしで拒否し、uncertain・曖昧・release・resetの既存回帰も成功した。
2. 非競合errorのUnix固有文字列を除き、`continue:false` と待機なしを検査した。ALREADY_GREEN（exit 0）。
3. callbackだけの疑似失敗を、実 `Service.Hook(PostToolUse)` → 保存失敗 → 元session全bytes不変 → 同じPostの再送成功へ置換した。Serviceごとのprivate保存seamは既定で既存保存関数を呼び、global・公開schemaは変更しない。保持仕様はALREADY_GREEN（exit 0）。testability用seamの未接続を人工REDに数えていない。
4. 歴史RAM `2026-09-11-hook-reliability-repair-planning.md` の本文を維持し、EOFの余分な空行だけを除去した。

境界再確認は次のcommandで全対象exit 0。

```sh
go test -count=1 ./src/internal/minimal -run '^(TestChildReportBoundary|TestChildReportToParent|TestHookSessionContention|TestHookTerminalPersistence|TestHookRecoveryGuidance|TestFlowHookSelectionRulesAndRecovery)$'
git diff --check cf5631b6a26f7f307c70f7e8df3547594a4c1da6
```

変更Goへgofmtを適用した。全package・race・vet・cross-build・E2E・live・commit・Issue／PR操作は実施していない。実機候補は修正後の確定版で別pathへ更新する。

## bootstrap上限超過の補修

`hook-reliability-bootstrap-size`、開始HEAD `ca6893a02dc83de1a2895148b4fca0df17ca762d`。親finalの実機ではSessionStartが4KiB上限で停止しモデルtokensは0だった。原本は `/Users/const/sori883/ai-dd-validation/hook-reliability-169/final-ca6893a/results.json`。同binary pathでmainは4042bytes、変更版は4103bytesと親が比較した。

新 `TestInstallBootstrapBinaryPathBudget` は実配置Skillのbyte数と実SessionStartのcontextを検査する。観測pathで4103bytes、513bytesの絶対pathで4534bytesとなり、両方の実拒否でRED（exit 1）を確認。初回のimport cycleはREDに数えず、外部test packageへ分離後に確認した。担当別詳細を既存aidlc-cliへ移し、bootstrapのRule読込み・承認・保存・worker管理・recover条件を維持した。4KiB上限やpath契約は変更しない。初回initは全担当、claim/reserveはworkerと分ける案内も追加回帰のRED後に修正した。

境界command: `go test -count=1 ./src/internal/install -run '^(TestInstallRecoveryGuidanceAndContextLimit|TestInstallBootstrapBinaryPathBudget)$'`。GREEN、gofmt、diffcheckを確認。全体・race・vet・live・buildmatrix・commit・Issue／PR操作と既存実機証拠の上書きはしていない。再review・新finalは親へ返す。

## 担当説明の参照先を配布testへ反映

work unit `hook-reliability-stage-planner-location`、開始HEAD `1643cccc1c084ed1b2790cd42f3e04c72fe2447d`。`go test -count=1 ./src/internal/install -run '^TestStagePlannerDistribution$'` で旧説明位置へのassertによるRED（exit 1、`aidlc omits planner`）を確認した。bootstrapのaidlc-cliリンクと、その参照先のplanner役割説明を検査する契約へtestだけを修正し、agent配布・read-only・PLAN等の既存検査は維持した。製品source/assetsは変更していない。

境界command `go test -count=1 ./src/internal/install -run '^(TestStagePlannerDistribution|TestInstallBootstrapBinaryPathBudget|TestInstallRecoveryGuidanceAndContextLimit)$'` はGREEN（exit 0）。gofmt・diffcheckを確認した。全package・race・vet・live・commit・Issue／PR操作は実装担当から実施していない。

親から受領したfinal1643cccの結果は `/Users/const/sori883/ai-dd-validation/hook-reliability-169/final-1643ccc/results.json`。all/raceは上記の旧説明位置assertだけがfail、vet/tidy/format/diff/storage-integration/flow-journey/distribution-journeyはpassだった。次の実機証拠は1643ccc時点のものであり、後続headへ付け替えない。

- Bashの正常／非zero／poll正常／poll非zeroの4caseは207.43秒で全pass。`basic-assessment-1643ccc.json`。
- extraは収集専用の意図したexit 1。実failed patchの終端errorにPostはなく、Stop後もToolを保持。同時読取り2要求は一方がdenyされ、先行操作のPostでclear。`extra-assessment-1643ccc.json`。
- 通常wrapperなしでもfailed patch後に終了確認してrecoverし、次のprintfが成功。`normal-observation-1643ccc/execution.json` はexit 0、69.86秒、Intent state hashは前後同一。

上記assessment・normal-observationは同じ `/Users/const/sori883/ai-dd-validation/hook-reliability-169/` 配下。担当初期化はユーザー回答待ちで未実施、native子報告の実機も未実施で、①②全完了とは判定しない。③は調査済み。差分安定後の全体finalは親が再開する。極端に長いbinary pathのinstall前サイズ検査欠落はbaselineからの既存課題で、今回悪化しておらず別改善として扱う。

## 暗号化された途中報告の到達判定

work unit `hook-reliability-opaque-delivery`、開始HEAD `5b7dfd0e0e5ecec85ec2051186a53a663cff295a`。計画の「暗号化された途中報告の実機判定」に従い、test-onlyのopaque-delivery評価とread-only入口を追加した。平文nonce判定は変更していない。暗号文の平文内容・nonce・本人認証を証明するものではなく、確認済みidentityを入力に、同じopaque messageの送信と親への到達を照合する。実機暗号文・内部推論は読み取りもGit保存もしていない。

`go test -count=1 ./src/cmd/aidlc -run '^TestHookReliabilityOpaqueEvidence$'` を各sliceで使用した。正常到達、sender/recipient/header/body・形式不一致、Pre/Postの失敗・拒否・欠落・ID不一致、受信重複/逆順/最終回答欠落、session変更/読取り不明の順に、runnable assertionのRED（exit 1）からGREEN（exit 0）へ進めた。異なる本文を持つ同じ子の二つ目のMESSAGEも曖昧として拒否する追加RED→GREENを確認した。欠落receipt・FINALだけ等は先行sliceで既に拒否されておりALREADY_GREEN。既存 `TestHookReliabilityProbeEvidence` と識別入力不足はALREADY_GREENだった。

送信Pre/Postはevent、send_message名、session/turn/tool ID、agent ID/type、target、decode後messageを照合し、hook exit/error/JSON/拒否と全session snapshotのbytes不変を検査する。受信はauthor/recipient、正確なMESSAGE header、input_text＋encrypted_contentの2要素、暗号文のdecode後文字列完全一致を要求する。同じ子の候補MESSAGEが一つだけあり、同じ子のFINAL_ANSWERより前という順序を確認する。hashは成功出力用であり、一致判定の代用ではない。

canonicalは親がnative応答とregistryから確認する。agent IDの出所は子hookのagent_idと子rolloutのsession IDであり、PostSpawn応答がagent IDを返すという意味ではない。この確認は呼出し側の前提で、評価関数が製品の本人認証を追加するものではない。

新入口 `TestHookReliabilityProbeOpaqueReceipt` は次の明示envを使う（全て `AIDLC_HOOK_RELIABILITY_` prefix）。`RECEIPT_FILE`、`PARENT_TASK`、`CHILD_TASK` を再利用し、`PRE_RECORD`、`POST_RECORD`、`CHILD_AGENT_ID` を追加した。成功時はopaque-deliveryとsha256のみを表示し、失敗時にraw内容を出力しない。spawn、trust、準備、製品変更は呼び出さない。

```sh
go test -count=1 ./src/cmd/aidlc -run '^(TestHookReliabilityOpaqueEvidence|TestHookReliabilityProbeEvidence)$'
go test -tags=integration -count=1 ./src/cmd/aidlc -run '^TestHookReliabilityProbeOpaqueReceiptSynthetic$'
# 親が検証済み入力をenvへ設定した後のread-only実測判定
go test -tags=integration -count=1 -v ./src/cmd/aidlc -run '^TestHookReliabilityProbeOpaqueReceipt$'
```

上の最初の2commandはsynthetic fixtureだけで成功。3番目の実測判定は実装担当から未実施。gofmt・diffcheckを確認し、全package・race・vet・final・実機・commit・Issue／PR操作はしていない。親から観測原本は `child-observation-1643ccc` へbytes保持で移され、archive-manifest.jsonにhashを保存したと連絡を受けた。既存実機結果を新しいheadの結果として付け替えていない。


## 親による初期化・子報告の実機確認

上記の「初期化回答待ち」「子報告未実施」は、この追記より前の履歴である。[ユーザーの明示承認](2026-09-11-hook-reliability-probe-initialization-approved.md)を受け、専用 `probe-project` で公開CLIの `assignment init` を実施した。初回epochは `ce550579f8aebfbf2d93cda0f26316d5`。既存の元checkout、旧pilot、既知でないworkerの記録には操作していない。

固定Codex 0.153.4、macOS arm64、gpt-6-astra/xhighで、native `aidlc-stage-planner` 一体による途中報告を確認した。製品binaryはcommit `1643cccc1c084ed1b2790cd42f3e04c72fe2447d`、SHA-256 `959ad09c1f9c0b3e9ca9b4274bc57d948f3c6578b64f405ebd9f8a7cd630a26f`。以後の変更は文書とtestのみであり、実測対象の製品・配布assetsを変更していない。

観測wrapper付きの原本は外部試験環境の `child-observation-1643ccc/` に保存した。Pre/Postの正式親宛 `send_message` は両方成功し、子処理前後の親session全bytesは一致した。登録していない兄弟宛はPreで拒否された。親の構造化MESSAGEへ送信と同一のopaque文字列が届き、その後に同じ子のFINAL_ANSWERがある。独立した子rolloutのsession metadata、子hookのagent_id、native spawnのcanonical応答、registryを照合した。27個のSpaceファイルは試験前後で全て同じhashだった。

親は `TestHookReliabilityProbeOpaqueReceipt` に既存のPre/Post原本、親rollout、確認済みcanonicalとagent IDを渡し、read-only判定のexit 0を確認した。到達データのSHA-256は `cc9bd252c24540775311b5a20f062b32529afe1d08de9bd57d948bea2d17ca9e`。平文nonceの既存判定は変更せず、元の収集専用helperのexit 1も保持した。暗号化payloadの平文内容や本人認証を証明したという意味ではない。

通常のwrapperなし配置へ戻して通常UIでhookを信頼し、別Intentでも再確認した。`normal-child-observation-1643ccc/execution.json` はexit 0、137.92秒。子 `/root/hook_reliability_report_normal` の兄弟宛送信は実際のPreToolUseに拒否され、正式親 `/root` 宛は成功した。親の暗号化MESSAGEと子の送信文字列が完全一致し、親の確認返答後に子の最終回答がある。MESSAGEのSHA-256は `84dc12a18db26c9be5b77ee960dcb7c219839e7dfb63aeb1f55a790a1387a779`。同directoryの `assessment.json` に構造化行hash、identity根拠、比較結果を保存した。通常配置ではwrapperのPre/Post記録を生成しておらず、nativeの送信結果・受信・拒否を根拠にした。Space全ファイルは不変、終了時の親Tool欄は空だった。

観測原本・暗号文・内部推論を本リポジトリへ複製していない。根拠は固定Codex sourceの [multi_agents_v2.rs](https://github.com/openai/codex/blob/rust-v0.153.4/codex-rs/core/src/tools/handlers/multi_agents_v2.rs#L58-L75) と [protocol.rs](https://github.com/openai/codex/blob/rust-v0.153.4/codex-rs/protocol/src/protocol.rs#L842-L912) で、send_messageのopaque文字列がEncryptedContentへそのまま渡される確認に限る。

親は新しいsyntheticの正常・不正到達test群と既存平文test、integrationのsynthetic入口を各一回再確認し、全てexit 0だった。次のgateはこのtest-only差分と実測判定の独立review、固定headのread-only final、現headのGitHub checksである。③の調査後の対応案は[別記録](2026-09-11-worktree-hook-followup-options.md)で、製品変更の承認とは区別する。

## opaque到達判定のFINAL type gateを補修

work unit `hook-reliability-opaque-final-type`、開始HEAD `5c33e62c6fd1cbaabf2ce52d99289575d4413047`。独立reviewで、FINALと候補件数を外側・payloadのtype確認より前に集計していた問題を確認した。正規MESSAGEに続く非response_item FINALと非agent_message FINALの2回帰を追加し、両方が誤って成功するRED（exit 1）を確認。type gateをdecode直後、候補とFINAL集計の前へ移しGREEN（exit 0）にした。

commandは `go test -count=1 ./src/cmd/aidlc -run '^(TestHookReliabilityOpaqueEvidence|TestHookReliabilityProbeEvidence)$'`。既存の正常・欠落・重複・順序・snapshot・平文保持も成功。境界再実行、gofmt、diffcheckを確認した。製品/assets/外部fixtureを変更せず、全体・race・vet・live・commit・Issue／PR操作は未実施。実測のMESSAGEとFINALは独立reviewで両方正規typeと確認済みという親の報告を受けたが、このsynthetic修正を新たな実測として記録していない。
