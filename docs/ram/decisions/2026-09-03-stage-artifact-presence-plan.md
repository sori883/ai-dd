# Stage completion artifact presence の実装計画

- 承認日: 2026-09-03（Asia/Tokyo）
- 状態: Accepted（ロードマップ包括承認内）
- 対象Issue: [#69](https://github.com/sori883/ai-dd/issues/69)
- 固定base: `61da013a8f3a62bbc9e8b5ec0eab2bb87630a77e`
- 実装branch: `codex/stage-artifact-presence`
- 検証mode: `loop`

## 目的と利用者が得る結果

通常Stageを完了へ進める処理が、Stage metadataに宣言された必須 `Produces` のうち少なくとも1件が
Intent record内の通常fileとして存在することを、内容を読まずに判定できるようにする。判定は
filesystem、state、audit、approval、lock、clock、CLIを変更せず、後続のStage completion接続が
read-onlyな内部APIを利用できる境界を提供する。

## 実装許可と範囲

この計画は、ユーザー承認済みの
[AI-DLC Go実装ロードマップ](2026-09-03-aidlc-implementation-roadmap.md)の「薄い全life-cycle」内の
成果物確認に完全に収まり、[ロードマップ単位の包括承認と自律マージ](2026-09-03-milestone-authorization-and-autonomous-merge.md)
が残作業を包括承認しているため実装許可がある。固定本家AI-DLC `2.6.123`の確認範囲は
[Stage completion artifact presence の参照契約](../research/2026-09-03-stage-artifact-presence-contracts.md)に記録する。
GitHub Issue #69を対応Issueとし、実装中のIssue・PR操作は親agentが所有する。

実装担当が所有するfileは次のとおりである。

- 新規 `src/internal/artifact/presence.go`
- 新規 `src/internal/artifact/presence_test.go`
- 新規 `docs/ram/research/2026-09-03-stage-artifact-presence-contracts.md`
- 新規 `docs/ram/decisions/2026-09-03-stage-artifact-presence-plan.md`
- 更新 `docs/ram/README.md`
- 更新 `docs/architecture.md`
- 更新 `docs/development.md`

既存の `src/internal/state`、`src/internal/graph`、`src/internal/orchestrator`、CLI、`go.mod`、
state/audit/approval/lock/clockは変更しない。Go標準ライブラリだけを使用し、外部Go module・外部toolは
追加しない。

## APIと挙動

```go
func HasRequiredOutput(recordFS fs.FS, stage graph.Stage) (bool, error)
```

- `Produces` が空なら、zero Stageやnil FSでもfilesystemを参照せず `(true, nil)`。
- 非空なら `Phase`、`Slug`、各required artifact名を `^[a-z][a-z0-9-]*$` で検証する。
- 不正metadataは `ErrInvalidMetadata` を `%w` でwrapしたlowercase context errorとして返し、boolはfalse。全metadata検証をFSアクセス前に行う。
- nil FSは `ErrInvalidFilesystem` をwrapしたinput errorとする。汎用typed-nilのreflect対策は追加しない。
- candidate pathは `path.Join(stage.Phase, stage.Slug, filename)`。既定filenameは `<artifact>.md`。
- `traceability` は `traceability.json`、`build-test-results` と `load-test-results` は `test-results.md`。
- `fs.Stat` が成功し `Mode().IsRegular()` の候補が1件でもあればtrue。個別Stat error、directory、FIFOなどは未存在扱いとする。
- `OptionalProduces`、per-unit、CodeKB、`produces_kinds`、`workspace_requires`、全required instance検査は対象外とする。
- 入力Stageのslice、filesystem、内容は変更・読取りしない。

本家の通常Stage any-of存在確認、path配置、filename例外に意図的な仕様差分は採用しない。per-unit・CodeKB等を
このAPIへ持ち込まないのは段階的実装境界であり、本家との新しい製品挙動差分ではない。

## TDDと検証

observable behaviorを次の順で一つずつRED→最小GREEN→green上のrefactorで実装する。

1. 空 `Produces` はnil FSでも成功する。
2. 通常artifactのcanonical pathでregular fileを認識し、欠損・directory・non-regularはfalseにする。
3. 複数 `Produces` は1件以上で成功し、OptionalProduces単独では成功しない。
4. 3つのfilename例外をtable-driven named subtestで確認する。
5. invalid Phase/Slug/artifactとnil FSをsentinelでfail-closedにし、invalid metadata時FSを読まない。
6. 入力sliceを変更せず、Statだけで内容を読まないことをspy FSで確認する。

`loop`で実行するcommandは次の対象packageだけである。

```sh
go test -count=1 -run '^TestHasRequiredOutput' ./src/internal/artifact
go test -count=1 ./src/internal/artifact
```

変更Go fileへgofmtを適用してから、最後の対象package testをfreshに再実行する。全package test、race、vet、
全体lint、cross compile、配布E2E、commit、push、Issue/PR操作は親agentの責務であり、このloopでは行わない。

## 残余risk

本API単体ではStage完了処理へ未接続であり、callerが適切なrecord-root FSと対象Stageを渡す必要がある。
特殊な配置（per-unit・CodeKB）、workspace source、内容妥当性、audit証跡、state更新の保証は後続Issueの
責務である。`fs.FS`供給元のcontainment、lifecycle、並行変更中のsnapshot一貫性もcaller側に残る。
