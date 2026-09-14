# M1: 別名入口とhelper自己検証の削減結果

[Issue #204](https://github.com/sori883/ai-dd/issues/204)、[承認](../ram/decisions/2026-09-14-test-reduction-approved.md)、[具体計画](test-reduction-milestones.md)に基づく `test-reduction-m1` の実装結果。開始HEADは `65458f6b70d8f029d52ca220bafca04f45913c4a`。製品挙動を維持し、24個のTest入口と最後のcallerを失った専用helper/child分岐を除去した。構文拒否6行では読まれない配布入力生成を省いた。製品Go、module、公開API、保存形式は変更していない。

## 候補ごとの処置と残る保証

下表のS1–S5は実装順と後述の実行証拠を表す。既存の正しい検査の変更前成功を `ALREADY_GREEN` とし、人工的REDは作っていない。

| 候補 | 実施した削減 | 生存保証と実行証拠 |
|---|---|---|
| P01 | `TestDistributionArchives`、`TestDistributionJourney`、`TestNaturalJapaneseDistributionArchives`、`TestNaturalJapaneseDistributionJourney` を削除。Package regexをMetadata/MetadataValidationへ更新 | `TestReleaseCandidateMetadata` / `TestReleaseCandidateNative` と共有command/write/snapshot/custom hook helperを保持。S1で3入口をdiscovery。実候補の検査は同head Distribution CIで行う |
| P02 | `TestVersionedAssetsLicense` を削除 | `TestBundledRelease` と候補の正本license照合、`TestReleaseLicenseInputs` を保持。S1でbundleと入力拒否を前後実行 |
| P03 | `TestProductCLI` を削除。`TestDistCommand` のmissing args、unknown flag、positional、empty/duplicate/unknown targetの6行のみ、fixtureを未存在の入出力/license pathと有効な版/commit/Go版へ変更 | `TestDistCommand` のexit 2、空stdout、error、出力なしを保持。成功とoperational failureの `releaseFixture` は維持。S1で前後実行 |
| P04 | `TestDistributionBinaryPath`、`TestReleaseCandidateNativeSelection`、`TestReleaseCandidateProjectDirectory` と専用 `fixtureBinaryPath` を削除 | `selectBundleNative` / `releaseProjectDirectory` はNativeで実利用するため保持。S1でintegration compile/discovery。helperの局所誤字は実候補Nativeで確認する |
| P05 | `TestBootstrapOutputStreams` / `TestBootstrapProjectDirectory` と `BOOTSTRAP_STREAM_HELPER` 分岐を削除 | `TestBootstrap` をS2で前後実行。`bootstrapCommandOutput` / `sameProjectDirectory` は実candidate/PowerShellで使用し保持。両PS engineと3OS candidateは同head CIで確認 |
| A01 | appの `TestRuleSkillSeparationHookApprovalPending` / `TestHookSplitCLIRepair`、workspaceの `TestRuleSkillSeparationRuleCopy`、okfcli/okfmemoryのalias `TestOKFMemoryContract` 計5個を削除。空になるworkspace fileを削除 | 承認待ち、hook修復、文書修復、Rule配置、OKF command、metadata保存とIntent検索の元Testを変更せずS3で前後実行。okfappの同名実検査は保持 |
| F01 | `TestEndSensorUnitCommandPair` / `TestFlowUnitDependencyContent` を削除 | `TestVerificationResults` / `TestUnitWithoutGit` のUnit結果対応とGit不要の予約→提出→統合→解除をS4で前後実行 |
| C01 | `TestBoundaryJourney` / `TestProcedureJourney` / `TestIntentDocumentsJourney` と空になるdocuments journey fileを削除 | 同じ `runBoundaryJourney` を呼ぶ `TestFlowJourney` を保持。S5でFlow/GitIndependent入口をdiscovery。現行手順3箇所をFlowへ更新。一周実行は親final |
| C03 | `TestObserverBinaryArgs` と未使用importを削除 | `observerHookCommand` と現在のobserver callers、`TestMainHookCommand`、Codex split asset検査を保持。S5でMainHookCommandを前後実行。wrapperの局所引数比較を除去した |
| C04 | `TestHookProbeCanonicalRoot` / `TestHookProbeTrustConfig` を削除 | 両helperと現live入口を保持。S5でObservedTransport/Replayを前後実行。これはroot/trustの実機成功を意味しない。標準path処理と設定文字列の局所自己検証を除去した |

## loopの実行証拠

各コマンドは作業root `/Users/const/sori883/ai-dd-release` で、対応sliceの変更前・変更直後・作業単位末尾に実行した。実testはすべて変更前 `ALREADY_GREEN`、変更後 `GREEN`、末尾成功（終了code 0）。`-list` は入口存在とcompileの確認だけであり、skipや実候補の実行成功として数えない。

### S1

```sh
go test -count=1 ./src/cmd/aidlc-dist -run '^Test(DistCommand|BundledRelease|ReleaseLicenseInputs|FiveProductManifestRequiresSixTargets)$'
go test -tags=integration -count=1 ./src/cmd/aidlc-dist -run '^TestReleaseCandidateMetadataValidation$'
go test -tags=integration -list '^TestReleaseCandidate(Metadata|MetadataValidation|Native)$' ./src/cmd/aidlc-dist
```

### S2

```sh
go test -count=1 ./src/bootstrap -run '^TestBootstrap$'
go test -list '^TestBootstrap(PowerShell)?$' ./src/bootstrap
```

### S3

```sh
go test -count=1 ./src/internal/app -run '^Test(ExecutionPlanCLIPendingHook|BoundaryHookRepairAndBegin|IntentDocumentsHookRepair|IntentDocumentsUnregisteredInputRepair|ProcedureDriftBlocksDocumentRepair|OKFSkillReadApprovalPending)$'
go test -count=1 ./src/internal/workspace ./src/internal/okfcli ./src/internal/okfmemory -run '^Test(CreateSpaceOKF|CreateSpaceOKFFallbackAndInvalid|OKFCommand|MetadataInputBuildAndPreserve|SearchIntentID)$'
```

### S4

```sh
go test -count=1 ./src/internal/flow -run '^Test(VerificationResults|UnitWithoutGit)$'
```

### S5

```sh
go test -count=1 ./src/cmd/aidlc -run '^Test(MainHookCommand|HookProbeObservedTransport|HookProbeReplay)$'
go test -tags=integration -list '^Test(FlowJourney|GitIndependentJourney)$' ./src/cmd/aidlc
```

S1のdiscoveryはMetadata、MetadataValidation、Nativeの3名、S2はBootstrapとPowerShellの2名、S5はFlowJourneyとGitIndependentJourneyの2名を前後とも確認した。変更Go fileへgofmtを適用し、末尾に全10コマンドを再実行した。`git diff --check` も成功した。

通常/tag付きのソース、workflow、現行development手順を検索し、削除入口の実行指示が残らないことを確認した。okfappの `TestOKFMemoryContract` はaliasではないため維持する。固定HEADの監査・過去RAM・旧計画にある名前は歴史的証拠として保持した。

## 残る確認

独立レビュー、親のread-only final（全package、race、vet、format、必要なintegration）、同head GitHub checks、同一配布候補のMetadata・3OS Native・bootstrap・PS 5.1/7、PRマージは親が確認する。loopでは全体検証・配布E2E・liveを実行していない。helper自己検証の除去により局所的な誤字の発見は実利用先へ移るが、製品保証のある入口と共有helperは維持している。
