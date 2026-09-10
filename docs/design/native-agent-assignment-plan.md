# メインAIによる起動を維持した担当・作業場所の管理

日付: 2026-09-10。状態: [具体計画全体とQ1・Q2を直接承認済み](../ram/decisions/2026-09-10-native-agent-assignment-implementation-approved.md)。単独writerの実装を開始する。

## 背景、目的、承認範囲

AI-DLCは、Intentという一つの目的を持つ作業のステージ・承認・成果物を管理するGo製CLIである。担当エージェントはメインAIがCodexの標準ツールで起動する。並列実装ではGit worktreeという独立した作業フォルダを使う。

現行のUnit割当は同じIntentの一部の状態だけで競合を検査するため、別Intentや結果提出後の作業場所を横断して保護できない。担当一覧はステージ定義にあるが、製品hookはまだ担当の起動を検査しない。

今回の目的は、現在ステージで許可された担当を起動し、同じ管理元のCLIが使用中のworktreeを複数workerへ二重に割り当てないことである。CLIは子AIを起動しない。メインAIはCLIの登録結果を受け取り、`spawn_agent`で直接起動する。

[A案の直接承認](../ram/decisions/2026-09-10-managed-worker-assignment-approved.md)により、この方針と保証対象の変更が許可された。任意の起動経路・実workerや残存processの全稼働を検知して制限する保証は持たせない。Goシングルバイナリ、外部Go moduleを追加しないこと、別worktreeの並列実装、Unitなし作業、1調整rootの全Space/Intent/sessionを対象とする境界を維持する。

従来の条件付き[稼働制限計画](stage-agent-worker-guard-plan.md)の製品部分は本計画へ置き換える。完了したG0の証拠は維持する。旧33 Stageの包括承認は使わない。Q1・Q2と本具体契約への直接承認を根拠に、この計画全体を1 Issue/PRの単独writerへ渡す。

## 利用手順とCLI案

以下のコマンドは実装予定であり、現時点で実行できる操作ではない。`A`は配置したaidlcバイナリ、`ID`はIntent ID、`SPACE`はSpace名、`R`は最新のIntent revision（保存版）とする。

1. 管理元で `A assignment init --file init.json` を一度実行する。メインAIはここで返されたepochを使う。epochは管理情報を作り直した世代を区別するIDである。
2. メインAIが通常のGit操作で別worktreeを準備する。CLIはその既存worktreeを検証する。新たなworktree作成engineや起動launcherは作らない。
3. Unitなしなら `A assignment reserve ID --space SPACE --session MAIN --expect R --file reserve.json`。Unitありは既存の `unit claim` / `unit reassign` を横断予約へ接続する。どちらも同じ内部予約処理を使う。
4. `A assignment list` / `A assignment show ASSIGNMENT` で、担当・root・task_name・予約状態を確認する。メインAIがそのtask_nameとagent_typeを標準の `spawn_agent` へ渡す。依頼文には担当手順・登録したroot・Unit範囲・必要文書を含める。
5. 同じ子へ追加依頼する前は `A assignment check ASSIGNMENT` で最新条件を確認する。hookもその時点のステージと担当を再検査する。
6. 結果は既存のUnit結果提出またはIntentの成果物・検証手順で回収する。結果を提出しても予約は残る。
7. Q1の確認を済ませ、`A assignment release ASSIGNMENT --session MAIN --expect ENTRY_REV --file release.json` で場所を解放する。新たな担当には新しい予約とtask_nameを発行する。

Unitなしreserve.jsonの例:

```json
{
  "registry_epoch": "initで得たID",
  "request_id": "この登録要求の一意なID",
  "step_id": "現在の実行step ID",
  "agent": "aidlc-worker",
  "root": "/project-worktrees/feature-a",
  "session": "worker-a"
}
```

`--session MAIN`は起動するメインAIの会話、JSONのsessionは既存Unit APIと同じ割当用識別子である。後者はメインAIが決める文字列であり、Codex実child sessionの本人確認情報ではない。結果提出まで同じ値を使う。

reserveは `assignment_id, registry_epoch, entry_revision, task_name, agent, root, space, intent_id, step_id, status` をJSONで返す。Unit CLIは既存のState出力を維持し、割当情報はlist/showで取得する。`unit claim/reassign`の入力には `registry_epoch, request_id, coordinator_session` を追加し、既存session/root/run_id・依存・base・scope検査を維持する。Unitなしに架空のUnitは作らない。

CLI解析は既存のParseMinimalへ追加し、JSONの不明field・必須値・長さ・列挙値を検査する。helpに有効な値・Unitあり/なしの例・再試行・復旧条件を収め、Skillはhelpを参照する。CLIの成功出力と診断出力、終了codeは既存規約に合わせる。

## 担当の起動と再依頼を検査するhook

担当一覧の正本は `src/core/workflow/stages/*.md` のagentsである。`Procedure.Agents`から読み、第二の許可一覧は作らない。承認済み方針に従い `aidlc-stage-planner` を既存6ステージへ追加する。

正常なPreToolUseでは、session/turn/tool_use_id、選択Intent、現在の定義・step、当該turnのRule読込みを確認し、要求のagent_typeを現在ステージのagentsと完全一致で照合する。未選択・不明担当・定義不一致は拒否する。workerはさらに有効な未解放予約、承認、開始Sensor、Unitがある場合の有効な割当を要求する。担当の人数全体には上限を設けない。

read-only担当は既存の文書を調べ、質問・計画案・レビュー結果を返す。worker用の作業開始Sensorを一律に課して、開始Sensorを直すための調査まで禁止しない。pendingの計画・人間承認中は、現在ステージで許されたread-only担当への依頼を認める。workerは承認完了まで拒否する。paused/cancelled/finishedでは新しい作業依頼を拒否し、停止操作・診断・解放は可能にする。

workerのtask_nameはCLIがepochと予約IDから払い出し、メインAIがそのままspawnへ渡す。Preは登録済みの値だけを許す。read-only担当についても、再依頼先の担当を照合するため、親sessionとtask_name、担当、Intent、step、tool_use_idの最小対応をPreで保存する。同じ親sessionの名前を別の起動で再利用しない。これは担当の対応表であり、全操作auditではない。

Pre/Postの対応キーは親sessionとtool_use_id。Postの `tool_response` は固定G0で観測したJSON文字列/構造を厳格に解析し、返されたcanonical task pathを登録する。これを実agent IDや実行rootの証明とは呼ばない。本文からrootを推測せず、製品は非公開transcriptを解析しない。

followup_task/send_messageのtargetは、同じ親sessionで登録・応答確認できたtask_nameとcanonical task pathに完全一致する場合だけ扱う。任意aliasの推測や曖昧な正規化はしない。実測済みの相対task_nameを標準手順とする。各依頼時点で同じstep・担当と未解放予約を再検査する。step変更や解放後は古い子へ再依頼せず、新しい登録で起動する。

起動のPostが欠けた場合は対応不明として保持し、そのtaskへの追加依頼・同名の再spawnを許可しない。workerではメインAIが旧依頼と既知の処理を確認して予約を解放し、新しい予約で再試行する。read-only担当はworker場所を占有しないため、旧taskの対応不明を残したまま、現在の担当検査を通る別名の新規起動を認める。旧taskが停止したとは判断せず、そこへの再依頼は禁止したままにする。interrupt_agentによる停止要求はステージ変更後でも可能とし、そのPostで場所を解放しない。

hook入力の対象はG0で観測した `collaborationspawn_agent`、`collaborationfollowup_task`、`collaborationsend_message`、`collaborationinterrupt_agent` と、明示して試験する標準名だけ。既存Bash/apply_patchの処理と分離し、Session.Toolを子の生存状態には使わない。子のSubagentStart/Stopを解放処理に接続しない。

rootとしてCLIへ登録した場所に実childが必ず入る保証はない。メインAIが登録した場所を依頼し、成果回収時は既存のcommit/root/Unit検査で確認する。hook自体が未信頼・故障・未到着の場合にCodex側を完全に遮断する保証も付けない。

## 保存場所、原子性、再試行

調整rootの `aidlc/.runtime/assignments/registry.json` を現在管理情報の正本とする。Git対象外で、全Space/Intent/sessionから共通に参照する。Git共有のIntent/Unit進捗stateとは分離する。

schema 1にはepoch、revision、調整root、worker割当、担当task対応、再試行のための最小記録を置く。worker割当はID、request_id、要求hash、親session、割当session、Space/Intent/step/定義hash、正規化root、任意Unit/run_id、entry revision、状態、対応taskを持つ。状態は `reserved / dispatch_pending / bound / uncertain / released` とし、`bound`は起動応答の対応が得られた意味だけを持つ。AIが稼働中・停止済みという判定ではない。

rootは実在する絶対path・symlink解決・Git worktreeのtop-levelと同じrepositoryへの所属を確認する。同じ実体の別名は同一場所として拒否し、調整root自身はworker場所にしない。別rootは双方を登録できる。OS上のroot差替えや別PC/別管理元までの排他へ広げない。

lock順序は必要なものだけ `session → flow-SPACE → assignments`。既存filestore.Lockを使い、検査と予約保存を同じ横断lock内で行う。他Spaceのflow lockは取得しない。assignment lockからflowへ呼び戻さない。単一ファイルの置換保存を使用し、保存が完了する前に成功を返さない。

同じepoch/request_idと同じ要求内容の再試行は同じ結果を返す。別内容でのID再使用は拒否する。解放後も最小のID・hash・released状態を保持し、古いreserve再試行で新たに場所を占有しない。日時やTTLでは削除しない。

既存filestoreの256 KiB上限内で保存する。新規受付時に解放・応答更新用の余裕も確保し、容量不足では新規受付を拒否する。list/show/releaseを容量だけで塞がない。全件解放後の明示的なepoch更新で管理情報を切り替えられるようにし、旧情報は保管する。自動GCや全操作履歴は追加しない。

## Unit連携、故障、中断、解放

Unitのclaim/reassignとUnitなしreserveは共通の横断予約処理を使う。Unitでは `予約保存 → 既存のUnit割当保存 → 進捗保存` の順にし、途中失敗を識別する元revision・要求hash・run_idを予約へ保持する。同じ要求による復旧だけを許し、復旧完了前は新しいworkerを開始しない。進捗保存まで成功した後に応答だけ失っても、再試行で二つ目の予約を作らない。

`unit result`、`unit integrate`、Intentのpause/wait/finish、spawn Post、SubagentStopは自動解放しない。Unit reassignは既存の `previous_run_stopped` と理由を確認し、Q1で許可した停止確認を根拠に旧予約の解放と新予約を処理する。旧予約と新予約をUnit進捗だけから推測しない。

release.jsonの通常案は `registry_epoch, request_id, previous_run_stopped:true, no_more_requests:true, reason`。登録したメインAIの会話から、追加依頼を終えることと、依頼した既知のコマンド・background処理が終わったことを確認して登録する。成果・残件の回収も理由へ要約する。これは確認申告をプログラムが保存する契約で、OS上の全process停止の認証ではない。起動前に取り消す場合はspawnを発行していないことを確認理由へ記す。

releaseの `--session MAIN` は予約に保存した親sessionと照合する。通常のAI操作ではBashの起動前hookが実呼出元sessionとCLI引数の一致も検査する。端末から直接実行した場合、この文字列は申告値であり本人認証ではない。元会話を利用できない場合は別sessionを装わず、Q2の人間確認による管理情報の復旧手順を使う。

保存前失敗は成功にせず、保存後の応答喪失は同一要求で照会・再試行する。異なる要求や遅れたPostで新しい予約を解放しない。lock残存は時間で横取りせず、当該CLI/hookの処理終了を確認する。起動したか・停止したか不明なら予約を保持してユーザーへ相談する。

## 初回導入、記録喪失、既存data

通常のreserve/checkはregistry欠落を未初期化エラーとして拒否し、空のregistryを自動作成しない。initは明示操作とし、既知のworkerと旧Unit割当を確認する。全runtimeが失われた場合、プログラムだけで未導入と情報喪失を完全に区別できるとは説明しない。

Q2の推奨案では、初回initおよび復元不能な管理情報の作り直しに人間の確認を求める。確認内容は「この管理元に属する既知の作業を停止・整理し、作業場所を再利用できる」。メインAIが確認回答と理由をCLIへ記録する。本人認証やOS権限による証明は追加しない。

元のregistryを復元できる場合は復元を優先する。再初期化は `assignment reset --file CONFIRMATION.json` で、元epoch/hashまたは欠落・破損の診断、確認回答、理由、request_idを明示し、新epochを発行する。読める旧情報は保管し、旧epochの変更要求・taskは拒否する。初期化自体は旧workerを停止しない。

旧Unit runtimeは自動移行しない。新しい横断予約がない既存の実行済みUnitへ、新方式のreassignで予約を後付けする操作も設けない。旧作業の停止と成果を確認した後、新定義の新しいIntentに必要な計画を登録して新規claimする。これにより、旧Unitがpendingでないためclaimできず、予約がないためreassignもできない状況の回避手順を明確にする。過去のIntent/state/Knowledgeを削除・自動書換えしない。旧Intentの継続には対応する旧版・定義が必要で、新しい横断予約の保証は適用しない。新旧方式を同じ配置で二重運用する機能は追加しない。

## 単独writerと変更ファイル

1 Issue/PR、`work_unit_id=native-agent-assignments`。実装担当1名が下表全体を所有し、親と他agentはloop中に対象ファイルを編集しない。計画・外部仕様調査・独立reviewは読み取り専用。必要なJSON型・小関数の内部配置は担当がこの境界内で調整できる。

| 対象 | 変更内容 |
| --- | --- |
| 新 `src/internal/assignment/{store,identity,reservation,dispatch,recovery}.go` と各test | 横断予約、task対応、epoch、保存・再試行・解放・復旧 |
| `src/internal/flow/{unit,reassign,review,store}.go`、新 `assignment.go` と関連test | 現在step/担当、既存Unitとの共通予約、部分保存、lock順序。一般review機能の変更は必要な共有lock入口だけ |
| `src/internal/minimal/{hook,session,command,flow}.go`、新 `assignment.go`、`agent_hook.go` とtest | CLI接続、native tool解析・担当拒否・Post/再依頼、停止時の保持 |
| `src/internal/cli/{minimal,help}.go` と関連test | assignment操作、Unit入力拡張、helpと具体例 |
| `src/internal/install/{install,relocate}.go` と関連test | 新しいmatcherを含む配布と、既知の新旧配置の移転検証 |
| `src/core/workflow/stages/*.md`、`src/internal/workflow/definition_test.go` | 各stageのstage-planner登録、担当一覧の単一正本の回帰 |
| `src/harness/codex/minimal/SKILL.md`、`aidlc-cli/SKILL.md`、`agents/aidlc-worker.toml` | メインAIによる直接起動、登録→起動→確認→解放、help参照 |
| 新 `src/cmd/aidlc/assignment_integration_test.go`、`assignment_live_integration_test.go` | 公開CLIと配布・固定Codexの実証 |
| `docs/development.md`、本計画、関連RAM/索引 | 利用条件、検証証拠、復旧と版の境界 |

既存filestoreを再利用し、外部module・汎用scheduler・ファイル編集範囲の網羅的な制限を追加しない。

## 順序付きTDDと検証分担

下記test名は追加予定。loop担当が各項目で意図したrunnable REDを確認し、最小GREENとrefactorを進める。項目ごとに親へ返さず、work unit末尾または契約上の停止時に返す。

新packageのrunnable REDへ到達するため、計画のJSONに対応するStore/Registry/Reservation/Dispatchおよび各操作Request型、Read/Init/Reserve/Check/Release/Reset/PreSpawn/PostSpawn/CheckTarget相当の宣言と、未実装error・ゼロ値だけを返すcompile-only scaffoldを許可する。内部の型・method名と引数分割は既存依存方向に合わせられるが、scaffoldに保存・検査・予約処理を先行実装しない。公開CLI・保存契約は本計画を維持する。

| 順 | 受入条件 | loopのtargeted command |
| --- | --- | --- |
| 1 | schema/明示init/欠落・破損・未知版拒否、root別名、保存境界 | `go test -count=1 ./src/internal/assignment -run '^TestRegistry'` |
| 2 | 別process・別Space/Intent/sessionでも同じrootは1件。別rootは双方成功、同一retry・ID再使用・解放後retry | `go test -count=1 ./src/internal/assignment -run '^TestReservation'` |
| 3 | Unitあり/なし、依存・scope維持、reported後保持、部分保存の同一復旧 | `go test -count=1 ./src/internal/flow ./src/internal/assignment -run '^TestUnitAssignment'` |
| 4 | 許可/不許可担当、Rule/step変更、read-onlyとworkerの承認境界、二重担当表なし | `go test -count=1 ./src/internal/flow ./src/internal/minimal -run '^TestAssignmentStage'` |
| 5 | taskの構造化対応、spawn retry/別tool ID、Post欠落/遅延、追加依頼、解放後・step変更後の拒否、interruptで保持 | `go test -count=1 ./src/internal/assignment ./src/internal/minimal -run '^TestAssignmentDispatch'` |
| 6 | Q1/Q2の確認、保存失敗・応答喪失・容量上限、旧epoch拒否、復旧を起動成功と扱わない | `go test -count=1 ./src/internal/assignment ./src/internal/minimal -run '^TestAssignmentRecovery'` |
| 7 | CLI/help、Unit入力、install/relocateの新旧判定、既存承認/Rule/編集hookの回帰 | `go test -count=1 ./src/internal/cli ./src/internal/minimal ./src/internal/install -run '^TestAssignmentContract'` |

独立reviewは、承認計画との一致、root競合、lock順序、Unit部分保存、taskの取り違え、停止の誤認、配布への影響を確認する。reviewで行う診断はfindingに必要なtargetedだけとする。

差分とblocking修正が安定したら、親が一度だけread-only finalを開始する。`go test -shuffle=on ./...`、`go test -race -shuffle=on ./...`、`go vet ./...`、`go mod tidy -diff`、`gofmt -l src`、`git diff --check`、`go test -tags=integration -count=1 ./...`、darwin/linux/windows×amd64/arm64の6build、native help、fresh install/relocate、Skill/TOML検証を集約する。変更が入れば古いfinal証拠を流用しない。

固定実機は既存Codex CLI 0.153.4、macOS arm64、gpt-6-astra/xhigh、一時プロジェクト・worktreeで、メインAIによる標準spawnを使う。許可と拒否、別rootで実並列、登録済み相対task_nameによるfollowup、reported後の競合、明示解放と新予約、Post欠落・保存失敗時の診断を確認する。canonical targetも製品で案内する場合は実機で対照確認する。新しいtool導入や利用者の実hooks書換えはしない。

hook/CLI自体の故障で子が開始したG0の観測を隠さない。正規経路で必要なtask照合が成立しない場合は、実験結果を残して契約を再検討し、専用起動CLIや推測へ切り替えない。固定CLIでの成功を全Desktop版・全ハーネスの保証にしない。

## 配布、信頼、既存配置、ロールバック

fresh installは既存Bash/apply_patchにnative起動・追加依頼・停止要求のPre/Post matcherを追加する。既存SessionStart/UserPromptSubmit/Stopを保持し、SubagentStopの新規登録で解放しない。新matcherは厳密な名前の集合として生成する。

installは既設の未知編集を自動上書きしない。relocateでは既知の旧matcherを旧構成として移転し、新matcherは新構成として移転する。旧matcherを移転しただけで担当制限が有効になったとは表示しない。汎用upgradeは追加しない。

既設へ導入する際は、子と既知の処理の終了確認、既存binary/hooks/skills/定義の保管、別の一時配置で生成した資材との比較、必要な製品資材だけの明示置換、通常のhook信頼確認、許可/拒否対照の確認を手順書にする。利用者のRule・Knowledgeを置換しない。実際の利用先への適用はこのリポジトリの実装とは別の操作として扱う。

stageの担当追加でdefinition hashが変わる。既存Intentを自動で新定義へ結び直さず、対応する定義で継続するか新定義で新Intentを作る。旧/新定義を取り違えたIntentを拒否する回帰を含める。

ロールバックは作業を確認してからbinary/hooks/skills/定義を対応する組で戻す。registryと確認根拠は保管し、削除して空きにしない。旧版は横断予約を知らないため、新しい保護を提供しない。旧定義で新Intentが再開できるとは保証せず、利用者の進捗・Knowledgeは巻き戻さない。

## 本家との差分と未確認範囲

比較対象は固定AI-DLC 2.6.123の[調査済み範囲](../ram/decisions/2026-09-10-upstream-agent-dispatch-and-worktree-reference.md)。本家ではメインAIが担当を起動し、SwarmがUnitごとのworktreeを用意する。コード生成担当の計画承認guardと、Team向けUnit claimがある。

採用する追加は、全stageの担当照合、通常運用でも全Intentを横断する割当、Unitなし割当、停止不明の場所の保持である。担当違いと作業場所の衝突を防ぐため、開始前の登録と解放確認が加わる。CLIが起動を代行しない点と別worktreeで並列化する点は維持する。最新upstream全体や全ハーネスの一致は未確認である。

## 承認済みの運用確認

**Q1・採用済み:** 通常の解放はメインAIが「追加依頼を終える・既知の処理が終了した・成果と残件を回収した」を確認し、CLIへ理由付きで記録する。不明なら保持して人間へ確認する。毎回人間の確認を必須にする代案は採用しない。

**Q2・採用済み:** 初回の管理開始と、復元できない管理情報の作り直しは、人間が既知の作業を整理したと確認してから行う。復元できる記録は復元し、再初期化では新epochを発行して旧要求を拒否する。メインAIだけの確認で再初期化する代案は採用しない。

A案そのものの承認は取り直さない。作業場所を再利用する責任と復旧条件の2点も具体計画とともに承認され、回答をRAMと本計画へ反映した。親がIssue作成、単独Go TDD実装、独立review、final、PR/checks/mergeを管理する。
