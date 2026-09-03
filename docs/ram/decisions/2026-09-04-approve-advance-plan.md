# 承認から次Stage・workflow完了までを接続する実装計画

- 日付: 2026-09-04
- 状態: Accepted（薄いライフサイクル包括承認内）
- Issue: [#81](https://github.com/sori883/ai-dd/issues/81)
- 基点: `3d8275293304e25dcb53a6780d43d57a6556845b`（PR #80）
- 実装許可: [薄いライフサイクルマイルストーン](2026-09-03-thin-lifecycle-milestone.md) PR6、[包括承認](2026-09-03-milestone-authorization-and-autonomous-merge.md)

## 現状と目的

Go版には開始、完了条件判定、承認待ち・差戻し・再提出、信頼済み人間応答の監査読取りがある。
本変更は承認を受けた通常Stageを完了し、次Stageを開始するかworkflowを完了する内部transactionを追加する。
Stageは作業工程、recordはIntentのstateとauditを持つ保存単位、transactionは同じrecordの操作を直列化する処理である。
固定AI-DLC 2.6.123の確認済み範囲を参照し、最新upstreamとの一致は主張しない。

## 契約と受け入れ条件

内部Approveは既存GateInput相当のbound Roots・identity・graphと明示的Stage slug・choiceを受ける。
自身でrecordlock.Withを取得し、保持中Guardを外部へ返さない。呼出側のboolean、timestamp、手作りaudit列を権限にしない。
同一lock内で以下を順に行う。

1. bound Rootsを確認し、stateとauditをfresh read。Running、指定current、唯一の[?] EXECUTE、graph/phase、
   exact approval choice、Revision Count、新しいHUMAN_TURN、対応能力、必要成果物、既知scopeを検証。
   前回拒否後の新receiptが必要であり、選択文字列は人間の作者証明ではない。
2. 下記の未記録revision backstopを判定。自動補完が必要または順序不明なら、最初の承認監査より前に停止。
3. currentを[x]、Completedを実[x]件数へ再集計、Last Updated、Last Completed Stageをpatch。
   GATE_APPROVED（Stage, User Input）とSTAGE_COMPLETED（Stage, Details: Stage <name> approved by gate）を順に追記し、
   第一stateをatomic保存する。
4. 同じlock内で第一stateを読み直し、保存済みsuffixとgraph順序から次Stageを導出。
   次があれば次Stage開始、なければ整合性を確認してworkflow完了のpatchを構築する。
5. 第二段階のauditを先に追記し、第二stateをatomic保存する。STAGE_COMPLETEDを重複追記しない。

二段階を一回の全event追記・state保存にまとめない。audit失敗はその段階のstateを変えない。
第一state成功後に次の処理が失敗したら、承認済み中間stateを保持し、結果型で承認保存済みと最終遷移未完了を区別する。
第一audit成功後の第一state失敗もerrorを隠さない。rollback、audit削除、再承認、自動回復は行わない。
caller-owned Rootは閉じない。append自身がleaseを使うため、外側から非再入WithLeaseで包まない。

## Routingと更新項目

scopeはCatalog.Scopeで存在を確認する。routing actionはscope-gridではなく保存済みEXECUTE/SKIPがauthority。
graph順序でcurrentより後の[x]/[S]とSKIPを飛ばし、次の未完EXECUTEを選ぶ。保存Next Stageやcaller指定nextを採用しない。
未知・欠落row、graph不一致、currentより前の未完EXECUTE、複数live、不正なmarker/action等を終端と推測しない。
次の未完Stageが未対応phase/能力なら、STAGE_STARTEDや[-]保存より前にunsupportedとする。
第一承認保存の後にこの停止が起き得るので、結果に中間保存を残す。

次Stageあり:
- 次Stageを[-]、Current Stage/In Progressを次slug、Lifecycle Phaseを次phaseの大文字、Active Agentをlead_agent。
- Next Stageはさらに次の未完EXECUTEまたはnone。Status Running、Last Completed Stageは承認slug、
  Next ActionはExecute <next.Name>、Completedは再集計、Last Updatedは内部clock。
- 同phaseはSTAGE_STARTED（Stage, Agent）のみを追加。
- phase境界はPHASE_COMPLETED（From phase, To phase, Stages completed）、PHASE_VERIFIED（Phase boundary: <from> → <to>）、
  PHASE_STARTED（Phase, Scope）、STAGE_STARTEDの順。Phase Progressは離脱phase Verified、到着phase Active。

終端:
- Status Completed、In Progress/Next Stage none、Next Action Workflow complete、最終phase Verified。
- Completed/Last Completed Stage/Last Updatedを整合させ、Current Stage/Lifecycle Phase/Active Agentは保持。
- PHASE_COMPLETED（From phase, To phase: (end), Stages completed）、
  PHASE_VERIFIED（Phase boundary: <phase> → end）、WORKFLOW_COMPLETED（Scope, Details: Scope: <scope>, <count> stages completed）。
- Stage移動や終端でRevision Countをリセットしない。

## 未記録revision backstop

固定sourceは、gate提示後の人間応答を受けて成果物を変更したのにreject/reviseが記録されていない場合に補完する。
本内部milestoneは補完経路を実装しないため、その条件を検出したら承認前にunsupportedで停止する。
fresh audit readerのEvent/Timestamp/Shard/Position/Fields（Stage, File, Recovered）を利用し、reader APIは拡張しない。

- 対象slugの最新organic STAGE_AWAITING_APPROVAL（Recovered != true）またはそれより後のSTAGE_STARTEDをanchorとする。
- anchorなし、anchor後に対象slugのGATE_REJECTEDあり、anchor後HUMAN_TURNなしなら補完不要。
- 最初のHUMAN_TURNはworkflow全体から選ぶ。
- anchorがSTAGE_STARTEDなら、anchorから最初humanの間に宣言成果物writeが必要。gate-openなら不要。
- 最初humanの後に宣言成果物へのARTIFACT_CREATED/ARTIFACT_UPDATEDがあれば補完必要。
- Fileのbackslashをslashへ正規化し、/<slug>/<artifactFilename(Produces)> suffixで一致判定。
  OptionalProduces・eventのStage field・mtime/hashは使わない。既存filename例外を再利用する。
- 異なるTimestamp/同一shard位置で順序を確定する。同秒別shardの順序に判定が依存する場合はunsupported。
  固定sourceの連結buffer filename順を安全性の証拠としない。新たなtie解決・補完の実装は追加判断gate。

## state patchと所有範囲

canonical文書に限定し、欠損field挿入やlegacy修復は行わない。元bytesとexpected値を確認して局所置換する。
allowlistへProject InformationのActive Agent、Session Resume PointのLast Completed Stage/Next Actionを追加。
Next Actionだけは内部空白を持つ安全な単行文字列を許容する専用kindとし、control/newlineを拒否、他scalarを緩めない。
canonical field読取りaccessorは必要最小限とし既存Parse受理範囲を変えない。

単独writer:
- src/internal/orchestrator/approve.go、advance.go（必要時）、revision_backstop.goと対応unit/integration tests
- gate.go/decision.go private helperの最小共有
- src/internal/state/document.go、patch.goと対応tests
- docs/architecture.md、docs/development.md、本計画、transition-contract調査RAM、RAM索引

既存public CLI、DataFS/ScopesFSのproduction選択、trusted human取得元、workspace registry updaterは変更しない。
workflow完了はstate/audit内の完了であり、Intent一覧registry status同期を意味しない。
未完Initialization/全Construction、summary（if-present含む）/pipeline/reviewer/sensor/per-unit/CodeKB/workspace要求は通さない。
usage rollupやValidity Basis補助情報は完成扱いしない。外部Go module/tool追加なし。

## TDDと検証

loopは1 behaviorずつRED/GREEN、対象のみ実行:
1. canonical追加field/accessorとNext Action単行境界、byte保持、missing/duplicate拒否。
2. 次Stage導出、SKIP/settled/保存suffix優先、未知scope/row/前方pending拒否。
3. fresh receipt/exact choice/capability/artifact guard、拒否時無変更。
4. revision backstop全条件（organic/start/recovered/restart/後発reject/File照合/filename例外/crossshard tie）。
5. 同phase二段階遷移、event順/field、回数保持。
6. phase境界と終端、metadata/件数/未知bytes。
7. 各audit/state保存点のfailure、部分結果、同record競合、再読取、重複承認拒否、caller Root ownership。

targeted: go test -count=1 -run '^(TestApprove|TestAdvance|TestRevisionBackstop)' ./src/internal/orchestrator
stateは対応accessor/patchのみ。実FSは-tags=integration。gofmt適用をloopで完了する。
独立reviewは固定base/head、verification_mode=review。blocking解消後親がread-only finalを一回:
go test -count=1 -shuffle=on ./...
go test -race -count=1 -shuffle=on ./...
go test -tags=integration -race -count=1 -shuffle=on ./...
go vet ./...
go vet -tags=integration ./...
gofmt -l src
go mod tidy -diff
git diff --check
darwin/linux/windows × amd64/arm64 CLI buildと対象state/orchestrator integration test binary compile。
内部API変更なので配布CLI lifecycle E2Eではない。current GitHub checks成功後merge commit、main反映とIssue close。

## 根拠・残余risk

固定aidlc-state.ts:3898–4155（advance）,4257–4409（complete）,4842–5137（approve）,412–561（backstop）、
aidlc-lib.ts:16569（field）,21852–21903（next）、t232-phase-progress-flip、t205-gate-revision-backstopを確認。
保持する保存suffix authorityとstrict canonical境界は既承認。新しい恒久的な意図的差分は採用しない。
production接続、回復、registry同期を決める場合は別承認gate。失敗時に残る中間state/auditは自動修復しない。

