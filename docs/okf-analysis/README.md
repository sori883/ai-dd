# OKF v0.2 参照インデックス

## 目的

このディレクトリは、Open Knowledge Format（OKF）v0.2の仕様と
AI-DLCへの適用検討を、公式リポジトリ全体を毎回読み込まずに参照するための索引である。
最初にこのファイルを読み、質問に対応する観点別文書へ進み、厳密な確認が必要な場合だけ
固定済みの公式仕様を読む。

## 固定した版

- 仕様バージョン: `0.2`
- 取得元: `GoogleCloudPlatform/open-knowledge-format`
- 固定commit: `ad30107c31c06aec8a7d5636e0d1058118604e6f`
- 取得日: 2026-09-03
- ライセンス: Apache License 2.0

取得元URL、hash、更新手順は[取得記録](upstream/SOURCE.md)に記載する。
固定時点でv0.2のGit tagは存在せず、固定commitの
[`SPEC.md`](upstream/SPEC-v0.2.md)が本文中で`Version 0.2`を宣言している。
ローカルコピーを最新upstreamと同一視しない。

## 読み分け

| 知りたいこと | 最初に読む文書 |
| --- | --- |
| Bundleの境界、Concept、適合条件 | [01-bundle-conformance.md](01-bundle-conformance.md) |
| front matter、来歴、信頼、ライフサイクル | [02-frontmatter-trust-lifecycle.md](02-frontmatter-trust-lifecycle.md) |
| リンク、`index.md`、`log.md` | [03-links-index-log.md](03-links-index-log.md) |
| AI-DLCでの検索、context制限、既存knowledgeとの境界 | [04-aidlc-retrieval-guidance.md](04-aidlc-retrieval-guidance.md) |
| 仕様の厳密な文言、Attested Computation、完全な例 | [公式仕様の固定コピー](upstream/SPEC-v0.2.md) |
| ライセンス条件 | [Apache License 2.0](upstream/LICENSE.md) |

通常は観点別文書を1つだけ読む。複数の観点にまたがる質問だけ、2つ目を読む。

## 情報の区別

この索引では、次の種類を混同しない。

- **仕様事実**: 固定したOKF v0.2公式仕様に書かれていること。
- **本家の観測事実**: ローカルのAI-DLC v2.6.123実装・配布物から確認したこと。
- **合意済み判断**: 本プロジェクトで利用者が承認した方針。根拠は
  [プロジェクトRAM](../ram/README.md)に置く。
- **推奨・未解決**: 後続設計の候補。OKFの要件や承認済み仕様として扱わない。

## 現在の合意と未解決事項

合意済みの境界は次のとおり。

- OKF v0.2へ固定する。
- metadata検索はプログラムで決定的に行う。
- OKF文書にAI-DLC固有の`aidlc:` metadataを追加しない。
- 検索indexは初期段階ではプロセスのメモリ上に構築する。
- Stage構成はIntent開始時だけ行い、Stage実行中には変更しない。

次は未解決であり、この索引では確定しない。

- `.codex/knowledge/`、Space固有knowledge、DocumentKBのどこまでをOKF化するか。
- 実行時に1つのOKF Bundleとみなす正確なディレクトリ境界。
- 注入する文書数とUTF-8 byte数のデフォルト値。
- Goで完全なYAMLを解析する際の依存ライブラリ。

## 主要根拠

- [OKF v0.2公式仕様の固定コピー](upstream/SPEC-v0.2.md)
- [公式ライセンスの固定コピー](upstream/LICENSE.md)
- [取得記録](upstream/SOURCE.md)
- [OKF v0.2参照基盤と初期統合境界](../ram/decisions/2026-09-03-okf-reference-boundaries.md)
