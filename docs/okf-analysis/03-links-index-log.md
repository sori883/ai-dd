# リンク、index、log

## Concept間リンク

Concept間の関係は標準Markdown linkで表す。

- `/tables/customers.md`のように`/`から始まるlinkはBundle root基準であり、
  文書移動の影響を受けにくいため推奨される。
- `./other.md`や`../shared/policy.md`のような相対linkも利用できる。

linkは有向の関係があることだけを表し、依存、参照、joinなどの意味は周囲の文章で説明する。
リンク切れは、まだ書かれていないConceptを指している可能性があるため、
consumerがBundle全体を拒否する理由にはならない。

`resource`、`sources[].resource`などのpath値は、絶対URL、Bundle root基準path、
相対pathを取れる。`references/`は外部資料、実行手順、codeなどをBundle内へ置く慣例であり、
必須directoryではない。

## index.md

`index.md`は任意であり、Concept本文を開く前にその階層の内容を把握する
progressive disclosureの入口になる。

entryは通常、Conceptへのlinkとfront matterの`description`を含む。
producerが自動生成してもよく、存在しない場合はconsumerが一時的に合成してもよい。

Bundle rootの`index.md`だけは、次のように対象版を宣言できる。

```yaml
---
okf_version: "0.2"
---
```

## log.md

`log.md`は任意の変更履歴で、日付ごとの平坦なlistを新しい順に記録する。
日付headingはISO 8601の`YYYY-MM-DD`形式でなければならない。
entry先頭の`Update`、`Creation`などは慣例であり必須語彙ではない。

## 検索indexとの違い

`index.md`は人間とagentのための可読な案内文書である。CLIがfront matterを走査して
メモリ上に作る検索用`[]Entry`やmapとは別物である。

本プロジェクトの初期検索indexは、CLI processの起動中だけメモリに保持して終了時に破棄する。
永続cacheは、走査時間が実測上の問題になった場合にだけ、無効化、atomic write、破損回復、
並行実行を含めて別途設計する。

最後の段落は本プロジェクトの合意であり、OKF v0.2の仕様要件ではない。

## 仕様根拠

- [§6 Cross-linking and paths](upstream/SPEC-v0.2.md#6-cross-linking-and-paths)
- [§8 Index files](upstream/SPEC-v0.2.md#8-index-files)
- [§9 Log files](upstream/SPEC-v0.2.md#9-log-files)
- [§11 Conformance](upstream/SPEC-v0.2.md#11-conformance)
