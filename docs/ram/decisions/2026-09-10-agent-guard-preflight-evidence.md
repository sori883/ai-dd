# G0 fixtureのloop実装証拠

状態: loop実装・修復の履歴。2026-09-10。[実測結果と未確定事項](2026-09-10-agent-guard-preflight-result.md)を別記し、最終ソースの独立reviewとfresh finalは対応PRへ記録する。以下の実機未実行・検証待ちは各節の作業時点を示す。

対応Issue: #159。実装許可は[直接承認](2026-09-10-stage-agent-worker-guard-approved.md)、作業単位は[agent-guard-preflight](../../design/agent-guard-preflight-work-unit.md)。初回loopの開始HEADは `e2566c2bf1634436b435c6abbfd27d26b17232e3`（初回の実装結果は未commit差分として返却）。製品コード・既存helper・外部依存を変更していない。

## RED／GREEN証拠

| Slice | exact command | RED | GREEN |
| --- | --- | --- | --- |
| P1 | `go test -count=1 ./src/cmd/aidlc -run '^TestAgentHookProbeProtocol'` | exit 1。compile-only scaffoldでstdoutが `PASS`、保存件数0、保存失敗時も成功する期待値不一致 | exit 0。指定spawnの拒否、許可対照、raw・応答・exit保存、並行一意記録、保存失敗を確認 |
| P2 | `go test -count=1 ./src/cmd/aidlc -run '^TestAgentHookProbeEvidence'` | exit 1。空評価器が結果を返さず、修正済み期待値が不一致 | exit 0。曖昧・欠落・重複・親cwd・prompt・Stopだけではpassにしない |
| P3 | `go test -count=1 ./src/cmd/aidlc -run '^TestAgentHookProbeFixture'` | exit 1。固定版判定false、fixture rootなし、process記録なし、故障応答未実装、caseなし | exit 0。opt-in、固定版・OS、隔離setup、有限process、故障応答、case一覧を確認 |

P2の最初のテストには、許可対照の試験印なしでpass、子との対応のない残存process boolだけでunsupportedを期待する誤りがあった。実装前に自己点検で停止し、親が計画を維持する通常のテスト設計修正として明示指示した後、前者へ試験印を追加し、後者をinconclusiveへ直して改めてREDを観測した。誤った初回期待値は有効なcycleとして扱わない。追加の回帰検査で、試験印の子tool呼出しが欠けるとpassしていたRED（exit 1）を確認し、子sessionのPostToolUse・command・nonce・exitとの照合後GREEN（exit 0）とした。

P3ではmacOSの `/var` と `/private/var` の別名でpath比較が失敗した。親の明示指示に従いテスト期待pathも `EvalSymlinks` で正規化した。別repoをworktreeと扱わないため、実Git worktreeの `.git` ファイルがないRED（exit 1）を追加し、一時repoの空commitと `git worktree add --detach` でGREEN（exit 0）にした。Git identity・hooks無効化は各コマンドの `-c` に限定し、既存/global設定は変更しない。

親の指示したlifecycle予算5分に対し既定2分を返すRED（exit 1）を確認し、case別上限とmanifest記録を加えてGREEN（exit 0）とした。

cleanup前のprocess観測ファイルがないRED（exit 1）も追加し、観測snapshotを保存してGREEN（exit 0）にした。末尾で上記3commandを再実行し、変更Go fileのgofmtと `git diff --check` を確認した。

## 実機で得る資料と判定境界

実機entry pointは `AIDLC_AGENT_HOOK_LIVE=1 go test -count=1 -timeout=30m ./src/cmd/aidlc -run '^TestAgentHookProbeLive$' -v`。loopでは実行していない。全package、race、vet、integration、cross-buildも実行していない。

一時ディレクトリ `aidlc-agent-hook-probe-*` を保持し、deny、allow、parallel-roots、lifecycle、unit-and-alias、missing-response、nonzero-exit、save-failure、hook-timeoutの9caseを独立実行する。lifecycleは子の複数turnを含むため5分、他caseは2分で打ち切り、上限をmanifestへ記録する。全caseの上限合計は21分で、独立した後続caseは継続する。各caseに `manifest.json`、`prompt.txt`、`command.json`、`execution.json`、`events/event-*.json`、`transcript-SESSION.jsonl`、`calls.json`、`summary.json`、`processes/process-NONCE.json`、cleanup前の `process-observations.json`、`cleanup.json` を保存する。manifestは各保存先を示す。

実際のtool名・input/output・未知JSONを保持する。code-modeの外側callを内側toolの成功へ読み替えない。各caseの `summary.json` は観測event/tool件数とcall件数を示すが、モデルが要求を実施したかは親が `calls.json` とraw transcriptを照合して確定する。根拠不足のgateを自動的にpass/unsupportedへ変更しない。固定版で確認できていないroot schema、追加依頼・再開・停止、Unitあり/なし・別調整root、hook故障時の実tool挙動はinconclusiveを既定とし、実機rawに基づく親の評価が必要。

有限processは独自nonceで排他的な観測ファイルを作り、最大20秒以内で終わる。cleanupは同じnonceの停止用ファイルだけを作り、PIDへのsignalや他processの停止はしない。process記録のnonce・時刻は生hookと実tool呼出しの照合資料であり、子agent_idやrootを自己申告から確定する証拠ではない。

既存Codex 0.153.4 macOS arm64、gpt-6-astra/xhigh、通常CLI認証を使う。認証fileの読取り・複製をしない。`--ignore-user-config`、一時rootのtrustとhook trust bypassを使い、`--add-dir` は同じ専用tempの証拠保存先だけをsandboxの書込み対象へ含める。製品の導入手順・保証ではない。現時点でG0の実機pass/unsupportedは未確定。

## 独立review後のcollector接続修復

`work_unit_id=agent-guard-preflight-review-repair`、開始HEAD `dd95e3f41726c4a5d0659e8e018b3471a7092a10`。Issue #159と同じ承認範囲で、親から一つにまとめて委譲されたblocking findingを修正した。

reviewでは、live collectorから許可対照が評価器へ渡らない、全tool callが1件だけという条件が正常なwait/closeを拒む、raw function string/custom array wrapperを直接agent_id objectとして扱ってしまう、というG0-1経路の欠陥が見つかった。

回帰commandは `go test -count=1 ./src/cmd/aidlc -run '^TestAgentHookProbe(Evidence|Fixture)'`。実ファイルのhook・親/子transcript・manifest・process記録をcollectorへ読み込ませ、aggregate→評価器へ渡す `TestAgentHookProbeEvidenceCollectedControl` を追加した。API scaffold段階のREDに加え、既存collectorと評価器を接続した時点でobject/function string/custom arrayの正例がすべてinconclusiveとなる意図したRED（exit 1）を確認した。修正後は同じcommandでGREEN（exit 0）、末尾再実行も成功。

collectorはraw call outputを変えず、出典sessionとprocess観測を含む証拠を返す。live末尾で独立したdeny/allowの証拠を集約し、実験rootの `aggregate-summary.json` にG0-1評価を保存する。spawnだけを一意に選び、無関係なwait/closeが存在しても照合できる。既知のJSON object、function outputのJSON string、固定CLIで確認済みの2つのinput_text block形式だけを評価時に正規化する。code-mode JavaScriptから仮想の内側callを作らない。

許可対照の試験印は、fixture manifestのnonce・有限helper command、実子sessionのPostToolUse、同session/同call IDの実exec_command入出力、nonceを持つ終了済みprocess記録を照合して判断する。markerのagent/nonceを手組みで注入する経路は削除した。回帰fixtureではwait/closeを含む正例に加え、未知wrapper、重複spawn/出力、session不一致、process欠落、nonce不一致、子call欠落、opaque code-modeをinconclusiveとして確認した。

これは既知wireに対する収集・評価経路の修復であり、固定0.153.4の実spawn wireがその形式で得られると断定するものではない。実機未実行を維持し、未知wireや根拠不足はinconclusiveとする。G0-2〜6のraw実測評価は引き続き親が行う。gofmt適用と `git diff --check` を確認し、製品コード・外部依存・既存helper・Issue/PRを変更していない。

## 非lifecycleの有限process待機時間修復

`work_unit_id=agent-probe-yield-repair`、開始HEAD `b9af85a09c28a03ee7c45895a9fa846a54e4525b`。再reviewの指摘に従い、15秒の有限helperに対する非lifecycleの `exec_command` を `yield_time_ms=20000` とpromptへ明示した。lifecycleはprocess残存を観測するため `yield_time_ms=1` を維持する。markerのexit 0要件や未知wireの扱いは変更しない。

先行した `TestAgentHookProbeFixtureYield` は全9caseの保存済みpromptを検査する。`go test -count=1 ./src/cmd/aidlc -run '^TestAgentHookProbeFixture'` で、非lifecycleの20秒指定が欠けるRED（exit 1）を確認した。lifecycleの既存1ミリ秒指定は初回から成立（ALREADY_GREEN）。case別指示の修正後に同じcommandがGREEN（exit 0）。末尾再実行、gofmt、`git diff --check` も成功。実機live・全体検証・Issue/PR操作は未実行。

## 初回実機finalの結果と実測wire追従修復

`work_unit_id=agent-probe-observed-wire-repair`。開始HEADは `4a8dc3fbb0b3fed5ed491c24f915c5c8af937060`。親が実行した初回finalの結果は `/var/folders/9w/921pjkys39q28sk4xsc0hs000000gn/T/ai-dd-agent-guard-final-vpgzpo5u/results.json`、実験rawは `/var/folders/9w/921pjkys39q28sk4xsc0hs000000gn/T/aidlc-agent-hook-probe-579906995` に保存された。結果JSONを読み、旧HEADで全test、race、vet、integration、6構成build、format/tidy/diff、CLI help/version、新live runnerがexit 0だったことを確認した。新live runnerは553.28秒で終了した。

ただし初回の自動G0判定は全項目inconclusiveだった。実hookのtool名が `collaborationspawn_agent` であるのにfixtureがbare `spawn_agent` だけを判定していたため、deny/faultが発火しなかった。runnerのexit 0を起動拒否成功やG0成功と扱わない。今回のコード変更により、初回finalは新コードの成功証拠としてはstaleとなる。再review後のfresh finalが必要。

### 維持する初回G0-4の有効な実測

lifecycleのmanifest nonce `f85ed37c4de8d0c05ae3c8acf0e4c374` と `processes/process-f85ed37c4de8d0c05ae3c8acf0e4c374.json` のnonceが一致した。`events/event-1680116618.json` のBash Pre commandはmanifestの有限helper commandと完全一致し、子agent_idは `01a0874b-6c42-7dd1-b974-a17305f4773a`、親sessionは `01a0874b-3374-70a3-ade4-b0cb5bbe39ab` だった。

- helper StartedAt: `2026-09-09T17:51:12.692478Z`
- 同子のSubagentStop記録 `events/event-1483143723.json` のObservedAt: `2026-09-09T17:51:16.770721Z`
- helper EndedAt: `2026-09-09T17:51:27.723715Z`

同じnonceのhelperはSubagentStop後も約10.953秒続いた。したがって、SubagentStopだけから残存process終端やworker枠の安全な解放を保証する案は、この実測では成立しない。これは単なるモデル未操作ではなく、親/子・command/nonce・process記録の対応がある観測である。初回のdeny/fault未発火やG0-2等の未確定とは分けて保持する。任意の外部processや全再開経路の保証へ拡張しない。

### 修復した3項目とTDD証拠

1. `TestAgentHookProbeProtocol` に観測済みtool名でのdenyとmissing/nonzero/save-failure/timeoutの回帰を追加。`go test -count=1 ./src/cmd/aidlc -run '^TestAgentHookProbeProtocol'` で、全て通常応答 `{}` となるRED（exit 1）を観測後、bare名と観測済み名の完全一致だけを認めてGREEN（exit 0）。probe_workerのPre限定を維持し、類似名・別role・Postへは拡大しない。
2. `TestAgentHookProbeEvidenceObservedWire` で実測形を小さな実ファイルへ再現。`go test -count=1 ./src/cmd/aidlc -run '^TestAgentHookProbe(Evidence|Fixture)'` で正例がinconclusiveになるRED（exit 1）後、collector→aggregateの照合を修正してGREEN（exit 0）。spawnの直接callとnative Pre/Postを結び、返却task_name、Startの明示agent_id/path、child `session_meta` のid/agent_path/parent_thread_id/forked_from_idを厳密照合する。child bodyは先頭metadataが一致してから収集し、未知・欠落・重複・不一致はinconclusive。rawのCRLFが変更される追加RED（exit 1）を確認し、改行を含む元bytesを保持してGREEN（exit 0）。
3. `TestAgentHookProbeFixtureObservedSchema` で、有効なtask名・条件付きfork_turns・実schemaに沿うlifecycle指示がないRED（同combined command、exit 1）後、promptだけを修正してGREEN（exit 0）。worker_a/worker_b等とroot pathを区別し、schemaにある場合だけ `fork_turns="none"` を指定する。followup_task/send_message/interrupt_agent/list_agents等は実際に公開された意味で使い、idleからの再開と実行中割込みを分ける。追加依頼は短い返答だけでhelper再実行・nonce上書きを禁止し、closeがない場合は完了/割込みで代替しない。9caseと既存時間制限を維持する。

Bashは親session＋子agent_id＋内部exec IDの実Pre/Post、command/nonceとhelperのprocess印を照合する。実測のBash Post tool_responseは空文字であり、exit 0や完全停止へ読み替えない。外側execのJavaScriptから仮想的な内側callを生成せず、外側execがあることだけを理由に直接spawnとnative hookの証拠を捨てることもやめた。

このmetadata対応はG0試験限定の観測候補であり、製品向けの安定API承認ではない。同じtask_nameの再使用・再試行での一意性、transcript APIの安定性、予定worker rootの構造化対応は未確認。fork_turns変更後にも必要metadataが得られるかはfresh liveで確かめ、不足時は推測しない。通常のroot cwdや自然文から割当rootを確定しない。

末尾で上記2commandを再実行し、gofmtと `git diff --check` を確認した。このloopではlive・全体test・race・vet・integration・cross-buildを実行せず、製品コード・依存・Issue/PR・既存helperも変更していない。

## race計測時のhook応答待機修復

`work_unit_id=agent-probe-race-timeout-repair`、開始HEAD `d637a46389241e0228d81f47f03ea3df0ad6a8b2`。親のfresh finalでintegration/testは成功したが、race実行は `/var/folders/9w/921pjkys39q28sk4xsc0hs000000gn/T/ai-dd-agent-guard-final-p449cv2c/race.log` の `TestAgentHookProbeProtocolObservedFault/missing` が0.50秒で `signal: killed` となって失敗した。DATA RACE警告はなく、正常終了を待つcaseにも共通500ms deadlineを使ったtest fixtureの問題だった。この既存回帰の実測失敗を修復根拠として保持する。

loop開始時の `go test -count=1 ./src/cmd/aidlc -run '^TestAgentHookProbeProtocolObservedFault$'` は通常buildでexit 0（ALREADY_GREEN）。人工REDやdeadline値を鏡写しに検査するtestは作らなかった。通常のmissing/nonzero/save-failureは既存helper呼出しと同じ3秒待機、意図したtimeoutだけhelperの3秒sleepより短い1秒deadlineへ分離した。missingは正常exit 0とcontext未取消、nonzero/save-failureは実exit code 2/74とcontext未取消を検査し、deadlineによるkillを故障応答成功にしない。timeoutはdeadline到達と呼出し失敗を区別して確認する。

修正後および末尾の同targeted commandはexit 0。gofmtと `git diff --check` も成功。loopではraceを再実行せず、親の再review/fresh finalへ返す。変更はprotocol testと本記録・索引だけで、実装・依存・Issue/PRは変更していない。

## 2回目実機結果と観測済み省略fieldの修復

`work_unit_id=agent-probe-observed-optional-fields-repair`、開始HEAD `05df6d1e951eb22e38b8a674079f1a9e6040cb20`。親の2回目final結果 `/var/folders/9w/921pjkys39q28sk4xsc0hs000000gn/T/ai-dd-agent-guard-final-45kzg_s3/results.json` を読み、同HEADのnormal test/race/integration/vet、6構成build、tidy/format/diff、CLI help/version、新live runnerのexit 0を確認した。raw保存先は `/var/folders/9w/921pjkys39q28sk4xsc0hs000000gn/T/aidlc-agent-hook-probe-3375108718`。

自動G0-1は新たに観測された拒否文字列とfork_turns=none時のmetadata省略に未対応のためinconclusiveだった。一方、rawのdenyはPreでdeny/selected exit 0を返し、実function outputに拒否文、子Start 0件・process印0件があった。allowは子Start 1件・process印1件で、child ID、agent_path、明示parent_thread_id、source内の親IDが一致していた。forked_from_idは欠落していた。モデルの説明だけで拒否成立を判断したものではない。

### 既存故障caseとlifecycleの観測

4つの故障caseすべてでspawn実結果がtask_name `/root/g0_probe` を返し、子Startが1件あった。

| case | Pre記録のselected exit | Pre記録の応答 | 子Start |
| --- | --- | --- | --- |
| missing-response | 0 | 空 | 1 |
| nonzero-exit | 2 | 空 | 1 |
| save-failure | 74 | 空 | 1 |
| hook-timeout | 0 | 空 | 1 |

これらのhook故障条件で起動が拒否されたとはいえず、故障時に必ず拒否する保証へは使えない。selected exitはhelperが記録した予定の終了値で、特にtimeoutで実際にその値を返した証拠ではない。既存の試験用故障helperの実測結果であり、fixtureの未発火やコード異常とは分ける。

hook-timeoutの同spawn Pre記録は `2026-09-09T18:30:52.189277Z`、Post記録は `18:30:53.291216Z` で、差は1.101939秒だった。fixtureのhook timeout設定は1秒、対象helperは記録後3秒sleepする。したがって、3秒待機するhookの完了前（約1.1秒後）にspawn Postと子開始へ進んだ観測である。stdout/stderrに明示timeout診断は確認されておらず、OSがhook processをkillしたとは主張しない。

lifecycleではfollowup_task、send_message、interrupt_agentのPre/Postが各1組、list_agentsが4組記録された。ただしinterrupt_agentの実結果は `previous_status.completed="Acknowledged."` で、すでにcompletedの子に対する操作だった。実行中の割込みはこの実験では未実証。特定操作名の存在やhook記録だけを、active割込みや完全停止の成功へ読み替えない。

### 最小修復と検証

実ファイルのcollector→aggregate回帰 `TestAgentHookProbeEvidenceObservedOptionalFields` を先に追加し、`go test -count=1 ./src/cmd/aidlc -run '^TestAgentHookProbe(Evidence|Fixture)'` で、観測拒否文＋fork field存在/欠落の2正例がinconclusiveになるRED（exit 1）を確認した。

修正は次の2点に限定した。

- 実測JSON stringの完全な拒否文 `Tool call blocked by PreToolUse hook: Expected G0 probe denial. Do not retry or substitute another agent.. Tool: collaborationspawn_agent` だけを既知の拒否errorへ正規化。別tool・別理由・類似文は認めない。
- forked_from_idをraw fieldとして保持し、欠落時だけ許容。存在するnull・空文字・不正型・別親は拒否する。collectorと評価器は同じ検査を使い、明示parent_thread_id・child ID・agent_path・Pre/Post/Startの厳密な対応を維持する。

同commandは修正後と末尾でexit 0。gofmtと `git diff --check` も成功。このloopではlive/full test/race等を再実行していない。scenario・matcher・lifecycle・製品コード・依存・Issue/PRは変更していない。省略fieldへの対応はG0観測限定で、製品APIの安定性を承認するものではない。今回のコード変更で旧finalは新コードに対してstaleとなり、親の再review/fresh finalが必要。
