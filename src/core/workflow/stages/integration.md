---
stage_id: integration
agents:
  - role: independent_review
    agent: aidlc-reviewer
inputs:
  - path: ${knowledge_root}/rules/rule.md
    metadata: {type: Rule}
    version: current
  - match: {type: Requirements, intent_id: "${intent_id}"}
    count: one
    version: accepted
    accepted_at: discovery
  - match: {type: ImplementationPlan, intent_id: "${intent_id}"}
    count: one
    version: accepted
    accepted_at: planning
  - match: {type: CurrentAnalysis}
    count: optional
    version: current
  - match: {type: Architecture}
    count: optional
    version: current
  - declared: intent_documents
outputs:
  - declared: intent_documents
sensors:
  start: integration-start
  end: integration-end
---

# 統合検証

既存または変更したコードの対象commitと実測結果を検証する。tddが選択されていなくても実行できる。必要な現行仕様のKnowledgeはoutputsへ宣言する。

## 現在回の操作

入口のCLIを `A`、Intentを `ID`、Spaceを `SPACE`、showのrevisionを `R` とする。
各turnでbindが返すRuleを読み、procedureに表示されたstep_idと入力path/hashを確認する。
`A intent check ID --space SPACE --boundary start` → `A intent begin ID --space SPACE --expect R` で開始する。
成果を用意したら `A intent check ID --space SPACE` と別担当のread-only reviewを行う。
実際の回答による成果承認は `A intent approval ID --space SPACE --expect R --file DECISION.json`。
現在回だけを `A intent finish ID --space SPACE --expect R` で完了する。計画承認だけで成果を合格にしない。
文書・Unit・実測結果は現在のstep_idへ結び付ける。過去の同stage成功を使い回さない。
共通のJSON型、会話出典、Knowledge保存、Unit、reopenと保存retryは `A intent ACTION --help` と配置済みaidlc-cliスキルを参照する。

統合対象の現在HEADで必要commandを実行し、step_id/stage/runsを持つ結果JSONをtest_resultsへ登録する。各runのcommand/commit/exit_code/output_pathは実測値を使う。必要な現行仕様のKnowledgeをoutputsへ宣言する。
