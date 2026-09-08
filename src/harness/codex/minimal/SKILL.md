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

選択後、`cat .agents/skills/aidlc/WORKFLOW.md` を単独で実行し、配置済み手順を**全文**読む。
完了・質問待ち・中断中でも、この手順読込はできる。現在stateのrevisionを `R` とする。
質問待ち・中断後は `A intent resume ID --space SPACE --expect R --reason TEXT`、
完了後の再検証は `A intent reopen ID --space SPACE --expect R --reason TEXT --stage STAGE`。
手順とRuleを読んでから明示的に再開し、通常作業へ進む。
欠落時は診断し、過去の手順や内包された原稿で補わない。
詳細手順にはconfig、Sensor、独立レビュー、Unit、Knowledge/ADRの正確な操作を記載している。
段階は discovery → planning → tdd → integration。`intent review` とSensorの現在の合格が
各境界に必要で、`unit claim` は担当割当だけを行う。調整役AIがworker/reviewerを起動する。

非同期toolは終端までpollする。編集が失敗終了しPostが来なかったことを確認した場合だけ、
同じID/Space/sessionで `A session bind ID --space SPACE --session SESSION --recover` を実行し、
再試行して検証する。実行中の処理を推測で解除しない。Stopは作業全体の完了ではない。
