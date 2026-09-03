# Current directive resolver の実装計画

- 承認日: 2026-09-03（Asia/Tokyo）
- 状態: Accepted
- 対象Issue: [#65](https://github.com/sori883/ai-dd/issues/65)
- 固定base: `d7682295191821011d908ea52a1be1dd6db2abb8`
- 検証mode: `loop`

## 初心者向けの目的

保存済みworkflowのstateと、利用可能なStageのcatalogを別々に読むだけでは、後続処理が「どのStageをどう扱うか」を
毎回判断しなければならない。このIssueでは、両方のtyped snapshotを受け取り、次の1件の指示（directive）へ変換する
read-onlyな内部APIを追加する。

- 実行中で、current Stageが実行可能なら、そのStageとcatalog metadataを`run-stage`として返す。
- 全Stageが終端条件を満たしていれば、Stageを持たない`workflow-complete`を返す。
- stateとcatalogの不一致や曖昧なlive markerは、黙って補正せず分類済みerrorとして返す。

このAPIはfilesystem、state更新、audit、Stage本文実行を行わない。CLI接続やrecomposeも後続Issueの責務である。

## 実装範囲と所有権

この実装で編集してよいファイルは次だけとする。

- 新規 `src/internal/orchestrator/directive.go`
- 新規 `src/internal/orchestrator/directive_test.go`
- 更新 `docs/architecture.md`
- 更新 `docs/development.md`
- 新規 `docs/ram/research/2026-09-03-current-directive-contracts.md`
- 新規 `docs/ram/decisions/2026-09-03-current-directive-resolver-plan.md`
- 更新 `docs/ram/README.md`

`src/internal/state`、`src/internal/graph`、`src/internal/stageplan`、CLI、`go.mod`は変更しない。Go標準ライブラリだけを
使い、外部module・toolは追加しない。

## API契約

```go
func ResolveDirective(state.State, graph.Snapshot) (Directive, error)
```

`DirectiveKind`は次の3値だけを持つ。

- `DirectiveKindUnknown = ""`: zero value。error時にも使う。
- `DirectiveKindRunStage = "run-stage"`: `Stage()`で実行対象を返す。
- `DirectiveKindWorkflowComplete = "workflow-complete"`: `Stage()`は存在しない。

`Directive`のfieldは非公開にし、`Kind()`と`Stage() (graph.Stage, bool)`だけを公開する。Stageを返すときは、
`SupportAgents`、`Scopes`、`Produces`、`OptionalProduces`、`Consumes`、`RequiresStages`をdeep copyする。
callerが返却値を変更してもdirective内部やgraph catalogへ影響してはならない。

## runningの判定

`WorkflowStatus == Running`では、stateの`Current Stage` rowが唯一の`[-] EXECUTE`であり、他にlive marker（`[-]`、`[?]`、
`[R]`）がないことを確認する。enabled graphに同じslugのStageがあり、graphのphaseが次のcanonical lowercaseで、stateの
uppercase Lifecycle Phaseへ一意に対応する場合だけ`run-stage`を返す。

- current `[ ]`、`[?]`、`[R]`: `ErrUnsupportedState`
- current `[x]`、`[S]`、current `SKIP`、複数live marker、zero/inconsistent running: `ErrInvalidState`
- graph slug不在、disabled、phase不一致または未知・非canonical phase: `ErrStateCatalogMismatch`

`Next Stage`、Summary、scope grid、Stage Planの再計算はrunningのrouting根拠にしない。ただしcurrent Stage rowの保存済み
`EXECUTE` suffixは実行可否を表すauthorityとして検証する。

## Completedの判定

`WorkflowStatus == Completed`では、`Next Stage == "none"`、`Summary.InProgress == "none"`、live markerなしを要求する。
各Stage rowは、`EXECUTE`なら`[x]`または`[S]`、`SKIP`なら`[ ]`または`[S]`だけを受け入れる。 `Current Stage`は文字列
`none`、またはstate内のsettled row（`[x]` / `[S]`）のいずれかでなければならない。成功時はcatalogを参照せず、
`workflow-complete`を返す。

## Errorと純粋性

`ErrInvalidState`、`ErrUnsupportedState`、`ErrStateCatalogMismatch`をsentinelとして公開し、context付きerrorへ`%w`で
wrapする。sentinelは`errors.Is`で判定可能にし、error textはlowercaseにする。error時は必ずzero Directiveを返す。

resolverはFS、clock、global cache、scope lookup、write、lock、auditを使わない。入力のstateとsnapshotを変更せず、毎回同じ
入力から同じ値を返す。

## 本家AI-DLCとの意図的な差分

比較対象はrepositoryに固定した本家AI-DLC `2.6.123`の、`aidlc-orchestrate.ts` Branch 10と
`aidlc-lib.ts`の`effectivePlanAction` / `parseStateStageSuffixes`を確認した範囲である。最新upstream全体との一致は主張しない。

| 本家の挙動 | 採用する挙動 | 変更理由 | 利用者・互換性への影響 |
| --- | --- | --- | --- |
| state suffixがない場合、`effectivePlanAction`がscope gridへfallbackする | parsed state suffixだけをauthorityとし、scope grid・Stage Planからfallbackしない | 承認済みstateの計画を暗黙に別計画へ置換せず、in-flight recomposeの決定を保持する | suffixの不整合をSKIPへ自動補正せず、上位callerに修復を要求する |
| 本家の`done`経路はscopeの次Stage、runtime/auditなどを含むworkflow全体の処理である | state rowのsettled条件だけをread-timeに確認し、catalog不要で`workflow-complete`を返す | internal join APIへ副作用とruntime証跡を持ち込まない | audit確認、state更新、実行記録は別の上位処理が担う。完了判定の範囲はこのAPIのstate契約に限定される |

上記はGoの型や内部構造の差ではなく、承認済みのfail-closed設計差分である。根拠と確認範囲は
[current directive resolverの参照契約](../research/2026-09-03-current-directive-contracts.md)に記録する。

## TDD slicesと検証

`verification_mode=loop`で、次のobservable behaviorを1 sliceずつRED→最小GREEN→green上のrefactorで実装する。
テストfixtureは`state.Parse`と`graph.Load(fstest.MapFS)`を通し、unexported fieldへ依存しない。

1. `TestResolveDirectiveRunningStage`: current `[-] EXECUTE`のStage metadataを返し、summary/next/scope gridをroutingに使わない。
2. `TestResolveDirectiveRejectsInvalidRunningState`: settled、SKIP、zero/inconsistent、複数live markerをinvalidとして拒否する。
3. `TestResolveDirectiveReportsUnsupportedCurrentState`: current `[ ]`、`[?]`、`[R]`をunsupportedとして分類する。
4. `TestResolveDirectiveRejectsStateCatalogMismatch`: enabled slugとphaseの不一致をcatalog mismatchとして分類する。
5. `TestResolveDirectiveWorkflowComplete`: terminalの2形とaction別checkbox条件を確認し、catalogなしでも完了を返す。
6. `TestDirectiveStageOwnership`:全nested sliceのdefensive copyとzero Directiveを確認する。

loopで許可するcommandは次だけとする。

```sh
go test -count=1 -run '^TestResolveDirective' ./src/internal/orchestrator
go test -count=1 -run '^TestDirective' ./src/internal/orchestrator
go test -count=1 ./src/internal/orchestrator
```

変更Go fileへの`gofmt`はloopの最後に適用し、affected targeted testを再実行する。全package、integration全体、race、vet、lint、
cross compile、配布E2Eはこの実装担当のloopでは実行せず、親agentのfinal gateへ集約する。

## 受け入れ条件

- API、kind、sentinel、zero/error、deep-copyが上記契約どおりである。
- runningとcompletedの分類がstateの保存値とenabled graph metadataだけを根拠に決定される。
- completed判定がcatalog、FS、clock、scope lookup、write、lock、auditへ依存しない。
- Issueで指定したTDD test群とtargeted commandがfreshに成功する。
- architecture、development、RAM research、decision、README索引から責務・境界・検証方法を初心者が追跡できる。
