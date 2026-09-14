# M4 テスト削減の実装結果

Issue [#210](https://github.com/sori883/ai-dd/issues/210)、work_unit_id `test-reduction-m4`、verification_mode `loop`。
承認済み[M4計画](test-reduction-m4-plan.md)の36候補を単独writerでS1–S8の順に整理した。公開挙動・保存形式・権限の変更はない。製品Goの変更はproduction caller不在を確認した `readDefaultOrganization` の削除だけである。

開始・終了HEADは `7ba542a9bb29f9a9bbde9ed563660e1b93820c64`（commit操作なし）。作業rootは `/Users/const/sori883/ai-dd-release`、branch `codex/test-reduction-m4`。独立review・final・同head checks・mergeは未実施で親が続ける。

## 各候補の処置と生存保証

全行は変更前の生存検査がALREADY_GREEN、移動・削減後もGREEN。Q番号は下の正確なcommand一覧を指す。

| ID | 実施した処置 | 残した保証・理由 | 証拠 |
|---|---|---|---|
| A02 | 廃止install/memory等の重複表・専用入口を削除。 | FiveCLIContractの4入口、code 2・Execute未呼出、現行reassign文法。 | S1/Q1–3 | 
| A03 | 未使用readDefaultOrganizationと専用6例/helperを削除。 | 現在のreadDefaultRuleとCreateSpaceOKF、実Rule読取り・link境界。 | S1/Q2–3 | 
| A04 | FlowSpaceADRと空fileを削除。 | CreateSpaceOKFのpath/内容、CreateSpaceScaffold。 | S1/Q2–3 | 
| A05 | CreateSpaceClaimsNewTargetの2例を削除。 | Team Alpha正規化と実scaffold内容。 | S1/Q2–3 | 
| A06 | SwitchSpaceRejectsCursorSymlinkを削除。 | SwitchSpaceCursorLinkBoundariesの実link拒否と旧bytes不変。 | S1/Q3 | 
| A07 | CRUD2関数をMemoryCommandForwardingへ置換。 | 実okfappへのcreate/update、非default Space/Target、body・actor・metadata・Intent検索・stale Expect拒否。CRUD詳細はokfapp。 | S2/Q4–5 | 
| A08 | CodeKBMemoryを削除。 | CodeKBBeginRepair、SessionMemoryShowOriginalHash、OKFWorkLogSearchAndShow。 | S2/Q4 | 
| A09 | pending hookの3fixtureを1表へ統合、旧2関数と空fileを削除。 | hash/OKF skill許可を移動。各allowed行後PostToolUse、write拒否と既存許可。 | S3/Q6 | 
| A10 | Rule/OKF読取りとFlowInactiveの拒否反復を縮小。 | 登録path正常・family旧配置・before begin、StageSkillsRead共通拒否、3status正常read/resume、current Rule/実file条件を保持。 | S3/Q6 | 
| A11 | ChildHookSplitCLIを削除し2拒否行をChildHookCommandBoundaryへ移動。 | 子rules read・create/update/__hook拒否と親state不変。 | S3/Q6 | 
| A12 | 3adapterのroot優先順位をexplicit代表へ縮小。 | ResolveRootへabsolute候補/relative cwdを移動。各adapterの転送と他root未使用。 | S4/Q7–8 | 
| A13 | relative拒否とopen error表を各入口1代表へ縮小。 | wrapped cause、zero result、不正rootのopen未呼出、実missing/file。 | S4/Q7–8 | 
| A14 | filesystem smoke1例とfallback2例へ縮小。 | cursor文字列/順序は単体、DirFSとos.Rootの別symlink境界は保持。 | S4/Q7–8 | 
| A15 | 名称14行を3行へ縮小。 | 同target再保存とprotected data、list許可、Help正規化およびraw help拒否。 | S4/Q8 | 
| A16 | 公開routeのflag位置表を共通parserへ集約。 | WorkspaceArgumentsの正常7行とcreate/switch/list/bareの実explicitDir転送。 | S5/Q10 | 
| A17 | 共有parserのinvalid表へ集約。 | invalid8行、equals dash正常対照、各routeの名前数/raw help/JSON可否/分類/callback未呼出。 | S5/Q10 | 
| A18 | help全文literalとspace別用語testを削除、root用語反復を縮小。 | 公開操作を1回独立確認、実先頭出力と別名一致、exitとcallback未呼出、unknown prefix/元引数/help案内。 | S6/Q11 | 
| A19 | help語彙列・stage反復を縮小。 | 文書実path/必須H2、Mermaid/intent条件、Sensorの限界、readonly・操作順・承認区別・再取得・required flags・実command/path・再割当安全条件は理由付き保持。 | S6/Q11 | 
| A20 | ConfigureHelpと空fileを削除、skill helpの件数下限/末尾辞書を除去。 | 全抽出help routeの到達性。実JSON投入ConfigureHelpExamplesは親finalで実行する。 | S6/Q11（実CLI例はfinal責任） | 
| A21 | 共通stdout失敗をHelpWriteErrorの1例へ集約。 | help/version/check-help成功dispatch。workspace独自short-write/JSON errorは維持。 | S6/Q11 | 
| A22 | stderr共通失敗をswitch代表へ集約、fakeの保存prefix検査を除去。 | 各routeのcallback error/JSON error/stdout failure/short-writeと非成功exit。 | S6/Q11 | 
| A23 | 出力準備の掛合せと全event列一致を縮小。 | 各route成功/構文拒否/callback失敗、PrepareOutputが最初に1回、help/version未呼出。 | S6/Q11 | 
| A24 | 旧hook拒否をFiveCLIContractへ集約、Run後のParseCommand重複を削除。 | 現行callbackのCommand/Action/ProjectDirとexit。 | S1/Q1・S5/Q10 | 
| A25 | workspaceのlowercase12例を削除。 | pathnorm固定vector/Windows限定foldingとworkspace固定lock filename2例。 | S7/Q12–13 | 
| A26 | overlay配列長・重複U+A7F1・unicode.Version gateを除去。FinalSigma7例をpathnormへ移動。 | 独立oracle不足のため3つのoverlay範囲本体を保持。共有alias/appendRuneRangeも生存callerあり。 | S7/Q12–13・移動前後単独 | 
| A27 | lock testのMD5再計算・中間identity/digestを除去。 | 別lexical aliasの同一lock、fallbackと正常lexical解決の一致、temp parent、固定filename2例。 | S7/Q13 | 
| A28 | 重複entry fake反復とerror位置/種類を縮小。 | 実default重複抑止、partial data/entries+error、middle Stat failureの前後結果/cause。共有readDirResultFSは保持。 | S4/Q7 | 
| A29 | nonregularを3種へ縮小、cursor失敗の全steps一致を除去。 | cause・早期失敗後rename禁止・own temp cleanup、permission/close後rename。CompleteCursorNoReplaceFailuresは既にobservableなので維持。 | S7/Q13–14 | 
| A30 | 注入Close後の標準Root.Stat再検査を削除。 | Close回数/cause/zero result/保存bytes。SwitchSpaceSavesNormalizedNameのdefault Close検査は独自なので保持。 | S7/Q14 | 
| A31 | 2field表をInfoの直接比較へ簡略化。 | dev/unknownの既定値。 | S7/Q15 | 
| A32 | wrapper invalid4例をempty type拒否1例へ縮小。 | 正本ParseConceptInvalidとinvalid type拒否のwrapper伝播、RoundTripのunknown metadata/body。 | S8/Q16–17 |
| A33 | usage windowの8×2をtop8＋source2へ縮小。 | top levelの形式/offset/from/to、source overrideの正常/拒否配線。 | S8/Q17 | 
| A34 | Scan末尾Tags改変後再Scanを除去。 | selection/warning/path/UTF-16順序、SearchLifecycleAndOwnershipの現実的な所有権。 | S8/Q17 | 
| A35 | session testから汎用stale CAS/update metadata反復を除去。 | create/update/search後選択不変、他Space検索空、index directory時error+JSONと保存Concept、raw hash。 | S2/Q4 | 
| A36 | git init・install後workflow再配置・err=nil後の死んだ3blockを除去。 | 担当/親子境界と生成workflowを使うsession5代表。installしないfixtureのdeployProcedureFixtureは保持。 | S3/Q6 | 
| A37 | ancestor obstructionのexplicit重複を除去。 | cwd経由2obstructionとResolvePreservesErrorsのexplicit override/原因保持。 | S4/Q9 | 

## 検証と途中修復

各sliceの変更前・移動後・削減後と末尾のcommand/exit/outputは `/tmp/m4-evidence.json` に保存した。末尾は下記17群がすべてexit 0。新しいWorkspaceArgumentsと移動するFinalSigmaは単独でも実行し、実testの成功を確認した。gofmtとgit diff --checkも成功。loopでは全体test/race/vet/cross-build/journey/配布E2Eを実行していない。

A07の新fixtureは初回にcreateの `CommandRequest.IntentID` を保存metadataと誤解した。現行Serviceのcreate/updateは `Metadata.IntentID`、searchは `CommandRequest.IntentID` を使うため、親が確認した一意の入力修復を適用した。新例成功後に旧CRUDを削除し、製品は変更していない。この失敗は有効REDではない。削除時の未使用import、残存help定数参照、構文片、captured変数の定型修復も有効REDに数えず、修復後の同targeted成功を記録した。

A36の死んだerr blockは計画の2箇所に対し実fileで同形3箇所があったため3箇所を除いた。ConfigureHelp削除後の空fileも除去した。未解決の製品bugや追加承認を要する判断はない。

## 正確な末尾command

Q1: exit 0

```sh
go test -count=1 ./src/internal/cli -run '^Test(FiveCLIContract|CommandRejectsInvalid|RelocationCLI|MemoryHelp|HookCommandDispatch)$'
```

Q2: exit 0

```sh
go test -count=1 ./src/internal/workspace -run '^Test(CreateSpaceOKF|CreateSpaceOKFFallbackAndInvalid)$'
```

Q3: exit 0

```sh
go test -tags=integration -count=1 ./src/internal/workspace -run '^Test(CreateSpaceScaffold|CreateSpaceCopiesOnlyDefaultOrganization|CreateSpaceSymlinkBoundaries|CreateSpaceOrganizationDirectoryIsError|SwitchSpaceCursorLinkBoundaries)$'
```

Q4: exit 0

```sh
go test -count=1 ./src/internal/app -run '^Test(MemoryCommandForwarding|CodeKBBeginRepair|SessionMemoryWritesAndSearchPreserveSelection|SessionMemoryBoundaries|SessionMemoryShowOriginalHash|OKFWorkLogSearchAndShow)$'
```

Q5: exit 0

```sh
go test -count=1 ./src/internal/okfapp -run '^Test(OKFMemoryContract|OKFSaveFailure|OKFConcurrentUpdate)$'
```

Q6: exit 0

```sh
go test -count=1 ./src/internal/app -run '^Test(ExecutionPlanCLIPendingHook|RuleSkillSeparationHook|OKFSkillRead|StageSkillsRead|FlowInactiveWorkflowReadAndResume|FlowWorkflowReadRequiresCurrentRulesAndRealFiles|ChildHookCommandBoundary|HookSplitCLI|HookSplitCLIPost|AssignmentContract|AssignmentContractOwnerBoundary|ChildReportBoundary|ChildReportToParent|RulesFullTextAndLimit|SessionStartLoadsPlacedSkillOnly|SessionMemoryWritesAndSearchPreserveSelection|SessionMemoryBoundaries|SessionMemoryShowOriginalHash)$'
```

Q7: exit 0

```sh
go test -count=1 ./src/internal/workspace -run '^Test(ResolveRoot|ReadSpacesRootPrecedence|SwitchSpaceRootPrecedence|ReadSpacesRejectsRelativeRoot|ReadSpacesInvalidRootDoesNotOpen|ReadSpacesProjectOpenError|CreateSpaceRejectsRelativeRoot|CreateSpaceProjectOpenError|SwitchSpaceRejectsRelativeRoot|SwitchSpaceProjectOpenError|ActiveSpace|ActiveSpaceFallback|ListSpacesDefault|ListSpacesSelection|ListSpacesUnique|ListSpacesOrder|ListSpacesReadDirError|ListSpacesStopsOnStatError)$'
```

Q8: exit 0

```sh
go test -tags=integration -count=1 ./src/internal/workspace -run '^Test(CreateSpaceRootPriority|SpaceReadersFilesystem|ReadSpacesFallbacks|SpaceReadersSymlinks|ReadSpacesSymlinkBoundaries|ReadSpacesInitialProjectSymlink|ReadSpacesProjectOpenIntegration|CreateSpaceRequiresExistingProject|SwitchSpaceNamesAndProtectedData|SwitchSpaceSavesNormalizedName)$'
```

Q9: exit 0

```sh
go test -count=1 ./src/internal/projectroot -run '^Test(ResolveAncestorFiles|ResolvePreservesErrors)$'
```

Q10: exit 0

```sh
go test -count=1 ./src/internal/cli -run '^Test(WorkspaceArguments|RunSpaceCreate|RunSpaceSwitch|RunSpaceListJSON|RunSpaceBareAlias|RunSpaceCreateInvalidArguments|RunSpaceSwitchInvalidArguments|RunSpaceSwitchInvalidRawName|RunSpaceListExtraArguments|RunSpaceListDuplicateJSON|RunSpaceListUnknownSubcommands|HookCommandDispatch|FiveCLIContract)$'
```

Q11: exit 0

```sh
go test -count=1 ./src/internal/cli -run '^Test(Run_Help|Run_HelpWriteError|Run_Version|Run_UnknownArguments|CheckHelpEvidenceContract|CheckHelpDocumentContracts|CheckHelpDocumentVersionsAndOptionalOutputs|CheckHelpWorkflowAndProcedure|CheckHelpForms|CheckHelpRejectsExecutionArguments|CheckHelpPreservesBeginHelp|AssignmentContract|ExecutionPlanCLIGrammar|MemoryHelp|MemoryHelpUnitConfirmVerification|CodeKBHelp|OKFWorkLogHelp|GitIndependentInstallReviewHelp|RelocationCLI|RuleSkillSeparationHelp|RunSpaceCreateFailureJSON|RunSpaceCreateStdoutFailure|RunSpaceCreateShortStdoutWrite|RunSpaceSwitchShortStdoutWrite|RunSpaceSwitchOutputFailures|RunSpaceListReadFailure|RunSpaceListPartialStdoutFailure|RunSpaceListShortStdoutWrite|RunSpaceOutputPreparation|RunSpaceSwitchOutputPreparation|RunSpaceListOutputPreparation)$'
```

Q12: exit 0

```sh
go test -count=1 ./src/internal/pathnorm -run '^Test(ECMAScriptDefaultLowerFixedWindowsVectors|NormalizeForPlatformOnlyWindowsFoldsCase|ECMAScriptDefaultLowerUsesUnicode15FinalSigmaContext)$'
```

Q13: exit 0

```sh
go test -count=1 ./src/internal/workspace -run '^Test(WorkspaceLockPathUsesCanonicalWorkspaceIdentity|WorkspaceLockPathFallsBackToLexicalAbsolutePath|WorkspaceLockPathMatchesKnownWindowsUnicodeIdentity|WorkspaceLockPathKeepsBunWindowsUnicode15Identity|ECMAScriptDefaultLowerKeepsUnicode15IdentityRunes|IsCasedUsesUnicode15Overlay|IsCaseIgnorableUsesUnicode15Overlay|ReplaceSpaceCursorNonRegular|ReplaceSpaceCursorPreservesPermissions|ReplaceSpaceCursorShortWrite|ReplaceSpaceCursorFailures|CompleteCursorNoReplacePublishesAfterClose|CompleteCursorNoReplaceFailures)$'
```

Q14: exit 0

```sh
go test -tags=integration -count=1 ./src/internal/workspace -run '^Test(ReadSpacesProjectClose|CreateSpaceProjectClose|SwitchSpaceRootCloseFailures|SwitchSpaceSavesNormalizedName|SwitchSpaceCursorFailurePreservesOldFile|SwitchSpaceCursorLinkBoundaries)$'
```

Q15: exit 0

```sh
go test -count=1 ./src/internal/buildinfo -run '^TestCurrent_Defaults$'
```

Q16: exit 0

```sh
go test -count=1 ./src/internal/okfmemory -run '^Test(ParseRejectsInvalid|ParseRoundTrip)$'
```

Q17: exit 0

```sh
go test -count=1 ./src/internal/okf -run '^Test(ParseConceptInvalid|ParseConceptUsageWindow|ScanBundleSelection|SearchLifecycleAndOwnership)$'
```

単独移動検査（各exit 0）:

```sh
go test -count=1 ./src/internal/cli -run '^TestWorkspaceArguments$'
go test -count=1 ./src/internal/workspace -run '^TestECMAScriptDefaultLowerUsesUnicode15FinalSigmaContext$'
go test -count=1 ./src/internal/pathnorm -run '^TestECMAScriptDefaultLowerUsesUnicode15FinalSigmaContext$'
```

変更fileの一覧とSHA-256は `/tmp/m4-files-sha256.txt`、差分hashは `/tmp/m4-diff-sha256.txt` に保存する。未追跡の新parser testと結果docもfile hash一覧へ含める。

## 独立review修復（同work unit）

HEAD `1b120a3c19db6e7e6b28e59d02c3338cb586aacc` 上で3件を修復した。A16/A17の正常表1行を ` path ` の入力と同じ期待値へ変更し、旧ListProjectDirLiteralの無加工転送保証を保持。A21の無参照checkHelpFailingWriter型/Writeと専用errors importは、src内の参照が定義だけであることを確認して削除した。A32は終端のないbodyではwrapper自身でも拒否されるため、下位ParseConceptの不正type拒否が必要な `---\ntype: ''\n---\n` へ代表入力を置換した。テスト件数・製品Goは変更していない。

次の3commandを変更前ALREADY_GREEN・変更後GREENとして各1回実行し、全6実行がexit 0。履歴は `/tmp/m4-review-fixes.json`。gofmt・git diff --checkも成功。再review/finalは親が行う。

```sh
go test -count=1 ./src/internal/cli -run '^TestWorkspaceArguments$'
go test -count=1 ./src/internal/cli -run '^TestCheckHelpForms$'
go test -count=1 ./src/internal/okfmemory -run '^TestParseRejectsInvalid$'
```
