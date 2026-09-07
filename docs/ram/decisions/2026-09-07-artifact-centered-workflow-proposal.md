# 合意・成果物・検証・再開を中心にするworkflowの設計検討

- 日付: 2026-09-07
- 状態: **Proposed（設計依頼を記録。実装・互換性変更・移行は未承認）**
- 設計文書: [合意・成果物・検証・再開を中心にしたAI-DLCの設計案](../../design/artifact-centered-workflow.md)
- 基点: `fc8bd7ea633f0c1ad8cc605233572a6106b3b911`（PR #127までのGo実装）
- 固定参照: AI-DLC `2.6.123`。最新upstreamではない。

## ユーザーの要望と今回の依頼

ユーザーは、33 Stageでは実装までの作業が多く、早く作って試し、修正することが難しいと説明した。
利用時に実行Stageを減らすだけでなく、製品として工程構成を簡潔にしたいという希望を示した。
要件定義書等の成果物を中心に考え、Workspaceで共有する成果物の扱いも検討したいと述べた。

不明点は、ユーザーへの質問、外部情報の調査、技術的な実装検証、画面等の試作と操作確認で解消したい。
Code Generation、Build and Test、CI Pipelineを行き来して、実装・テスト・修正を繰り返したい。

参照記事を精読してStageとstateの設計を検討した後、ユーザーはStage遷移をCLIが管理すること自体に
価値があるかを問い直した。親は、Stageは案内として残せる一方、保存の中心は合意・成果物・検証・再開情報と
する案を提示した。ユーザーはその案を具体的に設計し、運用も考慮するよう依頼した。

**今回許可されているのは設計案の作成である。** 7 Stageへの確定、Stage gateの廃止、JSON snapshot、
audit保証の変更、新CLI、shared artifact管理、既存state移行は直接承認されていない。
旧ロードマップを置換済みとせず、元の承認遷移修正・33 Stage実用化をこの設計承認なしに再開しない。

## 提案の要点

1. Intentを小さな目的と完成条件の作業単位として残す。
2. Stage順序を許可・完成の根拠にせず、質問・調査・試作・実装・検証を必要に応じて繰り返す。
3. 合意文書の採用版、出力・参照入力、根拠参照、未解決事項、checkpointだけを小さく保存する。
4. Git、CI、PR、文書本文を正本とし、検証は対象版と条件に結び付ける。AIの申告を実行証拠と混同しない。
5. 記録保存がコードのcommitを変えて検証を失効させないよう、runtime recordと製品コードを分ける。
6. 共有文書は正本・採用版・変更案を分け、入力を黙って最新へ差し替えない。
7. 新workflowは単一snapshotを現在情報の正本とする案。既存audit-firstの変更なので別途承認が必要。
8. 新規Intentの明示選択から開始する案。既存の完了markerを新方式の合格へ移行せず、新旧writerを分離する。

## 既存合意との関係

- [AI-DLC Go実装ロードマップ](2026-09-03-aidlc-implementation-roadmap.md)の33 Stage準拠・実用化方針は
  このProposed記録だけでは置換しない。採用時に新しいroadmap／milestoneを承認する。
- [包括承認](2026-09-03-milestone-authorization-and-autonomous-merge.md)の品質gate、独立review、
  自律merge条件は維持する。本家との新しい意図的差分、公開API、永続data、運用の判断は包括承認に含めない。
- [approve/advance](2026-09-04-approve-advance-plan.md)の二段階audit-firstと中間state保持は旧経路に維持。
  新経路への採否は設計文書の明示判断項目とする。
- [in-flight recompose](2026-09-03-inflight-recompose-policy.md)は旧Stage routingの規則である。
  新work recordを足すだけで旧completed markerを安全に再開できるとは扱わない。
- [HUMAN_TURN運用証拠](2026-09-06-human-turn-operational-evidence.md)の認証保証の限定と、
  [OKF metadata検索](2026-09-07-okf-metadata-knowledge-search-plan.md)の検索・trust境界は維持する。
- 外部Go moduleを追加しない。既存の承認済み依存を未承認のものとして扱い直さない。

## 確認できた事実と残る判断

固定本家のConstruction protocolにはBuild and TestからCode Generationへ戻る条件付きloop-backがある。
Goの現在の`advance`は前方未完Stageを選ぶ処理であり、そのloop-backは未接続である。
Stage名の変更だけでユーザーが求める運用を実現できるとは説明しない。

現在のGoにはrecord lock、atomic state writer、同一Stage内のreject/revise、安全なcontext読込、
限定されたreceipt現在性検証がある。新しいworkflowへ再利用できる範囲は個別の実装計画で確定する。

採用前には、Stageを案内へ変える範囲、現在recordの正本と監査保証、合意変更の承認適用、
共有正本の採用権限、新旧writer排他と移行・rollbackを確定する必要がある。
詳細は設計文書の「採用時に必要な決定」にまとめる。未確定を理由に設計自体を未提出にせず、推奨案と代償を提示する。
