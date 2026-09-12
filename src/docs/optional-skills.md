# 必要なプロジェクトだけで使う追加skill

`aidlc-github`は、AI-DLCのIntentやBoltをGitHub Issue・PRと対応付ける追加手順です。
本体の実行ファイルや標準15skillには含まれず、`aidlc install codex`でも自動配置されません。

## 導入

1. このリポジトリの[aidlc-githubフォルダ](../../.agents/skills/aidlc-github/SKILL.md)を、参照文書ごと取得します。
2. 利用プロジェクトの`.agents/skills/aidlc-github/`へフォルダ単位でコピーします。同名ファイルやsymlinkがあれば、内容を保全して比較してから更新します。
3. GitとGitHub CLIの`gh`を利用でき、対象repositoryの権限で認証されていることを確認します。GitHubを使わないプロジェクトには必要ありません。
4. Codexで`$aidlc-github`を指定し、対象repositoryとIssueの作業単位を伝えます。

このリポジトリを開いているCodexからは、原稿のdiscovery pathでも見つけられます。
他プロジェクトで使う場合はコピーした一つのフォルダで完結し、ai-dd開発用のgithub-pr-workflowは不要です。
配置だけでIssueやPRを公開する許可は与えません。毎Intentで使う運用にする場合は、利用者が採用を決めてプロジェクトの共通ルールへ記載します。

## 依頼の例

> このプロジェクトではaidlc-githubを使います。repositoryはexample/shopです。Intent単位で、開始時にIssueを作り、成果の検証とレビューが揃ったらPRを提出してください。マージは私が判断します。

> このIntentは承認済みのBolt単位でIssueとPRを分けてください。同じGit作業場所で順番に進め、UnitごとのIssueは作らないでください。

最初にIssueへ目的・受入条件を残し、実装・検証・独立レビュー・必要な承認を揃えてPRを出します。
本体hookの順序を保つため、Intent単位のPRは最後の工程のfinish前、Bolt単位のPRは対象TDD工程のfinish前に提出します。既にIntentを終了している場合は、通常の再開手順を確認します。
GitHubの対応先は、必要に応じて`aidlc/spaces/SPACE/knowledge/design/INTENT_ID/github-links.md`へOKFとして保存できます。
これはリンクの対応表で、進捗や承認の正本は引き続きAI-DLCのstateです。

PRの提出とmerge、Issue closeは別です。対応する作業範囲、レビューとchecks、利用先の許可を確認して進めます。
別worktreeは必須ではありません。アプリごとにGitがある構成では、repositoryごとにIssue／PRを作って対応を残します。

## 更新・取り外し

更新は取得した版と利用者の編集を比較し、一度に標準skill全体を上書きしないでください。
取り外す場合は、追加したフォルダと採用ルールだけを保全して戻します。既存Issue・PRやKnowledge、Intentのstateはそのまま残ります。
