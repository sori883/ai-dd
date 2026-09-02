# Scope metadata readerの参照契約

- 日付: 2026-09-02
- 状態: Current for local AI-DLC 2.6.123 snapshot
- 関連: [実装計画](../decisions/2026-09-02-scope-metadata-plan.md)、
  [Stage graph・scope routing](2026-09-02-stage-routing-contracts.md)

## 確認範囲

ローカルAI-DLC `2.6.123`について、分析索引から次のauthored source、unit test、canonical Codex dist、
配置済みCodexの必要箇所だけを静的に確認した。

- `docs/実装_aidlc-workflows/core/tools/aidlc-version.ts`
- `docs/実装_aidlc-workflows/core/tools/aidlc-lib.ts:20712-20845,21170-21230,22873-22925`
- `docs/実装_aidlc-workflows/tests/unit/t125-scope-files.test.ts`
- `docs/実装_aidlc-workflows/tests/unit/t225-scope-name-decoupling.test.ts:256-282`
- `docs/実装_aidlc-workflows/tests/unit/t309-listfield-flow-sequence.test.ts`
- `docs/実装_aidlc-workflows/core/scopes/*.md`
- `docs/実装_aidlc-workflows/dist/codex/.codex/scopes/*.md`
- `docs/配布_ai-dlc/.codex/scopes/*.md`

authored、canonical Codex、配置済みCodexの対象`aidlc-lib.ts`は同じSHA-256
`3b0ff3dda8abfbead8ff7e4a61b2ce334d1d3e78977995d1bd249668fc99db09`だった。
3箇所のscope directoryも確認した11ファイルに差分はなかった。これは上記sourceとdataだけの比較で、
最新upstream、他harness、参照snapshot全体のparityを主張しない。

SerenaとContext7はこのsessionでcallable toolとして公開されていなかったため、`aidlc-reference`の
分析索引から狭いsource readと`cmp` / `diff`へfallbackした。外部libraryやAPIは採用していない。

## 列挙とmetadata

本家`loadScopeMetadataAll`はscope directory直下を列挙し、`.md` suffixのfileだけをJavaScript
`.sort()`で並べて読む。したがって順序はUTF-16 code-unit順である。loaderはfilenameの
`aidlc-` prefix、filename stem、frontmatter `name`の一致を検証せず、`name`を返値のidentityにする。
同じ`name`が2ファイルにあれば、sort順で先に読んだfileと重複を見つけたfileをerrorへ示す。

metadataは`name`、`plugin`、`depth`、`description`、`keywords`、`testStrategy`、`runner`、
`skeleton`、`review_cap`、`freeform_default`である。`name`だけが必須で、depthとdescriptionの空値は
保存される。空のplugin / testStrategyは未指定である。pluginの`aidlc-` prefixはcore runner pathと
衝突するため拒否する。unknown frontmatter fieldは無視する。

- runnerはexact `true` / `false`だけをbooleanとして持ち、それ以外と欠損は未指定になる。
- skeletonは欠損・空ならfalse、`on` / `off`を受理し、それ以外はfileと値を示すerrorになる。
- freeform_defaultはexact `true`だけがtrueで、それ以外はfalseになる。
- review_capは`adversarial`、`advisory`、`none`を受理し、その他の非空値はerrorになる。

## zero-dependency frontmatter

frontmatterはfile先頭の`---`とLFまたはCRLFで始まり、次の`\r?\n---`をclosing delimiterとする。
`frontmatterBlock`の`[\s\S]*?` captureは内部の改行を正規化せず、そのまま保持する。scalarは周囲の
ECMAScript whitespaceを除き、両端のsingle / double quoteを外す。ECMAScript側ではU+FEFFをtrimし、
U+0085はtrimしない。`>`、`|`、`>-`、`|-`はblock scalar markerなので空値として扱う。

keywordsは、frontmatter全体から最初のvalid indent `- ` block sequenceを先に探索し、見つからない場合だけ
最初のsingle-line flow sequenceを使う。先行するflowや空・不正block keyは、後続のvalid blockを抑止しない。
block keyと最初のitemの間にJavaScript whitespaceだけの行があっても、正規表現の`\s*`が吸収してblockが
成立する。またdash後に複数のhorizontal whitespaceだけがあるitemもmatcherは成立し、抽出結果は空ではなく
1文字のwhitespaceになる。outer blockの`[^\r\n]+`はU+2028 / U+2029を受理する一方、inner extractorの
`(.+?)`はCR / LF / U+2028 / U+2029をmatchしない。U+2028 / U+2029だけのitemやseparator後にpayloadが
続くitemはblock自体は成立しても抽出されず、後続flowへfallbackしない。payload末尾だけのU+2028 / U+2029
は末尾`\s*`側で扱われ、payloadを抽出する。lone CRより前にouter payloadがなければblockは成立せず、後続
flowを探索する。outerがpayload後のlone CRをoptional terminatorとして消費し、直後のindent itemも反復
matchした場合、raw captureはlone CRを保持する。innerの`.split(/\r?\n/)`はそのlone CRを分割しないため、
space / tab indentの2 itemは1行のinner regexへ渡って抽出失敗し、block結果はemptyになる。通常suffixは
outer反復が止まるため先行payloadを抽出する。raw `\r\r\n`ではouterのoptional CRが最初のCRだけを
消費し、次のCRで反復が止まるためcaptureは先頭itemまでとなり、innerも先頭payloadだけを抽出する。この
matcher成立判定とitem抽出は別の正規表現境界である。
flow parserはquote内のcommaとclosing bracketをseparatorにせず、closing bracket後は空白または`#` comment
だけを受理する。unclosed quote/bracketや余分なsuffixはempty listとなる。unknown YAML全般を解釈する
parserではなく、本家がscope metadataに使う狭いfrontmatter primitiveである。

独立reviewで、当初のGo実装が文書順の最初のkeywords keyを即採用していたparity bugを検出した。
Issue #37内で本家`t309`のblock-first二段階探索へ修正した。これは新しい意図的差分ではない。
再reviewではblock regexの空白境界に残るparity bugを検出し、空白行を挟むblockとwhitespace-only itemを
本家2.6.123と一致させた。これも新しい意図的差分ではない。
final reviewではGoとECMAScriptのtrim集合、およびblock outer / innerのline terminator境界に残るparity bugを
検出した。本家2.6.123のauthored `listField`をBun 1.3.14で実行してU+0085 / U+FEFF / U+2028 / U+2029 /
lone CRの結果を照合し、Issue #37内で一致させた。これらも新しい意図的差分ではない。
post-fix reviewではoptional lone CR直後に反復itemが隣接する場合のraw capture保持が不足するparity bugを
検出した。space / tab indentと通常suffixを同じBun 1.3.14で追加照合し、outer capture後にCRLF / LFだけで
inner行を分割する本家構造へ一致させた。追補の直接実行では、`frontmatterBlock`がdelimiterのLF / CRLFを
認識しつつcapture内部を変換しないことと、raw `\r\r\n` fixtureが先頭itemだけになることを確認した。Go版の
body全体CRLF正規化を除去して一致させた。いずれも新しい意図的差分ではない。

## Go版の承認済み差分

| 本家の挙動 | Go版で採用する挙動 | 理由 | 影響 |
| --- | --- | --- | --- |
| root `readdirSync`の任意errorを空scope集合へ吸収する | `fs.ErrNotExist`だけ非nil empty sliceとし、permission / I/O / partial entries付きerrorはcauseを保持してerrorにする | 設定欠損と読取不能を同じ正常結果にしない | 未配置は従来どおり空。権限・I/O異常はcallerが検知できる |
| `scalarField`の正規表現はcolon後の`\s*`が改行を越え、空scalarで次fieldを値として読む場合がある | scalarは同一行だけを読む | 隣接fieldを黙って盗む不正parseを避ける | malformed metadataが別field値に化けず、必須nameならerrorになる。正常配布dataは不変 |
| process-global cacheを明示resetまで再利用する | cacheを持たず毎回FSを読み、返すslice・keywords・pointerはcaller所有とする | immutable read APIとtest isolationを保ち、staleな配置を隠さない | 呼出しごとにI/Oが発生し、file変更は次回呼出しへ反映される |

これら3件は2026-09-02に提示した計画で承認済みである。plugin selection、freeform_defaultの複数候補
解決、compiled graphとのjoin、state遷移、CLI、writeはこのreaderの責務に含めない。

## 安全性と限界

Go APIはscope directoryへroot化済みの`fs.FS`を受け取り、write、cwd、environment、cache、Root lifecycleを
所有しない。`fs.FS`自体はcontainmentを保証しないため、symlinkや外部pathの安全境界は供給FSとcallerの
責務である。frontmatterは一般YAML互換ではなく、本家2.6.123の狭いscalar/list契約だけを扱う。
