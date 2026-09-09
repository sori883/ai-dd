# Stage Graphと段階別手順の実装計画

## 目的と許可

現在はAIがWORKFLOW.md全体を読み、Goに固定された4段階を進めている。
小さなJSONで現在の段階と進める方向を調べ、必要な段階のMarkdownだけ読める構成にする。
ユーザーは2026-09-09に提案内容の実装を直接依頼し、outputsが空でもよいと指定した。
差戻し理由はIntent配下の作業記録に保存する。ADRは設計上の「なぜ」に限る。
この直接承認を使い、旧33段階ロードマップの承認は使わない。
定義変更時は元定義へ復元または新Intent、既設の自動更新は保留する方法も、ユーザーが追加承認した。

## 利用者が使う構成

```text
aidlc/workflow/
  stage-graph.json
  stages/discovery.md
  stages/planning.md
  stages/tdd.md
  stages/integration.md
aidlc/spaces/<space>/intents/<intent_id>/
  state.json
  work-log.md
```

`intent procedure ID --space SPACE`は、現在段階・定義hash・手順path・frontmatterを含む手順全文・
前進先・差戻し候補をJSONで返す読取り専用コマンドとする。引数の型と操作方法はhelpに載せる。
任意のファイルを読む引数やstageを偽装する引数は設けない。
SKILL.mdはこのコマンドと必要なhelpへの短い入口にする。WORKFLOW.mdは共通操作の短い索引へ整理する。
段階の変化・再開時には現在手順を取り直す。各turnの必須Rule読込みは継続する。

## 定義の契約

JSON schema_versionは1。stage、name、procedure、start_stage、completion_stage、advance、reopenの構造は
[提案](stage-graph-procedure-proposal.md)の例に従う。初期対象はdiscovery/planning/tdd/integrationの4段階で、
前進は全段階が到達可能な単一路とする。重複ID/key、未知key、欠落参照、循環、分岐、逆転した必須成果の前提を拒否する。
Goの前進switch・順位mapを同じgraphの次段階・祖先・後続判定へ置き換える。
段階別の具体的なSensor実装の振分けはGoに残し、定義に書かれた任意コマンドを実行しない。

Markdown frontmatterはstage_id、agents、inputs、outputs、sensorsを持つ。
agentsはroleとagentを持ち、既存のaidlc-researcher、aidlc-requirements、aidlc-worker、aidlc-reviewerだけを受理する。
役割はresearch、requirements、implementation、independent_review。必要な調査のみresearcherへ依頼し、
reviewerは終了Sensor合格・成果固定後に起動する。共有stateと文書は調整役が書く。

文書参照はpathまたはrefsのどちらかを指定する。pathの変数は`${knowledge_root}`と`${intent_id}`のみ、
refsはconfig.adr.refsとconfig.feature_knowledgeのみ。入力のversionはcurrent/accepted、accepted_atは合格段階。
required_whenはalways/exists/adr_required/materials_presentから用途に合うものを受理する。
outputsは同じ文書参照とroleを使用し、version/accepted_atは持たない。空配列を許可し、ダミー文書を作らない。
参照先はSpaceのKnowledge配下の.md文書に限定する。プログラム・テスト結果JSONはこのoutputsへ入れない。
型・必須見出し・Intent ID・合格版の検査は既存Go Sensorを再利用し、frontmatterの参照を検査へ接続する。
開始/終了Sensor IDは各段階の`<stage>-start`/`<stage>-end`に限る。
outputs空がコード・テスト・ADR要否など既存の必須検査を無効化する意味にはしない。
必須要件書・計画書を定義から消しても既存契約を弱められないよう検証する。

標準JSON decoderと既存依存go.yaml.in/yaml/v3で解析する。外部moduleは追加しない。
未知field、重複key、YAML alias/merge/tag、未対応参照式、空手順、symlink・非通常file・root外pathを拒否する。
1ファイル256 KiB以内、frontmatter64 KiB以内。欠落時に内包版や旧WORKFLOWへ黙ってfallbackしない。

## 定義の版と復旧（承認済み）

新Intent作成時にgraphと全段階MDのパス・bytesから定義hashを保存する。保存schemaは3。
本文を含め定義が変われば作業・begin・検査・review・遷移・configure・Unit変更を拒否する。
show/listなど診断は残す。procedureは不一致を明示して終了し、新定義を旧Intentの手順として返さない。
元定義をGit等で戻すか、新Intentを作成して再開する。暗黙の再bindや旧pass流用はしない。
旧schemaのファイルを削除・移行せず、未対応schemaとして明示的に拒否する。
配布は既存のfresh・衝突時拒否を維持し、既設の一括更新を今回に含めない。

## 差戻しと保存失敗

reopenは同段階またはgraphの祖先へ戻す。理由は必須、戻り先と後続のaccepted、entry、Sensor、reviewを無効化する。
TDD内のテスト修正反復はreopen不要。質問待ちはstageを変えずwaitingへ移す。

work-log.mdはCLI管理の差戻し記録とし、日時（UTC自動）、元・先stage、理由、要求元revisionを記す。
全tool auditや進捗履歴を別JSONへ蓄積しない。保存調整用にstateへ未完了のreopen一件だけを保持する。
既存のSpace lockとrevision照合の内側で以下を行う。

新規logはpending保存前に空の通常fileを用意する。元の未存在と保留中の外部削除を区別し、
削除を見逃さず復元を案内するためである。既存logは上書きしない。

1. 理由・候補・既存logを検証し、元revision、元/先、理由、日時、既存log hashをpending_reopenとして保存。
   この時点ではstageとrevisionを変えない。他操作はpendingがある間、stateを変更できない。
2. 既存logを保持して決定的な差戻し項目を原子的に追記。同一再試行で項目を重複させない。
   記録には「stateのrevisionが要求元revisionを超えた時に確定」と明記し、保存途中を成功と誤認させない。
3. stage・合格無効化・revision増加を一度にstateへ保存し、pendingを除く。

各保存失敗時は成功を返さない。同じexpect/from/to/reasonのreopenだけ再試行でき、異なる操作と一般作業を拒否する。
logが想定前版または想定追記版以外なら上書きせず診断する。読取り・help・同一再試行は残す。
成功後に古いexpectで再試行したらrevision conflictとなり、重複記録しない。
保留中に人がlogを削除・改変した場合は元版の復元が必要。全履歴をstateから再生成する構成にはしない。

## 対象と所有権

単独Go writerが以下と対応testを所有する。他writerと同時編集しない。

- src/core/workflow/（新規assets/embed）、src/internal/workflow/（新規reader/validator/graph）
- src/internal/flow/（store、transition、boundary、sensor、review、保存失敗guard）
- src/internal/cli/とsrc/internal/minimal/（procedure/help、hook、state取得と復旧例外）
- src/internal/install/（embed配置、移転の既知原稿参照とtest）
- src/harness/codex/minimal/とsrc/core/minimal/knowledge/rules/rule.md（短い入口と役割参照）
- src/cmd/aidlc/のjourney・live観測test、必要なfixture
- docs/ramの実装証拠、README・製品説明の該当部分

親が計画・承認RAM・Issue・PRを管理する。未追跡のdocs/実装_okf-agent-memory/には触れない。
保留中のcheck help実案件を本機能の実装へ混ぜない。

## 順序付きTDD

work_unit_idはstage-graph-procedures、verification_modeはloop。1 Issue/PR全体を1 writerへ渡す。
新APIが必要なtestには、型・signatureと空返値だけのcompile scaffoldを許可する。
各sliceはrunnableな期待値不一致REDを先に確認し、最小GREENへ進む。

|順|振る舞いと対象|exact targeted command|
|---|---|---|
|1|workflow parser・graph・不正定義拒否|`go test -count=1 ./src/internal/workflow -run '^TestDefinition'`|
|2|flowのgraph遷移・既存gate維持|`go test -count=1 ./src/internal/flow -run '^TestGraphTransition'`|
|3|frontmatter文書参照・空outputs・既存test保護|`go test -count=1 ./src/internal/flow -run '^TestProcedureBoundary'`|
|4|定義binding・schema・差戻しlogの各保存故障/再試行|`go test -count=1 ./src/internal/flow -run '^(TestDefinitionBinding|TestReopenLog)'`|
|5|procedure CLI/helpとhook読取/拒否|`go test -count=1 ./src/internal/cli ./src/internal/minimal -run '^TestProcedure'`|
|6|fresh配置・短い入口・移転の既知原稿|`go test -count=1 ./src/internal/install -run '^TestWorkflowDefinition'`|
|7|実CLI journeyと限定liveの観測検証器|`go test -count=1 ./src/cmd/aidlc -run '^TestProcedureEvidence'`|

末尾に各targeted群とaffected packages（workflow/flow/cli/minimal/install/cmd/aidlc）の通常test、
変更Goファイルgofmt、git diff --checkを実施。integration/live実行はloopで行わない。
親は末尾で差分とtargeted群を一度確認し、固定base/headの独立reviewへ渡す。

## 受入と最終検証

前進・差戻し先をJSONから取得し、現在段階の手順だけ取得できること。
空outputs、未対応定義、定義変更、差戻し保存故障を観測testで確認すること。
4段階を新CLI経由で完走し、TDD→planningの差戻しと再前進、既存Sensor/review拒否が維持されること。

blocking findingを解消後、親がread-only finalを実施する。
`go test -shuffle=on ./...`、`go test -race -shuffle=on ./...`、`go vet ./...`、`go mod tidy -diff`、
`gofmt -l src`、`git diff --check`、`go test -tags=integration -count=1 ./...`。
darwin/linux/windowsのamd64/arm64をCGO_ENABLED=0でcross buildし、native fresh配置を確認する。
固定Codex0.153.4の既存限定live方式で、小さい入口→現在手順→begin→作業→段階変更後の手順取得を観測する。
実機の自己申告・timeout・未起動を成功扱いしない。必要なtest fixtureは既存契約を弱めず更新する。
最終差分が変われば修復・再review後にfinalを更新する。PRの現在head checks成功後にmerge・Issue closeを確認する。

## 本家との差と影響

固定ローカルAI-DLC2.6.123の段階frontmatter・分析・compile hook冒頭を参照した。
本家は生成stage-graphを実行時ビューとして扱い、audit eventと連動する。
本製品では合意済み4段階を保ち、JSONを遷移、MDを担当・文書参照・手順の正本にする。
理由は読込み量と二重管理の削減。利用者は全WORKFLOWを毎回読む必要がなくなるが、対応する新配置が必要になる。
33段階やaudit/receipt機構を再導入しない。最新upstreamおよび未確認の生成経路との一致は主張しない。
