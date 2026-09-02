# Stage graph・scope routing内部APIの実装計画

- 日付: 2026-09-02
- 状態: Accepted（実装・独立review完了）
- GitHub Issue: [#35](https://github.com/sori883/ai-dd/issues/35)
- base: `e720440`
- 作業branch: `codex/graph-routing`
- 関連: [参照契約](../research/2026-09-02-stage-routing-contracts.md)、
  [初期実装の境界](2026-08-29-initial-implementation-boundaries.md)

## 承認と境界

workspace分析の次に、compiled stage graphとscope gridを読み取り、後続のstate初期化・routingが
利用できるimmutableな内部snapshotを独立sliceとして追加する詳細計画を提示し、ユーザーが
2026-09-02に明示承認した。GitHub Issue #35と本計画で実装範囲を固定する。

今回変更するのは`src/internal/graph`のread-only query、synthetic test、RAM、architecture、
development手順だけである。stage definition本文、scope metadata Markdown、state、audit、agent実行、
Intent作成core、CLI、配布dataは変更しない。Go標準libraryだけを使う。

## 内部API

callerが`stage-graph.json`と`scope-grid.json`の存在するdata directoryへroot化した`fs.FS`を渡す。

```go
func Load(dataFS fs.FS) (Snapshot, error)

func (Snapshot) Stages() []Stage
func (Snapshot) ScopeNames() []string
func (Snapshot) Scope(name string) (Scope, bool)

func (Scope) Action(stageSlug string) Action
func (Scope) Actions() map[string]Action
```

`Stage`はslug、number、name、phase、execution、lead agent、support agents、mode、fallback用scopes、
enabled判定を保持する。`Action`のzero valueは未知sentinelとし、正常snapshotには保存しない。
scope内でstage cellが欠損した場合の公開結果は本家runtimeどおり`SKIP`、未知scopeはbool falseである。
`ScopeNames`はexplicit gridとfallbackのどちらも本家JavaScript互換のUTF-16 code-unit順とし、
JSON objectの記述順を契約にしない。

Snapshotが返すstage slice、scope name slice、stage内slice、action mapは防御的copyとし、callerの変更で
保存済みsnapshotを変化させない。nil FSはpanicせずerrorにする。loaderは`fs.FS`だけを受け取り、
write、cwd、environment、Root lifecycleを所有しない。

## Load・fallback・検証

stage graphはJSON配列順を保つ。`enabled:false`は公開stageから除外し、fallbackもenabled stageだけを
使う。ただしscope gridの参照検証ではdisabledを含む全graph slugを既知とする。

stage graphのread、欠損、JSON decode、構造validation errorは文脈付きerrorとし、zero Snapshotを返す。
必須field、`support_agents`のstring array、slug・number重複、`ALWAYS` / `CONDITIONAL`以外を
fail-closedにする。stage field名は大小文字を含む完全一致で解釈し、unknown JSON fieldは無視する。

scope gridのread errorまたはJSON構文errorだけは、enabled stageの`scopes` membershipを純粋転置する。
scope名をUTF-16 code-unit順にsortし、各enabled stageをmembershipに応じて`EXECUTE` / `SKIP`へする。
compiler / designer側のinitialization特例はruntime `loadScopeMapping` queryの対象外なので持ち込まない。

構文上validなgridのtop-level、scope entry、必須`stages`が構造不正ならfallbackせずerrorにする。
actionは`EXECUTE` / `SKIP`だけを認め、全graphにないstage参照を拒否する。partial mapは有効で、
disabled stage参照はvalidだが公開`Actions`から除外する。

## 本家との意図的な差分

比較対象は[参照契約](../research/2026-09-02-stage-routing-contracts.md)に記したローカルAI-DLC
`2.6.123`の範囲であり、最新upstream全体との一致は主張しない。

| 本家の挙動 | 採用する挙動 | 変更理由 | 利用者・互換性への影響 |
| --- | --- | --- | --- |
| JSON parse後のgraph/gridをruntime typeへcastし、構造・enum・参照をLoad境界では網羅検証しない | 必須stage field、重複、execution/action enum、grid構造、stage参照をLoad時にfail-closed検証する | 壊れたroutingを正常snapshotとして後続へ渡さない | 本家で遅延errorや暗黙SKIPになり得るmalformed dataがLoad errorになる。正常なcompiled data、grid read/syntax fallback、missing actionのSKIPは維持 |

## Assertion-first TDD

実装writerを`go_tdd_implementer`の1名に限定し、compile failureをRED証拠にせず、最小stubで
runnable assertionへ到達してから次を順にRED、最小GREEN、GREEN上のrefactorで進める。

1. stage field load、配列順、enabled filtering。
2. scope routing、missing actionのSKIP、unknown scope、disabled actionの非公開化。
3. scope grid missing・JSON syntax error時のenabled `stage.scopes`転置。
4. stage graph read・decode errorの文脈とcause、error時zero Snapshot。
5. stage/grid構造、required field、duplicate、enum、stage referenceのfail-closed。
6. returned slice・map・Stage内sliceの防御的copyとnil FSの非panic error。

test fixtureは`testing/fstest.MapFS`で必要最小限のJSONだけを作り、本家data一式をrepositoryへ
複製しない。unknown field、partial action map、disabled stage referenceも回帰guardで固定する。

独立reviewで見つかった`encoding/json`の大小文字を無視するstruct field照合とGo string sortの
UTF-16順との差は、それぞれexact-key decodeとUTF-16 code-unit比較の回帰testを先にREDとして固定して
修正した。どちらも本計画の本家互換契約を満たすparity bug修正で、新しい意図的な差分ではない。

## 所有権と対象外

- `go_tdd_implementer`: `src/internal/graph/**`、本計画・調査RAM、`docs/architecture.md`、
  `docs/development.md`、RAM索引
- 親agent: GitHub、commit、固定diffの独立review、PR、最終gate
- `independent_reviewer`: 固定base/headのread-only review

同時に複数writerを動かさない。stage/scope authored definition、metadata parser、state machine、
agent dispatch、CLI、filesystem writer、external moduleが必要になった場合は実装を広げず停止する。

## 検証と完了gate

```sh
go test -count=1 ./src/internal/graph
go test -count=1 -shuffle=on ./...
go test -count=1 -race -shuffle=on ./...
go vet ./...
go mod tidy -diff
gofmt -l src/internal/graph
gopls check src/internal/graph/graph.go src/internal/graph/graph_test.go
git diff --check
```

darwin、linux、windowsのamd64/arm64でgraph test binaryをrepository外へcross compileし、各OSでの
native実行証拠とは区別する。read-only internal APIなので配布CLI E2Eは行わない。固定base/headの
独立reviewでblocking findingがなく、fresh検証とGitHub checksが成功してからIssue #35に紐づく
日本語PRを作る。自動mergeしない。

## 実装・検証結果

単独writerが6個のobservable sliceをRED→GREENで実装し、commit `76ce35c`へ固定した。最初の独立reviewで、
Goのstruct向けJSON decodeが大小文字違いのfield名を受理するP1と、Go string sortがJavaScriptのUTF-16順と
異なるP2を指摘した。それぞれexact-key decodeとUTF-16 code-unit比較の回帰testをREDとして追加し、
commit `c89c7cb`で修正した。修正後の固定diffを再reviewし、元のP1/P2解消と追加findingなしを確認した。

TDD実行記録は`/tmp/ai-dd-issue-35-tdd.log`、SHA-256は
`8d2d6c103f7e63430601581119c0ae6bde61b8dddb12a6729c1afa9b572963e8`である。記録はcommand、exit、
代表failure、結果を残した構造化要約であり、各RED時点の一時source全体やprocess transcriptは保存していない。

対象package、全package shuffle、race、vet、tidy差分、gofmt、gopls、diff checkはすべてPASSした。
darwin・linux・windowsのamd64/arm64向けgraph test binaryも最終実装から6構成でcross compileした。
cross compileは各対象OSでのnative test実行証拠ではない。外部Go moduleは追加していない。
