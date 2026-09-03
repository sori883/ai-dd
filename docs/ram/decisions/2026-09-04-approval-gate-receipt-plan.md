# 承認ゲート遷移と人間応答の監査記録を接続する実装計画

- 日付: 2026-09-04
- 状態: Accepted
- GitHub Issue: [#79](https://github.com/sori883/ai-dd/issues/79)
- 実装許可: [薄いライフサイクルマイルストーン](2026-09-03-thin-lifecycle-milestone.md)のPR5、[包括承認と自律マージ](2026-09-03-milestone-authorization-and-autonomous-merge.md)内
- 基点: `a36b92284d90ab8751761ecc4b95d85193739bc0`（PR #78）

## 現状、目的、利用者への効果

Go版にはStage完了条件の判定、state文書の局所置換とatomic保存、record単位lock、audit追記がある。
本変更はそれらを接続し、通常Stageを承認待ちにする操作、差戻し、修正後の再提出、承認に使える
新しい人間応答の有無を確認する内部APIを追加する。後続のapproveとadvanceは、この検証を同一lock内で使える。

`HUMAN_TURN`は信頼済み入力経路が発火したことを示す監査イベントであり、承認文言の作者や選択内容を
証明するものではない。選択肢の検証とは分離する。比較対象はリポジトリ固定AI-DLC 2.6.123であり、
最新upstreamとの一致は主張しない。

## 実装範囲と受け入れ条件

- `OpenGate`: `[-]`から`[?]`へ進め、`STAGE_AWAITING_APPROVAL`を追記する。既に`[?]`なら完了条件を
  再検証し、成功してもaudit/stateを変更しない。
- `ReviseGate`: `[R]`から`[?]`へ進め、同イベントに`Details: Re-entering gate after revision`を付ける。
- `RejectGate`: `[?]`または`[-]`から`[R]`へ進め、Revision Countを1増やす。`GATE_REJECTED`、
  `STAGE_REVISING`の順に追記する。成果物不足だけでは差戻しを拒否しない。
- approval検証helper: `[?]`で選択肢とfresh receiptを読み取り専用で検証する。承認event/state更新はPR6で接続する。
- 共通検証はRunning、指定slugとCurrent Stageの一致、対象の一意性、保存suffixの`EXECUTE`、
  他のlive Stageがないこと、graph membership、phase一致。既存`ResolveDirective`の契約は変えない。
- ゲートAPIは`CompletionEvidence`を受け取らない。summary policyが非空（`if-present`含む）、pipeline、
  reviewer、sensor、agent-team、per-unit、CodeKB、workspace sourceなどの未対応能力が必要ならfail-closed。
  対応する通常Stageだけに、zero evidenceで既存completion evaluatorを接続する。
- 呼び出し側の自由な文字列・boolean・timestampからHUMAN_TURNを生成しない。信頼済みadapterの代わりに
  integration fixtureが既存`audit.Append`を使う。production取得元や新しい公開CLI入口は追加しない。

## audit readerと鮮度

当該recordの`audit/*.md`だけを読む最小readerをaudit packageへ置く。各eventのTimestamp、shard identity、
shard内block位置を保持し、ファイル名順を実行順序にしない。authorityに必要なfieldの重複・不正・曖昧さ、
unreadable、symlink、nonregularを許可に変換しない。canonical UTC秒精度Timestampとshard内順序を検証する。
読取前後のRoot/directory/leaf bindingを確認し、FIFO等の差替えで無期限に停止しないよう既存防御を再利用する。

freshness境界は現在gate-openではなくworkflow-globalな最後のresolutionである。readerはwriterのallowlistと
分離し、`GATE_APPROVED`、`GATE_REJECTED`、`QUESTION_ANSWERED`、`SUMMARY_CONFIRMATION_RECORDED`、
`PLAN_APPROVAL_RECORDED`、`Mode=autonomous`の`AUTONOMY_MODE_SET`をresolutionとして認識する。

最新HUMAN_TURNの時刻が最新resolutionより新しければfresh、古ければstale。同秒では、ある最新HUMAN_TURNが
すべての最新resolutionと同一shardかつ後の位置にある場合だけfreshとする。別shardの同秒resolutionを
ファイル名順で解決しない。resolutionがなくてもHUMAN_TURN自体は必須であり、empty/document-only ledgerを
信頼済みreceiptとして扱わない。これは承認済みwalking skeletonの利用条件である。

## 選択肢とfeedback

ECMAScript trim後のapprovalは`Approve`、またはRevision Countが3以上の`Accept as-is`だけを受理する。
rejectは`Request Changes`とnonblank feedbackが必要。cancellation判定は固定本家の限定語彙・全文一致を
移植し、`cancel the standing order`等の実質的文章を拒否しない。

明示的なassistant/model/conductor自己帰属を検出するtripwireも移植する。引用、code block、inline code等の
例示を除外し、未知表現を拡大解釈しない。Go regexpで表せないlookahead/backreferenceは前後境界確認と
引用maskへ分解し、固定本家の回帰vectorで確認する。tripwireを作者証明と説明しない。
検証失敗ではresolutionを追記せず、receiptを消費しない。

## stateとtransaction

`state.ReadDocument`相当で1回の読取りからvalidated Stateと元bytesを取得する。返却bytesは所有権を分離し、
既存Read/Parseの受理範囲を狭めない。Revision Countは全stateの必須fieldへ昇格させず、gate用accessorで
Runtime State内の一意なcanonical非負整数として必要時に検証する。Patch allowlistへRevision Countを追加する。
Last Updatedとmarkerを含めexpected値を確認して局所置換し、未知section/field/comment/BOM/CRLF/終端を保持する。

対象は既存builder生成のcanonical state。欠損・曖昧・壊れたRevision Countの自動修復やlegacy coercionは
導入せず、既存strict patcherの境界で停止する。整数overflowも変更前に拒否する。

transactionは自身で`recordlock.With`を取得し、Guardを外部へ公開しない。

1. identityとproject/record Rootの対応を検証する。
2. lock内でstateを新しく読む。
3. graph、marker、必要なreceipt、完了条件を検証する。
4. replacement全体を`state.Patch`で構築する。
5. `audit.Append`でevent batchを追記する。
6. Root/Guardの対応を保ちながら`state.WriteState`で保存する。
7. lockを解放する。

Appendが非再入leaseを内部取得するため、外側を`Guard.WithLease`で囲まない。PR6は同じlock内のprivate helperで
再利用する。already-awaitingの無更新成功でもRoot binding検証を省略しない。Last Updatedとaudit Timestampは
本家同様に別の内部clockで生成してよい。caller指定時刻をaudit authorityに戻さない。

audit失敗ではstateはbyte-identical。audit成功後のstate保存失敗ではauditが残り得る非対称durabilityを維持する。
rollback、audit削除、自動再承認はしない。

## 単独writerの所有範囲

- `src/internal/audit/read*.go`、必要最小限の`ledger*.go`（読取と既存Root検証共有）
- `src/internal/state/document*.go`、`read*.go`、`patch*.go`（raw読取、accessor、Revision Count）
- `src/internal/orchestrator/gate*.go`、`decision*.go`（transaction、選択肢、test）
- 必要最小限のplatform別安全なread helperと対応testは同じpackage内に置く。
- `docs/architecture.md`、`docs/development.md`、本計画、transition-contract調査RAM、RAM索引

recordlockの再入化、既存completion evaluatorのAPI変更、CLI変更は含めない。実装担当は1名とし、他者の変更を
revertしない。外部Go module/toolは追加しない。

## TDDと検証

loopでは次の観測可能behaviorを1件ずつRED/GREENにする。

1. raw読取・所有権・Revision Count accessor/patch・未知bytes保持。
2. 全resolution種類、同秒両方向、複数shard、Stageをまたぐ鮮度、壊れたauthority情報の拒否。
3. exact choice、count 2/3境界、cancellation、自己帰属、引用除外、未知文言許容。
4. OpenGate、already-awaiting再検証、成果物消失、未対応能力、失敗時無変更。
5. fresh receipt付きReject、成果物不足での差戻し、audit順、count増分、再reject拒否、失敗時非消費。
6. ReviseGate、古いreceiptの再承認拒否、新receiptでの許可。
7. 実Rootのintegration: audit失敗、state保存失敗、identity不一致、同record競合、lock内再読取、deadlock防止。

targeted commandは実装test名に合わせ、state/audit/orchestratorの当該testだけを`go test -count=1 -run ...`で
実行する。実FSは`-tags=integration`を付ける。gofmt適用はloop内でreview前に完了する。

独立reviewは固定base/headで`verification_mode=review`。blocking finding解消後に親がread-only finalを1回開始する。

```sh
go test -count=1 -shuffle=on ./...
go test -race -count=1 -shuffle=on ./...
go test -tags=integration -race -count=1 -shuffle=on ./...
go vet ./...
go vet -tags=integration ./...
gofmt -l src
go mod tidy -diff
git diff --check
```

darwin/linux/windows × amd64/arm64のCLI buildと対象integration test binary compileを一時directoryで行う。
公開CLIを変更しないためPR5固有の配布E2Eは不要。GitHub checks成功後にmerge commitで取り込み、mainとIssue closeを確認する。

## 根拠とリスク

固定snapshotの`aidlc-lib.ts:6180-6310,6320-6455,7745-7870`、`aidlc-state.ts:4694-5494`、
`tests/unit/t261-audit-authority-floor.test.ts`を参照した。既承認のstrict state/suffix authorityを維持し、
新しい意図的差分は採用しない。production receipt sourceは未決であり、公開接続前に確認する。
canonical内部範囲を越える互換性やauthorityの選択が必要になれば停止する。
