# M3実装結果 — 工程・担当管理のテスト整理

Issue #208、`work_unit_id=test-reduction-m3`、`verification_mode=loop`。[計画](test-reduction-m3-plan.md)と[承認RAM](../ram/decisions/2026-09-14-test-reduction-approved.md)に従い、39候補を7sliceで処置した。開始HEADは `dbfd8d9084a1d9e474f0680c1218aa771ec6210e`。作業rootは `/Users/const/sori883/ai-dd-release`、開始時差分なし、writerは1名。製品Go・外部module・CIは変更していない。

## 候補別の処置と生存保証

Qは後述のexact commandを表す。表の生存保証は移動・削減後のtestが所有する。

| ID | 処置 | 生存保証 | 実行証拠 |
| --- | --- | --- | --- |
| F02 | flow/verification_results_test.go:11 `TestVerificationResults` の `self cycle` 行と成功条件の同名分岐を削除（実施済み） | 同関数 `valid` の成功と「result作成でもcode SHA不変」を残す。入力も処理も同じ行 | Q4a、exit 0 |
| F03 | flow/boundary_results_test.go:32 `TestEndSensorSharedCommandRequiresEachUnitResult` を削除（実施済み） | `TestVerificationResults/valid` と `second unit missing` が同commandのUnit a/b双方必須を所有。`resultPairFixture` は他callerが使うので保持 | Q4a、exit 0 |
| F04 | flow/verification_cli_test.go:64 `TestVerificationCLIAssignmentSchema` を削除（実施済み） | assignment/store_test.go `TestRegistry/explicit initialization and aliases` に初期schema/identityを残す。flow固有処理を通らない定数検査 | Q3a,Q3b、exit 0 |
| F06 | flow/selected_documents_test.go:166 `TestSelectedDocumentsIntegrationFeature` を削除（実施済み） | flow/codekb_test.go `TestCodeKBIntegrationFeature` のStepID付き宣言・正配置成功/誤配置拒否。旧側はStepIDなし宣言が無視される | Q4b、exit 0 |
| F07 | flow/codekb_test.go:51 `TestCodeKBSharedDocuments` を削除（実施済み） | `TestSelectedDocumentsSharedPaths` の実移動・metadata選択と `TestEndSensorIntegrationDocuments`。旧側は移動元=先 | Q4b、exit 0 |
| F08 | flow/rule_skill_separation_test.go:91 `TestUpstreamSkillInitialization` を削除（実施済み） | `TestExecutionPlanBootstrap` のCreate/Begin/retry/既存file保全、`TestRuleSkillSeparationRule/default` の実配置 | Q3a、exit 0 |
| F09 | workflow/definition_test.go:99 `TestDefinitionAcceptedInputMustPrecedeStage` を削除（実施済み） | workflow `TestDocumentDeclaration/input path`、flow `TestExecutionPlanEvidenceSelectedInputs` / `...ArtifactSelectedOrder`。旧path入力の先行拒否に隠れた固定catalog順の擬似契約を除く | Q5a,Q5b、exit 0 |
| F10 | flow/execution_plan_test.go:62 `TestExecutionPlanSchemaChoices` 全体を削除（実施済み） | 正常は `TestExecutionPlanApprovalSameAnswer`。省略拒否は `TestExecutionPlanDraftRejects` に `empty omission reason`、`duplicate omission`、`selected and omitted` を追加し、既存 `choice missing` と合わせ4意味を所有。詳細は下記 | Q2a、exit 0 |
| F11 | flow/boundary_store_test.go:46 `TestBoundaryStoreVersions` を正常Stage/StepID起点の小tableへ書換え（実施済み） | 正常対照、Stage不一致、入力path逸脱、SHA不正、Sourcesのruntime path、入力重複を同関数で検査。Step誤結合は `TestExecutionPlanEvidenceBindings/entry`、未知acceptanceは `...AcceptedRuns` へ限定 | Q2b、exit 0 |
| F12 | flow/work_log_okf_test.go:203 `TestOKFWorkLogRecoveryPendingValidation` を削除（実施済み） | flow/execution_history_test.go `TestExecutionPlanReopenHistoryPendingBinding` に `LogHash=""` と `LogAfterHash="bad"` の2行を追加。別fieldのguardを片方ずつ、正常draft/Step/Plan結合のまま検査。missing/bad直積4行は不要 | Q2b、exit 0 |
| F13 | flow/sensor_test.go:92 `TestFlowSensorUnitGraph` の不完全Unitを使う3fixtureとSave失敗を成功扱いする分岐を除去（実施済み） | 同関数で完全な2Unitの正常/未知依存/循環の直接 `unitPlanProblems` 小tableへ。全Sensor接続代表も1本残す（下記）。待ち続ける依存cycleの事故を維持 | Q2c、exit 0 |
| F14 | flow/execution_plan_test.go:233 `TestExecutionPlanDraftRejects/duplicate` を削除（実施済み） | 同一requestの `stage swap` を残す。保存された重複IDは `TestExecutionPlanSchemaState/duplicate id` | Q2a、exit 0 |
| F15 | flow/reassign_test.go:312 `TestFlowUnitReassignLiteralPaths` のtracked/untracked軸を除去（実施済み） | F16と同時に関数全体を削除。Git index状態は製品が読まない。文字を扱う保証はF16の実bytes照合へ | Q1b、exit 0 |
| F16 | flow/reassign_test.go `TestFlowUnitResultLiteralPaths` を複合名1件へ修正、旧ReassignLiteralPathsを削除（実施済み） | 同関数でliteral pathをScope **とUnit.VerificationPaths** に明示し、結果作成後のbytes改変拒否→復元成功を確認。基本FS/hashは `TestVerificationDigest` を残す | Q1b、exit 0 |
| F17 | flow/execution_approval_test.go:325 `TestExecutionPlanApprovalFinishDropsPreviousUnits` を削除（実施済み） | `TestExecutionPlanApprovalBootstrapFinish` へ旧Units設定+SaveとFinish後len=0を移す。未Begin/未承認拒否から完了まで1往復を所有 | Q3a、exit 0 |
| F18 | flow/execution_plan_test.go の `TestExecutionPlanEvidenceInitializationEnd`、`...InitialSensor`、`...InitialReview`、`...StartGateStep` を削除（実施済み） | Bootstrapへstart gate StepID、Check pass/StepIDを、BootstrapFinishへReview assign/acceptのStepIDとpassを移す。End成功は実Finishで維持 | Q3a、exit 0 |
| F19 | `TestExecutionPlanBootstrapMissingConfiguration` のRule欠損行を削除し、CLI/OKF skill欠損pathを追加。`TestRuleSkillSeparationHookMissingCLI`、`TestOKFSkillInitialization` を削除（実施済み） | 欠損tableはhooks.json、aidlc、aidlc-cli、okf-agent-memoryの4paths。Rule欠損は `TestRuleSkillSeparationRule/missing`。異なる配置漏れ4種類は保持 | Q3a、exit 0 |
| F20 | flow/verification_gates_test.go:87 `TestVerificationGatesIndependence` を削除（実施済み） | `TestVerificationGates/none` の正常assign直前にsame root+same session拒否を移す。別root同sessionの `TestFlowReviewIdentityTarget` とは別の保証 | Q4a、exit 0 |
| F21 | `TestBoundaryStoreSchema` を削除。`TestExecutionPlanSchemaState/old schema`、`TestDefinitionBindingMalformedState/old schema`、documents/reopen_log/verification_cliの単純schema数字assertを除去（実施済み） | flow/store_test.go `TestFlowStoreWireSchema` に未対応schemaのRead拒否とfile bytes不変を1行追加。Create/CASは `TestFlowStoreCreateCAS`、現wire形式はWireSchemaを正本とする | Q3a、exit 0 |
| F22 | flow `TestFlowStoreRejectsCorruptAndIsolates` の末尾duplicate schema keyのみ削除。assignment `TestRegistry` のbroken/unknown version/unknown field旧3行を削除（実施済み） | flow duplicate keyはWireSchema。assignment `TestRegistryRejectsDamagedRecords` へ正常Registryの不正schema、rawへの未知field、壊れたJSON代表1行を統合。ID/path/Space/root分離は保持 | Q3a,Q3b、exit 0 |
| F23 | flow/verification_cli_test.go `TestVerificationCLI` 末尾Marshal→旧Git field5名のContainsを削除（実施済み） | wire形式はWireSchema。Hashのread-only、unknown Unit、登録root、VerificationPathsの独自検査は同関数に保持 | Q3a、exit 0 |
| F24 | flow/documents_test.go `TestIntentDocumentsLegacyFieldsRejected` 全体、workflow/declaration_test.go `TestDocumentDeclaration/old refs` 行を削除（実施済み） | flow WireSchemaのunknown fieldとworkflow `TestDefinitionRejectsInvalid/unknown yaml`。独自metadata `unknown match` 等は残す。旧名だけのdecoder再確認を除く | Q3a,Q3c、exit 0 |
| F25 | workflow/execution_plan_test.go `TestExecutionPlanSchemaCatalog` と空になったfileを削除（実施済み） | `TestDefinitionRejectsInvalid` に `missing required prefix`（空配列）1行追加。正常6stageと順序拒否、unknown graphは既存DefinitionValid/RejectsInvalid。len!=2の上下直積は増やさない | Q5a、exit 0 |
| F26 | workflow/definition_test.go `TestDefinitionRejectsEmptyOrUnknownAgent` を削除（実施済み） | workflow/stage_planner_test.go `TestStagePlannerRole` をplanner正常、wrong agent、unknown role、missing roleの4行へ。未知roleは既存1行、role無しはagent付き1行のみ | Q5a、exit 0 |
| F27 | flow/stage_planner_test.go `TestStagePlannerProcedure` の日本語5語Containsだけ削除（実施済み） | 同関数の実Procedure viewの4つのrole/agent/order結合は維持。workflow loader正常だけではflow返却時の取り違えを保証しないため全関数削除はしない | Q5b、exit 0 |
| F28 | flow `TestEndSensorResults` を正常+不正exit代表の2caseへ。`TestExecutionPlanEvidenceResultRun` を削除（実施済み） | `TestVerificationResults` へmissing exit、unknown JSON field、empty step、invalid SHA formatを追加。既存step/sha/command/exit各意味は同table。EndSensorは拒否がGate failに届く接続を保持。`TestEndSensorIntegrationRequiresCurrentSHA` の実code改変と更新後成功は保持 | Q4a、exit 0 |
| F29 | flow/history_review_test.go `TestExecutionPlanReviewHistoryMutation` を15→7case（実施済み） | configureでmissing/hash/old head/invalid record/invalid UTF8の5種、pauseとsourceはhash代表各1。各caseのwrites=0、state bytes不変、approval source未作成を保持 | Q4c、exit 0 |
| F30 | flow/boundary_snapshot_test.go `TestBoundaryTransitionCollectorSnapshot` のremove=falseを削除し、remove=trueを単一caseへ（実施済み） | 変更後cacheは `TestSelectedDocumentsCollectorSnapshot` がselected/material/hashも所有。削除後再読込は残る単一CollectorSnapshotが所有。`TestBoundaryTransitionUsesOneSensorSnapshot` はFinish時の独自保証として保持 | Q4b、exit 0 |
| F31 | flow/reassign_test.go `TestFlowUnitReassignPendingBlocksStateUpdates` の後半later pause/resume/reassign/新runを3caseから削除（実施済み） | `TestFlowUnitReassignStoppedAndIdentity` の後日再割当で新runを維持。tableは3入口の拒否、state不変、他runtime未作成、同request同run復旧まで残す | Q6a、exit 0 |
| F32 | flow/reassign_test.go `TestFlowUnitReassignScopeAndDependency` の1行tableと死んだscope/Git elseを除去（実施済み） | 関数はdependency拒否単一caseとして残す。scope衝突は `TestFlowUnitReassignMissingRuntimeScope`。F38と同時に先行実施 | Q1b,Q6a、exit 0 |
| F33 | flow/reopen_log_test.go `TestReopenLogFailures` を削除。`TestOKFWorkLogRecovery` のfresh/existing直積を整理（実施済み） | Recoveryはfresh/base、existing/pending、existing/logの3case。final保存点は `TestExecutionPlanReopenHistoryLogRetry` のpublic Capture/DecidePlan往復へ集約。escape、操作拒否、旧expect、timestamp、既存本文保全を下記対応どおり移す | Q6b、exit 0 |
| F34 | `TestReopenLogChangedHistoryRejected` 全体と `TestOKFWorkLogRecoveryRejects` のmissing/changed pending旧行を整理（実施済み） | RecoveryRejectsへ `before log changed`、`after log deleted` の2独立case。前者はlog write失敗、後者はfinal state失敗後に改変/削除。両方state不変、改変bytes/欠損不変を保持。`TestReopenLogFreshDeletionRequiresRestore` は維持 | Q6b、exit 0 |
| F35 | `TestOKFWorkLogRecoveryRejects/metadata overflow` を削除（実施済み） | `TestReopenLogCapacity/overflow`、fits、fresh encoded reasonを残す。encoded sizeでstate/logを壊さない境界を一箇所にする | Q6b、exit 0 |
| F36 | `TestOKFWorkLogRecovery` のPendingReopen Marshal→map→LogAfterHash長64を削除（実施済み） | PendingReopenのtyped存在/結合はF12、復旧後の実OKF bytes/hash/時刻はF33。different request拒否はexisting/logで残す。serializer自己再解釈を反復しない | Q6b、exit 0 |
| F37 | assignment `TestReservation` のalias、fresh retry、別root成功を除去。`TestAssignmentRecoveryInitRetry` を削除（実施済み） | directory_test.go `TestDirectoryAssignment` に既存同root/親/子/alias/別root/retryを残し、初回ID/TaskName/Statusのassertも集約。Init retryは `TestRegistry/explicit initialization and aliases` へ。Reservationはchanged request、Space/Intent/owner衝突、release後retry非復活/再利用を残す | Q6c、exit 0 |
| F38 | 通常fixtureとその依存callerのGit init/commit/worktree/merge/rev-parse/ls-filesを除去（実施済み） | registryFixtureは通常3root、worker/reviewerは別通常root。unitの未転記で拒否→実bytes転記で統合、reviewの別root不一致拒否を残す。後述の全callerを同sliceで置換 | Q1a,Q1b、exit 0 |
| F39 | `TestVerificationResultsStageValidity` のinitialization/discovery/planningを代表planning1値へ、1行loopを除去（実施済み） | 非検証stage拒否1件を同関数で保持。正常TDDはVerificationResults、正常integrationはIntegrationRequiresCurrentSHA。将来stage別例外分岐を導入時はその変更のtestを追加する | Q7a、exit 0 |
| F40 | workflow `TestRuleSkillSeparationRuleReference/invalid metadata`（空title）だけ削除（実施済み） | title/description/status/tags/intent_idの各nil guardは軽く独立したfield落としを捕捉するため全て維持。path/type/version/match/count/role/accepted_atも維持 | Q7b、exit 0 |
| F41 | 旧容量2testを通常保持。stress/new Boundaryは非採用（実施済み） | 実Reserve/Release/PreSpawn/PostSpawnで全受付済み応答とescaped releaseを保持 | Q7c fallback、exit 0 |

## 重要な境界

- F16: 複合literal名をScopeとVerificationPathsへ指定。正常requestを一度作成し、bytes改変後は同requestを直接s.Unitへ渡して拒否、復元後に同requestで成功。改変後のSHA再生成は行わない。
- F38: registry/worker/reviewerは別directory。a/bをReadFile/WriteFileで統合rootへ転記し、未転記拒否も保持。reviewコピーは`.git`と`aidlc`全体を除外し、知識・状態・runtimeを複製しない。正常managed resultから予約を除いたRegistryのRead成功、result拒否、state/runtime不変を確認した。flowGitと全callerを削除した。
- F11/F13: 正常Stage/StepID付きentryのpersist成功、完全Unit graph正常成功を対照に置いた。Sensorは正常planningのpassから依存だけを変更し、unknown dependencyを含むfailまで確認する。Save失敗を成功扱いしない。
- F33/F34: base/pending/log/finalをpathとtyped PendingReopenで注入。注入error・pending・実log hashで到達点を確認する。復旧後の旧本文・escape・reason一度・時刻・旧expect拒否、public CaptureApproval/DecidePlanの同承認retryを保持した。before log changedとafter log deletedは独立rootでstate/log不変または欠損継続を確認する。
- F40: title/description/status/tags/intent_idの独立guardを全て保持。空titleの二重検査だけを削除した。

## F41判断

通常保持fallbackを採用した。合法seedを複製してRead成功を得るだけでは、受付直後は保存可能で最大応答またはescaped release後だけ容量を超えるという前提を証明できない。root長・識別子長・JSON増幅を含む安全側と危険側の小fixtureを本変更では確立していない。容量計算の写しや大量探索を追加せず、旧2testの実受付・全応答確定・解放を通常suiteに残した。stress/new Boundaryは非採用。旧2testの初回実行はexit 0（3.174秒）。承認済みfallbackの完了分岐であり、容量検査を隠していない。

## 検証証拠

各sliceの指定targetedは変更前ALREADY_GREEN、assert移動後および削減後もexit 0。製品挙動変更のない削減のため人工REDはない。F41は変更せず通常保持し、旧2testの初回成功をALREADY_GREENとした。phase・exact command・exit・出力は `/tmp/m3-evidence.json` に保存。文書生成時の一時Python入力のencoding errorはtest failureやREDではなく、UTF-8指定で修復した。

### Q1a

```sh
go test -count=1 ./src/internal/assignment -run '^Test(Registry|DirectoryAssignment|DirectoryAssignmentConcurrentAndSaveFailure|Reservation|ReservationProcess|ReservationRequestIDAcrossOperations|ReservationReplacementSaveFailure|AssignmentDispatch|AssignmentRecovery)$'
```

### Q1b

```sh
go test -count=1 ./src/internal/flow -run '^Test(FlowUnit.*|UnitWithoutGit.*|UnitAssignment.*|FlowReview.*|BoundaryReviewRequiresEndPass|ExecutionPlanApproval.*|VerificationGates.*|VerificationDigest|ProcedureBoundaryAcceptedTDDOutput|CodeKBIntegrationFeature|EndSensorIntegrationDocuments|SelectedDocumentsIntegrationDoesNotFreezeMaterials|BoundaryTransitionUsesOneSensorSnapshot|ExecutionPlanEvidence(SensorTarget|OptionalPlan|ArtifactSelectedOrder))$'
```

### Q2a

```sh
go test -count=1 ./src/internal/flow -run '^Test(ExecutionPlanDraft|ExecutionPlanDraftRejects|ExecutionPlanSchemaState|ExecutionPlanApprovalSameAnswer)$'
```

### Q2b

```sh
go test -count=1 ./src/internal/flow -run '^Test(BoundaryStoreVersions|ExecutionPlanEvidenceBindings|ExecutionPlanEvidenceAcceptedRuns|ExecutionPlanReopenHistoryPendingBinding)$'
```

### Q2c

```sh
go test -count=1 ./src/internal/flow -run '^Test(FlowSensorUnitGraph|FlowUnitClaimDependencyAndOverlap|UnitWithoutGit)$'
```

### Q3a

```sh
go test -count=1 ./src/internal/flow -run '^Test(ExecutionPlanBootstrap|ExecutionPlanBootstrapMissingConfiguration|ExecutionPlanApprovalBootstrapFinish|RuleSkillSeparationRule|FlowStoreCreateCAS|FlowStoreRejectsCorruptAndIsolates|FlowStoreWireSchema|FlowStoreFailurePreservesState|ExecutionPlanSchemaState|DefinitionBindingMalformedState|DefinitionBindingDrift|IntentDocumentsRegistration|IntentDocumentsEntryAndAccepted|VerificationCLI)$'
```

### Q3b

```sh
go test -count=1 ./src/internal/assignment -run '^Test(Registry|RegistryRejectsDamagedRecords)$'
```

### Q3c

```sh
go test -count=1 ./src/internal/workflow -run '^Test(DocumentDeclaration|DefinitionRejectsInvalid)$'
```

### Q4a

```sh
go test -count=1 ./src/internal/flow -run '^Test(VerificationResults|VerificationResultsUnit|EndSensorResults|EndSensorIntegrationRequiresCurrentSHA|VerificationGates|FlowReviewIdentityTarget)$'
```

### Q4b

```sh
go test -count=1 ./src/internal/flow -run '^Test(CodeKBIntegrationFeature|SelectedDocumentsSharedPaths|EndSensorIntegrationDocuments|SelectedDocumentsCollectorSnapshot|BoundaryTransitionCollectorSnapshot|BoundaryTransitionUsesOneSensorSnapshot)$'
```

### Q4c

```sh
go test -count=1 ./src/internal/flow -run '^Test(ExecutionPlanReviewHistoryMutation|ExecutionPlanReviewHistoryRevisionOrder|ExecutionPlanReviewHistoryConfigKeepsHead)$'
```

### Q5a

```sh
go test -count=1 ./src/internal/workflow -run '^Test(DefinitionValid|DefinitionRejectsInvalid|DocumentDeclaration|StagePlannerRole)$'
```

### Q5b

```sh
go test -count=1 ./src/internal/flow -run '^Test(StagePlannerProcedure|ExecutionPlanEvidenceSelectedInputs|ExecutionPlanEvidenceArtifactSelectedOrder|ExecutionPlanApprovalSameAnswer)$'
```

### Q6a

```sh
go test -count=1 ./src/internal/flow -run '^Test(FlowUnitReassignStoppedAndIdentity|FlowUnitReassignStateFailureRetry|FlowUnitReassignPendingBlocksStateUpdates|FlowUnitReassignMissingRuntimeScope|FlowUnitReassignScopeAndDependency)$'
```

### Q6b

```sh
go test -count=1 ./src/internal/flow -run '^Test(OKFWorkLogRecovery|OKFWorkLogRecoveryRejects|OKFWorkLogRecoveryFIFO|ExecutionPlanReopenHistoryLogRetry|ExecutionPlanReopenHistoryPendingBinding|ReopenLogFreshDeletionRequiresRestore|ReopenLogCapacity)$'
```

### Q6c

```sh
go test -count=1 ./src/internal/assignment -run '^Test(DirectoryAssignment|DirectoryAssignmentConcurrentAndSaveFailure|Reservation|ReservationProcess|Registry|AssignmentRecovery)$'
```

### Q7a

```sh
go test -count=1 ./src/internal/flow -run '^Test(VerificationResultsStageValidity|VerificationResults|EndSensorIntegrationRequiresCurrentSHA)$'
```

### Q7b

```sh
go test -count=1 ./src/internal/workflow -run '^TestRuleSkillSeparationRuleReference$'
```

### Q7c fallback

```sh
go test -count=1 ./src/internal/assignment -run '^TestAssignmentRecovery(Capacity|EscapedReleaseCapacity)$'
```

## 作業単位末尾と残件

末尾の全生存targeted 19コマンド（Q7c fallbackを含む）は全てexit 0。続く `go test -count=1 ./src/internal/flow ./src/internal/assignment ./src/internal/workflow` もexit 0（flow 23.178秒、assignment 5.323秒、workflow 0.580秒）。変更Goへのgofmt・`git diff --check`は成功し、現行開発手順/CIに削除した26 Test名の残参照はなかった。開始/終了HEADは同じ `dbfd8d9084a1d9e474f0680c1218aa771ec6210e`、コミットは行っていない。独立review・read-only final・同head GitHub checks・PR/mergeは親が担当する。loopで全package/race/vet/cross-build/E2E/liveは実行していない。
