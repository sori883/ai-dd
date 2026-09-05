# 配置済み情報からcanonical run-stageをread-only構成する

- 日付: 2026-09-05（Asia/Tokyo）
- 状態: Implemented（知識供給の包括承認内、Issue #109）
- Issue: [#109](https://github.com/sori883/ai-dd/issues/109)
- 実装許可: [ルール・知識のAI供給を個別承認なしで完了まで進める](2026-09-05-context-delivery-autonomous-authorization.md)
- 基準: リポジトリ固定AI-DLC 2.6.123

## 背景と目的

Go版には、現在工程を選ぶread-onlyな`orchestrator.Next`、配置Markdownから必須ruleを読む処理、
工程・担当AI別のpersona／knowledge一覧、artifact path解決、rule本文のchunk、継続token codecとprivate keyがある。
しかし、これらはまだ「今回AIへ渡す1回分の工程指示」へ接続されていない。

本Issueでは、利用projectへ配置した`.codex/`と`aidlc/spaces/<space>/`を呼出しごとに読み、
本家の`run-stage`に必要な工程、担当agent、knowledge path、工程手順path、input／output artifact path、
必須rule pathを正規JSONへ構成する。同時に、rule本文のchunkと、配信途中の変更を検出するhashを返す。

利用者は、binaryへ埋め込まれた古い情報ではなく、現在選択中のSpaceとIntentに対応する配置Markdownを
次回呼出しから使える。必須情報が欠けた場合や、現在のGo実装で安全に意味を確定できない場合は、
実行可能な指示を返さず停止する。

## 実装許可と維持する境界

ユーザーは「組織やprojectのrule、工程手順、必要資料を選び、AIに渡す情報を組み立てる」既存計画の
全範囲を、人間の個別承認なしで進めることを明示した。本計画はそのread-only composition部分であり、
追加承認を待たずIssue、実装、review、PR、品質gate後のmergeまで進める。

次の境界は変更しない。

- runtimeは利用先の`.codex/`と`aidlc/spaces/`だけを読む。開発用`src/core/`を直接読まず、
  binary埋込み、fallback、永続cacheを追加しない。
- 配置MarkdownをOKFへ変換しない。knowledgeのUTF-16 code-unit固定順と既存容量上限を維持する。
- 既存`orchestrator.Next`のread-only、record binding、未対応工程のfail-closedを迂回しない。
- knowledgeを読めることを、人間承認、工程完了、review／sensor実行の証拠にしない。
- Go標準libraryだけを使い、外部module、追加権限、認証情報、永続schemaを導入しない。
- 複数Markdownを同時編集した場合の原子的snapshotは保証しない。継続時のfresh再構成で古い配信を拒否する。

新しい意図的な本家差分は採用しない。Goの型やpackage分割、まだ未実装の公開配信は段階的移植の内部差分である。

## 確認済みの本家契約

固定AI-DLC 2.6.123の次を根拠とする。

- `core/tools/aidlc-orchestrate.ts:3152-3320`: `run-stage`の基礎field、warning、欠落consume、
  next stage、protocol、conductor persona、narrationの構成。
- 同`:3614-3728`: token binding、`sha256(JSON.stringify({node, scopeStages}))`のroute hash、
  fresh rule bundle、directive hash、chunk transport。
- 同`:8343-8458`: continuation時のstate、Stage、routeのfresh再確認。
- `tests/unit/t248-steering-content-delivery.test.ts:626-697,752-1084`: rule／state／route変更時の拒否、
  active Space、knowledge path、warningと容量。
- `tests/unit/t116-directive-path-resolution.test.ts`: artifact pathとpresent／absent、実行mode別roster。
- 配置Codex `.agents/skills/aidlc/SKILL.md`: rule全体の受領後にknowledge、stage file、consumeを読む順序。

固定配置の`mvp` scope、`intent-capture` stageから本家が計算するroute hashは
`b2b7deca926d64c0e55225db06e10e202c06ac6f0c26f759070f825146525d23`である。
実装中に固定snapshotから独立再計算してgoldenへ使用する。

## 実装範囲とfile所有権

単独implementerが次を所有する。

- `src/internal/graph/graph.go`: 読込んだenabled nodeのroute用正規JSONを非公開保持する。
- `src/internal/graph/route.go`, `route_test.go`: 本家式route hash。
- `src/internal/delivery/run_stage.go`, `run_stage_test.go`: fresh sourceの選択とread-only composition。
- `src/internal/delivery/wire.go`, `wire_test.go`: canonical `run-stage` wireとhash。
- `src/internal/delivery/*_integration_test.go`: 利用先配置、編集反映、非変更のnative filesystem検証。

親agentは本RAMと索引、Issue、独立review、final、PR、merge gateを所有する。implementerはIssue、PR、
本RAM、利用者所有の未追跡fileを変更しない。

## 内部API

中心入口は次の形とする。具体的な補助型名はGoの明瞭さを保つ範囲で調整できる。

```go
type RunStageInput struct {
    Identity       recordlock.Identity
    ProjectRoot    *os.Root
    RecordRoot     *os.Root
    EnabledPlugins []string
}

type RunStageComposition struct {
    Directive     RunStageDirective
    Wire          []byte
    Rules         []steering.RuleContent
    Chunks        [][]steering.RuleContent
    Bundle        string
    DirectiveHash string
    RouteHash     string
    StateHash     string
    Claims        *steering.ContinuationClaims
    Freshness     steering.ContinuationFreshness
}

func ComposeRunStage(context.Context, RunStageInput) (RunStageComposition, error)
```

`Claims`はrule chunkがある場合だけ非nilとし、最初の`load-steering`を受領した後に要求するpartを`NextPart=1`
で表す。このPRはkeyを読まずtokenを署名せず、active-directive cursorを作成・更新しない。

`RunStageComposition`内のbyte列、slice、pointerはcaller所有のcopyとし、入力や別の返却値を変更しない。

graph側は次を追加する。

```go
func (s Snapshot) RouteHash(stageSlug, scope string) (string, error)
```

`graph.Load`時に、各enabled nodeを簡略化した公開`graph.Stage`へ変換する前のJSONを、property順を保った
compact JSONとして非公開保持する。`RouteHash`はこのnodeと、scope-gridで`EXECUTE`になったenabled stageを
stage number順に並べたslug列を、`{"node":...,"scopeStages":[...]}`の順で正規化し、bare lowercase
SHA-256 hexを返す。現在の`graph.Stage`を再JSON化してはならない。そうすると`condition`、`inputs`、
`outputs`、review詳細、`sensors_applicable`等がhashから落ちるためである。

## fresh readの順序

`ComposeRunStage`は呼出しごとに次を行う。

1. `ProjectRoot.FS()`から`aidlc/active-space`とSpace内の`active-intent`を読み、
   `Identity.Space()`／`Identity.Intent()`と一致することを確認する。
2. `.codex/tools/data/stage-graph.json`と`scope-grid.json`をfreshに`graph.Load`する。
3. そのcatalogで既存`orchestrator.Next`を呼び、record bindingと実行能力を検証した`run-stage`だけを受理する。
4. `Next.Content`のDepth、stateのScope、Project Type、Next Stageを使用する。
5. `RecordRoot.FS()`でresolved consumeの存在を確認する。
6. `.codex/`と`aidlc/spaces/<active>/knowledge/`からknowledge rosterをfreshに作る。
7. `aidlc/spaces/<active>/memory/`から必須rule本文を宣言順でfreshに読み、bundle digestとchunksを作る。
8. directive wire、directive hash、route hash、state hash、claims／freshnessを構成する。

Rootはcaller所有であり閉じない。内部で開いたfile／sub-rootは必ず閉じ、Close errorを捨てない。
任意knowledgeの欠落・読込不能は既存のbounded warningとし、必須ruleの欠落・読込不能・不正UTF-8は
zero compositionとerrorにする。

## canonical run-stage wire

対応済みsubsetでは次のproperty順を固定する。

1. `kind`
2. `stage`
3. `phase`
4. `lead_agent`
5. `support_agents`
6. `mode`
7. `inline_context_paths`
8. `gate`
9. `memory_path`
10. `consumes`
11. `produces`
12. `rules_in_context`
13. `sensors_applicable`
14. `stage_file`
15. `context_warnings`（非空時）
16. `consumes_absent`（非空時）
17. `next_stage`
18. `protocol_modules`（非空時）
19. `conductor_persona`（最初の実質工程でread成功時）
20. `narration`（本家から一意に導出できる対応mode）

必須の空配列は`null`ではなく`[]`とする。文字列は`JSON.stringify`相当のUTF-8、control character escapeを使い、
Goのmap順やHTML escapeへ依存しない。全wireが28 KiBを超えたらzero compositionとerrorを返す。

- `stage_file`: `.codex/aidlc-common/stages/<phase>/<slug>.md`
- `memory_path`: `aidlc/spaces/<space>/intents/<intent>/<phase>/<slug>/memory.md`
- artifact: `aidlc/spaces/<space>/intents/<intent>/`へ`artifact.ResolvePaths`のrecord相対pathを前置する。
- `next_stage`: 次の実行対象stageの表示名。終端はJSON `null`。
- `gate`: 現在`orchestrator.Next`が通す実質工程では`true`。未対応のinitialization／constructionを
  composition側で独自に有効化しない。
- `protocol_modules`: 対応済みのsubagent、mob、support agentを含む工程では`ensemble`を付ける。

stage Markdown、knowledge本文、input artifact本文はこのwireへ埋め込まずpathを渡す。最終receiverが配置fileを
実際に読むため、本文変更は次回読込へ反映する。必須rule本文だけは、裁量読込へ格下げしないため、
先行する`load-steering`用chunksとしてcompositionに保持する。

optionalなreviewer、pipeline、construction、unit、swarm、legacy planning fieldは既存`Next`が未対応能力を
先に拒否するため、本PRから実行可能にしない。conductor personaやnarrationを本家から一意に構成できない場合も、
別の意味を推測せずoptional fieldを省略し、後続receiver接続時に固定根拠を追加する。

## artifact present／absent

`artifact.ResolvePaths`のrecord相対結果を次に分類する。

- 存在するconsumeはproject相対pathを`consumes`へ入れる。
- 欠落したoptional consumeは`consumes`にも`consumes_absent`にも入れない。
- 欠落したrequired consumeにactive scope上のproducerがなければ、
  `consumes_absent`へ`{"path":...,"expected":true}`を入れる。
- active scope上のproducerが実行対象でskipされていなければ`expected:false`とする。
- active scope上のproducerがstateでskippedでも、現Goのauditから`conditional-runtime` skipを証明できない場合は、
  `ErrUnsupportedConsumeProvenance`で停止する。`true`／`false`を推測しない。

存在確認はRoot内に限定し、outward symlinkを入力として受理しない。directory等を読めるartifact fileとして扱わない。

## freshnessとhash

- rule本文変更: `BundleDigest`が変わる。
- rule採否、knowledge path／readability／warning変更: directive hashが変わる。
- knowledge本文だけの変更: pathは同じなのでdirective hashは変えず、receiverが最新本文を読む。
- stage Markdown本文変更: pathだけを渡し、receiverが最新本文を読む。route hash対象にはしない。
- graph nodeまたはscope-grid変更: route hashが変わる。
- stateのどのbyteの変更でもstate hashが変わる。

directive／route／state hashはbare lowercase SHA-256 hex、rule bundleだけ既存の`sha256:<hex>`形式とする。
`ContinuationFreshness`にはStage、Scope、Bundle、DirectiveHash、RouteHash、StateHashを全て入れる。
claimsには現在のgate、next stage、state-aware等の固定本家fieldを入れ、後続facadeが同じcompositionから署名する。

## TDDの由来単位

各behaviorは同一implementerへRED-only、親のexact再実行・hash確認、GREEN-onlyの別依頼として渡す。
compile failureだけをREDとして受け入れない。先行実装で通る項目は`ALREADY_GREEN`とし人工的なfailureを作らない。

1. `route-raw-node`
   - lost fieldまたはscope stage変更でhashが変わり、固定goldenと一致する。
   - `go test -count=1 -run '^TestSnapshotRouteHashBindsCanonicalNodeAndScopeStages$' ./src/internal/graph`
2. `route-invalid`
   - unknown stage／scope、正規化不能なnodeを拒否する。
   - `go test -count=1 -run '^TestSnapshotRouteHashRejectsUnknownRoute$' ./src/internal/graph`
3. `compose-selection-next`
   - active Space／Intentをfresh確認し、既存`Next`が検証したrun-stageだけを受理する。
   - `go test -count=1 -run '^TestComposeRunStageUsesFreshSelectionAndValidatedNext$' ./src/internal/delivery`
4. `compose-core-wire`
   - 必須field、stage／memory／artifact path、gate、next stageをcanonical順で構成する。
   - `go test -count=1 -run '^TestComposeRunStageBuildsCanonicalRequiredWire$' ./src/internal/delivery`
5. `compose-artifact-presence`
   - present、optional欠落、required expected true／falseを分類する。
   - `go test -count=1 -run '^TestComposeRunStageClassifiesArtifactPresence$' ./src/internal/delivery`
6. `compose-artifact-ambiguous`
   - provenance不明のskipped producerをfail closedにする。
   - `go test -count=1 -run '^TestComposeRunStageRejectsAmbiguousSkippedProducer$' ./src/internal/delivery`
7. `compose-rules`
   - active Spaceの必須ruleを順序どおり読み、bundle、chunksを作り、欠落・不正UTF-8を拒否する。
   - `go test -count=1 -run '^TestComposeRunStageBuildsFreshRequiredRuleBundle$' ./src/internal/delivery`
8. `compose-knowledge`
   - execution mode、Depth、plugin selectionを使ったrosterとwarningをfreshに構成する。
   - `go test -count=1 -run '^TestComposeRunStageBuildsFreshKnowledgeRoster$' ./src/internal/delivery`
9. `compose-hashes-budget-ownership`
   - canonical wire hash、route／state hash、28 KiB境界、deep ownershipを検証する。
   - `go test -count=1 -run '^TestComposeRunStageBindsCanonicalHashesAndOwnsResult$' ./src/internal/delivery`
10. `compose-freshness-integration`
    - tempの利用先配置でrule、knowledge、graph、stateを個別変更し、再構成と非変更を確認する。
    - `go test -tags=integration -count=1 -run '^TestComposeRunStageIntegrationFreshness$' ./src/internal/delivery`

loopでは各exact testと変更Go fileの`gofmt`だけを実行する。全package、race、vet、cross compileを繰り返さない。

## review、final、受け入れ条件

固定baseと実装headの独立reviewで、fixed route golden、raw node保持、property／stage順、fresh installed source、
selectionとrecord binding、artifact分類、rule fatal／knowledge warning、canonical wire、hash、28 KiB、deep ownership、
Root close／非変更、unsupported能力非混入を確認する。

blocking finding解消後、親が固定headでread-only finalを1回だけ実施する。

- 通常／integrationの全package test、race、vet。
- `go mod tidy -diff`、`gofmt -l src`、`gopls check`、`git diff --check`。
- CLIと該当test binaryをdarwin／linux／windows × amd64／arm64へcross compile。

受け入れ条件は次のとおりである。

- fixed 2.6.123 route goldenと一致し、lossyな`graph.Stage`再JSON化を使わない。
- supported catalogで、最新の配置rule／knowledge／artifact状態からcanonical wire、chunks、全freshnessを返す。
- 必須rule欠落・不正UTF-8、selection不一致、曖昧なrequired consume、28 KiB超過はzero resultとerrorになる。
- fixed catalogの未対応能力は既存`orchestrator.Next`で停止し、実行可能に見せない。
- composer前後でstate、audit、registry、active cursor、key、directive cursor、Markdownを変更しない。

## 後続Issueへ残す境界

次の配信facade／receiver Issueは本compositionを再利用し、private keyによる初回token署名、
first `load-steering`、継続ごとのfresh再構成、active-directive cursor transaction、same-token exactly-once、
全chunk後の`run-stage` publication、Codexによるrule全体保持後のknowledge／stage／consume実読込を接続する。

本PRはpublic `next`／`continue`、key作成、cursor、publication、receiver receipt、stage実行、state mutation、
人間approval/reportを変更せず、供給全体の完成とは報告しない。

## 停止条件

- fixed goldenと正規化結果が一致せず、新しい互換性判断が必要になる。
- conditional skip provenanceを推測しなければdirectiveを構成できない。
- 既存`Next`のunsupported検査を迂回する必要がある。
- 外部module、追加権限、新しい永続schema、新しい意図的差分が必要になる。

通常のtest failure、実装bug、review findingは包括承認内で範囲を広げず修正する。

## 実装記録

2026-09-05に、計画したsliceを1振る舞いずつRED／GREENで実装した。固定AI-DLC 2.6.123のraw graph nodeと
scope stage列からroute hashを作り、`delivery.ComposeRunStage`でfresh selection、既存`orchestrator.Next`、
artifact分類、配置rule本文、knowledge roster、任意presentation、canonical wire、28 KiB上限、
directive／route／state hash、deep ownershipを接続した。外部Go moduleと新しい意図的差分は追加していない。

最後のfilesystem freshness testは、テスト追加前のproduction実装ですでに成功したため`ALREADY_GREEN`として
受理した。同じcaller Rootを使ってrule本文、knowledge一覧、raw graph、valid state bytesを順に変更し、次回の
構成結果へ反映されること、構成前後で対象入力の内容とmodeが変わらないこと、Rootがcloseされないことを確認した。

この記録はcomposer内部APIの実装完了を示す。公開配信facade、継続tokenの一回限り消費、全rule受領後の
`run-stage`公開、Codex receiverの実読込は、上記「後続Issueへ残す境界」に従い次のIssueで接続する。
