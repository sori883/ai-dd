# Intent開始時Stage Planの参照契約

- 日付: 2026-09-03
- 状態: Current for local AI-DLC v2.6.123 snapshot
- GitHub Issue: [#57](https://github.com/sori883/ai-dd/issues/57)

## 確認範囲

この記録は、このリポジトリに固定された本家AI-DLC `2.6.123`の次の実装とテストを、Intent開始時
Stage構成と成果物依存に必要な範囲だけ読み取った結果である。最新upstream全体や、列挙していない
workflowの挙動との一致は主張しない。

- `docs/実装_aidlc-workflows/core/tools/aidlc-graph.ts`
- `docs/実装_aidlc-workflows/core/tools/aidlc-utility.ts`
- `docs/実装_aidlc-workflows/core/tools/aidlc-orchestrate.ts`
- `docs/実装_aidlc-workflows/tests/integration/t66.test.ts`
- `docs/実装_aidlc-workflows/core/tools/aidlc-version.ts`

## 本家のPlan構築契約

`resolvePlanForScope`は、enabled graphをStage numberで並べ、graphにある全Stageについて
`{ slug, phase, action }`のrowを作る。scopeにaction cellがないStageは`SKIP`になり、明示された
`EXECUTE` / `SKIP`以外は実行対象にしない。CLIの`aidlc-graph resolve`が`.aidlc-plan.json`へ保存する
経路はあるが、runtime orchestratorはこのfileを実行時の情報源にせず、process内でcompiled graphを
読み込む。

Intent作成時の`handleIntentCreateStateBuild`は、scope mappingからexecute/skipとStage Progressを作り、
Greenfieldの`reverse-engineering`を`EXECUTE`から`SKIP`へ補正する。ただし、この経路は
`validateScope`を直接呼ばない。

`validateScope`の成果物検証は、実効scopeで検討する`required: true`かつproject typeに一致するconsumeを
対象にする。producerはenabled graphの`produces`と`optional_produces`の和から探す。producerが1つも
存在しない必須成果物はerrorとなる。producerは存在するが全てscope外なら通常のscope検証ではadvisoryで、
strict mode（in-flight recompose）だけがそのadvisoryをerrorへ昇格する。optional consumeと条件不一致の
consumeは対象外である。

固定snapshotのstock scopeでは、bugfix、infra、poc、refactor、security-patch、classic、workshop、
expressに意図的なoff-path producer advisoryがあり、feature、enterprise、mvpにはない。このため、
通常のIntent開始でoff-path advisoryを一律errorにすると本家の正常scopeを壊す。

`requires_stage`はdata dependencyと同一phase内の弱い表示順edgeを兼ねる。runtimeのserial orderは
number順であり、scope外・disabledの依存Stageを自動的に実行closureへ追加しない。成果物producerが
scope外の場合も、edgeだけを理由にscopeを変更しない。

## Go側で固定する契約

`stageplan.Build(stageplan.Input) (stageplan.Plan, error)`は、enabled `graph.Snapshot`、scope名、
`Greenfield` / `Brownfield`のproject typeから副作用なしにin-memory Planを作る。PlanはStage number順の
全entryを保持し、entryごとに開始時点の完全な`graph.Stage` snapshot、実効action、判断理由を持つ。
scope cell欠損はSKIP、Greenfieldの`reverse-engineering`はEXECUTE指定でもSKIPである。

requiredかつ条件一致のconsumeについて、producer不在は部分Planを返さないerror、producer全てSKIPは
consumer slug・artifact・producer slugを持つstructured advisoryとする。producerが1つでもEXECUTEなら
advisoryは出さない。`requires_stage`はentry metadataとして保持するだけで、closure化やSKIP参照だけを
理由とするerrorは行わない。

Planの各accessorは返却sliceとStage metadataをdeep copyする。Planはrecompose/mutator APIを持たず、
`state.BuildInitial`は一度構築したPlanからRoutingとstate contentを導出し、`Initial.Plan`へ同じPlanを返す。

## 未解決事項

Planの永続化形式、StartIntent全体への接続、runtime executorがPlan snapshotをどう参照するか、
recompose時の別Plan生成は後続Issueで決める。このIssueでは既存state出力を変更しない。
