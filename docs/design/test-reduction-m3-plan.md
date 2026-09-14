# M3 詳細実装計画 — 工程・担当管理の重複テストと準備の集約

実装基準はM2のPR #207がマージされたmain `91b54e1ae7555f9bc422a1ad185830b714668366`、対応は[Issue #208](https://github.com/sori883/ai-dd/issues/208)。M2はF05のcontent_test.go以外のM3対象Go fileを変更していない。以下に残る調査時点・親が固定するという記述の開始条件は、この段落によって満たした。

## 目的と実装許可

工程の進行を管理する flow、担当の予約と復旧を管理する assignment、工程定義を読む workflow のテストを整理する。同じ検査の反復と製品が使用しない Git 準備を除き、別のエラーに隠れていたテストは、正常な入力を一つだけ変更する小さな検査へ置き換える。承認、担当の衝突、保存途中の復旧、実ファイルの内容照合は維持する。削除数や coverage 数値を成功条件にしない。

対象は監査 F02–F04、F06–F41 の39件。F01はM1、F05はM2で扱う。ユーザーの「それらを削除」「区切って対応」「Issue作ってmrでマージ」という直接承認と、`docs/ram/decisions/2026-09-14-test-reduction-approved.md`、`docs/design/test-reduction-milestones.md` のM3を実装許可の根拠とする。追加の製品契約、保存形式、公開API、module、tool、汎用framework、release/tag操作は含めない。

作業rootは `/Users/const/sori883/ai-dd-release`。元の `/Users/const/sori883/ai-dd` は対象外。計画調査の基準HEADは親から指定された `65458f6` で、M1 writerの共有ツリー変更中に必要なsourceだけを読み取った。実装開始HEADは **M2 merge後に親が確定**する。この計画作成ではrepo編集、テスト実行、GitHub操作を行っていない。

`AGENTS.md`、implementation-planning、golang-how-to、golang-testing、`docs/agent-workflow.md`、`docs/tdd-handoff.md`、承認RAMと索引、主計画、`docs/test-suite-reduction-audit.md` を根拠とする。過去監査全体の再監査はしていない。一般skillの外部test module導入、機械的なtest追加、多agent追加は採用しない。

## 親が確定するhandoff

- 1 Issue / 1 PR / 単独writer / `work_unit_id=test-reduction-m3` / `verification_mode=loop`。39候補を下記7 sliceの順で1回の依頼へまとめる。
- 親はM2のmerge、M3 Issue、開始HEAD、既存差分、下記所有範囲を確定して渡す。writer稼働中に親は同じfileを編集しない。現在の他者変更を戻さない。
- M1で消した `TestEndSensorUnitCommandPair` と `TestFlowUnitDependencyContent` を実行名に使わない。生存名は `TestVerificationResults` と `TestUnitWithoutGit`。
- 初回成功は `ALREADY_GREEN`。固有assertを生存先へ先に移し、指定targetedで成功を確認してから旧側を削り、同じcommandで再確認する。製品を壊して人工REDを作らない。
- 正しい正常対照が失敗する、移すべき検査が前段の別エラーでしか通らない、新しい製品bugが見つかる場合は削減の成功条件を緩めず親へ返す。製品修正はこのtest削減計画から自動的に実行しない。

## 所有file

以下はすべて作業rootからの相対path。編集できるのは列挙したtestと専用fixture/importだけで、production Goは読み取り対象とする。

**flow**

`src/internal/flow/` 配下の
`assignment_test.go`、`boundary_results_test.go`、`boundary_review_test.go`、
`boundary_sensor_test.go`、`boundary_snapshot_test.go`、`boundary_store_test.go`、
`boundary_transition_test.go`、`codekb_test.go`、`documents_test.go`、
`execution_approval_test.go`、`execution_history_test.go`、`execution_plan_test.go`、
`history_review_test.go`、`procedure_test.go`、`reassign_test.go`、`reopen_log_test.go`、
`review_test.go`、`rule_skill_separation_test.go`、`selected_documents_test.go`、
`sensor_test.go`、`stage_planner_test.go`、`store_test.go`、`unit_test.go`、
`unit_without_git_test.go`、`verification_cli_test.go`、`verification_gates_test.go`、
`verification_results_test.go`、`work_log_okf_test.go`。

`boundary_review_test.go` は監査のF38の例示に直接挙がっていないが、`boundaryFixture` のGit削除で壊れる `flowGit(...worktree...)` の現callerなので、準備置換に必要な所有範囲に含める。`content_test.go` はM2/F05が所有し、M3で再編集・復元しない。

**assignment**

`src/internal/assignment/{directory,reservation,recovery,store}_test.go`。
F41が成立した場合だけ `src/internal/assignment/recovery_stress_test.go` を追加し、旧2testを移す。容量直前fixtureは `recovery_test.go` 内の小さい専用helperを既定とし、別packageや汎用fixture frameworkを作らない。

**workflow**

`src/internal/workflow/{definition,declaration,execution_plan,stage_planner,rule_skill_separation}_test.go`。
`execution_plan_test.go` は唯一のTest削除後にfileを削除できる。他のfileは共有helper/callerを確認してから整理する。

**記録**

親が確定するM3計画、M3結果記録 `docs/design/test-reduction-m3-result.md`、主計画のM3進捗、RAM索引の現行結果リンク、`docs/development.md` の実行入口（F41 stressを採用した場合）を同じwriterへ渡す。過去RAMや過去設計に残る実行証拠のtest名は書き換えない。CI一般の変更はM6でありM3に持ち込まない。

## 候補別の確定処置

以下の「検証Q」は後半のexact commandを指す。これにより各削除候補と実行する生存testが一対一に追跡できる。行番号は監査基準の位置を補助情報とし、編集時は関数名を優先する。

| ID | file・関数と処置 | 固有assertの生存先・理由 | 検証 |
|---|---|---|---|
| F02 | flow/verification_results_test.go:11 `TestVerificationResults` の `self cycle` 行と成功条件の同名分岐を削除 | 同関数 `valid` の成功と「result作成でもcode SHA不変」を残す。入力も処理も同じ行 | Q4a |
| F03 | flow/boundary_results_test.go:32 `TestEndSensorSharedCommandRequiresEachUnitResult` を削除 | `TestVerificationResults/valid` と `second unit missing` が同commandのUnit a/b双方必須を所有。`resultPairFixture` は他callerが使うので保持 | Q4a |
| F04 | flow/verification_cli_test.go:64 `TestVerificationCLIAssignmentSchema` を削除 | assignment/store_test.go `TestRegistry/explicit initialization and aliases` に初期schema/identityを残す。flow固有処理を通らない定数検査 | Q3a,Q3b |
| F06 | flow/selected_documents_test.go:166 `TestSelectedDocumentsIntegrationFeature` を削除 | flow/codekb_test.go `TestCodeKBIntegrationFeature` のStepID付き宣言・正配置成功/誤配置拒否。旧側はStepIDなし宣言が無視される | Q4b |
| F07 | flow/codekb_test.go:51 `TestCodeKBSharedDocuments` を削除 | `TestSelectedDocumentsSharedPaths` の実移動・metadata選択と `TestEndSensorIntegrationDocuments`。旧側は移動元=先 | Q4b |
| F08 | flow/rule_skill_separation_test.go:91 `TestUpstreamSkillInitialization` を削除 | `TestExecutionPlanBootstrap` のCreate/Begin/retry/既存file保全、`TestRuleSkillSeparationRule/default` の実配置 | Q3a |
| F09 | workflow/definition_test.go:99 `TestDefinitionAcceptedInputMustPrecedeStage` を削除 | workflow `TestDocumentDeclaration/input path`、flow `TestExecutionPlanEvidenceSelectedInputs` / `...ArtifactSelectedOrder`。旧path入力の先行拒否に隠れた固定catalog順の擬似契約を除く | Q5a,Q5b |
| F10 | flow/execution_plan_test.go:62 `TestExecutionPlanSchemaChoices` 全体を削除 | 正常は `TestExecutionPlanApprovalSameAnswer`。省略拒否は `TestExecutionPlanDraftRejects` に `empty omission reason`、`duplicate omission`、`selected and omitted` を追加し、既存 `choice missing` と合わせ4意味を所有。詳細は下記 | Q2a |
| F11 | flow/boundary_store_test.go:46 `TestBoundaryStoreVersions` を正常Stage/StepID起点の小tableへ書換え | 正常対照、Stage不一致、入力path逸脱、SHA不正、Sourcesのruntime path、入力重複を同関数で検査。Step誤結合は `TestExecutionPlanEvidenceBindings/entry`、未知acceptanceは `...AcceptedRuns` へ限定 | Q2b |
| F12 | flow/work_log_okf_test.go:203 `TestOKFWorkLogRecoveryPendingValidation` を削除 | flow/execution_history_test.go `TestExecutionPlanReopenHistoryPendingBinding` に `LogHash=""` と `LogAfterHash="bad"` の2行を追加。別fieldのguardを片方ずつ、正常draft/Step/Plan結合のまま検査。missing/bad直積4行は不要 | Q2b |
| F13 | flow/sensor_test.go:92 `TestFlowSensorUnitGraph` の不完全Unitを使う3fixtureとSave失敗を成功扱いする分岐を除去 | 同関数で完全な2Unitの正常/未知依存/循環の直接 `unitPlanProblems` 小tableへ。全Sensor接続代表も1本残す（下記）。待ち続ける依存cycleの事故を維持 | Q2c |
| F14 | flow/execution_plan_test.go:233 `TestExecutionPlanDraftRejects/duplicate` を削除 | 同一requestの `stage swap` を残す。保存された重複IDは `TestExecutionPlanSchemaState/duplicate id` | Q2a |
| F15 | flow/reassign_test.go:312 `TestFlowUnitReassignLiteralPaths` のtracked/untracked軸を除去 | F16と同時に関数全体を削除。Git index状態は製品が読まない。文字を扱う保証はF16の実bytes照合へ | Q1b |
| F16 | flow/reassign_test.go `TestFlowUnitResultLiteralPaths` を複合名1件へ修正、旧ReassignLiteralPathsを削除 | 同関数でliteral pathをScope **とUnit.VerificationPaths** に明示し、結果作成後のbytes改変拒否→復元成功を確認。基本FS/hashは `TestVerificationDigest` を残す | Q1b |
| F17 | flow/execution_approval_test.go:325 `TestExecutionPlanApprovalFinishDropsPreviousUnits` を削除 | `TestExecutionPlanApprovalBootstrapFinish` へ旧Units設定+SaveとFinish後len=0を移す。未Begin/未承認拒否から完了まで1往復を所有 | Q3a |
| F18 | flow/execution_plan_test.go の `TestExecutionPlanEvidenceInitializationEnd`、`...InitialSensor`、`...InitialReview`、`...StartGateStep` を削除 | Bootstrapへstart gate StepID、Check pass/StepIDを、BootstrapFinishへReview assign/acceptのStepIDとpassを移す。End成功は実Finishで維持 | Q3a |
| F19 | `TestExecutionPlanBootstrapMissingConfiguration` のRule欠損行を削除し、CLI/OKF skill欠損pathを追加。`TestRuleSkillSeparationHookMissingCLI`、`TestOKFSkillInitialization` を削除 | 欠損tableはhooks.json、aidlc、aidlc-cli、okf-agent-memoryの4paths。Rule欠損は `TestRuleSkillSeparationRule/missing`。異なる配置漏れ4種類は保持 | Q3a |
| F20 | flow/verification_gates_test.go:87 `TestVerificationGatesIndependence` を削除 | `TestVerificationGates/none` の正常assign直前にsame root+same session拒否を移す。別root同sessionの `TestFlowReviewIdentityTarget` とは別の保証 | Q4a |
| F21 | `TestBoundaryStoreSchema` を削除。`TestExecutionPlanSchemaState/old schema`、`TestDefinitionBindingMalformedState/old schema`、documents/reopen_log/verification_cliの単純schema数字assertを除去 | flow/store_test.go `TestFlowStoreWireSchema` に未対応schemaのRead拒否とfile bytes不変を1行追加。Create/CASは `TestFlowStoreCreateCAS`、現wire形式はWireSchemaを正本とする | Q3a |
| F22 | flow `TestFlowStoreRejectsCorruptAndIsolates` の末尾duplicate schema keyのみ削除。assignment `TestRegistry` のbroken/unknown version/unknown field旧3行を削除 | flow duplicate keyはWireSchema。assignment `TestRegistryRejectsDamagedRecords` へ正常Registryの不正schema、rawへの未知field、壊れたJSON代表1行を統合。ID/path/Space/root分離は保持 | Q3a,Q3b |
| F23 | flow/verification_cli_test.go `TestVerificationCLI` 末尾Marshal→旧Git field5名のContainsを削除 | wire形式はWireSchema。Hashのread-only、unknown Unit、登録root、VerificationPathsの独自検査は同関数に保持 | Q3a |
| F24 | flow/documents_test.go `TestIntentDocumentsLegacyFieldsRejected` 全体、workflow/declaration_test.go `TestDocumentDeclaration/old refs` 行を削除 | flow WireSchemaのunknown fieldとworkflow `TestDefinitionRejectsInvalid/unknown yaml`。独自metadata `unknown match` 等は残す。旧名だけのdecoder再確認を除く | Q3a,Q3c |
| F25 | workflow/execution_plan_test.go `TestExecutionPlanSchemaCatalog` と空になったfileを削除 | `TestDefinitionRejectsInvalid` に `missing required prefix`（空配列）1行追加。正常6stageと順序拒否、unknown graphは既存DefinitionValid/RejectsInvalid。len!=2の上下直積は増やさない | Q5a |
| F26 | workflow/definition_test.go `TestDefinitionRejectsEmptyOrUnknownAgent` を削除 | workflow/stage_planner_test.go `TestStagePlannerRole` をplanner正常、wrong agent、unknown role、missing roleの4行へ。未知roleは既存1行、role無しはagent付き1行のみ | Q5a |
| F27 | flow/stage_planner_test.go `TestStagePlannerProcedure` の日本語5語Containsだけ削除 | 同関数の実Procedure viewの4つのrole/agent/order結合は維持。workflow loader正常だけではflow返却時の取り違えを保証しないため全関数削除はしない | Q5b |
| F28 | flow `TestEndSensorResults` を正常+不正exit代表の2caseへ。`TestExecutionPlanEvidenceResultRun` を削除 | `TestVerificationResults` へmissing exit、unknown JSON field、empty step、invalid SHA formatを追加。既存step/sha/command/exit各意味は同table。EndSensorは拒否がGate failに届く接続を保持。`TestEndSensorIntegrationRequiresCurrentSHA` の実code改変と更新後成功は保持 | Q4a |
| F29 | flow/history_review_test.go `TestExecutionPlanReviewHistoryMutation` を15→7case | configureでmissing/hash/old head/invalid record/invalid UTF8の5種、pauseとsourceはhash代表各1。各caseのwrites=0、state bytes不変、approval source未作成を保持 | Q4c |
| F30 | flow/boundary_snapshot_test.go `TestBoundaryTransitionCollectorSnapshot` のremove=falseを削除し、remove=trueを単一caseへ | 変更後cacheは `TestSelectedDocumentsCollectorSnapshot` がselected/material/hashも所有。削除後再読込は残る単一CollectorSnapshotが所有。`TestBoundaryTransitionUsesOneSensorSnapshot` はFinish時の独自保証として保持 | Q4b |
| F31 | flow/reassign_test.go `TestFlowUnitReassignPendingBlocksStateUpdates` の後半later pause/resume/reassign/新runを3caseから削除 | `TestFlowUnitReassignStoppedAndIdentity` の後日再割当で新runを維持。tableは3入口の拒否、state不変、他runtime未作成、同request同run復旧まで残す | Q6a |
| F32 | flow/reassign_test.go `TestFlowUnitReassignScopeAndDependency` の1行tableと死んだscope/Git elseを除去 | 関数はdependency拒否単一caseとして残す。scope衝突は `TestFlowUnitReassignMissingRuntimeScope`。F38と同時に先行実施 | Q1b,Q6a |
| F33 | flow/reopen_log_test.go `TestReopenLogFailures` を削除。`TestOKFWorkLogRecovery` のfresh/existing直積を整理 | Recoveryはfresh/base、existing/pending、existing/logの3case。final保存点は `TestExecutionPlanReopenHistoryLogRetry` のpublic Capture/DecidePlan往復へ集約。escape、操作拒否、旧expect、timestamp、既存本文保全を下記対応どおり移す | Q6b |
| F34 | `TestReopenLogChangedHistoryRejected` 全体と `TestOKFWorkLogRecoveryRejects` のmissing/changed pending旧行を整理 | RecoveryRejectsへ `before log changed`、`after log deleted` の2独立case。前者はlog write失敗、後者はfinal state失敗後に改変/削除。両方state不変、改変bytes/欠損不変を保持。`TestReopenLogFreshDeletionRequiresRestore` は維持 | Q6b |
| F35 | `TestOKFWorkLogRecoveryRejects/metadata overflow` を削除 | `TestReopenLogCapacity/overflow`、fits、fresh encoded reasonを残す。encoded sizeでstate/logを壊さない境界を一箇所にする | Q6b |
| F36 | `TestOKFWorkLogRecovery` のPendingReopen Marshal→map→LogAfterHash長64を削除 | PendingReopenのtyped存在/結合はF12、復旧後の実OKF bytes/hash/時刻はF33。different request拒否はexisting/logで残す。serializer自己再解釈を反復しない | Q6b |
| F37 | assignment `TestReservation` のalias、fresh retry、別root成功を除去。`TestAssignmentRecoveryInitRetry` を削除 | directory_test.go `TestDirectoryAssignment` に既存同root/親/子/alias/別root/retryを残し、初回ID/TaskName/Statusのassertも集約。Init retryは `TestRegistry/explicit initialization and aliases` へ。Reservationはchanged request、Space/Intent/owner衝突、release後retry非復活/再利用を残す | Q6c |
| F38 | 通常fixtureとその依存callerのGit init/commit/worktree/merge/rev-parse/ls-filesを除去 | registryFixtureは通常3root、worker/reviewerは別通常root。unitの未転記で拒否→実bytes転記で統合、reviewの別root不一致拒否を残す。後述の全callerを同sliceで置換 | Q1a,Q1b |
| F39 | `TestVerificationResultsStageValidity` のinitialization/discovery/planningを代表planning1値へ、1行loopを除去 | 非検証stage拒否1件を同関数で保持。正常TDDはVerificationResults、正常integrationはIntegrationRequiresCurrentSHA。将来stage別例外分岐を導入時はその変更のtestを追加する | Q7a |
| F40 | workflow `TestRuleSkillSeparationRuleReference/invalid metadata`（空title）だけ削除 | title/description/status/tags/intent_idの各nil guardは軽く独立したfield落としを捕捉するため全て維持。path/type/version/match/count/role/accepted_atも維持 | Q7b |
| F41 | 通常の容量直前fixtureを先行させ、成立した場合だけ旧2反復testをstress fileへ移す | 下記の通常2testで実Reserve/Release/PreSpawn/PostSpawnと危険な新規受付拒否を保持。旧全admission反復はstressで別の累積事故を保持。安全なfixture不成立なら旧2testを通常に維持して完了理由を記録 | Q7c,Q7d |

## 特に事故を起こしやすい置換の具体手順

### F38 / F15 / F16 / F32: Gitを準備から外す

1. `registryFixture` のroot/a/bは全て独立した実在する `t.TempDir()` にする。symlinkの実alias、親/子root、process競合はそのまま残す。`reservation_test.go` の `os/exec` は `TestReservationProcess` が使用するので削らない。
2. `boundaryFixture` のGit2呼出し、`discoveryApprovalFixtureOrder` のinit/add/commitを除去する。`sensorFixture` の使われない `var err error; if err != nil` も同じ準備の整理として除ける。
3. `unitFixture`、unitのworker-b/c、reassignのother_unit、assignmentのother rootを通常directoryへ。`TestFlowUnitResultIntegrationAndResume` はworkerのa.txtをprojectへ転記する前のintegrate拒否を必ず保持し、その後ReadFile/WriteFileで同bytesを転記して成功。`TestFlowUnitTwoParallelThenDependent` もa/bを各別rootから転記し、両方統合後にcをclaimする。
4. `flowReviewRoot` は通常directoryを作り、fixture内の検証対象となる通常fileを明示的にコピーする。`filepath.WalkDir` を使う場合も小さいtest専用処理にし、`.git` と `aidlc/` の知識/状態/runtimeは複製しない。製品のComputeVerification選別を再実装したoracleを作らない。`TestFlowReviewRejectsWrongCheckout`、`...ChangedCheckout` の異なるroot・実bytesの差を維持する。
5. `boundary_review_test.go/TestBoundaryReviewRequiresEndPass`、`discoveryApprovalFixtureOrder`、`TestExecutionPlanApprovalArbitraryOrder` のreview rootも上記通常rootへ。必要な検証対象bytesがないfixtureなら空の別directoryでよいが、同rootに置換しない。
6. discarded HEADを `procedure_test.go/TestProcedureBoundaryAcceptedTDDOutput`、codekb、boundary_snapshot、boundary_sensor、boundary_transition、selected_documents、execution_planから削る。F06で消すtest内のHEADは関数削除時に消える。execution_planのResultRun/SensorTarget/OptionalPlan/ArtifactSelectedOrderのinit/commitも除去する。
7. `assignment_test.go` のreassign要求へのHEAD設定2箇所を除去。`TestUnitAssignmentLegacyResult` は「無管理runの結果を採用しない」保証を失わせないため、現形式のmanaged claimと正しいresult証拠を準備し、registryから当該予約だけを外した正常Registryを置く。その後のresult拒否、state/runtime不変を `TestUnitAssignmentUnmanagedResult` に改名して所有させる。40桁SHAや旧runtime形式による先行拒否を正例の代わりにしない。`TestUnitAssignmentLegacyReassign` の存在しない予約を後付けしない境界は保持する。
8. F16は `TestFlowUnitResultLiteralPaths` の1件を `src/ 日本\nfile.go` のような日本語・改行・要素先頭spaceを含む名前にする。ScopeとVerificationPathsを同じ実pathに設定してから保存する。正常なresult requestを **一度** 作成した後、そのfileのbytesだけを変え、`s.Unit` へ元requestを直接渡して拒否を確認し、bytesを復元して元requestで成功する。`assignmentUnit` が証拠を再生成して不一致を隠さないよう、改変後に補助生成を呼ばない。既存TestVerificationDigestの対象集合・filesystem境界は変更しない。
9. `unitWithoutGitFixture` と `TestVerificationGates` の不要になった `.git` 削除準備を外せる。全callerを置換した後に `rg -n 'flowGit\\(' src/internal/flow --glob '*_test.go'` で残りを確認し、最後のcallerがなくなった場合だけ `flowGit` と専用importを削除。関数を先に消してcompile failureをRED扱いしない。

このsliceは初期のGit削除だけを先に適用すると生存testのworktree作成が壊れるため、fixtureと依存callerを一組で扱う。Git対応を製品から撤去する変更ではない。

### F10–F14: 前段拒否に隠れない正常対照

- F10は `initialPlanRequest()` の有効なSteps/Omittedを使う。既存 `choice missing` はそのまま、空reasonは一つのOmitted.Reasonだけ、duplicate omissionは同Omitted1つの追加、selected+omittedは選択済みtddをOmittedへ追加する。未承認のProposePlan入力なのでPlanHash不一致を別の失敗理由に持ち込まない。正常ProposePlanを `TestExecutionPlanDraft`、承認された全省略をSameAnswerで確認してから旧SchemaChoicesを削る。
- F11は `schemaPlanState(s)` など小さい有効Stateを使い、`Entry.Stage=st.Stage`、`Entry.StepID=st.CurrentStepID` を設定する。正常Inputs/Sourcesをpersistできる対照を1つ置き、各caseでslice/entryを独立コピーする。identity不正をpath/hash不正と同時に作らない。重複path、Sources runtime pathは同validatorの別呼出しを消さないため残す。
- F12は既存ReopenHistoryPendingBindingの正常persistを維持し、LogHashの空とLogAfterHashの不正を別々に変更する。state mapを手で組み直さず、既存の正しいdraft/StepID/Revision/PlanHash/Reasonを使う。
- F13の直接tableは完全なUnit a/b（ID、Bolt、Scope、Tests、正常DependsOn）で正常=問題なし、unknown/cycle=対象の問題が存在し `incomplete Unit plan` はないことを確認する。Sensor固有forwardingを消さないため、同関数の `sensor forwarding` subtestを1つだけ残す。正常planningのCheck passを先に作り、dependencyのみ未知へ変えてCheck failに到達することを確認する。Save失敗を許容せず、Gate failuresの対象依存メッセージまで確認する。全validator直積をもう一度Sensorへ流さない。
- F14は同一requestのduplicate行のみ削る。現形式のstate duplicate idは保持。

### F17–F30: 公開入口と小さなvalidatorの分担

- BootstrapFinishには未開始Finish拒否後、旧Units設定+Saveを移す。Begin後のReview assign/acceptにStepID assertを添える。BootstrapにはstartStateのStepIDとpublic Check pass/StepIDを1回追加する。旧4testのために同一設定を4回作らない。
- F21のWireSchemaは名前付きtableへ整理し、未対応schema行は既存正常rawのschema値だけ変更する。Read前後のfile bytes一致を確認する。未知field/重複key/trailing JSON/ID不一致の独立入力は維持する。
- F22のassignment正常RegistryはRead成功を先に確認し、各caseのrawを正常bytesから独立生成する。unknown fieldはschemaやidentityを同時に壊さない。型で表現できない未知fieldだけraw挿入を使う。`TestRegistryRejectsDamagedRecords` の既存missing root/unknown state/duplicate idも保持する。
- F28で小tableへ移す不正SHAは、既存64桁の「現在内容との不一致」と別の形式拒否。既存EndSensorで使っていた40桁例を小tableに1つ残せる。未知fieldは正常serialized resultに1fieldのみ挿入、missing exitはnil、empty stepは空。EndSensor正常とexit!=0は同じ正常設定からresultだけ変更し、Checkのstatusがpass/failとなる接続を残す。
- F29は `[]struct{mode,action string}` の7行へ変更し、全caseごとに独立TempDir/State。5×3を条件skipに置換しない。invalid record/UTF8は正しいhashへ付け替える既存方法を維持し、hash不一致だけで通るように弱めない。
- F30の削除後cacheは `selected` の再探索に無理に通さない。存在しないfileを直接再読込するcache境界は小さい単一caseのまま残す。Finishの承認snapshotは別保証なので統合しない。

### F31–F37: 保存失敗を4点で一度ずつ扱う

F33は以下の4点に固定する。障害注入の回数そのものを期待値にせず、対象保存点に到達したことと利用者が観測する保存内容を確認する。必要なら既存 `s.write` closure内でpathと書き込むState.PendingReopenを読み分ける小さな注入にする。汎用fault frameworkは作らない。

| 保存失敗点 | 生存関数/case | そこへ集める保証 |
|---|---|---|
| 新規logのbase作成 | `TestOKFWorkLogRecovery/fresh/base` | 失敗を成功扱いしない、revision不変、同request再試行で正規OKF作成・reason一度 |
| pending state保存 | `TestOKFWorkLogRecovery/existing/pending` | 既存本文保全、revision不変、同request再試行が成功、正常OKF/search |
| log追記 | `TestOKFWorkLogRecovery/existing/log` | pending実在、configure/pause/CheckWork拒否、different request拒否、同request復旧、旧expect拒否。reasonを `line one\n## forged marker` にし、見出し偽装をescape・一度だけ出現・旧本文保全。生成時刻=PendingReopen.At |
| final state保存 | `TestExecutionPlanReopenHistoryLogRetry` | 現public CaptureApproval/DecidePlanを維持。既存logをseedしてからReopenし、旧ExecutionPlan.Revisionとpendingを確認。同じ承認で再試行し、完成済みlog bytes不変、Reopen見出し一度、pending消去、次revision。時刻は実parsed metadataとpendingの対応を維持 |

Recoveryのfresh/pending、fresh/log、fresh/final、existing/finalの重複行は、上記成功経路と固有assert移動を実測した後に削る。fresh/baseの復旧は新規作成から後続保存まで正常に通る。finalのpublic testからCapture/DecidePlanを取り去ってhelperだけにしない。

F34はF33とは別に「保存途中で外部がlogを変えた」境界を維持する。RecoveryRejectsの2caseはそれぞれ独立fixture。before log/changedはlog writeを失敗させ、元logへ文字を追記してretry拒否。after log/deletedはfinal stateを失敗させて完成logを削除しretry拒否。state bytes不変、改変したlog bytes不変または欠損継続を確認する。after log用の失敗注入はF33と同じ保存点を使えるが、request/stateは共有しない。新規log削除時の手動restore要求testは削らない。

F37の `TestReservation` 冒頭のReserve自体はchanged request/衝突/解放に必要なので残す。重複するassertと成功往復だけ除く。`TestRegistry/explicit initialization and aliases` は同一InitRequestの再試行でEpochとRevisionが変わらないことを確認してから別request拒否へ進む。

### F41: 通常の容量境界とstressを分ける条件

容量制限は、受付済みの担当を解放できない、受付済みの子taskの応答を確定できない事故を防ぐ。旧 `TestAssignmentRecoveryCapacity` と `TestAssignmentRecoveryEscapedReleaseCapacity` を単に通常suiteから隠さない。

採用する通常test名は `TestAssignmentRecoveryCapacityBoundary` と `TestAssignmentRecoveryEscapedReleaseCapacityBoundary`。fixtureは次の方式に限定する。

1. 少数のpublic Init/Reserve/Release/PreSpawn/PostSpawnで得た**有効なseed記録**を用意する。容量埋めは完了したbound Dispatch等の履歴をmemory上で複製し、ToolID/TaskName/Canonical等の一意性と長さを保つ。無意味なunknown field、無効なInitialization.Reason、JSON whitespaceのpaddingで容量を埋めない。
2. serialized bytes長と `filestore.MaxBytes` を使って、余裕のある直前fixtureと、最大の合法な完了データを保存できない直前fixtureを少数用意する。例として32KiB程度の余裕、8KiB程度の余裕から開始できるが、数値を製品のheadroom定数としてassertしない。`admission` の16384/1024計算式・status別加算をtestへコピーしない。
3. seedを複製した最終Registryは通常のReadで通り、ID/root/request hash/重複に問題がないことを先に確認する。最後の最大応答（受理される512bytes以内のcanonical path）と最大release理由（`strings.Repeat("\x01", 2048)`、JSONで増幅する値）が合法であることを確保する。
4. **安全側**は実Reserveと少数（最低2つ）の実PreSpawnを受理し、実Releaseと全受付済みPostSpawnを成功させ、Store.Read成功・released/bound・期待するID/path/理由を確認する。
5. **危険側**は、単に通常保存のMaxBytes超過で落ちるfixtureにしない。「受付直後のserialized RegistryならMaxBytes以下だが、合法な最大ReleaseまたはPostSpawn後にはMaxBytesを超える」をtest入力のbytesで確認してから、新規Reserve/PreSpawnが拒否され、元registry bytesが不変であることを検査する。候補は高々数個の明示fixtureで調整し、production admissionを呼ぶ大量探索や上限計算を写すhelperを作らない。
6. root path長やencodingのため上記の安全/危険の前提を一意に満たすfixtureを小さく作れなければ、F41を **理由付き通常保持**として終了する。旧2testを通常suiteに残し、stress fileも通常追加testも採用しない。このfallbackは主計画が承認済みであり、他38件の完了を妨げない。

この2通常testの成功を先に確認できた場合だけ、旧2関数は同じ関数名のまま `recovery_stress_test.go`（`//go:build stress`）へ移す。元の全pending応答を最大pathへ確定する反復と、escaped releaseを何回もadmissionする反復を弱めない。通常fixtureは境界、stressは全履歴の累積という役割になる。通常PRでstressを隠れてskipせず、M3 finalでQ7dを明示実行し、現行開発手順へ同commandを記載する。stress tagを採用したかどうかと通常の生存保証を結果記録へ残す。

## 順序付きsliceとexact targeted commands

全commandは `/Users/const/sori883/ai-dd-release` で実行する。この計画作成時は未実行。

### Slice 1 — 通常directory化と実bytes境界（F15/F16/F32/F38）

先に別rootの既存拒否/成功を確認し、上記Git callerを一括置換。literalは正しいVerificationPathsと古い証拠の拒否を先行させる。

Q1a:

~~~sh
go test -count=1 ./src/internal/assignment -run '^Test(Registry|DirectoryAssignment|DirectoryAssignmentConcurrentAndSaveFailure|Reservation|ReservationProcess|ReservationRequestIDAcrossOperations|ReservationReplacementSaveFailure|AssignmentDispatch|AssignmentRecovery)$'
~~~

Q1b:

~~~sh
go test -count=1 ./src/internal/flow -run '^Test(FlowUnit.*|UnitWithoutGit.*|UnitAssignment.*|FlowReview.*|BoundaryReviewRequiresEndPass|ExecutionPlanApproval.*|VerificationGates.*|VerificationDigest|ProcedureBoundaryAcceptedTDDOutput|CodeKBIntegrationFeature|EndSensorIntegrationDocuments|SelectedDocumentsIntegrationDoesNotFreezeMaterials|BoundaryTransitionUsesOneSensorSnapshot|ExecutionPlanEvidence(SensorTarget|OptionalPlan|ArtifactSelectedOrder))$'
~~~

### Slice 2 — 有効な正常入力から拒否を検査（F10–F14）

Q2a:

~~~sh
go test -count=1 ./src/internal/flow -run '^Test(ExecutionPlanDraft|ExecutionPlanDraftRejects|ExecutionPlanSchemaState|ExecutionPlanApprovalSameAnswer)$'
~~~

Q2b:

~~~sh
go test -count=1 ./src/internal/flow -run '^Test(BoundaryStoreVersions|ExecutionPlanEvidenceBindings|ExecutionPlanEvidenceAcceptedRuns|ExecutionPlanReopenHistoryPendingBinding)$'
~~~

Q2c:

~~~sh
go test -count=1 ./src/internal/flow -run '^Test(FlowSensorUnitGraph|FlowUnitClaimDependencyAndOverlap|UnitWithoutGit)$'
~~~

### Slice 3 — 初期化とwire schemaの正本（F04/F08/F17–F19/F21–F24）

Q3a:

~~~sh
go test -count=1 ./src/internal/flow -run '^Test(ExecutionPlanBootstrap|ExecutionPlanBootstrapMissingConfiguration|ExecutionPlanApprovalBootstrapFinish|RuleSkillSeparationRule|FlowStoreCreateCAS|FlowStoreRejectsCorruptAndIsolates|FlowStoreWireSchema|FlowStoreFailurePreservesState|ExecutionPlanSchemaState|DefinitionBindingMalformedState|DefinitionBindingDrift|IntentDocumentsRegistration|IntentDocumentsEntryAndAccepted|VerificationCLI)$'
~~~

Q3b:

~~~sh
go test -count=1 ./src/internal/assignment -run '^Test(Registry|RegistryRejectsDamagedRecords)$'
~~~

Q3c:

~~~sh
go test -count=1 ./src/internal/workflow -run '^Test(DocumentDeclaration|DefinitionRejectsInvalid)$'
~~~

### Slice 4 — Sensor接続、選択snapshot、履歴（F02/F03/F06/F07/F20/F28–F30）

Q4a:

~~~sh
go test -count=1 ./src/internal/flow -run '^Test(VerificationResults|VerificationResultsUnit|EndSensorResults|EndSensorIntegrationRequiresCurrentSHA|VerificationGates|FlowReviewIdentityTarget)$'
~~~

Q4b:

~~~sh
go test -count=1 ./src/internal/flow -run '^Test(CodeKBIntegrationFeature|SelectedDocumentsSharedPaths|EndSensorIntegrationDocuments|SelectedDocumentsCollectorSnapshot|BoundaryTransitionCollectorSnapshot|BoundaryTransitionUsesOneSensorSnapshot)$'
~~~

Q4c:

~~~sh
go test -count=1 ./src/internal/flow -run '^Test(ExecutionPlanReviewHistoryMutation|ExecutionPlanReviewHistoryRevisionOrder|ExecutionPlanReviewHistoryConfigKeepsHead)$'
~~~

### Slice 5 — workflow定義とProcedure返却（F09/F25–F27）

Q5a:

~~~sh
go test -count=1 ./src/internal/workflow -run '^Test(DefinitionValid|DefinitionRejectsInvalid|DocumentDeclaration|StagePlannerRole)$'
~~~

Q5b:

~~~sh
go test -count=1 ./src/internal/flow -run '^Test(StagePlannerProcedure|ExecutionPlanEvidenceSelectedInputs|ExecutionPlanEvidenceArtifactSelectedOrder|ExecutionPlanApprovalSameAnswer)$'
~~~

### Slice 6 — 再割当、途中保存、registry往復（F31/F33–F37）

Q6a:

~~~sh
go test -count=1 ./src/internal/flow -run '^Test(FlowUnitReassignStoppedAndIdentity|FlowUnitReassignStateFailureRetry|FlowUnitReassignPendingBlocksStateUpdates|FlowUnitReassignMissingRuntimeScope|FlowUnitReassignScopeAndDependency)$'
~~~

Q6b:

~~~sh
go test -count=1 ./src/internal/flow -run '^Test(OKFWorkLogRecovery|OKFWorkLogRecoveryRejects|OKFWorkLogRecoveryFIFO|ExecutionPlanReopenHistoryLogRetry|ExecutionPlanReopenHistoryPendingBinding|ReopenLogFreshDeletionRequiresRestore|ReopenLogCapacity)$'
~~~

Q6c:

~~~sh
go test -count=1 ./src/internal/assignment -run '^Test(DirectoryAssignment|DirectoryAssignmentConcurrentAndSaveFailure|Reservation|ReservationProcess|Registry|AssignmentRecovery)$'
~~~

### Slice 7 — 軽い表と容量の実境界（F39–F41）

Q7a:

~~~sh
go test -count=1 ./src/internal/flow -run '^Test(VerificationResultsStageValidity|VerificationResults|EndSensorIntegrationRequiresCurrentSHA)$'
~~~

Q7b:

~~~sh
go test -count=1 ./src/internal/workflow -run '^TestRuleSkillSeparationRuleReference$'
~~~

Q7c（新通常testを先行して作れた場合）:

~~~sh
go test -count=1 ./src/internal/assignment -run '^TestAssignmentRecovery(CapacityBoundary|EscapedReleaseCapacityBoundary)$'
~~~

F41 fallback時はQ7cの代わりに以下を実行し、旧2testが通常suiteに残ることを記録する:

~~~sh
go test -count=1 ./src/internal/assignment -run '^TestAssignmentRecovery(Capacity|EscapedReleaseCapacity)$'
~~~

Q7d（stress採用時だけ、**親のfinalで1回**。loop/reviewは原型反復を繰り返さない）:

~~~sh
go test -tags=stress -count=1 ./src/internal/assignment -run '^TestAssignmentRecovery(Capacity|EscapedReleaseCapacity)$'
~~~

新通常testをまだ作っていない時点のQ7cの `no tests to run` を成功と扱わない。初回旧容量2testを比較対象として実行する必要があればslice開始に1回だけ実行し、変更のない原型を途中に反復しない。

## work unit末尾、review、final、merge

writerは全sliceのtargeted（Q7d以外、F41の採用branchに合ったcommand）をまとめて再確認し、変更Go fileへgofmtを適用した後、次を確認して1回返す。

~~~sh
go test -count=1 ./src/internal/flow ./src/internal/assignment ./src/internal/workflow
git diff --check
~~~

上記はこのwork unitのaffected package testで、全package/race/vet/cross build/配布E2Eはloopへ持ち込まない。tag採用時は `go test -tags=stress -list '^TestAssignmentRecovery(Capacity|EscapedReleaseCapacity)$' ./src/internal/assignment` でcompile/discoveryだけ確認できるが、実行成功とは区別する。

返却は `WORK_UNIT_READY` または `BLOCKED`、各IDの実処置、sliceごとのALREADY_GREENまたは正当なRED/GREEN証拠、exact command/exit、変更file/開始終了HEAD/最終hash、既存差分との区別、F41採用または通常保持理由を含める。

親はtargeted群と全差分を一度確認後、固定base/headで独立reviewを開始する。reviewでは次を重点確認し、全検証を代行しない。

- 各削除の固有assertが生存先に存在し、別エラーに隠れていない。
- 生存するpublic入口のforwardingを下位validator正常だけで代替していない。
- 正常fixture、state、registry、承認source、証拠をcase間で共有していない。
- runtime/知識のコピーや同root化で、別root照合・無管理run拒否を消していない。
- 途中保存点はbase/pending/log/finalを一度ずつ本当に通り、同request復旧と外部改変拒否を混同していない。
- F41の危険側がRead不正/通常保存超過で先に拒否されておらず、受付後の合法な完了を救う容量予約を検査している。通常境界がないstress移動を認めない。

blocking findingを解消して差分が安定したら、親がread-only finalを1回開始する。主計画の共通検証をM3用出力名で実行する。

~~~sh
go test -count=1 -shuffle=on -coverprofile=/tmp/ai-dd-test-reduction-m3-coverage.out ./...
go test -count=1 -race -shuffle=on ./...
go vet ./...
go mod tidy -diff
gofmt -l src
git diff --check
go test -tags=integration -count=1 ./src/internal/workspace ./src/internal/okf
go test -tags=integration -count=1 ./src/cmd/aidlc -run '^Test(FlowJourney|GitIndependentJourney)$'
~~~

stress採用時はQ7dをここで実行する。coverageは比較資料であり維持率のgateにしない。M3で配布資材・実機仕様を変更しないため、新たなCodex live実行やローカル配布build反復は追加しない。対象PRで起動する既存GitHub checksの現在HEAD成功は親が確認する。final後にfile変更があれば証拠はstaleとして、必要なloop/review後にfinalを更新する。

親がIssueに紐づくPRを作り、独立review・final・現在HEADのchecks成功後に既存方式でmergeし、mainへの反映とIssue closeを確認する。rollbackは問題のPRをrevertする別PRで行い、共有treeや他writerの変更をresetで巻き戻さない。

## 残る確認点と受入条件

- M2 merge後の正確なHEADと既存差分、削除/改名済みの生存入口を親が確定する。この段階でM2資材testをM3から再監査しない。
- F41は読み取り調査だけでは安全な容量直前fixtureの実測成功を断言できない。通常境界が成立した場合のみstressへ移す、成立しなければ旧2通常testを理由付きで保持する、という主計画の分岐をそのまま受入条件とする。
- F29で消す8組合せとF34で消す交差組合せは、将来入口ごとのdecoderや保存点ごとの別の破損処理を新設した場合には、その新分岐の回帰検査が必要。現共有validatorの検査を各層で総反復しない。
- F40は独立field guardに現実の検出価値があるため、他5metadata field行を削らない。維持を削減未完了とは扱わない。
- 完了は39IDすべてに削除・統合・維持の処置と生存保証が記録され、前段拒否の偽対照が解消し、別root/実bytes/承認/4保存点/容量通常境界（または旧通常保持）と現行test入口が検証された状態。新製品挙動や新しい公開形式が増えていないことも確認する。

## 実装時の判断（2026-09-14）

F41は通常保持fallbackを採用した。合法seedのRead成功だけでなく、受付直後は保存可能で最大応答またはescaped release後だけ容量を超える安全側・危険側の小fixtureを本変更では確立していない。root長・識別子長・JSON増幅の調整に容量式のコピーや大量探索を追加せず、旧2通常testを保持する。stress tagと新Boundary testは追加しない。Q7cはfallback command、Q7dは非適用。[実装結果](test-reduction-m3-result.md)に39候補と証拠を記録する。
