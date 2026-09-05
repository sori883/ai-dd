# カスタムエージェント運用

このリポジトリでは、計画、技術調査、Go実装、レビューを別のCodex custom agentへ分離する。親エージェントは要件、実装許可、GitHub、handoffを管理し、同じ作業ツリーを同時に編集する実装担当は常に1つとする。

## 役割

| Agent | 主な入力 | 出力 | 権限 |
| --- | --- | --- | --- |
| `project_planner` | ユーザー要件、リポジトリ、調査報告 | 許可判定可能な実装計画 | read-only |
| `technical_researcher` | 明確な調査質問、対象version、制約 | 一次資料に基づく判断材料 | read-only |
| `go_tdd_implementer` | 実装許可のある計画、Issue、順序付きTDD work unit | 全項目のRED／GREEN証拠と実装結果 | workspace-write |
| `independent_reviewer` | base/head、計画、Issue、差分 | 優先度付きfindingと残余リスク | read-only |

各agentは同名の役割skillを明示的に使用する。Go担当は追加で `golang-how-to` を読み、タスクに必要なGo skillsだけを選ぶ。

## 起動判断とコンテキスト予算

サブエージェントは個別のモデル実行として入力tokenを消費するため、親エージェントは役割が存在することを
理由に自動起動せず、委譲によって品質または所要時間が実質的に改善するかを先に判断する。

- `technical_researcher` は、外部仕様、対象version、API、互換性、設計選択に未解決の不確実性がある場合に
  起動する。モデルは `.codex/agents/technical-researcher.toml` に固定した `gpt-5.6-terra`、推論強度は
  `medium` とし、起動時に別の値で上書きしない。
- `project_planner` は、複数package、重要な設計判断、複数のTDD slice、移行または安全性の境界を含み、
  独立した計画レビューが有効な場合に起動する。小さく定型的な変更は親エージェントが計画できる。
- 実装と独立reviewは、実装許可のある計画とリポジトリ規則が要求するgateに従う。起動数を減らすことを理由に、
  必須の書き込み所有権、review、最終検証を省略しない。

起動時の会話継承は `fork_turns="none"` を既定とする。親エージェントは会話全文を複製せず、次の情報を
短い自己完結したhandoffとして渡す。

1. 依頼の目的、担当する具体的な範囲、期待する出力。
2. 実装計画、実装許可の根拠、GitHub Issue。該当する場合はbase/headと変更対象ファイル。
3. 適用するRAM、仕様、skill、`AGENTS.md`への参照。
4. 既に確定した制約、未解決事項、外部依存の承認状態。
5. 書き込みの所有権と、同じ作業ツリーに他の作業者がいる場合の注意。
6. 実装・reviewの場合は `verification_mode` と必要な検証範囲。Goのloopはさらに`work_unit_id`、
   順序付きslice一覧、work unit全体の所有file、各sliceの正確なtargeted commandを渡す。

サブエージェントはhandoffに列挙された参照先から必要な箇所だけを読み、親は長いファイル本文やコマンド
出力を依頼文へ重複掲載しない。最近の会話を継承する正の `fork_turns`、または全文を継承する
`fork_turns="all"` は、会話そのものがレビュー対象などの不可分な入力で、短いhandoffやIssue・RAMでは
意味を保てない場合だけ使用できる。その場合、親エージェントは継承が必要な理由を起動依頼に明記する。

## 検証mode

| Mode | 使用時点 | 実行する検証 | 所有者 |
| --- | --- | --- | --- |
| `loop` | 実装、refactor、review finding修正 | 最小のtargeted test | `go_tdd_implementer` |
| `review` | 固定base/headのreview・再review | findingの再現・解消に必要なtargeted testまたは診断 | `independent_reviewer` |
| `final` | blocking findingがなく差分が安定した後 | 計画に定めた全検証を1回 | 親エージェント |

handoffにmodeがなければ`loop`とする。agentの役割とmodeが一致しなければ暗黙に昇格せず停止する。
reviewerは明示された`review`だけ、implementerは`loop`または親が明示した`final`だけを受理する。
個々のTDD sliceやreview finding修正の完了を、`final`の開始条件と解釈しない。`final`は親エージェント
だけが明示的に開始し、必要な場合は実行自体をimplementerへ委譲できる。

Goのloopは[作業単位handoff](tdd-handoff.md)に従い、承認済みの複数sliceを1つのwork unitとして渡す。
担当は各sliceでtestを先に追加し、runnableなRED、最小GREEN、refactorを順番に完了してから一度返す。
親はwork unit末尾に全差分とtargeted test群を一度再確認し、項目ごとの返却や旧`tdd_phase`／
`red_acceptance`を要求しない。1 Issue／PRの全実装項目を既定の単位とし、slice数だけを理由に分割しない。
途中連絡の有無を進行gateにしない。

Go codeの`gofmt`適用は実装変更として`loop`中かつreview前に完了させ、適用後のtargeted testを確認する。
固定headのreview後に行う`final`はread-onlyとし、`gofmt -l`等でformatを確認するだけでファイルを変更しない。

## 標準フロー

1. 親エージェントが要件と制約を整理する。
2. 不確かなAPI、ライブラリ、互換性、設計判断がある場合だけ `technical_researcher` へ委譲する。独立した読み取り調査だけを並列化できる。
3. 複雑な変更では `project_planner` がリポジトリと調査結果を統合し、スコープ、対象ファイル、TDD slice、検証、リスクを提示する。それ以外は親エージェントが同じ計画gateを満たす。
4. 親エージェントが計画の実装許可を判定する。計画自体が明示承認済みか、承認済みの包括承認枠に
   完全に収まる場合は進める。どちらでもない場合は、コード、設定、Issue、PRを変更する前に計画を
   ユーザーへ提示して承認を待つ。
5. 親エージェントが `github-pr-workflow` skillを使い、主要な成果に対応する分類ラベルを
   1つ付けて日本語のGitHub Issueを作成し、番号、計画、実装許可の根拠を `go_tdd_implementer` に渡す。
6. 親が`loop`のwork unitを1回依頼する。担当は各behaviorについてtestを先に追加し、compile可能な
   意図したRED、最小GREEN、GREEN上のrefactorを順に行い、全behavior完了時またはblocking条件で
   一度返す。初回GREENは`ALREADY_GREEN`と記録し、compile failureをREDと偽らない。親は全差分と
   work unitのtargeted test群を一度再確認する。
7. `independent_reviewer` が`review`で固定したbase/headを読み取り専用でレビューする。
8. P0/P1、受入条件違反、またはblockingなテスト不足を一つのrepair work unitへまとめ、実装担当へ
   `loop`で戻す。観測可能な修正は回帰testのREDを先行させ、文書・設定だけの修正に人工REDを作らない。
   親のboundary確認後、まとめて一度再reviewする。
9. blocking findingがなく差分が安定したら、親エージェントが`final`を開始し、計画に定めた全検証を
   1回実行する。検証後に対象ファイルが変わった場合は証拠をstaleとして`loop`へ戻す。
10. 親エージェントが最終検証を確認し、`github-pr-workflow` skillを使ってIssueへ紐づく
   日本語のPRを作る。ユーザーがmanual mergeを指定していなければ、対象workflowのchecksが出現して
   すべて成功したことを確認し、repositoryの既存方式で自律的にmergeする。checksが想定されない変更は、
   workflowのpath条件とfinal証拠から理由を記録する。merge後にdefault branchへの反映と作業完了を
   確認してIssueを閉じる。

## 実装許可と包括承認枠

実装計画は、次のどちらかを満たすと「実装許可あり」とする。

1. ユーザーがその計画を直接、明示承認している。
2. ユーザーが承認したroadmapまたはmilestoneの範囲、準拠根拠、重要な境界に計画全体が収まっている。

後者を包括承認枠と呼ぶ。詳細計画とIssueはPRごとの小さな範囲を固定するために毎回作成するが、
包括承認枠内なら追加の返答を待たない。計画、Issue、handoffには、根拠となるRAMを記載する。

現在の最初の包括承認枠は、
`docs/ram/decisions/2026-09-03-aidlc-implementation-roadmap.md`の残作業である。後続RAMが旧決定を
置換している場合は後続決定を優先する。本家準拠は、リポジトリ固定snapshotの実際に確認した範囲を
根拠とし、最新upstreamとの一致を推測しない。

次の場合は包括承認枠から自動的に許可せず、親エージェントが選択肢と影響をまとめてユーザーへ確認する。

- 本家の根拠が曖昧・矛盾し、結果が異なる複数案を一意に選べない。
- 新しい意図的な本家との差分、または包括承認枠外の変更が必要である。
- 公開API、永続data、互換性、移行、安全性、権限、運用へ重大な判断がある。
- 外部Go module、有料service、認証情報、追加権限、不可逆操作が必要である。
- testまたはreviewの問題を、安全に一意な修正へ落とせない。

Issue範囲内の通常の実装判断、明らかなbug、review findingの修正は追加承認を要しない。PRの粒度、
単独writer、TDD、独立review、final検証は包括承認によって省略しない。

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

- 実装開始には、直接承認または包括承認枠による実装許可、GitHub Issue、受入条件、変更範囲が必要。
- 外部Go moduleまたは外部Go toolが必要になったら、標準ライブラリで代替できない理由を示して停止し、ユーザー承認を待つ。
- Context7またはSerenaが利用できない場合は、安全な組み込み手段と一次資料へfallbackし、利用できなかったMCPと影響を報告する。
- reviewerはworking treeを変更しない。修正は親を経由してimplementerへ戻す。
- read-only agentはSerenaの変更系toolsもagent設定で無効化し、MCP経由の書き込みを防ぐ。
- サブエージェントはさらにサブエージェントを起動しない。追加の並列化が必要なら親が境界と所有権を定義する。

## Handoff契約

すべての実装・review handoffは`verification_mode`を含める。技術調査は、質問、対象version、事実、推論、
選択肢、推奨、未知事項、参照URLを返す。計画は、scope、non-goals、設計、対象ファイル、TDD slice、
targeted検証、final検証、リスク、未決事項、実装許可の根拠または承認gateを返す。実装は、`loop`ではRED/GREENのtargeted
コマンド証拠、変更ファイル、refactor、残余リスクを返し、`final`では全検証のfresh evidenceを返す。
レビューは、severity、file/line、発生条件、影響、根拠、最小修正と、`final`へ進めるかを返す。
handoffは上記のコンテキスト予算に従い、会話履歴の代わりに永続資料への具体的な参照を渡す。

Goのloopで必要な入力、work unit内のtest-first順序、証拠、停止条件、終了statusは
[作業単位handoff](tdd-handoff.md)を正本とする。親と実装担当は最初のloop依頼前に同文書を読み、
loopのtargeted確認と独立review後のfinal検証を混同しない。

## 検証

skillは `skill-creator` の `quick_validate.py`、agentとproject configはTOML parserおよび `codex features list` による設定ロードで検証する。動作確認ではplanner、researcher、reviewerがファイルを変更しないこと、implementerが実装許可のある計画とIssueなしでは停止することを確認する。

`loop`と`review`では変更package・findingに対応するtargeted testだけを実行する。全package test、race、
vet、全体lint、cross compile、配布E2Eを途中のhandoffごとに実行しない。

`final`では、Go moduleが存在する実装なら`go test ./...`、`go vet ./...`、必要な場合の
`go test -race ./...`、`gofmt -l`、`git diff --check`と、計画に含まれるcross compile・配布E2E等を
1回実行する。skill・agent設定の変更では、上記validatorと設定ロードを`final`へ集約する。

## 参考資料

- [OpenAI: Subagents](https://learn.chatgpt.com/docs/agent-configuration/subagents)
- [OpenAI: Build skills](https://learn.chatgpt.com/docs/build-skills)
- [Kent Beck: Canon TDD](https://newsletter.kentbeck.com/p/canon-tdd)
- [Go: Add a test](https://go.dev/doc/tutorial/add-a-test)
- [Go: Data Race Detector](https://go.dev/doc/articles/race_detector)
- [Google Engineering Practices: What to look for in a code review](https://google.github.io/eng-practices/review/reviewer/looking-for.html)
