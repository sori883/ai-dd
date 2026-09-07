# SpaceのknowledgeへKnowledge・KDR・RuleをOKFとして集約する

- 日付: 2026-09-07
- 状態: Accepted（配置とOKF対応へのユーザー直接指示。実装開始の承認ではない）

## 指示と採用する境界

ユーザーはAI-DLCのSpaceのknowledge配下へOKFを統合し、Knowledge本体・KDR・Ruleをすべて配置すること、
KDRとRuleもすべてOKFへ対応させることを指定した。

製品の利用先では `aidlc/spaces/<space>/knowledge/` 自体をOKF bundle rootとする。
その配下に一般知識、共有設計、IntentごとのKDR、Ruleを置く。`knowledge/okf/`を別rootとして挟まず、
KDRだけをbundle外の独自Markdownに分離しない。
予約ファイルindex.md/log.mdを除く文書はOKF front matterと本文を持ち、OKFの検索・表示・検証対象とする。
Spaceは知識の共有範囲であり、旧工程stateや全操作auditを復活させる根拠にはしない。

## 具体化する案

- 下位分類は `knowledge/`（一般知識）、`design/`、`kdr/`、`rules/` を提案する。
- KDRは `kdr/<intent-id>.md` とし、OKF Concept IDは `kdr/<intent-id>`。同じIntentは同じ文書を更新する。
- KDRの `type: KDR`、Ruleの `type: Rule` はプロジェクトの型名として提案する。OKFの標準必須型を主張しない。
- 独自 `intent_id` / `kdr_version` metadataの案を撤回し、pathと標準OKF metadataを使う。
- 必須Ruleの入口も `rules/entry.md` のOKF文書とする案。hook設定は入口の指定だけを持ち、本文は複製しない。
- CLIとsessionはSpaceとIntentを明示して保存先を決める案。別Spaceへの暗黙の切替・混合をしない。
- KDRの安全保存に加え、OKF index/logの更新と部分失敗時の修復をM1計画へ含める。

配置と全文書のOKF対応は指示済み。具体的な下位分類、型名、CLI文法、保存実装は設計案である。
外部okf CLIを呼ぶかGo実装を組み込むかは、この配置指示だけで確定したと扱わない。
ローカルの[OKF Go実装参考](../../実装_okf-agent-memory/)を使い、M1の保存・導入方法を整合させる。

## 置換する提案と維持する制約

[M0最小契約案](../../design/minimal-product-contract-m0.md)の `docs/kdr/` と `docs/knowledge/` 分離配置、
Spaceを使わない案、KDR専用front matter、bundle外の必須Rule一覧を置換する。
[前回のRAM](2026-09-07-minimal-product-contract-proposal.md)は検討履歴として保持し、本記録を後続判断とする。

既存Intent・stateの移行、旧方式との互換性、新旧二重運用は引き続き不要。
既存ファイルの移動・削除や、旧knowledgeの一括変換は実施しない。
この開発リポジトリのdocs/ramと利用先Spaceのknowledgeは混在させない。
M0全体の採用、M1実装、外部moduleやtool導入を承認したとは扱わない。

## 確認した根拠

- 現行 `src/internal/workspace/` のSpace配置は `aidlc/spaces/<space>/`。
- [OKF bundle適合条件](../../okf-analysis/01-bundle-conformance.md): Concept IDはbundle相対path、非予約Markdownにはfront matterとtypeが必要。
- [OKF metadata](../../okf-analysis/02-frontmatter-trust-lifecycle.md): 未知のtypeを許容。statusは文書の成熟度であり工程stateとして使わない。
- [ローカルConvention](../../実装_okf-agent-memory/docs/CONVENTION.md): プロジェクトによる型分類とOKF適合、未知metadataの保持。

今回の変更は設計・RAM・索引だけ。製品コード・設定・Issue・PR・参照元の変更は行わない。
