# OKF・KDR・hookへ製品構成を絞る方針

- 日付: 2026-09-07
- 状態: ユーザーの方向性を記録。具体的な設計・実装・移行はProposed。
- 設計: [OKF・KDR・hookを中心とする最小構成案](../../design/okf-kdr-minimal-workflow.md)

## ユーザーが示した方向性

工程の遷移管理を減らして製品をミニマムにしたい。rulesとknowledge、設計の保存には
`okf-memory/okf-agent-memory`を使いたい。作業の意図と実行結果はPRのようなKDRへ記録し、
AIがCLIでMarkdownテンプレートを読み、記録する。エージェント定義は最小にし、作業のauditは不要。
CLI操作はhookで強制したい。実行順序はCojiの記事を参考にする。

## 今回の設計判断と確認点

提案はOKFの共有記憶、KDR Markdown、読取り・保存・確認CLI、hook、作業とレビューの2役に絞る。
工程state、独自JSON snapshot、全操作ledgerを新案の必須要素にしない。
テンプレート、保存単位、共有正本、検証対象版、再開時の実物確認を設計へ具体化した。

hookの意味には重大な選択が残る。推奨は対応toolでの記録漏れ防止。
CLI以外の書込みそのものを禁止するには権限分離が必要になり、構成・運用が増える。
公式Codex文書もhookを完全な強制境界とはしていない。保証範囲をユーザーへ確認してから実装契約を確定する。
OKF自身の知識変更logは全操作auditとは異なる。前者の無効化までユーザーが指定したとは扱わない。

## 以前の記録との関係・許可の境界

[前の成果物中心設計案](2026-09-07-artifact-centered-workflow-proposal.md)を簡素化する後続提案である。
以前の文書は履歴として残し、独自snapshot等を今回の合意済み事項として持ち越さない。
固定AI-DLC 2.6.123準拠ロードマップと既存OKF検索契約を実装上置換したとは扱わない。
この製品方向転換は旧Stage実用化の包括承認で自動実装できる範囲ではない。
今回の許可は設計と記録まで。コード、設定、Issue、PR、外部tool導入、既存state/audit削除は実施しない。
このrepositoryを開発する側のagent・RAM・GitHub運用ルールも変更しない。

## 調査根拠

OKF Agent Memoryはcommit `d4c523ed5ce916fa207fe314851b98721421c891`のREADME、CLI文書、go.modを参照。
Context7には一致するlibraryがなく、指定repositoryの一次資料を参照した。Codex hookは2026-09-07の公式文書と現行配置設定を照合。
具体的な参照URLと未確定の配布・互換性・運用境界は設計文書へ記載した。
