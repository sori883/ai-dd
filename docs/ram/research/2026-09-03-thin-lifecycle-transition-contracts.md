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

## PR6接続時の確認記録（2026-09-04）

Issue #81の実装では、上記の順序を一つのrecord lock内の二段階transactionとして接続した。第一段階は
`GATE_APPROVED`、`STAGE_COMPLETED`をauditへ先に追記してからstateを保存し、保存済みstateを同じlock内で読み直して
次Stageまたは終端を導出する。第二段階もauditを先に追記してstateを保存する。第一state保存後の失敗はrollback・再承認を
行わず、承認済み中間stateと最終遷移未完了をresultで区別する。

保存済みStage suffixとgraph順序をrouting authorityとして維持し、scope gridやcallerのNext Stageで補完しない。未知scope／row、
前方pending、複数live、未対応phase・capabilityは終端と推測せずfail-closedにする。承認receiptは保持中のidentity-bound Guardと
Rootからfreshにauditを読み直して判定し、`HUMAN_TURN`はこのAPIから生成しない。

未記録revision backstopは補完経路を実装せず、対象artifactの宣言filename（`traceability`、build/load test-resultsの例外を含む）を
既存presence判定と共有するcanonical helperで照合する。同秒別shardの順序候補が判定を左右する場合は、候補列挙を256件までに
制限し、上限超過または候補間の判定相違をunsupportedとして承認audit前に停止する。これは新しいauthorityや順序規則を導入せず、
固定snapshotの不確実性を安全側へ倒す実装詳細である。

この接続で編集した責務は、内部orchestratorのapprove／advance／revision backstop、stateのcanonical accessor／patch allowlist、
artifact filename共有、および対応するunit／integration testと文書に限定した。公開CLI、registry同期、recordlockの再入化、外部Go
module／toolは追加していない。audit/stateを跨ぐtransactionの電源断耐性、productionのtrusted receipt取得元、registry status同期は
引き続き後続境界であり、このPRの完了条件には含めない。

## PR7接続時の確認記録（2026-09-04）

Issue #83の内部`Next`／`Report`は、PR6のgate／approve transactionを新しいlock wrapperで包まずに呼び出す薄いfacadeとして接続した。
Nextは自身のrecord lock内でRoot・Guard・identity bindingを前後に確認し、保存済みstateを一度fresh readして`run-stage`、
`awaiting-approval`、`revising`、`workflow-complete`へ分類する。Reportは必須Current stageとSlugの完全一致を確認し、明示されたkindだけを
一度委譲して、下位操作が返すpartial result／errorを保持する。終端Nextは壊れたaudit／graph／artifact本文に依存せず、stateとbindingだけで判定する。

integration fixtureは`StartIntent`が作る実filesystem recordを使用し、artifactと`HUMAN_TURN`だけを製品外のfixture準備として配置した。
保存suffixと注入graphのSKIP差異、旧receipt再利用、reject/revise、phase境界、unknown bytes、registry未同期、無関係record、
terminal後のaudit／graph／artifact欠落を確認した。`Directive.Stage`はSensors／ProducesKindsをdeep copyし、caller間のslice共有を防ぐ。
CI quality jobは既存のpin・matrix・権限を変更せず、audit／recordlock／orchestratorのintegrationをrace・shuffle付きで追加実行する。
