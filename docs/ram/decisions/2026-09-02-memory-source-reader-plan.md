# 4層Memory source readerの実装計画・実施記録

- Issue: [#47「4層のMemory source readerを実装する」](https://github.com/sori883/ai-dd/issues/47)
- 分類: `機能開発`
- 計画承認日: 2026-09-02
- 実装担当: `go_tdd_implementer`（Luna / max）
- verification mode: `loop`
- 状態: Accepted（実装loop・targeted検証完了、独立review待ち）

## 目的と所有範囲

Memory rootにある4層のMarkdownを、後続のresolverやstage consumerが利用できる純粋な
内部APIとして取得する。今回の所有範囲は次のとおり。

- 新規`src/internal/memory/**`
- `.github/workflows/ci.yml`のMemory integration test step
- `docs/architecture.md`のpackage境界とAPI契約
- 本記録、参照契約、`docs/ram/README.md`の索引

workspaceのroot解決・Space選択・CLI接続・filesystem writer・merge/override・frontmatter解析・
stage実行は今回の責務に含めない。外部Go moduleは追加せず、標準ライブラリだけを使う。

## 承認済みAPI

実装packageは`src/internal/memory`とする。

```go
type Layer string

type Source struct {
    Layer Layer
    Path string
    Content string
}

func ReadSources(memoryFS fs.FS, phase string) ([]Source, error)
```

`LayerOrg`、`LayerTeam`、`LayerProject`、`LayerPhase`を公開する。`Path`はMemory root相対の
slash pathとし、結果のsliceはcaller-ownedとする。

## 採用契約

1. 候補は`org.md`、`team.md`、`project.md`、`phases/<phase>.md`の固定順に読む。
2. `phase`は非空かつASCIIの`^[a-z][a-z0-9-]*$`で、known phase enumへは限定しない。不正値はI/O前に`fs.ErrInvalid`をwrapして拒否する。
3. `fs.ErrNotExist`の候補だけをskipし、全欠損はnon-nilの空sliceとする。その他のread errorはpathとcauseを保持し、途中データも返さない。
4. 不正UTF-8は`fs.ErrInvalid`をcauseとするerrorにして結果をnilにする。CRLF、BOM、空内容、frontmatter、空行、末尾改行はそのまま保持する。
5. merge、override、frontmatter parse、substantive判定は行わず、unknown fileのwalkもしない。
6. cacheを持たず、毎回候補をfresh readする。nil FSはpanicせず`fs.ErrInvalid`を返す。
7. 実filesystemのcallerは`memory/`を`os.OpenRoot`で開いた`Root.FS()`を供給し、readerはRootをcloseしない。root内通常file・相対symlinkを許可し、root外・絶対symlinkでは外部bytesを返さずerrorにする。
8. stage固有の第5層は、本家v2.6.123でも予約・未実装のため扱わない。

## 本家根拠と意図的差分

比較対象はローカルAI-DLC v2.6.123で、確認範囲は`docs/実装_aidlc-workflows/core/tools/`の
`aidlc-graph.ts:271-324,500-529,604-655`、`aidlc-steering.ts:85-107`である。graph側の
Memory layout、4層scope名、phase regex、unknown fileを対象外にするwalk、steering側のfresh
read・fatal UTF-8・partial result破棄を根拠にした。最新upstream、全workflow、全配布物との完全な
parityは確認していない。詳細は[参照契約](../research/2026-09-02-memory-source-reader-contracts.md)に記録する。

| 本家の挙動 | 採用した挙動 | 理由 | 利用者・互換性への影響 |
| --- | --- | --- | --- |
| 通常Node filesystemでMemory pathを解決し、root外symlinkを追跡し得る | callerが渡す`os.Root.FS()`を境界とし、root外・絶対symlinkを拒否 | project外の任意file読取を防ぐ | 外向きsymlinkはerrorとなり外部内容を返さない。通常fileとroot内相対symlinkは影響なし |

この差分は2026-09-02のIssue #47計画として承認済みであり、reader自体がsymlinkを判定するのではなく、
境界付きFSを受け取るAPI設計によって適用する。

Go 1.26.0〜1.26.4のGO-2026-4970は末尾slashを伴うroot外leaf symlinkに関する注意である。
固定候補pathには末尾slashがないが、CIと最終検証は修正版Go 1.26.5以降を前提とする。

## TDD実施記録

実装は`source.go`と対応する`source_test.go`を、observable behaviorごとにRED→最小GREENで進めた。
loop中は`./src/internal/memory`だけを対象にし、全体test・race・vet・cross compile・配布E2Eは実行していない。

| slice | RED evidence | GREEN evidence |
| --- | --- | --- |
| fixed order・exact content | `go test -count=1 -run '^TestReadSources' ./src/internal/memory` — `no non-test Go files`でpackage未実装を確認 | 同コマンド — `ok` |
| missing skip・safe phase no-I/O | 同targetで、旧実装がmissingをerrorにしinvalid phaseをI/O後に扱う失敗を確認 | 同コマンド — `ok` |
| read error・partial data・UTF-8・nil FS | 同targetで、旧実装が不正UTF-8を内容として返しtyped nilでpanicする失敗を確認 | 同コマンド — `ok` |
| fresh read・caller-owned slice・候補I/O順 | 追加assertionが旧実装のcache/共有結果を検出する形で作成 | 同コマンド — `ok` |
| `os.Root.FS()` boundary | integration fixtureを追加し、通常file・in-root symlink・outward symlinkを固定 | `go test -tags=integration -count=1 -run '^TestReadSources' ./src/internal/memory` — `ok` |

変更後は次を実行し、unitとintegrationがともにGREENであることを確認した。

```sh
gofmt -w src/internal/memory/source.go src/internal/memory/source_test.go src/internal/memory/source_integration_test.go
go test -count=1 -run '^TestReadSources' ./src/internal/memory
go test -tags=integration -count=1 -run '^TestReadSources' ./src/internal/memory
```

## 残余リスクとレビュー引き継ぎ

- 各layerを個別に読むため、並行更新中の一貫したsnapshotは保証しない。
- `os.Root`はmount、device、特殊filesystem全般を遮断する完全sandboxではない。
- symlink作成権限がない環境では該当integration caseを理由付きskipする。
- Go 1.26.5未満、Node/Bunの全OS path解釈、最新upstreamと全配布物の完全互換は未確認である。
- packageは内部APIのみで、CLI、workspace resolver、stage routing、配布E2Eには未接続である。

この記録時点では独立reviewとparent管理のfinal gateが未実施であり、完了を先取りしない。
