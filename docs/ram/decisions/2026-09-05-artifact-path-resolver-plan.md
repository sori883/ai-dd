# 工程の成果物名をIntent record相対pathへ解決する

- 日付: 2026-09-05（Asia/Tokyo）
- 状態: Accepted（知識供給の包括承認内）
- GitHub Issue: [#103](https://github.com/sori883/ai-dd/issues/103)
- 実装許可: [ルール・知識のAI供給を個別承認なしで完了まで進める](2026-09-05-context-delivery-autonomous-authorization.md)
- verification mode: `loop`（独立review後の`final`は親エージェントが1回だけ実施）

## 背景と利用者が得る結果

stage graphの`produces`、`optional_produces`、`consumes`はpathではなく、`intent-statement`のような
成果物語彙名である。AIが工程を実行するときは、前工程が作った入力資料を実際に読めるpathと、今回の
成果物を保存するpathが必要になる。

このIssueでは、通常Stageの語彙名をIntent record root相対の
`<producer-phase>/<producer-slug>/<canonical-filename>`へ変換する純粋な内部APIを追加する。
後続の配信構成がproject相対のrecord prefixを一度だけ前置できるよう、このAPIはSpace、Intent、absolute
project rootを受け取らない。

## 固定本家2.6.123の根拠

比較対象はリポジトリ固定AI-DLC 2.6.123の次の確認範囲であり、最新upstreamとの一致は主張しない。

- `core/tools/aidlc-orchestrate.ts:2310-2540`: artifact名、producer所有directory、consume条件、first producer、
  orphan時の消費Stage fallback、resolved consumeのartifact／required保持。
- 同`:2649-2685`: required outputにoptional outputを続けるproduce一覧。
- `core/tools/aidlc-graph.ts:830-840`: producer検索はrequired／optional outputsをgraph順で走査する。
- `core/tools/aidlc-lib.ts:90-117`: per-unit markerと固定5 Stageの防御的判定。
- 既存Go `src/internal/artifact/presence.go`: record root相対path、artifact名検証、`Filename`例外。

固定catalogでは同名`traceability`を複数Stageがproduceするがconsumeはしない。本家runtimeはconsume時に
最初のproducerを選ぶ。このIssueは複数producerをerrorへ変更せず、同じgraph順first-winsを維持する。
producer 0件でも本家どおり消費Stage自身へfallbackする。新しい意図的な本家差分は追加しない。

## APIと所有範囲

Go実装担当の所有対象は次に限定する。

- `src/internal/artifact/paths.go`（新規）
- `src/internal/artifact/paths_test.go`（新規）
- 必要な場合だけ`src/internal/artifact/presence.go`のprivate validation helperを再利用可能な形へ整理する。
  `HasRequiredOutput`の観測可能な契約は変えない。

親エージェントは計画、RAM索引、architecture、development、Issue、PR、review、finalを所有する。

```go
type Input struct {
    Artifact string
    Path     string
    Required bool
}

type Paths struct {
    Consumes []Input
    Produces []string
}

var ErrUnsupportedPlacement error

func ResolvePaths(stage graph.Stage, catalog graph.Snapshot, projectType string) (Paths, error)
```

## 詳細契約

### outputs

- `stage.Produces`、`stage.OptionalProduces`の順で各sliceの宣言順を維持する。
- 各pathは`path.Join(stage.Phase, stage.Slug, Filename(artifact))`で組み立てる。
- `Filename`を唯一の変換元とし、`traceability.json`と2種類の`test-results.md`例外を共有する。
- duplicate artifactや、異なる語彙が同じfilenameになる場合も除去・並べ替えない。

### inputs

- `stage.Consumes`を宣言順に処理する。
- `catalog.Stages()`をgraph順に走査し、`Produces`または`OptionalProduces`にartifactを含む最初のStageを
  ownerとする。複数producerを補正・sortしない。
- producerが0件なら、固定版runtimeどおり消費Stage自身をownerとする。
- `Input.Artifact`と`Required`はgraphの値を保持し、`Path`だけをowner directoryへ解決する。
- consumeのduplicateも保持する。

### conditional filter

- `projectType`をcase-insensitiveに`brownfield`または`greenfield`と識別できる場合だけ既知値とする。
- 既知project typeと`ConditionalOn`が不一致のconsumeを除外する。
- project typeが空または未知ならfilterせず、条件付きconsumeを含む全候補を保持する。
- `ConditionalOn`の許容値は空、`brownfield`、`greenfield`だけとし、不正metadataはfilter前に拒否する。

### 安全性と段階的境界

- phase、slug、required／optional output、consume artifactを既存kebab-case規則で検証してからpathを返す。
- pathを実際に所有するcurrent／producer Stageがper-unit、CodeKB、`ProducesKinds`のいずれかなら
  `ErrUnsupportedPlacement`でfail-closedにする。
- per-unitは`ForEach != ""`または固定5 Stage slug、CodeKBは固定版で唯一の`reverse-engineering`、
  kind-awareは`ProducesKinds != nil`で判定する。
- unsupported判定は、現在のwalking skeletonが既に工程実行前に止める段階的未実装境界である。
  per-unitのunit segment、Space-level CodeKB、unit kindを推測して通常pathへ落とさない。
- validationまたはunsupported errorでは`Paths{}`を返し、partial結果を公開しない。
- filesystem I/O、存在確認、state/audit/registry変更、record prefix前置、stage手順path、memory pathは扱わない。
- 空入力成功時の`Consumes`と`Produces`は、本家の空arrayに合わせてnon-nil empty sliceとする。
- 入力sliceやcatalog snapshotを変更せず、返却sliceは相互にも入力にも共有しない。

unsafe metadataの早期拒否は、既存Issue #69で承認されたartifact path境界を、path一覧にも適用するもの
である。固定版の正規compiled graphには影響せず、新しい永続形式・公開CLI・外部moduleを追加しない。

## TDD単位

各sliceは別のRED/GREEN依頼とする。最初のREDでは`paths.go`へcompile-only stubを置き、testは最初から
最終APIを呼ぶ。GREENではtestをbyte-for-byte維持してstubだけを最小実装へ変える。後続testが現実装で
通る場合は`ALREADY_GREEN`とし、人工的に不具合を入れない。

1. `TestResolvePathsProducesCanonicalPathsInDeclarationOrder`
   - required→optional順、通常`.md`、3つのfilename例外、duplicate保持、empty non-nilを固定する。
2. `TestResolvePathsConsumesFromFirstProducerOrConsumerFallback`
   - required／optional producer、graph順first-wins、orphan fallback、artifact／required／順序を固定する。
3. `TestResolvePathsFiltersConditionalConsumesForKnownProjectType`
   - Brownfield／Greenfieldのcase-insensitive filterと、空／未知project typeのno-filterを固定する。
4. `TestResolvePathsRejectsInvalidMetadataOrUnsupportedPlacement`
   - current／selected producerのunsafe component、不正condition、per-unit、固定known per-unit slug、CodeKB、
     non-nil`ProducesKinds`を拒否し、zero Pathsを返す。
5. `TestResolvePathsOwnsResultsAndInputs`
   - 入力stage／catalog由来sliceを不変に保ち、返却Consumes／Producesの変更が相互・次回結果へ影響しない。

targeted commandは各test名をexactに指定する。

```sh
GOTOOLCHAIN=go1.26.8 go test -count=1 -run '^TestResolvePathsProducesCanonicalPathsInDeclarationOrder$' ./src/internal/artifact
GOTOOLCHAIN=go1.26.8 go test -count=1 -run '^TestResolvePathsConsumesFromFirstProducerOrConsumerFallback$' ./src/internal/artifact
GOTOOLCHAIN=go1.26.8 go test -count=1 -run '^TestResolvePathsFiltersConditionalConsumesForKnownProjectType$' ./src/internal/artifact
GOTOOLCHAIN=go1.26.8 go test -count=1 -run '^TestResolvePathsRejectsInvalidMetadataOrUnsupportedPlacement$' ./src/internal/artifact
GOTOOLCHAIN=go1.26.8 go test -count=1 -run '^TestResolvePathsOwnsResultsAndInputs$' ./src/internal/artifact
```

test fixtureは標準ライブラリの`testing/fstest`と`encoding/json`で`graph.Load`へ渡し、外部moduleを加えない。
loopでは全package、race、vet、cross compileを実行しない。各Go変更後のgofmtと対象test再実行だけを行う。

## reviewとfinal gate

固定base/headの独立reviewで、Issue範囲、本家first-wins／fallback、Filename共有、unsupported fail-closed、
path安全性、condition、順序・所有権、回帰testを確認する。blocking findingを解消して差分が安定した後、
親がread-only finalを1回だけ実施する。

- 全package test、race、integration tag付きtest/race。
- 通常・integrationのvet、`go mod tidy -diff`、`gofmt -l src`、対象fileの`gopls check`。
- `git diff --check`。
- CLIとartifact test binaryをdarwin/linux/windows × amd64/arm64の6構成へcross compileする。

cross compileは各OS上の実行証拠とはしない。final後に対象fileが変われば証拠はstaleとし、loopと再review後にfinalをやり直す。

## 実装記録

- `artifact-produce-paths`: compile-only stubに対してREDを確認後、required→optional順、canonical filename、
  duplicate保持、empty non-nilをGREENにした。
- `artifact-consume-paths`: 空consume結果のREDを確認後、graph順first producer、optional producer、
  orphanのconsumer fallback、`Required`と順序の保持をGREENにした。
- `artifact-conditional-consumes`: 既知project typeで全候補が残るREDを確認後、case-insensitiveな
  Brownfield／Greenfield filterと空・未知値のno-filterをGREENにした。
- `artifact-invalid-unsupported`: 15件のinvalid／unsupported caseがerrorなしまたはpartial pathを返すREDを確認後、
  current／選択producerの事前検証とerror時zero `Paths{}`をGREENにした。
- `artifact-result-ownership`: current Stage、catalog、1回目と2回目の返却値の独立性testを追加し、
  production変更なしの`ALREADY_GREEN`として受理した。
- 独立reviewで、`ConditionalOn`までcase-insensitiveに受理していた計画違反を検出した。mixed-case metadataで
  REDを固定し、生値をexactな空・`brownfield`・`greenfield`だけに制限してGREENにした。project typeの
  case-insensitive比較は維持した。

各GREEN／`ALREADY_GREEN`後に親エージェントがexact targeted commandと対象file hashを再確認した。
