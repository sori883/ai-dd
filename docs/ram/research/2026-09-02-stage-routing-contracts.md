# Stage graph・scope routingの参照契約

- 日付: 2026-09-02
- 状態: Current for local AI-DLC 2.6.123 snapshot
- 関連: [実装計画](../decisions/2026-09-02-stage-routing-plan.md)、
  [読み取り専用workspace分析](2026-09-02-workspace-detection-contracts.md)

## 確認範囲

ローカルAI-DLC `2.6.123`について、分析索引から次のauthored sourceとcanonical Codex dist、
配置済みCodex distの必要箇所だけを静的に確認した。

- `docs/実装_aidlc-workflows/core/tools/aidlc-lib.ts:31-75,20677-20767`
- `docs/実装_aidlc-workflows/core/tools/aidlc-graph.ts:141-178,423-447,1038-1108,1408-1468`
- `docs/実装_aidlc-workflows/core/tools/aidlc-stage-schema.ts:11-70`
- `docs/実装_aidlc-workflows/dist/codex/.codex/tools/data/stage-graph.json`
- `docs/実装_aidlc-workflows/dist/codex/.codex/tools/data/scope-grid.json`
- `docs/実装_aidlc-workflows/dist/codex/.codex/scopes/aidlc-classic.md`

canonical Codex distと配置済みCodex distで、確認対象dataのSHA-256はそれぞれ一致した。

| data | SHA-256 |
| --- | --- |
| `stage-graph.json` | `79e5750b368417da8978a4ae74ceaa8aae64ccec0ef0cb60d66bda4d6507d0e6` |
| `scope-grid.json` | `2b24c6b3b861f9d2c3a3f9fd2e2bb34c3d3c0e37ddc06ff96145f1d40d3d7c16` |

これは上記2 data fileと列挙したsource範囲の確認であり、最新upstream、全配布物、全workflowの
parityを主張しない。SerenaとContext7はこのCodex sessionで呼び出し可能なtoolとして公開されて
いなかったため、索引と`rg`、狭いsource readで確認した。外部libraryやAPIは採用していない。

## Stage graph

`StageEntry`のruntime必須fieldは`slug`、`number`、`name`、`phase`、`execution`、
`lead_agent`、`support_agents`、`mode`である。`number`は`"0.1"`のようなstring、
`execution`は`ALWAYS`または`CONDITIONAL`、`support_agents`はstring arrayである。
`scopes`はstage側のscope membershipで、欠損と空配列を同じように扱う。

`loadStageGraphAll`は`stage-graph.json`のreadとJSON parse errorへ文脈を付ける。
`loadStageGraph`は配列順を保ったまま`enabled !== false`のstageだけを返す。`enabled:false`は
plugin selectionで無効化されたnodeにだけ現れ、通常の有効nodeではfield自体が省略される。

本家runtime loaderはparse結果を`StageEntry[]`へcastし、各field、重複slug・number、enumを
この境界では検証しない。Go版は承認済みfail-closed境界として、必須field、array shape、
execution enum、identity重複をLoad時に検証する。

## Scope gridとrouting

compiled gridの形は次である。scopeの説明、depth、keywords、test strategy等のmetadataは
このJSONではなく、別の`.codex/scopes/aidlc-<name>.md` frontmatterが所有する。

```json
{
  "classic": {
    "stages": {
      "workspace-scaffold": "EXECUTE",
      "intent-capture": "SKIP"
    }
  }
}
```

runtimeのstage解決では、cellが厳密に`EXECUTE`のときだけ実行し、欠損やそれ以外は`SKIP`に
解決する。partialな`stages` mapも有効である。compiled gridのscope keyはsortされ、stage keyは
graph順で出力される。

新しいGo APIの`ScopeNames`は、JSON objectの記述順を外部契約にせず、explicit gridとfallbackの
どちらも名前昇順で返す。これは本家のcanonical grid・fallback・metadata列挙の決定性と整合させる
新設内部APIの順序契約であり、利用者向け仕様の意図的な差分ではない。

本家loaderはgridのreadまたはJSON parseが失敗すると、enabled graphの各`stage.scopes`を
転置する。runtime `loadScopeMapping`系の`transposeScopeGridForMapping`はscope名をsortし、
全enabled stageのcellをmembershipに応じて`EXECUTE`または`SKIP`へする純粋な転置である。

`aidlc-graph.ts`のcompiler / designer側には、initialization phaseをmembershipにかかわらず
`EXECUTE`にする別の`transposeScopeGrid`もある。今回のqueryはstate-initが利用するruntime
`loadScopeMapping`系を対象とするため、このinitialization特例は採用しない。現在の配布dataでは
initialization stageが全stock scopeを列挙しており、通常dataで両転置の結果差はない。

本家は`JSON.parse`後のgridをruntime typeへcastするため、構文上validでもtop-level、scope entry、
`stages`やactionが不正な構造をLoad境界でfail-closedにしない。Go版は承認済みの意図的な差分として、
read errorとJSON構文errorだけをfallback対象にし、valid JSONの構造不正、未知action、全graphにない
stage参照をerrorにする。disabled stageは全graphには存在するため参照自体はvalidだが、公開routing
からは除外する。

## Go実装への示唆と限界

`fs.FS`をdata directoryにroot化して受け取れば、loader自身はwrite能力、cwd、environment、
project root選択を持たずに済む。`encoding/json`の構文確認とtyped decodeを分けることで、fallbackする
syntax errorとfail-closedにするstructural errorを区別できる。unknown JSON fieldは将来互換のため
無視する。

Snapshotはenabled stageとscope actionだけを公開し、scope metadata Markdown、stage definition本文、
agent dispatch、state遷移を解釈しない。full AI-DLC dataをtest fixtureへ複製せず、syntheticな
`testing/fstest.MapFS`で境界を固定する。供給された`fs.FS`のcontainment、更新中の2 file間の一貫性、
data versionの自動migrationはこのread-only loaderの保証外である。
