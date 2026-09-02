# Stage catalog metadataの実装計画

- 日付: 2026-09-03
- 状態: Accepted（Issue #55、実装担当へ委譲済み）
- GitHub Issue: [#55](https://github.com/sori883/ai-dd/issues/55)
- base: `390e1e9`
- 関連: [参照契約](../research/2026-09-03-stage-catalog-metadata-contracts.md)、
  [Stage routing計画](2026-09-02-stage-routing-plan.md)

## 背景と目的

現在のGo `graph.Stage`はroutingに必要なfieldだけを保持し、本家compiled graphにある成果物と
依存metadataを読み捨てている。後続のIntent開始時Stage Plan builderが、依存関係と成果物の
前提を安全に組み立てられるよう、今回のsliceではcatalogをread-only metadata sourceとして拡張する。

この計画はStage実行、scopeの自動変更、producer-consumer意味解決、CLI、書込みを含まない。
既存のgraph順、scope routing、state builderの製品挙動は維持する。

## 実装範囲とファイル所有権

実装担当は次のファイルだけを変更する。

- `src/internal/graph/graph.go`
- `src/internal/graph/graph_test.go`
- `src/internal/state/initial_test.go`（graph fixtureに必須空metadataを追加）
- `docs/architecture.md`
- `docs/ram/research/2026-09-03-stage-catalog-metadata-contracts.md`
- `docs/ram/decisions/2026-09-03-stage-catalog-metadata-plan.md`
- `docs/ram/decisions/2026-09-03-aidlc-implementation-roadmap.md`
- `docs/ram/README.md`

Issue/PR、commit、push、全体final検証は親エージェントが担当する。外部Go moduleや外部toolは
追加しない。テストは標準`testing`を使う。

## 受け入れ条件

1. `graph.Stage`が`Produces []string`、`OptionalProduces []string`、`Consumes []Consume`、
   `RequiresStages []string`を公開し、`Consume`が`Artifact string`、`Required bool`、
   `ConditionalOn string`を値として持つ。
2. graph順、配列順、値を保持する。必須3配列は欠損・`null`・型不正を拒否し、空配列を受理する。
   `optional_produces`は欠損を受理し、空配列を値として保持する。
3. consume行の`artifact`と`required`を必須とし、`required:false`と欠損を区別する。
   `conditional_on`は省略、`brownfield`、`greenfield`だけを受理する。
4. stageとconsume行のexact lowercase key契約を維持し、unknown fieldは無視する。
5. `requires_stage`の既知slug、依存先numberの先行、同一stage内の重複edgeを検証する。
   disabled/scopeによる絞り込み、実行closure化、producer-consumer意味検証はしない。
6. `Snapshot.Stages()`の新規sliceをdefensive cloneする。
7. 既存graph/stateの製品挙動を変えず、state fixtureへ必須空metadataだけを追加する。
8. 外部依存を追加しない。
9. 本家runtimeより厳しいfail-closed検証の拡張を本計画、参照契約、architectureに記録する。
10. 粗い全体ロードマップをRAMに記録する。

## TDD手順

最初のbaselineは`go test -count=1 ./src/internal/graph`で取得する。各sliceでは、まず対象の
観測可能な失敗テストを追加し、意図したREDを確認してから最小のproduction変更を行う。

実装するテスト順は次のとおり。

1. metadataの保持、配列順、optional欠損と空配列
2. 必須配列とconsume行の欠損・`null`・型不正
3. exact keyと`required:false`の区別
4. `requires_stage`の未知slug、number順、重複edge、disabled参照
5. Snapshotの新規slice defensive clone
6. state fixture更新による既存routing/stateテストの回帰確認

loop中に実行するのは、該当する`go test -count=1 -run ... ./src/internal/graph`と
`go test -count=1 ./src/internal/state`の対象テストだけとする。`gofmt`を変更済みGoへ適用し、
影響テストを再実行する。全package test、race、vet、lint、cross compile、配布E2Eは親が
`verification_mode=final`を明示したときだけ行う。

## 本家との差分

比較対象はローカルAI-DLC v2.6.123のruntime graph loader、compiler invariant、Stage定義文書で、
確認範囲は[参照契約](../research/2026-09-03-stage-catalog-metadata-contracts.md)に固定する。

| 本家の挙動 | 採用する挙動 | 変更理由 | 利用者・互換性への影響 |
| --- | --- | --- | --- |
| runtime Load境界はcompiled graphの必須metadata、consume行、依存slug・number順・重複を網羅検証しない | Go loaderはこれらをLoad時にfail-closed検証する | 不完全なcatalogから誤ったStage Planを作らないため | malformed custom graphは早期にLoad errorになる。正規v2.6.123 graphと既存の正常routingには影響しない |

これは本家の意味を変更する差分ではなく、既存Go loaderのfail-closed方針をcatalog metadataへ
拡張する設計判断である。`requires_stage`を実行closureにする変更や、scope中のedgeだけを検証する
変更は採用しない。
