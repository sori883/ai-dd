# M5: CLI一周・旧実験・診断入口の削減計画

作業rootは `/Users/const/sori883/ai-dd-release`。参照headはM1の `a17f6ebe379b72dac41338489ec4a8c2684f5434`。これは読み取り専用の詳細計画で、repo編集・test実行・GitHub操作は行っていない。親がM4 merge後のmainをbaseとして、移動済みの生存test名と所有fileを固定する。

許可は `docs/ram/decisions/2026-09-14-test-reduction-approved.md` の直接依頼と `docs/design/test-reduction-milestones.md` のM5。対象はP26/C02/C05–C36の34候補、1 Issue/PR・1 writer・`work_unit_id=test-reduction-m5`、実装時は `verification_mode=loop`。製品仕様、公開CLI、保存、承認・親子権限、7asset配布を変えない。既存正挙動の保証移動は `ALREADY_GREEN` と記録し、人工REDを作らない。他者の編集を戻さない。

以下の `cmd/` は `src/cmd/`、`internal/` は `src/internal/`。file名だけのセルは `src/cmd/aidlc/` 配下。新test名は本計画で固定する予定名。同名がM3/M4で既に存在した場合は増設せず、その生存testへ固有assertを統合する。

開始基準: M4マージ済みmain `271efb4b1e1030e7b25ef683ad6e49be1b859d35`。対応は [Issue #212](https://github.com/sori883/ai-dd/issues/212)、branchは `codex/test-reduction-m5`。M4後のFiveCLIContract、HookCommandDispatch、ExecutionPlanCLIGrammar、BoundaryRootHelp、RuleSkillSeparationHelp、ConfigureHelpExamples、M3のUnitAssignmentUnmanagedResult/LegacyReassignが生存することを親が確認した。既存tag条件を含む残るhelperのcallerは各削除直前に照合する。実観測記録のない診断には計画の小synthetic fallbackを使い、実機の成功に算入しない。

## 候補別の処置と生存先

| ID | 正確な対象・処置 | 生存する保証／移動先 |
|---|---|---|
| P26 | `cmd/natural-japanese-go/integration_test.go::TestCommandBinary` を削除。`cmd/aidlc-dist/release_integration_test.go::TestReleaseCandidateNative` に同候補の `--json -`＋stdin入力を1回追加。`.github/workflows/ci.yml` の旧test専用stepも同PRで削除 | 同候補の空PATH環境で入力由来の `forbidden_phrase` JSON。候補未設定skipは移動完了に数えず、同head Distributionの3OSで実行 |
| C02 | `git_independent_journey_integration_test.go::TestGitIndependentJourney` の2×2直積を削る | `TestFlowJourney` をno-git/direct、`TestGitIndependentJourney` をdetect-git/unitsの計2一周にする。前者にも明示空PATH、後者に検出stubと呼出なし・`.git`なしのassertを残す |
| C05 | `main_test.go::TestSpace{Creator,Lister,Switcher}LazyCLIInputs` の44入力を縮小 | root help/versionの依存未呼出代表＋各routeの不正入力1件。文法分類とcallback未呼出はM4後の `internal/cli/space*_test.go` |
| C06 | 同file `TestSpace{Creator,Lister,Switcher}RootInput` のgetwd/getenv回数・順序・constructor時0回を削除 | 3adapterそれぞれのRootInput各欄、生name、返却値の取り違えを残す。早期副作用はC05 |
| C07 | 同fileのWorkingDirectoryFailure/CreationFailure/ReadFailure/SwitcherFailuresを `TestSpaceAdapterErrors` に統合 | cwd失敗1代表と独立3adapter各1個のworkspace error伝達を保持。3caseの完全削除は採用しない。文字列＋同一性の二重assertは除く |
| C08 | `flow_command_test.go::TestFlowCommandPublicCutover`、`rule_skill_separation_test.go::TestRuleSkillSeparationHelp` を削除、空fileも除去 | M4後の `internal/cli` FiveCLIContract/HookCommandDispatch/ExecutionPlanCLIGrammar/BoundaryRootHelp/Rule helpと `internal/okfcli`。必要な現行routeのassertだけ先に統合、廃止別名全列挙は保持しない |
| C09 | `project_root_test.go::TestProjectRootWithoutGit` と空fileを削除 | application祖先探索・missing・new installの3件を `internal/projectroot/root_test.go::TestResolveInstallationContext` へ。既存ResolveAncestorFiles/ResolvePreservesErrorsは維持 |
| C10 | `main_unix_test.go::TestMainSpace{List,Switch,Create}ClosedPipes` / `TestMainRootCommandsKeepSIGPIPE` を代表へ縮小 | List4件（human/stdout、JSON/stdout、bare human/stderr構文、bare JSON/stderr root）、Switch成功stdout/構文stderr、Create stdout/名前不足stderr、root help/unknown。実SIGPIPE・`TestMainSpaceList`は残す |
| C11 | 同file `assertSpaceRetainedAfterOutputFailure` のseed全文と正確件数を除去 | Space/代表fileの存在、同名retry拒否、retry前後snapshot不変。生成内容はM4後の `internal/workspace` Scaffold |
| C12 | `git_independent_fixture_test.go::TestGitIndependentFixture` / `TestGitIndependentFixtureReservation` の自己生成JSON roundtrip、bad SHA、empty/invalid JSON表を削除 | 実journeyの製品受理・実reservation選択とC02のPATH隔離。共有 `gitIndependentEnvironment` / reservation等はcallerが残る限り保持 |
| C13 | `flow_evidence_test.go` の5Test、`verifyFlowProof` / `validFlowProof` 等と、`flow_live_integration_test.go::TestFlowJourneyLive` / `flowHostJob` / `flowHostGit` / `flowLiveCodeHash`、専用request/test modeを削除 | 現行Flow/GitIndependent/Assignment Journeyとdistinct live。`flowShellWords`をintegration helperへ分離、`flowRunModel` / hook mode / raw transportは現callerのため保持。旧Review専用記録field・schema分岐は最後caller確認後に削除 |
| C14 | `boundary_evidence_test.go::{TestBoundaryEvidenceRejectsIncomplete,TestBoundaryEvidenceSequence,TestBoundaryEvidenceCommand}`、`procedure_evidence_test.go::TestProcedureEvidenceSequence`、`execution_plan_evidence_test.go::TestExecutionPlanDistributionEvidence` を診断へ縮小 | 各checkerの正例・実行transport欠落・identity不一致程度。製品のBoundary/PendingHook/ExecutionApprovalはM3/M4後の所有suite、実機はBoundary/Procedure/HumanApproval |
| C15 | `hook_probe_test.go::TestHookProbeVerify` の22変形を撤去・縮小 | `hook_probe_live_test.go::TestHookProbeObservedTransport` / `TestHookProbeReplay` にasync start→poll→terminal、terminal不足、unknown wrapperの少数例を集約し診断へ |
| C16 | `hook_probe_live_test.go::TestHookProbeHelperProtocol` を診断へ、4processを2代表へ | standalone JSONとone-time Stop。subprocess helper自体はReplay/Liveのため残す |
| C17 | `agent_hook_probe_protocol_test.go::TestAgentHookProbeEvidence` の常時inconclusive5件とSession欠落で前段拒否されるnegative群を削除 | `measured_deny_and_allow_control` と適正なcontrolを使う `TestAgentHookProbeEvidenceObservedWire` |
| C18 | 同file `TestAgentHookProbeProtocol/parallel_unique` の12process、旧tool名の重複routeとFixture/fault_modesを削除 | 観測名のallow/denyと `TestAgentHookProbeProtocolObservedFault` の記録欠落・nonzero・保存失敗を少数の診断に保持 |
| C19 | 同file `TestAgentHookProbeFixture` のopt_in_and_platform/actual_worktrees/all_cases_have_requests、FixtureBudget/FixtureYield/FixtureObservedSchemaを削除 | 生成物の隔離・capture保全だけC20の1診断へ。prompt語句、固定timeout、Git自身の形状は保証しない |
| C20 | 同file EvidenceCollectedControl/ObservedWire/ObservedOptionalFieldsとFixtureCaptureの33変形を `TestAgentHookProbeEvidenceObservedWire` へ集約 | deny+allow、wrong-parent/provenance、未完了capture、必要なopaque-not-provenとraw bytes保全。実記録がなければ小さいsyntheticと明記し、観測済みと表示しない |
| C21 | `agent_hook_probe_live_test.go::TestAgentHookProbeLive` と専用prepare/collector/processを明示診断へ | 固定Codex互換調査として残す。G0-2〜6とaggregateのinconclusiveを製品成功に数えない |
| C22 | `hook_reliability_probe_test.go::TestHookReliabilityProbeProtocol` を診断へ、毎回buildと4経路×4caseを削減 | C26の同source binary、wire成功/nonzero/capture失敗fallbackの3代表。製品TerminalPersistence/MainHookは通常suiteに残す |
| C23 | 同file `reliabilityPrepare`、Protocol登録、Evidence/prepare_preserves_registrationの旧4引数hook自作正例を除去 | 実 `install.Codex` 生成の `--okf-binary` 付きhookを入力にする1診断へ。既存user hook・timeout・trust設定を保全。製品コードや旧互換を追加しない |
| C24 | 同file `TestHookReliabilityProbeEvidence` のchild_request文章と18変形、`hook_reliability_opaque_test.go::TestHookReliabilityOpaqueEvidence` の直積を縮小。`hook_reliability_probe_integration_test.go::TestHookReliabilityProbeOpaqueReceiptSynthetic` は削除 | CollectedEvidence/OpaqueReceiptへ正例・wrong identity・no Post・final-only・opaque mismatchを集約。記録再生とsyntheticを区別、製品解放/親宛制限は内部app |
| C25 | `assignment_process_test.go::TestAssignmentProcessRendezvous` を診断へ、壁時計30ms/1秒のassertを除去 | 有限pairとcleanupの1診断。`TestAssignmentProcess` はAssignmentLiveから使う。製品の実process予約競合は `internal/assignment` とAssignmentJourney |
| C26 | `flow_journey_integration_test.go::buildAIDLCBinary` の毎caller3buildを同process1回のaidlc/okfへ | 同source/flagsの不変binaryだけ共有。naturalの常時buildを削除、StageSkillsは既存 `AIDLC_NATURAL_JAPANESE_BINARY`。root/state/evidence/移転先binaryコピーは個別 |
| C27 | `flow_command_unix_test.go::TestFlowCommandFailureOutput`、`operationsNew/tdd/worktree`、`configure_help_integration_test.go::TestConfigureHelpExamples`、AssignmentJourney/現行liveの不要init/commit/HEAD/worktree準備を除去 | 別rootは通常directory＋必要な実bytesで保持。読まれないHEADとhelpにないCURRENT_HEAD置換を削除。C29のruntime追跡除外を確かめる最小Git利用だけ例外として残す |
| C28 | `flow_journey_integration_test.go::runGitIndependentBoundaryJourney` のfailed review→廃止advance非zeroを削除 | 同じ現在stageの `finish` 拒否とstate不変へ。review失敗が理由だと確認できる有効fixture・errorを使い、別の承認欠落だけで通る負例にしない。pass→承認→finishは維持 |
| C29 | `operations_integration_test.go::TestOperationsGitHandoff` / `TestOperationsGitConflict` のclone/merge/commit演習を撤去 | 新 `TestOperationsRuntimeBoundary` にruntime追跡除外・コピー先旧sessionなし・無管理Unit/review run拒否を、新 `TestOperationsCorruptState` に壊れJSONのCLI error/空stdout/bytes不変を集約。詳細は後述 |
| C30 | `relocation_integration_test.go::TestRelocationCommand` 後半再TDD/2worker/mergeとquoted pathの `bytes.ReplaceAll` oracleを削除 | 移転先の生成hook実canary、新root/新binaryの使用、user file保全、source snapshot不変。初期copyを通常directoryへ簡略化し、実installer全体は配布Native |
| C31 | `relocation_live_integration_test.go::TestRelocationCommandSelectionEvidence` / `TestRelocationLive` と最後callerの旧selectorを削除 | 現行OKF exerciseは `TestMemoryMetadataLive`、移転はC30/Native。旧selectorが常に前段拒否する正例を製品仕様へ合わせる変更はしない |
| C32 | `memory_metadata_integration_test.go::TestMemoryMetadataCommand` を `cmd/okf/memory_metadata_integration_test.go` へ移動 | 実okf binaryを直接呼び、Design CRUD/extension維持/CASとRuleのIntent非自動付与だけ残す。ADRの全CRUD反復を削除、search期待件数も残る2型に整合 |
| C33 | `memory_live_integration_test.go::TestMemoryMetadataCommandEvidence`、`stage_skills_live_integration_test.go::TestStageSkillsEvidence` を各3代表へ縮小 | 正例・実transportなし・denyされたread。辞書/versionの文字列保証は自然CLI/Native。Memory/StageSkillsの実full-read・実入力由来reportは保持 |
| C34 | `operations_integration_test.go::TestOperationsMultiIntent` をcreateだけへ、UnitConflictsをAssignmentJourneyの占有→解放へ統合。SaveRecoveryのOKF部分を `cmd/okf/memory_metadata_integration_test.go::TestMemorySaveRecovery` へ | 別process `TestOperationsConcurrentCAS` の1勝1敗、Intentの実FS失敗→retry、OKFのpre-save不変/postcommit部分成功JSON・hash・index復旧を保持 |
| C35 | Boundary/Procedure/HumanApproval/Memory/StageSkillsの各 `*_live_integration_test.go` を統合せず保持、全liveを明示診断へ。AssignmentLiveもmanual診断 | begin前禁止/repair、reopen、後続user turn承認、実full-read、実実行という異なる境界。固定version/env/既存trust経路を変えず新規live起動を要求しない |
| C36 | `flow_journey_integration_test.go` の全mutation後procedure呼出を除き、加算RED/GREENをdirect一周だけへ | 初期/stage遷移/reopenにprocedure照合。Unit A/Bの成果変更後の実testと新SHA/RunID/結果登録、最終stageの実testと新結果は保持。同じbytes/stageの余分な再実行だけ削る |

## file所有権とtag/helperの最小構成

M2マージ後の追加確認: `cmd/aidlc-dist/distribution_integration_test.go` の `fixtureGitRoot` は、通常・tag・OS別を含むsrcの参照検索で定義以外にcallerがない。C27の不要Git準備整理でこの関数だけを削除する所有範囲に含める。同fileの `distributionCommand`、`distributionOK`、write/snapshot/custom hookの共有helperは保持する。配布Native入口のcompile/discoveryと同head Distributionを検証に使い、削除だけのために実候補を追加buildしない。

単独writerが上表のaidlc test群、移動先okf test、projectroot/root_test.go、自然CLI integration、配布Nativeのstdin、`.github/workflows/ci.yml` の廃止step、`docs/development.md` 等の現行実行入口を所有する。製品Goは変更不要。親のRAM/計画更新もwriter稼働中は競合しないよう一方に寄せる。古いRAM・実行証拠のcommandは履歴として書き換えない。

- **通常**: main/Unix出力/公開hook testsは現tagを維持。`flow_command_unix_test.go` をintegration helperに依存させない。
- **integration**: deterministic journey、operations、relocation、configure help、assignment、OKF CRUD/recoveryを維持。`flowShellWords` は新 `shell_fixture_integration_test.go` に置く。唯一の通常側callerはなく、deterministic relocationが必要なのでdiagnostic内には閉じ込めない。
- **integration && diagnostic**: boundary/procedure/execution_plan evidence、hook_probe2file、agent_hook_probe3file、hook_reliabilityのprobe/opaque/integration3file、assignment_process_test、assignment_live、C35の5live file、残すflow_liveの共通transport、`observer_command_test.go` を同じ条件で揃える。agent process/helperはAssignmentLive、hookProbeTrustConfigはflowRunModel/AssignmentLive、observerは全liveで共有するため、一部だけtagを付けない。
- `flow_live_integration_test.go` は共有hook mode・モデル起動・保存だけへ縮小して残してよい。旧hostの `flowProofJob` は現callerが使うSession/Root/時刻等の必要fieldだけへ縮小し、旧proof fileから移す。Review記録を読むのは旧hostだけ。shell lexer以外のflow_evidence型/helperは最後callerを確認して除去する。
- binary共有は新 `binary_fixture_integration_test.go` の小さいpackage内helperでよい。現aidlc packageにTestMainはない。lazy `sync.Once`＋process所有 `os.MkdirTemp` を使い、TestMainで `m.Run` 後に削除する。最初のtestの `t.TempDir/t.Context` をcache寿命に使わない。異なるflags/source、可変環境、出力失敗は共有しない。OS別exe名を扱い、root/state/evidenceは共有しない。okf側はそのpackageで必要な1binaryだけbuildする小helperに留め、package横断cacheやframeworkは作らない。
- `fixtureInstall` は内部installを呼ぶsynthetic setupであり、実公開installerの証拠ではないことを維持。`fixtureProduct` のmemory転送は移動後の全callerが明示okfになった時だけ除去する。hookへの `--okf-binary` 接続は保持。`operationsResult` 等は残るcaller用に置き、OKF移動を理由に共通公開test packageを作らない。

## C29・移動失敗を隠さない条件

`TestOperationsRuntimeBoundary` は1回だけ小さいGit index smokeを残す。実配置後、共有されるIntent/knowledgeの代表と `.runtime` 内のsession/Unit/review sentinelを作り、`git init` / `git add -A` / `git ls-files` で前者が追跡され後者が追跡されないことを確認する。生成側と同じignore文字列を期待値としてコピーするだけにはしない。clone/branch/commit/mergeは不要。これはruntime非共有の実効性を保つための低コストな保持判断で、C27の不要Git準備と区別する。

その後は小さい通常directoryに共有対象だけをコピーし、旧sessionがなく、新しいbindingから管理runtimeのないUnit/review結果を受理しないことを確認する。現在schemaの有効なStepID・要求形・必要なSHAを使い、欠落runtime由来の拒否とstate bytes不変を確認する。40桁HEADだから旧testは無効、という理由付けはしない。M3で同じ固有境界が実製品へ既に移っていればそのtestへ統合し、CLI error代表だけ残す。

`TestOperationsCorruptState` は正常なstateのraw JSONを壊し、`intent show` がexit2・空stdout・診断を返してbytesを変えないことを直接検査する。Gitが衝突markerを作る手順や手動解決commitは削除する。C28のfinishをreview以外の不足で拒否する場合は、それをfailed review保証と記録しない。正常対照から最小の不足だけを残すか、M3の固有review拒否testへ集約する。

## M3固定headでのC28/C29追加確認

M3 merge `b958199018563f24178106f0ec365bb2cfbedd8b` を独立した読み取り担当が限定確認した。M4はこれらのflow製品・testsを変更しない。以下は旧testを製品へ合わせるための仕様変更ではなく、意図した拒否を実際に通すfixture条件の具体化である。

C28: `src/internal/flow/review_test.go::TestFlowReviewFailRecorded` はfail結果の保存のみ。`execution_approval_test.go::TestExecutionPlanApprovalBootstrapFinish` は承認待ちの拒否であり、`transition_test.go::TestFlowTransitionGatesAndStages` もpass時だけ承認を作るfixtureのため、failed review固有の代替とは扱えない。単にadvanceをfinishへ置換しない。最小案は、同じstageでpass結果と必要な計画・成果承認を用意し、Finish前に同じTargetへ再assign/accept failを記録すること。直前のApproval=approved、draftなし、End Sensor=passを確認し、finishのexit1・空stdout・result changed診断・state bytes不変を確認する。判定順の根拠は `approval.go::Finish`（固定head260–276行）。既存pass→承認→finish成功は維持する。この案は未実行で、製品の既存制約で成立しない場合は別エラーによる拒否を成功扱いせず親へ返す。

C29: `src/internal/flow/assignment_test.go::TestUnitAssignmentUnmanagedResult` は、正常managed claimと実bytes・正しいStepID/Session/RunID/SHA/結果証拠を用意し、RegistryのReservationsだけを空にしてUnit result拒否とstate/runtime bytes不変を所有する。`TestUnitAssignmentLegacyReassign` は現在schemaのneeds_confirmation Unitで予約なしreassign拒否を所有する。共有copy先のruntime file不存在、旧sessionの不在、Review runtime file不存在は同等保証として確認できないため、予定のCLI代表を維持する。

小Git index smokeの後に共有fileだけをcopyし、旧sessionのinspect結果でIntentが空であることを確認する。CLIでは、needs_confirmation・現在StepID・正しい実bytes SHAを用いるUnit confirmと、現在Target・整合する実root/session・pass/summaryを用いるreview acceptを各1回残す。それぞれの診断が `aidlc/.runtime/flow/units` と `reviews` の欠落に由来し、state bytesが不変であることを確認する。無管理結果の詳細な直積はM3所有に任せ、異なるpublic routingを一方だけへ統合しない。

## 順序付きsliceとexact targeted commands

以下はwriterへ渡す実行予定であり、この計画担当は未実行。全commandは作業root。変更前の生存側を先に確認し、削除・移動後に同じtargetedを実行する。新移動testは先に追加して `ALREADY_GREEN` を確認する。compile/listはtest実行成功と別記する。sliceは1 work unit内の順序で、別Issueへ分割しない。

1. **main・root移動（C05–C11）**。入力/出力境界を残して直積とwrapperを除去。

   `go test -count=1 ./src/cmd/aidlc -run '^Test(Space(Creator|Lister|Switcher)(RootInput|LazyCLIInputs)|SpaceAdapterErrors|MainSpace(List|SwitchClosedPipes|ListClosedPipes|CreateClosedPipes)|MainRootCommandsKeepSIGPIPE|MainHookCommand)$'`

   `go test -count=1 ./src/internal/projectroot -run '^TestResolve(AncestorFiles|PreservesErrors|InstallationContext)$'`

2. **helper依存分離・旧host/selector撤去・診断tag（C13/C21/C31/C35）**。上記tag閉包を一括でcompile可能にし、通常suiteのhelper依存を取り残さない。

   `go test -run '^$' ./src/cmd/aidlc`

   `go test -tags=integration -run '^$' ./src/cmd/aidlc ./src/cmd/okf`

   `go test -tags='integration,diagnostic' -run '^$' ./src/cmd/aidlc`

3. **小診断へ縮小（C14–C20/C22–C25/C33）**。命名は生存入口を維持する。以下は外部Codexを呼ばないreplay/protocolだけ。必要実記録がないtestは小syntheticに明記し、env依存skipだけで保証を移したことにしない。

   `go test -tags='integration,diagnostic' -count=1 ./src/cmd/aidlc -run '^Test(BoundaryEvidence(Sequence|Command)|ProcedureEvidenceSequence|ExecutionPlanDistributionEvidence|HookProbe(ObservedTransport|Replay|HelperProtocol)|AgentHookProbe(Protocol|ProtocolObservedFault|EvidenceObservedWire)|HookReliabilityProbe(Protocol|CollectedEvidence|OpaqueReceipt)|HookReliabilityOpaqueEvidence|AssignmentProcessRendezvous|MemoryMetadataCommandEvidence|StageSkillsEvidence)$'`

4. **build共有・必要境界を先に移動・Git準備を削減（C26/C27/C29/C34）**。M3の無管理runとM4の所有者を確認し、runtime/corruptの移動先を先に成立させる。実process CAS・実保存失敗は通常integrationのまま。

   `go test -tags=integration -count=1 ./src/cmd/aidlc -run '^TestOperations(MultiIntent|RuntimeBoundary|CorruptState|ConcurrentCAS|SaveRecovery)$'`

   `go test -count=1 ./src/cmd/aidlc -run '^TestFlowCommandFailureOutput$'`

5. **OKFの公開binaryへ移動（C32/C34）**。Design/Ruleと実保存失敗だけの小さいfixtureを移し、aidlcのmemory専用caller/helperを除去。

   `go test -tags=integration -count=1 ./src/cmd/okf -run '^TestMemory(MetadataCommand|SaveRecovery)$'`

6. **二つのjourney・relocation canaryへ縮小（C02/C12/C28/C30/C36）**。実行証拠のないexit0を作らない。loopでは一周を実行せず、生存入口のcompile/discoveryとM3/M4の必要な固有testだけ。下記5名が存在することを確認。

   `go test -tags=integration -list '^Test(FlowJourney|GitIndependentJourney|AssignmentJourney|RelocationCommand|ConfigureHelpExamples)$' ./src/cmd/aidlc`

7. **stdin保証を実候補へ移動（P26）・手順整合**。既存Nativeの空PATH/env・binaryをそのまま使い、stdin用に小さいexec.Cmdだけ追加。再buildや新helper frameworkは不要。削除名のCI stepとdocs例を同時に更新する。

   `go test -tags=integration -list '^TestReleaseCandidate(Metadata|Native)$' ./src/cmd/aidlc-dist`

   `git diff --check`

末尾で対象Go fileのみgofmt適用、上記targetedを一括確認して親へ一度返す。通常/tag/OS別から最後callerを検索し、使われなくなったtest専用型・helper・fixture・importだけ除く。productionへ到達しない旧コードが見つかっても、今回所有外のproduction変更はせず親へ範囲を返す。

## review・final・残件

親の独立reviewは固定base/head、34候補の処置→生存先、tag閉包、cache寿命、実害のない複写assert再導入、前段の別errorで通る負例を確認する。通常/integration/diagnostic各入口を記録し、manual liveのskipを成功にしない。

blocking finding解消後、親だけがread-only finalを開始する。上位計画の全package/coverage/race/vet/format/moduleと、最終名の `TestFlowJourney|TestGitIndependentJourney|TestAssignmentJourney|TestRelocationCommand|TestConfigureHelpExamples`、OKF移動先、非live diagnostic生存群を各所定頻度で実行。chmod境界は権限が効くOSでの実行が必要。診断の現在のshell前提をWindows互換へ拡張せず、必要な通常/tagのcompileと既存OS条件を明示する。

P26は同head GitHub Distribution Package＋3OS Native/bootstrap（PS5.1/7含む）の実成功を必須にする。ローカル30binary再buildは行わない。M5でCI整理全般はせず、廃止/移動名だけを修正する。全checks成功後のPR merge/Issue closeは親が承認済み手順で実施し、公開/tag操作はない。

重要な未承認仕様選択はない。未確定なのはM3/M4 merge後の正確な生存test名、再利用できるsanitized実記録の有無、不要helperの最終callerで、親がbaseline固定時に確定する。実記録なしは小synthetic診断を理由付き保持、保証移動が成立しない場合は旧固有assertを残して理由を返す。製品の仕様を変えて古いtestに合わせない。問題があれば当該PRをrevertする別PRを用い、他の区切りや他者の変更をresetしない。

## 実装時のfixture整合と結果

C29 CorruptStateのexit1は誤期待だった。現行flow/store.go:252のfs.ErrInvalidがcli/command.go:74でexit2/invalid JSONへ分類されるため、非成功・空stdout・破損診断・state bytes不変という主保証を保ってexit2へ訂正した。親が同work unit内で修復を許可し、新例成功後に旧GitConflict演習を除去した。製品Goは変更しない。

C27の別review rootでは旧worktreeに含まれた実 `.agents/.codex` bytesが必要なので、通常directoryへその内容をcopyする。C23はinstallのcanonical rootと現在の--okf-binary付きhookを使い、生成しないtrust設定は既存user fileとして用意する。詳しい処置と検証境界は[M5結果](test-reduction-m5-result.md)を参照。
