# Stage catalog metadataの参照契約

- 日付: 2026-09-03
- 状態: Current for local AI-DLC v2.6.123 snapshot
- 対象Issue: #55

## 確認範囲

ローカルに配置されたAI-DLC v2.6.123の実装形式、canonicalなCodex配布形式、Stage定義文書を
読み取り専用で確認した。

- `docs/実装_aidlc-workflows/core/tools/aidlc-graph.ts`
- `docs/配布_ai-dlc/.codex/tools/data/stage-graph.json`
- `docs/実装_aidlc-workflows/docs/reference/15-stage-definition.md`

これは上記のローカルスナップショットの確認であり、最新upstream、全workflow、全配布形式との
parityを主張しない。SerenaとContext7はこのセッションで利用可能なtoolとして公開されていない
ため、索引とローカル原典を使った。

## 本家のStage metadata

本家のcompiled Stageは、stage固有の実行情報に加えて次のmetadataを持つ。

- `produces`: 必ず生成する成果物の配列
- `optional_produces`: 条件付きで生成し得る成果物の配列。欠損は「なし」
- `consumes`: 入力成果物の配列。各行は`artifact`と`required`を持ち、必要に応じて
  `conditional_on: brownfield|greenfield`を持つ
- `requires_stage`: 依存または表示順序を表すstage slugの配列

Stage定義文書では、`requires_stage`に二つの役割がある。成果物を供給するstageへの意味的な
依存と、同じphase内の表示順序を安定させるための弱い順序edgeである。そのため、これを単純な
「必ず同じscopeで実行するstageのclosure」と解釈してはいけない。

`consumes[].required`はactive planの中での条件付き意味を持ち、producerがscopeから除外される
場合にもgraph自体は不正にならない。`conditional_on`を持たない行は無条件のconsumeを表す。

本家のruntime loaderはJSON parse後の値をStage型として利用するが、runtime Load境界ではこれらの
必須field、consume行、未知slug、重複edge、number順を網羅検証しない。一方、compiler側には
`numericStageOrder`による依存先numberの先行検証がある。

## Go側の初期契約

`src/internal/graph`は、次の値型を`graph.Stage`に保持する。

```go
type Consume struct {
    Artifact      string
    Required      bool
    ConditionalOn string
}
```

`produces`、`consumes`、`requires_stage`は欠損・`null`・型不正をerrorとし、空配列は有効とする。
`optional_produces`は欠損を許容し、存在する空配列は空sliceとして保持する。`Consume`の
`artifact`と`required`は必須で、`required:false`とfield欠損を区別する。`conditional_on`は
省略、`brownfield`、`greenfield`だけを受理する。

stageとconsume行のfield名は完全一致で解釈し、case違いをunknown fieldとして扱う。unknown field
自体は既存loaderと同じく無視する。配列とconsume行は入力順・値を保持し、`Snapshot.Stages()`は
新しいsliceを返してcaller mutationからsnapshotを守る。

`requires_stage`は全compiled graphのslugを参照でき、disabled stageやscope外のstageも参照対象に
含める。既知slug、依存先numberの先行、同一stage内の重複edgeだけをLoad時に検証し、実行closure、
scope filtering、producer-consumer意味解決は行わない。後者はStage Plan builderの責務とする。

## 未確定事項

`condition`、`for_each`、`produces_kinds`、sensor、reviewer、workspace artifactの意味解決は、
Stage実行またはStage Plan builderのsliceで調査・実装する。今回のcatalog readerでは保持しない。
