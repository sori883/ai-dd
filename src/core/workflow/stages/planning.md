---
stage_id: planning
agents:
  - role: execution_planning
    agent: aidlc-stage-planner
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
  - match: {type: CurrentAnalysis}
    count: optional
    version: current
  - match: {type: Architecture}
    count: optional
    version: current
  - declared: intent_documents
outputs:
  - path: "${knowledge_root}/design/${intent_id}/implementation-plan.md"
    role: result
    metadata: {type: ImplementationPlan, intent_id: "${intent_id}"}
  - declared: intent_documents
sensors:
  start: planning-start
  end: planning-end
---

# 実装計画

実装範囲・手順・検証を具体化する。編集範囲scopeとは別に、ソース・テスト・設定・共通部品を含むverification_pathsを定める。TDDとintegrationでは非空の集合が必須であり、任意のUnit集合はIntent集合へ含める。Unit分割する場合は担当範囲・依存・検証・Boltを定める。

## 現在回の操作

入口のCLIを `A`、Intentを `ID`、Spaceを `SPACE`、showのrevisionを `R` とする。
各turnでbindが返すRuleを読み、procedureに表示されたstep_idと入力path/hashを確認する。
`A intent check ID --space SPACE --boundary start` → `A intent begin ID --space SPACE --expect R` で開始する。
成果を用意したら `A intent check ID --space SPACE` と別担当のread-only reviewを行う。
実際の回答による成果承認は `A intent approval ID --space SPACE --expect R --file DECISION.json`。
現在回だけを `A intent finish ID --space SPACE --expect R` で完了する。計画承認だけで成果を合格にしない。
文書・Unit・実測結果は現在のstep_idへ結び付ける。過去の同stage成功を使い回さない。
共通のJSON型、会話出典、Knowledge保存、Unit、reopenと保存retryは `A intent ACTION --help` と配置済みaidlc-cliスキルを参照する。

## 工程skill

メインAIがaidlc-planningを使い、実装手順・Unit・所有範囲・受入条件と検証を具体化する。stage-plannerの担当は工程採否・順序の提案のまま。 配布先は `.agents/skills/<skill名>/SKILL.md`。必要な本文だけを読む。日本語の本文案はnatural-japanese-goで推敲できる。担当は案と根拠をメインAIへ返し、共有保存は既存のaidlc memory経路を使う。既存の許可担当・Sensor・承認・単独writerに従い、skill読込み自体を工程合格条件にしない。
