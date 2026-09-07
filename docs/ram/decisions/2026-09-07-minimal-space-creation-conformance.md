# Space作成もAI-DLCに準拠する

- 日付: 2026-09-07
- 状態: Accepted（ユーザー直接指示。設計上の準拠方針）

ユーザーは「spaceの作成もAI-DLC準拠です」と指定した。
新製品でもSpace作成は独自方式へ置き換えず、リポジトリ固定AI-DLC `2.6.123` の確認済み契約を基準にする。

## 設計へ反映すること

- 公開入口は `aidlc space create <name> [--project-dir <path>]` を基準にする。
- 名前の正規化・予約名・重複時の扱い、`aidlc/spaces/<name>/` への配置を本家契約と照合する。
- 本家では作成と選択は別操作で、作成成功だけではactive Space・Intentやsession bindingを変更しない。この境界を維持する。
- Space作成だけでIntent/KDRを自動作成したり、作業を開始したりしない。
- M1の一周試験は、ディレクトリを手作業で用意するだけでなく、正規のSpace作成入口から始める。

## OKF初期化との関係

本家の作成処理はmemory・intents・codekb・knowledgeの初期構造を作り、knowledge自体は空の置場である。
新製品ではユーザー指定により、Spaceのknowledge配下にKnowledge・KDR・RuleをOKFとして置く。
Space作成契約とOKF初期化を区別し、両者の接続をM1計画へ明示する。

OKF初期化を作成内部へ接続するか、配布・導入処理で接続するかは未確定。
本家のorg継承や初期memoryと初期Rule配置の対応、途中失敗・再試行は、固定版の根拠と新製品方針を合わせて具体化する。
今回の指示から旧初期ファイルの削除や新しい継承仕様を推測して採用しない。
差分が必要なら、本家の挙動・採用する挙動・理由・利用者への影響を計画で提示する。

## 根拠と許可

- [Space作成の固定版参照契約](../research/2026-08-31-space-creation-contracts.md)
- [配布準拠の合意](2026-09-07-minimal-layout-distribution-conformance.md)
- [Space knowledgeへのOKF集約](2026-09-07-space-knowledge-okf-unification.md)

本家最新upstreamとの一致を主張せず、旧Intent移行不要・既存data削除なしの合意も維持する。
今回の反映は設計・RAM・索引のみ。M1全体の実装許可や、旧ロードマップの包括承認の流用とは扱わない。
