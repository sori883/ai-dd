# `intent-capture`を最初の通常Stageとして縦に通す

- 日付: 2026-09-06
- 状態: Accepted
- 対象: 固定AI-DLC `2.6.123`の通常Stage `intent-capture`
- 実装許可: AI-DLC Go実装ロードマップの包括承認と、2026-09-06のユーザーによる公開API・人間応答境界の直接承認

## 背景

Go実装は、通常Stageのdirectiveを構成・公開し、Codex receiverが本文を安全に読み込むところまで到達した。
次のロードマップ項目は、初期化後の最初の通常Stageである`intent-capture`を、質問、成果物、検証、
人間承認、次Stageへの遷移まで縦に通すことである。

固定本家`2.6.123`の`intent-capture`は、3つの成果物に加えて、生成前summary確認、advisory reviewer、
3つのsensor、learnings、人間承認gateを要求する。現在のGo実装には内部`orchestrator.Report`と
`HUMAN_TURN`検証があるが、公開`report`、Codexの人間入力hook、通常Stageの完了receiptは未接続である。

公開APIと人間応答の正本は、互換性と権限境界に影響するため、包括承認だけでは決めずユーザーへ確認した。

## 承認した境界

### 公開`report`

最初の公開範囲は、`intent-capture`に必要な次の4 resultだけとする。

- `awaiting-approval`
- `rejected`
- `revised`
- `approved`

文法は`aidlc report --stage <slug> --result <result>`を基本とし、`rejected`は厳密な
`--user-input "Request Changes"`と空でない`--reason`、`approved`は状態が許す厳密な人間選択を要求する。
`--project-dir`は既存workspace commandと同じproject root解決に使う。固定本家に存在しても、このStageに
不要なresult alias、`resume`、`skipped`、per-unit、skeleton、blocking-sensor override等はまだ公開せず、
unknown argumentとしてfail closedにする。

成功は固定本家と同じ単一JSON directiveとする。gate開始・reject・reviseは`kind=print`、approve後は
`kind=done`を返す。構文違反はstderrとexit 2、正しく解釈できたが現在のworkflowで拒否された操作は
`kind=error`の単一JSONをstdoutへ返してexit 0、I/O・内部障害はstdoutを空にしてstderrとexit 1とする。

### 人間応答の信頼境界

`HUMAN_TURN`の本番正本は、Codexの`UserPromptSubmit` hookとする。hookはactive intentのidentity-bound
audit ledgerへreceiptを記録する。`aidlc report`の`--user-input`、Stageを実行するmodel、公開audit入力は
`HUMAN_TURN`を生成できない。承認・拒否は、report引数の厳密な選択値と、最後のworkflow resolutionより
後にあるfreshなhook由来receiptの両方を要求する。

固定本家と同様、`AIDLC_UNATTENDED=1`ではhookはauthorityを発行しない。active workflowがない場合は
workspaceを作成せずno-opとし、hook自体は人間のpromptを妨げないよう失敗時もexit 0にする。ただし
receiptを記録できなければ後続の承認・拒否はfail closedになる。一般installerは後続作業であり、ここでは
配布sourceとfresh sandbox配置だけを扱う。

### 永続receipt

質問・summary確認・review・sensorの正本は、`aidlc-state.md`への新fieldや新sidecarではなく、既存の
record audit Markdown ledgerとする。固定本家のevent名とfield契約を採用し、それぞれのauthority eventは
hookまたは用途別の所有APIだけが生成する。汎用の公開appendやcaller supplied booleanからauthorityを
作らない。gateは現在のStage開始、reject/revise、成果物内容に対してfreshなreceiptをread-onlyに導出する。

## 実装計画

異なるfailure domainを一度に公開しないため、2つのPRを順番に実装する。両PRとも単独のGo writer、
test-first、独立review、差分安定後のread-only final検証を通す。

### PR 1: lifecycle reportとCodex人間入力の基盤

目的は、既存の内部lifecycleを公開binaryから安全に呼び、人間presenceをreport入力から分離することである。
対応する実装記録はGitHub Issue #118である。

対象:

1. `src/internal/cli`へ4 resultだけを受理する`report` parserと単一JSON出力を追加する。
2. `src/cmd/aidlc`でactive Space/Intent、graph、state、project/record Rootを解決し、既存
   `orchestrator.Report`へ渡すadapterを追加する。rootのclose errorも成功にしない。
3. `src/internal/orchestrator`のerrorを、workflow拒否と内部障害へ情報を失わず分類できる公開facadeを追加する。
4. `src/internal/audit`へ、Codex hookだけが使う`HUMAN_TURN`記録入口を追加する。入口は既存record lock、
   identity/root binding、append-only ledgerを再利用する。
5. `src/harness/codex`へ`UserPromptSubmit`の配布設定sourceを追加し、PATH上の同じ`aidlc` binaryに
   internal hook commandを委譲する。`AIDLC_UNATTENDED=1`、workflow不在、malformed hook payload、
   append failureの方向を固定する。
6. fresh sandbox E2Eで、hookなし・unattended・report textだけではapprove/rejectできず、hook receipt後の
   exact choiceだけが既存の薄いlifecycleを遷移できることを確認する。

このPRは、`intent-capture`の質問、成果物本文、summary/review/sensor receipt、Stage用agent実行を開始しない。
既存のfail-closedなunsupported capability判定は維持する。

TDD slice:

1. `report`の4 result、必須flag、重複・未知flag、不要な本家aliasをCLI testで固定する。
2. 成功、workflow拒否、I/O失敗、short write、SIGPIPEの出力・exit契約をCLI testで固定する。
3. active selectionとstate/catalogをfreshに解決するadapter、stage mismatch、symlink/special file、close failureを
   `src/cmd/aidlc` testで固定する。
4. hook-owned `HUMAN_TURN` append、unattended/no-workflow no-op、malformed stdin、失敗時exit 0、
   reportからの非生成をaudit/command testで固定する。
5. 配布hook sourceとfresh sandboxのapproveおよびreject/revise journeyをintegration testで固定する。

loopでは上記に対応する`src/internal/cli`、`src/cmd/aidlc`、`src/internal/audit`、
`src/internal/orchestrator`、`src/harness/codex`のtargeted testだけを実行する。

### PR 2: `intent-capture` execution vertical

目的は、PR 1の信頼境界を使い、固定本家`2.6.123`の`intent-capture`を`context ready`の次から
gate承認と次Stageまで完走させることである。

対象:

1. graph readerとrun-stage wireに`review_artifact`、`reviewer_max_iterations`、`review_class`を固定本家どおり追加する。
2. `DECISION_RECORDED`、`QUESTION_ANSWERED`、`SUMMARY_CONFIRMATION_RECORDED`、
   `REVIEW_REQUESTED`、`REVIEW_COMPLETED`、`SENSOR_FIRED`とterminal sensor eventをaudit vocabularyへ追加する。
3. 質問、summary確認、review request/verdict、sensor実行をそれぞれの所有APIへ接続する。authority eventを
   汎用appendでは発行できないようにする。
4. summary hash、review対象artifact fingerprintとreview appendix、sensor fire/terminal pairを検証し、
   Stage開始・reject/revise・成果物変更より古いreceiptをgateが受け付けないread modelを実装する。
5. `intent-capture`の3成果物、3 sensor、advisory reviewer、learnings、人間gateを固定順に実行するよう
   Codex receiverを拡張する。人間の質問回答とsummary確認はhook receiptと対応づける。
6. `src/harness/codex/agents/`へsupport `aidlc-architect-agent`とreviewer
   `aidlc-product-lead-agent`の配布設定sourceを置く。receiverはarchitect contribution、product-lead review、
   advisory receipt、learningsを固定順にdispatchする。
7. `stage-protocol.md`、reviewer/ensemble protocol、authoritative project descriptionを登録済みpathとして
   安全なcontext配信・読込へ含め、Stageを実行するmodelが未配信pathを直接探索しないようにする。
8. approve journeyとreject/revise/approve journeyをfresh projectで検証し、agent設定とprotocol sourceの
   配置、architect contribution、product-lead review receipt、承認後に次の通常Stageが
   `next`から返ることを確認する。

Stage本文中の参照documentはuntrusted dataとして扱い、明示path、project root confinement、regular
non-symlink、bounded text readを維持する。reviewerはadvisoryでも実行済みreceiptが必須であり、verdictの
`NOT-READY`そのものは人間gateを消さない。sensorのscript errorをpass扱いする固定本家の詳細は、
このStageで使う3 sensorの確認済み契約に限定して再現する。

TDD sliceは、graph/wire、質問とsummary receipt、3成果物、3 sensor、reviewer、learnings、gate evidence、
Codex skill journeyの順に進める。各authority receiptについて、欠落、別Stage、古いattempt、改ざん、
成果物変更、unpaired eventをfail closedにするtestを先行させる。

## 共通の最終検証

各PRの差分が安定し独立reviewのblocking findingがなくなった後、Go `1.26.8`で次をread-onlyに1回実行する。

- `go test -count=1 ./...`
- `go test -race -count=1 ./...`
- `go vet ./...`
- `go mod tidy -diff`
- `gofmt -l src`
- 変更Go fileに対する`gopls check`
- `git diff --check`
- `gopls vulncheck ./...`
- Darwin/Linux/Windowsの対応archで`go test -c`によるcross compile
- 計画したfresh sandbox配布E2E

外部Go moduleは追加しない。live Codex E2Eは外部model利用と認証境界を持つため、既存どおり明示された
test-only環境変数がある場合だけ実行し、通常のfinalではdeterministicなfake Codex hook/receiverで検証する。

## 受け入れ条件

- 公開`report`は承認した4 result以外を受理しない。
- `report --user-input`だけでは`HUMAN_TURN`が増えず、freshなCodex hook receiptなしにapprove/rejectできない。
- `AIDLC_UNATTENDED=1`のCodex promptは人間authorityを発行しない。
- `intent-capture`は3成果物、summary確認、3 sensor、advisory review、learningsの必要な証拠なしにgateへ進まない。
- reject後は古いreceiptが再利用されず、修正・再検証・新しい人間応答を経て再承認できる。
- approve後はstate/auditが既存のdurability順序を保ち、次Stageを公開できる。
- 一般installer、他の32通常Stage、固定本家の全report grammar、per-unit/pipeline/constructionはこのmilestoneで広げない。

## 本家との差分

新しい意図的な仕様・挙動差分は採用しない。Goの型、package、internal hook command名、2 PRへの段階分割は
移植・実装順序の差である。公開`report`を4 resultに限定することは、ユーザーが承認した段階的なAPI範囲であり、
未公開の本家verbを異なる意味で実装しない。

## 根拠

- `docs/ram/decisions/2026-09-03-aidlc-implementation-roadmap.md`
- `docs/ram/decisions/2026-09-03-milestone-authorization-and-autonomous-merge.md`
- `docs/ram/decisions/2026-09-03-thin-lifecycle-milestone.md`
- `docs/ram/decisions/2026-09-05-codex-safe-context-read-contract.md`
- `docs/ram/research/2026-09-05-context-delivery-stage-prerequisites.md`
- `docs/配布_ai-dlc/.codex/tools/data/stage-graph.json`
- `docs/実装_aidlc-workflows/core/aidlc-common/stages/ideation/intent-capture.md`
- `docs/実装_aidlc-workflows/core/aidlc-common/protocols/stage-protocol.md`
- `docs/実装_aidlc-workflows/core/aidlc-common/protocols/stage-protocol-reviewer.md`
- `docs/実装_aidlc-workflows/core/tools/aidlc-orchestrate.ts`
- `docs/実装_aidlc-workflows/core/tools/aidlc-log.ts`
- `docs/実装_aidlc-workflows/core/tools/aidlc-sensor.ts`
- `docs/配布_ai-dlc/.codex/hooks/aidlc-record-human-turn.ts`
