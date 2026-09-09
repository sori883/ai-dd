# ステージ別の担当起動とworkerの同一worktree排他 — 実装前計画

作成日: 2026-09-10。状態: **推奨運用方針とG0先行の実施を承認済み**。製品guardの技術契約には未確定事項が残るため、まずG0を実測する。

更新: ユーザーの「変更してください。承認。」を受け、Q1〜Q3の推奨方針とG0先行の実施を[承認記録](../ram/decisions/2026-09-10-stage-agent-worker-guard-approved.md)へ追記した。[具体的なG0作業単位](agent-guard-preflight-work-unit.md)から進め、実機で未確定の契約や専用scheduler等の代案を自動承認とは扱わない。

## 目的と利用者が得る結果

現在のAI-DLCには、各ステージで使う担当の定義と、Unit（分割した実装作業）の割当検査がある。しかし、メインAIがCodexの`spawn_agent`を直接呼ぶ場面では担当を制限しておらず、Unitの結果提出後も子workerが動いている可能性を扱えない。

この変更では、現在のステージに登録されていない担当の起動を起動前hookで拒否する。また、同じ調整rootが管理する全Space・Intent・sessionを通じて、同じworktreeで複数の`aidlc-worker`が同時に稼働することを防ぐ。調整rootとは、共有stateと担当割当を管理するプロジェクトの作業ディレクトリである。

例えばTDDでworker Aが`/project-wt/api`を使用中なら、別Intentのworker Bも同じ場所では開始できない。Bの場所が`/project-wt/ui`なら、Aと並列に実装できる。researcher等の読取り担当をworker人数に数えず、全サブエージェントの人数上限は変更しない。担当範囲外の全ファイル編集を網羅的に禁止する機能、メインAIと子workerの同時編集を一般的に封じる機能は対象に含めない。

これは通常のAI操作に対する制御であり、同一OS利用者やhook対象外の経路を含む完全な封じ込めではない。

## 調査基準と承認の境界

- 元checkout `/Users/const/sori883/ai-dd` は`codex/human-stage-approval`、HEAD `f0e97d4504ffdffa474abf31082626711f954d63`。未commitのRAM等を保持しており、変更しない。
- GitHub mainは`c990f7629c69dd0a3f11ad1994dd69c165e70749`。[PR #158](https://github.com/sori883/ai-dd/pull/158)までマージ済み、確認時のOpen Issue・PRは各0件。
- 計画のコード根拠はmainと同じHEADの`/Users/const/sori883/ai-dd-stage-okf-documents`。この計画とRAMだけを未commitで保存する。実装開始前にはmain・他タスクの変更を再確認する。
- 適用規則は[AGENTS.md](../../AGENTS.md)、`implementation-planning`、`golang-how-to`、[エージェント運用](../agent-workflow.md)、[TDD依頼契約](../tdd-handoff.md)。計画担当は読取り専用、将来の実装writerは常に1名とする。
- [既存の合意](../ram/decisions/2026-09-08-four-stage-runtime-approved.md)は、調整rootを一つ明示し、Go CLIは割当・成果・遷移を扱い、Codex自体を起動するschedulerは作らないとしている。別PCのリアルタイム排他を保証しない。
- 当初はこの新機能の計画作成依頼であり、その後の直接承認を受けG0から進める。旧33 Stage、4段階、製品agent追加、PR #158の承認を流用しない。Go単一バイナリ、標準ライブラリ優先、外部Go module・外部tool追加なしを維持する。

## 現在の実装で確認したこと

| 確認対象 | 現在の意味と不足 |
| --- | --- |
| `src/internal/install/install.go`、`relocate.go` | 製品のPre/PostToolUse matcherは`^(Bash\|apply_patch)$`。spawn用の登録ではない |
| `src/internal/minimal/session.go` | `HookInput`は主に`tool_input.command`を読む。spawnの要求・応答を扱う型はない |
| `src/internal/minimal/hook.go` | Rule読込み・承認待ち・tool実行中を検査。`Session.Tool`は一つのtool呼出しのPreからPostまでであり、子の寿命ではない |
| `src/internal/workflow/definition.go`、`src/internal/flow/graph.go` | 段階の`Procedure.Agents`と固定した定義の照合がある。ここを担当一覧の正本として使う |
| `src/internal/flow/unit.go` | claimは同じIntentの`running`/`needs_confirmation`についてroot・session等の重複を検査。resultは`reported`にするが、実子の停止を確認しない |
| `src/internal/flow/reassign.go` | 再割当の確認と保存途中の復旧がある。別Intentを横断する実子の排他ではない |
| `src/internal/filestore/file.go` | 指定rootの`aidlc/.runtime/locks/<key>`をmkdirで取得する非横取りlock。別processの同時要求にも使えるが、現在のsession lockだけではsession横断排他にならない |

現在の段階定義は次のとおり。これを別JSONやGoの担当一覧へ複製しない。

| ステージ | 登録済みの担当 |
| --- | --- |
| initialization | aidlc-reviewer |
| discovery | aidlc-requirements、aidlc-researcher、aidlc-stage-planner、aidlc-reviewer |
| architecture-analysis | aidlc-researcher、aidlc-reviewer |
| planning | aidlc-reviewer |
| tdd | aidlc-worker、aidlc-reviewer |
| integration | aidlc-reviewer |

**既存指示との不一致:** `src/harness/codex/minimal/SKILL.md`は途中の計画変更時にもstage-plannerを呼ぶが、現在の段階定義にはdiscoveryだけが登録している。厳密な起動制限を追加する前に、後述Q1を確定する。

## G0: 固定Codexで確認してから契約を決める

今回確認したのは手元の`codex --version`が`codex-cli 0.153.4`であることまで。起動・拒否・終了・再開の実機実験やテストは実行していない。

[公式hook仕様](https://learn.chatgpt.com/docs/hooks)では、`spawn_agent`はPreToolUse対象で`Agent`にもマッチする。拒否には`permissionDecision: deny`を使う。`SubagentStart`の`continue: false`では起動を止められない。`SubagentStop`には子ID等があるが、継続を要求する応答もあり、OSプロセスの消滅を保証するとの記載はない。一部のtool経路はhook対象外になり得る。これらは2026-09-10に読んだ公開仕様であり、固定0.153.4やDesktop上の全経路を実測した証拠ではない。

過去の[4担当の実機記録](../ram/decisions/2026-09-08-product-agent-roles-evidence.md)は0.153.4での名前解決、Start/Stop、返答の確認である。起動前拒否・worktree拘束・再開拒否・残存process停止の証拠として流用しない。

G0だけを実施する場合も、先に調査用fixtureの範囲を承認する。Goのfixture追加にはIssue・単独writer・loop・独立reviewを適用し、固定HEADのfinalで実験する。G0成功は製品実装の承認を意味しない。

| Gate | 実験と必要な証拠 | 通らない場合 |
| --- | --- | --- |
| G0-1 起動前拒否 | 実`spawn_agent`のPre入力、拒否応答、親toolの結果を保存。許可時だけ子が試験用印を残す対照実験も行い、拒否時は子の開始・副作用がないことを確認 | Start側の拒否へ代替しない。対象版変更または起動経路変更の案と影響を提示する |
| G0-2 一意な対応 | 担当・親session/turn/tool-call ID・子agent_id・実worktreeの関係を、要求/応答/Startの構造化情報と照合。別rootの同時起動、通知順序逆転も確認 | promptの文章、時刻の近さ、未確認の`cwd`をrootとして推測しない。構造化した割当と安全に結べなければ機能2の実装を止める |
| G0-3 追加依頼・再開 | 固定版にある追加依頼・再開・割込み・終了toolの名前と引数、Pre/Postの到着を採取。待機中の子へ新作業を送る経路も含む。`send_input`/`resume_agent`等は候補名であり確定schemaではない | 捕捉できない経路を許したまま排他を保証しない。対応外として拒否可能かを確認し、不可能なら起動方式等の変更を再提案 |
| G0-4 終了の意味 | 子の一turn終了、親の割込み、close、再開、他hookによる継続、Bashの非同期processを残した返答、遅延/欠落/重複Stopを比較する | Stopだけでは自動解放しない。停止確認を伴う明示復旧案を提示する |
| G0-5 調整root・Unit | Unit割当から子の実root/session/run IDを確定できるか、Unitなし予約、別調整rootの同一worktreeとpath別名を実測する | Q2/Q3と公開CLI/runtime契約を再確定。保証範囲を黙って縮めない |
| G0-6 故障時の拒否 | hookの保存エラー、stdout喪失、timeout、異常終了、他hookの併用を確認。保存できない時に明示denyを返せるケースと、Codexがhook故障後に続行するケースを分ける | hook故障時も完全に遮断すると説明しない。必須の通常経路を守れない場合は製品実装を止める |

実験は一時Gitプロジェクトと二つ以上の既存worktree、試験専用hooksを使う。対象はmacOS arm64・既存0.153.4、モデル/effortは依頼時の値を固定して記録する（候補gpt-6-astra/xhigh）。Codex更新・インストール・利用者の実hooksや設定変更はしない。承認操作が必要なfixtureでは試験用回答と明示する。raw event、process終了、ファイルhash、子の実cwdを独立に突き合わせ、モデルの「停止した」という文章だけを証拠にしない。

Desktopの`followup_task`等が利用対象なら別経路として追加実測する。CLIの実測をDesktopや全ハーネスへ一般化しない。実測したtool schema・版・OSと、対応できない経路を結果に明記する。

## 条件付きの実装案

### 1. 現在ステージの担当照合

PreToolUseで、要求元の実sessionから選択中Intentと現在stepを取得し、固定definitionを再読込みして`Procedure.Agents`へ実際の担当識別子を完全一致で照合する。未選択、担当不明、定義欠落・変更、対応が曖昧な要求は拒否理由を返す。名前やrootを自然言語promptから抽出しない。`SubagentStart`は照合の補助であり起動拒否には使わない。

既存のRule読込み、承認、独立review、Unit割当、開始Sensorの条件は維持する。読取り担当をworkerと同じ書込みgateへ一律に押し込まず、許可済みの調査・レビューを妨げない。workerは現在stepの有効な割当と既存の作業開始条件を満たす必要がある。具体的なhook接続順はG0のschemaに基づいて固定する。

再開・追加依頼は保存済みの実agent_idから担当と割当を再取得し、その時点のstageを再検査する。古いstageで起動できたことを、次のstageでの新たな依頼の許可にはしない。既に稼働する同じ子への追加依頼では同じ予約を保持し、停止後の再開では新しいgeneration（再実行を区別する番号）を取得する。中断要求・停止確認そのものは、新たな作業の許可がなくてもできるようにする。

### 2. worker予約の保存場所と内容

Q3の推奨案を選ぶ場合の保存先は、**調整rootの`aidlc/.runtime/agents/registry.json`**とする。Space別・session別に分割せず、同じ調整rootの全予約を一つの検査対象にする。短時間の保存lockには既存`filestore.Lock(root, "agent-runs")`を使う案とする。子が稼働する間lockディレクトリを占有し続ける方式にはしない。

record案はruntime schema version、registry revision、正規化したworker rootと実体識別情報、調整root、Space/Intent/step、definition hashと計画版、親session、要求ID、実agent_id、generation、任意のUnit IDと既存run ID、予約状態を持つ。日時は診断表示に使い、失効期限にはしない。進捗や品質の正本は既存stateに残す。全toolの履歴、prompt全文、全操作audit、成果物snapshotは追加しない。遅延イベントを取り違えないための最後のgeneration/解放済み対応は最小限保持する。

rootは絶対path化・symlink解決・既存Git worktreeの実体確認を行う。文字列の小文字化だけで同一性を判断せず、同じ実体の別名と別worktreeを区別する。未作成rootや差替わった実体へ予約を付けない。対応するUnitがある場合は割当root・run IDとの一致を確認する。構造化した要求からこの実rootを拘束できることがG0-2の前提である。

### 3. 原子的な検査と状態変更の競合

起動前は、最新state確認→同じrootの予約検査→予約保存→許可応答の順にする。二つの別processから同時に要求されても、同じrootを両方へ許可しない。lock競合は明示的な再試行結果とし、別rootは最終的に両方を予約して並列実装できる。

既存のflow更新は`flow-<space>` lockを使う。`review.go`の`changeReassignment`、`store.go`の`Save`、`boundary.go`の`Begin`、`reopen_log.go`の`reopen`が主要な入口である。新しい予約の検査もflow lock内で最新stateを読み、その後registry lockを取得する案とする。lock順は必要なものだけ **session → flow → registry** とし、逆取得や公開更新APIの再入呼出しを避ける。

予約後のfinish/reopen/再割当などで予約のstep・root・runを無効にしてから古い要求を実行させないことも必要である。状態変更側へ整合検査を追加する。停止要求・pause・結果回収まで一律に禁止しない。提案は、稼働/起動未確定の予約を無効にする変更は停止確認まで確定せず、計画案の読取りや作成、停止・復旧を引き続き許す方式である。個別操作の許可表はG0後の契約確定対象とする。

stateとregistryは別ファイルであり、単一transactionとは説明しない。revision・generationを比較し、片側保存失敗は保留して同じ要求から復旧する。また、hookの許可応答から実spawnまでの区間はGoのファイルlockで原子的にはできない。定義ファイルの外部編集もflow lockだけでは防げない。これらをG0と受入条件で確認し、満たせなければ開始手順の変更を再提案する。

### 4. 実行予約の寿命

| 観測した出来事 | 予約の扱い |
| --- | --- |
| Preで検査・保存できた | `reserved`。保存前に許可しない |
| spawn成功と実子IDの対応が取れた | `running`。spawn Postでは解放しない |
| 子が一切開始していないと確定した失敗 | 同じ要求/generationだけ解放可能 |
| 応答・Startが不明、保存失敗、通知欠落 | `uncertain`。占有を保持し、時間だけで解放しない |
| Unit result/reported/integrated、親Post/Stop、session recover | 子停止の証拠にはしない。予約を保持する |
| 子の一turn終了、SubagentStop、割込み要求 | 終了候補の観測。残存processと再開可能性を照合するまで保持する |
| 停止とprocess終端が確認され、新作業開始を再検査できる | 該当generationだけ`released`。具体的な必要証拠はG0-3/4で確定 |
| 既存子を再開・再利用する | 現在stageとrootを再検査し新generationを予約。他workerが使用中なら拒否 |
| 古いStop/失敗通知が遅れて届く | 新generationを解放しない。対応不能なら保留・診断 |

同一hookイベントの再配信は同じ要求ID/内容/generationで冪等に扱う。ただし「同じ要求を再受信した」ことと「Codexが実spawnをもう一度呼ぶ」ことは区別する。子が一度しか生成されない契約を確認できない再試行へallowを再発行しない。新しいtool-call IDでのやり直しは既存予約との照合が必要である。

Unitの`reported`などの進捗は変更せず、実子の予約との対応だけを追加する。Unit再割当でも旧workerの停止未確認枠を横取りしない。Unitなし作業はQ2の回答で決める。メインAI自身の編集と子workerを同じ人数枠へ含める変更は提案しない。

### 5. 復旧と互換性

registryの未知version・破損・予期しない欠落は空き枠と見なさない。初回導入時の空registry作成と、運用開始後の消失を区別する初期化手順を契約に含める。既存子が動いている環境へ空registryを置いて新規導入したことにしない。

読取り専用の実行一覧/診断と、要求ID・agent_id・generation・registry revisionを指定する明示復旧CLIを提案する。コマンド名とJSONはG0後に確定し、helpを正本にする。停止通知が十分でない場合は、旧run停止・残存process終端を実際に確認した利用者/調整役の記録を必要とする。確認せず`force`や経過時間だけで解放する操作は作らない。この運用確認はOSレベルの停止証明ではない。

Git共有するstate schemaを予約のためだけに変更しない案を優先する。Unit関連付けで永続schema変更が必要と判明したら、移行・拒否・復旧を明示して再承認する。runtimeはGit共有しない。root移転、旧版からの導入、registryの退避/削除は全関連agentの停止確認が前提であり、稼働記録を黙ってリセットしない。

## 未確定事項と代案

以下3点は推奨案で進めることを直接承認された。代案は比較の記録として保持し、実機で技術契約が成立するかは別途G0で確認する。

| 項目 | 推奨する案 | 代案と影響 |
| --- | --- | --- |
| Q1 計画変更時のstage-planner | 各段階の既存`agents`へ明示追加し、途中の計画見直しを維持する。第二の許可一覧は作らない | discoveryだけに限定する。その場合、途中の計画変更手順を見直し、戻るための計画作成との循環も解決する必要がある |
| Q2 Unitに分割しない子worker | UnitなしでもIntent/step/rootに結び付く実行予約を持つ。架空のUnitを作らない | 子worker使用時は1 Unit以上を必須にする。既存direct作業の利用条件が変わる |
| Q3 別の調整root | 従来どおり1つの調整rootへ集約。同じrootの全session/Intent/Spaceは排他。既知の所有者不一致は拒否する | 複数調整rootを保証対象にするなら、worker root内の共通記録またはhost全体registryが必要。共有場所・権限・移転・ロック範囲を再設計する |

Q3推奨案でも、別調整rootが私有registryを使って同じworktreeを開始する行為を、このregistryだけで検出できるとは説明しない。複数調整rootを許さない運用条件と、プログラムが検出できる条件を分ける。

G0で予定rootがspawn入力へ結び付かない場合の代案は、構造化した割当情報を受け取る専用起動経路である。ただしGo CLIによるCodex起動は既存の「schedulerを作らない」合意の変更になり得るため、黙って実装しない。開始要求を一つずつにする案も、遅延通知・retryを含めて一意な対応が証明できた場合だけ候補にする。別worktreeで動いているworkerまで直列化する案には置き換えない。

停止証拠、実機対象のCLI/Desktop、state変更の許可表、初期化/欠落識別、公開復旧CLIはG0後に確定する技術契約である。今回の3点への回答だけで、この未確定部分まで承認されたとは扱わない。

## 変更ファイルと単独writerの所有範囲

次はG0後の契約が成立した場合の具体的な予定。新規名は提案である。実装時は1 Issue/PR、`work_unit_id=stage-agent-worker-guard`にまとめ、以下を1名のGo実装担当が所有する。

| ファイル | 変更内容 |
| --- | --- |
| `src/internal/minimal/session.go`、新`agent_hook.go`、関連test | 観測済みのtool_input/tool_responseと子イベントを型付きで扱うadapter。Bash/apply_patch入力の互換性を維持 |
| `src/internal/minimal/hook.go`、新`agent_hook_test.go` | spawn/追加依頼/再開の検査、実子との対応、拒否出力。Session.Toolとは分離 |
| 新`src/internal/agentrun/store.go`、`identity.go`、`lifecycle.go`と各test | root同一性、registry、要求/generationの照合、原子的保存、故障時保留。既存filestoreを使用 |
| 新`src/internal/flow/agent_guard.go`とtest、`graph.go` | boundDefinition/Procedure.Agentsからの担当照合、最新stateと予約の接続 |
| `src/internal/flow/review.go`、`store.go`、`boundary.go`、`reopen_log.go`と関連test | state変更と予約の競合、停止・回収を妨げない整合検査 |
| `src/internal/flow/unit.go`、`reassign.go`と関連test | 既存Unit割当/runとの対応、reported時の予約保持、再割当の停止確認 |
| `src/internal/cli/minimal.go`、`help.go`、`src/internal/minimal/command.go`と関連test | 契約確定後の一覧・診断・復旧操作。型/値/JSONをhelpへ記載 |
| `src/internal/install/install.go`、`relocate.go`と関連test | 新hook matcher/eventの明示配置、事前検査、既知ファイルのみの移転、旧配置保全 |
| `src/core/workflow/stages/*.md`、`src/internal/workflow/definition.go`とtest | Q1承認時だけ担当登録を変更。取得APIの必要最小限の接続。第二の担当一覧は追加しない |
| `src/harness/codex/minimal/SKILL.md`、`aidlc-cli/SKILL.md`、`agents/aidlc-worker.toml` | 担当起動・停止確認・復旧の案内。4 KiB bootstrapと既存役割を維持 |
| 新`src/cmd/aidlc/agent_hook_probe_live_test.go`、`agent_guard_integration_test.go`、`agent_guard_live_integration_test.go` | 固定実機の入力証拠、製品接続と配布journey。G0 fixtureと製品受入を区別 |
| `docs/development.md`、本計画、関連RAM/索引 | 利用条件、復旧、実測の範囲を記録 |

`src/internal/filestore/file.go`自体は既存機能を再利用し、必要性が確定するまで一般的なロック再設計へ広げない。Go依存方向はflow/minimal→agentrun→filestoreとし、agentrunからflowへ逆依存させない。既存CLI/永続仕様の変更は確定契約とIssueへ含める。

G0の調査fixtureを先に作る場合は、未解決の承認gateが理由で別の小さな作業単位に分ける。単に項目数が多いことを理由にwriterを増やさない。今回の計画作成用read-only agentと、将来の製品内worker制限を混同しない。

## 順序付きTDDと検証分担

以下は将来追加するtest名と実行予定であり、現在のテスト実行結果ではない。

| 順 | REDで確認する契約 → GREENの受入条件 | loopのtargeted command |
| --- | --- | --- |
| 1 | 許可担当を通し、他stage/不明担当/未選択/定義変更を拒否。Q1の許可例も定義由来 | `go test -count=1 ./src/internal/flow ./src/internal/minimal -run '^TestAgentSpawnStagePolicy'` |
| 2 | 同じ実rootを指す別session/Intent/Space・別processの競合で最大1件だけ予約。別rootは双方許可。読取り担当の人数を制限しない | `go test -count=1 ./src/internal/agentrun -run '^TestWorkerReservationAcquire'` |
| 3 | 起動要求と子ID/rootを一意対応。同一イベント再配信は冪等、別の実spawnは重複拒否。起動失敗と成功不明を分ける | `go test -count=1 ./src/internal/agentrun ./src/internal/minimal -run '^TestWorkerReservationSpawn'` |
| 4 | spawn Post/reported/親Stopで解放しない。確定停止後だけ解放し、再開時に再予約、旧世代Stopで新予約を消さない | `go test -count=1 ./src/internal/agentrun ./src/internal/minimal -run '^TestWorkerReservationLifecycle'` |
| 5 | Unitあり/なし、reassign、root別名・差替え、Q3境界を検査。stage変更と予約の同時要求を安全に処理 | `go test -count=1 ./src/internal/flow ./src/internal/minimal -run '^TestWorkerReservationOwnership'` |
| 6 | 書込み前後・rename前後・応答前後の失敗注入、破損/欠落/未知version、lock残存で黙って許可しない。復旧は同一要求と世代に限定 | `go test -count=1 ./src/internal/agentrun ./src/internal/flow ./src/internal/minimal -run '^TestWorkerReservationFailure'` |
| 7 | 実測schemaのstrict解析、hook配布/移転、復旧help、旧配置保全、既存Rule/承認/編集tool gateの回帰 | `go test -count=1 ./src/internal/install ./src/internal/cli ./src/internal/minimal -run '^TestAgentSpawnHookContract'` |

- **loop:** 単独writerがtest-firstで各項目を進め、runnable RED→最小GREEN→refactorの証拠を残す。文書修正だけへ人工REDを作らない。失敗修復もtargetedのみ。親は作業単位末尾で差分とtargeted群を一度確認する。
- **review:** 別担当が固定base/headを読取り専用で確認。scope、担当一覧の二重化、別process競合、実spawnの二重発行、イベント世代、保存不整合、state変更との競合、終了根拠を重点確認する。必要なtargeted再現だけを行う。
- **final:** blocking修正後の固定HEADで親が1回開始。ソースを変更せず、`go test -shuffle=on ./...`、`go test -race -shuffle=on ./...`、`go vet ./...`、`go mod tidy -diff`、`gofmt -l src`、`git diff --check`、`go test -tags=integration -count=1 ./...`を実行。darwin/linux/windows×amd64/arm64の6buildとnative help、fresh install/relocate、skill/TOML検証を集約する。変更後の古い検証は流用しない。

製品finalの限定liveはG0で確定したCodex/経路で、許可・拒否の対照、同一rootの同時要求、別rootの実並列、reported後の二重要求、再開/遅延Stop、通知欠落・保存失敗の復旧を実測する。Bash/apply_patchだけのmock成功をspawn制限の証拠にしない。実機liveは通常CIから明示opt-inへ分離し、GitHub CIの未実行/skipを成功として数えない。

## 配布、信頼確認、既存配置、ロールバック

Pre/Postには既存Bash/apply_patchに加え、G0で確認した正確な起動・追加依頼・再開toolを登録する。`Agent`はmatcher別名であり、曖昧な正規表現で全toolをworkerとして数えない。Start/Stopは照合用に分離する。必須guardは同一の判定処理にまとめ、複数hookの並列実行順に依存しない。

新規配布と既設の更新は区別する。現行installは既設資材を自動上書きせず、relocateは既知のbinary参照移転であってupgradeではない。旧hooksをそのまま移転しただけで新機能が有効になったとは案内しない。既設への新guard導入は、関連子の停止確認→旧配置と版の保管→差分提示→明示した更新手順→利用者のhook再信頼確認→対照probe成功を条件とする。汎用upgradeを今回のscopeへ黙って追加しない。

Q1で段階のagentsを変更するとdefinition hashも変わる。既存Intentは対応する旧定義で継続するか、新定義で新Intentを開始する。保存済みIntentを新定義へ暗黙に再結合しない。新旧Intentを同じ配置で継続できる新機能も追加しない。実装時の配布回帰では、旧Intentと新定義の不一致を拒否することを確認する。

利用者環境で`--dangerously-bypass-hook-trust`を通常導入手順にしない。公式の信頼機構で変更済みhookを確認する。試験用のtrust設定は隔離プロジェクト内のものと明示する。無効/未信頼/未対応のhookではこの制御を利用できない。

ロールバックも先に子と残存processを停止・確認し、registryを証拠として保管する。旧binary・対応する旧hooks/skills/定義を組で戻し、再信頼する。稼働中にregistryだけ消したり、旧binaryへ切り替えて枠が空いたと扱ったりしない。旧版へ戻すと新しい担当制限・root排他は提供されないことを明示する。ユーザーRule・Knowledge・stateを巻き戻さない。

新定義で作ったIntentは、旧定義へ戻した配置ではhash不一致で停止する。stateを保持していても旧版で再開できるとは保証しない。対応する新定義を復元して続けるか、利用する定義で新Intentを作る。逆方向のhash不一致拒否も配布回帰へ含める。

## 本家との比較と実装開始条件

参照はローカル固定AI-DLC **2.6.123**。`docs/実装_aidlc-workflows/core/tools/aidlc-version.ts`と分析索引の版一致を確認した。最新upstreamとの同一性は主張しない。

- 確認した本家の範囲: `harness/codex/hooks/aidlc-codex-adapter.ts`のspawn変換とplan-approval guard、`dist/codex/.codex/hooks.json`の登録、`core/hooks/aidlc-log-subagent.ts`。spawn時のstage rule供給、特定developer担当の計画承認検査、SubagentStop時のin-flight処理と監査記録がある。
- 採用を提案する挙動: 最小製品の現在の`Procedure.Agents`による担当拒否と、正規化root・実子ID・generationによるruntime予約。監査全体を再導入せず、Stopだけを停止済みの根拠にしない。
- 理由と影響: 別worktreeでの並列実装を維持しながら、不適切な担当と同一作業場所のworker重複を防ぐ。保留予約のため明示復旧が必要になる場合がある。従来通った起動の一部は拒否される。
- 未確認: 本家の全scheduler/worker排他・全ハーネス・最新upstreamの挙動。確認したファイルだけから「本家に同等機能がない」「差分がない」と断定しない。G0後に比較範囲と採用差分を計画・承認RAMへ確定する。

現在はQ1〜Q3の推奨方針とG0先行を承認済みであり、具体的なG0作業単位のIssue・fixture・review・実測を進める。製品guardへ進むには、実測結果と起動/再開/停止/復旧/runtimeの契約を確定する必要がある。必要な保証を確認できない場合は未確定事項と代案を提示して止まる。今回の承認を、専用起動schedulerや未知の永続形式の採用へ拡張しない。
