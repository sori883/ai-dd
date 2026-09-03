# Current directive resolver の参照契約

- 調査日: 2026-09-03（Asia/Tokyo）
- 状態: Current（このrepositoryに固定された本家AI-DLC `2.6.123`の確認範囲）
- 対象Issue: [#65](https://github.com/sori883/ai-dd/issues/65)
- 固定base: `d7682295191821011d908ea52a1be1dd6db2abb8`

## 背景と確認範囲

現在のworkflow位置を、保存済み`state.State`とenabledな`graph.Snapshot`から、後続処理が扱いやすい1件のtyped
directiveへまとめる契約を調べた。これはStage本文を実行したり、stateを書き換えたりする処理ではなく、現在の
Stageを安全に識別するread-onlyな境界である。

参照した本家はrepository内に固定された配布snapshotだけである。最新upstream全体との一致は主張しない。確認範囲は
現在Stageの読み取り、Stage suffixのplan action、enabled graphのStage metadata、workflow完了判定に必要な箇所へ限定した。

| 根拠 | 固定snapshotで確認した範囲 |
| --- | --- |
| `docs/実装_aidlc-workflows/core/tools/aidlc-version.ts:1-4`、`docs/実装_aidlc-workflows/CHANGELOG.md:1-6` | 本家versionが`2.6.123`であることと、同versionのCHANGELOG見出し |
| `docs/実装_aidlc-workflows/core/tools/aidlc-orchestrate.ts:3738`、`:4559-4650` | `handleNext`のBranch 10が`Current Stage`、checkbox、plan action、次のin-scope Stageを使い、残りがなければ完了directiveを出す経路 |
| `docs/実装_aidlc-workflows/core/tools/aidlc-lib.ts:21896-21935` | `effectivePlanAction`がstate suffixをscope gridより優先し、suffix parserが`EXECUTE` / `SKIP`をStage slugごとに読む処理 |
| `docs/実装_aidlc-workflows/core/tools/aidlc-lib.ts:23267-23270` | 本家のstate schema versionが`8`であること |

本家の`handleNext`は、実際のdirective生成に加えてscope解決、runtime/audit、各種特殊経路も持つ。この調査ではそれらを
Goの内部resolverへ持ち込まず、Issue #65が明示したstateとgraphのjoinだけを採用対象にした。

## 本家で確認したroutingとsuffix authority

本家の通常経路では、`Current Stage`を起点にcheckbox状態を読み、in-flightなら同じStageを再度run-stageとして扱う。
currentがsettledなら、`nextInScopeStage`で後続の`EXECUTE` Stageを探し、見つからなければworkflow完了となる。

plan actionは`effectivePlanAction`が解決する。state documentに同じStageのsuffixがあればそれを使い、なければscope gridへ
fallbackする。つまり、本家では保存済みstateの`EXECUTE` / `SKIP` suffixがscope gridより強いauthorityであり、recompose後の
state overrideもruntimeへ反映される。

Goのstate readerはsuffix先頭の`EXECUTE` / `SKIP`を`StageProgress.PlanAction`へ変換し、raw suffixも保持する。Directive
resolverでは、runningのcurrent rowが`EXECUTE`であることと、terminalの全rowのaction/checkbox組合せをこの値から判定する。
scope gridやStage Planを再計算してfallbackすることはしない。

## terminalの2形

Issue #65で受け入れるworkflow-completeは、次の2つのCurrent Stage表現を許可する。どちらも`Next Stage`とSummaryの
`In Progress`が文字列`none`で、live markerがなく、全Stage rowがactionに応じてsettledである必要がある。

1. `Current Stage: none`。全`EXECUTE` rowは`[x]`または`[S]`、全`SKIP` rowは`[ ]`または`[S]`である。
2. `Current Stage: <slug>`。そのslugのstate rowがsettled（`[x]`または`[S]`）で、上記の全row条件も満たす。

terminal判定はcatalogのmembershipやphaseを必要としない。保存済みstateが上記の終端条件を満たせば、graph snapshotが
空でもworkflow-completeを返せる。

## Goで固定する観測契約

次のAPIを`src/internal/orchestrator`へ追加する。

```go
func ResolveDirective(state.State, graph.Snapshot) (Directive, error)
```

`Directive`のfieldは非公開で、`Kind()`は`"run-stage"`、`"workflow-complete"`、またはzeroの`""`を返す。
run-stageだけ`Stage() (graph.Stage, true)`でenabled graphのmetadataを返し、workflow-completeとerrorでは
`Stage()`がzeroと`false`になる。返却Stageの`SupportAgents`、`Scopes`、`Produces`、`OptionalProduces`、`Consumes`、
`RequiresStages`はdeep copyする。

runningは、current rowが唯一の`[-] EXECUTE`で、他に`[-]`・`[?]`・`[R]`がなく、同slugのenabled graph Stageが存在し、
graph phaseのcanonical lowercase（`initialization`、`ideation`、`inception`、`construction`、`operation`）をstateの
uppercase Lifecycle Phaseへ正確に対応できる場合だけ成功する。`Next Stage`、Summary、scope grid、Stage Planの再計算は
routing根拠にしない。

currentが`[ ]`、`[?]`、`[R]`なら`ErrUnsupportedState`、currentが`[x]`、`[S]`、`SKIP`、live markerの重複、zeroまたは
不整合なrunningなら`ErrInvalidState`、graph slugまたはphaseの不一致なら`ErrStateCatalogMismatch`を返す。sentinelは
`errors.Is`で判定でき、error textはlowercaseで、どのerrorでもDirectiveはzeroである。

resolverはfilesystem、clock、global cache、scope lookup、write、lock、auditを扱わない。入力snapshotと返却Stageを変更しない
純粋なread-only関数とする。

## 本家AI-DLCとの意図的な差分

比較対象は上記の固定snapshot `2.6.123`の確認範囲だけであり、最新upstreamとの差分は未確認である。次の2点はGoの型変換で
不可避な差分ではなく、Issue #65で承認されたfail-closedな設計判断である。

| 本家の挙動 | Goで採用する挙動 | 変更理由 | 利用者・互換性への影響 |
| --- | --- | --- | --- |
| state suffixがなければ`effectivePlanAction`がscope gridへfallbackする | parsed stateのsuffixをrouting authorityとし、resolverはscope gridやStage Planへfallbackしない | 承認済みstateとruntimeで計画が食い違う暗黙補正を防ぎ、in-flight recomposeの決定を保持する | suffixが欠落・不整合のstateはSKIPへ黙って補正されず、errorまたはunsupportedとして修復が必要になる |
| `handleNext`の完了経路はscopeの後続Stage、runtime/auditなどを含むworkflow経路の一部として`done`を生成する | read-timeにstate rowのsettled条件を全件確認し、catalog不要で`workflow-complete`を返す | このinternal APIをstate/graph joinに限定し、副作用とruntime証跡を別責務へ残す | 完了判定はこのresolverが読めるstate rowの整合性に限定され、audit確認やstate更新は上位処理の責務になる |

この差分を採用する理由と実装計画は[Current directive resolverの実装計画](../decisions/2026-09-03-current-directive-resolver-plan.md)に記録する。

## 未確認事項と残余risk

- 最新upstream全体、Issue #65で列挙していない特殊directive、将来のstate versionとの互換性は確認していない。
- resolverはparsed `State`を入力にするため、Markdownの構文・encoding検証はstate readerの責務であり、ここでは再実装しない。
- graph snapshotは`graph.Load`済みの入力を想定する。enabled Stage除外やgraph構造検証はgraph packageの責務である。
- CLI接続、state advance、audit、recompose、Stage本文実行はこのIssueの対象外である。
