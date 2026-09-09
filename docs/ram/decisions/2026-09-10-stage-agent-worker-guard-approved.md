# 担当起動制限の推奨案を承認し、G0から進める

状態: Accepted（推奨する運用方針とG0先行の実施）。2026-09-10。

ユーザーは[計画](../../design/stage-agent-worker-guard-plan.md)と確認中の3点の提示後に「変更してください。承認。」と回答した。提示した推奨案への承認として、次の方針で進めることを会話で明示した。

1. 途中の計画変更でも担当を呼べるよう、stage-plannerを各段階の既存agentsへ明示追加する。担当一覧の二重管理はしない。
2. Unitに分割しない子workerも、Unitなしの実行予約で扱う。メインAI自身の編集をworker人数に数えない。
3. 調整rootを一つに集約し、その全Space/Intent/sessionを横断して同一worker rootを検査する。別worktreeの並列実装を維持する。複数調整rootや複数PCを横断する分散排他へ拡張しない。

この回答は旧版ロードマップの包括承認ではなく今回の直接承認である。まずG0の固定Codex実機検証用fixtureを実装・review・検証する。[G0の具体計画](../../design/agent-guard-preflight-work-unit.md)は、承認された前提確認のファイル所有・順序・検証を具体化する。コード変更前にIssueを作成する。

G0成功を製品機能の完成と扱わない。起動要求・実agent_id・予定worktreeを構造化情報で結べない、必要な再開経路を捕捉できない、停止証拠を確定できない場合は製品guardへ進まず、結果と代案を提示する。専用scheduler、root推測、保証範囲の黙った縮小、未知の永続schemaや公開復旧APIの採用は今回の承認に含めない。G0後に契約を確定する必要がある。

既存ユーザー設定・hooks・認証情報・利用先データを変更しない。実験は既存Codex CLI 0.153.4と一時プロジェクトで行い、外部Go moduleやtoolを導入しない。コードwriterは1名、独立reviewと固定HEADのread-only finalを維持する。以前の[計画のみの記録](2026-09-10-stage-agent-worker-guard-planning.md)は当時の状態として保持する。
