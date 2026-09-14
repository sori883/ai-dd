# M4: app・CLI・workspace・OKFの重複テストを削減する詳細計画

作成日: 2026-09-14。作業rootは /Users/const/sori883/ai-dd-release。元の /Users/const/sori883/ai-dd は触らない。

開始基準: M3マージ済みmain `b958199018563f24178106f0ec365bb2cfbedd8b`。対応は [Issue #210](https://github.com/sori883/ai-dd/issues/210)、branchは `codex/test-reduction-m4`。M3の変更はflow・assignment・workflowのtestsで、M4の対象へ新たな契約変更を加えていない。M2で整理したfixtureは保持する。F41は通常suiteに保持されたためstress tagの追加責任はない。

## 目的・許可・開始条件

AI-DDの公開挙動を保ち、同じ検査を各入口で繰り返すテスト、廃止された入口で先に拒否されて検査対象まで到達しないテスト、標準処理やテスト用helper自身の再検査を削減する。対象は監査A02–A37の36候補。件数やcoverage率の維持を目的にテストを補充しない。

実装許可は docs/ram/decisions/2026-09-14-test-reduction-approved.md に記録された、監査に基づく削除・統合を区切ってIssue/PR/mergeするユーザーの直接依頼である。上位計画は docs/design/test-reduction-milestones.md のM4、根拠一覧は docs/test-suite-reduction-audit.md。追加の外部module、汎用test framework、製品契約は導入しない。保存競合・部分書込み・復旧、承認・親子・担当、内容SHA、OKF metadata/search、CLI転送、OS固有境界は保持する。

この案はM1レビュー済みHEAD a17f6ebe379b72dac41338489ec4a8c2684f5434 時点の確認を基礎とする。M3 merge後に親が開始HEAD・Issue・この計画の保存先を固定する。M2/M3が変更した生存関数・helperと競合する部分だけ親が照合し、他者の編集を戻さない。行番号は後続mergeで変わるため、以下の正確なfile名・関数名を編集境界とする。重大な未承認判断はない。

work_unit_id=test-reduction-m4、実装verification_mode=loop。1 Issue/PR、1人のwriterでS1–S8を順番に実施する。親はIssue・独立review・read-only final・PR/checks/mergeを管理する。plannerはrepo編集、test実行、GitHub操作をしていない。

## 所有範囲と進め方

以下に列挙したtest fileと、その変更でcallerがなくなる専用helper/importがwriterの所有範囲。製品Goで削除するのは src/internal/workspace/space_create.go の未使用 readDefaultOrganization のみ。参照検索でproduction callerが存在しないことを開始HEADで再確認する。他の製品Goには変更を加えない。

新しいtest fileは共有parserを直接検査する src/internal/cli/workspace_arguments_test.go（package cli）の1件だけで足りる。appの短い転送検査は既存 memory_body_test.go 内へ置き、productionへの注入点は作らない。既存のhelperはfileを削る前に全callerを検索し、残るtestが使うものを保持する。特に bodyRequest/bodyString、deployProcedureFixture、cursorOps、root/open用fakeをまとめて消さない。

各sliceでは、後述の生存testを変更前に実行してALREADY_GREENを記録する。保証を統合する場合は生存先へassertを先に移して実行し、元の重複を削る。既存製品に対して初めから通ることを正直に記録し、人工的REDや製品変更は作らない。終了時にそのsliceの同じtargetedを実行する。新しい観測可能な不具合が判明した場合は削減へ紛れ込ませず親へ返す。

M1で削除済みの別名Testは復活させない。とくにappの RuleSkillSeparationHookApprovalPending / HookSplitCLIRepair、workspaceの RuleSkillSeparationRuleCopy、okfcli/okfmemoryのalias OKFMemoryContract、flow/journeyの旧別名をtargetedへ入れない。src/internal/okfapp/command_test.go の TestOKFMemoryContract は実検査であり生存する。

## S1: 廃止入口・旧reader・弱い存在検査（A02–A06、A24の廃止入口）

| 候補 | 正確な処置 | 生存する保証・削除理由 |
|---|---|---|
| A02 | src/internal/cli/project_root_test.go の TestProjectRootWithoutGitInstall を削除。relocation_test.go の TestRelocationCLI からinstallの先頭ケースと old without relocate / missing old binary / duplicate / relative を削り、TestRelocationCLIEmptySourceFlags を削除。command_test.go の TestCommandRejectsInvalid から actor_missing / space_missing / empty_id / duplicate / unknown を削り mixed_name_id を維持。memory_help_test.go の TestMemoryHelp からmemory unknown/createの廃止入口3行を削除。 | five_cli_test.go の TestFiveCLIContract にinstall・memory・kdr・旧hookの公開入口拒否を各1行に集約する。既存行を重複追加しない。各行は終了code 2とExecute未呼出を維持。現行unit reassignの正常文法は TestRelocationCLI に残る。退役入口を細かく変えても内部validatorに届かない。 |
| A03 | src/internal/workspace/space_create_test.go の TestReadDefaultOrganizationFailures 全6ケースと、space_create.go の readDefaultOrganization を削除。専用helper/importは無参照のものだけ削除。 | 現在の populateSpace → readDefaultRule は保持。space_okf_test.go の TestCreateSpaceOKF / TestCreateSpaceOKFFallbackAndInvalid と、integrationの CopiesOnlyDefaultOrganization / SymlinkBoundaries / OrganizationDirectoryIsError が現在のRule読取りを検査する。旧memory/org.md readerにはproduction callerがない。 |
| A04 | src/internal/workspace/flow_test.go の TestFlowSpaceADR を削除し、空になるfileを削除。 | TestCreateSpaceOKF の必要path・内容と TestCreateSpaceScaffold を維持。弱い存在確認だけの重複を除く。 |
| A05 | src/internal/workspace/space_create_integration_test.go の TestCreateSpaceClaimsNewTarget 全2ケースを削除。 | 同fileの TestCreateSpaceScaffold、space_okf_test.go の TestCreateSpaceOKF のTeam Alpha正規化と作成結果を維持。target directoryが存在するだけの確認を除く。 |
| A06 | src/internal/workspace/space_switch_integration_test.go の TestSwitchSpaceRejectsCursorSymlink を削除。 | 同file TestSwitchSpaceCursorLinkBoundaries の inside relative と、旧target内容・cursor無変更のassertを維持。同じ実symlink拒否の重複。 |
| A24（前半） | src/internal/cli/command_test.go の TestHookCommandDispatch の廃止hook拒否をA02のTestFiveCLIContractへ集約。 | 旧入口拒否1例を残し、現行Run経由のcallback転送・exit検査を残す。後半のParseCommand重複はS5で除く。 |

targeted（rootから実行）:

~~~sh
go test -count=1 ./src/internal/cli -run '^Test(FiveCLIContract|CommandRejectsInvalid|RelocationCLI|MemoryHelp|HookCommandDispatch)$'
go test -count=1 ./src/internal/workspace -run '^Test(CreateSpaceOKF|CreateSpaceOKFFallbackAndInvalid)$'
go test -tags=integration -count=1 ./src/internal/workspace -run '^Test(CreateSpaceScaffold|CreateSpaceCopiesOnlyDefaultOrganization|CreateSpaceSymlinkBoundaries|CreateSpaceOrganizationDirectoryIsError|SwitchSpaceCursorLinkBoundaries)$'
~~~

## S2: appのOKF転送を残し、CRUDの検査を所有packageへ（A07、A08、A35）

| 候補 | 正確な処置 | 生存する保証・削除理由 |
|---|---|---|
| A07 | src/internal/app/memory_body_test.go の TestMemoryBodyWrite を短い TestMemoryCommandForwarding へ置換。TestMemoryBodyWriteRejectsAndPreserves の7ケースを削除。 | app Service.Execute のRoot、Action、Target、Space、BodyFile、Actor、Expect、IntentID、Metadataの取り落としを、実okfappを通るcreate/updateの短い連続例で検出する。非defaultのSpace/Target、actorとmetadataを変えたupdate、body bytes、Intent検索、stale Expect拒否を1回ずつ検査し、ProjectDirはapp Service.Rootとの既存関係に従う。生成時刻の窓、no-op、index/log、全invalid表をapp側へ複製しない。CRUD/CAS/拡張metadata/失敗時保全の所有者は src/internal/okfapp/command_test.go の実 TestOKFMemoryContract / TestOKFSaveFailure / TestOKFConcurrentUpdate。 |
| A08 | src/internal/app/codekb_test.go の TestCodeKBMemory 全体を削除。 | 同file TestCodeKBBeginRepair、session_test.go の TestSessionMemoryShowOriginalHash、work_log_test.go の TestOKFWorkLogSearchAndShow とA07転送例を維持。汎用create/show/searchの繰返しを除く。 |
| A35 | src/internal/app/session_test.go の TestSessionMemoryWritesAndSearchPreserveSelection からstale CASの再試行部分を削除。TestSessionMemoryBoundaries から汎用update/custom metadata保持の部分を削除。 | 前者のcreate/update/search後も選択session fileが不変であるassertを維持。後者の他Space検索が空、indexがdirectoryのときcreateがerrorと結果JSONを返しConceptの保存が残る部分を維持する。部分成功結果をappが捨てない独自保証である。TestSessionMemoryShowOriginalHash はraw frontmatterと元hashのまま維持。CASはA07とokfappが所有。 |

A07はfieldを直接比較する偽dispatcherを追加しない。現在のokfapp実行結果から観測できないfieldを検査するためだけに製品にseamを追加しない。既存requestの意味が変わる場合は、重複CRUDを先に消さず該当の短いassertを残す。

~~~sh
go test -count=1 ./src/internal/app -run '^Test(MemoryCommandForwarding|CodeKBBeginRepair|SessionMemoryWritesAndSearchPreserveSelection|SessionMemoryBoundaries|SessionMemoryShowOriginalHash|OKFWorkLogSearchAndShow)$'
go test -count=1 ./src/internal/okfapp -run '^Test(OKFMemoryContract|OKFSaveFailure|OKFConcurrentUpdate)$'
~~~

変更前の最初のcommandのみ、未作成のMemoryCommandForwardingに代えてMemoryBodyWriteを指定する。他の生存名は同じ。新しい転送例の実行成功を確認してから旧2関数を除く。

## S3: hookの共通拒否と固有接続を整理（A09–A11、A36）

| 候補 | 正確な処置 | 生存する保証・削除理由 |
|---|---|---|
| A09 | src/internal/app/verification_gates_test.go の TestVerificationGatesPendingHook と execution_plan_test.go の TestOKFSkillReadApprovalPending を、同fileの TestExecutionPlanCLIPendingHook へ統合。前者のhash許可と後者のOKF skill読取りを同じ表の2行に追加してから旧2関数を削除。空fileを削除。 | 通常write拒否、aidlc/cli skill読取り、code読取り、plan閲覧、plan-approval実行、intent hash、OKF skill読取りを各1行。allowed行はPostToolUseでinflightを解放して次行へ進み、別の前段拒否で通る表にしない。同一fixtureを3回構築していた負担を除く。 |
| A10 | src/internal/app/rule_skill_separation_test.go の TestRuleSkillSeparationHook / TestOKFSkillRead、stage_skills_test.go の TestStageSkillsRead、flow_test.go の TestFlowInactiveWorkflowReadAndResume / TestFlowWorkflowReadRequiresCurrentRulesAndRealFiles を整理。unread/inflight/missing/symlink/redirect/compound/arbitrary/retiredの共通拒否はTestStageSkillsReadへ各1例。 | Rule用とOKF用の登録済みpathの正常読取り、family固有の旧配置拒否、before begin等の固有条件は各元testへ残す。FlowInactiveのcompleted/waiting/pausedは各正常readとresume/reopen後の作業まで維持する。7拒否×3statusの反復を各statusの1拒否へ縮小し、共通parserの悪意ある文字列表を掛け合わせない。current Ruleと実fileが必要な固有assertはFlowWorkflowReadRequiresCurrentRulesAndRealFilesに維持。製品hook許可規則は変更しない。 |
| A11 | src/internal/app/hook_split_cli_test.go の TestChildHookSplitCLI を削除。create拒否とOKF unknown/__hook拒否を child_hook_test.go の TestChildHookCommandBoundary の表へ先に移す。 | 同表のrules readとupdate拒否を維持。createとupdateは別routingなので各1行、親state無変更を保持。子fixtureと同じrules readの重複を除く。 |
| A36 | src/internal/app/assignment_test.go と child_report_test.go のfixture準備にあるgit initを除く。session_test.go の setup でinstall後に再度行う deployProcedureFixture を除く。flow_test.go のTest内の err=nil; if err != nil の死んだblock2箇所を除く。 | TestAssignmentContract / TestAssignmentContractOwnerBoundary / TestChildReportBoundary / TestChildReportToParent は実directoryと担当・親子境界を維持。installの生成workflowを使うsession fixtureを全session代表で確認する。installを呼ばない部分fixtureの deployProcedureFixture は保持する。 |

~~~sh
go test -count=1 ./src/internal/app -run '^Test(ExecutionPlanCLIPendingHook|RuleSkillSeparationHook|OKFSkillRead|StageSkillsRead|FlowInactiveWorkflowReadAndResume|FlowWorkflowReadRequiresCurrentRulesAndRealFiles|ChildHookCommandBoundary|HookSplitCLI|HookSplitCLIPost|AssignmentContract|AssignmentContractOwnerBoundary|ChildReportBoundary|ChildReportToParent|RulesFullTextAndLimit|SessionStartLoadsPlacedSkillOnly|SessionMemoryWritesAndSearchPreserveSelection|SessionMemoryBoundaries|SessionMemoryShowOriginalHash)$'
~~~

S3後、以後のtargetedに削除したOKFSkillReadApprovalPendingを残さない。共有setup変更の確認対象は上記のsession系5関数であり、app全packageをloopで実行しない。

## S4: root解決・filesystem smoke・名称表（A12–A15、A28、A37）

| 候補 | 正確な処置 | 生存する保証・削除理由 |
|---|---|---|
| A12 | src/internal/workspace/space_read_test.go の TestReadSpacesRootPrecedence、space_switch_test.go の TestSwitchSpaceRootPrecedence、space_create_integration_test.go の TestCreateSpaceRootPriority を各1例へ縮小。全候補rootを同時設定してexplicitが選ばれ、他rootが使われない例を残す。 | root_test.go の TestResolveRoot に優先順位・relative解決を集約。Read側だけにある「absolute候補＋relative cwd」の例が未収録なら同表へ移す。各公開adapterのexplicit転送は各1例を維持。 |
| A13 | space_read_test.go の TestReadSpacesRejectsRelativeRoot をrelative explicit拒否1行へ、TestReadSpacesInvalidRootDoesNotOpenを保持。同packageのcreate/switchのRejectsRelativeRootも各1行。3入口のProjectOpenError表を各1個のwrapped sentinelへ縮小。 | errors.Isでcauseを保持、zero result、不正rootではopen未呼出を各入口で残す。root候補の優先順位やmissing/permission/arbitraryを各adapterで掛け合わせない。実missing/fileの拒否はintegrationのProjectOpen/RequiresExistingProjectへ残す。 |
| A14 | space_integration_test.go の TestSpaceReadersFilesystem を正常filesystem smoke1例へ、space_read_integration_test.go の TestReadSpacesFallbacks をuninitializedとaidlcがfileの2例へ縮小。 | space_test.go の TestActiveSpace / TestActiveSpaceFallback / TestListSpacesDefault / TestListSpacesSelection / TestListSpacesOrderがcursor文字列・既定選択・UTF-16順序を所有。Read/DirFSとos.Root.FSのsymlink境界は同一視せず、SpaceReadersSymlinks、ReadSpacesSymlinkBoundaries、ReadSpacesInitialProjectSymlinkを保持。 |
| A15 | space_switch_integration_test.go の TestSwitchSpaceNamesAndProtectedData の14行を、teamへの同一target書換え＋protected data保全、予約語listの許可、raw helpの拒否の3例へ縮小。 | Unicode・prefix・truncate等のnormalizeSlugはspace_create_test.goの既存tableへ残す。raw helpの大文字小文字の違いは公開switchの入力guardに固有なのでそのassertを保持する。 |
| A28 | space_test.go の TestListSpacesUnique のcustom fsが同じentryを繰返す2行を削除し、実default directoryとの重複抑止を維持。TestActiveSpaceFallbackのpermission/arbitraryをpartial data＋errorの1例、TestListSpacesReadDirErrorをpartial entries＋errorの1例、TestListSpacesStopsOnStatErrorのfirst/middle/lastをmiddle1例へ縮小。 | OSが返さない重複entriesをテストするためのfakeは専用なら削除。途中Stat失敗で先行rowが残り、後続が処理されないこととcauseを維持。別のerror伝播経路は統合しない。 |
| A37 | src/internal/projectroot/root_test.go の TestResolveAncestorFiles の各obstructionに対するexplicit=project行を削除し、Resolve("", project, false)の2例を残す。 | TestResolvePreservesErrorsのexplicit override1例、multiple installed拒否、filesystem errorをnot installedへ潰さないassertを保持。M5/C09がこのfileへ後から固有root検査を移すため、その移動を先取りしない。 |

~~~sh
go test -count=1 ./src/internal/workspace -run '^Test(ResolveRoot|ReadSpacesRootPrecedence|SwitchSpaceRootPrecedence|ReadSpacesRejectsRelativeRoot|ReadSpacesInvalidRootDoesNotOpen|ReadSpacesProjectOpenError|CreateSpaceRejectsRelativeRoot|CreateSpaceProjectOpenError|SwitchSpaceRejectsRelativeRoot|SwitchSpaceProjectOpenError|ActiveSpace|ActiveSpaceFallback|ListSpacesDefault|ListSpacesSelection|ListSpacesUnique|ListSpacesOrder|ListSpacesReadDirError|ListSpacesStopsOnStatError)$'
go test -tags=integration -count=1 ./src/internal/workspace -run '^Test(CreateSpaceRootPriority|SpaceReadersFilesystem|ReadSpacesFallbacks|SpaceReadersSymlinks|ReadSpacesSymlinkBoundaries|ReadSpacesInitialProjectSymlink|ReadSpacesProjectOpenIntegration|CreateSpaceRequiresExistingProject|SwitchSpaceNamesAndProtectedData|SwitchSpaceSavesNormalizedName)$'
go test -count=1 ./src/internal/projectroot -run '^Test(ResolveAncestorFiles|ResolvePreservesErrors)$'
~~~

## S5: 共通のworkspace引数解析と公開routeの代表（A16、A17、A24後半）

| 候補 | 正確な処置 | 生存する保証・削除理由 |
|---|---|---|
| A16 | src/internal/cli/space_list_test.go の TestRunSpaceListFlagPositions、space_test.go の TestRunSpaceCreateProjectDirPositions、space_switch_test.go の TestRunSpaceSwitchProjectDirPositionsの大きな表を撤去。 | 新規workspace_arguments_test.goのTestWorkspaceArgumentsへ共通workspaceArgumentsの正常表を集約する。first/middle/end、split/equals、JSONとproject-dirの前後、allowJSON=false時の通常flagsを計8例以内。TestRunSpaceCreate / TestRunSpaceSwitch / TestRunSpaceListJSON / TestRunSpaceBareAliasに各1個のexplicitDir転送を残す。listの二段階parse後もproject-dirが失われないよう実Runで検査する。 |
| A17 | space_test.go のTestRunSpaceCreateInvalidArguments / TestRunSpaceCreateDashProjectDir、space_switch_test.goのTestRunSpaceSwitchInvalidArguments / TestRunSpaceSwitchDashPath、space_list_test.goのTestRunSpaceListInvalidFlags / TestRunSpaceListProjectDirLiteralを共通部分だけTestWorkspaceArgumentsへ移す。 | 共通表のinvalidはunknown option、project-dirのmissing/empty/duplicate、dashで始まるsplit値、JSON duplicate、allowJSON=falseのJSON、equalsによるdash literalの正常対照を各1行。json=true/false/emptyは同じunknown-option枝の代表1行。公開Runにはroute別の名前数、raw help、list/bare分類、JSON可否、syntax failure時callback未呼出を各1例残す。name数と分類は共有parserの責任でないため消さない。 |
| A24（後半） | command_test.go のTestHookCommandDispatchから、Runの後に同じ引数をParseCommandで再検査するblockを削除。 | Runがcallbackへ渡すCommand/Action/ProjectDir等とexitのassertをそのまま維持する。廃止hook例はS1のTestFiveCLIContractが所有。 |

公開routeの重複表を削除する前に、共通parserを直接検査する新tableを実行する。既存package cli_testからprivate関数へ到達するためにexportしない。

~~~sh
go test -count=1 ./src/internal/cli -run '^Test(WorkspaceArguments|RunSpaceCreate|RunSpaceSwitch|RunSpaceListJSON|RunSpaceBareAlias|RunSpaceCreateInvalidArguments|RunSpaceSwitchInvalidArguments|RunSpaceSwitchInvalidRawName|RunSpaceListExtraArguments|RunSpaceListDuplicateJSON|RunSpaceListUnknownSubcommands|HookCommandDispatch|FiveCLIContract)$'
~~~

変更前はWorkspaceArguments未作成でも他の生存名が実行される。新table単独でも実行して件数ゼロでないことを確認し、その後に重複表を削る。

## S6: helpと出力失敗・準備順序（A18–A23）

| 候補 | 正確な処置 | 生存する保証・削除理由 |
|---|---|---|
| A18 | src/internal/cli/cli_test.go のwantHelp全文literalを削除。TestRun_Helpは実際の先頭route出力を基準に別名routeの同じ出力・exit 0・callback未呼出を検査する。TestRun_UnknownArgumentsの全文一致をunknown prefix・元引数・help案内へ縮小。space_test.go / space_switch_test.go / space_list_test.go の3つのTestRunHelpIncludesSpace*を統合して削除。execution_plan_review_test.goのroot用語列を除く。 | root helpの公開操作群が1回は案内される短い独立assertと、各操作helpの到達性は維持する。ヘルプ全体の写しを期待値にしない。別名出力の同一性とcallback未呼出が固有保証。 |
| A19 | 下記のhelp辞書整理を行う。dispatch・required flags・実JSON例・修復command・安全な操作順は残す。語彙だけの列を削除する。 | 実際に利用できるcommandと利用条件を残し、説明文の日本語言換えで壊れる検査を減らす。具体的な生存範囲は直後の補足。 |
| A20 | src/internal/cli/configure_help_test.go のTestConfigureHelpを削除（JSONをmapへ再解釈してschema/初期statusを再検査する表と用語列）。rule_skill_separation_test.go のTestRuleSkillSeparationHelpはcommands数の20下限を非zeroへ変更し、末尾4routeの用語mapを削除。 | src/cmd/aidlc/configure_help_integration_test.go のTestConfigureHelpExamplesで、help由来のUnitなし/ありJSONを実CLIへ投入する検査を維持し、M4 finalで明示実行する。TestRuleSkillSeparationHelpの配布skill内の実help commandがHelpで解決される全route検査は残す。 |
| A21 | cli_test.goのTestRun_HelpWriteErrorを共通writeStdoutの失敗1例へ縮小し、TestRun_VersionWriteErrorとcheck_help_test.goのTestCheckHelpOutputFailureを削除。専用writerだけ無参照なら削除。 | help/version/check-helpの成功dispatchとstdout内容は各生存testで維持する。workspace route独自writerのshort-writeやJSON error処理は別経路なのでこの削除へ含めない。 |
| A22 | space_test.goのTestRunSpaceCreateStderrFailureとspace_list_test.goのTestRunSpaceListStderrFailureを削除し、space_switch_test.goのTestRunSpaceSwitchOutputFailures内のstderr unavailableを共通writeCommandErrorの1代表にする。同表のboth/short-stderr重複を削除。ListPartialStdoutFailureの固定prefix「Spa」「{\"a」とSwitchのpartial prefix文字列assertを削除。 | 各公開routeのcallback error→exit・JSON error、stdoutのwrite failure、short stdout writeを保持。fakeが何byte保存するかの自己検査を除き、CLIが成功扱いしないことを検査する。 |
| A23 | space_test.goのTestRunSpaceOutputPreparation、space_switch_test.goのTestRunSpaceSwitchOutputPreparation、space_list_test.goのTestRunSpaceListOutputPreparationからflag位置・alias/formatの掛合せと全event列一致を削る。 | 各routeのsuccess1、syntax-before-callback1、callback failure1を残す。PrepareOutputがcallback/最初の出力より前、必要回数1である部分順序だけを検査。root help/versionは準備を呼ばない。root unknownのstderr,stderrという内部write回数を固定しない。 |

A19のfile・関数別補足:

- check_help_test.go: TestCheckHelpStageBoundariesの6段階×語彙表を削除。TestCheckHelpEvidenceContractの「全段階の入出力」「終了時の共通条件」の単語列を削り、「実測結果と文書の区別」は文書出力にcode/test/commitを登録しないこととSensorが真正性を証明しない案内に絞る。TestCheckHelpDocumentContractsは共通metadataの語彙列を除くが、各文書の実pathと必須H2の抽出・一致は維持する。ここには実help例を使う代替が確認できないため、書かれていない必須H2を案内して利用者を止める回帰を引き続き検出する。extraの一般語列は除き、ArchitectureのMermaid必要条件、Rule/ADRに固定H2がない条件、intent_idが必要な文書の条件は残す。
- 同fileのTestCheckHelpDocumentVersionsAndOptionalOutputsは単語列を、受入済みpath/hash・planning省略時・共有現在版・既定出力を一律要求しない既存案内の少数assertへ縮小。TestCheckHelpWorkflowAndProcedureは「read-only」「test commandは実行しない」「passだけでは完了しない」、既存の開始Sensor→begin→終了Sensor→独立レビュー→成果承認→finish、plan-approvalと成果approvalの区別、実procedure commandと再取得の案内を維持。TestCheckHelpPreservesBeginHelpはbegin実commandと一般作業前begin・開始入力版の再試行条件を残し、check専用説明文の否定一致を除く。TestCheckHelpForms / TestCheckHelpRejectsExecutionArgumentsは維持する。
- assignment_test.goのTestAssignmentContractは正常dispatchとrequired flags拒否を残し、registry_epoch単語の全route反復を除く。execution_plan_test.goのTestExecutionPlanCLIGrammarは正常/異常grammarとhelp存在を残し「実行」という単語条件を除く。
- memory_help_test.goのTestMemoryHelpはOKF create/updateの必要flagsとparser errorからhelpへ戻れる案内を残し、draft/stable/deprecated、日時説明語、stage/status列挙だけの表を削る。TestMemoryHelpUnitConfirmVerificationはunit/session/root/run_id/verification_sha256の組と登録run・現在内容の照合案内を残し「64桁」単独辞書を除く。
- help_codekb_test.goのTestCodeKBHelpは正しい実command/pathと旧誤path否定を維持（単純用語表ではない）。help_work_log_test.goのTestOKFWorkLogHelpは保存path、okf search/showと--intent-idを残しstate/revision/generated.atの単語列を除く。
- git_independent_test.goのTestGitIndependentInstallReviewHelpは別session必須・同root可・別rootで内容SHA一致・同一review identityという利用条件を維持する。relocation_test.goのTestRelocationCLIRootHelpはA18へ統合して削除。TestRelocationCLIはunit reassign grammarとprevious_run_stopped=true/reason/run_idという安全な再割当て条件を残し、status/「再試行」単語だけのassertを除く。

~~~sh
go test -count=1 ./src/internal/cli -run '^Test(Run_Help|Run_HelpWriteError|Run_Version|Run_UnknownArguments|CheckHelpEvidenceContract|CheckHelpDocumentContracts|CheckHelpDocumentVersionsAndOptionalOutputs|CheckHelpWorkflowAndProcedure|CheckHelpForms|CheckHelpRejectsExecutionArguments|CheckHelpPreservesBeginHelp|AssignmentContract|ExecutionPlanCLIGrammar|MemoryHelp|MemoryHelpUnitConfirmVerification|CodeKBHelp|OKFWorkLogHelp|GitIndependentInstallReviewHelp|RelocationCLI|RuleSkillSeparationHelp|RunSpaceCreateFailureJSON|RunSpaceCreateStdoutFailure|RunSpaceCreateShortStdoutWrite|RunSpaceSwitchShortStdoutWrite|RunSpaceSwitchOutputFailures|RunSpaceListReadFailure|RunSpaceListPartialStdoutFailure|RunSpaceListShortStdoutWrite|RunSpaceOutputPreparation|RunSpaceSwitchOutputPreparation|RunSpaceListOutputPreparation)$'
~~~

実ConfigureHelpExamplesは毎sliceのbuildを増やさずfinalへ集約する。削除前に同testが当該JSONを本当に投入することはコードで確認済み。親がM3後baselineでも入口・内容の変更がないことを照合する。

## S7: Unicode・lock/cursor・close・buildinfo（A25–A27、A29–A31）

| 候補 | 正確な処置 | 生存する保証・削除理由 |
|---|---|---|
| A25 | src/internal/workspace/workspace_lock_test.goのTestNormalizeWorkspaceLockCanonicalMatchesECMAScriptDefaultLower全12行を削除。 | src/internal/pathnorm/unicode_test.goのTestECMAScriptDefaultLowerFixedWindowsVectorsとTestNormalizeForPlatformOnlyWindowsFoldsCaseが同じ変換を所有。workspace側の固定lock filename2例を維持する。 |
| A26 | workspace_lock_test.goのTestECMAScriptDefaultLowerKeepsUnicode15IdentityRunes / TestIsCasedUsesUnicode15Overlay / TestIsCaseIgnorableUsesUnicode15Overlayの55/107/88件という配列長assertと重複U+A7F1末尾assertを削除。TestUnicodeVersionIsAuditedForBunWindowsCompatibilityを削除。TestECMAScriptDefaultLowerUsesUnicode15FinalSigmaContextの7入出力をpathnorm/unicode_test.goへ同名で移す。 | 3つのoverlay範囲表本体はM4では理由付き維持。既存pathnormの固定出力13例だけではoverlayの全域を置換できず、独立資料に基づくrange端/外のoracleはこの計画で確認できていない。コピーしたrangeから端/外を機械生成して「独立」と呼ばない。範囲表を全面削除する未承認選択はしない。固定Unicode15の意味を保ち、Goのunicode.Version文字列そのものはgateにしない。必要なaliases/appendRuneRangeは残るcallerがあるため維持。 |
| A27 | workspace_lock_test.goのTestWorkspaceLockPathUsesCanonicalWorkspaceIdentity / TestWorkspaceLockPathFallsBackToLexicalAbsolutePathのtest内MD5期待値計算を除く。前者は別lexical pathをresolverで同一canonicalへ向けた結果の同一性、後者はresolver失敗時とlexical identityを返す成功時の同一性、temp directoryの親を検査する。 | canonical採用とfallbackというbranchを検査し、hash式はTestWorkspaceLockPathMatchesKnownWindowsUnicodeIdentityの.aidlc-audit-211f1998.lockとTestWorkspaceLockPathKeepsBunWindowsUnicode15Identityの.aidlc-audit-a3f33a77.lockを独立固定値として維持する。2固定例の途中のidentity文字列・手動digest再計算は削除し、最終filenameへ絞る。lock生成・競合・reap・tokenは変更しない。 |
| A29 | space_switch_test.goのTestReplaceSpaceCursorNonRegularからdevice/socketを削りdirectory/link/named pipeを残す。TestReplaceSpaceCursorFailuresのsteps期待列と全列一致を削除し、causeと禁止されたpublish・own temp cleanupを残す。 | TestReplaceSpaceCursorPreservesPermissionsのwrite後permission復元・close後rename、TestCompleteCursorNoReplacePublishesAfterClose、TestCompleteCursorNoReplaceFailuresのcause/linkされない失敗/own cleanup、実integration TestSwitchSpaceCursorFailurePreservesOldFileを維持。cursor_test.goの現Failuresは既にobservableなlinked/removedのboolであり、削る全呼出列がないため理由付き維持。全sequenceを消す際にcleanupを単に無assertにしない。 |
| A30 | space_read_integration_test.goのTestReadSpacesProjectClose、space_create_integration_test.goのTestCreateSpaceProjectClose、space_switch_integration_test.goのTestSwitchSpaceRootCloseFailuresから、注入Close後にcaptured rootをStatして標準os.Rootの閉鎖自体を再確認するassertを削除。 | close回数、close failureのcause/zero result、保存済みbytesを維持。TestSwitchSpaceSavesNormalizedNameのdefault closeを確認するassertは実Closeに対する他の観測がないため保持。 |
| A31 | src/internal/buildinfo/buildinfo_test.goのTestCurrent_DefaultsをInfo{Version: dev, Commit: unknown}とのstruct比較1つへ縮小。 | 開発buildの既定値という公開表示契約は残す。fieldごとの同じ比較を除き、release ldflagsの検査とは統合しない。 |

A26の理由付き保持は上位計画の条件付き削除をそのまま具体化した判断であり、保留gateではない。独立oracleの追加調査やUnicode契約の変更をwriterへ要求しない。

~~~sh
go test -count=1 ./src/internal/pathnorm -run '^Test(ECMAScriptDefaultLowerFixedWindowsVectors|NormalizeForPlatformOnlyWindowsFoldsCase|ECMAScriptDefaultLowerUsesUnicode15FinalSigmaContext)$'
go test -count=1 ./src/internal/workspace -run '^Test(WorkspaceLockPathUsesCanonicalWorkspaceIdentity|WorkspaceLockPathFallsBackToLexicalAbsolutePath|WorkspaceLockPathMatchesKnownWindowsUnicodeIdentity|WorkspaceLockPathKeepsBunWindowsUnicode15Identity|ECMAScriptDefaultLowerKeepsUnicode15IdentityRunes|IsCasedUsesUnicode15Overlay|IsCaseIgnorableUsesUnicode15Overlay|ReplaceSpaceCursorNonRegular|ReplaceSpaceCursorPreservesPermissions|ReplaceSpaceCursorShortWrite|ReplaceSpaceCursorFailures|CompleteCursorNoReplacePublishesAfterClose|CompleteCursorNoReplaceFailures)$'
go test -tags=integration -count=1 ./src/internal/workspace -run '^Test(ReadSpacesProjectClose|CreateSpaceProjectClose|SwitchSpaceRootCloseFailures|SwitchSpaceSavesNormalizedName|SwitchSpaceCursorFailurePreservesOldFile|SwitchSpaceCursorLinkBoundaries)$'
go test -count=1 ./src/internal/buildinfo -run '^TestCurrent_Defaults$'
~~~

FinalSigmaContextは移動前にはworkspace側で同名を実行し、移動後にはpathnorm側で実行する。新規Unicode oracleのふりをして期待値をproductionから計算しない。

## S8: OKF parser・usage window・Search所有権（A32–A34）

| 候補 | 正確な処置 | 生存する保証・削除理由 |
|---|---|---|
| A32 | src/internal/okfmemory/document_test.goのTestParseRejectsInvalidの4例をempty type拒否1例へ縮小。 | src/internal/okf/frontmatter_test.goのTestParseConceptInvalidが正本parserのinvalid表を維持。okfmemory側は正本parserのinvalid type拒否がwrapperへ伝わることを検査し、TestParseRoundTripのunknown metadataとbody保持は維持。 |
| A33 | src/internal/okf/frontmatter_test.goのTestParseConceptUsageWindowのtop level8×source override8を、top level8＋source override valid without usage count / not mappingの2行へ縮小。 | 同じusage window validatorの形式/offset/from/to拒否をtop levelで所有。source overrideは正常と拒否各1行で配線を確認し、同じ8行を掛け合わせない。 |
| A34 | src/internal/okf/scan_test.goのTestScanBundleSelection末尾のTagsをmutatedへ変え再Scanするblockを削除。 | 同関数のselection・warning・path・UTF-16順序を維持。search_test.goのTestSearchLifecycleAndOwnershipが、結果を利用者が変えても次のSearchへ影響しない現実的なalias防止を引き続き検査する。毎回再parseするScanの所有権再検査を除く。 |

~~~sh
go test -count=1 ./src/internal/okfmemory -run '^Test(ParseRejectsInvalid|ParseRoundTrip)$'
go test -count=1 ./src/internal/okf -run '^Test(ParseConceptInvalid|ParseConceptUsageWindow|ScanBundleSelection|SearchLifecycleAndOwnership)$'
~~~

## 受入条件・review・final

全36候補に処置と生存保証を結果docへ記録する。A19の実path/H2/安全案内、A26のoverlay範囲本体、A29の既にobservableなcursor failure、A30のdefault close等は、理由付き維持も完了結果である。名前やcoverage目標のために全削除へ強制しない。

writer末尾で変更Go fileへgofmtを適用し、S1–S8の最終生存名のtargeted群とgit diff --checkを一度まとめて確認する。途中削除したMemoryBodyWrite / OKFSkillReadApprovalPending / ConfigureHelp等を末尾commandへ残さない。各regexについて期待するTest名が1件以上存在することを静的に確認し、必要な新test/移動testの単独実行ではゼロ件を成功扱いしない。

親はwork unit末尾の差分とtargeted群を1回確認し、独立reviewへ verification_mode=review を明示する。reviewではテスト削除後の固有assert、生存helper、前段拒否で通る表、実装コピーを再導入していないことを重点確認。修正があれば必要sliceのloopへ戻す。

blocking finding解消後、親が開始HEADと最終HEADを固定し、対象fileを変えないfinalを1回開始する。M4のfinalは次の範囲。

~~~sh
go test -count=1 -shuffle=on -coverprofile=/tmp/ai-dd-test-reduction-m4-coverage.out ./...
go test -count=1 -race -shuffle=on ./...
go vet ./...
go mod tidy -diff
gofmt -l src
git diff --check
go test -tags=integration -count=1 ./src/internal/workspace ./src/internal/okf
go test -tags=integration -count=1 ./src/cmd/aidlc -run '^Test(FlowJourney|GitIndependentJourney|ConfigureHelpExamples)$'
~~~

gofmt -lは空を要求し、finalでは適用しない。M3でstress等のtagへ移されたtestはそのmilestoneの実行責任を引き継ぎ、M4で無条件に通常suiteへ戻さない。M4は配布build/assetを変更しないため30binaryのローカル再生成や新しいlive実験を追加しない。対象PRで起動する同headのGitHub checksを親が全て確認し、skipやdiscoveryを実candidate/native成功と呼ばない。

親が計画と候補別結果をdocs/designへ保存し、承認RAMとdocs/ram/README.mdの現状を更新する。歴史的監査の記載や過去RED/GREENは改変しない。現在の実行手順・CIに削除Test名があれば同PRで生存入口へ置換する（M5/M6の重複job整理は先取りしない）。Issueを紐づけた日本語PRで既存方式のmerge後、main反映とIssue closeを確認する。

## 保全とrollback

共有treeの未commit変更は開始時に親が記録し、writer以外はrepoを編集しない。専用fileを削る直前にhelperのcallerを検索し、タグや別OS側にcallerが残るhelperは維持する。固有assertの移転先を安全に確定できない場合は該当assertを元の場所に保持し、他の確定済み削減を完了して親へ理由を返す。新契約やmodule追加によって解決しない。

製品の永続dataや利用プロジェクトの移行は発生しない。問題のあるM4を取り消す場合は当該PRのrevertを別PRとして行い、他者・後続PRの変更をresetしない。PR後の対象変更はfinal証拠をstaleにするため、必要なloop/review後にfinalを更新する。

## この計画の確認範囲

AGENTS.md、implementation-planning/golang-how-to/golang-testing、docs/agent-workflow.md、docs/tdd-handoff.md、承認RAM、上位milestones、既存監査を読んだ。監査を全file再実施せず、旧reader/current Rule、app→okfapp実転送とCRUD/session、pending hook、共通workspace parser、条件付きhelp、Unicode/lock、cursor failure、usage window/Scan/rootの必要箇所を追加確認した。plannerはrepo編集・test・build・依存追加・GitHub操作を行っていない。本fileだけが今回の成果物である。

## 実装時に一意に確定した詳細

A07のcreate/updateは現行Serviceの `Metadata.IntentID` を保存し、searchは `CommandRequest.IntentID` を使う。新転送例の初回誤期待をこの入力へ修復し、ALREADY_GREENを確認後に旧CRUDを削除する。製品の転送対象は変更しない。A36の `err=nil` 後の死んだblockは実fileで3箇所あり、同じ根拠で3箇所を除去した。実施結果は[M4結果](test-reduction-m4-result.md)を参照。

## 独立reviewの修復

既存WorkspaceArgumentsの正常1行を前後空白付きpathへ置換し、入力bytesを無加工で返す保証を保つ。無参照checkHelpFailingWriterと専用errors importを削除する。A32の代表をfrontmatterなしからempty typeへ置換する。前者はwrapperの終端検査だけでも拒否できるため、下位ParseConceptの拒否を必要とする旧empty_typeを使う。件数・製品仕様は変更しない。
