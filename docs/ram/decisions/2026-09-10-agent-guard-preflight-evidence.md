# G0 fixtureのloop実装証拠

状態: fixtureのloop完了、独立review・実機final待ち。2026-09-10。

対応Issue: #159。実装許可は[直接承認](2026-09-10-stage-agent-worker-guard-approved.md)、作業単位は[agent-guard-preflight](../../design/agent-guard-preflight-work-unit.md)。開始・終了HEADは `e2566c2bf1634436b435c6abbfd27d26b17232e3`（未commitの実装差分）。製品コード・既存helper・外部依存を変更していない。

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
