# テスト削減の実装ロードマップ

対象は `/Users/const/sori883/ai-dd-release`、開始commitは `adc4682ca265c9bda4434a94d9d99cff1f4b9394`。現在の製品挙動を変えず、同じ事故を繰り返し検査するテスト、旧実験、不要な準備を削減する。件数・coverage率を目標にせず、残る検査がどの実害を検出するかを各PRに記録する。

ユーザーの「それらを削除」「区切って対応」「Issue作ってmrでマージ」という直接依頼と `docs/ram/decisions/2026-09-14-test-reduction-approved.md` が全区切りの実装許可である。各区切りの詳細化・Issue・単独writer・独立review・read-only final・現headのGitHub checks成功を経て、PRを順番にmainへmergeする。同じ許可範囲の再承認は不要。区切りはdocsで管理し、GitHub milestoneは作らない。

監査正本は `docs/test-suite-reduction-audit.md`。151個の候補IDは削除するTest件数ではなく、一部は複数Test・assert・fixtureにまたがる整理項目である。

## 6つの区切りと重複しない割付

| 順 | Issue/PRの目的 | 候補ID（このPRのみで完了判定） | 件数 |
|---|---|---|---:|
| M1 | 別名による再実行とtest helperの自己検証を除く | P01–P05、A01、F01、C01、C03–C04 | 10 |
| M2 | 配布・配置資材・自然日本語の重複検査と準備を集約 | P06–P25、P27–P31、F05 | 26 |
| M3 | 工程・担当管理のfixtureと同じ拒否表を集約 | F02–F04、F06–F41 | 39 |
| M4 | アプリ接続・CLI・Space・OKFの多層反復を集約 | A02–A37 | 36 |
| M5 | CLI一周の準備を共有し、旧実験を撤去 | P26、C02、C05–C36 | 34 |
| M6 | 残った検査の所有者を明確にしてCI重複を除く | W01–W06 | 6 |

P01–31、A01–37、F01–41、C01–36、W01–06を各1回だけ割り付けた。M1の明確な削除、M2の資材正本、M3/M4のdomainとadapter、M5のtag・実行入口、M6のCI所有者という検証上の境界で分ける。すべて1 Issue/PR・1 writer・1 work unitを既定とし、区切り内の項目数だけで細分化しない。各PRを単独で正常な状態にしてから次のPRへ進む。

## 共通の変更条件

- 公開API、保存形式、承認・親子権限、予約/復旧、同じ版の導入・移転、7配布asset、license/sourceの契約は維持する。外部module/tool、一般検証framework、公開/tag操作を追加しない。
- 条件付き候補は生存先へ固有assertを移してから削除する。表の移動だけで同じ直積を温存せず、validatorの意味ある入力分類と各入口の接続代表に分ける。
- 共有できるのは同じsource/flagsのbinaryと不変の配布入力等。Space、Intent、registry、mutable archive、証拠は各caseで分離する。
- test専用関数の最後のcallerを消したら、helper・fixture・importも同時に除く。共有callerがあるfileを丸ごと消さない。旧内部productionは呼出なしを通常/tag/OS別から確認してから削除する。
- 既存の正しい挙動を検査へ移す場合は先に生存testを実行し `ALREADY_GREEN` を記録する。人工的なREDやcoverage補充は不要。削除により製品の既存bugが判明した場合は削減の成功条件を緩めず、親へ範囲を返す。
- 現在の手順にあるtest名は削除と同じPRで更新する。過去RAMのRED/GREEN・観測記録は歴史的事実として書き換えず、新計画/RAMから現行入口を示す。

## M1の実装可能なwork unit

`work_unit_id=test-reduction-m1`、`verification_mode=loop`。単独writerに下記16 Go fileと `.github/workflows/distribution.yml`、`docs/development.md`、親が確定した計画/RAMの更新を一括所有させる。親はwriter稼働中の編集を停止する。監査doc3件と承認RAM・索引・ロードマップは最初のPRへ含める。他作業treeの変更は戻さない。

### 正確な削除・保持対象

パスは作業rootからの相対表記。

| 所有file | 削る範囲 | 生存する保証/関数 |
|---|---|---|
| `src/cmd/aidlc-dist/distribution_integration_test.go` | `TestDistributionArchives`、`TestDistributionJourney`、`TestNaturalJapaneseDistributionArchives`、`TestNaturalJapaneseDistributionJourney`、`TestDistributionBinaryPath`、専用 `fixtureBinaryPath` | `release_integration_test.go` の `TestReleaseCandidateMetadata` / `TestReleaseCandidateNative`。`distributionCommand`、`distributionOK`、write/snapshot、custom hook等の共有helperは保持 |
| `src/cmd/aidlc-dist/release_test.go` | `TestVersionedAssetsLicense` | `TestBundledRelease` と現候補の正本license照合。`releaseFixture`、入力license拒否は保持 |
| `src/cmd/aidlc-dist/main_test.go` | `TestProductCLI`。`TestDistCommand` の構文拒否6行で使う `releaseFixture` を未存在入出力path＋有効な版/commit等の最小optionsへ | unknown flagを含む `TestDistCommand`、exit2/空stdout/error/出力なし。成功・operational failureのfixtureは維持 |
| `src/cmd/aidlc-dist/release_integration_test.go` | `TestReleaseCandidateNativeSelection`、`TestReleaseCandidateProjectDirectory` | `selectBundleNative`、実project directory helper、Metadata/Nativeは保持 |
| `src/bootstrap/bootstrap_test.go` | `TestBootstrapOutputStreams`、`TestBootstrapProjectDirectory`、TestMainの専用 `BOOTSTRAP_STREAM_HELPER` 分岐 | `TestBootstrap`、`TestBootstrapPowerShell`、`TestBootstrapCandidateNative`。`fixture_test.go` の `bootstrapCommandOutput` / `sameProjectDirectory` は保持 |
| `src/internal/app/rule_skill_separation_test.go` | `TestRuleSkillSeparationHookApprovalPending` | `TestExecutionPlanCLIPendingHook` |
| `src/internal/app/hook_split_cli_test.go` | `TestHookSplitCLIRepair` | `TestBoundaryHookRepairAndBegin`、`TestIntentDocumentsHookRepair`、`TestIntentDocumentsUnregisteredInputRepair`、`TestProcedureDriftBlocksDocumentRepair`、`TestOKFSkillReadApprovalPending` |
| `src/internal/workspace/rule_skill_separation_test.go` | `TestRuleSkillSeparationRuleCopy`。他要素がないためfile削除 | `TestCreateSpaceOKF`、`TestCreateSpaceOKFFallbackAndInvalid` |
| `src/internal/okfcli/command_test.go` | alias `TestOKFMemoryContract` | `TestOKFCommand` |
| `src/internal/okfmemory/metadata_test.go` | alias `TestOKFMemoryContract` | `TestMetadataInputBuildAndPreserve`、`TestSearchIntentID` |
| `src/internal/flow/boundary_sensor_test.go` | `TestEndSensorUnitCommandPair` | `TestVerificationResults` |
| `src/internal/flow/unit_test.go` | `TestFlowUnitDependencyContent` | `TestUnitWithoutGit` |
| `src/cmd/aidlc/flow_journey_integration_test.go` | `TestBoundaryJourney`、`TestProcedureJourney` | `TestFlowJourney` と `runBoundaryJourney` |
| `src/cmd/aidlc/documents_journey_integration_test.go` | `TestIntentDocumentsJourney`。他要素がないためfile削除 | `TestFlowJourney` |
| `src/cmd/aidlc/observer_command_test.go` | `TestObserverBinaryArgs` | `observerHookCommand` の現caller、`TestMainHookCommand`、Codex split asset tests |
| `src/cmd/aidlc/hook_probe_live_test.go` | `TestHookProbeCanonicalRoot`、`TestHookProbeTrustConfig` | `hookProbeCanonicalRoot` / `hookProbeTrustConfig`、ObservedTransport/Replayと既存live入口。liveの実行はM1に追加しない |

24個のTest関数の削除と、P03の読まれないfixture生成の削減である。製品Goの変更はない。

`.github/workflows/distribution.yml` のPackage実行regexは、現状ArchivesとMetadataの両方を実行する。削除名を外して `^TestReleaseCandidate(Metadata|MetadataValidation)$` にする。Metadata/Nativeの入口存在確認、artifact ID、3OS Native、PS 5.1/7、draft前再照合は変えない。CI全般の整理はM6が所有する。

`docs/development.md` の現行実行例3箇所に残る BoundaryJourney / ProcedureJourney / IntentDocumentsJourney はすべて `TestFlowJourney` へ改める。旧設計計画の過去の実行証拠は置換せず、ロードマップから現行入口の対応を明示する。

### 実装順とexact targeted commands

全commandは作業rootで実行。変更前に以下の生存testを実行し、各slice内で削除・import整理・gofmt後に同じcommandを再実行する。初回成功は `ALREADY_GREEN`。`-list`はcompile/discoveryの確認であり、実候補を検証した成功には数えない。

1. **配布aliasと小helper（P01–P04）**。共有helperを残し、workflowの削除名も同時に修正。

   `go test -count=1 ./src/cmd/aidlc-dist -run '^Test(DistCommand|BundledRelease|ReleaseLicenseInputs|FiveProductManifestRequiresSixTargets)$'`

   `go test -tags=integration -count=1 ./src/cmd/aidlc-dist -run '^TestReleaseCandidateMetadataValidation$'`

   `go test -tags=integration -list '^TestReleaseCandidate(Metadata|MetadataValidation|Native)$' ./src/cmd/aidlc-dist`

   listに3名すべてがあることを確認。実候補Metadata/Nativeの実行はfinalへ集約する。

2. **bootstrap helper自己検証（P05）**。専用child modeを消し、実scriptが使う出力分離・directory helperは維持。

   `go test -count=1 ./src/bootstrap -run '^TestBootstrap$'`

   `go test -list '^TestBootstrap(PowerShell)?$' ./src/bootstrap`

   PSの非Windows skipをGREEN扱いしない。PS両engineと実candidateは同headのDistribution CIで実行する。

3. **app/Space/OKF alias（A01）**。移動ではなく再呼出の入口だけ除く。

   `go test -count=1 ./src/internal/app -run '^Test(ExecutionPlanCLIPendingHook|BoundaryHookRepairAndBegin|IntentDocumentsHookRepair|IntentDocumentsUnregisteredInputRepair|ProcedureDriftBlocksDocumentRepair|OKFSkillReadApprovalPending)$'`

   `go test -count=1 ./src/internal/workspace ./src/internal/okfcli ./src/internal/okfmemory -run '^Test(CreateSpaceOKF|CreateSpaceOKFFallbackAndInvalid|OKFCommand|MetadataInputBuildAndPreserve|SearchIntentID)$'`

4. **flow alias（F01）**。結果のUnit対応・Git不要の予約→提出→統合→解除を保持。

   `go test -count=1 ./src/internal/flow -run '^Test(VerificationResults|UnitWithoutGit)$'`

5. **CLI journey aliasとobserver/probe自己検証（C01/C03/C04）**。実一周helperを残し、現行手順のtest名を変更。

   `go test -count=1 ./src/cmd/aidlc -run '^Test(MainHookCommand|HookProbeObservedTransport|HookProbeReplay)$'`

   `go test -tags=integration -list '^Test(FlowJourney|GitIndependentJourney)$' ./src/cmd/aidlc`

   listに2名があることを確認。一周の実行はfinalで1回行い、loopでは行わない。

work unit末尾に変更Go fileだけgofmtを適用し、上記targeted群・discovery・`git diff --check`をまとめて確認して一度返す。親は全差分と同targeted群を境界で一度確認する。通常/tag/OS別の参照・現在の検証手順を静的検索し、廃止名によるゼロ件実行を残さない。

## M2以降の所有範囲・順序・判断

### M2: 配布・資材・自然日本語

所有は監査P対象の `src/cmd/aidlc-dist`、`src/bootstrap`、`src/internal/install`、`src/internal/release`、`src/harness`、`src/core`、`src/internal/naturaljapanese`、`src/cmd/natural-japanese-go/main_test.go` と F05の `src/internal/flow/content_test.go`。P26のbinary integrationはM5へ残す。

順序は、(1)必要asset/role/path/license・参照・tokenの独立期待値と実配置bytes/regular/mode/umask/Pathsを正本へ統合して固定SHA fixtureを撤去、(2)文面/存在/件数表を削り固有Rule selector・agent権限・quote・bootstrap上限を残す、(3)fresh/relocate/同要求retry・衝突表を集約、(4)新bundleに無変更/途中失敗/入力拒否を移して旧単品branchを撤去、(5)候補を一度解析し6target Metadataと自target Nativeの重複を削減、(6)自然日本語のログ専用・二重CLI表・同じtokenizeを縮小。

決定: P06/P07のmanifest/checksumを再計算したsource/license改変の代表は保持。P08は同一不変入力だけ共有し、draft前の別境界は再照合。P09は旧 `packageArchives` 単品branchと専用schema/helperのみ削り、現 `packageRelease` に通る `validateInputs` を削らない。P10はPS5.1/7各公開ScriptBlockに8mode、`-File`は正常/子exit/catch代表を残す。P18/P19はlicense衝突・途中保存・移転とHTTP/redirect/size/offlineを保持。P21のUnpack/unpackBundleは別decoder経路のため、tar/zipと親子順序を両入口で守る。P28は同じ不成立側の1点だけ削り、`<`と`<=`を分ける2点は保持。**P31は同一行内の公開JSON順序に影響するため当該順序assertを保持して完了判断する**。この小さな契約を変える目的ではない。

### M3: flow / assignment / workflow

所有は `src/internal/flow`、`src/internal/assignment`、`src/internal/workflow` の監査対象testと専用fixture。F05はM2で完了済み。

順序は、(1)F38の通常directory化と別rootの実bytes一致/不一致を確保、(2)前段拒否に隠れた表を正しいStepID/draft/正常Unitに基づく最小表へ統合、(3)初期化・schema・Sensor・履歴表を入口代表＋validator正本へ集約、(4)reassign/work-log失敗の固有assertを生存先へ移す、(5)容量fixtureと軽い境界表を整理。

決定: F10/F11/F12は現在無効な表を温存せず、有効な正常対照から必要な省略・path/hash・pending結合を単一変更で検査する。F13は完全Unitから依存だけ変えた正常/未知依存/循環の小表を残す。F16は複合名1件を実VerificationPathsに入れて提出照合する。F20は同じroot・同じsession拒否を `TestVerificationGates/none` へ移す。F34はbefore log/改変、after log/削除と各失敗点の正しい復旧を保持する。F40は重複する空titleだけ削り、別fieldのnil guardは小さく固有bugを検出するため残す。F41は**正しい容量直前RegistryからReserve/Release/Postを少数回行う通常testを先に作り、旧大量admission反復をstress tagの明示実行へ移す**。上限計算式をtestへコピーせず、許可された予約が必ず解放/確定できる境界を両側で検査する。独立した有効fixtureを安全に作れない場合は旧2testを通常suiteに保持し、頻度変更は行わない。

### M4: app / CLI / workspace / OKF

所有は `src/internal/app`、`cli`、`workspace`、`pathnorm`、`buildinfo`、`okf`、`okfcli`、`okfapp`、`okfmemory`、`projectroot` の各監査対象。A03の未使用 `readDefaultOrganization` はproduction callerなしを再確認して削除する。

順序は、(1)廃止入口/旧reader/弱い存在確認を撤去、(2)app→okfapp field転送を短い1本で残してCRUDを所有packageへ、(3)hook共通拒否とfamily/status/childの固有接続を分離、(4)root/flags/help/stdout/stderrの表を共有実装とroute代表へ、(5)Unicode固定oracleとlock/cursor/OS境界を維持して内部呼出列を除く。

決定: A19はdispatch・必須flags・実JSON例・repair・安全な操作順を保持し、用語辞書だけ除く。A20は実help例の使用testを残し、同じschema再解釈は除く。A26は固定Unicode15契約を保持し、現production rangeを写す表を範囲端/外と独立出力へ置換できた部分だけ削る。A28の非OS custom fs重複entryは削るがdefaultとの実重複は残す。A29は全呼出列を消してもpublish前close/permission復元、own staging cleanup、error cause、古いbytes不変を残す。A31はdev defaultの小さいstruct比較1つへ縮小。A34の再Scan所有権は削り、Search結果の現実的alias防止は維持する。

### M5: CLI一周・診断入口

所有は `src/cmd/aidlc` の監査対象test/helper、`src/cmd/okf` へのmemory integration移動、`src/cmd/natural-japanese-go/integration_test.go`、`src/cmd/aidlc-dist/release_integration_test.go` のstdin接続、`src/internal/projectroot/root_test.go` のC09移動先、現在の実行手順と削除/移動名だけのworkflow修正。

順序は、(1)current journey・現行live・旧hostのhelper依存を分け、(2)aidlc/okfの同じsource/flagsのbuildをtest実行単位で1回共有、(3)Git準備・一周内の反復を削りprocess CAS/保存回復/relocated hook canaryを保持、(4)旧flow live/selector/専用evidenceを撤去、(5)現行診断の正例/identity誤り/未完了を少数の実観測記録再生へ、(6)自然CLI stdinを実candidate Nativeへ移す。

決定: C07は3adapter各1つのerror forwardingを短い表に残し、独立した関数の握り潰しを無保証にしない。C10の閉pipeは代表を保持。C13は旧advance/4stage host専用の型とhelperだけ削り、他の現行liveが使うshell lexer/model/transportは残す。C17は常時inconclusiveやSession欠落で前段拒否の行を除く。C23の旧hook自作正例は除き、診断を残す箇所は実配布生成hookのwire保全代表へ統合する。C29は現schemaのGit共有であり旧互換不要を削除理由にしない。runtime非共有・無管理run拒否・不正JSONの無変更拒否を生存先へ明示してからclone/merge演習を除く。C30の移転先hook実実行は短く保持。C31の旧selector/liveを退役し、現在も必要なOKF exerciseは現行Memoryへ寄せる。

C14–C25の生存診断helperは、追加package/frameworkを作らず同package内の明示 `diagnostic` tagへ整理する。integration依存のものは両tagを指定する入口にし、通常の製品suiteを膨らませない。共有helperは必要な通常/integration fileへ残す。観測記録は既存のsanitizedな実記録を使い、未観測のsyntheticを実記録とは表示しない。適切な実記録がなければ最小正例/不完全/identity拒否を合成と明記して残し、実機成功を新たに主張しない。

**C35のBoundary/Procedure/HumanApproval/Memory/StageSkillsはdistinctな実機境界として保持し、巨大なモデル一周へ無理に統合しない**。AssignmentLiveのinconclusiveはmanual診断と明記する。通常trust等の既存経路を変えず、新しい実機実行や認証を削減作業の前提にしない。C36は代表一周だけ加算RED/GREENを残し、Unit成果変更後と最終stageの新しいSHA・結果登録を省略しない。

P26は配布Nativeにstdin JSON＋empty PATH保証を移し、必ず同候補がPR/finalで実行される状態にしてから旧standalone buildを削る。削除した `TestCommandBinary` をCIから同PRで外し、ゼロ件成功にしない。配布envなしでskipした結果を移動先の成功証拠に使わない。

### M6: CI所有者の集約

所有は `.github/workflows/ci.yml`、`.github/workflows/distribution.yml`、現行検証手順。新規toolやworkflow frameworkは作らない。

- W01: 5CLI×6targetはDistribution固定Go1.26.4が所有。通常Go1.26.x/stableの互換testsは残し、CI側の重複cross-buildを削る。
- W02/W05: 通常suiteはGo1.26.xとstableの両方を保持。workspace/okfはproductionのtag切替がないため、各版でこの2packageだけintegration付き実行し、それ以外の通常package一覧を `go list` から作る案で二重実行を避ける。race/format/module/default smokeは主要1版、journeyは1版。動的に両Go版が同じと推測して片方を消さない。
- W03: 注入版の別build/help重複は実candidate Nativeへ集約。dev/unknown表示の確認は1回残す。
- W04: 全gateをPRとmain pushで実行し、作業branch push＋PRの重複を除く。手動Distributionと公開直前の境界は維持する。
- W06: test入口存在確認、artifact ID一致、candidate/remote tagのdraft直前再照合は保持して完了判断する。3OS・PS5.1/7も残す。

CI変更の完了はYAMLの見た目で判断せず、新headのPR上で通常2Go版、主要版race、必要なfilesystem/journey、6target Package、同候補3OS Native/bootstrapが実際に成功したことで判定する。pending/skip/未開始を成功扱いしない。branch protection/rulesetを追加しない代わりに親がこのgateを管理する。

## review・final・merge・rollback

各PRのwriter返却後、親がtargeted群と全差分を一度確認する。独立reviewは固定base/headの差分と削除→生存先の対応を確認し、全検証を代行しない。特に前段の別エラーで通っている移動test、生成側をそのままoracleにするassert、mutable fixture共有、tagで製品境界が消える問題を確認する。

blocking finding解消後、親がread-only finalを開始する。M1の共通コマンドは次のとおり。全package検査をloop/reviewへ持ち込まない。

```sh
go test -count=1 -shuffle=on -coverprofile=/tmp/ai-dd-test-reduction-m1-coverage.out ./...
go test -count=1 -race -shuffle=on ./...
go vet ./...
go mod tidy -diff
gofmt -l src
git diff --check
go test -tags=integration -count=1 ./src/internal/workspace ./src/internal/okf
go test -tags=integration -count=1 ./src/cmd/aidlc -run '^Test(FlowJourney|GitIndependentJourney)$'
```

M1では通常/integrationの暫定重複は維持し、M6で所有者を整理する。`gofmt -l`は空であることを要求し、適用しない。M2以降は削除/移動後の生存入口へ詳細計画を更新する。M3のstress、M5のdiagnosticの生存入口は当該PR finalで明示的に1回確認し、手動liveのskipを実機成功には含めない。

配布を触るM1/M2/M5/M6では、現headのDistribution CIで同じ5CLI×6target候補を一度生成する。CIのPackageがMetadata、3OSのNative jobが実バイナリとbootstrapを検証するため、同じ30buildをローカルで重複生成しない。必要な診断をローカルで行う場合は、`AIDLC_DIST_DIR`、`AIDLC_RELEASE_VERSION`、`AIDLC_RELEASE_COMMIT`、`AIDLC_RELEASE_GO_VERSION`を実候補と一致させ、Metadataを1回、local Nativeを1回、bootstrap候補を1回検証する。実行入口は以下であり、候補未設定のskipは禁止する。

```sh
go test -tags=integration -count=1 -v ./src/cmd/aidlc-dist -run '^TestReleaseCandidateMetadata$'
go test -tags=integration -count=1 -v ./src/cmd/aidlc-dist -run '^TestReleaseCandidateNative$'
go test -tags=integration -count=1 -v ./src/bootstrap -run '^TestBootstrapCandidateNative$'
```

OS差は同headのGitHub Distributionで3OS・両PS engineを確認する。final後に対象変更があれば証拠をstaleとし、必要なloop/review後にfinalを更新する。全checks成功後、親がIssueに紐づく日本語PRを既存方式でmergeし、default branch反映とIssue closeを確認。次の区切りはmerge済みmainから開始する。

rollbackは問題のあるPRだけをrevertする別PRで行い、他者の変更や後続PRをresetしない。通常の削除/統合で保証の所在が不明になった場合は、同work unit内で元の固有assertを保持したまま親へ理由を返す。既存公開物・tagは一切操作しない。

計画担当はrepoの編集・test実行・GitHub操作を行わず、親が承認記録とともにこの計画を保存した。監査候補全文、承認RAM、作業規則・skill、M1の実Test/helperと現在のworkflow/手順を読み取りで照合した。後続M2–M6のexact targeted群は直前PRをmergeしたheadで親が詳細化して各work unitへ固定する。


## 実施状況

| 区切り | 状態 | Issue / PR |
|---|---|---|
| M1 | 完了・main反映済み。[結果](test-reduction-m1-result.md) | [#204](https://github.com/sori883/ai-dd/issues/204) / [PR #205](https://github.com/sori883/ai-dd/pull/205) |
| M2 | 完了（[結果](test-reduction-m2-result.md)） | [#206](https://github.com/sori883/ai-dd/issues/206) / [#207](https://github.com/sori883/ai-dd/pull/207) |
| M3 | [具体計画](test-reduction-m3-plan.md)、実装中 | [#208](https://github.com/sori883/ai-dd/issues/208) / PR未作成 |
| M4 | 未着手 | — |
| M5 | 未着手 | — |
| M6 | 未着手 | — |

候補ごとの削除・統合・維持の結果は、各区切りの結果記録に追記する。維持は独自の不具合検出と負担を説明できるものに限る。
