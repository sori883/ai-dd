# 初期 state builderの参照契約

- 日付: 2026-09-02
- 状態: Current for local AI-DLC 2.6.123 snapshot
- GitHub Issue: [#43](https://github.com/sori883/ai-dd/issues/43)
- 関連: [実装計画](../decisions/2026-09-02-initial-state-builder-plan.md)、
  [Stage graph・scope routing](2026-09-02-stage-routing-contracts.md)、
  [Scope metadata](2026-09-02-scope-metadata-contracts.md)

## 確認範囲

ローカルに置かれたAI-DLC `2.6.123`の実装とCodex配布物を対象に、初期state生成に必要な範囲だけを
静的に確認した。最新upstream全体や、列挙していないworkflowの挙動との一致は主張しない。

- `docs/実装_aidlc-workflows/core/tools/aidlc-utility.ts:202-230`
- `docs/実装_aidlc-workflows/core/tools/aidlc-utility.ts:5703-5983`
- `docs/実装_aidlc-workflows/core/tools/aidlc-utility.ts:7061-7077`
- `docs/実装_aidlc-workflows/core/tools/aidlc-lib.ts:29-75,117-160`
- `docs/実装_aidlc-workflows/core/tools/aidlc-lib.ts:16301-16386`
- `docs/実装_aidlc-workflows/core/tools/aidlc-lib.ts:21846-21890`
- `docs/実装_aidlc-workflows/core/tools/aidlc-lib.ts:23258-23270`
- `docs/実装_aidlc-workflows/dist/codex/.codex/tools/data/stage-graph.json`
- `docs/実装_aidlc-workflows/dist/codex/.codex/tools/data/scope-grid.json`
- `docs/配布_ai-dlc/.codex/tools/data/stage-graph.json`
- `docs/配布_ai-dlc/.codex/tools/data/scope-grid.json`

canonical Codex distと配置済みCodex distのstage graph / scope gridは、確認時点で同じ内容だった。
確認したcanonical Codex distのSHA-256は`stage-graph.json`が
`79e5750b368417da8978a4ae74ceaa8aae64ccec0ef0cb60d66bda4d6507d0e6`、`scope-grid.json`が
`2b24c6b3b861f9d2c3a3f9fd2e2bb34c3d3c0e37ddc06ff96145f1d40d3d7c16`である。
SerenaとContext7はこのCodex sessionで呼び出し可能なtoolとして公開されていなかったため、ローカルの
分析索引、`rg`、狭いsource readと標準コマンドで調査した。外部Go moduleは必要ない。

## 入力と責務

本家の`handleIntentCreateStateBuild`は、workspace scan、scope mapping、scope metadata、開始日時、
project descriptionを受け、state-initのauditと2つのファイル書込みまで同じ処理で行う。
Go版の初期sliceはそのうち決定的なメモリ上の生成だけを担当し、filesystem、audit、CLI、plugin選択、
Intent作成への接続はcallerまたは後続sliceの責務とする。

`state.BuildInitial`は次の入力を受ける。

```go
type Input struct {
    Graph                     graph.Snapshot
    Scope                     string
    ScopeMetadata             scope.Metadata
    Workspace                 WorkspaceInfo
    ProjectRoot               string
    ProjectDescription        string
    ProjectDescriptionPreview string
    StartDate                 string
    DepthOverride             string
    TestStrategyOverride      string
    ReviewOverride            string
}

func BuildInitial(input Input) (Initial, error)
```

`WorkspaceInfo`は`ProjectType`、`Languages`、`Frameworks`、`BuildSystem`だけを持つ専用DTOである。
`state`は将来workspaceから呼べるように`workspace.ScanResult`をimportしない。
`ProjectDescription`はJSON sidecarへ保存するraw値、`ProjectDescriptionPreview`はcallerが解決済みの
安全なsingle-line表示値であり、builderは両者を混同しない。
raw値が空の場合は本家の`flags.arguments || "[Project description]"`に合わせて既定値を使う。

返却する`Initial`は、canonical `aidlc-state.md`の`StateContent`、raw descriptionを標準
`encoding/json`でJSON構文化した後に本家`JSON.stringify`との過剰escape差分を補正した
`ProjectDescriptionJSON`、後続consumerが使える`Routing`を含む。I/Oはなく、返却routingの
execute/skip sliceはbuilder内部や相互で共有しない。

## 設定解決

- depthは明示`DepthOverride`、scope metadataの`Depth`の順で選ぶ。
- test strategyは明示`TestStrategyOverride`、scope metadataの`TestStrategy`、effective depthの順で選ぶ。
- depthとtest strategyは`minimal`、`standard`、`comprehensive`を大文字小文字に関係なく受け、
  `Minimal`、`Standard`、`Comprehensive`へcanonicalizeする。
- review overrideは`adversarial`、`advisory`、`none`を大文字小文字に関係なく受ける。未指定と
  `adversarial`はstateへ空文字を保存し、`advisory`と`none`はcanonical lowercaseで保存する。
- 未知のoverride、未知のscope、scope名とmetadata名の不一致、無効なscope depthはerrorとする。

## Routing

stage graphのenabled stageをgraph順に歩き、scope cellが`EXECUTE`のときだけexecuteへ入れる。
cellの欠損は本家runtimeどおり`SKIP`であり、skipは`number (slug)`の順でstateへ出す。
`Routing`はexecute/skipの`StageRoute`（slug、number、action、補正理由）、effective設定、first/next、
first phase/agent、total stage数、初期化stage数を返す。

workspaceがGreenfieldでraw mappingの`reverse-engineering`がEXECUTEなら、effective mappingだけを
SKIPへ補正する。executeから除外し、skip末尾へ`number (reverse-engineering — greenfield)`を追加する。
incremental scope（`bugfix`、`refactor`、`security-patch`）では、同じ条件を構造化bool
`IncrementalGreenfieldWarning`で返す。ユーザー向け警告文はbuilderの責務にしない。

first stageは補正後mappingで初期化phaseを除いた最初のEXECUTEで、該当がなければ
`intent-capture`。該当stageがあるときはそのphaseを大文字化しlead agentを使い、なければ
`IDEATION` / `aidlc-product-agent`へfallbackする。next stageは本家`nextInScopeStage`に合わせ、
補正前のraw scope mappingでfirstより後にある最初のEXECUTEを使い、なければ`none`とする。

## Canonical出力

`StateContent`は本家生成templateのsection、field、コメント、phase順、空行、末尾LFを保持する。
State Versionは`8`、Project Description Sourceは`project-description.json`固定である。
初期化stageはすべて`[x]`、それ以外は`[ ]`、補正後の最初のpost-init stageだけ`[-]`にする。
phase statusは初期化を`Verified`、first stageのphaseを`Active`、残りをeffective mappingにEXECUTEが
あれば`Pending`、なければ`Skipped`とする。construction phaseには`Per unit: [TBD]`を置く。

sidecarは`encoding/json`のJSON string化を基盤にし、`JSON.stringify`がescapeしない`<`、`>`、`&`、
U+2028、U+2029の過剰escapeだけを安全に戻した結果へLFを1つ加える。quote、backslash、U+0000〜
U+001FのJSON escapeと、raw値中のliteralな`\u`列は保持する。previewへraw値を再利用したり、sidecarを
読み書きしたりしない。本家の書込み根拠は`aidlc-utility.ts:5964-5967`の
`${JSON.stringify(rawProjectDesc)}\n`である。

## 本家との差分と限界

本家の処理をGo内部APIへ段階移植すること自体は、意図的な仕様・挙動差分ではない。今回、利用者が
観測する初期stateの値とbyte形式について意図的な差分は採用しない。packageを分離し、workspaceの
scan resultを専用DTOへ写像する点は依存方向と段階実装の設計であり、出力契約の変更ではない。

本家が同じhandler内で行うstate file / JSON write、audit event、workspace scanning、scope/plugin
selection、Intent record作成は本sliceに含めない。生成結果のI/O、audit、CLI接続、stage本文実行は後続
sliceで別途検討する。
