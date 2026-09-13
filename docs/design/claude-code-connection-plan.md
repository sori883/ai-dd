# Codex・Claude Codeのアダプターと新規配置の実装計画

2026-09-13追記：[ユーザー指示により未完了のまま保留](../ram/decisions/2026-09-13-claude-adapter-paused-incomplete.md)。実装と証拠を保持し、追加実機試験・review・final・PR・mergeは再開指示を待つ。以下の受入条件を満たしたことにはしない。

2026-09-12。D2、[Issue #185](https://github.com/sori883/ai-dd/issues/185)。共通配布基盤D1（Issue #183／PR #184）はmainへ反映済み。Claude対応の直接実装依頼、質問画面による承認の採用、新規配置への限定、環境別アダプターという指定を本計画へまとめる。

## 背景と利用者が得る結果

現在のGo製品にはCodex用の配置機能がある。Spaceは作業と知識の置場、Intentは一つの目的を持つ作業単位であり、工程、OKF文書、Sensor、独立review、人間の承認を共通処理で管理する。Claude Code用の配布資材とhook接続はまだない。hookはAIの操作前後に呼ばれ、工程の担当資格や承認待ちを検査する接続である。

同じGo実行ファイルで `aidlc install codex --project-dir <project>` または `aidlc install claude --project-dir <project>` を実行し、選んだ環境のskill、5種類の専門担当、hookと共通資材を配置できるようにする。子担当はメインAIが環境標準のtoolで起動し、CLIは工程・承認・担当の記録を管理する。CLIからAIを起動しない。

新規配置とは、AI-DLCが配置するfileがまだないことを意味する。既存アプリのソースがあるフォルダも利用できる。既存設定や文書は上書きせず、配置先の競合を保存前に拒否する。未リリースのため、旧AI-DLC配置の更新、移行、環境切替・併用、旧runtimeの互換読込みは追加しない。

## 実装許可と境界

直接承認は[製品対応の実装依頼](../ram/decisions/2026-09-12-three-host-product-implementation-approved.md)、[質問回答による承認の採用](../ram/decisions/2026-09-12-claude-question-approval-accepted.md)、[新規配置とアダプターの指定](../ram/decisions/2026-09-12-fresh-host-adapters-only.md)。初回導入範囲は回答済みで追加確認を待たない。[Copilotは当面除外](../ram/decisions/2026-09-12-copilot-support-deferred.md)する。

Go単一バイナリ、標準ライブラリ優先、外部Go module追加なし、Git不要、通常フォルダでの順次実装、既存6種類の工程を維持する。Knowledgeは現行の「何を・どうする」、ADRは設計変更の「なぜ」、rule.mdは利用プロジェクトの共通ルールである。同じ管理rootのworker占有を共有し、結果提出・tool終了・時間経過だけで予約を解放しない。

本家固定AI-DLC 2.6.123のmanifestと環境別生成処理を再利用する。固定snapshotを最新upstreamと同一視せず、旧33 Stageや全操作auditを取り込まない。通常hook対象経路での操作検査であり、OSの全書込み封鎖や回答者の本人認証を保証しない。

## アダプターと共通処理

アダプターは、AI環境固有の入力を共通処理へ渡す接続部である。共通ドメインの判定へCodex／Claudeという条件分岐を増やさない。

```text
cmd/aidlc ──→ harness/codex または harness/claude
    │                         │
    └──→ app ←── 共通の接続契約 ──┘
             ├── flow / assignment / filestore
             └── internal/host（環境を特定しない型・interface）

install ──→ harness.Manifest / Asset（生成済み配置資材）
```

| 所有先 | 責任 |
| --- | --- |
| `src/harness/{codex,claude}/` | hook JSONの読取り・出力、tool名、native会話・子ID、質問画面、設定path、skill・agent書式、固有runtime |
| `src/internal/host/` | 環境を特定しないイベント・判定・承認対象と証拠の契約。標準ライブラリだけに依存 |
| `src/internal/app/`、`flow/` | 工程、Rules、承認待ち、現在対象と証拠の照合、state・履歴保存 |
| `src/internal/assignment/` | 担当資格、起動要求と子の対応、worker占有、排他と復旧。native JSONを解釈しない |
| `src/internal/install/` | 渡された資材の全件事前検査と非上書き配置。具体アダプターを選ばない |
| `src/cmd/aidlc/` | 配置先の選択情報を読み、具体アダプターと共通処理を接続 |

`src/core/` の工程・OKF・Rules本文を変更しない。共通側に `Claude`、`AskUserQuestion`、`.claude` の分岐を置かない。既存appのCodex固有JSON・native tool分類・具体資材importはCodexアダプターへ分離する。必要な共通fileの変更は汎用interfaceの接続に限定する。

### 接続契約

新規 `internal/host` に次の役割を持つ型・interfaceを置く。

- `Event`：`SessionID, TurnID, ToolID, AgentID, Role, Action, Command, Paths, ParentTarget` 等の意味上の入力。native raw JSONは渡さない。
- `Decision`：許可／拒否、理由、追加context。実際のhook JSONはアダプターが生成する。
- `Runtime.Handle(Event) (Decision, error)`：会話開始、入力回、tool開始・終了、子の開始・対応確認等を共通判定へ渡す。
- `Runtime.WithApprovalTarget(ApprovalRef, func(ApprovalTarget) error) error`：session→flowのlock順で現在Intentと承認対象を照合し、callback中にアダプターが質問を保存する。
- `EvidenceReader.Read(ApprovalRef, AnswerRef) (ApprovalEvidence, error)`：flowの判断登録から呼ぶ読取り専用の証拠取得。追加lockを取らず、判断中の対象照合を迂回しない。
- `Deployment`：初期化Sensorの必須配置path、bootstrap path、既知の手順Markdown判定を返す。共通app/flowに環境別pathを記述しない。

ApprovalRefはSpace・Intent・計画/成果の区別・承認ID、ApprovalTargetは対象hash・工程ID・定義hash・計画revision/hashを含む。AnswerRefはsession・回答キー・引用・decision。ApprovalEvidenceは同じ承認対象と `session, answer_key, quote, decision, source_hash` を返す。補助型名はこの役割と公開CLI・保存契約を変えない範囲で調整できる。

Codexの通常会話承認はCodexアダプターから共通の入力証拠保存を呼ぶ。Claudeの通常Submitは証拠を保存せず、質問に限定したreaderを注入する。flow.Store生成をappのfactoryへ集約し、通常CLIとhookが同じreaderとDeploymentを使う。

## 配置・会話・保存

初回配置で `aidlc/.runtime/adapter.json` を他の資材と一緒に作る。

```json
{"schema_version":1,"adapter":"claude"}
```

値はcodex/claudeだけ。通常CLIと `__hook --project-dir <root>` はcmdの入口でこのfileを読み、同じアダプターを使う。tool名・環境変数・session名から推測しない。不在・未知値・不正JSONは失敗とし、新規配置が必要と案内する。help/versionと初回installは未配置でも使える。全配置先の事前検査にdescriptorも含める。

各アダプターは `SHA256(固定namespace + NUL + native_session_id)` の64桁hexをsessionキーとして共通処理へ渡す。CLIの `--session` にはbootstrapで示したキーを使う。既存の `aidlc/.runtime/flow/sessions/<session-key>.txt` の6項目（Space、Intent、Turn、Tool、RuleTurn、RuleHash）を再利用し、共通sessionに環境名・native IDを加えない。lockとdraftも同じキー。旧session変換は行わない。

Claudeは `.claude/settings.json`、`.claude/skills/`、`.claude/agents/`、Codexは既存 `.codex/` と `.agents/skills/` へ配置する。共通工程とOKF本文は共通sourceから投影し、環境選択だけで工程hashを変えない。共通skill本文のpath等はmanifestで変換して再利用し、Claude固有の起動・質問手順をClaude資材へ置く。

既存Codexの `--relocate` は削除せず、既存の対象4fileをアダプターが生成して共通installerへ渡す。Claudeへの移行・切替・既設更新として転用しない。

## Claudeの質問と承認

メインAIが対象と変更内容を説明し、標準 `AskUserQuestion` で `Approve`（承認）と `Request Changes`（修正依頼）を提示する。利用者は質問へ回答し、AIが既存 `intent plan-approval` または `intent approval` に記録する。

承認質問は一回一件・単一選択・上記2値に固定する。質問先頭は `[AI-DLC approval:<plan|result>:<request_id>]` とし、以降に日本語の対象説明を書く。headerは `AI-DLC`。IDはCLIの現在値を使い、利用者に入力させない。印のない通常質問は承認記録を作らず、印を持つ不正形式は拒否する。子の質問をメインの承認に使わない。

1. Preで会話・承認ID・対象版の一致と回答の事前入力がないことを検査し、記録保存後に質問を許可する。
2. Postで会話・prompt・tool IDとquestions部分の構造を照合する。実機ではPostのinputにanswers/annotationsが追加されるためraw input全体の一致は要求しない。
3. answersは該当質問文をキーとする一件だけ。Approveはapprove、Request Changesはrejectへ固定する。自由文・無回答・取消・timeoutは承認証拠にしない。画面内で選択値を手入力した場合は実機上区別できず、同じ完全一致値として扱う。
4. 回答を保存してから対応する質問待機だけを解除する。通常Submitや自動通知で回答を上書きしない。
5. CLIは承認ID・対象・版・session・回答キー・引用・decisionを照合し、既存Sensor/review/state/history検査へ合流する。引用は選択値と完全一致し、判断反転・別対象への転用を拒否する。

既存承認JSONのturnにはsessionキーとtool IDからSHA-256で導出した質問固有の回答キーを指定する。prompt IDとは分ける。Postの追加contextとquestion showでAIに提示する。質問が使えない起動方式で通常Submitへ切り替えて承認しない。

### 質問記録と復旧

保存先はアダプター所有の `aidlc/.runtime/adapters/claude/questions/<session-key>.json`、schema_versionは1。一つの未解決質問と既出tool IDを保存し、全操作auditにはしない。

fieldは `native_session_id, prompt_id, tool_use_id, space, intent_id, kind, request_id, target, step_id, definition_hash, plan_revision, plan_hash, questions, questions_hash, status`。回答後に `selection, decision, answer_key, answer_hash` を加える。statusはprepared/answered/no_answer/abandoned。既出ID上限1,024件、file上限1 MiB。上限時は新質問を拒否して新しい会話での接続を案内し、古いIDを時間経過で削除しない。

同一Pre/Post再送は冪等、不一致再送・別ID・古い対象・失効IDは拒否。Post保存失敗はpreparedを保持し、同じPostを再試行できる。state/history保存前に回答を消費・削除しない。回答の別承認ID・版への転用を認めない。

回答済みでCLIへの判断登録が終わっていない間は、新しい質問で証拠を上書きしない。未登録の回答を保持し、新質問を拒否する。登録成功の確認後に次の質問を準備できる。

登録成功はpending一覧からの消失では判定しない。計画の差替えでも消失するためである。既存の確定history recordへ任意の `approval_decision`（plan/resultのkindと、判断後の既存Approvalのコピー）を加える。承認判断のcommitだけが付与し、特にplan rejectでDraftを消す前にコピーする。State snapshotと同じhistory recordを既存commit順序で確定し、別のreceipt台帳や新工程stateは作らない。

共通の登録照会は現在stateのHistoryHeadから到達するchainだけを対象とし、承認ID、kind、対象hash、step・版、session、回答キー、quote、判断、source hashの完全一致を求める。host.ApprovalRef.Previousには前の回答証拠全体を渡し、lock内のcallbackでも記録が変わっていないことを確認する。history保存後にstate保存が失敗した孤児recordは登録済みと認めない。確定後にCLI出力が失敗してもchainから確認できる。未登録のまま対象が置換された回答は保持し、新質問で消さない。

Esc取消では終了hookが届かない実測があるため、明示復旧を作る。

```text
aidlc question show --project-dir <root> --session <key>
aidlc question abandon --project-dir <root> --session <key> --intent <id> --request <request_id> --tool <tool_use_id>
```

選択アダプターへ委譲し、prepared/no_answerだけを失効させる。answeredは拒否。同じ失効の再送は冪等。Postと失効は同じsession→flow lockで直列化し、先に回答を保存すれば失効拒否、先に失効すれば後着回答拒否。失効保存成功後に新tool IDの質問を許可する。

承認stateはpendingを保ち、別tool、Rules証拠、worker予約を変更しない。汎用session bind --recoverへ丸投げしない。元のUI・プロセス停止を記録せず、残った旧質問の回答は使わない旨を示して新質問へ回答を案内する。

承認対象が途中で置き換わった未回答質問も失効できる。現在選択したsession・Intentと保存済みのrequest/tool IDを厳密に照合し、失効には現在の承認対象との同一性を要求しない。これを要求すると旧質問を片付けられない。質問準備・回答保存・承認登録では引き続き現在対象との一致を必須にする。

### 本家との確認済み差分

| 本家固定2.6.123 | 採用する挙動 | 理由と影響 |
| --- | --- | --- |
| 標準質問のApprove／Request Changes | 同じ画面・選択値と日本語の説明 | CLI構文の入力が不要 |
| 通常Submitと質問Postの両方から入力記録。通常Submitの自動通知を構造的に除外しない | 現在対象に結び付いた質問PostだけをClaude承認根拠にする | 自動通知を入口に入れない。チャットの「はい」だけでは承認が完了しない |
| 汎用の入力順序記録も使用 | 既存製品の承認ID・対象版と一つの質問を照合 | 全操作auditを導入せず回答の流用を拒否 |

[本家調査](../ram/research/2026-09-12-upstream-claude-approval-and-revised-plan.md)の確認範囲に限る。質問入口限定は採用済みで再承認を求めない。未確認の本家経路を差分なしと断定しない。

## 担当起動と共通予約

Agent Preの構造化されたnameを既存task_name、subagent_typeを役割に使う。自然文promptから担当rootやIDを推測しない。固定Claude 2.1.238でnameがPre/Postに残ることは観測済みだが、Startはnameも親tool IDも返さない。nameを再開用のnative aliasとは扱わない。

共通assignment registryはschema 3とする。dispatchはsession・turn・親tool・task_name・役割・割当ID・opaqueな子キー・親宛先・Start/Post観測有無を保存する。Codex canonical pathやClaude responseのparserは各アダプターへ置く。旧schemaの移行読込みは追加しない。

以下のStart/native子ID結合と未結合一件制限はClaude接続に適用する。Codex固定版はspawnの応答にcanonical task pathを返すが子UUIDを返さないため、既存の役割・正式な親宛先の検査を維持する。共通registryには解釈しない宛先値を保存し、canonical parserはCodex側へ置く。Codexに未観測のUUID認証やClaudeの未結合制限を追加しない。[既存契約](../ram/decisions/2026-09-11-fixed-codex-hook-reliability-contract.md)と[実測証拠](../ram/decisions/2026-09-11-hook-reliability-core-evidence.md)を根拠とする。

1. 起動Preで現在工程の許可担当とworker予約を確認し、Claudeではassignment lock内で同じsession・turn・役割の未結合起動を一件に制限する。全サブエージェント人数制限ではない。
2. Startで一致する候補が正確に一件なら子キーへ結ぶ。0件・複数・ID衝突は拒否側へ倒す。Start自体で起動を止める方式ではなく、未結合子のPreを拒否する。
3. 子の最初のPreで結合・現在Intent/工程/役割/予約を照合する。同期Agentでは親Postが子終了後なので、親Postを初回操作の必須条件にしない。
4. 親Postは親tool IDからdispatchを引き、response.agentId由来の子キーを照合する。一致なら確認済み、不一致ならuncertainとして以後を拒否し予約保持。Post先行時は結合し、後着Startの同一性を確認する。
5. SendMessage.toはnative IDから同じ子を解決して現在工程・役割・予約を再検査する。子の親宛報告と兄弟宛操作を区別する。

同一通知は冪等。未知の子、同じ子IDの別dispatch結合、古い工程の追加依頼を拒否する。未結合起動を明示回収したsession・turn・役割の組合せは同じturnで再利用しない。遅延した旧Startの誤結合を防ぎ、新しい入力回で再試行する。Stop、結果提出、時間経過だけでworker予約を解放しない。

Claudeの子からメインへの報告先は厳密に `SendMessage.to: "main"` とする。結合済みの子・親会話・現在Intent/工程/予約を確認し、兄弟や別会話への宛先を代用しない。同期の担当は通常の最終回答で結果を返せるため、中間メッセージを必須にしない。固定2.1.238で同期・backgroundの両方からmainへの実配送を観測した。backgroundでは親PostがStartより先に届き、通知でpromptが更新された後にStopが届く場合もある。[wire観測](../ram/research/2026-09-12-claude-parent-report-wire.md)は製品guardの実機成功とは区別する。

ClaudeのEdit/Write等はアダプターが書込み先を抽出し、共通appの正本保護・工程検査へ渡す。未知の書込みを黙って読取り扱いしない。Bashのcommand検査は共通処理を維持し、tool終端は一致した枠だけを解除する。

Preと識別できるnative入力の変換失敗・共通判定の保存失敗は、Claudeのdeny JSONへ変換する。adapterへ到達する前のdescriptor不正・入力読取り失敗等も含め、`__hook` の処理失敗は終了コード2とstderrで返す。通常CLIの終了コード1を変更しない。終了コード1だけではClaudeのPreが処理を継続するため、拒否条件を満たさない。Codex/Claudeの両方で阻止可能なhook eventの失敗を停止側へ接続する共通入口であり、Startで子起動を阻止できるという意味ではない。

5担当の役割を配布時に落とさない。必要なOKF読取りCLIを使うためBashを各担当へ提供し、researcherには一次資料のWebSearch/WebFetchも提供する。既知のWeb読取りはadapterで共通Readへ変換する。worker以外の製品正本の更新は共通child gateが拒否し、Bashの追加をOSレベルの読取り専用保証とは説明しない。MCPや追加権限を無条件には提供しない。

ClaudeのWrite/Editからも、自分のsessionのDraftを作成・編集できるようにする。承認待ちやbegin前にCLI用JSONや文書草稿を作るための既存Codexと同じ例外である。例外は正規化された自分のDraft一件で、別session、正本、別tool実行中を許可しない。共有文書やstateはCLIで保存する。

## 単独writerの所有範囲

1 Issue／PR、work_unit_idはclaude-connection-d2。作業rootは `/Users/const/sori883/ai-dd-naming`、元 `/Users/const/sori883/ai-dd` の未commit資材は保全する。実装担当稼働中は親も同treeを編集しない。

| file／package | 内容 |
| --- | --- |
| 新規 `src/internal/host/{host.go,host_test.go}` | 環境を特定しない契約とidentity補助 |
| `src/harness/codex/` のGo/test、新規adapter/hook | native変換の移管、配置・手順・承認の接続 |
| 新規 `src/harness/claude/` | assets.go, manifest.go, emit.go, adapter.go, hook.go, question.go, question_store.goとtest、入口skill、CLI/OKF案内、5担当、question-rendering |
| `src/internal/install/{install.go,relocate.go}` とtest | 具体環境依存除去、資材とdescriptorの非上書き保存 |
| `src/cmd/aidlc/command.go`、`src/internal/cli/{command.go,help.go}` とtest | 接続選択、install claude、question show/abandon、help |
| `src/internal/app/{command.go,session.go,hook.go,agent_hook.go,child_hook.go,flow.go,assignment.go}` と補助file/test | native処理移管、共通port、Store factory |
| `src/internal/flow/{store.go,procedure.go,approval.go}` とtest | 初期化資材と証拠の注入、lock内承認対象照合 |
| `src/internal/flow/{history.go,execution_reopen.go}` とtest | 既存承認判断の履歴metadataと確定chainによる登録照会。別receiptを作らない |
| `src/internal/assignment/{store.go,dispatch.go,reservation.go,recovery.go}` とtest | schema 3、opaque子結合、未結合排他と回収 |
| 影響する `src/cmd/aidlc/*test.go`、`src/cmd/aidlc-dist/*test.go` | 実CLI・新規配置・Codex回帰fixture |
| README.md、src/docs/user-guide.md、docs/architecture.md、docs/distribution.md | 導入、通常trust、復旧、対応条件 |
| 本計画・RAM・索引 | 親がwriterの前後に更新 |

補助Go fileは表のpackage内に限る。src/core、外部module、既存ユーザー配置の変更許可ではない。testをcompileするための新型・signature・空返値だけのscaffoldは許可し、compile errorをREDと扱わない。

## 順序付きTDDと受入条件

verification_mode=loop。各項目でrunnableなtestを先に追加し、RED→最小GREEN→refactorを進める。既存動作はALREADY_GREENを正直に記録し人工REDを作らない。

| 順 | 観測する動作 | exact targeted command |
| --- | --- | --- |
| 1 | Codex hook・会話承認・手順読取りをport経由へ移して既存許可/拒否を維持 | `go test -count=1 ./src/harness/codex ./src/internal/app -run '^Test(CodexAdapter\|HostPort)'` |
| 2 | 選択descriptor・opaque session、不正/未知/不在、配置衝突・保存失敗 | `go test -count=1 ./src/internal/host ./src/internal/install ./src/internal/cli ./src/cmd/aidlc -run '^Test(AdapterSelection\|AdapterInstall\|AdapterSession\|QuestionCommand)'` |
| 3 | Claude資材・hook JSON・初期化Sensor・共通工程hash・編集拒否 | `go test -count=1 ./src/harness/claude ./src/internal/app ./src/internal/flow -run '^Test(ClaudeDistribution\|ClaudeHook\|AdapterInitialization)'` |
| 4 | 質問と回答、通常通知非承認、反転/別ID/古い対象/再送/保存失敗 | `go test -count=1 ./src/harness/claude ./src/internal/flow -run '^Test(ClaudeQuestion\|AdapterApproval)'` |
| 5 | 未回答失効・後着回答・競合、別tool/Rules/予約維持 | `go test -count=1 ./src/harness/claude ./src/internal/app -run '^Test(ClaudeAbandon\|QuestionRecovery)'` |
| 6 | 子結合、Start/Post順序差・重複、一意未結合、回収済み再利用拒否 | `go test -count=1 ./src/internal/assignment ./src/harness/claude -run '^Test(AdapterDispatch\|ClaudeDispatch)'` |
| 7 | 子初回操作・追加依頼、古い工程・別子、同root競合、終了不明・保存失敗 | `go test -count=1 ./src/internal/app ./src/internal/assignment ./src/harness/claude -run '^Test(AdapterChild\|AdapterReservation\|ClaudeChild)'` |
| 8 | 実CLIの配置→工程/OKF/review/質問承認→完了fixture | `go test -count=1 ./src/cmd/aidlc -run '^TestAdapterJourneyProtocol$'` |

各sliceのfile対応を次に固定する。新規testは表の名前で追加し、移管で影響する既存testは同じpackage内で合わせて調整する。実装file欄の補助fileはこの責任を分割する場合だけ追加でき、公開挙動や所有packageを広げない。

| 順 | test file（src/からの相対） | 実装file（src/からの相対） |
| --- | --- | --- |
| 1 | harness/codex/adapter_test.go、internal/app/host_port_test.go | internal/host/host.go、harness/codex/adapter.go・hook.go、internal/app/hook.go・agent_hook.go・child_hook.go・session.go・host.go |
| 2 | internal/host/host_test.go、internal/install/adapter_test.go、internal/cli/adapter_test.go、cmd/aidlc/adapter_test.go | harness/codex/manifest.go・emit.go、internal/install/install.go・relocate.go、internal/cli/command.go・help.go、cmd/aidlc/command.go、internal/app/command.go、internal/host/host.go |
| 3 | harness/claude/manifest_test.go・hook_test.go、internal/app/adapter_initialization_test.go、internal/flow/adapter_initialization_test.go | harness/claude/assets.go・manifest.go・emit.go・adapter.go・hook.goと配布資材、internal/app/host.go・flow.go、internal/flow/store.go・procedure.go |
| 4 | harness/claude/question_test.go・question_store_test.go、internal/flow/adapter_approval_test.go | harness/claude/question.go・question_store.go・hook.go、internal/host/host.go、internal/app/host.go・flow.go、internal/flow/approval.go・store.go |
| 5 | harness/claude/question_recovery_test.go、internal/app/question_recovery_test.go | harness/claude/question.go・question_store.go・adapter.go、internal/app/host.go・session.go、cmd/aidlc/command.go、internal/cli/command.go・help.go |
| 6 | internal/assignment/adapter_dispatch_test.go、harness/claude/dispatch_test.go | internal/assignment/store.go・dispatch.go・recovery.go、harness/claude/hook.go・adapter.go、harness/codex/hook.go、internal/app/agent_hook.go |
| 7 | internal/app/adapter_child_test.go、internal/assignment/adapter_reservation_test.go、harness/claude/child_test.go | internal/app/child_hook.go・agent_hook.go・assignment.go、internal/assignment/dispatch.go・reservation.go・recovery.go、harness/claude/hook.go・adapter.go |
| 8 | cmd/aidlc/adapter_journey_test.go、影響する既存journey/配布test | 先行sliceの接続修正に必要な計画対象file、導入・復旧文書。新しい挙動は先に該当sliceの回帰testへ戻る |

表の正規表現内の縦棒はMarkdownのescapeを除いたORとして実行する。no tests to run、skipは成功証拠にしない。影響する既存testは観測契約を維持して更新し、制約を弱めて通さない。末尾にtargeted群と変更packageのtest、変更Go fileのgofmt、git diff --checkをまとめる。loopでは全package/race/vet/cross build/配布E2Eを実行しない。

末尾確認で検出した既存fixture修復も同じwriterが所有する。具体対象は `internal/install/bootstrap_test.go`（削除したnative入口からadapter入口へ）、`harness/codex/manifest_test.go`（descriptorを含む配置集合）、`cmd/aidlc/hook_reliability_probe_test.go`（新規配置descriptor、native入力から導出するsession/lock path）、`internal/app/documents_test.go`・`verification_cli_test.go`（明示したPreイベントの変換）、`internal/flow/verification_cli_test.go`（schema 3）。この補正は承認済みの入口分離と新保存形式に追従するもので、製品の拒否条件を弱めない。compile failureや誤ったfixtureによる失敗は製品REDに含めない。

追加の補修も同じwork unitで、次の順序でtestを先に追加する。

1. C4: `flow/adapter_approval_test.go` と `claude/question_test.go` 等で、plan差替え後の未登録回答保持、plan/resultのapprove/rejectの確定照会、別回答拒否、history成功/state失敗の孤児拒否、確定後の次質問を検証。既存C4 commandで実行する。
2. C3: `claude/hook_test.go`、`cli/adapter_test.go`、`cmd/aidlc/adapter_test.go` で未知tool・保存失敗のdeny JSONと、`__hook` のdescriptor/入力失敗終了コード2、通常CLIの終了コード1維持を検証。`go test -count=1 ./src/harness/claude ./src/internal/cli ./src/cmd/aidlc -run '^Test(ClaudeHook|AdapterHook|AdapterSelection)'`。`cli/cli.go`・`command.go`等の実行入口もこの修復の所有範囲とする。
3. C3/C7: `claude/manifest_test.go`・`hook_test.go`、`app/adapter_child_test.go` で必要toolの配置・既知Web読取り・OKF CLI許可と製品正本更新拒否を対にして検証。既存C3/C7 commandで実行する。fixture-onlyの変更を人工REDにはしない。

実製品G0で判明した補修は同じ単独writerの所有範囲へ加える。順序は、(1) `internal/assignment/store.go` とhook経由のStore生成、対応testでStart/Postの同時取得を再現し、hook更新の取得だけ2秒上限・20ms間隔で待つ。最新registry再読、逆順イベント、期限超過の記録保持、保存失敗を再実行しない対照、通常CLIの即時失敗を確認する。(2) `harness/claude/SKILL.md` と `aidlc-cli/SKILL.md`、`internal/install/bootstrap_test.go` で長いbinary pathの新規配置とSessionStartを確認し、入口4 KiBを保ったまま詳細をCLI skillへ整理する。回帰根拠は[初回製品G0](../ram/research/2026-09-12-claude-product-g0-first-run.md)。承認済みの正常なイベント順序差と配布成立の修復であり、core変更、ロック自動削除、予約解放条件の変更は含めない。

親は全差分・TDD時系列・targeted群を確認する。実装/実測の故障で新しい重要選択が必要なら結果と代案を明記し、保証を黙って弱めない。

## 実機、独立review、final

Claudeの64文字制約への補修も同じwork unitに含める。[起動名の決定](../ram/decisions/2026-09-12-claude-worker-dispatch-name.md)に従い、`harness/claude/task_name.go` とtest、adapter.go、`internal/host/host.go` の名前投影port、Codexの恒等投影、`internal/app/assignment.go` の応答、対応CLI fixtureとClaude skill/worker案内を単独writerが所有する。順序は(1)予約名71文字と可逆base32名58文字の往復・不正拒否、(2)公開list/show/reserve等のdispatch_name追加と保存不変更・Codex同値、(3)worker Pre/Postの厳密な逆変換と別予約拒否、(4)公開CLIの起動名をnative入力へ渡すfixture。各項目でRED→GREENを記録し、その後新binaryの固定Claudeで実起動・再開・予約保持を確認する。外部module、alias台帳、core資材の変更はしない。

既存G0は固定Claude 2.1.238の質問Pre/Post、取消通知欠落、Agent nameと子IDの出現位置を観測したもの。製品の予約・拒否判定の成立までは証明していない。検証専用guardを二重実装せず、TDDで作った製品アダプターを隔離fixtureへ新規配置し、次の実コードG0をreview/final/merge前の必須gateにする。

- 通常trustでhookが読まれ、許可しない担当の起動をPreで拒否する。
- 同期AgentのPre→Start保存→子初回Pre→親Postが製品判定で成立する。Start保存と子Preの競合を未確認のまま成功にしない。
- 同役割連続起動、background、SendMessage再開を対応付け、無関係な子や古い工程の依頼を拒否する。
- 同root worker競合を拒否し、tool/子終了だけで予約を解放しない。
- 質問Approve/Request Changes、自由文・Esc取消、show/abandon・再質問、通常通知非承認を確認する。
- Codex 0.153.4でも変更した接続の回帰を実測する。

Claudeは `/Users/const/.local/share/claude/versions/2.1.238` を明示し、自動更新されたdefaultコマンドと区別する。実験は `/Users/const/sori883/ai-dd-validation/three-hosts-20260912/` 配下の専用projectと試験成果物のみ。通常確認画面を使い、hook trust/permissionを迂回しない。試験回答は製品開発の実承認と区別する。認証や利用枠で止まれば未確認を記録する。

追加実測で、通常TUIはrun_in_backgroundを省略しても非同期になることを確認した。現行の公式仕様は2.1.232以降のinteractive既定を説明しており、この実測と整合する。配布案内から同期を指定できるとする例を除き、標準Agentの実際のtool schemaに従って結果通知を待つ。`src/harness/claude/{SKILL.md,aidlc-cli/SKILL.md}` だけを同じwriterが修正し、配置と4 KiBの既存対照を確認する。core、設定、runtime判定は変えない。同期順序の決定論的testを実機成功へ読み替えず、上記同期G0は未確認として残す。公式にある試験プロセス限定の `CLAUDE_CODE_DISABLE_BACKGROUND_TASKS=1` は追加検証候補で、利用者の導入必須設定として採用しない。利用残高不足が解消するまでは実機gateとmergeを完了扱いしない。[実測記録](../ram/research/2026-09-12-claude-product-g0-first-run.md)。

独立担当へverification_mode=reviewで、環境非依存・計画適合・回答/子対応・排他・保存失敗・非上書きを依頼する。必要なtargeted testだけを実行する。blocking finding解消後、親が安定差分にread-only finalを一度開始する。

```text
go test -shuffle=on ./...
go test -race -shuffle=on ./...
go vet ./...
go mod tidy -diff
gofmt -l src
git diff --check
go test -tags=integration -count=1 ./src/internal/workspace ./src/internal/okf
go test -tags=integration -count=1 ./src/cmd/aidlc -run '^Test(FlowJourney|GitIndependentJourney|AdapterJourneyProtocol)$'
```

配置Markdownの参照、Claude agent frontmatter、両環境設定の構文・ロード、依存方向も検査する。Claude側の実装済みCLI fixture名は `TestAdapterJourneyProtocol` で、initializationからdiscoveryまでの合成回答を使う検査である。存在しない旧仮名のtestを実行済みとして数えず、D3の全工程実案件完走と区別する。6OS/archのcross buildと配布native検査は対象PRの既存CIへ委ね、全checks成功を確認する。

## 信頼確認とロールバック

installerは既存設定をmergeせず全配置先を事前検査する。hook commandは選んだbinaryとproject rootへ固定し、通常の信頼確認を経て読み込む。dangerously-skip-permissionsやCodex trust bypassを導入手順へ入れず、AIの全体設定を変更しない。

途中保存失敗はpartial一覧を返す。再試行でも残存fileの競合を検査し、上書きしない。復旧は試験専用の新しい配置先での再配置を基本とする。製品を戻す場合はPRのrevertと対応する旧binary/資材を新しい配置先で使い、schema 3・質問記録を旧binaryへ読ませない。既存Knowledge・ADR・state・履歴を自動削除しない。

親はIssue、単独writer、実機G0、独立review、final、PR/checks/mergeを管理する。実装済みと未確認の実機条件を分け、D1の完了だけをClaude対応の完了と報告しない。
