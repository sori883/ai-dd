# GitHub連携を任意のskillとして追加する

日付: 2026-09-12。状態: 直接実装依頼。

ユーザーはGitHubのIssue・PRを扱うskillの追加と作業実施を許可した。IntentやBoltの単位でIssueを作り、作業が完了したらPRを出す使い方を求めている。AI-DLC本体へ組み込まず、skillとして任意に導入する境界を明示した。

採用するのは`.agents/skills/aidlc-github/`の独立した原稿。CLI、標準配置、hook、工程、state、既存15skillは変えない。利用者はフォルダ単位で導入できる。本リポジトリの開発用github-pr-workflowと区別し、専用ラベルや自律マージ許可を他プロジェクトへ流用しない。

Intentまたは承認済みBoltのまとまりを選び、repository・Space・Intent ID、Boltではstep_idとboltを照合する。単位を二重にIssue化しない。Issue／PRの対応は本文と任意のOKF文書design/INTENT_ID/github-linksへ残し、既存の承認済み仕様やCLI管理work-logは直接編集しない。PR作成とmerge、Issue closeは別の操作として扱う。

具体的な内容、単独writer、受入条件、独立適用・review・final・GitHub checksは[実装計画](../../design/optional-github-skill-plan.md)に記録した。計画の許可は今回の直接依頼であり、旧33 Stageロードマップや前回の工程skill全体の承認ではない。

開発Issueは[#181](https://github.com/sori883/ai-dd/issues/181)。現行hookがactive・begin済みを要求するため、PRは成果確認後、Intent単位なら最後の工程、Bolt単位なら対象TDD工程のfinish前に提出する。finishで現在のUnit一覧が消去されるため、Boltの対応と成果は先に確認・保存する。既に完了済みの場合は通常の再開を確認し、外部操作のためにhookを外さない。対応表も検証範囲内なら変更後の検証・承認を揃える。

開始時のmainはPR #180のabd08b8082bac4becbd4b3fc8faa1e0aee4b3466、Open Issue・PRは各0件。既存Issue #27／PR #28は開発用GitHub skillの履歴で、新しい利用者向け任意skillとは対象が異なる。元checkoutは保持し、ai-dd-namingのcodex/optional-github-skillで実施する。

## 作成した内容と確認範囲

入口SKILLと作業単位・GitHub操作・PR提出の3参照文書、任意導入の案内を作成した。標準配布、Goコード、依存、CI、既存skillは変更していない。

独立した担当2名が、コピーしたskillだけで架空案件を適用した。既存Issue／PRの再利用、異なるSpaceや実行回の区別、通信結果不明・対応表の部分保存、未完了Unitと古いSHA、親Intentの誤close、完了済みIntentのhook回避をしない手順を確認した。実GitHubへの作成試験やhook実機検証ではない。

適用で見えた曖昧さを解消し、PRで確認するUnitは対象repositoryのIssueに含む集合と明記した。リポジトリ間の依存・全体検証は別途満たす。Issue開始時のbase/head未確定は記載して残せるが、commit/push・PR作成前に確定する。OKFの保存故障は現物・診断・現行helpを確認し、存在しない復旧コマンドや保存成功を作らない。

loopのskill validator、単独配置と相対参照の確認は成功した。固定差分の独立review、read-only final、GitHub checksとmerge結果は、Issueに紐づくPRの検証欄へ記録する。
