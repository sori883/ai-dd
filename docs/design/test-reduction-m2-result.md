# M2: 配布・資材・自然日本語テストの削減結果

[Issue #206](https://github.com/sori883/ai-dd/issues/206)、[具体計画](test-reduction-m2-plan.md)、[承認RAM](../ram/decisions/2026-09-14-test-reduction-approved.md)に基づく `test-reduction-m2`、`verification_mode=loop` の結果。作業rootは `/Users/const/sori883/ai-dd-release`、開始HEADは `9a3fb08dae11e47b9796686b87b6aa1a9f5fcfaf`。

固定全文SHA・文章の存在反復・旧単品archive経路・繰り返し解凍/圧縮・CLI拒否表を削減した。製品仕様、公開CLI、保存形式、Unpack/BuildData/ValidateBundleArchive本体、外部moduleは変更しない。製品側の差分は現bundleへの直接接続、同じSHA実装の既存release.Hashへの委譲、callerのない旧内部群の削除だけ。

## 候補ごとの処置

| ID | 削除・統合・維持 | 生存保証 | 実行証拠 |
|---|---|---|---|
| P06 | MetadataValidationの手作りnative出力・BuildInfo直積と重複schema/target/mode表を縮小 | 正常・schema不正接続・commit/toolchain不一致・manifest/hash再計算後のsource/license改変拒否。実BuildInfo/5CLI表示はMetadata/Native | S4 |
| P07 | license fixtureを1回生成し5製品正常＋missing/modified/extraの各1代表へ。ExecutableMode自己検証を削除 | 製品別の独立license集合と正本bytes、natural README。実header modeはValidateBundleArchiveと下位mode検査 | S4・S3 |
| P08 | validateCandidateBundleからmanifest/entriesを返し再解凍・製品ごとのheader走査を除去 | 1target1parse。独立0755/0644期待値、source/license bytes。Metadataは6target、Nativeは自target、公開直前は全targetを再照合 | S4 |
| P09 | 旧packageArchives/archiveBytes/productArchiveBytes・manifest writer/schema1専用testと未使用validator群を削除。runをpackageReleaseへ直結 | ReleaseInputValidationでsyntax/target/file種別、ReleasePreflightでlate欠損・既存出力・部分保存。BuildData/DataManifest/DataFile/Unpackを保持 | S3 |
| P10 | sh/PSの正常archiveを同formatで1回圧縮。PS各engineのFileを3caseへ | ScriptBlock8case、Fileのvalid/exit/start failure、project/result/curl/logの分離。Windows実行は同head CI | S6 |
| P11 | 固定全文SHA fixtureを削除。Distributionを独立資材集合、Parityを生成bytesと保存結果へ置換 | 13原典skill・5role権限・原典license/source・追加resource・相対リンク・未展開token、実regular/mode/umask/Paths | S1 |
| P12 | Fresh/FlowInstallAssets/WorkflowDefinitionFresh/DocumentDistribution/CodeKBDistributionの重複配置検査と固定件数を除去 | Parityのworkflow.Load、小文字adr/type、default codekbリンク、integration output。Scaffoldにteam/defaultリンク、Rule selector実接続は維持 | S1 |
| P13 | 計画の文章Contains 13項目を除去し、空のtest fileを削除 | role sandbox/encoded instructionsはDistribution、matcher/生成hookはAssignmentContract/InstallHookCommands、出力pathはParity。実help testsは保持 | S1 |
| P14 | 削除対象内の同じ4096上限assert反復を除去 | 長い絶対binary path展開後のTestInstallBootstrapBinaryPathBudgetを変更せず保持 | S1 |
| P15 | StageSkillsInstall/NaturalJapaneseSkill/UpstreamSkillNames/References系の個別installを除去 | Distributionの13skill/SKILL/LICENSE/source、natural5license/writing、原典repository/調整表示、相対リンク、旧接頭辞拒否 | S1 |
| P16 | StageSkills11path衝突・SplitCLIInstallConflict・OKF existing・二重symlink入口を共通Preflightへ | file/dir/leaf link/parent file、late skillとnested parent linkの拒否、保存なし・外部無書込・利用者bytes保持 | S2 |
| P17 | OKF known/SplitCLIRelocation/RuleSkill successとGitIndependent末尾aliasを除去 | RelocateReferencesで非sibling3binary、custom hook/JSON、不存在旧root、同要求retry。別installer adapterを保持 | S2 |
| P18 | OKF/Rule/composed文章別拒否表をlate assetのedited/missing/symlinkへ集約 | early skill・hooks・concurrent hooksの3保存失敗点、実saved/pending集合、利用者file、retry/再retry。SameReferencesAndLockも保持 | S2 |
| P19 | Candidateをvalid/hash/versionへ。2downloadとfresh licenseを統合しBundleDownloads/Fresh行を除去。Offlineへ欠損を統合 | 3runtime bytes/必要path/license、installer/dist非配置、HTTP/HTTPS redirect/size、collision/partial/relocate/tampered/missing。各case root/mapは分離 | S5 |
| P20 | 予約の2要求目からcandidateFiles生成を除去 | lock中の拒否、Fetch未呼出、先行要求の意図した取得失敗完了 | S5 |
| P21 | installのunsafe tar検査をreleaseへ移し、小さいtar/zip fixtureを共通化 | Unpackはpath/duplicate/link等、unpackBundleはそれらにmode/親子前後衝突を追加。別decoderの仕様と実装を変更しない | S3 |
| P22 | AnalyzerExamples/Fixturesと最後のcallerが消えた2testdataを削除 | Analyzer mapping、Surface/Morph/Reportの意味とCLI Analysisを保持。過去観測の説明は残し、現行testの言及を更新 | S7 |
| P23 | Report末尾のMarshal→Valid自己確認とimportを削除 | Report内容、CLIのJSON parse/内容確認は保持 | S7 |
| P24 | Commandの重複拒否5行とOutputErrorを除去 | Diagnosticsのexit/空stdout/stderr。help/version/rules/stdin成功・file入力保存は維持 | S7 |
| P25 | BaselineExcerptRequiredをmissing1例へ | CLIのexit1/空stdout/excerpt診断と転送、下位でfield欠損/空値。読込/JSON失敗は別検査に維持 | S7 |
| P27 | Lexicalの文字数/語数gateとMorphNominalを少数tokenへ | 3999/4000・29/30語・4/5文・名詞終端の有無、Morph1999/2000の既存小fixture。実Lexical Analyze接続1例とMorph意味例は維持 | S7 |
| P28 | Rhythmの同じ5/6文を各1回解析し両categoryを確認。TTR/Burstiness above行を除去 | at/belowによる不等号境界、MTLD/Mora等の独立literal期待値 | S7 |
| P29 | 完成path重複の2caseをstages/discovery.md1caseへ | 同じ出力先になる.md/.md.tmpl衝突の拒否 | S8 |
| P30 | path分類直積をdestination12例、source/generatedは../escape各1例へ | 3入口のvalidator接続、RootTree、親子前後・missing source・generated衝突等は維持 | S8 |
| P31 | TestSurfaceCatalogOrderは保持して完了 | 公開JSONの同じ行内の順序を守る小さい1assert。契約を弱める削減利得がない | S7 |
| F05 | ProcedureBoundaryComposedWorkflowの固定SHA比較を削除 | Distribution/Parityに配置保証を集約し、SharedOperationPropagationの原稿変更伝播を保持 | S1 |

## slice別の検証

各sliceは残存検査を変更前に実行し、移動した固有保証も削除前に確認した。S3の新入口は既存validateInputs/packageRelease/Unpackに直接接続してから初回成功を確認した。いずれも `ALREADY_GREEN` であり、製品を壊して人工的REDは作っていない。正常な移動確認と削減後の同コマンドはexit 0。S6のPowerShellは入口discoveryのみで、Windows実行成功には数えない。

S1移動時、macOSのcase-insensitive FSで `Lstat("ADR")` が既存 `adr` を解決する誤期待が失敗した。親が既存と同じReadDir entry.Name照合への修復を許可し、成功を確認して継続した。この失敗はREDに数えない。S5開始時は削除済みunsafe検査の未使用archive/tar importを除去し、変更前コマンドを再実行した。これもREDではない。

### S1 資材正本と配置の分担（P11–P15/F05）

```sh
go test -count=1 ./src/harness/codex -run '^Test(Distribution|ContentSharedAgentContractAndTOML|ContentRoleDescriptionComesFromCommonSource|SplitCLIDistribution)$'
go test -count=1 ./src/internal/install -run '^Test(CodexManifestParity|DocumentDistributionRuleSelector|InstallBootstrapBinaryPathBudget|AssignmentContract|InstallHookCommands)$'
go test -tags=integration -count=1 ./src/internal/workspace -run '^TestCreateSpaceScaffold$'
go test -count=1 ./src/core ./src/internal/flow -run '^Test(ContentSharedChangeReachesEveryConsumer|ProcedureBoundarySharedOperationPropagation)$'
```

### S2 配置衝突と移転（P16–P18）

```sh
go test -count=1 ./src/internal/install -run '^Test(CodexManifestPreflight|RelocateReferences|RelocateRejectsBeforeSaving|RelocateRejectsSymlinkAndEditedSkill|RelocateSameReferencesAndLock|RelocatePartialAndConcurrentRetry|InstallerCommandRelocation)$'
```

### S3 現bundleへの拒否移動と旧code撤去（P09/P21）

```sh
go test -count=1 ./src/cmd/aidlc-dist -run '^Test(ReleaseInputValidation|ReleasePreflight|BundledRelease|ReleaseLicenseInputs|FiveProductManifestRequiresSixTargets|DistCommand)$'
go test -count=1 ./src/internal/release -run '^Test(UnpackRejectsUnsafe|BundleArchiveRejectsUnsafe|BundleArchiveRejectsIncomplete|BundleManifest|BundleSums)$'
go test -count=1 ./src/internal/release -run '^TestBundleArchiveComplete$/(valid|missing|unknown|hash|mode)$'
```

### S4 候補検証の再解凍・真偽表を縮小（P06–P08）

```sh
go test -count=1 ./src/cmd/aidlc-dist -run '^Test(CandidateLicense|BundledRelease)$'
go test -tags=integration -count=1 ./src/cmd/aidlc-dist -run '^TestReleaseCandidateMetadataValidation$'
go test -tags=integration -list '^TestReleaseCandidate(Metadata|MetadataValidation|Native)$' ./src/cmd/aidlc-dist
```

### S5 Release導入のfixture・表を集約（P19/P20）

```sh
go test -count=1 ./src/internal/install -run '^Test(ReleaseAssetValidationCandidate|ReleaseAssetValidationOffline|InstallReservation|InstallFailurePartial|ReleaseLicenseRetention|ReleaseDownload|ReleaseDownloadHTTP|ReleaseDownloadRedirectMustRemainHTTPS|InstallerCommandRelocation)$'
```

### S6 bootstrapの同値直積と圧縮準備（P10）

```sh
go test -count=1 ./src/bootstrap -run '^TestBootstrap$'
go test -list '^TestBootstrapPowerShell$' ./src/bootstrap
```

### S7 自然日本語の重複・重い入力（P22–P25/P27/P28/P31）

```sh
go test -count=1 ./src/internal/naturaljapanese -run '^Test(Analyzer|AnalyzerEmpty|Surface|SurfaceSeverity|SurfaceCatalogOrder|Report|Baseline|BaselineInvalid|BaselineExcerptRequired|Text|Lexical|LexicalMTLD|LexicalSpecificity|LexicalTTRBoundary|Morph|MorphNominal|MorphCharacterBoundary|Rhythm|RhythmMora|RhythmBurstinessBoundary)$'
go test -count=1 ./src/cmd/natural-japanese-go -run '^Test(Command|CommandFile|CommandAnalysis|CommandBaselineError|CommandBaselineExcerptRequired|CommandDiagnostics)$'
```

### S8 独自path判定の直積（P29/P30）

```sh
go test -count=1 ./src/harness -run '^TestManifest(Render|RootTree|RejectsInvalid)$'
go test -count=1 ./src/core/workflow -run '^TestRenderRejectsDuplicateCompletedPaths$'
```

`TestContentDistribution` はTestDistributionへ統合して削除し、計画のexact commandからも除いた。Metadata/MetadataValidation/Nativeの3入口、PowerShell入口はdiscoveryで確認し、候補環境変数の未設定skipを実行成功として数えていない。

## 残る確認

独立review、親のread-only final、同head Distribution CIの6target Metadata・3OS Native・bootstrap・PS5.1/7、PRマージは親が行う。loopで全package/race/vet/cross-build/配布E2E/liveを実行していない。過去audit/RAMの観測と旧計画の実行証拠は書き換えていない。

## work unit末尾の検証

全8sliceの変更後、上記18コマンド（実行16、入口discovery 2）を一度まとめて再実行し、全てexit 0、実行対象なし・失敗はなかった。詳細は `/tmp/test-reduction-m2-writer-boundary.json` に保存した。変更Goファイルへgofmtを適用し、`git diff --check` と現行開発手順・CIの削除名参照確認も成功した。開始・終了HEADは `9a3fb08dae11e47b9796686b87b6aa1a9f5fcfaf`。
