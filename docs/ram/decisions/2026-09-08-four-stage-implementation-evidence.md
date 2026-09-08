# 四段階製品切替の実装証拠

Issue #130、work_unit_id `four-stage-product-cutover`、verification_mode `loop`。
開始HEAD `68430a80ea6edff87f651ddeabcf7cbfc7aec3aa`、branch `codex/four-stage-workflow`。
[承認済み計画](../../design/four-stage-workflow-implementation-plan.md)と
[work unit](../../design/four-stage-workflow-work-unit.md)の単独writerとして実装した。
実装前に [詳細契約](../../design/four-stage-workflow-contract.md)へflag/schemaを具体化した。

## 実装した結果

- Git共有のIntent stateと期待revisionによる原子的な比較保存。
- 現在の成果物・コード版・ADR要否・Unit依存を検査するSensor。
- 別root/sessionのreview割当、実対象hashの一致、pass/fail受理とstale拒否。
- discovery/planning/tdd/integration、待機・中断・再開・戻し・完了。
- Unitの別worktree、run identity、依存、scope重複、成果commit、統合、再確認。
- 新CLIとhook。毎操作のKDR記録義務を廃止し、現在Rule全文と実行中toolの境界を維持。
- 配布skill/Rule/reviewer、ADRの配置とOKF metadata保持、共通Space基盤。
- 新fresh非live/live harness、CI切替と旧専用依存の除去。

旧製品の削除範囲は [除去記録](../../design/four-stage-removed-product.md)に記録した。
旧利用dataの削除・移行はしない。AGENTSのユーザー差分、未追跡参照、過去RAMは保全した。

## TDD

各項目のテストはproduction実装前に実行可能なassertionで失敗を確認した。
compile-only scaffoldには型・signature・空返値だけを置いた。

| 項目 | runnable RED | GREEN |
| --- | --- | --- |
| S1 | TestFlowStoreCreateCAS/RejectsCorruptAndIsolates/FailurePreservesState/Names。空state、入力受理、保存失敗の隠蔽。WireSchemaは非canonical key。 | 同prefix全件 exit 0 |
| S2 | TestFlowSensorDiscovery/UnitGraph。空gate。DirectImplementationは検証なしpass。KnowledgePreservesOKFTypeはDesign拒否。RejectsInventedIntegratedCommitは架空commit受理。 | 同prefix全件 exit 0 |
| S3 | TestFlowReviewIdentityTarget/FailRecorded。同一担当受理、結果未保存。 | 同prefix全件 exit 0 |
| S4 | TestFlowTransitionGatesAndStages/WaitPauseResumeReopen。reviewなし遷移、理由なし操作受理。 | 同prefix全件 exit 0 |
| S5 | TestFlowUnitClaimDependencyAndOverlap/ResultIntegrationAndResume。依存前開始、別run結果受理。 | 同prefix全件 exit 0 |
| S6 | TestFlowGrammar/IntentCommand/StopNoRecordObligation。旧文法、新state未接続、不要なKDR Stop。HookSelectionRulesAndRecoveryはKDR読込へ誤接続。ConfigurePreservesActiveAssignmentは実行中Unitの除去を受理。 | minimal/cli TestFlow全件 exit 0 |
| S7 | TestFlowInstallAssets/SpaceADR。ADR不在、旧配布。 | install/workspace/okfmemory TestFlow全件 exit 0 |
| S8 | TestFlowCommandPublicCutover。旧help/旧入口。EvidenceVerifierは不十分な観測を受理。後続回帰でCLI操作証拠なしも拒否。 | cmd TestFlowCommand全件 exit 0 |

`TestFlowADRMetadataSearch` の初回fixtureに不正な `status: accepted` があり、
`INVALID_TEST_FIXTURE` として停止した。親の明示継続指示でparserの許容値 `stable` を確認して修復。
この初回を製品REDに数えない。修復後は `ALREADY_GREEN`。
`TestFlowSensorRequiredADR` と `TestFlowUnitTwoParallelThenDependent` の追加coverageも `ALREADY_GREEN`。
既存の共通testは削除せず、旧help表記と旧KDR配布treeの契約直結期待のみ修正した。

実行ログは `/tmp/ai-dd-flow-s1-red.log`〜`s8-red.log`、各 `sN-final.log`、
`/tmp/ai-dd-flow-affected-final.log`。S1〜S8最終commandはwork unit記載のexact commandで全てexit 0。
許可された影響package flow/filestore/minimal/cli/install/workspace/okfmemoryのpackage testもexit 0。
filestore自体には独立test fileがなく、flowとokfmemoryの保存・path境界testで検証する。
`go test -tags=integration -count=1 ./src/cmd/aidlc -run '^TestFlowCommand'` もexit 0。
これはharnessのcompile確認を含むが、一周の実行証拠ではない。gofmtと `git diff --check` 成功。

## 親finalで行う受入

```sh
go test -tags=integration -count=1 ./src/cmd/aidlc -run '^TestFlowJourney$'
AIDLC_FLOW_LIVE=1 go test -tags=integration -v -count=1 -timeout=35m ./src/cmd/aidlc -run '^TestFlowJourneyLive$'
```

非liveはfresh install、名前/ID、config、4段階、review fail、実Go assertion RED/GREEN、
Knowledge、同一IDの別session再開と完了をCLIで通す。独立AIの実証とは扱わない。
ADR有/不要と2並列Unit→依存Unitはdeterministic flow testで対応する。

liveは実調整役が配置skill/hookに従いCLI state/割当/review/advanceを行う。
別workerの実test RED/GREEN、同じtest本文hash、実編集source hashとcommit bytes、
実稼働の重なり、別worktree/session、依存Unitの後続開始を確認する。
別read-only reviewerの固定対象報告、fail→修正→再review、stale拒否を観測する。
別会話で同じIntentをbind・再開する。raw hook、モデルJSONL/stdout/stderr、host job、
実test出力は表示した一時ディレクトリへ保持し、製品state/auditには使わない。

固定Codex 0.153.4、gpt-6-astra/medium、workspace-write、approval=neverを維持する。
HOME/CODEX_HOME/認証を変更しない。固定sandboxでGitを書けない制約は、親確認のうえ
test hostが専用fixtureのworktree作成・実worker bytesのcommit・統合を担う形で適応した。
これはモデル自身のGit操作成功の証拠ではなく、製品Goの起動/Git管理機能でもない。

loopではfresh一周、live、全project/race/vet/cross-buildを実行していない。
これらは未観測であり、親の独立reviewとfinalが必要。
