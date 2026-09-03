# StartIntentとin-flight recomposeの参照契約

- 調査日: 2026-09-03（Asia/Tokyo）
- 状態: Current（このrepositoryに固定された本家AI-DLC `2.6.123`の確認範囲）
- 対象Issue: [#61](https://github.com/sori883/ai-dd/issues/61)

## 背景

Go再実装にはIntent作成、workspace分析、Stage graph、scope metadata、初期state builder、state writerが
それぞれ存在する。`StartIntent`は、callerが解決したlabel、scope、description、repositoryを使ってこれらを
一つのworkspace lock内へ接続する内部APIである。自然文の解析、scopeの自動選択、公開CLIはこの契約に含めない。

本家の確認は、固定snapshotの次の実装と関連testを静的に照合した範囲に限る。最新upstream全体との一致や、
未確認箇所の完全互換は主張しない。

- `docs/実装_aidlc-workflows/core/tools/aidlc-utility.ts` のIntent作成・初期state経路
- `docs/実装_aidlc-workflows/core/tools/aidlc-lib.ts` のregistry、cursor、state保存経路
- `docs/実装_aidlc-workflows/core/tools/aidlc-orchestrate.ts` のruntime routing/recompose経路
- `docs/実装_aidlc-workflows/tests/unit/t194-recompose.test.ts`
- `docs/実装_aidlc-workflows/tests/unit/t232-phase-progress-flip.test.ts`

## StartIntentの接続契約

workspaceのregistry RenameがIntent作成のcommit境界である。commit前のerrorはzero `CreatedIntent`、commit後の
cursor・初期化・Root Close・lock release errorは作成済み`CreatedIntent`とerrorを併せて返す。初期化用の狭い
seamは次の責務を持つ。

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

initializerはregistry commitとcursor処理後、workspace lockを保持し、project/record RootをCloseする前に実行する。
cursor処理が失敗しても明示record Rootを開いてinitializerを呼ぶ。cursor、initializer、Root Close、lock releaseの
複数原因は`errors.Join`で保持し、partial Intentをrollbackしない。既存`CreateIntent`はinitializerなしで従来の
commit境界と観測可能な挙動を保つ。

`StartIntent`のinitializerは、次の順で処理する。

1. `workspace.Detect(projectRoot)`
2. `graph.Load(DataFS)`
3. `scope.ReadAll(ScopesFS)`と入力scopeのcase-sensitive exact選択
4. Intent作成後にUTC timestampを一度取得
5. `state.BuildInitial`へworkspace summary、scope metadata、説明、override、timestampを渡す
6. `state.WriteInitial(recordRoot, initial)`

`DataFS`と`ScopesFS`はcaller-ownedで、StartIntentはCloseしない。BuildInitialの返値はwrite前に結果へ保持し、
write成功時だけ`InitializationComplete`をtrueにする。write成功後のRoot Close・lock release errorでもtrueは保持する。
graph、scope、build、write errorではcommit済みIntentを保持し、初期化完了はfalseとする。

## 初期stateとrecompose

初期stateの全Stage行へ`EXECUTE`または`SKIP` suffixを保存する。`Initial.Plan`はその時点のgraph・scope・workspace
から得るin-memory結果であり、`.aidlc-plan.json`、`.aidlc-stage-plan.json`などの永続runtime sourceは作らない。
Greenfield補正、submodule情報、off-path producer advisoryは既存の構造化結果へ保持し、producer不在は既存の
fail-closed方針を継承する。

将来のin-flight recomposeでは、保存済み`aidlc-state.md`のsuffixを正とする。人間のApprove/Edit gate後、currentより
後ろのpending Stageだけを変更し、completed・in-progress・current以前は凍結する。Current Stage、checkbox marker、
scope・Depth・Test Strategy・Review Overrideは変更しない。recompose本体、state parser、audit接続はIssue #61の
範囲外である。

## Go実装へ引き継ぐ制約

今回新しい本家との差分は追加しない。malformed graph/scope fail-closed、strict registry、cursor failure通知、
stale lock非回収、`os.Root`/nonregular state、producer不在早期拒否は既存の承認済みGo方針として維持する。
外部Go moduleは追加しない。

### 未確認事項

最新upstreamとの差分、全workflowのrecompose相互作用、state/auditのcrash耐久、2つのdata FSの同時更新、
`os.Root`でmount/deviceまで隔離できるかは、この参照契約の確認範囲ではない。
