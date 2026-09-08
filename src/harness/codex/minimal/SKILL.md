---
name: aidlc
description: 目的の理解、実装計画、TDD、統合検証を、現在の状態と独立レビューで進める。
---
# 作業の入口

このプロジェクトでは @@BINARY@@ を使う。以下ではこの実行ファイルを `A` と記す。
SessionStartが示すsession IDとdraftパスを使い、別の実行ファイルへ置き換えない。

最初は `A intent create NAME --space default` で目的を作る。
再開時は `A intent list --space default` で既存IDを確認する。各会話で
`A intent switch --id ID --space default --session SESSION` または
`A session bind ID --space default --session SESSION` を実行する。
出力には現在のstate、必須Rule全文、draftパスがある。Ruleを読み、その内容に従う。
この入口の読込だけでRuleを読んだことにはならない。

選択後、配置済み `.agents/skills/aidlc/WORKFLOW.md` を通常のfile読込で**全文**読む。
欠落時は診断し、過去の手順や内包された原稿で補わない。
詳細手順にはconfig、Sensor、独立レビュー、Unit、Knowledge/ADRの正確な操作を記載している。
段階は discovery → planning → tdd → integration。`intent review` とSensorの現在の合格が
各境界に必要で、`unit claim` は担当割当だけを行う。調整役AIがworker/reviewerを起動する。

非同期toolは終端までpollする。編集が失敗終了しPostが来なかったことを確認した場合だけ、
同じID/Space/sessionで `A session bind ID --space SPACE --session SESSION --recover` を実行し、
再試行して検証する。実行中の処理を推測で解除しない。Stopは作業全体の完了ではない。
