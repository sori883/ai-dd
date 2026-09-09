# プロジェクトRuleとAI-DLCスキルの責任を分ける

状態: Accepted（役割分離と実装への直接承認）。具体的な実装手順は関連計画へ記録する。

## 合意と背景

現在のknowledge/rules/rule.mdにはAI-DLCの工程・承認・記録規約があり、WORKFLOW.mdにも操作案内や同じ規約が重複している。ユーザーはrule.mdへ利用プロジェクトの共通ルールを入れたいと指定した。言語、命名、設計制約などの利用プロジェクト固有ルールをrule.mdへ置き、AI-DLC固有ルールとCLI案内をスキルへ分ける案を提示し、ユーザーは実装を依頼した。

## 承認した責任分担

- knowledge/rules/rule.md: 利用プロジェクトの共通ルール。
- aidlcスキル: 現在Intent/ステージの確認、担当起動、人間承認、Knowledge/ADR/stateの使い分けなどAI-DLCの進め方。
- aidlc-cliスキル: CLIのコマンド選択とhelp参照、操作上必要な案内。正確な引数・型・値・JSON例はCLI helpを正本とする。
- ステージ定義: 各工程の手順、入力、文書outputs、Sensor。
- エージェント定義: 各担当の責任・権限。

WORKFLOW.mdは必要な内容を適切な定義へ移して参照を更新した後、製品原稿・新配布から廃止する。メインAIはプロジェクトRuleを読み、専属担当へ必要なRuleと依頼を渡す。Sensor・承認のプログラム検査は維持する。文書outputsは文書のみ、なし可。ADRは設計判断のwhy、Knowledgeは現行what/how、進捗はstate、差戻し理由はknowledge/logのwork-logへCLIが保存する。

## 実装許可と境界

この責任分離を実現するスキル/原稿、配置・参照とその検査、テスト、文書更新は今回の直接承認内で進める。既存ユーザーRule/配置/stateを自動移行・上書きしない。Go単一binary・外部Go module追加なし。ユーザープロジェクトの言語や設計方針をAI-DLCが勝手に決定しない。新しい承認・Sensor条件の緩和は承認に含めない。

本記録はsrc/core/minimal/knowledge/rules/rule.mdにAI-DLC運用規約を持たせる従来配置を置き換える。過去RAMは履歴として残し、過去の製品ルール自体の意味は対応するスキル/工程定義へ保持する。

具体的な対象、Rule初期本文・固定path、配布・移転・hook・検証の根拠は[実装計画](../../design/project-rule-and-cli-skills-plan.md)に記録した。
