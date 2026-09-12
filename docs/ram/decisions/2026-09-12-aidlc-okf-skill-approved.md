# AI-DLCに合わせたOKF専用skillを追加する

- 日付: 2026-09-12
- 状態: Accepted（ユーザーの直接作成依頼）

ユーザーは、本家OKF Agent Memoryに専用skillがあり、製品のOKF案内はaidlc-cli内にあるという説明を受け、
「このプロジェクトに適用したokfのskillsを作成してほしい」と依頼した。
製品用 `aidlc-okf` として、既存のSpace配置、Go単一binary、memory CLI、metadata生成、共有writerと
承認境界に合わせた検索・保存手順を作成する。配布と参照接続も依頼を満たす範囲として実装する。

[具体計画](../../design/aidlc-okf-skill-plan.md)に対象・所有範囲・TDD・独立review・final・復旧を記載した。
`aidlc` は進行、`aidlc-cli` は操作選択・文書宣言、`aidlc-okf` は知識の検索・保存を担当する。
これは[従来の責任分担](2026-09-10-project-rule-and-cli-skill-approved.md)の文書保存案内を細分化する。
Ruleの利用者専用という位置付け、Knowledgeのwhat/how、ADRのwhy、進捗state、CLI作業ログは変えない。

本家参照commitは `a09e04918aa84d275b784374b5236d9eeac56c9e`。現行本家のMCP/okfコマンドや
本文検索を導入したものではなく、既存aidlc CLIの実装に合う操作だけを案内する。
開発用okf-referenceと開発RAMは製品の保存対象から区別する。外部module/tool導入、旧記録移行、
既設配置の上書きは含まない。元checkoutの未commitファイルも保全する。

## 実装とloop確認

Issue #177、work_unit_id `aidlc-okf-skill` の単独writerで実装した。
新skillの配置・既存file保全、4資材移転、開始前の保護付きskill読取り、初期化時の欠落拒否、
実機証拠の新skill読取り判定を、それぞれ実行可能な失敗testを先行させて修正した。
承認待ちの単独catは既存の読取り許可でも通り、ALREADY_GREENと区別した。
文書の移管には人工的なREDを作らなかった。

親は全差分と新skillを確認し、installの配置・移転・既存案内、app/flowのskill読取りと初期化、
CLI help、TestMemoryMetadataCommandEvidenceを作業単位末尾で再実行し、すべて成功した。
独立reviewとread-only final、実Codexは別gateであり、このloop結果だけでは完了と扱わない。
実機fixtureは既存helperを通して製品hookを呼び、試験用trust設定を使用する。
通常利用者のtrust操作やhookの全経路を再検証する試験ではない。
最終検証結果はIssueとPRに記録する。
