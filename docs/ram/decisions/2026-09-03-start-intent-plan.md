# Issue #61 StartIntent内部接続の実装計画

- 状態: Accepted
- 承認日: 2026-09-03（ユーザー明示承認）
- 対象Issue: [#61](https://github.com/sori883/ai-dd/issues/61)
- verification mode: loop（独立review後のfinal検証は親agentが担当）

## 目的

既存のworkspace Intent作成core、workspace分析、graph/scope reader、初期state builder、state writerを、
内部`orchestrator.StartIntent`として利用可能な開始処理へ接続する。Intentをregistryへcommitした後の初期化失敗を
rollbackせず、呼出し側がpartial Intentと初期化結果を診断できる契約を固定する。

## 承認対象のAPIと責務

workspaceへ次の狭いinitializer seamを追加する。

```go
type IntentInitializer func(
    projectRoot *os.Root,
    recordRoot *os.Root,
    created CreatedIntent,
) error

func CreateIntentWithInitializer(
    ctx context.Context,
    root RootInput,
    input IntentCreateInput,
    initialize IntentInitializer,
) (CreatedIntent, error)
```

initializerはregistry commit・cursor処理後、lock保持中、project/record Root Close前に明示record Rootを受け取って
実行する。cursor failureでも実行し、cursor、initializer、Root Close、lock releaseの原因をjoinする。既存
`CreateIntent`はinitializerなしでcommit前zero／commit後result+errorの挙動を維持する。

orchestratorへ次の内部APIを追加する。

```go
func StartIntent(ctx context.Context, input StartInput) (StartedIntent, error)
```

StartIntentはinitializer内で`workspace.Detect`→`graph.Load`→`scope.ReadAll`とexact scope選択→UTC timestamp一回取得→
`state.BuildInitial`→`state.WriteInitial`を行う。DataFS/ScopesFSはcaller-ownedでCloseしない。BuildInitial結果はwrite前に
`StartedIntent.Initial`へ保持し、write成功だけ`InitializationComplete=true`とする。graph/scope/build/write failureは
registry commit後のIntentとerrorを保持し、rollbackしない。

## 対象ファイルと所有権

- 変更: `src/internal/workspace/intent_create.go`
- 変更: `src/internal/workspace/intent_create_test.go`
- 新規: `src/internal/orchestrator/start_intent.go`
- 新規: `src/internal/orchestrator/start_intent_test.go`
- 新規: `src/internal/orchestrator/start_intent_integration_test.go`
- 更新: `docs/architecture.md`, `docs/development.md`
- 新規: 本計画、参照契約、in-flight recompose方針
- 必要時のみ最小fixture/test調整

この実装期間のwriterは担当agent一名とし、範囲外の変更、Issue/PR操作、commit、push、merge、外部Go module追加は
行わない。既存差分はrevert・上書きせず維持する。

## TDDと検証

observable behaviorごとに次の順で進める。

1. initializerの実行順、lock保持、Root所有、commit境界をRED→最小GREEN。
2. cursor failureでもinitializerを実行し、複数causeを保持するRED→GREEN。
3. StartIntent成功順、exact scope、caller-owned FS、単一UTC timestampをRED→GREEN。
4. graph/scope/build/write failureのpartial Intent、Initial保持、completion flagをRED→GREEN。
5. 実filesystem integrationでregistry、cursor、description、canonical state suffix、sidecar不在を確認する。
6. green上の小さなrefactor後、変更Goへgofmtを適用し、対象testを再実行する。

必須targeted検証は次であり、loop中に全package test、race、vet、全体lint、cross compile、配布E2Eを実行しない。

```sh
go test -count=1 -run '^TestCreateIntentWithInitializer' ./src/internal/workspace
go test -count=1 ./src/internal/orchestrator
go test -tags=integration -count=1 ./src/internal/orchestrator
go test -count=1 -run '^(TestBuildInitial|TestWriteInitial)' ./src/internal/state
```

## 本家準拠と既存差分

比較対象はrepository固定の本家AI-DLC `2.6.123`。初期stateのsource of truthは全Stage suffixであり、Plan sidecarや
永久固定照合は採用しない。将来のrecomposeは別Issueで、人間承認後にpending Stageだけを変更する。

今回意図的な新規差分はない。既存のmalformed graph/scope fail-closed、strict registry、cursor failure通知、stale lock
非回収、`os.Root`/nonregular state、producer不在早期拒否は維持する。既存の粗いロードマップとStage Plan計画にある
「実行中は固定」という決定は、[in-flight recompose方針](2026-09-03-inflight-recompose-policy.md)で置換対象として参照する。
