# 薄いライフサイクルのreport・approval・state遷移契約

- 日付: 2026-09-03
- 状態: Current for local AI-DLC 2.6.123 snapshot
- 対象: 通常Stageのreport、approval gate、state advance、workflow completion、auditとlock

## 調査結果

本家の`report`はorchestratorが入力を分類し、state toolがguard、audit、state遷移を所有する。通常のgated Stageでは、次の順序を崩さない。

```text
artifact presence
→ summary confirmation
→ pipeline link
→ reviewer receipt
→ gate sensorとartifact fingerprint
→ blocking sensor enforcement
→ STAGE_AWAITING_APPROVAL audit
→ [?] state
→ fresh human decision
→ evidence再検証
→ GATE_APPROVED / STAGE_COMPLETED audit
→ [x] state
→ next Stageまたはworkflow completion
```

marker遷移は`gate-start: [-]→[?]`、`reject: [?]または[-]→[R]`、`revise: [R]→[?]`、`approve: [?]→[x]`である。approve/rejectは、提示された選択肢に対応するfresh human responseを必要とする。

通常advanceはcurrentを`[x]`、次の実行Stageを`[-]`にし、Current Stage、Next Stage、In Progress、Completed、Lifecycle Phase、Phase Progress、Last Updated等を同じstate内容へ更新する。Completedは既存値への加算ではなく更新後の`[x]`件数から再集計する。phase境界では離脱phaseをVerified、到着phaseをActiveにする。

最終StageはStatusをCompleted、Next StageとIn Progressを`none`、最終phaseをVerifiedにする。Current Stageは本家どおり最後のsettled Stageを保持できる。

## 永続化順序

本家は同一recordのlock内でstateを再読込し、新stateを組み立て、auditを先にappendし、その後にtemporary fileとrenameでstateを置換する。audit append失敗ではstateはbyte-identicalのまま、state置換失敗ではauditだけが先行して残り得る。これはaudit/stateを横断するrollback transactionではない。

Goの既存`state.WriteInitial`はtemporary file、排他的作成、short-write検出、close-before-rename、失敗時cleanupを備えるが、project descriptionと初期stateの2 file初期化専用である。更新用writerは機械部分だけを再利用し、初期化契約を変えない。

Goの`state.State`は未知sectionや未知field、Active Agent、Last Updated等のraw byteを保持しないため、typed Stateだけから全体を再renderすると既存文書を損失する。更新は検証済みraw documentの対象箇所だけをstrictにpatchし、patch後を再度`state.Parse`で検証する。

## 段階的な境界

本家の完了gateはreview receipt、sensor、pipeline等を統合する。未実装中にそれらを不要と推測してmarkerを進めると意図的な挙動差分になる。そのため、薄いwalking skeletonは条件を要求しないsyntheticな通常Stageで完走させ、production Stageが未実装能力を要求する場合はfail-closedにする。

## 主要根拠

- `docs/実装_aidlc-workflows/core/tools/aidlc-orchestrate.ts:6776-8205`
- `docs/実装_aidlc-workflows/core/tools/aidlc-state.ts:2566-2644,3149-3160,3299-3370,3910-4386,4596-4654,4750-5494`
- `docs/実装_aidlc-workflows/core/tools/aidlc-lib.ts:18155-18623,21846-21935`
- `docs/実装_aidlc-workflows/tests/integration/t185-stage-artifact-guard.test.ts:615-704`
- `docs/実装_aidlc-workflows/tests/unit/t115.test.ts:746`
- `docs/実装_aidlc-workflows/tests/unit/t125.test.ts:20-90`
- `docs/実装_aidlc-workflows/tests/unit/t232-phase-progress-flip.test.ts:12`

