# 現行製品の入口と利用者ガイドを整える計画

## 背景と利用者が得る結果

READMEはCLI基盤とhelp/versionだけを説明し、architectureには旧4段階や大文字ADRの記載が残っている。
現在の製品は6種類のステージ、5種類の専門担当、OKF文書、Sensor、独立レビュー、会話承認を持つ。
初めて読む人がREADMEから導入と初回依頼へ進み、承認する内容、保存先、困ったときのhelpを
見つけられるように、短いREADMEと一つの利用者ガイドへ整理する。

ユーザーの「実案件→配布・更新→利用者文書を順に進める」という直接依頼の3番目である。
実案件はPR #164、配布の整備はPR #166でmainへ反映済み。Issue #163・#165は完了した。
開始基準はmain `eac541fa439da6533c2d4173eaec6500654ded4c`。
実装管理は[Issue #167](https://github.com/sori883/ai-dd/issues/167)。
現行挙動の説明を直す作業であり、製品コード・設定・保存契約は変更しない。
旧33 Stageロードマップの包括承認は使用しない。過去の意思決定本文は履歴として保持する。

## 対象fileと単独writer

作業場所は `/Users/const/sori883/ai-dd-user-guide`、branchは`codex/user-guide`。
親AIがdocsの単独writerとなり、実装担当との同時編集を行わない。元checkoutとpilot dataを保全する。

| file | 変更内容 |
| --- | --- |
| README.md | 目的、必要環境、配布手順、初回利用、開発資料への短い入口 |
| src/docs/user-guide.md（新規） | 初回利用、工程と担当、承認、記録先、help、再開の案内 |
| docs/architecture.md | 旧4段階・大文字ADR・古い契約の現行扱いを補正 |
| docs/e2e-testing.md | 冒頭に旧方式の検証履歴であることと現行検証へのリンクを追加 |
| docs/ram/README.md | 現行入口と今回記録を案内し、過去要約と区別 |
| docs/design/user-guide-plan.md、今回RAM | 依頼、範囲、検証結果を記録 |

docs/distribution.mdとdocs/development.mdは配布整備の成果へリンクする。
製品原稿の配置規則に従い、新規利用者ガイドはsrc/docs/へ置く。

## 説明する契約

- Spaceは案件と知識をまとめる領域、Intentは一つの目的を持つ案件、stageは作業段階。
- initializationとdiscoveryは必須。構成分析・実装計画・TDD・統合検証は目的整理時に採否と順序を
  提案し、ユーザーが実行計画を承認する。途中変更も承認する。6段階固定順とは説明しない。
- メインAIはユーザーとの対話、共有記録、許可された専門担当の起動を担当する。
  researcher、requirements、stage-planner、worker、reviewerの5担当を説明する。
  stage-plannerは段階の採否・順序を提案し、実装計画全文の作成と混同しない。
- agentはメインAIの標準ツールで起動する。CLIは状態・文書・割当を管理する。
  Unitへ分けられる実装は担当範囲と別worktreeを割り当てて並列に進める。
- Sensorはプログラムによる開始・終了条件の検査、独立reviewは成果内容の確認。
  計画承認と各実行回の成果承認を区別し、Sensor pass、独立review、明示回答、finishの役割を説明する。
  今回のpilotだけに委任したAI試験用承認を、通常利用の自動承認として案内しない。
- knowledge/codekbは現行の何をどう行うか、knowledge/design/<intent_id>は要件と実装計画、
  knowledge/adrはアーキテクチャ判断の理由、knowledge/rules/rule.mdはプロジェクト共通ルール、
  knowledge/log/<intent_id>-work-log.mdは差戻し理由を保存する。
- OKFは検索・識別のmetadataを持つMarkdown。AIがmemory CLIでfrontmatterを生成・更新し、
  本文を整備する。日時の手入力を案内しない。intent_idで関連文書を探せる。
- 進捗はintents/<id>/state.jsonと状態変更履歴、機械ローカルの会話・予約はaidlc/.runtimeへ保存。
  全操作auditとは説明しない。本製品開発用docs/ramと利用者のKnowledgeを区別する。
- 正確な引数・型・JSONは利用中binaryのhelp、現在回の入出力・担当はintent procedureを参照する。
  guideに大きなJSON契約を複製しない。保存失敗や中断は同じIntentのstateと現在helpから再開する。

## 本家・公開境界

本家AI-DLCは固定2.6.123の確認済み範囲を参照する。Go単一binary、最小工程、OKFなどの既承認の
製品契約を説明し、新しい意図的差分は採用しない。公開版・ライセンス・未実測OSの動作保証は決めない。
配布候補の生成、archiveのnative実行、Codexでの実hook動作を区別し、配布文書の確認範囲へリンクする。

## 検証と完了条件

文書だけの変更なので新Go testや人工REDは作らない。
loopではリンク・見出し・保存先・コマンド名を現物へ照合し、git diff --checkを行う。
専用の独立review担当がread-onlyで、初心者の導線と現行契約への一致を確認する。

差分安定後のfinalはread-onlyで次を一度実施する。

1. 追加/変更したMarkdownの相対リンクと見出しリンク、関連fileの存在を確認する。
2. 配布整備で検証済みのbinaryを使い、掲載したversion/helpを実行して終了codeとstdoutを確認する。
3. 実データ確認を伴う例は必要な範囲だけ新しい隔離fixtureで実行する。
4. 対象fileのbytesとGit状態が変わっていないこと、git diff --checkを確認する。

この文書作業のために全package/raceをローカルで繰り返さない。PRで起動する既存GitHub checksは
すべて成功するまで待つ。独立review、final、対象checks後に通常のmerge方式でmain反映とIssue closeを確認する。
