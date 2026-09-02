# Front matter、来歴、信頼、ライフサイクル

## 基本metadata

OKF v0.2で常に必須なのは`type`だけである。次の項目は推奨または任意である。

| 項目 | 役割 |
| --- | --- |
| `title` | 人間向け表示名 |
| `description` | 検索結果やpreviewに使える1文の要約 |
| `resource` | Conceptが表す実体のURI |
| `tags` | 複数の観点から分類する短い文字列の配列 |

`type`の値には中央registryがなく、consumerは未知の値を一般的なConceptとして扱える必要がある。

## 来歴

`sources`はConceptの根拠となった資料を表す。各entryでは`resource`が必須で、
`id`、`title`、`author`、`usage_count`、`last_modified`などを任意で持てる。
`usage_count`を使う場合は、原則として`usage_window`で観測期間を明示する。

本文中の個別claimを出典へ結び付ける場合は、`sources[].id`と同じlabelの
Markdown footnoteを使う。配列位置ではなく安定したIDで結ぶため、entryの並べ替えで
誤った出典へ接続されにくい。

## 生成と検証

- `generated.by`: 現在の内容を生成したactor。指定時は必須。
- `generated.at`: 内容が最後に意味のある変更を受けた日時。
- `verified`: 出典やresourceに照らして確認したactorと日時の一覧。

actorは次の表記を使う。

- agentまたはtool: `<producer>/<version>`
- 人間: `human:<id>`
- 自動process: `process:<id>`

`verified`は1個のmappingまたはlistの両方を受け入れ、consumerは単独mappingを
1要素のlistとして扱う。

## 信頼とライフサイクル

consumerは`verified`から次のtrust tierを導出できる。

| 状態 | 導出されるtier |
| --- | --- |
| `verified`なし | unverified |
| 人間以外による確認だけ | machine-confirmed |
| `human:<id>`による確認あり | human-reviewed |

これはaccess controlではなく、利用者が判断するための助言的signalである。

`status`は`draft`、`stable`、`deprecated`を取り、省略時は`stable`である。
`stale_after`は明示的なUTC offsetを持つISO 8601日時で、その時刻以降をstaleと判定する。

## AI-DLCでのmetadata方針

本プロジェクトでは、OKF文書へ`aidlc:` namespaceや`phase`、`stage`、`agent`といった
AI-DLC固有metadataを追加しない。

IntentやStageから検索するときは、実行側が標準metadataへ対応する検索条件を組み立てる。
検索精度のためにAI-DLC固有の対応表が必要になった場合も、OKF文書内ではなくStage定義や
検索profile側で管理する。この方針により、BundleをAI-DLC以外のconsumerでも利用できる。

この節は本プロジェクトの合意であり、OKF v0.2の必須要件ではない。

## 仕様根拠

- [§4.1 Frontmatter](upstream/SPEC-v0.2.md#41-frontmatter)
- [§5 Provenance, trust, and lifecycle](upstream/SPEC-v0.2.md#5-provenance-trust-and-lifecycle)
- [§7 Actor convention](upstream/SPEC-v0.2.md#7-actor-convention)
- [§11 Conformance](upstream/SPEC-v0.2.md#11-conformance)
