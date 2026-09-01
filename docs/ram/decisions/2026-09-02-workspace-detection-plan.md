# 読み取り専用ワークスペース分析の実装計画

- 日付: 2026-09-02
- 状態: Accepted（実装前）
- GitHub Issue: [#33](https://github.com/sori883/ai-dd/issues/33)
- base: `33e78a54b2a92f89ce6802178d44431974f23032`
- 作業branch: `codex/workspace-detection`
- 関連: [参照契約](../research/2026-09-02-workspace-detection-contracts.md)、
  [Intent作成coreの計画](2026-09-01-intent-create-core-plan.md)、
  [初期実装の境界](2026-08-29-initial-implementation-boundaries.md)

## 承認と境界

Intent作成coreの次は、本家Stage 0.2相当の読み取り専用ワークスペース分析を独立sliceとして作る方針を
提示した。ユーザーは、将来ステージ内容を変更する可能性があるため今回はその中身へ触れないことを確認し、
「まずは実行して良い」と2026-09-02に明示承認した。

今回変更するのはdirectory treeから分析結果を算出する内部API、test、説明文書だけである。stage graph、
stage definition、scope、state、audit、Intent作成core、CLIは変更しない。外部Go moduleを追加しない。

## 内部API

既に開かれ、callerがlifecycleを所有する`*os.Root`を受け取る。scanner自身へproject rootの選択、open、
close、存在errorという別の契約を持ち込まない。

```go
type Submodule struct {
    Name        string
    Path        string
    URL         string
    Initialized bool
}

type ScanResult struct {
    ProjectType string
    Languages   string
    Frameworks  string
    BuildSystem string
    NestedRoot  string
    Submodules  []Submodule
}

func Detect(projectRoot *os.Root) ScanResult
```

空の`NestedRoot`は本家のfield不存在、non-nilの空`Submodules`は本家の空配列に対応する。JSONやCLIの
公開形式はこのsliceで決めない。scannerはread-onlyで、内部read・parse失敗をsignalなしとして扱う。

## 実装設計

`src/internal/workspace/workspace_scan.go`へroot-scopedなread operationsと検出処理を置く。
directory走査は`Root.Open`と`File.ReadDir(-1)`でnative順を保ち、各entryを`Root.Lstat`してsymlinkを
追わない。nested containerだけJavaScript UTF-16順でsortし、pathは`/`で組み立てる。

language countはmapに加えて初回観測順を保持し、count降順のstable sortを行う。新しいlexical tie-breakは
追加しない。frameworkは固定順、build systemは本家の優先順位、nested mergeは最初のnon-Unknownを使う。

`.gitmodules` parserはprivateにし、表示用pathをnormalizeせず、安全検証後の原文順で返す。
package.jsonはJSON boundaryとしてplain objectとweakly typed fieldを扱い、JavaScriptのObject.keys、
object spread、truthinessに必要な範囲を標準ライブラリで再現する。

既存`os.Root`方針によりroot外・absolute symlinkを拒否する。本家の通常filesystem APIが追従し得る点との
差は、これまで承認されたworkspace安全境界を継承するものであり、scannerから新しい緩和経路を作らない。

## Assertion-first TDD

実装writerは`go_tdd_implementer`の1名に限定し、次を1 behaviorずつRED、最小GREEN、GREEN上のrefactorで
進める。

1. 空root、root source・manifest・空source directoryによるGreenfield / Brownfield。
2. 全extension、index 0の隠しfile、source depth上限、symlink、primaryと20%閾値、初回観測tie。
3. dependencies、devDependencies、peer React、weakly typed field、invalid package JSON。
4. framework markerと固定出力順、build manifestとlockfileの優先順位、read失敗fallback。
5. root優先、nested深さ3/4、複数hitのUTF-16順、除外、hit後非descend、重複count防止。
6. `.gitmodules`の複数entry、comment、未知key、optional URL、部分parse、unsafe path、宣言順。
7. initialized・uninitialized submodule、submoduleだけのBrownfield、nested後評価。
8. injected read・lstat・readFile failureの吸収と、実`os.Root`でのroot外link拒否・tree無変更。
9. 本家known fixture相当の`Brownfield / TypeScript / Vite, React / npm (package.json)`。

test fileはsourceに対応する`workspace_scan_test.go`と、build tag付き
`workspace_scan_integration_test.go`へ分ける。testは標準`testing`だけを使う。

## 所有権と文書

- `go_tdd_implementer`: `src/internal/workspace/workspace_scan.go`、対応unit / integration test、
  `docs/architecture.md`、`docs/development.md`
- 親エージェント: 本RAM、GitHub、commit、固定base/headの独立review、PR、最終証跡
- `independent_reviewer`: 固定diffのread-only review

同時に複数writerを動かさない。承認範囲外のpackage分割、stage・state・audit・CLI変更、外部moduleが必要に
なった場合は停止する。

## 検証と完了gate

```sh
go test -count=1 -run '^(TestDetect|TestParseGitmodules)' ./src/internal/workspace
go test -tags=integration -count=1 -run '^TestDetect' ./src/internal/workspace
go test -count=1 -shuffle=on ./...
go test -count=1 -race -shuffle=on ./...
go test -tags=integration -race -shuffle=on ./...
go vet ./...
go vet -tags=integration ./...
go mod tidy -diff
gofmt -l src
git diff --check
```

公開CLIを変更しないため配布binary E2Eとは扱わない。実filesystem integrationでread-only、symlink境界、
known resultを検証する。darwin、linux、windowsのamd64/arm64でworkspace test binaryをcross compileし、
native実行証拠とは区別する。

固定base/headの独立reviewでP0/P1、受入条件違反、blockingなtest不足がなく、最終検証とGitHub checksが
成功してからIssue #33へ紐づく日本語PRを作る。自動マージしない。
