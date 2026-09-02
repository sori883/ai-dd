# 初期state永続化writerの実装計画

- 日付: 2026-09-02
- 状態: Accepted（Issue #45、実装完了・独立review前）
- GitHub Issue: [#45](https://github.com/sori883/ai-dd/issues/45)
- 承認: 2026-09-02、ユーザー承認済み計画を親agentから受領
- base: `3a6932d`
- 作業branch: `codex/initial-state-writer`
- 関連: [初期state builder](../decisions/2026-09-02-initial-state-builder-plan.md)

## 目的と境界

`state.BuildInitial`が返す2つのcanonical payloadを、既に開かれたrecord `*os.Root`の固定leafへ
永続化する。実装APIは次のとおりである。

```go
func WriteInitial(recordRoot *os.Root, initial Initial) error
```

`recordRoot`の選択・open・Close、lock、record directory作成はcallerの責務である。writerは
`project-description.json`へ`Initial.ProjectDescriptionJSON`、続いて`aidlc-state.md`へ
`Initial.StateContent`を文字列のままexact bytesで保存する。空payloadも有効なbytesとして保存する。
stateが存在しない場合は作成し、既存stateがstubであることを要求しない。

## 保存契約

各leafは同一directoryの一意なsibling temporary fileへ、`O_WRONLY|O_CREATE|O_EXCL`、全量write、
Close、`Root.Rename`の順で置換する。temporary fileはこのwriterが取得したものだけを失敗時にcleanupし、
衝突した他writerのfileは削除しない。short writeは`io.ErrShortWrite`として扱い、write・Close・Rename・
cleanupの原因はcontext付きで返す。

既存`aidlc-state.md`はsidecar保存後、state置換前に`Lstat`で通常fileであることを確認し、通常fileなら非truncateの
`O_WRONLY` open/Closeをwrite barrierとして実行する。directory、symlink、その他の特殊file、barrierの
失敗ではstateを変更しない。sidecarは本家の順序どおり既にcommit済みとなり、rollbackしない。`aidlc-state.md`
不存在時はこのbarrierを省略する。

sidecarのRename成功が最初のcommit境界である。その後のstate保存失敗ではsidecarをrollbackせず、
既存state（または不存在状態）を保持する。WindowsでRenameが既存leafを原子的に置換できない場合の
delete-before-rename fallbackは行わない。

## 意図的な差分

比較対象はローカルAI-DLC `2.6.123`の`writeFileAtomic`とstate-build経路であり、最新upstream全体との
一致は主張しない。本家がtemporary cleanup errorを抑止するのに対し、Go版は元errorとcleanup errorを
`errors.Join`で返す。これは失敗原因を失わないための承認済み唯一の意図的差分である。その他の成功bytes、
保存順、partial commit、Windowsの非atomic可能性は本家に合わせる。

## TDDと検証

`verification_mode=loop`で、次の順に公開`WriteInitial`と非公開failure-injection seamのobservable
behaviorを固定する。

1. nil rootのmutation前error、sidecar→stateの保存順、exact bytes、Close-before-Rename。
2. 既存stateのwrite barrier、nonregular fail-closed、state不存在作成、空payload。
3. sidecarのcreate/write/short write/Close/Rename/cleanup失敗とstate未変更。
4. stateのcreate/write/short write/Close/Rename/cleanup失敗、sidecar保持、旧state保持、原因join。
5. sibling temporaryのcollision retry・他writerのtemp非破壊、caller-owned root継続利用。
6. 実`os.Root`でstub置換、state不存在作成、exact bytes、成功後tempなし、root未Closeをintegration testで確認。

修正中に実行するtargeted commandは次の2つだけである。

```sh
go test -count=1 -run '^TestWriteInitial' ./src/internal/state
go test -tags=integration -count=1 -run '^TestWriteInitial' ./src/internal/state
```

loopでは全体test、race、vet、lint、cross compile、配布E2Eを実行しない。Go変更にはgofmtを適用し、
親agentが独立review後にfinal gateを実施する。

## 所有権

- `go_tdd_implementer`: `src/internal/state/write.go`, `write_test.go`, `write_integration_test.go`、
  `.github/workflows/ci.yml`、本計画、architecture/development、RAM索引
- 親agent: Issue、commit、push、独立review、final gate、PR、Issue close

state writerはaudit、CLI、Intent作成接続、directory作成、lock管理を行わない。

## 実装記録

実装は承認済みの境界内で完了した。変更したGoコードは`write.go`、unit testは`write_test.go`、実`os.Root`
を使うfilesystem integration testは`write_integration_test.go`、CIにはstate integration stepを追加した。
外部Go moduleは追加していない。

TDDでは、まず次のREDを確認してから最小実装を追加した。

- baselineのtargeted testは実装前で`ok [no tests to run]`だった。
- nil rootの最初のobservable testは`undefined: WriteInitial`でREDになった。
- 保存順のtest追加時は`undefined: writeInitialWithOps` / `initialWriteOps`でREDになった。

実装後、gofmtを適用して次のloop gateを実行し、いずれもGREENとなった。

```text
go test -count=1 -run '^TestWriteInitial' ./src/internal/state
ok
go test -tags=integration -count=1 -run '^TestWriteInitial' ./src/internal/state
ok
```

targeted testではsidecar→stateの順序、exact bytes、既存stateのwrite barrier、nonregular fail-closed、
empty payload、各failure point、short write、close-before-rename、collisionしたtemporaryの非破壊、
sidecar成功後のpartial commit、cleanup errorのcause、caller-owned root継続利用を固定した。独立reviewと
親agentによるfinal gateは未実施であり、実行後にこの記録を更新する。
