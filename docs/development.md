# 開発と利用の手順

ソースからのbuildにはGo 1.26以上を使います。製品の利用にGitは不要です。
本リポジトリの履歴管理や一部のtest host fixtureはGitを使います。外部Go moduleは承認済みYAML parserだけです。

```sh
go build -o /tmp/aidlc ./src/cmd/aidlc
/tmp/aidlc install codex --project-dir /absolute/project
```

fresh projectで配置済み `aidlc` skillを使います。Intentを名前で作成し、各会話で同じIDを選択します。
`intent show` のrevisionを `--expect` に渡します。競合時は再読込して判断し、強制上書きしません。

```sh
aidlc intent create "加算機能" --space default
aidlc intent list --space default
aidlc intent switch --id <id> --space default --session <session>
aidlc intent configure <id> --space default --expect <revision> --file config.json
aidlc intent check <id> --space default
```

初期化、目的整理、および選択した各段階の境界で、調整役とは別sessionの独立read-only reviewを受けます。
同じrootを使えます。別rootの場合はIntentの検証対象集合SHAが一致する必要があります。
`intent review` のassignは担当session/rootを指定し、acceptはその担当の実報告と対象hashを受け取ります。
修正で対象が変わったら再reviewします。実際の会話回答を `intent approval` へ記録し、`intent finish` が現在回を完了します。
初期化→目的整理は必須です。構成分析・計画・TDD・統合検証の採否と順序は `intent plan` で提示し、
`intent plan-approval` で会話承認を記録します。計画承認と成果承認は別々です。
待機は `intent wait --reason ... --resume-condition ...`、中断は `intent pause --reason ...`、
再開は `intent resume --reason ...`。いずれもID、Space、期待revisionを指定します。

Intentの `verification_paths` に、検証へ影響するソース・テスト・設定・共通部品をproject相対パスで指定します。
編集の担当範囲 `scope` とは区別し、TDDとintegrationでは非空の集合が必須です。
分割時はUnitのscope/tests/依存/Boltを計画します。Unitの `verification_paths` は省略するとIntentの集合を使い、
個別指定する場合もIntentの集合内に含めます。同じrootの順次作業を許可し、同一・親子rootの重複workerを拒否します。
独立した別の通常フォルダを使う場合は並列化できます。

`intent hash ID --space SPACE` でSHAを取得し、実テストの前後で一致を確認します。
結果JSONは `aidlc/evidence/` 等へ保存し、現在step_id、stage、verification_scope、verification_sha256、runsを記録します。
runsにはUnitがある場合のunit_id、command、exit_code、非空出力のoutput_pathが必要です。
Unitの `result` はunit-scope結果のunit_id/run_idと担当session/root、その時点のSHAを確認します。
`integrate` は管理元の同じ検証集合が提出SHAと一致することを確認する操作です。別rootの成果ファイルを反映する作業は担当側で行います。
後続Unitによる変更で過去のintegrated状態を戻さず、現在の全体SHAで各Unitとコマンドのテストを行ってからレビューします。
現在stepのUnit結果JSON・出力もレビュー対象なので、内容が変わると以前のレビュー・成果承認は利用できません。
小さなIntentはUnitなしで同じ全体検証を行えます。
完全なJSONと文法は `aidlc intent configure --help`、`aidlc intent hash --help`、`aidlc unit result --help` を参照してください。

新方式は新規配置・新規Intentで開始します。flow schema 6・assignment schema 2を使い、旧記録の互換読込み・変換や併用は行いません。
既存ファイルを自動削除するものではありません。

Knowledgeは現行の仕様と手順、ADRは判断理由です。Concept IDは拡張子なしです。

```sh
aidlc memory create codekb/addition --space default --body-file note.md --actor process:codex --type Design --title "加算" --description "加算の仕様"
aidlc memory show codekb/addition --space default
aidlc memory update codekb/addition --space default --body-file note.md --actor process:codex --expect <hash>
aidlc memory search addition --space default --intent-id <id>
```

草稿はfrontmatterを含まない本文だけです。metadataはCLIが生成し、generated.atは保存時刻になります。
`aidlc memory create --help` / `aidlc memory update --help` で型、statusの値、任意JSON、全flagを確認できます。
helpはIntent未選択でも利用できます。更新時は省略したmetadataを保持し、本文またはmetadataに変更が必要です。ADRは `ADR/name`、typeは `ADR`。不要ならstateに理由を置きます。
一操作ごとの日誌や一律ADRは作成しません。編集失敗後にPostが来ない場合は、AIが失敗終了を確認し、
同じID/Space/sessionへ `session bind --recover` を実行して再試行・検証します。未終了toolはpollします。

## 検証

loopでは実装計画のtargeted testだけを実行します。全package/race/vet/cross-buildは親のfinalに集約します。

```sh
go test -count=1 ./src/internal/flow -run '^TestFlow'
go test -count=1 ./src/internal/app ./src/internal/cli -run '^TestFlow'
go test -count=1 ./src/internal/install ./src/internal/workspace ./src/internal/okfmemory -run '^TestFlow'
go test -count=1 ./src/cmd/aidlc -run '^TestFlowCommand'
```

以下は親のfinalで実行するfresh配布の一周です。非live fixtureと実AIの証拠を区別します。

```sh
go test -tags=integration -count=1 ./src/cmd/aidlc -run '^(TestFlowJourney|TestGitIndependentJourney)$'
AIDLC_FLOW_LIVE=1 go test -tags=integration -v -count=1 -timeout=50m ./src/cmd/aidlc -run '^TestFlowJourneyLive$'
```

liveはCodex CLI 0.153.4、gpt-6-astra/medium、workspace-write、approval=neverを使用します。
HOME/CODEX_HOMEや認証を変更しません。fixtureのtrust mapをCLI引数で渡し、検査済みhookだけを実行します。
既存live fixtureでは固定sandboxの制約によりtest hostがworktree作成・検証bytesのcommit・統合を行います。
これはtest hostの準備・転送方法であり、製品の利用条件ではありません。TestGitIndependentJourneyはGitなしの通常フォルダと製品PATHで検証します。
AIによるGit操作成功とは報告しません。調整役AIは実CLIのstate・割当・review・approval・finishを担当し、
workerは実編集と実test、reviewerは独立read-only会話で固定対象をレビューします。

live evidenceは表示した一時ディレクトリへ保持します。raw hook、Codex JSONL/stdout/stderr、
host job、実testのRED/GREEN・同一test本文hash・source hashを記録します。これらは検証fixtureであり製品auditではありません。
通常testでliveがskipされてもlive成功とは扱いません。timeout、自己申告、test不在も成功にしません。

Knowledge CLIだけの親final検証は次を使用します。liveはhelpの実読取、本文のみの作成・更新、保存metadataをraw hookと実CLI結果で確認します。

```sh
go test -tags=integration -count=1 ./src/cmd/aidlc -run '^TestMemoryMetadataCommand'
AIDLC_MEMORY_LIVE=1 go test -tags=integration -v -count=1 -timeout=15m ./src/cmd/aidlc -run '^TestMemoryMetadataLive$'
```

## 日常運用の実CLI検証

```sh
go test -tags=integration -count=1 -v ./src/cmd/aidlc -run '^TestOperations'
```

複数Intent、別session再開、Git clone引継ぎ、同revision並列更新、Unit割当競合、
実filesystem保存障害と復旧、Git競合の明示解消を一時fixtureで検証します。
実CLIのstdout/stderr/exit、保存stateのhash/revisionと文書bytesを比較します。Git操作はtest runnerが行い、
製品のGit必須条件や実AIによる運用完走の証拠とは区別します。権限障害が効かない環境では成功やskipにせず失敗します。

Git cloneはstate・Knowledge・ADRを保持しますが、runtimeの会話・worker/reviewer割当は共有しません。
新sessionでIntentを選択して再開しても、旧Unitのconfirmや旧reviewのacceptをそのまま引き継げません。
また配置hook/Skillの絶対root/binary参照は自動で移転しません。この検証は配布設定を書き換えません。
同時更新の拒否は先行する排他lock競合またはrevision競合です。旧revisionを再送せず現物を再読込します。
Knowledgeの「Concept saved; bookkeeping failed」は本文保存済みの部分失敗です。返却hashとshowで確認し、
索引障害を取り除いた後、明示した内容またはmetadata変更を現在hashで保存し補助索引を更新します。

## 別フォルダへの配置移転と担当の再割当

日常運用検証で記録した旧path・runtime非共有の制約には、次の明示操作を追加しました。
移転先AIの開始前に端末で実行します。旧pathは存在不要ですが配置済み参照と一致する絶対pathが必要です。

```sh
aidlc install codex --relocate --project-dir NEW_ROOT --from-project-dir OLD_ROOT --from-binary OLD_BINARY
aidlc unit reassign --help
aidlc unit reassign INTENT_ID --space SPACE --expect REVISION --file reassignment.json
```

relocateは現在版のaidlc/aidlc-cli両skillと製品hooksの3ファイルを事前検査し、既知の参照だけを更新し、既存WORKFLOW・独自hook・Knowledgeを保持します。
Pathsは更新済み、Pendingは未処理です。部分失敗は同じ引数で再検査して再試行できます。
新ROOT/.codex/hooks.jsonの絶対pathを確認し、利用者がCodex hook trustを確認します。trust/認証は自動変更しません。

reassignは旧処理の終了を確認してから使います。runningならpause/resumeを経てneeds_confirmationにし、
新しい実在root/session、reason、previous_run_stopped=trueを指定します。rootは通常フォルダで、管理元と同じ場所も使えます。遠隔processは停止しません。
新runでSHAを計算して再テストし、result/integrateします。レビューは別sessionと現在targetで受け直し、同じrootを使えます。
保存途中は同じexpect/JSONで再試行し、すでにrunningなら現在state/assignmentを確認します。

```sh
go test -tags=integration -count=1 ./src/cmd/aidlc -run '^TestRelocationCommand'
AIDLC_RELOCATION_LIVE=1 go test -tags=integration -v -count=1 -timeout=15m ./src/cmd/aidlc -run '^TestRelocationLive$'
```

限定liveは親finalで実行し、固定Codex/model/通常sandboxで移転後の同じIntent選択とKnowledge作成更新を観測します。
2Unitの実CLI引継ぎと実Go testの証拠は、実AI workerの完走とは区別します。

### 製品の5担当

fresh installは `.codex/agents/aidlc-{researcher,requirements,stage-planner,worker,reviewer}.toml` を配置する。
調整役はintent procedureが返す現在手順を読み、調査・要件整理・承認済み実装・固定成果の独立レビューを必要に応じて委譲する。
workerはworkspace-write、残る4担当はread-only。
model/effortは定義で固定せず利用者設定を継承する。共有stateとKnowledge/ADRの保存は調整役が担当する。
既存配置を自動上書きする更新機能ではないため、利用には5定義とaidlc/aidlc-cliの両skillが配置された環境が必要。
配置原稿との一致は `go test -count=1 ./src/internal/install -run '^TestProductAgent'` で確認する。
実際のnamed agent起動は固定Codex環境で別途検証し、配置testだけで実行や任意の成果品質を保証しない。

専用のaidlc-stage-plannerはdiscovery内で要件整理・調査結果が揃った後にメインAIが呼び出します。
Intent/Space、stateと現在計画、6段階カタログ・手順、Rule、要件、調査結果、制約と利用可能な成果物を渡します。
採否・順序・理由・省略理由・期待する文書（なければなし）・不足情報とPLAN.json案を回収し、メインAIがユーザーへ説明して
plan/plan-approvalで保存します。
プログラム・テストコード・commitを文書outputsへ列挙しない。検証証拠の必要性は文書outputsとは別に説明する。途中の追加・省略・並べ替え・reopenにも同担当を使います。
実装手順やUnit詳細のplanning、広い追加調査のresearcher、独立reviewのreviewerとは責任を分けます。
新担当はread-onlyで共有保存・承認・子起動を行いません。model/effortは既存担当と同じく利用者設定を継承します。
定義hashが変わるため新しい配布と新Intentで利用し、旧Intentを移行したり既設定義を上書きしたりしません。

### 段階の開始・終了Sensor

新規Intentはschema5で、旧schemaのIntentはファイルを保持して明示エラーにする。
各段階の入力を正規memory CLIで整え、`intent check ID --space SPACE --boundary start`、
`intent begin ID --space SPACE --expect REV`で開始する。終了check（boundary省略時も終了）が合格したら
独立reviewを割り当て、実報告を受理し、会話の成果承認を記録してfinishする。次段階もbeginが必要。
共有文書の日時やIntentIDの空更新は要求せず、前段合格要件・計画と現在の共有版を区別する。
新しい文書型・必須節・実行証拠JSONはintent procedureが返す段階手順と公開helpを参照する。

限定確認は `go test -count=1 ./src/internal/flow -run '^(TestBoundary|TestStartSensor|TestEndSensor)'`。
実CLIの初期化・目的整理と選択した段階はfinalで `go test -tags=integration -count=1 -v ./src/cmd/aidlc -run '^TestBoundaryJourney$'`。
固定Codex 0.153.4の限定実機は `AIDLC_BOUNDARY_LIVE=1 go test -tags=integration -count=1 -v -timeout 15m ./src/cmd/aidlc -run '^TestBoundaryLive$'`。
後者は未開始拒否→必要文書修復→begin→一般編集の実hook/CLIと現物証拠に限定し、選択計画の完走と同一視しない。
既存model/認証/通常sandboxを保ち、test observerは製品hook出力を変更せず一時fixtureに記録する。


## Stage Graphと現在手順

fresh配置は `aidlc/workflow/stage-graph.json` と `stages/*.md` を含む。
`aidlc intent procedure ID --space SPACE` は現在段階、定義hash、手順path・frontmatter・本文、前進先、差戻し候補をJSONで返す。
aidlcスキルは進行・承認規約とこの読取りを案内し、aidlc-cliは操作目的からhelpへ案内する。段階変更・再開後に現在手順を取り直す。
定義のpathとbytesはIntent作成時に結び付く。変更・欠落時は作業を停止し、元版の復元または新Intentで再開する。
show/list診断は維持する。旧schemaの移行や既設assetの自動上書きは行わない。

reopenは `--step STEP_ID` で変更案を提示し、plan-approvalで承認された時に新IDへ適用する。理由を`aidlc/spaces/<space>/knowledge/log/<intent_id>-work-log.md`へOKFで追記し、過去の完了実績を保持する。新実行回へ過去の合格を流用しない。
`aidlc memory search work-log --space SPACE --intent-id ID`でmetadataを検索し、
`aidlc memory show log/ID-work-log --space SPACE`で本文を読む。検索はtitle/description/tagsを対象とし、intent_idは完全一致で絞り込む。本文全文検索ではない。
初回logはpending前に、正しいmetadataと空の本文を持つOKF文書を原子的に作る。
pending保存前に失敗すると有効な土台が残る場合があるが、要求は未保存なので再試行で新しい時刻を選べる。
pending保存後は元revision・要求・時刻・文書の前後hashを保持し、同一要求のretryだけで完了する。logが変わったり削除されたら元版を復元する。
完成文書が保存済みなら追記や日時更新をせずstate確定だけを再試行する。stateのrevision確定前は文書の存在だけで成功扱いしない。
文書outputsが空でも、既存の実装・テスト・必要ADRの検査は継続する。

loopの指定targetedとaffected通常testの証拠はRAMへ記録する。親final用の限定実行は次のとおり。

```sh
go test -tags=integration -count=1 -v ./src/cmd/aidlc -run '^TestProcedureJourney$'
AIDLC_PROCEDURE_LIVE=1 go test -tags=integration -count=1 -v -timeout 15m ./src/cmd/aidlc -run '^TestProcedureLive$'
```

journeyは実CLIの必須2段階と選択した計画・TDD・統合、TDDからplanningへの差戻し・再前進、Sensor/reviewの拒否を確認する。
限定liveのhostは実CLIでTDD直前までfixtureを準備する（fixtureのreviewは独立AIの実報告ではない）。
実Codexが現在手順を取得し、begin、通常作業、planningへのreopen、新しい現在手順取得を行う。
固定Codex 0.153.4、既存gpt-6-astra/medium・workspace-write・認証保持・test hook方式を使用し、raw Pre/Postとexit、session/Intent、現物を照合する。
本体からCodexを起動するschedulerや追加権限は設けていない。

## 段階の入力選択と出力一覧

新規 Intent は schema 5 です。旧 schema のファイルは保持して明示エラーにし、自動移行しません。会話承認と進捗履歴は実行回IDと計画版へ結び付けます。
`intent procedure ID --space SPACE` は metadata selector・件数・版条件と解決 path、具体的な outputs、診断を返します。条件は完全一致の AND、tags は順序なし集合です。配布 Rule は固定 path `${knowledge_root}/rules/rule.md` と metadata type Rule、current版を参照します。titleは利用者が変更でき、type/title/description/非空本文の検証は維持します。Rule 入口や他の Rule を削除する必要はありません。

```sh
aidlc intent documents ID --space SPACE
aidlc intent documents ID --space SPACE --expect REV --file documents.json
```

```json
{"inputs":[],"outputs":[{"step_id":"現在の実行回ID","stage":"integration","path":"aidlc/spaces/default/knowledge/codekb/addition.md","metadata":{"type":"Knowledge","title":"加算","description":"現行の利用方法"}}]}
```

`inputs` と `outputs` を両方指定して一覧全体を置換します。metadata は type/title/description が必須、status（draft/stable/deprecated）・tags（文字列配列）・intent_id（32桁小文字16進数）が任意です。上の path は利用する Space に合わせます。未存在の output は登録でき、保存後に同じ path と metadata が検査されます。Requirements/ImplementationPlan と新規 adr の Intent ID は登録時に保持されるため、本文保存の `memory create --intent-id ID` にも登録結果の値を使います。generated の日時は memory CLI が生成します。

`configure` は文書一覧を保持し、document_inputs/document_outputs・旧 feature_knowledge・旧 adr.refs の直接指定を拒否します。既存共有 Knowledge/adr の ID や日時を形式だけのために書き換えません。ADR の type とフォルダは小文字 `adr`、概念名は引き続き ADR です。新しい成果文書は outputs へ、採用する過去 ADR は inputs へ具体 path で登録します。

accepted 入力は合格した path/hash を保持し、変更には前段への reopen が必要です。共有 current 文書は更新できますが、開始時の選択を別 path へ黙って交換できません。入力変更は begin を無効化し、現在・将来段階の output 追加だけなら begin を保持します。
`test_results` は文書宣言と異なり、実行後に存在する strict JSON と出力ファイルだけを登録します。将来の integration 結果は実行後に追記します。

親 final 用の実 CLI 一周は `go test -tags=integration -count=1 -v ./src/cmd/aidlc -run '^TestIntentDocumentsJourney$'` です。loop では実行しません。

## 実行計画と会話承認

`intent show` の `current_step_id` が現在の実行回です。同じstageでも再実行は別IDになります。
`intent procedure` はその回の短い段階手順を返します。共通CLI詳細は配置済みaidlc-cliと各helpを参照します。
`intent plan ID --space SPACE` で承認済み計画と変更案を読み、`--expect REV --file PLAN.json` で変更案を提示します。
任意4段階は選択するか省略理由を記します。初回の目的整理終了までに計画承認を完了してください。

計画と成果のrequest_id・targetを明示して回答を待ちます。UserPromptSubmitで到着した実際の回答の
session・turn・quoteを、それぞれ `plan-approval` と `approval` のJSONに記録します。
両requestが回答到着前に存在し対象が不変なら、同じ回答を両方へ使えます。AIの自己承認はできません。
承認待ち中は通常作業を停止し、読取り・質問・計画整理・正規承認を行います。
変更後は以前のSensor・review・承認を使わず、必要な開始検査から取り直します。

未完了の現在回をreopenすると新IDに置換し、旧回と理由・変更前後の計画は `intent history` に残します。
完了済みの回は有効な計画にも保持します。例えば初期化完了後の目的整理を再実行すると、
`s01 initialization completed, s03 discovery pending (reopens:s02)` になります。
Entry・文書宣言・実測JSON・Unit要求には実際のstep_idを用い、過去回のUnit結果やテスト結果を転用しません。
次回へ進んだらUnit計画が必要な場合も現在回に登録し直します。
履歴はstateのheadが指す確定列だけを表示し、途中保存の未確定ファイルを成功扱いしません。

限定実機の会話承認確認は親finalで実施します。
`AIDLC_HUMAN_APPROVAL_LIVE=1 go test -tags=integration -count=1 -v -timeout 20m ./src/cmd/aidlc -run '^TestHumanApprovalLive$'`
は試験用の2承認待ちを準備し、実hookの作業拒否、同じ回答の出典、別承認CLIの成功、finishと確定履歴を照合します。

プロジェクトの共通ルールはknowledge/rules/rule.mdに記載します。初期状態は「追加ルールはありません」で、製品が言語や設計制約を決めません。AI-DLCの進行・会話承認・記録先はaidlcスキル、操作案内はaidlc-cli、正確な引数・JSONはhelpにあります。工程手順はstage、担当責務はagent定義から取得します。新配布にはWORKFLOW.mdを含めません。既設Ruleや旧配置は自動移行・削除せず、新版は新配布・新Intentで利用します。

## native担当と作業場所の予約

メインAIが標準の`spawn_agent`で担当を起動します。CLIは起動しません。現在の担当一覧は`intent procedure`で確認します。
workerには、管理root全体で共通の`assignment`予約を先に作ります。別Space・別Intent・別会話でも同一・親子rootを二重に予約できません。rootのsymlink別名は正規化して扱います。
登録rootと実childの実行rootが一致することや、OS上の全process停止までを保証する仕組みではありません。

人間が既知の作業と残存処理を整理したことを確認してから、`assignment init --file INIT.json`を実行します。
`INIT.json`は`request_id`、`human_confirmed:true`、回答と理由を記す`reason`です。通常操作は欠落したregistryを自動作成しません。
Unitなしでは、開始Sensorと承認を満たしたIntentに`assignment reserve ID --space SPACE --session MAIN --expect REV --file RESERVE.json`を使います。
JSONの`registry_epoch/request_id/step_id/agent/root/session`は`assignment reserve --help`の例に従います。
Unitありは`unit claim`のJSONへ`registry_epoch/request_id/coordinator_session`を追加します。
rootは既存の通常ディレクトリを指定し、管理元と同じ場所も使えます。Unitは依存Unitのintegrated状態を確認します。
Git履歴やremote URLは割当条件に含みません。同じrootのworkerは停止確認と明示解放を経て順次割り当てます。

`assignment list/show`の`task_name`と`agent`をnative spawnへ渡します。`fork_turns`が提供される場合は`none`にし、Ruleと必要な資料を渡します。
追加依頼前に`assignment check ASSIGNMENT`を実行し、応答確認済みの相対`task_name`を使います。
Post欠落時は同名を再利用せず、予約を不明として保持します。Stop、interrupt、結果提出、時間経過は解放の根拠になりません。
メインAIが追加依頼終了・既知コマンド/background終了・成果と残件回収を確認して、`assignment release --help`のJSONと現在entry revisionで解放します。
不明なら保持し、人間へ確認します。session引数や確認回答は申告であり、本人認証ではありません。

保存や応答を失った場合は、同じepoch/request_id・内容で照会・再試行します。Unit保存途中は元の要求以外の更新を拒否します。
registryは`aidlc/.runtime/assignments/registry.json`に置き、Gitで共有しません。元ファイルを復元できる場合は復元を優先します。
復元不能な場合だけ人間の確認を記録し、`assignment reset --help`に従って元epoch/hashまたは欠落・破損診断を指定します。
resetは新epochを発行し、読める旧記録を保管します。古い要求は拒否されます。reset自体はworkerを停止しません。

候補は隔離した新規配置で確認し、新旧の進行中stateを共有しません。既存環境のbinary/hooks/skills/定義とruntimeを保管し、利用者のRule・Knowledgeを置換しません。
現在形式の通常移転では、既知の子と処理の終了を確認してから上記relocate手順を使います。
新しいhooksの絶対pathと通常のCodex trustを確認し、許可/拒否の対照を取ります。実際の利用先への適用はリポジトリ開発とは別作業です。
旧matcherのrelocateは参照を移すだけで担当保護を追加しません。未知編集は自動上書きしません。
定義hashが変わるため、旧Intentは旧定義と対応版で扱うか、旧作業と成果を確認して新Intentへ新規claimします。旧Unitへ予約を後付けしません。
ロールバックも停止確認後にbinary/hooks/skills/定義を対応する組で戻し、registry・進捗・Knowledgeを削除して空き扱いにしないでください。

固定実機の観測fixtureは`AIDLC_ASSIGNMENT_LIVE=1 go test -tags=integration -count=1 ./src/cmd/aidlc -run '^TestAssignmentLive$' -timeout 30m`です。
既存Codex CLI 0.153.4、macOS arm64、gpt-6-astra/xhighを使い、専用temp rootだけを変更します。認証ファイルは読み取り・コピーしません。
一時fixtureのhook trust bypassを利用者配置へ適用しないでください。出力された`aidlc-assignment-live-*`にはcase別のhook生入力/出力、model transcript、process印、manifestを残します。
モデルexit 0だけでは成功としません。許可/拒否、実並列、追加依頼、明示解放、Post欠落・保存失敗をrawで確認し、未実行は未確定として報告します。

## 配布候補と既設更新

6targetのarchive・manifest・SHA256SUMS生成、native展開/導入確認、利用者dataを保全する比較・手動切替・復旧は[配布手順](distribution.md)を参照してください。開発用`src/cmd/aidlc-dist`はlocal/CI候補を生成し、tag/Release/uploadを行いません。既存relocateは参照補正であり、自動updaterではありません。

## Hook probeの名称

hookの入力・transportを照合する補助testは `src/cmd/aidlc/hook_probe_test.go`、
実機用の補助処理は `hook_probe_live_test.go` にあります。
`TestHookProbeVerify` 等の限定testで証拠の検査処理を確認できます。
実機用の起動指定は `AIDLC_HOOK_LIVE=1`、保存済み証拠を再検査する入力先は `AIDLC_HOOK_EVIDENCE` です。
通常のtest成功を実機検証の成功に数えず、実機実行は承認済み計画のfinal範囲で行います。
