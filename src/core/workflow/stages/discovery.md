---
stage_id: discovery
agents:
  - role: requirements
    agent: aidlc-requirements
  - role: research
    agent: aidlc-researcher
  - role: independent_review
    agent: aidlc-reviewer
inputs:
  - match: {type: Rule, title: "Intent実行の作業合意"}
    count: one
    version: current
  - match: {type: CurrentAnalysis}
    count: optional
    version: current
  - match: {type: Architecture}
    count: optional
    version: current
  - declared: intent_documents
outputs:
  - path: "${knowledge_root}/design/${intent_id}/requirements.md"
    role: result
    metadata: {type: Requirements, intent_id: "${intent_id}"}
  - declared: intent_documents
sensors:
  start: discovery-start
  end: discovery-end
---

# 目的整理と深掘り

目的・範囲・受入条件・未確定事項を具体化し、任意段階の採否と省略理由を整理する。資材があるだけで解析・構成図を要求しない。

## 現在回の操作

入口のCLIを `A`、Intentを `ID`、Spaceを `SPACE`、showのrevisionを `R` とする。
各turnでbindが返すRuleを読み、procedureに表示されたstep_idと入力path/hashを確認する。
`A intent check ID --space SPACE --boundary start` → `A intent begin ID --space SPACE --expect R` で開始する。
成果を用意したら `A intent check ID --space SPACE` と別担当のread-only reviewを行う。
実際の回答による成果承認は `A intent approval ID --space SPACE --expect R --file DECISION.json`。
現在回だけを `A intent finish ID --space SPACE --expect R` で完了する。計画承認だけで成果を合格にしない。
文書・Unit・実測結果は現在のstep_idへ結び付ける。過去の同stage成功を使い回さない。
共通のJSON型、会話出典、Knowledge保存、Unit、reopenと保存retryは `A intent ACTION --help` と配置済みWORKFLOW.mdを参照する。

任意4段階の採否・省略理由をintent planで提示し、intent plan-approvalで承認する。計画と成果の両requestを提示済みなら同じ後続回答を双方へ記録できる。後から作ったrequestへ転用しない。
