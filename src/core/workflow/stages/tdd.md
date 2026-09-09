---
stage_id: tdd
agents:
  - role: implementation
    agent: aidlc-worker
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
  start: tdd-start
  end: tdd-end
---

# TDD

実行可能なtestを先に追加し、意図したRED、最小GREEN、整理の順で実装する。compile failure、skip、test不在をRED/GREENと扱わない。planning省略でも承認済み範囲・受入条件・テスト方法を明示する。

## 現在回の操作

入口のCLIを `A`、Intentを `ID`、Spaceを `SPACE`、showのrevisionを `R` とする。
各turnでbindが返すRuleを読み、procedureに表示されたstep_idと入力path/hashを確認する。
`A intent check ID --space SPACE --boundary start` → `A intent begin ID --space SPACE --expect R` で開始する。
成果を用意したら `A intent check ID --space SPACE` と別担当のread-only reviewを行う。
実際の回答による成果承認は `A intent approval ID --space SPACE --expect R --file DECISION.json`。
現在回だけを `A intent finish ID --space SPACE --expect R` で完了する。計画承認だけで成果を合格にしない。
文書・Unit・実測結果は現在のstep_idへ結び付ける。過去の同stage成功を使い回さない。
共通のJSON型、会話出典、Knowledge保存、Unit、reopenと保存retryは `A intent ACTION --help` と配置済みaidlc-cliスキルを参照する。

Unit分割時は担当範囲・依存・base_commit・testsを確認する。claim/result/integrateにもstep_idを付ける。実測結果JSONはstep_id/stage/runs、runはcommand/commit/exit_code/output_path。直接実装はdirect_commit、Unitは各ResultCommitで成功を確認する。
