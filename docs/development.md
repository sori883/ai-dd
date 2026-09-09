# 開発と利用の手順

Go 1.26以上とGitを使用します。外部Go moduleは承認済みYAML parserだけです。

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

初期化、目的整理、および選択した各段階の境界で別rootの独立read-only reviewを受けます。
`intent review` のassignは担当session/rootを指定し、acceptはその担当の実報告と対象hashを受け取ります。
修正で対象が変わったら再reviewします。実際の会話回答を `intent approval` へ記録し、`intent finish` が現在回を完了します。
初期化→目的整理は必須です。構成分析・計画・TDD・統合検証の採否と順序は `intent plan` で提示し、
`intent plan-approval` で会話承認を記録します。計画承認と成果承認は別々です。
待機は `intent wait --reason ... --resume-condition ...`、中断は `intent pause --reason ...`、
再開は `intent resume --reason ...`。いずれもID、Space、期待revisionを指定します。

分割時はUnitのscope/tests/依存/Bolt/base_commitを計画します。調整役AIがworkerを起動し、
別worktreeへ割り当て、成果commitを統合します。`unit claim/result/integrate/confirm` のJSONと
完全な操作文法は `aidlc unit claim --help` と [実行計画契約](design/intent-execution-plan-proposal.md) を参照してください。
小さなIntentはUnit分割せず、直接実装の受入・検証・結果commitを定義できます。

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
go test -count=1 ./src/internal/minimal ./src/internal/cli -run '^TestFlow'
go test -count=1 ./src/internal/install ./src/internal/workspace ./src/internal/okfmemory -run '^TestFlow'
go test -count=1 ./src/cmd/aidlc -run '^TestFlowCommand'
```

以下は親のfinalで実行するfresh配布の一周です。非live fixtureと実AIの証拠を区別します。

```sh
go test -tags=integration -count=1 ./src/cmd/aidlc -run '^TestFlowJourney$'
AIDLC_FLOW_LIVE=1 go test -tags=integration -v -count=1 -timeout=50m ./src/cmd/aidlc -run '^TestFlowJourneyLive$'
```

liveはCodex CLI 0.153.4、gpt-6-astra/medium、workspace-write、approval=neverを使用します。
HOME/CODEX_HOMEや認証を変更しません。fixtureのtrust mapをCLI引数で渡し、検査済みhookだけを実行します。
固定sandboxのGit制約によりtest hostがfixtureのworktree作成・検証bytesのcommit・統合を行います。
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
実AIによる運用完走の証拠とは区別します。権限障害が効かない環境では成功やskipにせず失敗します。

Git cloneはstate・Knowledge・ADRを保持しますが、runtimeの会話・worker/reviewer割当は共有しません。
新sessionでIntentを選択して再開しても、旧Unitのconfirmや旧reviewのacceptをそのまま引き継げません。
また配置hook/Skillの絶対root/binary参照は自動で移転しません。この検証は配布設定を書き換えません。
同時更新の拒否は先行する排他lock競合またはrevision競合です。旧revisionを再送せず現物を再読込します。
Knowledgeの「Concept saved; bookkeeping failed」は本文保存済みの部分失敗です。返却hashとshowで確認し、
索引障害を取り除いた後、明示した内容またはmetadata変更を現在hashで保存し補助索引を更新します。

## 別cloneへの配置移転と担当の再割当

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
新しい実在worktree/session、現在HEAD、reason、previous_run_stopped=trueを指定します。遠隔processは停止しません。
新runで再テストしてresult/integrateし、レビューを新root/sessionと現在targetで受け直します。
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
