---
stage_id: tdd
agents:
  - role: execution_planning
    agent: aidlc-stage-planner
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

verification_pathsへコード・設定・テスト・共通部品を指定する。intent hash→テスト→intent hashの一致を確認し、step_id/stage/verification_scope/verification_sha256/runsを持つ結果JSONをaidlc/evidenceへ保存してtest_resultsへ登録する。各runはunit_id/command/exit_code/output_path。Unit結果にはトップレベルunit_id/run_idも指定する。Unitのresult/integrateは内容を照合する。終了時は最新Intent全体SHAで全Unit+commandの成功を確認する。

## 工程skill

workerはaidlc-tddで失敗testから実装し、不具合調査ではaidlc-systematic-debuggingを使う。reviewerはaidlc-code-reviewで独立確認する。 配布先は `.agents/skills/<skill名>/SKILL.md`。必要な本文だけを読む。日本語の本文案はnatural-japanese-goで推敲できる。担当は案と根拠をメインAIへ返し、共有保存は既存のaidlc memory経路を使う。既存の許可担当・Sensor・承認・単独writerに従い、skill読込み自体を工程合格条件にしない。
