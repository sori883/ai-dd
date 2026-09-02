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

## 検証mode

| Mode | 使用時点 | 実行する検証 | 所有者 |
| --- | --- | --- | --- |
| `loop` | 実装、refactor、review finding修正 | 最小のtargeted test | `go_tdd_implementer` |
| `review` | 固定base/headのreview・再review | findingの再現・解消に必要なtargeted testまたは診断 | `independent_reviewer` |
| `final` | blocking findingがなく差分が安定した後 | 計画に定めた全検証を1回 | 親エージェント |

handoffにmodeがなければ`loop`とする。個々のTDD sliceやreview finding修正の完了を、`final`の開始条件と
解釈しない。`final`は親エージェントだけが明示的に開始し、必要な場合は実行自体をimplementerへ委譲できる。

## 標準フロー

1. 親エージェントが要件と制約を整理する。
2. 不確かなAPI、ライブラリ、互換性、設計判断を `technical_researcher` へ委譲する。独立した読み取り調査だけを並列化できる。
3. `project_planner` がリポジトリと調査結果を統合し、スコープ、対象ファイル、TDD slice、検証、リスクを提示する。
4. ユーザーが計画を明示承認する。承認前はコード、設定、Issue、PRを変更しない。
5. 親エージェントが `github-pr-workflow` skillを使い、主要な成果に対応する分類ラベルを
   1つ付けて日本語のGitHub Issueを作成し、番号と承認済み計画を `go_tdd_implementer` に渡す。
6. 実装担当が`loop`で1 behaviorずつREDを観測し、最小GREEN、GREEN上のrefactorを繰り返す。
7. `independent_reviewer` が`review`で固定したbase/headを読み取り専用でレビューする。
8. P0/P1、受入条件違反、またはblockingなテスト不足を実装担当へ`loop`で戻す。修正中はtargeted test、
   再reviewも`review`のtargeted確認に留める。
9. blocking findingがなく差分が安定したら、親エージェントが`final`を開始し、承認済み計画の全検証を
   1回実行する。検証後に対象ファイルが変わった場合は証拠をstaleとして`loop`へ戻す。
10. 親エージェントが最終検証を確認し、`github-pr-workflow` skillを使ってIssueへ紐づく
   日本語のPRを作る。マージはユーザーの判断に委ね、自動マージを設定せず、マージ後に
   作業完了を確認してIssueを閉じる。

## GitHubを実装記録として使う

Issue・PRの作成、更新、参照には
`.agents/skills/github-pr-workflow/SKILL.md`を使用する。

- 現在の実装状況は、default branchへマージ済みのPRを第一の履歴根拠とする。
- Issueには `機能開発` または `ユーザーリクエスト` の分類ラベルを主要な成果に応じて1つ付け、
  両方を同時に付けない。`bug`や`documentation`等の補助ラベルは必要に応じて併用できる。
- Draft、Open、未マージのClosed PRは、default branchの実装済み機能に含めない。
- マージ済みPRは導入時点の証拠であるため、後続PR、revert、現在のコードとテストを照合して
  現在も有効か判断する。
- 新しいPRには、対応Issue、実装した機能、検証、レビューを日本語で残す。通常の未実装機能や
  「今回は作らないもの」は列挙せず、特殊な利用条件・互換性・安全性の制約だけを必要時に残す。
- コード識別子や既存タイトルの原文などを除き、Issue・PRとユーザー向け報告は日本語を基本とする。

## Gateと停止条件

- 実装開始には、明示承認済み計画、GitHub Issue、受入条件、変更範囲が必要。
- 外部Go moduleまたは外部Go toolが必要になったら、標準ライブラリで代替できない理由を示して停止し、ユーザー承認を待つ。
- Context7またはSerenaが利用できない場合は、安全な組み込み手段と一次資料へfallbackし、利用できなかったMCPと影響を報告する。
- reviewerはworking treeを変更しない。修正は親を経由してimplementerへ戻す。
- read-only agentはSerenaの変更系toolsもagent設定で無効化し、MCP経由の書き込みを防ぐ。
- サブエージェントはさらにサブエージェントを起動しない。追加の並列化が必要なら親が境界と所有権を定義する。

## Handoff契約

すべての実装・review handoffは`verification_mode`を含める。技術調査は、質問、対象version、事実、推論、
選択肢、推奨、未知事項、参照URLを返す。計画は、scope、non-goals、設計、対象ファイル、TDD slice、
targeted検証、final検証、リスク、未決事項、承認gateを返す。実装は、`loop`ではRED/GREENのtargeted
コマンド証拠、変更ファイル、refactor、残余リスクを返し、`final`では全検証のfresh evidenceを返す。
レビューは、severity、file/line、発生条件、影響、根拠、最小修正と、`final`へ進めるかを返す。

## 検証

skillは `skill-creator` の `quick_validate.py`、agentとproject configはTOML parserおよび `codex features list` による設定ロードで検証する。動作確認ではplanner、researcher、reviewerがファイルを変更しないこと、implementerが承認済み計画とIssueなしでは停止することを確認する。

`loop`と`review`では変更package・findingに対応するtargeted testだけを実行する。全package test、race、
vet、全体lint、cross compile、配布E2Eを途中のhandoffごとに実行しない。

`final`では、Go moduleが存在する実装なら`go test ./...`、`go vet ./...`、必要な場合の
`go test -race ./...`、`gofmt`、`git diff --check`と、計画に含まれるcross compile・配布E2E等を
1回実行する。skill・agent設定の変更では、上記validatorと設定ロードを`final`へ集約する。

## 参考資料

- [OpenAI: Subagents](https://learn.chatgpt.com/docs/agent-configuration/subagents)
- [OpenAI: Build skills](https://learn.chatgpt.com/docs/build-skills)
- [Kent Beck: Canon TDD](https://newsletter.kentbeck.com/p/canon-tdd)
- [Go: Add a test](https://go.dev/doc/tutorial/add-a-test)
- [Go: Data Race Detector](https://go.dev/doc/articles/race_detector)
- [Google Engineering Practices: What to look for in a code review](https://google.github.io/eng-practices/review/reviewer/looking-for.html)
