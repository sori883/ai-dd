# Intent開始時Stage Plan builderの実装計画

- 日付: 2026-09-03
- 状態: Accepted（Issue #57、ユーザー明示承認済み）
- GitHub Issue: [#57](https://github.com/sori883/ai-dd/issues/57)
- 承認: 2026-09-03、ユーザーがStage Plan builderの実装計画を明示承認
- base: `09f98b6a8179f04bd53a115f495c76321e56e331`（PR #56 merge commit）
- 関連: [参照契約](../research/2026-09-03-stage-plan-contracts.md)、
  [Stage catalog metadata](2026-09-03-stage-catalog-metadata-plan.md)

## 背景と目的

PR #56で`graph.Stage`が成果物と依存metadataを保持できるようになった。次の段階として、Intent
（1つの開発目的）の開始時に、scopeで実行・スキップするStageとそのmetadataを一つのPlanへ固定する。
同じIntentのStage実行中にcatalogやpluginが変わっても、開始時に決めた構成を再計算しない土台を作る。

## 実装範囲と所有権

実装担当が変更するファイルは次のとおりである。

- `src/internal/stageplan/plan.go`
- `src/internal/stageplan/plan_test.go`
- `src/internal/state/initial.go`
- `src/internal/state/initial_test.go`
- `docs/architecture.md`
- `docs/development.md`
- `docs/ram/README.md`
- `docs/ram/research/2026-09-03-stage-plan-contracts.md`
- `docs/ram/decisions/2026-09-03-stage-plan-builder-plan.md`
- `docs/ram/decisions/2026-09-03-aidlc-implementation-roadmap.md`

Go標準ライブラリと既存内部packageだけを使う。Issue・commit・push・独立review・final検証・PRは親agentが
担当する。CLI、Plan永続化、StartIntent全接続、runtime recompose、Stage本文実行は含めない。

## APIと受入条件

```go
type Input struct {
    Graph       graph.Snapshot
    Scope       string
    ProjectType string
}

func Build(input Input) (Plan, error)
```

`Plan`はStage number順の全entryを保持する。各entryは完全な`graph.Stage`、実効`graph.Action`、判断
理由を含み、execute/skip一覧とstructured advisoryをaccessorで公開する。accessorの返却値はdeep copyで、
callerのslice・Stage metadata変更がPlanに波及しない。Planを再構成・変更するAPIは作らない。

受入条件は次のとおりである。

- known scopeと`Greenfield` / `Brownfield`から全Stageをnumber順に構築できる。
- scope cell欠損はSKIP、Greenfieldの`reverse-engineering`はEXECUTE指定でもSKIPになる。
- unknown scope・unknown project typeは部分Planなしのerrorになる。
- 実効EXECUTE Stageのrequiredかつ条件一致consumeだけを検証する。
- producerは`produces`と`optional_produces`の和から探し、不在はerror、全てSKIPはstructured advisoryとする。
- producerが1つでもEXECUTEならadvisoryを出さず、optional/条件不一致consumeは無視する。
- `requires_stage`はmetadataとして保持するが、SKIP Stageのexecution closureやSKIP参照だけのerrorにはしない。
- `state.BuildInitial`のRoutingとstate contentは一度構築した同じPlanから導出し、`Initial.Plan`にも返す。
- 既存stateのcanonical出力を不必要に変更しない。

## 本家AI-DLC 2.6.123との意図的な差分

確認対象はローカル固定snapshotの`aidlc-graph.ts`、`aidlc-utility.ts`、`aidlc-orchestrate.ts`、
integration testである。詳細は[参照契約](../research/2026-09-03-stage-plan-contracts.md)を参照する。

| 本家の挙動 | 採用する挙動 | 変更理由 | 利用者・互換性への影響 |
| --- | --- | --- | --- |
| Intent作成経路では`validateScope`を直接呼ばず、producer不在をそこで確定しない | Plan builderでproducer不在をerror、全producerがSKIPならadvisoryとして確定する | 実行不能なcatalogをIntent開始前にfail-closedで拒否し、標準scopeのoff-path依存は許容する | 壊れたcatalogは早期に拒否される。off-pathだけの正常構成はStage追加なしで警告になる |
| runtimeはprocessごとにcompiled graphを読み直し、`.aidlc-plan.json`を実行時sourceにしない | Intent開始時の完全な`graph.Stage` metadataをPlanへsnapshotする | 実行中のcatalog/plugin変更で同一Intentの構成を変えない | 作成済みPlanに後のcatalog変更は反映されない。永続化・実行接続は後続consumerが担当する |
| 一部経路ではunknown project typeでも条件付きconsume比較を通過し得る | `Greenfield` / `Brownfield`以外はerrorにする | 条件付き依存を暗黙に無視する誤構成を防ぐ | callerは未知値でなく正しいproject typeを指定する必要がある |

off-path producerをadvisoryに留めることと、`requires_stage`をexecution closureにしないことは本家の
scope/Stage契約に合わせるため、意図的差分ではない。

## TDD slices

`verification_mode=loop`で対象packageだけを検証し、各behaviorをRED→最小GREEN→必要なrefactorの順で進める。

1. Stage number順の全entry、完全metadata、scope/project type識別。
2. Greenfield補正、scope欠損SKIP、unknown入力とpartial Planなし。
3. required consumeのproducer不在error、optional producer、条件一致判定。
4. off-path advisory、execute producerでadvisoryなし、`requires_stage`非closure。
5. Plan accessorのdeep copy。
6. `state.BuildInitial`がPlanからRouting/state contentを導出し、Initialへ返すこと。

各sliceのtargeted commandは`go test -count=1 -run ... ./src/internal/stageplan`または
`go test -count=1 -run ... ./src/internal/state`とする。最後に変更Goへgofmtを適用し、
`go test -count=1 ./src/internal/stageplan`と`go test -count=1 ./src/internal/state`を実行する。
全package test、race、vet、lint、cross compile、配布E2Eは親agentが`final`で一度だけ実施する。

## リスクと後続

- Planはin-memoryのため、process再起動後の再利用や永続化の契約はまだない。
- runtimeが後続でPlanを参照するまで、実行中catalog変更を防ぐ責任は上位接続側にある。
- off-path advisoryはwarningとして返すが、ユーザー向け表示文や自動Stage追加はこのIssueで決めない。

## 実装時点のTDD証拠

実装担当は`verification_mode=loop`で、最初に次のtargeted baselineが成功することを確認した。

```text
go test -count=1 ./src/internal/state ./src/internal/graph
ok   github.com/sori883/ai-dd/src/internal/state
ok   github.com/sori883/ai-dd/src/internal/graph
```

代表的なRED/GREENは次のとおりである。

```text
RED  go test -count=1 ./src/internal/stageplan -run '^TestBuildPreservesOrderedStageEntriesAndMetadata$'
     undefined: Build / undefined: Input
GREEN 同じtargeted test
      ok   github.com/sori883/ai-dd/src/internal/stageplan

RED  go test -count=1 ./src/internal/stageplan -run '^TestBuildAdjustsReverseEngineeringForGreenfield$'
     reverse-engineeringがEXECUTEのままで、greenfield補正を表せない
GREEN 同じtargeted test
      ok   github.com/sori883/ai-dd/src/internal/stageplan

RED  go test -count=1 ./src/internal/stageplan -run '^TestBuildRejectsRequiredArtifactWithoutProducer$'
     producer不在なのにerrorがnil
GREEN 同じtargeted test
      ok   github.com/sori883/ai-dd/src/internal/stageplan

RED  go test -count=1 ./src/internal/state -run '^TestBuildInitialReturnsTheStagePlanUsedForRouting$'
     InitialにPlan fieldがなくcompile error
GREEN 同じtargeted test
      ok   github.com/sori883/ai-dd/src/internal/state
```

追加したoff-path advisory、optional producer、conditional consume、`requires_stage`非closure、deep copy、
Greenfield skip順維持の回帰testを含め、最後に次を実行して成功した。

```text
gofmt -l src/internal/stageplan/plan.go src/internal/stageplan/plan_test.go src/internal/state/initial.go src/internal/state/initial_test.go
（出力なし）
go test -count=1 ./src/internal/stageplan
ok   github.com/sori883/ai-dd/src/internal/stageplan
go test -count=1 ./src/internal/state
ok   github.com/sori883/ai-dd/src/internal/state
```

全package test、race、vet、lint、cross compile、配布E2Eはloopでは実行せず、親agentのfinal gateへ委譲する。
