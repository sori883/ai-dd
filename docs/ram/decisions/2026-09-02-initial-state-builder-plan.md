# 初期 aidlc-state.md builderの実装計画

- 日付: 2026-09-02
- 状態: Accepted（実装・P1修正・独立review完了）
- GitHub Issue: [#43](https://github.com/sori883/ai-dd/issues/43)
- 承認: 2026-09-02、ユーザーが実装計画を明示承認
- base: `3a14925`
- 作業branch: `codex/initial-state-builder`
- 関連: [参照契約](../research/2026-09-02-initial-state-builder-contracts.md)、
  [初期実装の境界](2026-08-29-initial-implementation-boundaries.md)

## 目的と境界

既存の`graph.Snapshot`、`scope.Metadata`、workspace分析結果を、後続のIntent作成接続が利用できる
canonicalな初期 stateへ純粋に変換する。実装範囲は`src/internal/state/**`と本記録、参照契約、
`docs/ram/README.md`、`docs/architecture.md`、`docs/development.md`である。

`state.BuildInitial(input) (Initial, error)`はfilesystem、audit、CLI、plugin選択、Intent作成、
stage本文実行を行わない。Go標準ライブラリと既存内部packageだけを使い、外部moduleは追加しない。
本家ローカル`2.6.123`からの意図的な仕様・挙動差分はない。

## APIと受入条件

実装するAPIは次のとおりである。

```go
func BuildInitial(input Input) (Initial, error)
```

`Input`はgraph、scope名・metadata、ProjectType/Languages/Frameworks/BuildSystem専用DTO、project
root、開始日時、raw description、callerが安全化したpreview、depth/test/review overrideを持つ。
`Initial`は`StateContent`、`ProjectDescriptionJSON`、構造化`Routing`を返す。execute/skip sliceは
callerが変更してもbuilder内部や相互の結果を壊さない。raw descriptionが空の場合は本家に合わせて
`[Project description]`をJSON sidecarへ保存する。

受入条件は次のとおりである。

- 本家と同じstate section/field順、空行、コメント、phase順、marker、末尾LF、State Version 8を出す。
- depthはoverride > scope Depth、test strategyはoverride > scope TestStrategy > effective depthで解決し、
  3値をcase-insensitive canonicalizeする。
- reviewは未指定/adversarialを空保存、advisory/noneをcanonical lowercaseで保存する。
- graph順のexecute/skip、missing cell SKIP、Greenfield reverse-engineering補正、incremental warning bool、
  first/next/phase/agent/countを構造化して返す。
- raw descriptionを本家`JSON.stringify`互換のJSON string + LFで返し、stateのProject表示には安全なpreviewだけを使う。
- unknown scope、無効設定、metadata不一致などの境界はcontext付きerrorにし、部分結果を返さない。

## TDD sliceと実施証拠

`verification_mode=loop`で、`src/internal/state`だけを対象に、受入条件の観測可能な動作を順番に
assertionとして固定した。

1. Brownfield canonical stateのgoldenとJSON sidecar。
2. graph順routing、missing cell SKIP、first/next、初期化件数、返却slice所有権。
3. Greenfieldのreverse-engineering補正、skip末尾理由、incremental warning、raw mappingによるnext。
4. depth/test/review override、case-insensitive canonicalization、previewとrawの分離、construction marker。
5. fallback（`intent-capture` / `IDEATION` / `aidlc-product-agent`）、empty slice、unknown scopeと無効値のerror。

代表的なRED/GREEN証拠:

```text
RED  go test -count=1 -run '^TestBuildInitialBrownfieldGolden$' ./src/internal/state
     undefined: BuildInitial / Input / WorkspaceInfo
GREEN go test -count=1 -run '^TestBuildInitialBrownfieldGolden$' ./src/internal/state
     ok

RED  go test -count=1 -run '^TestBuildInitialGreenfieldAdjustsReverseEngineering$' ./src/internal/state
     StageRoute に Reason がなく、greenfield補正理由を構造化できない
GREEN go test -count=1 -run '^TestBuildInitialGreenfieldAdjustsReverseEngineering$' ./src/internal/state
     ok

RED  go test -count=1 -run '^TestBuildInitialFallbackAndErrors$' ./src/internal/state
     unknown scopeでmetadata不一致を先に返し、期待するunknown scope境界に到達しない
GREEN go test -count=1 -run '^TestBuildInitialFallbackAndErrors$' ./src/internal/state
     ok
```

slice追加後およびgofmt適用後の最終loop確認:

```text
go test -count=1 ./src/internal/state
ok   github.com/sori883/ai-dd/src/internal/state
```

loop中は全package test、race、vet、全体lint、cross compile、配布E2Eを実行していない。独立review後、
親agentが差分を固定して必要なreview修正をloopへ戻し、blocking findingがなくなった時点でfinal gateを
一度だけ実行する。

## 所有権と後続

- `go_tdd_implementer`: `src/internal/state/**`、本計画・参照契約、architecture/development、RAM索引
- 親agent: Issue、commit、push、独立review、final gate、PR、Issue close
- `independent_reviewer`: 固定base/headのread-only review

次のsliceではstate writer、audit bootstrap、workspace Detect/CreateIntentとの接続、public intent create
CLIを別Issueで扱う。state builderにfilesystem権限を追加しない。

## P1 review修正（2026-09-02）

独立reviewで、標準`encoding/json`が`<`、`>`、`&`、U+2028、U+2029をHTML安全化のためescapeするため、
本家2.6.123の`JSON.stringify`とsidecar byteが一致しないことが判明した。`aidlc-utility.ts:5964-5967`
の`${JSON.stringify(rawProjectDesc)}\n`を根拠に、標準ライブラリのみでJSON構文を生成し、該当5文字の
過剰escapeだけを安全に復元する実装へ修正した。quote、backslash、U+0000〜U+001Fのescapeとliteralな
`\u`列は回帰テストで保持を確認する。これは本家との意図的な仕様差分ではなく、byte互換性の修正である。
回帰テストのTDD evidenceは次のとおりである。

```text
RED  go test -count=1 -run '^TestBuildInitialProjectDescriptionJSONMatchesJSONStringifyEscaping$' ./src/internal/state
     FAIL: got \\"\\u003c\\u003e\\u0026\\u2028\\u2029...\\\\u003c\\"\\n, want unescaped <>&/U+2028/U+2029
GREEN go test -count=1 -run '^TestBuildInitialProjectDescriptionJSONMatchesJSONStringifyEscaping$' ./src/internal/state
     ok
GREEN go test -count=1 ./src/internal/state
     ok   github.com/sori883/ai-dd/src/internal/state
```

修正後の固定head `1b80110`を再reviewし、前回P1の解消と追加のblocking findingがないことを確認した。
targeted test以外の全体検証はreview modeでは実行せず、差分安定後のfinal gateへ進めた。
