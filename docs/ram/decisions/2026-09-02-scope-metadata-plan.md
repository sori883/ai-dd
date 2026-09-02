# Scope metadata read-only APIの実装計画

- 日付: 2026-09-02
- 状態: Accepted（Issue #37実装・再review P1解消・fresh再検証・6構成cross compile完了、再々review待ち）
- GitHub Issue: [#37](https://github.com/sori883/ai-dd/issues/37)
- base: `a6c19f673c685facc450fb3c0399bf06b36c4542`
- 作業branch: `codex/scope-metadata-reader`
- 関連: [参照契約](../research/2026-09-02-scope-metadata-contracts.md)、
  [Stage routing計画](2026-09-02-stage-routing-plan.md)

## 承認と範囲

compiled graph readerに続き、`.codex/scopes/*.md`のfrontmatter metadataを読み取る独立sliceの
詳細計画を提示し、ユーザーが2026-09-02に明示承認した。Issue #37と本計画で範囲を固定する。

変更対象は`src/internal/scope`のread-only queryとsynthetic test、architecture、development、
本計画・調査RAM・索引である。plugin filtering、freeform_default候補の衝突解決、graph join、state、
stage execution、CLI、filesystem writeは追加しない。Go標準libraryだけを使う。

## 内部API

callerがscope directoryへroot化した`fs.FS`を渡す。

```go
type ReviewCap string

type Metadata struct {
    Name            string
    Plugin          string
    Depth           string
    Description     string
    Keywords        []string
    TestStrategy    string
    Runner          *bool
    Skeleton        bool
    ReviewCap       ReviewCap
    FreeformDefault bool
}

func ReadAll(scopesFS fs.FS) ([]Metadata, error)
```

`runner`は欠損・invalidとfalseを区別するためpointerを使う。ReviewCapは`adversarial`、`advisory`、
`none`のtyped string constantを持ち、欠損はzero valueで表す。返値と内部cacheを共有しない。

## 読取・parse・validation

- root直下の`.md` suffixだけを対象にし、filenameをJavaScript互換のUTF-16 code-unit順に読む。
- filename prefix、stemとfrontmatter nameの一致は検証しない。unknown fieldは無視する。
- frontmatterはfile先頭`---`、LF / CRLF、同一行scalar、quote除去、block marker空値を扱う。
- keywordsは本家のindent block listとsingle-line flow listだけを扱う。frontmatter全体で最初のvalid
  block sequenceを優先し、blockがなければ最初のflow sequenceを使う。quoted comma / bracket、
  terminal comment、malformed emptyも固定する。block keyとitem間のJavaScript whitespace行、
  whitespace-only itemのmatcher / extractor境界も本家regexへ合わせる。
- nameだけを必須とする。depth / descriptionの空値、plugin / testStrategyの未指定を受理する。
- plugin `aidlc-` prefix、invalid skeleton、invalid review_capはfileと値を示すerrorにする。
- duplicate nameはUTF-16 sort順の先後両filenameを示す。
- 個別read、frontmatter、name、validation、duplicateのerrorではpartial resultを返さない。
- nil FSはpanicせずerror。root `fs.ErrNotExist`だけは非nil empty sliceを返す。

## 本家との意図的な差分

比較対象と根拠範囲は[参照契約](../research/2026-09-02-scope-metadata-contracts.md)に記す。

| 本家の挙動 | 採用する挙動 | 変更理由 | 利用者・互換性への影響 |
| --- | --- | --- | --- |
| root列挙の任意errorをemptyへ吸収 | `fs.ErrNotExist`だけempty、他errorはcause保持 | 欠損と読取不能を区別 | permission / I/O failureをcallerが検知できる |
| 空scalarが次行fieldを値として読む場合がある | scalarは同一行限定 | 隣接fieldの誤読防止 | malformed inputの結果がfail-closedまたは空値になる。正常dataは不変 |
| global cacheをresetまで再利用 | 毎回再読込しcaller所有値を返す | stale readと共有mutationを避ける | 呼出しごとのI/Oが増えるが変更を即時観測する |

## Assertion-first TDD

compile failureをREDにせず、最小stubでrunnable assertionへ到達してから次を順にRED→最小GREENとする。

1. basic scalar metadataとblock keywords。
2. 直下`.md` filter、filename/name decoupling、UTF-16順。
3. LF / CRLF、quote、block marker、same-line scalar。
4. block / flow keywords、block-first二段階探索とmalformed empty。
5. optional値とplugin / skeleton / review_cap validation。
6. duplicate、frontmatter/name、個別read errorのnil partial result。
7. root missing / other / partial errorとnil FS。
8. no-cacheとcaller ownershipは先行GREEN上のguardとして固定する。

fixtureは`testing/fstest.MapFS`と最小error injection FSだけを使い、本家11scope一式を複製しない。

## 独立review修正

固定head `bf35047`のreviewで次を確認し、Issue #37の範囲内で修正・guard追加した。

- P1: list parserが文書順の最初のkeywords keyを即採用していた。本家`t309`どおりblock探索をflow探索
  より先に行う回帰testを追加し、flow-before-block、空・不正block-before-valid、複数候補の先頭選択を
  REDからGREENにした。これはparity bug修正であり、新しい意図的差分ではない。
- P2: `Runner *bool`がrecord間・ReadAll呼出し間で共有されないguardを追加した。現実装は追加時からGREEN
  だったため、RED件数には含めない。
- 再review P1: block keyと最初のitemの間の空・空白行、dash後が空白だけのitemに本家regexとの差が残って
  いた。固定head `dd22500`へ回帰testを追加してREDを確認し、matcher成立判定とitem抽出を分離してGREENに
  した。重複keyの前後でもblock-first順を維持する。これもparity bug修正であり、意図的差分ではない。

## 所有権と対象外

- `go_tdd_implementer`: `src/internal/scope/**`、architecture、development、本計画・調査RAM・索引
- 親agent: GitHub、commit、固定diffの独立review、PR、最終gate
- `independent_reviewer`: 固定base/headのread-only review

外部module、CLI、graph/state接続、write、別package変更が必要になった場合はscopeを広げず停止する。

## 検証

```sh
go test -count=1 ./src/internal/scope
go test -count=1 -shuffle=on ./...
go test -count=1 -race -shuffle=on ./...
go vet ./...
go mod tidy -diff
gofmt -l src/internal/scope
gopls check src/internal/scope/metadata.go src/internal/scope/metadata_test.go
git diff --check
```

darwin、linux、windowsのamd64 / arm64へscope test binaryをrepository外にcross compileする。
これは各OSでのnative実行証拠とは区別する。内部read-only APIなので配布CLI E2Eは行わない。

独立review修正後、上記targeted、全体shuffle、race、vet、tidy diff、gofmt、gopls、diff checkを
fresh実行してすべて成功した。6構成のtest binary cross compileも再実行して成功したが、各target OSでの
native実行とは扱わない。

再review P1修正後にも同じfresh検証と6構成cross compileを再実行し、すべて成功した。
