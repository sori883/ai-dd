# Memory bundle filterの実装計画・実施記録

- Issue: [#49「Memory bundle filterを実装する」](https://github.com/sori883/ai-dd/issues/49)
- 分類: `機能開発`
- 計画承認日: 2026-09-02
- 実装担当: `go_tdd_implementer`（Luna / max）
- verification mode: `loop`
- 状態: Accepted（実装loop・targeted検証完了、独立review待ち）

## 目的と所有範囲

Issue #47の4層Memory source readerが取得したsourceから、本家AI-DLC v2.6.123と同じ
substantive判定でbundleを構築する純粋な内部APIを追加する。変更範囲は次のとおり。

- 新規`src/internal/memory/bundle.go`
- 新規`src/internal/memory/bundle_test.go`
- `docs/architecture.md`のpackage境界とbundle filter契約
- 本記録、参照契約、`docs/ram/README.md`の索引

外部Go moduleは追加せず、標準libraryだけを使用する。source reader、workspace接続、graph/stage
consumer、CLI、merge/override、frontmatter解釈、filesystem I/O、dedupeは今回の責務に含めない。

## 承認済みAPI

```go
func BuildBundle(sources []Source) []Source
```

classifierは全layerに共通で、結果は入力順を保持する。`Layer`、`Path`、`Content`は変更せず、
duplicate pathも保持する。nil、空、全件filter時はnon-nilの空sliceを返す。入力は変更せず、結果sliceは
caller-ownedとする。global cache、I/O、error、その他の副作用は追加しない。

## 承認済み判定

1. closed HTML commentを`/<!--[\s\S]*?-->/g`相当でglobal・non-greedyに除去する。unclosed commentは残す。
2. `/\r?\n/`相当でLFとCRLFだけを行分割する。lone CR、U+2028、U+2029、U+0085では分割しない。
3. 各行をECMAScript trim集合でtrimする。対象はU+0009、U+000B、U+000C、FEFF、全Zs、
   U+000A/U+000D/U+2028/U+2029で、U+0085は対象外とする。
4. trim後の空行、`#`開始行、shipped template preambleの正確な12行、ASCII hyphenだけの3文字以上の
   行を除外する。それ以外の行が1つでもあればsourceを保持する。
5. 一般blockquote、frontmatter field、変更されたpreambleはsubstantiveとし、frontmatter delimiter
   だけはfilterする。

根拠と比較範囲は[Memory bundle filterの参照契約](../research/2026-09-02-memory-bundle-filter-contracts.md)
に記録した。ローカルAI-DLC v2.6.123の確認範囲に対して、新たな利用者向けの意図的な本家差分は採用しない。

## TDD実施記録

loop中は`./src/internal/memory`の`BuildBundle` targeted testだけを実行し、全体test、race、vet、lint、
cross compile、配布E2Eは実行していない。

| slice | RED evidence | GREEN evidence |
| --- | --- | --- |
| 空・空白・heading・本文 | `go test -count=1 -run '^TestBuildBundle' ./src/internal/memory` — `memory.BuildBundle`未定義でcompile failure | 同コマンド — `ok` |
| preamble 12行、ASCII separator、Unicode dash | 同コマンド — preambleと3文字以上ASCII hyphenを旧実装が保持 | 同コマンド — `ok` |
| ECMAScript trim、LF/CRLF、lone CR、U+0085、U+2028/U+2029 | 初回API未定義のcompile failureが導入前提。個別assertion追加時は既存実装でGREEN | 同コマンド — `ok` |
| closed commentのglobal/non-greedy・multiline・unclosed | 同コマンド — comment-only等を旧実装が保持 | 同コマンド — `ok` |
| Markdown境界、collection order/duplicate/ownership/non-nil empty | 追加assertionをpublic API経由で追加 | 同コマンド — `ok` |

実装後の最終loopでは次を実行し、対象packageがGREENであることを確認した。

```sh
gofmt -w src/internal/memory/bundle.go src/internal/memory/bundle_test.go
go test -count=1 -run '^TestBuildBundle' ./src/internal/memory
```

## 残余リスクとレビュー引き継ぎ

- 判定は純粋な本文filterであり、sourceの読み取り一貫性、merge/override、frontmatter構造、graph接続は保証しない。
- dedupeは将来のgraph/manifest/read orchestrationで扱う前提で、同一path重複を保持する。
- 比較対象はローカルv2.6.123の`aidlc-steering.ts`確認範囲に限られ、最新upstream、全workflow、全harness、全配布物の完全互換は未確認である。
- packageはinternal APIのみで、workspaceやCLIへ未接続である。
