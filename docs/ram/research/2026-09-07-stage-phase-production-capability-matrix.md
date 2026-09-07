# 固定AI-DLC 2.6.123の33 StageとGo production能力の対応調査

- 日付: 2026-09-07
- 状態: Current for repository-pinned AI-DLC `2.6.123`
- 比較対象: `docs/配布_ai-dlc/`に固定されたAI-DLC `2.6.123`
- Go実装基準: `origin/main`のmerge commit
  `41e9e3da2d060f1063d41db5be3c47062bb99860`（PR #125まで）
- 調査方法: 配置済みStage frontmatter、共通protocol、scope grid、Goのdelivery・completion・
  artifact・audit・review・sensor・orchestrator・Codex receiverをread-onlyで照合した

## 読み方

能力欄は、`D`=delivery、`C`=completion、`A`=artifact、`S`=summary、`R`=review、
`X`=sensor、`P`=pipeline、`U`=per-unit、`K`=CodeKBを表す。

- `実`: 固定Stageのproduction経路へ接続済み
- `核`: 共通の内部model・reader・判定骨格はあるが、Stageのproduction経路は未接続
- `専`: `intent-capture`専用実装だけがある
- `阻`: metadataを読めても、未対応能力として明示的にfail closedになる
- `—`: 固定版の当該Stageでは要求されない

`summary confirmation`を要求するStageは、questions fileに対する別のfreshな人間応答、厳密な
`Looks correct`、`SUMMARY_CONFIRMATION_RECORDED`、確認対象digestの不変性に加え、そのreceipt後の
成果物writeが完了条件になる。成果物writeの正本は固定版の`PostToolUse`が記録する
`ARTIFACT_CREATED` / `ARTIFACT_UPDATED`である。Goの現行Codex hookは`UserPromptSubmit`だけなので、
この後半は`intent-capture`を含めて一般的なproduction証拠経路がまだない。

## Initialization

| Stage | 固定版のdelivery・成果物・完了契約 | 現在のGo能力 D/C/A/S/R/X/P/U/K | production経路の不足 |
| --- | --- | --- | --- |
| `workspace-scaffold` | ALWAYS、orchestrator inline。Intent record tree、対象phase directory、Space knowledge directoryを作る。approvalなしで自動進行 | 核/核/核/—/—/—/—/—/— | workspace・record作成APIはあるが、固定Stageとして起動・完了・次Stageへ送る公開conductorがない |
| `workspace-detection` | ALWAYS、orchestrator inline。filesystem scanからgreenfield/brownfield分類とtechnology stackを得る。approvalなし | 核/核/核/—/—/—/—/—/— | read-only detectionは実装済みだがStage lifecycleへ接続されず、Stage完了receiptもない |
| `state-init` | ALWAYS、orchestrator inline。workspace classificationとscopeから完全な`aidlc-state.md`を作る。approvalなし | 核/核/核/—/—/—/—/—/— | state builder/writerと`StartIntent`内部接続はあるが、3 Initialization Stageとしてのproduction entryと自動遷移がない |

## Ideation

| Stage | 固定版のdelivery・成果物・完了契約 | 現在のGo能力 D/C/A/S/R/X/P/U/K | production経路の不足 |
| --- | --- | --- | --- |
| `intent-capture` | ALWAYS。project descriptionからintent statement、stakeholder map、questions。summary必須、product-lead advisory review、`claim-sources`・`required-sections`・`upstream-coverage` | 実/実/実/実/実/実/—/—/— | 唯一の縦切り実装。既存active Intentが前提。承認後の固定graph successor能力検証が永続化後なので、非対応後続Stageで部分完了が残り得る |
| `market-research` | CONDITIONAL。intentからcompetitive analysis、market trends、build-vs-buy、questions。summary必須、reviewなし、RS・UC | 核/核/核/専/—/阻/—/—/— | wireとcontextは作れるがreceiverは`context ready`で停止。summaryはintent path固定、sensorは非intentを拒否 |
| `feasibility` | CONDITIONAL。intentと任意market成果からfeasibility assessment、constraint register、RAID、questions。summary必須、reviewなし、RS・UC | 核/核/核/専/—/阻/—/—/— | `market-research`と同じ。MVPでは`intent-capture`の直後なので現行固定graph遷移を止める |
| `scope-definition` | ALWAYS。intentと任意feasibility/constraintsからscope document、intent backlog、questions。summary必須、reviewなし、RS・UC | 核/核/核/専/—/阻/—/—/— | ordinary artifact存在判定はany-ofに留まり、summary・sensorのStage共通receipt解決がない |
| `team-formation` | CONDITIONAL。scope/backlog等からteam assessment、skill matrix、mob composition、questions。summary必須、reviewなし、RS・UC | 核/核/核/専/—/阻/—/—/— | 条件不成立を正規に送る公開`skipped` resultもなく、実行時のStage共通receiverもない |
| `rough-mockups` | CONDITIONAL。scope等からwireframes、user flow、questions。summary必須、product-lead advisory review、RS・UC | 核/核/核/専/専/阻/—/—/— | summary・sensorに加え、任意Stageのreview artifact集合とfreshnessを解決するdispatcher/read modelがない |
| `approval-handoff` | ALWAYS。Ideation成果からinitiative brief、decision log、questions。summary必須、reviewなし、RS・UC | 核/核/核/専/—/阻/—/—/— | phase終端までのStage共通receipt、成果物write監査、learnings、承認・phase遷移が未接続 |

## Inception

| Stage | 固定版のdelivery・成果物・完了契約 | 現在のGo能力 D/C/A/S/R/X/P/U/K | production経路の不足 |
| --- | --- | --- | --- |
| `reverse-engineering` | CONDITIONAL brownfield。developer→architect pipeline。repositoryごとのCodeKB 9成果物。summary/reviewなし、RS・UC | 阻/阻/阻/—/—/阻/阻/—/阻 | pipeline link receipt、repository列挙、Space-level CodeKB配置・freshness・完了判定がすべて未実装。POCではintent直後のblocker |
| `practices-discovery` | CONDITIONAL subagent。stateと任意CodeKBからteam practices、rules/evidence/timestamp、questions。summary必須、RS・UC。承認後memory promotion | 核/核/核/専/—/阻/—/—/— | subagent execution、Stage共通summary/sensor、承認後promotion receiptがない |
| `requirements-analysis` | ALWAYS。Ideation成果、任意CodeKB、practicesからrequirements、questions。summary必須、product-lead advisory review、RS・UC | 核/核/核/専/専/阻/—/—/— | 汎用summary/review/sensorとproducer executionが未接続 |
| `user-stories` | CONDITIONAL mob。requirements等からstories、personas、assessment、traceability、questions。summary必須、product review、RS・UC・TR | 核/核/核/専/専/阻/—/—/— | mob dispatch、traceability sensor、汎用review receiptがない |
| `refined-mockups` | CONDITIONAL。rough mockups等からmockups、interaction spec、design mapping、accessibility checklist、questions。summary必須、product review、RS・UC | 核/核/核/専/専/阻/—/—/— | 複数成果物とreview bindingをStage metadataから解決するproduction経路がない |
| `domain-design` | CONDITIONAL。requirements等からcomponents、ADR、traceability、questions。summary必須、architecture review、RS・UC・TR | 核/核/核/専/専/阻/—/—/— | architecture reviewerとtraceabilityを含む汎用receiptがない |
| `units-generation` | scope内ではALWAYS。components/requirementsからUnit DAG、dependencies、story map、traceability、questions。summary必須、architecture review、RS・UC・TR | 核/核/核/専/専/阻/—/—/— | Unit定義成果物は通常artifactとして解決できるが、後続per-unit実行へ渡すenumeration/stateはない |
| `contract-design` | CONDITIONAL。Unit DAG等からcontract summaryとinline API/schema spec、questions。summary必須、architecture review、RS・UC | 核/核/核/専/専/阻/—/—/— | public/external contractを作るStage実行はなく、review/sensor共通化もない |
| `delivery-planning` | ALWAYS。Inception成果からbolt plan、team allocation、risk/sequencing、external dependency map、questions。summary必須、reviewなし、RS・UC | 核/核/核/専/—/阻/—/—/— | phase終端のsummary/write/sensor/learningsとConstruction遷移が未接続 |

## Construction

| Stage | 固定版のdelivery・成果物・完了契約 | 現在のGo能力 D/C/A/S/R/X/P/U/K | production経路の不足 |
| --- | --- | --- | --- |
| `functional-design` | CONDITIONAL、per-unit inline。entities、business rules、functional spec、traceability等。summary必須、architecture review、RS・UC・L・T・TR | 阻/阻/阻/専/専/阻/—/阻/— | Unit列挙、Unit別state/audit、kind-aware artifact path、fan-out/fan-in完了がない |
| `nfr-requirements` | CONDITIONAL、per-unit inline。performance/security/scalability/reliability/observability requirements、technology decisions等。summary・architecture review、RS・UC・L・T・TR | 阻/阻/阻/専/専/阻/—/阻/— | per-unit基盤に加え、複数sensor dispatcherと任意CodeKB consume解決がない |
| `nfr-design` | CONDITIONAL、per-unit inline。各NFR design、logical components等。summary・architecture review、RS・UC・L・T・TR | 阻/阻/阻/専/専/阻/—/阻/— | 前Stageと同じper-unit、kind-aware成果物、review/sensor不足 |
| `infrastructure-design` | CONDITIONAL、per-unit inline。infrastructure spec、monitoring design、CI/CD pipeline等。summary・architecture review、RS・UC・L・T・TR | 阻/阻/阻/専/専/阻/—/阻/— | 前Stageと同じ。Unit成果を後続へ集約する完了receiptがない |
| `code-generation` | ALWAYS、per-unit subagent、workspace必須。workspace code、code plan、unit-test instructions、code summary、traceability。plan approval、architecture review、RS・L・T・TR | 阻/阻/阻/専/専/阻/—/阻/— | workspace write権限、plan approval、subagent、Unit別artifact/audit/review/sensorを安全に束ねる経路がない |
| `build-and-test` | ALWAYS、全Unit完了後。build/test instructions・results、cross-unit traceability。summary/reviewなし、RS・UC・T。失敗時最大3回loop | 核/核/核/—/—/阻/—/—/— | per-unit成果のfan-in、test実行receipt、失敗loop-back、type-check sensorがない |
| `ci-pipeline` | CONDITIONAL。code/build成果からCI config、quality gates、questions。summary必須、reviewなし、RS・UC・L・T | 核/核/核/専/—/阻/—/—/— | `skipped`、summary/write、linter/type-check sensor、Operation境界traceabilityが未接続 |

## Operation

| Stage | 固定版のdelivery・成果物・完了契約 | 現在のGo能力 D/C/A/S/R/X/P/U/K | production経路の不足 |
| --- | --- | --- | --- |
| `deployment-pipeline` | CONDITIONAL。CI/infra成果からCD config、deployment strategy、rollback plan、questions。summary必須、RS・UC | 核/核/核/専/—/阻/—/—/— | 実行時conditionの`skipped`、summary/write/sensor、外部pipeline変更の権限境界が未接続 |
| `environment-provisioning` | CONDITIONAL。infra/CDからenvironment inventory、validation report、questions。summary必須、RS・UC | 核/核/核/専/—/阻/—/—/— | cloud認証・権限と実処理receiptを決めるproduction adapterがない |
| `deployment-execution` | CONDITIONAL。CD/environment/buildからdeployment log、smoke tests、health report、questions。summary必須、RS・UC | 核/核/核/専/—/阻/—/—/— | deployment副作用、rollback、安全な承認と実行receiptの境界がない |
| `observability-setup` | CONDITIONAL。NFR/infra/deployed appからdashboard、alarm、SLO、logs/tracing/anomaly config、questions。summary必須、RS・UC | 核/核/核/専/—/阻/—/—/— | provider操作と権限、成果物・実環境の対応receiptがない |
| `performance-validation` | CONDITIONAL。NFR/observabilityからload-test plan/results、NFR matrix、questions。summary必須、RS・UC | 核/核/核/専/—/阻/—/—/— | load実行の安全制約と外部結果receipt、Stage共通証拠がない |
| `incident-response` | CONDITIONAL。observability/NFR/infraからrunbooks、incident plan、escalation matrix、questions。summary必須、RS・UC | 核/核/核/専/—/阻/—/—/— | 外部連絡・権限を伴う実行境界とStage共通証拠がない |
| `feedback-optimization` | CONDITIONAL、最終Stage。observability/deployment等からSLO/cost/drift reports、feedback loop、questions。summary必須、RS・UC | 核/核/核/専/—/阻/—/—/— | 最終Stage completion、workflow-complete delivery、次Ideation cycleへのhandoffがない |

## 横断的な現在地

1. **delivery**: `aidlc next` / `continue` / `read-context`、identity-bound marker、継続token、
   fresh配置読込はproduction接続済みである。ただしbundled Codex receiverが実行するのは
   `intent-capture`だけで、通常Stageはcontext読込後に停止する。
2. **completion**: `EvaluateStageCompletion`はartifact→summary→pipeline→review→sensor→blockingの
   順序を持つが、非intent Stageのaudit evidence resolverがなく、capability guardがsummary・review・
   sensor等を先に拒否する。
3. **artifact**: 通常Stageのrecord相対path解決と存在判定はあるが、required outputsは現在any-ofである。
   per-unit、`produces_kinds`、CodeKBは明示拒否される。
4. **summary/review/sensor**: authority eventと値型の大部分は`intent-capture`縦切りに閉じている。
   特にquestions pathが`ideation/intent-capture/intent-capture-questions.md`へ固定されている。
5. **pipeline/per-unit/CodeKB**: graph metadataは保持するが、production dispatcher、receipt writer、
   read model、完了判定はない。OKF metadata検索はSpace knowledge discoveryであり、CodeKBの代替ではない。
6. **遷移安全性**: `ApproveGate`は現Stageの完了をaudit/stateへ保存してから後続Stage能力を検証する。
   固定graphでは`intent-capture`後継が非対応なため、errorと完了済みstateが同時に残り得る。

## 最初の共通基盤の選択

最初は、28のsummary-required Stageで共有するquestions・summary confirmationのauthority coreを
Stage-genericにする。現行の`intent-capture`用event名・field・human-turn freshness・semantic digest・
record lock・root confinementをそのまま保ち、engine-resolved questions pathを扱える内部APIへ抽出する。

Initializationの再実装は既存のworkspace detection/state builderと重複が大きい。一方、reviewなしの
Ideation sliceをproduction完走させるには、このsummary coreに加えてPostToolUse artifact-write receipt、
汎用sensor、learnings、CONDITIONAL Stage向け公開`skipped` resultが必要になる。後者は公開API・hookの
安全境界を同時に変えるため、最初の最小基盤には含めない。

この選択はStageをまだ新たにadvance可能にはしない。非intentのgateとCodex hidden actionをfail closedの
まま保ち、複数Stageで同一のreceipt契約を再利用できる内部authority coreと回帰証拠を先に作る。
後続sliceは、固定版の正本であるPostToolUse artifact-write receiptを実装し、Ideationの
reviewなしStageへsummary coreを接続する。

## 根拠

- `docs/ram/decisions/2026-09-03-aidlc-implementation-roadmap.md`
- `docs/ram/decisions/2026-09-03-milestone-authorization-and-autonomous-merge.md`
- `docs/ram/decisions/2026-09-06-intent-capture-vertical-milestone.md`
- `docs/ram/decisions/2026-09-06-intent-capture-upstream-conformance-corrections.md`
- `docs/aidlc-analysis/README.md`
- `src/core/aidlc-common/stages/`
- `src/core/aidlc-common/protocols/stage-protocol.md`
- `src/core/aidlc-common/protocols/stage-definition.md`
- `src/core/knowledge/aidlc-shared/audit-format.md`
- `docs/配布_ai-dlc/.codex/tools/data/stage-graph.json`
- `docs/配布_ai-dlc/.codex/tools/data/scope-grid.json`
- `src/internal/delivery/`
- `src/internal/orchestrator/completion.go`
- `src/internal/orchestrator/gate.go`
- `src/internal/orchestrator/approve.go`
- `src/internal/artifact/`
- `src/internal/audit/intent_capture.go`
- `src/cmd/aidlc/codex_stage.go`
- `src/harness/codex/skills/aidlc/SKILL.md`
