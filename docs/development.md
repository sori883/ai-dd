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

理解、計画、TDD、統合検証の各境界で別rootの独立read-only reviewを受けます。
`intent review` のassignは担当session/rootを指定し、acceptはその担当の実報告と対象hashを受け取ります。
修正で対象が変わったら再reviewします。`intent advance` は一段階だけ進めます。
待機は `intent wait --reason ... --resume-condition ...`、中断は `intent pause --reason ...`、
再開は `intent resume --reason ...`。いずれもID、Space、期待revisionを指定します。

分割時はUnitのscope/tests/依存/Bolt/base_commitを計画します。調整役AIがworkerを起動し、
別worktreeへ割り当て、成果commitを統合します。`unit claim/result/integrate/confirm` のJSONと
完全な操作文法は [詳細契約](design/four-stage-workflow-contract.md) を参照してください。
小さなIntentはUnit分割せず、直接実装の受入・検証・結果commitを定義できます。

Knowledgeは現行の仕様と手順、ADRは判断理由です。Concept IDは拡張子なしです。

```sh
aidlc memory create knowledge/addition --space default --file note.md --actor process:codex
aidlc memory show knowledge/addition --space default
aidlc memory update knowledge/addition --space default --file note.md --actor process:codex --expect <hash>
aidlc memory search addition --space default --intent-id <id>
```

更新時は既存metadataを保持します。ADRは `ADR/name`、typeは `ADR`。不要ならstateに理由を置きます。
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
AIによるGit操作成功とは報告しません。調整役AIは実CLIのstate・割当・review・advanceを担当し、
workerは実編集と実test、reviewerは独立read-only会話で固定対象をレビューします。

live evidenceは表示した一時ディレクトリへ保持します。raw hook、Codex JSONL/stdout/stderr、
host job、実testのRED/GREEN・同一test本文hash・source hashを記録します。これらは検証fixtureであり製品auditではありません。
通常testでliveがskipされてもlive成功とは扱いません。timeout、自己申告、test不在も成功にしません。
