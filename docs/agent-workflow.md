# カスタムエージェント運用

このリポジトリでは、計画、技術調査、Go実装、レビューを別のCodex custom agentへ分離する。親エージェントは要件、承認、GitHub、handoffを管理し、同じ作業ツリーを同時に編集する実装担当は常に1つとする。

## 役割

| Agent | 主な入力 | 出力 | 権限 |
| --- | --- | --- | --- |
| `project_planner` | ユーザー要件、リポジトリ、調査報告 | 承認可能な実装計画 | read-only |
| `technical_researcher` | 明確な調査質問、対象version、制約 | 一次資料に基づく判断材料 | read-only |
| `go_tdd_implementer` | 承認済み計画、Issue、受入条件 | Red-Green-Refactorの実装と検証証拠 | workspace-write |
| `independent_reviewer` | base/head、計画、Issue、差分 | 優先度付きfindingと残余リスク | read-only |

各agentは同名の役割skillを明示的に使用する。Go担当は追加で `golang-how-to` を読み、タスクに必要なGo skillsだけを選ぶ。

## 標準フロー

1. 親エージェントが要件と制約を整理する。
2. 不確かなAPI、ライブラリ、互換性、設計判断を `technical_researcher` へ委譲する。独立した読み取り調査だけを並列化できる。
3. `project_planner` がリポジトリと調査結果を統合し、スコープ、対象ファイル、TDD slice、検証、リスクを提示する。
4. ユーザーが計画を明示承認する。承認前はコード、設定、Issue、PRを変更しない。
5. 親エージェントがGitHub Issueを作成し、番号と承認済み計画を `go_tdd_implementer` に渡す。
6. 実装担当が1 behaviorずつREDを観測し、最小GREEN、GREEN上のrefactorを繰り返す。
7. `independent_reviewer` が固定したbase/headを読み取り専用でレビューする。
8. P0/P1、受入条件違反、またはblockingなテスト不足を実装担当へ戻し、修正後に再レビューする。
9. 親エージェントが最終検証を確認し、Issueへ紐づくPRを作る。マージはユーザーの判断に委ね、マージ後にIssueを閉じる。

## Gateと停止条件

- 実装開始には、明示承認済み計画、GitHub Issue、受入条件、変更範囲が必要。
- 外部Go moduleまたは外部Go toolが必要になったら、標準ライブラリで代替できない理由を示して停止し、ユーザー承認を待つ。
- Context7またはSerenaが利用できない場合は、安全な組み込み手段と一次資料へfallbackし、利用できなかったMCPと影響を報告する。
- reviewerはworking treeを変更しない。修正は親を経由してimplementerへ戻す。
- read-only agentはSerenaの変更系toolsもagent設定で無効化し、MCP経由の書き込みを防ぐ。
- サブエージェントはさらにサブエージェントを起動しない。追加の並列化が必要なら親が境界と所有権を定義する。

## Handoff契約

技術調査は、質問、対象version、事実、推論、選択肢、推奨、未知事項、参照URLを返す。計画は、scope、non-goals、設計、対象ファイル、TDD slice、検証、リスク、未決事項、承認gateを返す。実装は、RED/GREENのコマンド証拠、変更ファイル、refactor、最終検証、残余リスクを返す。レビューは、severity、file/line、発生条件、影響、根拠、最小修正をfindingごとに返す。

## 検証

skillは `skill-creator` の `quick_validate.py`、agentとproject configはTOML parserおよび `codex features list` による設定ロードで検証する。動作確認ではplanner、researcher、reviewerがファイルを変更しないこと、implementerが承認済み計画とIssueなしでは停止することを確認する。

Go moduleが存在する実装では、変更packageのtargeted test、`go test ./...`、`go vet ./...`、必要な場合の `go test -race ./...`、`gofmt`、`git diff --check` を完了条件とする。

## 参考資料

- [OpenAI: Subagents](https://learn.chatgpt.com/docs/agent-configuration/subagents)
- [OpenAI: Build skills](https://learn.chatgpt.com/docs/build-skills)
- [Kent Beck: Canon TDD](https://newsletter.kentbeck.com/p/canon-tdd)
- [Go: Add a test](https://go.dev/doc/tutorial/add-a-test)
- [Go: Data Race Detector](https://go.dev/doc/articles/race_detector)
- [Google Engineering Practices: What to look for in a code review](https://google.github.io/eng-practices/review/reviewer/looking-for.html)
