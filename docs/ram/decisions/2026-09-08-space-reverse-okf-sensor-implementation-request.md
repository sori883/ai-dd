# Space入口・資材解析・OKF Sensorの実装依頼

状態: 構成への同意と実装依頼を受領。Sensorの合否を変える2点は回答待ち。
ユーザーは「実装してほしい」「内容については問題ない」と回答した。
前記の整理・説明のみという作業範囲を、今回の実装依頼が後続する。
[前回の整理](2026-09-08-space-reverse-okf-sensor-request.md)の大枠を採用する意向であり、
未確定の具体的な保存契約や鮮度の選択まで回答済みとは扱わない。

## 進める内容

既存Space作成を入口から使い、4段階のdiscovery内で必要に応じて既存資材を解析する。
調整役AIがOKFを検索し、現状解析・要件等をCLIで作成更新する。
Sensorは必要な文書・形式・Intent ID・更新の鮮度・参照対象を検査し、独立reviewが内容を確認する。
Go単一binary、既存agent役割、KnowledgeのWhat/HowとADRのWhy、全操作auditを要求しない境界を維持する。
配布更新・README全体整理は引き続き保留。解析だけで完了する新workflow modeは今回追加しない。

## 提示した確認

1. 新規作成/更新が必要な文書の鮮度は、文書を担当する工程の開始以降か、Intent開始以降か。
   前者を推奨。前工程の文書は後工程で参照可能にし、やり直し時の鮮度を明確にする。
2. 内容変更が不要な既存文書は参照資料にとどめ、今回の必須文書を別途作成更新するか、
   今回Intentでの再確認記録があれば既存文書も今回必須成果物に数えるか。
   前者を推奨。日時だけの更新を内容の確認に代えない。

いずれも未回答。コード/設定/Issue/PR変更前に結果を反映した自己完結計画を作る。
結果が変わる選択を根拠から一意に決められない場合に確認するAGENTS.mdの規則による。

## エージェントについての質問への確認結果

製品専用の固定agent定義はsrc/harness/codex/minimal/agents/aidlc-reviewer.toml。
調整役は通常のAI会話、workerは起動する分担担当という役割で、製品の専用定義は別にない。
この製品を開発する側にはproject_planner、technical_researcher、go_tdd_implementer、
independent_reviewer、explanatory_html_writerの設定がある。製品に全員を配布する構成ではない。
