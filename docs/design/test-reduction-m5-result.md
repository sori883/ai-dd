# M5 テスト削減の実装結果

[Issue #212](https://github.com/sori883/ai-dd/issues/212)、work_unit_id `test-reduction-m5`、verification_mode `loop`。[承認済み計画](test-reduction-m5-plan.md)のS1–S7・34候補を単独writerで整理した。製品Go・公開CLI・保存/権限・7asset契約・外部moduleは変更していない。

作業rootは `/Users/const/sori883/ai-dd-release`、branch `codex/test-reduction-m5`。開始/終了HEADは `78e70f765606336f85eb8bb206b2e5d243e1fbfd`、commit操作なし。実装とloop確認を終了した段階であり、独立review・final・同head CI・mergeは親が続ける。

## 候補別の処置

| ID | 実施した処置 | 生存保証・保持理由 | 証拠 |
|---|---|---|---|
| P26 | 旧standalone binary test/CI stepを削除。同候補Nativeへstdin JSONを追加。 | 空PATHで入力由来forbidden_phrase・有効JSON。 | Q11 discovery。3OS実行は親final待ち |
| C02 | 2×2一周をno-git/directとdetect-git/unitsの2一周へ。 | 明示製品PATH、stub未呼出・.gitなし。 | Q10 discovery。一周はfinal待ち |
| C05 | lazy入力44行をhelp/versionと各route不正1例へ縮小。 | 早期cwd/env/filesystem未呼出。文法表はinternal/cli。 | Q1 |
| C06 | getwd/getenvの回数・順序・constructor再検査を除去。 | 3adapterのRootInput全field、生name、返却値。 | Q1 |
| C07 | 4旧error testをSpaceAdapterErrorsへ統合。 | cwd失敗1代表、create/read/switchの独立causeとzero result。 | Q1 |
| C08 | PublicCutoverとRuleSkillSeparationHelpのwrapper/空fileを削除。 | M4後のinternal/cli・okfcliの公開入口、文法、help接続。 | Q1、既存所有者の静的照合 |
| C09 | 旧root wrapperを削除しInstallationContext3例へ移動。 | application祖先・missing・new install。既存ancestor/error保持。 | Q2 |
| C10 | closed pipeの直積をList4、Switch2、Create2、root2へ。 | 実process SIGPIPE、stdout/stderr・JSON/nonJSONの別経路。 | Q1 |
| C11 | seed全文と生成件数を除去。 | Space/代表Rule存在、同名retry拒否、retry前後snapshot不変。 | Q1 |
| C12 | GitIndependentFixture/Reservationの自己JSON表を削除。 | 実journeyの受理・実reservation選択。共有env/selectorは保持。 | Q10 discovery |
| C13 | 旧proof5test・host・FlowJourneyLive・request/test modeを撤去。 | shell lexerはintegration専用file、現行liveのhook/model/transport保持。flowProofJobはRoot/Session/時刻だけ。 | Q3–5 compile |
| C14 | Boundary/Procedureの全欠落反復とapproval辞書を小診断へ。 | 正例・transport欠落・canary/target/request不整合。syntheticと明示。 | Q6 |
| C15 | HookProbeVerifyと専用fixtureを削除。 | ObservedTransportのasync start/poll/terminal、unknown wrapper・terminal欠落。既存Replayは手動記録。 | Q6（Replayは記録envなしskip） |
| C16 | helper subprocess4行をStop初回/2回目の2行へ。 | standalone JSONとone-time Stop。実live用helperは保持。 | Q6 |
| C17 | 前段拒否のinconclusive表を削除。 | measured_deny_and_allow_controlと整合するObservedWire。 | Q6（ObservedWire） |
| C18 | 12process反復と旧tool名経路・fixture fault表を除去。 | 観測名deny/allow、ObservedFaultの欠落/nonzero/保存失敗。 | Q6 |
| C19 | Fixture形状/予算/yield/schema/captureの自己検査群を削除。 | ObservedWireの分離したdeny/allow rootとraw bytes保存。 | Q6 |
| C20 | 33変形をObservedWireの少数代表へ。 | synthetic control・wrong parent/provenance・未完了capture・opaque-not-proven・raw bytes不変。実測とは呼ばない。 | Q6 |
| C21 | AgentHookProbeLiveとprocess/collector一式へdiagnostic tag。 | 固定Codex調査。G0のinconclusiveを製品成功へ換算しない。 | Q5 compile。実live未起動 |
| C22 | 毎回単体buildを共有binaryへ。Protocolをterminal/invalid wireとcapture失敗fallbackへ縮小。 | 製品wire/exit/raw保存と失敗fallback。 | Q6 |
| C23 | 旧4引数hook自作正例を撤去。実install.Codex登録を使用。 | --okf-binary付きcommand、既存user hook/timeout/trustと元登録不変。 | Q6 |
| C24 | 旧Evidence変形・Opaque直積・OpaqueReceiptSyntheticを削減。 | CollectedEvidenceの正例/wrong identity/no Post/final-only、OpaqueEvidenceの正例/body mismatch。手動receiptは保持。 | Q6（手動receiptはenvなしskip） |
| C25 | missing peerの壁時計assertを除去し有限pair診断へ。 | pair終了・記録、duplicate nonce拒否。AssignmentLive用processは保持。 | Q6 |
| C26 | aidlc/okfをlazy sync.Onceとprocess所有temp/TestMain cleanupで共有。natural常時buildを除去。 | 不変binaryだけ共有。test TempDir/Contextをcache寿命に使わずroot/state/evidenceは独立。 | Q4–9 |
| C27 | 不要init/commit/HEAD/worktreeと無参照fixtureGitRootを除去。 | 別rootへ必要な実fileをcopy。RuntimeBoundaryのGit index smokeのみ製品接続として保持。G0の明示診断用worktreeは調査fixtureとして保持。 | Q4–8、Q10–11 discovery |
| C28 | 廃止advance拒否を同stage finish拒否へ置換。 | pass→必要承認→同Target再assign/fail、approved/noDraft/endSensorpassを前提assert。result changed/exit2/空stdout/bytes不変。後続pass成功を保持。 | Q10 discovery。実行は親final待ち |
| C29 | GitHandoff/GitConflict演習をRuntimeBoundary/CorruptStateへ置換。 | 実Git indexのruntime除外、共有fileだけcopy、旧session空、現在Stage/StepID/SHAのUnit confirm/review acceptがruntime欠落で拒否。壊れJSON非成功/空stdout/bytes不変。 | Q7 |
| C30 | relocation後半再TDD/2worker/mergeと置換oracleを削除。 | 通常dir copy、実生成hookの新root/binary canary、user file保全、source snapshot不変。 | Q10 discovery。実行はfinal待ち |
| C31 | 旧Relocation live/selectorを削除。 | 現行MemoryMetadataLiveとRelocationCommand/Nativeへ責任を整理。 | Q5 compile・Q10–11 discovery |
| C32 | MetadataCommandをcmd/okfへ移動。ADR全反復を除去。 | 実okf binaryのDesign CRUD/extension/CAS、RuleのIntent非付与。検索件数1へ整合。 | Q9 |
| C33 | Memory/StageSkills evidence各3代表へ。 | 正例・実transportなし・denyされたread。実full-read/入力由来reportは維持。 | Q6 |
| C34 | MultiIntentをcreate隔離へ、UnitConflictsを既存AssignmentJourney所有へ。OKF保存復旧をcmd/okfへ移動。 | 別process CAS1勝1敗、Intent FS失敗/retry、OKF pre-save不変・postcommit JSON/hash/index復旧、Unit占有→解放。 | Q7/Q9実行・Q10 discovery |
| C35 | 独立live5種とAssignmentLiveを維持してdiagnostic tagへ。 | begin/repair、reopen、後続承認、実full-read/実行の別境界。model/env/trust入口は維持。 | Q5 compile。実live未起動 |
| C36 | 全mutation後procedureとUnitsの同bytes再実行を除去。加算RED/GREENはdirectだけ。 | 初期/遷移/reopen procedure、A/B成果変更後の実test/SHA/RunID/結果登録、最終stageの新結果。 | Q10 discovery。一周はfinal待ち |

## 検証の意味と途中修復

変更前の生存検査をALREADY_GREENとして確認し、保証移動後・削減後に対象を再確認した。人工的REDは作っていない。末尾11commandは全てexit 0。Q1/2/6/7/8/9はtargeted実行、Q3–5はcompileだけ、Q10/11は正確な入口名のdiscoveryだけである。Q6のHookProbeReplayとOpaqueReceiptは外部記録envなしのskipであり、記録再生/実機成功ではない。選択したsynthetic/固定transport診断の成功と区別する。gofmt・git diff --checkも成功。

計画に従い、CLI一周・AssignmentJourney・RelocationCommand・ConfigureHelpExamplesと配布Nativeはloopで実行していない。C28の新finish負例を含む実一周の成功、P26の同候補3OS Native成功は親finalの必須残件。compile/discoveryだけで保証移動の実行成功とはしていない。外部Codex liveは起動していない。

CorruptStateの計画はexit 1/invalid characterを期待していたが、現行 `flow/store.go:252` は不正JSONをfs.ErrInvalidで返し、`cli/command.go:74` がexit 2へ分類する。親の修復許可を受け、exit 2/invalid JSON・空stdout・元bytes不変へ一意に整合した。新例成功後に旧GitConflictを削除。これは製品修正や有効REDではない。

通常directoryへ移すUnix review fixtureは、旧worktreeにあった実 `.agents/.codex` bytesが必要だったため、その2directoryだけcopyして同targeted成功を確認した。実install登録を使う診断ではcanonical rootへ合わせ、生成しないtrust fileは既存user設定のfixtureとして明示作成した。削除時のunused import/構文片/残存参照も定型修復後に同targeted成功を確認した。診断checkerの実記録不足を理由に製品の判定を変えていない。

`fixtureProduct` のmemory転送は既存journeyの文書準備callerが残るため保持した。observer・agent process・hook trust・shell lexer等も通常/integration/diagnosticのcallerに合わせて配置。G0の明示診断が観測する別worktree準備は残すが、通常製品suiteのGit要件ではない。

## 正確な末尾command

Q1 (targeted): exit 0

```sh
go test -count=1 ./src/cmd/aidlc -run '^Test(Space(Creator|Lister|Switcher)(RootInput|LazyCLIInputs)|SpaceAdapterErrors|MainSpace(List|SwitchClosedPipes|ListClosedPipes|CreateClosedPipes)|MainRootCommandsKeepSIGPIPE|MainHookCommand)$'
```

Q2 (targeted): exit 0

```sh
go test -count=1 ./src/internal/projectroot -run '^TestResolve(AncestorFiles|PreservesErrors|InstallationContext)$'
```

Q3 (compile): exit 0

```sh
go test -run '^$' ./src/cmd/aidlc
```

Q4 (compile): exit 0

```sh
go test -tags=integration -run '^$' ./src/cmd/aidlc ./src/cmd/okf
```

Q5 (compile): exit 0

```sh
go test -tags='integration,diagnostic' -run '^$' ./src/cmd/aidlc
```

Q6 (targeted): exit 0

```sh
go test -tags='integration,diagnostic' -count=1 ./src/cmd/aidlc -run '^Test(BoundaryEvidence(Sequence|Command)|ProcedureEvidenceSequence|ExecutionPlanDistributionEvidence|HookProbe(ObservedTransport|Replay|HelperProtocol)|AgentHookProbe(Protocol|ProtocolObservedFault|Evidence|EvidenceObservedWire)|HookReliabilityProbe(Protocol|CollectedEvidence|OpaqueReceipt)|HookReliabilityOpaqueEvidence|AssignmentProcessRendezvous|MemoryMetadataCommandEvidence|StageSkillsEvidence)$'
```

Q7 (targeted): exit 0

```sh
go test -tags=integration -count=1 ./src/cmd/aidlc -run '^TestOperations(MultiIntent|RuntimeBoundary|CorruptState|ConcurrentCAS|SaveRecovery)$'
```

Q8 (targeted): exit 0

```sh
go test -count=1 ./src/cmd/aidlc -run '^TestFlowCommandFailureOutput$'
```

Q9 (targeted): exit 0

```sh
go test -tags=integration -count=1 ./src/cmd/okf -run '^TestMemory(MetadataCommand|SaveRecovery)$'
```

Q10 (discovery): exit 0

```sh
go test -tags=integration -list '^Test(FlowJourney|GitIndependentJourney|AssignmentJourney|RelocationCommand|ConfigureHelpExamples)$' ./src/cmd/aidlc
```

Q11 (discovery): exit 0

```sh
go test -tags=integration -list '^TestReleaseCandidate(Metadata|Native)$' ./src/cmd/aidlc-dist
```

変更前/移動後/修復/末尾のcommand・exit・outputは `/tmp/m5-evidence.json`。変更file一覧とSHA-256は `/tmp/m5-files-sha256.txt`、tracked差分hashは `/tmp/m5-diff-sha256.txt` に保存する。新規binary/shell/OKF fixtureと本結果docもfile一覧へ含める。

## PR review修復（同work unit）

開始HEADは `167db50b87b387a7b1860edf67f34cc46829b9c4`。初回PR CI [34804857150](https://github.com/sori883/ai-dd/actions/runs/34804857150) の両Go Qualityで、Flow/GitIndependentJourneyがresult changed・code2を返し、code1の誤期待で失敗した（親ログ `/tmp/ai-dd-m5-first-ci-failed.log`）。Finishのfs.ErrInvalidをCLIがexit2へ分類する現仕様に期待値を整合した。製品修正や有効REDではない。

- Relocationの書換え許容を現行6pathへ整合し、生成hookのargsをそのまま直接実行する。既存の移動先runtime・移動元bytes保全を維持。共有binary directoryは初回配置前にcanonical化し、失敗時のerrorとTestMain cleanupを維持する。
- OKF Metadataは3番目の本文で古いhashを再送し、exit2・Concept hash conflict・空stdout・保存bytes不変を検査。no-op拒否による偽passを防ぐ。
- ObservedWire正例をsynthetic CRLFに置換し、raw bytes比較を保持。Q6へTestAgentHookProbeEvidenceを加え、noPost/duplicateStartも実行する。
- Recorded/ObservedControl、execution-plan、memory/stage証拠、hook lock、projectrootの削除case専用分岐と空fixture fileを撤去。実機評価器・collectorは保持。

修復前の非live Q6・OKF Metadata・projectrootはすべてALREADY_GREEN。修復後も同対象が成功し、integrationおよびintegration+diagnostic compile、5journey名のdiscoveryが成功。正確なcommand・exit・出力は `/tmp/m5-review-fixes.json` に追記保存する。compileのno-tests出力とdiscoveryは実挙動の成功とは扱わない。実journey・全normal/race/vet・Native・liveは本修復で未実行で、親finalの境界を維持する。
