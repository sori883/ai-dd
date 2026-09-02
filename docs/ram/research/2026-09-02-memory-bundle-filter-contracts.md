# Memory bundle filterの参照契約

- 調査日: 2026-09-02
- 比較対象: ローカルAI-DLC v2.6.123（`docs/実装_aidlc-workflows`）
- 対象Issue: [#49「Memory bundle filterを実装する」](https://github.com/sori883/ai-dd/issues/49)
- 確認範囲: `core/tools/aidlc-steering.ts:25-50`のtemplate preambleとsubstantive判定。最新upstream、全workflow、全harnessの完全なparityは確認していない。

## 本家の根拠

`aidlc-steering.ts:25-38`は、空のteam/project templateに含まれる12行を
`TEMPLATE_PREAMBLE_LINES`として定義している。これは説明用のpreambleであり、著者が追加した
blockquoteを誤ってfilterしないため、行単位で完全一致させる。

`aidlc-steering.ts:42-50`の`isSubstantiveRuleText`は、次の順序で判定する。

1. `<!--[\s\S]*?-->`をglobal・non-greedyに置換してclosed HTML commentを除去する。
2. `\r?\n`で行分割する。
3. 各行をJavaScriptの`String.prototype.trim()`でtrimする。
4. trim後の行が空、`#`開始、上記12行のいずれか、またはASCII hyphenだけの3文字以上なら無視する。
5. それ以外の行が1行でもあればsubstantiveとする。

同じ`aidlc-steering.ts`は、実装版core、canonical Codex dist、配置済みCodex配布物でbyte一致を確認した。
比較対象はローカルスナップショットであり、未確認の最新upstreamと同一視しない。

## 採用APIと判定契約

```go
func BuildBundle(sources []Source) []Source
```

`BuildBundle`はlayerを問わず同じclassifierを適用し、substantiveな`Source`を入力順のまま返す。
返却sourceの`Layer`、`Path`、`Content`は変更せず、duplicate pathも保持する。nil、空、全件filter時は
non-nilの空sliceを返し、入力sliceを変更しない。結果sliceはcaller-ownedで、global cache、I/O、error、
その他の副作用を持たない。

| 判定対象 | 採用契約 |
| --- | --- |
| closed comment | `/<!--[\s\S]*?-->/g`相当。global、non-greedy、改行横断。unclosed commentは除去しない |
| 行分割 | `/\r?\n/`相当。LFとCRLFだけをsplitし、lone CR、U+2028、U+2029、U+0085はsplitしない |
| trim | U+0009、U+000B、U+000C、FEFF、全ECMAScript Zs（U+0020、U+00A0、U+1680、U+2000–U+200A、U+202F、U+205F、U+3000）、U+000A/U+000D/U+2028/U+2029。U+0085はtrimしない |
| filter | 空、`#`開始、shipped preambleの12行、ASCII hyphenのみ3文字以上の行を除外 |
| retain | 一般のblockquote、frontmatter field、変更済みpreamble、判定後に1行でも残る本文 |

frontmatterのdelimiterだけがfilter対象になり、fieldの解釈や構造化は行わない。comment除去後に
前後が連結してseparatorや本文になる場合も、本家と同じく除去後の本文を判定する。

## 境界と非対象

source acquisition（Issue #47）とbundle filterを分離する。filterはMemoryのmerge、layer間のoverride、
frontmatter parse、substantive以外のMarkdown解釈、path解決、filesystem read、dedupeを行わない。
upstreamのentry-level first-wins dedupeは将来のgraph/manifest/read orchestrationへ残し、ここでは同じpathを
重複保持する。

Goの`regexp`、`strings`、`unicode/utf8`だけを使用し、外部Go moduleは追加しない。Goの型表現や
内部helper名の差は利用者向けの意図的差分ではなく、新たな本家との仕様・挙動差分は採用しない。

## 未解決事項

- 本記録はローカルv2.6.123の上記ファイルと3形式の一致範囲に基づく。最新upstream、全workflow、全harnessの完全互換は未確認である。
- filter単体はinternal APIで、workspace、graph、stage consumerへの接続は後続作業とする。
