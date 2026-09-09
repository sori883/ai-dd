---
name: aidlc
description: 目的の理解、実装計画、TDD、統合検証を、現在の状態と独立レビューで進める。
---
# 作業の入口

このプロジェクトでは @@BINARY@@ を使う。以下ではこの実行ファイルを `A` と記す。
SessionStartが示すsession IDとdraftパスを使い、別の実行ファイルへ置き換えない。
引数・型・値に迷ったら `A help`、`A memory create --help`、`A memory update --help` を参照する。
正規helpは未選択・Rule未読・待機中でも読める。本文だけの草稿を作り、metadataはCLI引数へ渡す。

最初は `A intent create NAME --space default` で目的を作る。
再開時は `A intent list --space default` で既存IDを確認する。各会話で
`A intent switch --id ID --space default --session SESSION` または
`A session bind ID --space default --session SESSION` を実行する。
出力には現在のstate、必須Rule全文、draftパスがある。Ruleを読み、その内容に従う。
この入口の読込だけでRuleを読んだことにはならない。

選択後は `A intent procedure ID --space SPACE` を実行し、返された現在段階の手順全文と文書参照に従う。
未開始・質問待ち・中断中でも読める。段階変更・再開後は取り直す。共通操作の索引はWORKFLOW.md、正確な引数・型・値は各CLI helpにある。
手順の欠落や定義変更は停止して診断する。元定義を復元するか新Intentを作り、旧passを流用しない。
質問待ち・中断後は `A intent resume ID --space SPACE --expect R --reason TEXT`、
差戻しは `A intent reopen ID --space SPACE --expect R --reason TEXT --step STEP_ID`。
差戻し保存失敗時は同一要求だけを再試行し、work-logの改変時は元版へ復元する。
現在のSensorと独立reviewのpassを確認して前進する。調整役が担当を起動し共有state/文書を保存する。

非同期toolは終端までpollする。編集が失敗終了しPostが来なかったことを確認した場合だけ、
同じID/Space/sessionで `A session bind ID --space SPACE --session SESSION --recover` を実行し、
再試行して検証する。実行中の処理を推測で解除しない。Stopは作業全体の完了ではない。

製品の5担当はaidlc-requirements、aidlc-researcher、aidlc-stage-planner、aidlc-worker、aidlc-reviewer。
メインAIはdiscoveryの要件整理・調査結果が揃った後、および途中の計画変更時にaidlc-stage-plannerを呼ぶ。
担当はread-onlyでステージ採否・順序・PLAN案を返す。メインAIが案を説明してユーザー承認を受け、共有保存を行う。
期待する文書はなければ「なし」とし、プログラム・テストコード・commitを文書outputsへ列挙しない。検証証拠の必要性は別に説明する。
