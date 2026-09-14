# M2: 配布・配置資材・自然日本語テストの削減

対応Issue: [#206](https://github.com/sori883/ai-dd/issues/206)

## 目的・許可・開始条件

作業rootは `/Users/const/sori883/ai-dd-release`。計画時に読んだHEADは `65458f6b70d8f029d52ca220bafca04f45913c4a`。M1 writerが並行してM1の所有fileを変更中なので、本計画では編集・test・GitHub操作をしていない。M1はPR #205としてmainへマージ済み。M2の基準HEADは `a782bfbcb9abfa0352349911523e4baebb34a6ec`、計画保存後の開始HEADをwriter handoffで固定する。

ユーザーの「できるだけ削除」「区切って対応」「Issue作ってmrでマージ」という直接承認、`docs/ram/decisions/2026-09-14-test-reduction-approved.md`、`docs/design/test-reduction-milestones.md`が許可の根拠。M2は P06–P25/P27–P31/F05 の26候補を、同じ実害を検出する少数の検査へ集約する区切りである。P26の実binary build統合とCI全般の整理は後続区切りが所有する。

結果は、固定された全文章SHA・重複した配置・旧単品archive・多重解凍・同じ文章のtokenizeを減らしつつ、必要資材、5CLI、役割/権限、原典license、unsafe input、同じ版の導入/移転、途中保存の保証を保つこと。公開CLI/API、保存形式、7配布asset、通常のGo/PowerShell/sh処理の挙動は変えない。外部module/tool、新しい一般検証framework、Release/tag操作は追加しない。

1 Issue/PR、1 writer、`work_unit_id=test-reduction-m2`、`verification_mode=loop`。8つのsliceを指定順に完了して一度返す。親はwriter稼働中の編集を止める。既存の正しい動作をまとめるため初回成功を `ALREADY_GREEN` と記録し、人工REDを作らない。

## 候補ごとの完了条件

以下のパスは作業root基準。参照は関数名で固定し、M1の削除による行番号変化に依存しない。

| ID | 実施内容 | 生存する保証と移動先 |
|---|---|---|
| P06 | `cmd/aidlc-dist/release_integration_test.go::TestReleaseCandidateMetadataValidation` の手作りnative出力とBuildInfo field総当たりを削る。候補変形のschema/version/target/modeを下位へ集約 | 同関数に正常、1つの構造不正接続、commit不一致、toolchain不一致、manifest/hashを再計算したsource改変・license改変を残す。実30構成のBuildInfoとnative5CLIの完全一致表示はMetadata/Nativeで維持 |
| P07 | `candidate_license_test.go::TestCandidateLicense` はfixture1回・5製品正常＋missing/modified/extra各1代表へ。`TestCandidateLicenseExecutableMode`を削除 | 製品ごとの正確なlicense集合/正本bytes・natural READMEは独立期待値として残す。modeは実archiveを検証する `ValidateBundleArchive` と `TestBundleArchiveComplete/mode`、Metadataの独立mode期待値が所有 |
| P08 | `validateCandidateBundle` から検証済みmanifest/entriesを返し、license/source/BuildInfoのassertへ渡す。`validateCandidateLicenses` の製品ごとのraw再解凍・header再走査を廃止 | 1target1回の `ValidateBundleArchive`。Metadataは6target、Nativeは自targetだけの静的確認＋実行。draft前は新たに全候補を検証。異なる変形caseのmapを共有しない |
| P09 | 旧単品builderとschema1専用test/helperを削る。現一式へ必要な拒否・途中保存を移す | 新 `TestReleaseInputValidation`、`TestReleasePreflight`、既存 `TestBundledRelease` / `TestReleaseLicenseInputs` / `TestDistCommand`。削除するproductionの詳細は後述 |
| P10 | `bootstrap/powershell_test.go::TestBootstrapPowerShell` を各engineのScriptBlock全8mode、`-File`はvalid/exit/start failureの3modeへ。共通exeの圧縮は同formatで1回 | PS5.1/7ともcaller復帰、exit0/17、Start失敗で元診断を保つ。各caseのproject/result/PATH/curl配置は分離。`TestBootstrap`の正常tarも1回生成してreuse |
| P11 | `harness/codex/manifest_test.go::TestDistribution` を独立した資材集合/role/参照/原典/未展開token契約へ変更。`install/manifest_parity_test.go::TestCodexManifestParity` は生成bytesと実配置の照合へ | 資材正本の独立期待値と配置層のbytes/regular/mode/umask/Pathsを分担。`install/testdata/five-cli-assets-sha256.json`を削除。固定全文hashや同等の巨大snapshotへ置換しない |
| P12 | `TestInstallFresh`、`TestFlowInstallAssets`、`TestWorkflowDefinitionFresh`、`TestDocumentDistribution`、`TestCodeKBDistribution`の重複配置/存在/件数を削る。Codex Content/Splitの固定15/5/71件assertも削る | fresh配置はParity、追加Spaceは `workspace::TestCreateSpaceScaffold`。workflow.Load、default側のcodekbリンク・小文字adr/type、integration出力pathはParityへ。Rule selectorの実接続は `TestDocumentDistributionRuleSelector` を保持 |
| P13 | 監査の13項目の文面Containsを削除。専用testが空ならfileも削除 | 5agentのsandbox/encoded instructionsはDistribution、tool matcher/生成hookは `TestAssignmentContract` / `TestInstallHookCommands`、実help解決は既存CLI tests。CodeKBの出力path構造はParityへ。言い回しを新たなhashに置換しない |
| P14 | 同じ `len(bootstrap)>4096` の反復を削除 | `bootstrap_test.go::TestInstallBootstrapBinaryPathBudget` の長い絶対path展開後の実context上限を維持 |
| P15 | `TestStageSkillsInstall` / `TestNaturalJapaneseSkill` / `TestUpstreamSkillNames` / `TestStageSkillsReferencesResolve` / `TestUpstreamSkillReferences` をDistributionの1回の生成物検査へ移す | 13原典skill、SKILL/原典LICENSE/source、natural writingと補助license、相対リンク、旧接頭辞混入拒否を保持。正常assetを毎回installしない |
| P16 | `TestStageSkillsCollision` の11pathをlate skill1例へ。`TestSplitCLIInstallConflict`、`TestOKFSkillInstall/existing`は共通衝突へ。symlinkの2入口を代表へ | `TestCodexManifestPreflight` のfile/dir/leaf link/parent file、late skill衝突、nested parent linkで外部無書込・preflight全file無変更を維持。別3binaryの接続はSplitCLIDistribution |
| P17 | `TestRelocateReferences` へ明示3binary・user hook・同要求retryを統合。`TestOKFSkillRelocate/known`、`TestSplitCLIRelocation`、RuleSkillRelocate/successとGitIndependent末尾aliasを除く | 正常RelocateFrom一周と、別の公開installer adapter `TestInstallerCommandRelocation` を維持。旧rootの不存在を入力文字列として扱える契約も残す |
| P18 | `TestRelocateRejectsSymlinkAndEditedSkill`へedited/missing/symlink代表を集約。OKF/Rule/composed文章別表を除く | `TestRelocatePartialAndConcurrentRetry`、SameReferencesAndLock、early skill保存失敗とhook保存失敗、各結果の保存済み/未保存集合・利用者file不変・同要求retryを維持 |
| P19 | `TestReleaseAssetValidationCandidate`へ2downloadとfresh license保持assertを統合。schema/extra path表、`TestReleaseBundleDownloads`、LicenseRetention/freshを削る。`TestInstallFailure`はoffline表へ | Candidateはvalid/hash不一致/選択version不一致。Offlineは選択targetだけで成功・欠損失敗。HTTP error/HTTPS redirect/size、license collision/partial/relocate/tampered/missingと実partial導入は保持 |
| P20 | `TestInstallReservation` の2要求目から `candidateFiles` を除き、呼ばれたら記録するFetchへ | lock中の拒否、2要求目Fetch呼出なし、先行要求の完了を確認。先行は現testでも意図した取得失敗なので成功installへ変更しない |
| P21 | install側 `TestReleaseAssetValidationUnsafeArchives` を低層releaseへ移し、同じ小さなarchive fixtureを共通化 | UnpackとunpackBundleは別実装。共有のpath/link/duplicate拒否と、bundle専用のmode/親子前後衝突を区別して維持。詳細は後述 |
| P22 | `naturaljapanese/analyzer_test.go::TestAnalyzerExamples`、`fixtures_test.go::TestFixtures` を削除 | Analyzerのmapping、Morph/Surface/Reportの意味、CLI Analysisを保持。2つの未使用testdataだけcaller確認後に削除し、説明文の現在testへの言及を修正 |
| P23 | `rules_test.go::TestReport` 末尾のMarshal→Validだけの段落を削る | Report内容の既存assertとCLI JSON parse/内容を保持。残る別目的がなければMarshal自体とencoding/json importも除く |
| P24 | natural CLI `TestCommand` のmissing/flag/genre/unreadable/invalid UTF8、`TestCommandOutputError`を削る | `TestCommandDiagnostics` の強いexit/空stdout/stderr確認。help/version/rules/stdin成功・file保存/入力保持は残す |
| P25 | `TestCommandBaselineExcerptRequired` のmissing true/falseをmissingの1例へ | CLIのexit1/空stdout/stderrとbaseline入力転送。field有無/空値は `naturaljapanese::TestBaselineExcerptRequired`。読めないfile/不正JSONは別経路として保持 |
| P27 | Lexicalの3999/4000文字とMorphの1999/2000文字を小token fixtureへ。Nominalの500文反復を少数token群へ | 文字数境界、必要な語数、名詞終端の有無を保持。Lexicalの実Analyze接続は意味のある1例だけ残し、Markdown除外はTestText/Reportに集約 |
| P28 | `TestRhythm` の同じ5文/6文を1回ずつ解析して2categoryをassert。LexicalTTRBoundary/RhythmBurstinessBoundaryのabove行を削る | at/belowの2点で不等号境界を保持。MTLD/Mora/独自数値アルゴリズムは削らない |
| P29 | `core/workflow/content_test.go::TestRenderRejectsDuplicateCompletedPaths` はstages/discovery.mdの1ケースへ | `.tmpl`除去後に同じ出力pathとなる実衝突を維持 |
| P30 | `harness/manifest_test.go::TestManifestRejectsInvalid` はdestinationにpath分類12例、source/generatedに各 `../escape` の代表のみ | 各入口のvalidator接続、root treeの特例、親子前後衝突、missing source、unknown token、generated衝突を残す。非公開validatorの公開や新frameworkは不要 |
| P31 | `naturaljapanese/surface_test.go::TestSurfaceCatalogOrder` を保持 | 公開JSONの同一行内の順序を削減目的で変更しない。1assertで小さく、契約を弱める利得がない。理由付き保持で候補完了 |
| F05 | `flow/content_test.go::TestProcedureBoundaryComposedWorkflow` を削除 | Distribution＋実配置Parityが所有。`TestProcedureBoundarySharedOperationPropagation` は独自の原稿伝播なので保持 |

表の `cmd/`、`internal/` 等はすべて `src/` 配下。

## 最小構成の具体設計

### 1. 固定SHAを置換する資材検査

既存 `TestDistribution` の中でDistributionを1回呼び、path→assetのローカルmapを作る。汎用assertライブラリやproductionに公開したtest helperは不要。期待値は次の契約だけをliteralの表として置く。

- 独自SKILL `aidlc` / `aidlc-cli`。原典13skillは architecture/code-review/domain-modeling/grill-with-docs/grilling/natural-japanese-go/okf-agent-memory/planning/research/systematic-debugging/tdd/to-spec/verification-before-completion。各SKILL、LICENSE、references/source.mdを要求する。
- roleは aidlc-requirements/researcher/reviewer/stage-planner/worker の5つ。workerだけworkspace-write、他はread-only。既存方式でdeveloper_instructionsのencoded文字列が解釈可能なことを確認する。quote/backslash/triple delimiterの挿入は既存 `TestContentSharedAgentContractAndTOML` を保持する。
- workflowはstage-graph.jsonと initialization/discovery/planning/architecture-analysis/tdd/integration の6原稿。default knowledgeのindex、adr/codekb/design各index、rules/entry・rule、ADR template、hooksを要求する。
- 原典側の追加resourceは code-review/review.md、planning/handoff.md、tdd/testing.md、natural cli.md/writing.mdと既存5license。存在・license bytesの正本一致を確認する。原典本文の通常説明をhashでは固定しない。
- 元skill名とfrontmatter名、原典URL/作者/翻案表示を1表で確認する。作者の独立期待値は既存承認済み6出典群（mattpocock、owainlewis、mblode、obra、coji、okf-memory）の現記録を使う。原典LICENSE/source.mdのbytesはcore.Filesと比較する。source/作者の正しさそのものを生成コードの返値から期待値へコピーしない。
- Markdown相対リンクは既存のregexpと同じ解釈で生成asset map上の実在先へ解決する。URL/fragmentを除く。旧skill接頭辞の混入、未知 `@@...@@`、未展開include、`.tmpl`や内部shared原稿の直接配布を拒否する。追加するregex parserやMarkdown一般validatorは不要。

この表は必要集合を独立に検査し、配置結果から同じ欠落を持つ期待集合を作らない。全体の71件という単独assertを維持する必要はない。資材種類ごとの名前集合や想定しない未展開templateは確認する。

`TestCodexManifestParity` は実root・明示binaryでCodexFromを実行し、そのroot/binaryで `codex.DistributionFrom` が生成したassetsと保存bytes/Pathsを照合する。旧のroot文字列正規化→hash比較を除く。既存apostrophe入りroot/binary、regular file、caller umaskを観測するcontrol file、戻りPathsと実保存集合を保持。生成側の不足は独立したTestDistribution、書き込み側の欠落/破損はParityが検出する。

freshのworkflow.Load・templateのtype・小文字adr・default codekbリンク・integrationのcurrent_analysis/architecture出力pathを同じParityの不変rootへ集約する。追加Space側のcodekbリンクと二重knowledge directory不在は `workspace/space_create_integration_test.go::TestCreateSpaceScaffold` の既存team/defaultへ移す（このfileをM2所有へ追加）。実Rule selectorがRule本文に一致する `TestDocumentDistributionRuleSelector` は保持する。

P13の削除対象は `install_test.go` の RecoveryGuidanceAndContextLimit / MemoryCommandGuidance / FlowInstallInactiveResumeGuidance / MemoryHelpPlacedSkill / OKFWorkLogInstalledGuidance、`flow_test.go::TestFlowInstallJapaneseProcedure`、`assignment_test.go::TestAssignmentContractDocumentRules`、`codekb_test.go::TestCodeKBGuidance` の文章部、`execution_plan_test.go::TestExecutionPlanDistribution`、`stage_planner_test.go::TestStagePlannerDistribution`、`product_agents_test.go::TestProductAgentAssets` の文章部、`git_independent_test.go::TestGitIndependentInstall`、`rule_skill_separation_test.go::TestRuleSkillSeparationAssets`。残す構造を移して空になった関数/fileを除く。

### 2. 旧productionを削る正確な境界

`cmd/aidlc-dist/main.go::run` は現状options.Productをallにして `packageArchives` を呼び、即 `packageRelease` へ分岐する。runを直接packageReleaseへ接続し、次を除く。

- `archive.go::packageArchives` の全体、`archiveBytes`、`productArchiveBytes`。optionsのProductはvalidateInputsで製品ごとのbinaryを選ぶため残す。`binaryInput`、`validateInputs`、`writeNewFile`、supportedTargets、errInvalidInputは現行使用のため保持。
- `manifest.go` の旧 `artifact` / `manifest` / `writeManifest` / `writeProductManifest`。`checksum`はlicenses.goにも現callerがあるので、ここだけ既存 `release.Hash` 呼出へ置換して同ファイルを削除できる。正本license digest定数を変更しない。
- archive_testの旧 Layout/ProductArchive/ProductInvalid と manifest_testのReproducibility/専用digest。`fixtureTargets` / `mustRead` は残るcallerへ移す。`archiveFixture` のoptions作成を `releaseFixture` へ取り込み、先にaidlcだけ6fileを作りその後再度30fileを書く二重準備をやめる。
- 参照検索では `internal/release::ValidateManifest` / `ValidateBinary` / `ValidateData` にproduction/testのcallerがなく、`VerifySums`はこの旧validatorだけ、`MetadataNames`は旧validatorと旧writerだけ、Artifact/Manifest型も同じ閉じた参照群である。M1後の通常/tag/OS別全src検索でも同じなら、この旧43形式の内部群を `validate.go` / `format.go` から削除する。Goのinternal package内の未使用群で、公開CLI/APIの除去ではない。
- **BuildData、DataManifest/DataFile、Unpackは保持する**。packageReleaseは今も共通原稿をBuildData→Unpackで得る。旧schema1という名前だけで一括削除しない。BuildDataの返却shapeやsource schema、bundle schema2は今回変更しない。

旧 `TestArchiveRejectsInvalidInput` のsafe version/commit/toolchain/targets/file種別は、直接validateInputsを呼ぶ小さな `TestReleaseInputValidation` へ移す。比較的後ろの製品/targetが欠ける場合にpackageReleaseがoutputを作らないこと、既存出力が不変であること、途中write失敗がerrorかつ部分候補になることを新 `TestReleasePreflight` へ移す。

現packageReleaseはファイル名をsortして書くため、SHA256SUMSが先に保存される。旧testの「途中失敗ならSHA256SUMSは存在しない」は現製品の保証ではないため移さない。失敗はerrorになり、残ったfileは部分候補として扱われ、全6archiveがない候補を成功/公開候補と扱わないことを検査する。保存順を変えて旧assertを成立させる修正はしない。既存出力の保全とall6target必須は維持する。

### 3. Candidateを一度解釈する

`validateCandidateBundle` の返値を `(release.BundleManifest, map[string][]byte, error)` とし、現 `release.ValidateBundleArchive` を1回だけ呼ぶ。ここから各license/source/body期待値とBuildInfoへentriesを渡す。license helperからraw/archive decoderを外し、bundle専用の製品prefix選択にする。旧単品形式の自動判定も不要になる。

実headerの種別・modeとmanifestの一致はValidateBundleArchiveが既に検査する。checker側は返されたmanifestのmodeに対して5binary=0755、その他=0644という独立期待値を保持する。実headerの誤modeを拒否する小さい下位regressionも残す。これによりmode APIを新たに公開せず、多数の再解凍を除ける。

Metadataは全6targetでこのhelperを呼び、戻ったentriesで5製品の実BuildInfoを確認する。NativeはParseBundleSumsで一覧・自targetを選び、同helperで自targetだけ確認して実5binary起動・version/help・導入・relocateへ進む。foreign targetをNative各jobで再検査しない。Packageが成功したartifact IDをNativeへ渡す既存workflow依存は保持。draft jobのverifyReleaseCandidateは全6targetを新たに検査し、公開直前の別境界を維持する。

`TestCandidateLicense` は1回のpackageReleaseからlinuxの検証済みentriesを得て、5製品の正例を1回ずつ確認。missingはokf/yaml-NOTICE、modifiedはnatural/UniDic-NOTICE、extraはaidlc/extra.txt等各代表1件。caseごとにmapをcloneし、値を差し替える。Source/Licenseを書き換えてmanifest/hashまで再計算した上位負例はMetadataValidationに必ず残す。mutated bytesに元のparse結果を使わない。

### 4. FS境界を統合するときの制限

RelocateReferencesは明示的な非siblingの3binaryをCodexFrom→RelocateFromへ渡し、各役割の参照、利用者hook/独自JSON field、同要求の二度目が無変更であることを1往復で検査する。legacy wrapperのRelocate自身はSameReferencesAndLock/不正系とInstallerCommandRelocationの接続へ残る。

Relocateのinvalid表はedited/missing/symlinkを同じlate managed assetへ寄せる。composed文章の日本語を特定語へ置き換えるfixtureは、任意bytes改変で十分。RuleSkillRelocateのpartialは早いskill保存点、既存PartialAndConcurrentRetryは最後のhooks保存点なので、同じものとして消さず、1つの表へearly skill/hooks/concurrent hooksの3caseとして集約する。固定件数5/1等ではなく実際に変わった集合、結果Paths/Pending、利用者file保全と再試行後完了を比較する。

install側UnsafeArchivesのescape/duplicate/symlinkを `internal/release/bundle_archive_test.go` の小fixtureへ移す。既存手作りtar/zip生成部分をtest専用関数に抽出する程度とし、validator frameworkは作らない。新 `TestUnpackRejectsUnsafe` はvalid対照と共有のpath/duplicate/link等だけをUnpackへ通す。`unpackBundle` には現行のmode制約とparent collision/reverse collisionも残す。**Unpackは0777や親子名の併存を現在拒否しないため、両decoderへ同じ全表の拒否を要求しない**。Unpackはin-memory source読込として別の役割を持ち、これを変更する安全性仕様変更はM2に含めない。

Release導入のcandidateFilesは同一testの中で1回だけ作る。caseごとにmapをcloneし、改変対象archiveを作り直す。LicenseRetentionの5異常/移転caseも基礎候補は不変共有できるが、root/配置file/options/writeは各caseで分離する。固定120件だけのassertは、3runtimeの正しいbytes/paths、3製品のlicense正本bytesと必要資材、installer/distがproject binへ置かれないことへ置き換える。公開先集合を製品の返値だけから期待値へコピーしない。

## 所有file

単独writerは次だけを編集する。新しい共通production検証packageは作らない。

- `src/cmd/aidlc-dist/{archive.go,manifest.go,main.go,licenses.go,archive_test.go,manifest_test.go,release_test.go,bundle_test.go,candidate_license_test.go,release_integration_test.go}`。main_test.goはM1の最小optionsと関数名接続が必要な場合のみ。test helperを整理する小さなfixture_test.go追加は同package内で可。
- `src/internal/release/{format.go,validate.go,bundle_archive_test.go}`。生存productionのUnpack/BuildData/ValidateBundleArchiveのロジックは変えない。
- `src/harness/{manifest_test.go}`、`src/harness/codex/{manifest_test.go,content_test.go,split_test.go}`、`src/core/workflow/content_test.go`、`src/internal/flow/content_test.go`。
- `src/internal/install/{manifest_parity_test.go,install_test.go,flow_test.go,workflow_test.go,documents_test.go,codekb_test.go,assignment_test.go,execution_plan_test.go,stage_planner_test.go,product_agents_test.go,git_independent_test.go,rule_skill_separation_test.go,stage_skills_test.go,split_test.go,relocate_test.go,release_test.go,testdata/five-cli-assets-sha256.json}`。bootstrap_test.goの上限testは原則そのまま保持。
- `src/internal/workspace/space_create_integration_test.go` の既存Scaffoldへのcodekb/assert移動のみ。root/lock等のM4所有の整理はしない。
- `src/bootstrap/{bootstrap_test.go,powershell_test.go,fixture_test.go}` のtest準備だけ。install.sh/install.ps1本体は変更しない。
- `src/internal/naturaljapanese/{analyzer_test.go,fixtures_test.go,rules_test.go,lexical_test.go,morph_test.go,rhythm_test.go}`、未使用なら `testdata/{ai-smelly.md,natural.md}`。surface_test.goは理由付き保持。`src/cmd/natural-japanese-go/main_test.go`。
- 現行手順の `docs/development.md`、`docs/distribution.md`、`src/docs/natural-japanese-go.md`、M2計画・結果RAM・索引。workflowは生存test名を維持するため原則変更不要。必要な参照名だけの修正は同PR、gate構成変更はM6へ残す。

## 順序付きsliceとtargeted commands

全commandは作業root。正常対照と生存保証を先に揃えて実行し、ALREADY_GREENを確認してから旧test/旧コードを除く。下記の新test名はM2で作る具体的な生存入口で、人工REDのための追加ではない。各slice末尾は同commandを再実行する。実candidate build・cross build・広域testはloopで行わない。

### S1 資材正本と配置の分担（P11–P15/F05）

まずTestDistribution/Parityへ独立期待値・固有assertを移し、次に固定SHAと文章/存在反復を削除する。追加Space側のassertも移してからCodeKBDistributionを除く。

```sh
go test -count=1 ./src/harness/codex -run '^Test(Distribution|ContentDistribution|ContentSharedAgentContractAndTOML|ContentRoleDescriptionComesFromCommonSource|SplitCLIDistribution)$'
go test -count=1 ./src/internal/install -run '^Test(CodexManifestParity|DocumentDistributionRuleSelector|InstallBootstrapBinaryPathBudget|AssignmentContract|InstallHookCommands)$'
go test -tags=integration -count=1 ./src/internal/workspace -run '^TestCreateSpaceScaffold$'
go test -count=1 ./src/core ./src/internal/flow -run '^Test(ContentSharedChangeReachesEveryConsumer|ProcedureBoundarySharedOperationPropagation)$'
```

`TestContentDistribution` がTestDistributionへ全統合されて空になる場合は削除し、先頭commandから同名だけ除く。他の共有原稿変更伝播2testは維持。

### S2 配置衝突と移転（P16–P18）

late collision/nested linkをPreflightへ、3binary/user hook/retryをReferencesへ、early/late/concurrent partialをPartialAndConcurrentRetryへ移してから旧表を削る。

```sh
go test -count=1 ./src/internal/install -run '^Test(CodexManifestPreflight|RelocateReferences|RelocateRejectsBeforeSaving|RelocateRejectsSymlinkAndEditedSkill|RelocateSameReferencesAndLock|RelocatePartialAndConcurrentRetry|InstallerCommandRelocation)$'
```

### S3 現bundleへの拒否移動と旧code撤去（P09/P21）

ReleaseInputValidation/ReleasePreflight、Unpackの共有契約を先に確認する。既存partialのSHA未作成assertを持ち込まない。mainのpackageRelease直接接続と旧参照群を削除する。

```sh
go test -count=1 ./src/cmd/aidlc-dist -run '^Test(ReleaseInputValidation|ReleasePreflight|BundledRelease|ReleaseLicenseInputs|FiveProductManifestRequiresSixTargets|DistCommand)$'
go test -count=1 ./src/internal/release -run '^Test(UnpackRejectsUnsafe|BundleArchiveRejectsUnsafe|BundleArchiveRejectsIncomplete|BundleManifest|BundleSums)$'
go test -count=1 ./src/internal/release -run '^TestBundleArchiveComplete$/(valid|missing|unknown|hash|mode)$'
```

input表のversion/toolchain syntaxは直接validateInputsへ通す。packageRelease側で先にlicense/versionの別条件に拒否されて通る偽の検査へしない。旧参照の削除後、src全体の通常/tag/OS別callerとimportを静的検索する。

### S4 候補検証の再解凍・真偽表を縮小（P06–P08）

独立license/source/mode期待値を維持した返値共有へ変更し、Metadataは6target、Nativeは選択targetへ責任を分ける。

```sh
go test -count=1 ./src/cmd/aidlc-dist -run '^Test(CandidateLicense|BundledRelease)$'
go test -tags=integration -count=1 ./src/cmd/aidlc-dist -run '^TestReleaseCandidateMetadataValidation$'
go test -tags=integration -list '^TestReleaseCandidate(Metadata|MetadataValidation|Native)$' ./src/cmd/aidlc-dist
```

listは3名の存在確認だけ。実BuildInfo/native起動を行ったとは扱わない。手作りBuildInfo/文字列表を別名で残さない。

### S5 Release導入のfixture・表を集約（P19/P20）

validへdownload/fresh license、offlineへ欠損、reservationへFetch未呼出assertを移し、元testを削る。

```sh
go test -count=1 ./src/internal/install -run '^Test(ReleaseAssetValidationCandidate|ReleaseAssetValidationOffline|InstallReservation|InstallFailurePartial|ReleaseLicenseRetention|ReleaseDownload|ReleaseDownloadHTTP|ReleaseDownloadRedirectMustRemainHTTPS|InstallerCommandRelocation)$'
```

### S6 bootstrapの同値直積と圧縮準備（P10）

同formatの正常archive bytesだけ共有。異常archiveのbytes、result/sentinel/curlログ、tempのcleanupはcaseごとに分離する。

```sh
go test -count=1 ./src/bootstrap -run '^TestBootstrap$'
go test -list '^TestBootstrapPowerShell$' ./src/bootstrap
```

Windowsでは `go test -count=1 ./src/bootstrap -run '^TestBootstrapPowerShell$'` がPS5.1/7の22caseを実行する。非Windowsのskip/listは動的GREENに数えず、同headのDistribution Windows jobを必須gateへ残す。

### S7 自然日本語の重複・重い入力（P22–P25/P27/P28/P31）

独自数値期待はliteralに保ち、手作りtokenで文字数/名詞終端を検査。実Analyzeを使う意味あるLexical接続1例とMorphの意味例は保持。TestRhythmは入力ごとに結果を1回得る。

```sh
go test -count=1 ./src/internal/naturaljapanese -run '^Test(Analyzer|AnalyzerEmpty|Surface|SurfaceSeverity|SurfaceCatalogOrder|Report|Baseline|BaselineInvalid|BaselineExcerptRequired|Text|Lexical|LexicalMTLD|LexicalSpecificity|LexicalTTRBoundary|Morph|MorphNominal|MorphCharacterBoundary|Rhythm|RhythmMora|RhythmBurstinessBoundary)$'
go test -count=1 ./src/cmd/natural-japanese-go -run '^Test(Command|CommandFile|CommandAnalysis|CommandBaselineError|CommandBaselineExcerptRequired|CommandDiagnostics)$'
```

### S8 独自path判定の直積（P29/P30）

destinationをpath分類の正本、source/generatedは接続代表とする。TestManifestRootTree等の特例は削らない。

```sh
go test -count=1 ./src/harness -run '^TestManifest(Render|RootTree|RejectsInvalid)$'
go test -count=1 ./src/core/workflow -run '^TestRenderRejectsDuplicateCompletedPaths$'
```

末尾は変更Go fileだけgofmt、全sliceのtargeted群、`git diff --check`を一度まとめて確認して親へ返す。成功・skip・listのみを分けて記録。親は全差分とtargeted群を境界で一度再確認する。

## review / final / CI

独立reviewは、26候補の削除→生存保証の対応、old inputによる前段拒否、manifest/checksum再計算したsource/license誤配布、実mode、未使用参照群、mutable data共有、早期/後期保存失敗の残存を固定headで確認する。全検証は代行しない。

blocking finding解消後、親がread-only finalを1回開始する。

```sh
go test -count=1 -shuffle=on -coverprofile=/tmp/ai-dd-test-reduction-m2-coverage.out ./...
go test -count=1 -race -shuffle=on ./...
go vet ./...
go mod tidy -diff
gofmt -l src
git diff --check
go test -tags=integration -count=1 ./src/internal/workspace ./src/internal/okf
go test -tags=integration -count=1 ./src/cmd/aidlc -run '^Test(FlowJourney|GitIndependentJourney)$'
```

gofmtは空を要求するcheckだけで、finalでは適用しない。integrationを含めた現在の製品testsの実施範囲はM1の親finalと一致させ、P26のstandalone実binary入口はM2では残す。**30binaryの重複ローカルbuild/配布E2Eは行わず、正規配布E2Eは同headのGitHub Distribution Package＋3OS Nativeへ集約する**。

PRではMetadata/MetadataValidationのPackage、同artifact IDのNative3OS、実5CLIのversion/help・offline installer・移転、bootstrap実候補、WindowsのPS5.1/7がすべて成功することを親が確認する。候補環境変数なしのskipを成功に数えない。draft前再照合・remote tag照合・公開権限境界は変えない。Release/tagの実操作はしない。

final後の修正は証拠をstaleとして必要なloop/review/finalへ戻す。現headのchecksすべて成功後に親がIssue連結PRをmergeし、main反映・Issue closeを確認。M3はmerge済みmainから始める。

## 保持判断・未決事項・rollback

未承認の重要な製品判断を新たに選ぶ必要はない。P31は公開順序を保持、P09は現partial保存契約を保持、P21は別decoderの現在の保証を区別する方針で一意に進められる。

実装中に、旧参照群へ未知の現callerが見つかる、前段の別拒否しか通らない、ライセンス/権限の期待値を一次資料から一意に置けない、実際の動作修正を要するfailureが出る場合は、元の固有保証を保持して親へ返す。削減のために仕様やtest合格条件を緩めない。

rollbackは問題のM2 PRを戻すPRで行い、M1や他の変更をresetしない。元の作業tree `/Users/const/sori883/ai-dd` を触らない。この計画は一時fileだけへ保存し、repo/GitHub/testは変更・実行していない。
