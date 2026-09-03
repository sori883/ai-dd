# 既存state atomic update writerの実装計画

- 日付: 2026-09-03
- 状態: Accepted
- GitHub Issue: [#75](https://github.com/sori883/ai-dd/issues/75)
- 実装許可: [薄いライフサイクルマイルストーン](2026-09-03-thin-lifecycle-milestone.md)とロードマップ包括承認内

## 現状と目的

`state.Patch`は保存予定の新しい`aidlc-state.md` bytesを、未知内容を保持して純粋に構築できる。既存の`state.WriteInitial`には同一record root内で一時fileを書き、close後にrenameする機械部分があるが、project descriptionと初期stateを作るinitializer専用である。

本変更では、既存の`aidlc-state.md`だけを置換する更新専用writerを追加する。後続のgateやadvance処理が、readerへpartial stateを見せずに検証済みbytesを保存できる境界を提供する。

## 設計

- caller-ownedのrecord `*os.Root`とreplacement bytesを受け、RootをCloseしない。
- replacement bytesを`state.Parse`で検証し、malformed stateをfilesystemへ書かない。
- target `aidlc-state.md`は既存のregular fileでなければならず、不在、directory、symlink、FIFO等をfail-closedで拒否する。
- targetのwriteabilityを置換前に確認し、read-only targetをrenameで迂回して更新しない。
- targetと同じRoot内に予測困難なsibling tempを`O_EXCL`で作成し、全byte write、close、renameの順で置換する。
- short write、create、write、close、renameの失敗を原因付きで返し、rename成功前は旧target bytesを維持する。
- 失敗時は自分が作成したtempだけを削除し、cleanup失敗は主因と`errors.Join`する。既存fileや他processのtempは削除しない。
- rename後にtempが存在しないことを確認し、`WriteInitial`のsidecar先行・write barrier・error契約を変更しない。
- close+renameをatomic visibility境界とし、`fsync`や電源断耐性を新しい保証として主張しない。
- lock、audit、retry/idempotency、state再読込、遷移認可は後続transactionが所有する。

## 対象ファイル

- `src/internal/state/write.go`
- `src/internal/state/write_test.go`
- `src/internal/state/write_integration_test.go`
- `docs/architecture.md`
- `docs/development.md`
- `docs/ram/README.md`
- 本計画

実装担当は上記を単独所有し、他の作業者の変更をrevertしない。

## TDD slice

1. nil Root、malformed replacement、target不在、directory、symlink、FIFOを拒否する。
2. regular targetのwriteability barrier後、sibling tempを排他的に作り、全byte write、close、renameする順序を固定する。
3. temp name collisionを有限回retryし、上限到達時にtargetを変更せず失敗する。
4. short write、write error、close error、rename errorでtargetが旧bytesのままになることと、所有temp cleanupを確認する。
5. cleanup failureを主因とともに保持し、rename成功後はcleanupを呼ばない。
6. 実`os.Root`で、成功時のexact bytes、失敗時の旧bytes、target・temp・Root ownershipをintegration確認する。
7. `WriteInitial`の既存targeted testを回帰として実行する。

loopではstate writerの対象unit/integration testだけを実行する。full、race、vet、cross buildは独立review後のfinalへ集約する。

## 受け入れ条件

- 成功後はreplacement bytesだけが`aidlc-state.md`として読める。
- rename前の失敗では旧state bytesを保持する。
- target不在またはnon-regular、read-only targetを拒否する。
- tempを同一Root内に排他的に作り、所有tempだけをcleanupする。
- caller-owned RootをCloseせず、`WriteInitial`の契約を変えない。
- `fsync`を保証せず、外部Go moduleを追加しない。

## 互換性・リスク

固定AI-DLC 2.6.123で確認したclose+same-directory renameの可視性境界へ準拠し、新しい意図的差分は採用しない。auditとstateをまたぐtransactionではなく、単一state fileの置換だけを保証する。audit-first順序とrecord lockは後続PRで接続する。

## APIと実装記録

公開する内部APIは次のとおりとした。

```go
func WriteState(recordRoot *os.Root, replacement []byte) error
```

`WriteState`はreplacementを先に`Parse`し、既存`aidlc-state.md`のregular判定と非truncate `O_WRONLY`
barrierを通過した場合だけ、同一Root内のsibling temporaryへ全bytesを書き込む。temporaryは
`O_EXCL`で有限回衝突retryし、close後に`Root.Rename`する。rename前の失敗ではtargetを変更せず、writerが
取得したtemporaryだけをcleanupし、cleanup errorは主原因と`errors.Join`する。caller-owned Rootはcloseせず、
`WriteInitial`のsidecar契約には触れない。

TDDではnil root、malformed replacement、barrier順序、target不在・nonregular・read-only、全failure point、
collision budget、cleanup cause、exact bytes、Root ownership、temporary不在を対象unit/integration testで固定した。
外部Go module、filesystem以外の副作用、fsync保証、audit/lock/遷移認可は追加していない。

loop実行結果:

```text
go test -count=1 -run '^(TestWriteState|TestWriteInitial)' ./src/internal/state  # ok
go test -tags=integration -count=1 -run '^(TestWriteState|TestWriteInitial)' ./src/internal/state  # ok
gofmt -l src/internal/state/write.go src/internal/state/write_test.go src/internal/state/write_integration_test.go  # no output
git diff --check  # no output
```

残余riskは、audit/stateをまたぐtransaction、record lock、lost-update防止、fsync・電源断耐性、Windowsでの
rename atomicityをこの単一file writerが保証しないことである。これらは後続transactionまたはfinal gateの責務とする。
