# AI-DLC Go実装ロードマップ（概要）

- 日付: 2026-09-03
- 状態: Accepted（ユーザー承認済みの概要方針）
- 対象: AI-DLC v2をGoの単一バイナリへ段階的に実装する開発プロジェクト

## 目的

AI-DLCの全体を一度に作り込まず、Intent開始から完了までの薄い接続を早めに作り、各機能を
順次差し込んで動作を確認できる状態にする。これは詳細仕様を先に固定する文書ではなく、
実装順序を思い出すための粗いメモである。

## 大きな順序

1. Intent開始を完成させる
   - Stage catalog metadata
   - IntentごとのStage Plan
   - `StartIntent`への接続
2. 薄い全life-cycleを接続する
   - 現在Stage、directive、成果物確認、承認、state advance、次Stage、完了
3. OKF v0.2を統合する
   - Knowledge Bundle、metadata検索、byte・文書数制限、context組み立て
4. 33 Stageをphase単位で実用化する
   - Initialization、Ideation、Inception、Construction、Operation
5. 高度機能と配布を整備する
   - review loop、sensor、並列Construction、CodeKB/DocumentKB、park/resume、install/update、E2E

各段階は完全な縦割りではなく、薄い接続、機能slice、対象テスト、独立review、final検証、PRの
gateを繰り返す。細かな順序や対象fieldは各Issueの承認済み計画で確定する。

## 開発上の前提

- 各sliceは実装計画、GitHub Issue、TDD、独立review、final検証、PRのgateを持つ。
- Stage構成はIntent開始時点で確定し、Stage実行中の構成置換は行わない。
- OKF参照はローカルの固定v0.2原典を基準にし、必要なcontextだけを決定的に選ぶ。
- AI-DLC利用プロジェクトのKnowledgeと、この開発プロジェクトのRAMは混在させない。

## 更新方針

この概要は大きな順序の記録であり、各Stageの詳細や未実装機能一覧はここへ追記しない。方針を
変更する場合は過去の記録を消さず、新しいRAM決定から本記録を置換する。
