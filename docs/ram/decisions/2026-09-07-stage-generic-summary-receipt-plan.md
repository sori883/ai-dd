# Ideation Stage共通の質問・summary receipt基盤を作る

- 日付: 2026-09-07
- 状態: Accepted（AI-DLC Go実装ロードマップ第4段階の包括承認内）
- 対象: 固定AI-DLC `2.6.123`のIdeation Stageに共通する質問・summary confirmation authority
- 実装許可: AI-DLC Go実装ロードマップの包括承認と、本タスクでの第4段階着手依頼
- GitHub Issue: #126
- work unit: `ideation-stage-summary-receipt-foundation`

## 背景

33 Stageのread-only対応調査では、Go実装はgraph、context delivery、artifact path、completionの
内部骨格を持つ一方、productionで完走する通常Stageは`intent-capture`だけだった。28のStageが
`summary_confirmation: required`を宣言するが、質問・人間応答・summary receiptの実装は
`ideation/intent-capture/intent-capture-questions.md`へ固定されている。

固定AI-DLC `2.6.123`のsummary確認は、次を一体のauthority契約とする。

1. 現在Stageが宣言したmandatory `*-questions` artifact
2. summary decisionの後に観測した別のfresh `HUMAN_TURN`
3. 完全一致の回答`Looks correct`
4. `SUMMARY_CONFIRMATION_RECORDED`
5. `Stage`、`Checkpoint`、`Questions File`、`Questions SHA-256`、
   `Hash Scope=confirmed-content-v1`のcanonical field
6. receipt後も変わらないquestionsのsemantic digest
7. Stage完了時にはreceipt後の成果物write evidence

今回共通化するのは1から6の内部authority coreである。7の正本は固定版の`PostToolUse`が記録する
`ARTIFACT_CREATED` / `ARTIFACT_UPDATED`であり、現在のGo配布harnessにはまだそのhookがない。
この不足を曖昧な代替receiptで埋めず、非`intent-capture` Stageのproduction gateは閉じたままにする。

## 利用者が得る結果

既存`intent-capture`の質問・summary確認は同じ監査eventと安全境界のまま動作する。後続の
`market-research`、`feasibility`、`scope-definition`等は、Stageごとにsecurity-sensitiveな
人間receipt処理を再実装せず、固定metadataから導出したquestions pathと同じ内部coreを利用できる。

このPRだけでは新しいStageを完了・advance可能にしない。共通部品を完成Stageと誤認させないため、
非`intent-capture`のCodex hidden bridge、completion gate、receiverは引き続きfail closedにする。

## 採用する範囲

- `phase=ideation`、`mode=inline`、`for_each`なし、`summary_confirmation=required`のStageを対象にする。
- freshなstateとcatalogで選ばれた現在Stageのmandatory `produces`から、`<stage>-questions`を
  ちょうど1件だけ解決し、既存artifact engineと同じcanonical record相対pathを使う。
- callerは`Questions File`、digest、audit positionをauthority値として指定できない。
- 現行のrecord lock、identity/root binding、stage epoch、cross-shard ambiguity検出、fresh
  `HUMAN_TURN`、regular non-symlink bounded read、semantic digestを維持する。
- `intent-capture`固有の公開済みinternal APIは互換wrapperとして残し、共通実装へ委譲する。
- 汎用audit appendからauthority eventを発行できない現在の所有権を維持する。
- `resolveCodexStage`の対応範囲は`intent-capture`から広げない。

次は対象外である。

- 非`intent-capture` Stageのgate・receiver・Codex skillの有効化
- `PostToolUse` artifact write hookとartifact freshness完了判定
- reviewer、sensor dispatcher、learnings、pipeline、per-unit、workspace、CodeKB、`produces_kinds`
- CONDITIONAL Stageの公開`report --result skipped`
- 公開CLI/report grammar、永続event/field、state/artifact schemaの変更
- `ApproveGate`が非対応の後続Stageを永続化後に検出する既存問題の修正

最後の遷移原子性問題は重要なproduction gapだが、summary authorityの抽出とは変更failure domainが
異なるため、このwork unitでは挙動を変えず、後続のStage接続前に独立して解消する。

## 対象fileと所有権

単独の`go_tdd_implementer`がGo codeとtestの唯一のwriterとなり、次を所有する。

- 新規候補 `src/internal/audit/stage_receipt.go`
  - Ideation summary receipt contractの解決
  - canonical questions pathの導出
  - Stage共通decision・answer・summaryの記録
  - Stage共通receipt現在性検証
- 新規候補 `src/internal/audit/stage_receipt_test.go`
- `src/internal/audit/intent_capture.go`
  - 既存APIを互換wrapperにする
  - 固定path reader、parser、digestを共通実装へ委譲または移動する
- `src/internal/audit/intent_capture_test.go`および同packageのplatform別安全性test
  - audit event/field、path、error分類、安全なfile readの互換回帰
- 必要な場合だけ`src/cmd/aidlc/codex_stage.go`と`codex_stage_test.go`
  - 現行`intent-capture` actionを共通内部APIへ接続する
  - 非`intent-capture` rejectionを固定する

RAMと索引、GitHub Issue/PRは親エージェントが管理する。`src/internal/orchestrator/gate.go`、
Codex skill/hook、公開CLIは変更しない。

## 順序付きTDD項目

1. **Stage contract解決**

   固定catalogの`intent-capture`、`market-research`、`scope-definition`を含むIdeation metadataから
   mandatory questions artifactとcanonical pathを導出するtestを先に失敗させる。summary不要、
   非inline、per-unit、Ideation外、questionsが0件・複数・optionalだけ、metadata不整合はfail closedにする。

2. **動的pathの安全なreader**

   固定questions readerをengine-resolved path対応へ変更する。`os.Root` confinement、regular
   non-symlink、8 MiB上限、UTF-8、Lstat/open/Stat identityを維持し、absolute/traversal、symlink、
   FIFO、oversize、open前・read中のidentity replacementを拒否するtestを先行する。

3. **通常質問のdecision/answer authority**

   現在Stage、stage epoch、backend-derived fingerprint、decision後のfresh `HUMAN_TURN`、一回答、
   cross-shard orderingを複数Ideation Stageのtable testで固定する。callerがpath、fingerprint、
   audit positionをauthorityとして差し込めないAPIにする。

4. **summary confirmation authority**

   `Looks correct`完全一致、canonical checkpoint/field、`confirmed-content-v1` semantic digest、
   questions変更、重複、古いattempt、hidden/duplicate heading、ambiguous orderingをfail closedにする
   testを追加してから、Stage共通の記録・現在性検証を実装する。

5. **`intent-capture`互換移行**

   既存APIを共通coreのwrapperへ変更する。既存のevent名・field値・questions path・digest・error chainが
   変わらない回帰testと、approve/reject/revise journeyを通す。Codex adapterを変更する場合も、
   `market-research`等がhidden bridgeで拒否され、rejection時にstate/audit/artifactが変わらないことを固定する。

6. **refactorとloop検証**

   重複するintent固有helperを共通名へ整理し、`gofmt`を適用する。loopでは次のtargeted testだけを使う。

   ```text
   go test -count=1 ./src/internal/audit
   go test -count=1 ./src/internal/orchestrator
   go test -count=1 ./src/cmd/aidlc
   go test -tags=integration -count=1 ./src/cmd/aidlc -run 'TestCodexIntentCapture'
   git diff --check
   ```

各項目で、変更前の実装では失敗するrunnableなtestを確認してから最小GREENへ進み、全項目を1つの
work unitとして完了時に返す。

## 受入条件

- 固定Ideation Stage metadataから各Stage固有の
  `ideation/<stage>/<stage>-questions.md`をbackendで一意に導出できる。
- summary必須Stageにmandatory questions artifactが一意にない場合、authority receiptを発行しない。
- receiptはfreshなcurrent Stageと、matching decision後の別のfresh human turnなしに発行できない。
- canonical event名・field名・`confirmed-content-v1`の意味とquestions semantic digestが変わらない。
- symlink、special file、path traversal、oversize、identity replacement、cross-shard ambiguityを
  authorityとして受理しない。
- 既存`intent-capture`のreceipt、approve、reject/revise/approve journeyが回帰しない。
- `market-research`等は内部testでreceipt coreを利用できるが、CLI、hidden bridge、gateでは
  production実行・完了できない。
- 非`intent-capture` rejection時にstate、audit、artifactを変更しない。
- receipt後のartifact write evidenceがないStageを完了済みとして扱わない。
- public help/report grammar、永続schema、`go.mod`、`go.sum`に差分がない。

## リスクと軽減

- **不正なStage/pathをauthorityにする**: fresh state/catalogとmandatory `produces`からbackendが導出し、
  caller指定のpath/digest/positionを受け取らない。record相対path以外を拒否する。
- **`intent-capture` receiptを壊す**: event/field/path/digestの完全互換testと既存journeyで防ぐ。
- **共通coreをproduction-ready Stageと誤認する**: capability guard、Codex resolver、skillを開かず、
  非対応rejectionを受入条件とtestへ残す。
- **TOCTOUまたはcross-shard orderingを弱める**: 現行`os.Root` identity check、record lock、
  epoch/order検証を省略しない。
- **実装範囲が膨張する**: artifact write hook、sensor、review、learnings、`skipped`が必要になった時点で停止する。

永続schema migrationがないため、rollbackはPRのrevertで完了し、既存receiptの変換は不要である。

## final検証

独立reviewでblocking findingがなく、差分が安定した後、親エージェントが対象fileを変更しない
`verification_mode=final`として次を1回実行する。

```text
go test -count=1 ./...
go test -race -count=1 -shuffle=on ./...
go test -tags=integration -race -count=1 -shuffle=on ./src/internal/audit ./src/internal/recordlock ./src/internal/orchestrator
go test -tags=integration -count=1 ./src/cmd/aidlc -run 'TestCodexIntentCapture'
go vet ./...
go mod tidy -diff
gofmt -l src
git diff --check
gopls check <変更したGo file>
gopls vulncheck ./...
```

さらに`CGO_ENABLED=0 go build -trimpath`で`src/cmd/aidlc`をdarwin/linux/windows ×
amd64/arm64の6 targetへcross compileする。公開harnessを広げないため新しいlive-model E2Eは行わず、
既存`intent-capture` fresh sandbox integrationを回帰証拠とする。

## 実装許可の根拠

承認済みAI-DLC Go実装ロードマップの第4段階は「33 Stageをphase単位で実用化する」であり、
この計画は28のsummary-required Stageに共通する最初の内部基盤である。後続RAMで採用した
milestone包括承認により、固定AI-DLC `2.6.123`へ準拠し、新しい意図的差分を採らない実装は
個別計画への追加承認を待たずに進められる。本タスクでもユーザーが第4段階の着手を明示している。

公開API、永続schema、互換性、権限、安全性の新しい重大選択は行わない。summaryの人間receiptは既存の
`UserPromptSubmit`由来`HUMAN_TURN`と用途別audit API、成果物write receiptは固定版の`PostToolUse`由来
eventが正本であり、今回は取得元を変更しない。外部Go moduleも追加しない。したがって本計画は
包括承認枠内で実装可能である。

## 本家との差分

新しい意図的な仕様・挙動差分は採用しない。Go内部APIへの抽出と段階的にproduction gateを閉じておく
ことは移植・実装順序の差であり、未公開のStageを別の意味で完了させない。

## 停止条件

次の場合は実装範囲を広げず、ユーザー確認へ戻る。

- 固定metadataと固定`2.6.123`のquestions artifact選択規則を一意に両立できない。
- 公開`skipped`、新hook、永続event/field、非`intent-capture` bridge/gate解放が必要になる。
- 既存`intent-capture` receiptの互換性を維持できない。
- 外部Go moduleが必要になる。
- test/review failureに複数の結果変更案があり、安全に一意解決できない。

## 根拠

- `docs/ram/research/2026-09-07-stage-phase-production-capability-matrix.md`
- `docs/ram/decisions/2026-09-03-aidlc-implementation-roadmap.md`
- `docs/ram/decisions/2026-09-03-milestone-authorization-and-autonomous-merge.md`
- `docs/ram/decisions/2026-09-06-intent-capture-vertical-milestone.md`
- `docs/ram/decisions/2026-09-06-human-turn-operational-evidence.md`
- `docs/ram/decisions/2026-09-06-intent-capture-upstream-conformance-corrections.md`
- `src/core/aidlc-common/protocols/stage-protocol.md`
- `src/core/aidlc-common/protocols/stage-definition.md`
- `src/core/knowledge/aidlc-shared/audit-format.md`
- `docs/配布_ai-dlc/.codex/tools/data/stage-graph.json`
- `docs/配布_ai-dlc/.codex/hooks/aidlc-write-audit-log.ts`
