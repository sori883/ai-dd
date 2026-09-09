# G0: 固定Codexの担当起動・終了・再開を観測する作業単位

状態: 実施許可あり。[ユーザー直接承認](../ram/decisions/2026-09-10-stage-agent-worker-guard-approved.md)に基づく前提検証。製品guardの実装とは分ける。

実施後の追記: [実測結果](../ram/decisions/2026-09-10-agent-guard-preflight-result.md)へ成立した条件と未確定を記録した。以下は実施契約であり、製品guardのgate通過を表すものではない。最終ソースのreview/finalは対応PRに記録する。

対応Issue: [#159 固定Codexで担当起動制限の前提を実測する](https://github.com/sori883/ai-dd/issues/159)。

## 背景、成果、範囲

製品hookでステージ外の担当と同一worktreeのworker重複を防ぐためには、Codexが起動前に拒否を受け付け、要求と子ID/rootを対応付け、再開と停止を観測できる必要がある。公開hook仕様だけでは固定0.153.4の実挙動を確定できない。

Goの試験用hookと証拠検査器、一時プロジェクトで動くopt-in live testを作る。成果はG0各項目のpass/unsupported/inconclusiveの区別とraw証拠であり、製品が制限を実装済みという結果ではない。unsupportedは実測または固定版の確実な根拠がある場合だけ使い、単にモデルが要求を行わなかった場合はinconclusiveとする。失敗した実験をskipや合格へ置き換えない。

開始基準はmain `c990f7629c69dd0a3f11ad1994dd69c165e70749`。作業treeは`/Users/const/sori883/ai-dd-stage-okf-documents`、branchは`codex/agent-guard-preflight`。元checkoutの未commit資料を保全する。

## 所有範囲と単独writer

`work_unit_id=agent-guard-preflight`。Go担当1名が次の新規ファイルを所有する。

- `src/cmd/aidlc/agent_hook_probe_test.go`: 観測用hook subprocess、記録形式、証拠の評価補助。
- `src/cmd/aidlc/agent_hook_probe_protocol_test.go`: hook出力・保存・証拠検査器の単体テスト。
- `src/cmd/aidlc/agent_hook_probe_live_test.go`: `TestAgentHookProbeLive`。`AIDLC_AGENT_HOOK_LIVE=1`の時だけ実Codexを呼ぶ。
- `docs/ram/decisions/2026-09-10-agent-guard-preflight-evidence.md`: loop証拠。索引更新。

親が計画・承認RAM・Issue/PRを管理する。既存の`minimalProbeQuote`等の純粋なtest helperを再利用してよいが、既存helperと製品コードを変更しない。実験用agent/TOML/hooks/一時Git rootはtestが専用temp配下へ生成し、製品の配布定義・hooks・registryは変更しない。G0が未確定のため、製品7項目TDDとは別の作業単位とする。

## 観測の契約

イベントは元のJSONを保存し、未知fieldも診断用に保持する。公開済みの共通field（event、session/turn、tool_use_id、tool_name、tool_input/tool_response、agent_id/agent_type）以外を必須schemaとして発明しない。各hook記録に自身が返したJSON/exitも保存する。ファイル名は一意とし、並行到着や重複配信で上書きしない。stdoutへGo testのPASS等を混ぜない。

試験用Preは指定した起動要求をdenyし、同じagentの許可対照では子が実行できることを確認する。G0-1では拒否応答だけでなく、親の実tool結果と、子Start/試験印の有無を照合する。新規JSON fieldの捏造や、自然言語promptから予定rootを抽出して対応が取れたと扱うことは禁止する。

G0-2は実tool入力/応答、子Start/Stopと、その作業rootの構造化証拠を保存する。同一親から別worker rootへの並行開始を試み、現在のtool schemaが構造化root指定を持たない等の不明点はそのまま記録する。`cwd`が親の作業場所なら子の予定rootの証拠にしない。子の「移動した」という説明やファイル作成だけでagent割当rootを確定しない。

G0-3/4は実在する追加依頼・再開・割込み・close操作のPre/Postを記録し、子の返答、再利用、Stop、外部process終端を区別する。tool名は実際に提供されたものを使い、無い操作を別名で成功したことにしない。親や子が一turnを終えた後でもprocessが残るかを、有限時間で終了する試験用helperの観測で確かめる。全試験processに一意なnonceと専用tempを使い、他processを停止しない。終了待ち・cleanupは時間制限付きで行い、タイムアウトでも証拠を残す。

G0-5はUnitあり/なしの実現に必要な対応情報、同じ実worktreeを別path/別調整rootから指定した時の観測差を整理する。製品Unit APIの状態変更を実子の状態として流用しない。G0-6は試験hookの保存失敗・応答欠落・非ゼロexit/timeoutに対するCodexの挙動を観測する。意図したdenyと、Codexがhook故障後に実行する場合を分ける。

各caseは独立したrootまたは明示的な同一session継続として記録し、実験間の混入を防ぐ。raw transcriptを使う場合は試験sessionのものだけを読み、安定した製品APIとは見なさない。モデルの説明文だけでgateをpassにしない。

## 順序付きTDD（loop）

以下のtest名は新設予定。新しいtest-only型/関数は、runnable testへ到達するための空の型・signature・ゼロ値返却だけ先に置くcompile-only scaffoldを許可する。実験fixtureを製品仕様と取り違えない。

| slice | 振る舞いと受入 | exact targeted command |
| --- | --- | --- |
| P1 | 試験用hookが指定spawnだけへ有効なdeny JSONを返し、許可時は通常応答。Raw・応答・未知field・並行記録を保持。保存失敗の扱いを検査 | `go test -count=1 ./src/cmd/aidlc -run '^TestAgentHookProbeProtocol'` |
| P2 | 証拠評価が拒否と子開始の矛盾、IDの曖昧対応、親cwd、欠落/遅延/重複、Stop後の残存processを誤ってpassにしない。inconclusive/unsupportedと実測passを分ける | `go test -count=1 ./src/cmd/aidlc -run '^TestAgentHookProbeEvidence'` |
| P3 | opt-in既定off、固定版/OS検査、試験root以外の無変更、prompt/コマンド/証拠保存、有限processとcleanup、実機未実行の契約を静的/helperレベルで検査 | `go test -count=1 ./src/cmd/aidlc -run '^TestAgentHookProbeFixture'` |

各sliceでtest-first、runnable RED→GREEN。文書だけの変更や初回GREENへ人工REDを作らない。loop中にCodexモデルを呼ばず、上記targetedとgofmt/diff確認だけを実行する。親は末尾に差分と同targeted群を確認する。

## 独立reviewとread-only final

review担当は固定base/headを読み、試験が都合のよいmockだけを確認していないか、実子の寿命とtool呼出しを区別するか、根拠不足をpassにしないか、他の設定やprocessを変更しないかを確認する。追加実行は必要なtargetedだけ。

固定HEADのfinalで親が全test/race/vet、format/tidy/diff、全integration、6構成buildを1回に集約する。既存の製品liveを無関係に全て再実行しない。次の新liveだけを明示実行する。

`AIDLC_AGENT_HOOK_LIVE=1 go test -count=1 -timeout=30m ./src/cmd/aidlc -run '^TestAgentHookProbeLive$' -v`

対象は既存Codex CLI 0.153.4、macOS arm64、gpt-6-astra/xhigh。既存認証を通常CLI経由で使い、認証fileは読まない/複製しない。`--ignore-user-config`と試験専用trust設定を使う。hook trust bypassが試験起動に必要なら一時rootの検証用と明示し、製品の導入手順にはしない。CLI以外のDesktop tool経路の保証はこのliveから導かない。

保存形式/runnerの不具合ならloopへ戻して修復・再review・fresh final。Codexの技術的な未対応が判明した場合はそれを結果として記録し、要件を弱めて再実装しない。G0の結果を整理するIssueは、実験が正しく完了して根拠と未確定を記録できれば完了可能であり、製品guardの完成とは分ける。GitHub checksと独立review成功後のPR/mergeは承認済み運用に従う。
