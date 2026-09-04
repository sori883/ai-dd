# 工程の必須ルール参照を配置ファイルへ解決する実装計画

- 日付: 2026-09-04
- 状態: Accepted（配置Markdownによる知識供給マイルストーン内）
- Issue: [#91](https://github.com/sori883/ai-dd/issues/91)
- 基点: `93512c8c41877fbaf0bfc8475e9bad99fe9b8da6`（PR #90）
- writer: `go_tdd_implementer` 1名。親がIssue・review・final・PR・mergeを管理する。

## 目的

工程ごとの必須ルール一覧を、選択中の作業領域（Space）に配置されたMarkdownへ結び付けます。後続の配信処理が、工程で指定された順序どおりに最新本文を取得できる内部接続を作ります。

## 現状と背景

PR #90の `steering.ReadRules` は、渡されたファイル読込先（FS）とパス一覧から必須Markdownを毎回読み込めます。一方、工程定義（stage graph）の `rules_in_context` を保持し、どのSpaceのファイルを読むかを解決する接続がありません。
固定参照のAI-DLC 2.6.123では、工程定義の一覧にある最初の `/memory/` より後をactive SpaceのMemoryへ読み替え、明示されたルール配置先があればそちらを読みます。Memoryは組織・チーム・プロジェクト・工程区分のルールを置く場所です。

## 利用者への効果

工程の指定と実際の配置先を分離でき、同じ配置ファイルを編集すれば次の読込みに反映されます。表示にはactive Spaceのパスを使い、別途指定した物理配置先を混ぜません。このIssueは内部接続の完成を扱い、公開CLIやCodexへの配信完了を意味しません。

## 対象

- `graph.Stage.RulesInContext []Rule` を追加し、`path`・`scope`の順序と重複を保持する。省略はnil、明示した空配列はnon-nil空配列。宣言時のnull、不正型、欠損・空field、不正scopeを拒否する。
- `scope`はorg/team/project/phase。exact lowercase keyだけを解釈し、unknown fieldは既存同様に無視する。phaseからの補完や並べ替えはしない。
- graph、stageplan、orchestratorのStage返却経路3箇所で追加sliceを複製し、呼出側の変更を内部に持ち込ませない。
- `steering.ResolveRulePaths` を純粋関数として追加。絶対projectDir、activeSpace、任意rulesDir、参照一覧を受け取り、表示パス・FS相対読込パス・Project/Memoryの区分とMemory絶対directoryを返す。
- rulesDir空はproject/aidlc/spaces/activeSpace/memory、相対指定はproject基準、絶対指定はそのdirectory。trim・環境変数読取・cwd参照・fallback・Root開閉をしない。
- 最初の `/memory/` がある参照をMemoryへ読み替え、ない参照はproject相対として保持。exact default prefixだけに制限しない。
- `ReadResolvedRules` は全Entry検証→表示パスfirst-wins→一意Entryで実際に使うFSだけnil検査→既存reader委譲、の順序を守る。重複で捨てるEntryも構造検証し、未使用FSはnilを許す。
- templateだけの文書が除外されても各本文の表示パスを正しく保持。読取失敗は表示パスと原因を残して結果全体をnilにする。
- 所有対象: graphのコード/test、steering/resolve*.go、stageplanとorchestratorのcloneと所有権test、architecture/development、計画RAMと索引。

## 受け入れ条件

- [ ] rules_in_contextの省略・空・順序・重複・exact keyと不正値を回帰testで確認する。
- [ ] 3経路のStage返却値を変更しても、内部および後続返却値のルール一覧を変更しない。
- [ ] active Space、相対/絶対override、最初のmemory marker、非memory参照を期待する表示/読込先へ解決する。
- [ ] 不正パス・Entry・実際に必要なnil/typed-nil FSをI/O前に拒否する。空EntryはFS不要・I/Oなし・non-nil空結果。
- [ ] first-wins、未使用FS、template除外後の表示対応、必須欠落時の部分結果破棄を確認する。
- [ ] 実際の同じRootでMarkdownを変更すると次の読込みで反映され、Rootを閉じず、root外symlinkから本文を返さない。
- [ ] 原稿140件と既存Memory・CLI・state/audit・Nextのread-only契約を維持し、独立review、final、現在headのCIに成功する。

## 本家AI-DLCとの意図的な差分

新規の意図的変更なし。比較対象はリポジトリ固定2.6.123の `core/tools/aidlc-graph.ts:110-116,176-183,604-697` と `aidlc-steering.ts:55-115`。既存Goの構造検証・所有権・root境界を継承します。最新upstreamや全工程との完全一致は主張しません。

## 検証

単独writerがGo 1.26.8でRED→GREENを観測し、loopではgraphのルールmetadata、3経路の所有権、steering resolver/readerのtargeted testだけを実行します。gofmt適用はreview前に終えます。
固定base/headの独立review後、親がread-only finalを実施します。全package test、race、integration付きrace、通常/integration vet、gofmt -l、go mod tidy -diff、git diff --check、対象gopls診断、6構成（darwin/linux/windows × amd64/arm64）のCLIとsteering integration test binaryのcross compile、原稿hashを確認します。内部APIのため公開CLI配布E2Eは非該当です。

## 依存関係・阻害要因

PR #90はmainへmerge済み（基点 `93512c8`）。公開接続で即時編集対象をsrc/coreと利用先配置のどちらにするか、knowledgeの文字順をどうするかはユーザー確認待ちですが、このIssueはrootと一覧を注入するためどちらも決定しません。

## 特記事項

実FSのRoot開閉・正しい読込先の供給はcallerが担当します。project参照とMemory参照へ別々の `os.Root.FS()` を貸し、readerは閉じません。複数ファイル同時編集の原子的snapshotや特殊filesystemを含む完全sandboxは保証しません。埋込み・cache・外部moduleを追加しません。無関係な `docs/implementation-overview.html` を変更しません。

## 実装許可の根拠

承認済み「配置Markdownからルールと知識を供給する」マイルストーンの第1段階内です。ユーザーは必須ルール本文→工程別知識→Codexでの実読込を承認し、供給は配置ファイルのみ・バイナリ埋込み禁止・Markdown編集の次回反映を明示しました。内部の参照保持と配置先解決はこの承認を実現する通常の実装詳細です。
`docs/ram/decisions/2026-09-04-file-based-knowledge-delivery.md` と詳細計画 `docs/ram/decisions/2026-09-04-stage-rule-path-resolution-plan.md` を根拠とします。既存Nextのread-only、人間承認、未対応工程のfail-closed境界は変更しません。新しい公開配置・知識順・永続形式・権限の判断が必要なら確認gateへ戻ります。

## 内部APIの詳細

```go
type Rule struct {
    Path  string
    Scope string
}
// graph.Stageへ RulesInContext []Rule を追加。

type RuleSource uint8
const (
    RuleSourceUnknown RuleSource = iota
    RuleSourceProject
    RuleSourceMemory
)
type RulePath struct {
    Path     string
    ReadPath string
    Source   RuleSource
}
type RulePaths struct {
    MemoryDir string
    Entries   []RulePath
}
func ResolveRulePaths(projectDir, activeSpace, rulesDir string, refs []graph.Rule) (RulePaths, error)
func ReadResolvedRules(projectFS, memoryFS fs.FS, entries []RulePath) ([]RuleContent, error)
```

APIは `src/internal` 内だけで使用する。`MemoryDir` はnative形式の絶対directory、
`Path` と `ReadPath` はslash形式とする。戻り値のsliceはcallerが所有し、入力sliceを共有しない。

パス検証はJoin前に行う。projectDirの絶対性、activeSpaceの単一component、
`fs.ValidPath` と既存 `validateRulePath`、native localityを確認する。
不正UTF-8、`.`、`..`、絶対参照、backslash、その他FS相対として安全でない経路を
正規化で隠して受理しない（これはgraph由来の参照pathと生成Entryの`ReadPath`に適用する）。
生成Entryの表示`Path`とactive Spaceはnative localityで検証し、POSIX literal backslashを保持する。
`rulesDir`は明示された配置rootの指定なので、相対指定は
project基準で解決し、絶対指定をproject内へ勝手に制限しない。
Windowsではdrive-relative（`C:rules`）やdriveなしroot-relative（`\rules`／`/rules`）を
project基準へ誤解釈せず拒否し、native absoluteまたは明確なproject-relative指定だけを受け付ける。

`ResolveRulePaths` は全参照の元pathも検証し、最初の `/memory/` の後続部分を
Memoryの `ReadPath` にする。表示先は必ず `aidlc/spaces/<activeSpace>/memory/<subpath>`。
markerなしならProjectの `ReadPath` と表示pathは元の参照pathのまま。
scopeはgraphで検証・保持するが、resolverの並べ替えや追加選択には使わない。

`ReadResolvedRules` は不正な重複EntryもI/O前に拒否する。構造検証が全件終わってから
表示Pathの最初のEntryを選び、その一意Entry群が使うFSだけを全件事前検査する。
これにより、捨てられる重複Entryだけで指定されたFSがnilでも、採用側が正常なら読み込める。
空EntryはFS不要でnon-nil空slice。本文を採用したEntry単位で表示Pathを付け、
template除外による配列indexのずれを起こさない。読込失敗は表示Pathで包み、下位causeを保持する。

## 採用しない代替案と理由

- graphのphaseから4ファイルを推測し直す方法は採らない。本家はcompiled graphの指定一覧を使う。
- 表示pathをそのままprojectFSで読む方法は採らない。active Spaceと明示overrideへ対応できない。
- 全ての入力FSを無条件に要求しない。未使用sourceの欠落で正常な一覧を失敗させない。
- 単一FSへの非明示fallbackは行わない。必須文書の誤読込や欠落の隠蔽を避ける。
- filesystemのopen/closeやenv読取をresolverに含めない。内部接続と公開供給元の未回答事項を分離する。

## TDD sliceとloop

1. `graph.Load` でrules metadataの保持、省略/明示空、exact key、不正型・scopeをRED→GREEN。
2. `Snapshot.Stages`、`stageplan.Plan`、`orchestrator.Directive.Stage` の3経路で所有権をRED→GREEN。
3. active Space、default、相対/絶対overrideの純粋解決をRED→GREEN。
4. 最初のmarker、非memory参照、表示と物理配置の分離、安全なpathをRED→GREEN。
5. 全Entry検証、display first-wins、実際に使うFSだけのnil検査をRED→GREEN。
6. template除外後の表示対応、必須欠落・不正UTF-8・部分結果破棄をRED→GREEN。
7. 同じRootで編集後に新本文を取得する実FS接続を確認する。
8. Root継続利用・内向き/外向きsymlink・無関係ファイル不変を確認する。

```sh
GOTOOLCHAIN=go1.26.8 go test -count=1 -run 'TestLoadRulesInContext|TestSnapshotReturnsDefensiveCopies' ./src/internal/graph
GOTOOLCHAIN=go1.26.8 go test -count=1 -run '^TestPlanRulesInContextOwnership$' ./src/internal/stageplan
GOTOOLCHAIN=go1.26.8 go test -count=1 -run '^TestDirectiveRulesInContextOwnership$' ./src/internal/orchestrator
GOTOOLCHAIN=go1.26.8 go test -count=1 -run 'TestResolveRulePaths|TestReadResolvedRules' ./src/internal/steering
GOTOOLCHAIN=go1.26.8 go test -tags=integration -count=1 -run '^TestReadResolvedRules' ./src/internal/steering
```

test名は実際の観測seamに合わせて具体化できる。失敗は目的の動作差であることを確認し、
最小実装→GREEN→refactorの順とする。loopでは全package、race、vet、cross compileを行わない。

## finalと引渡し

独立review後、差分が安定したheadに対して親が1回だけ開始し、targetを変更しない。

```sh
GOTOOLCHAIN=go1.26.8 go test -count=1 -shuffle=on ./...
GOTOOLCHAIN=go1.26.8 go test -race -count=1 -shuffle=on ./...
GOTOOLCHAIN=go1.26.8 go test -tags=integration -race -count=1 -shuffle=on ./...
GOTOOLCHAIN=go1.26.8 go vet ./...
GOTOOLCHAIN=go1.26.8 go vet -tags=integration ./...
GOTOOLCHAIN=go1.26.8 go mod tidy -diff
gofmt -l src
git diff --check 93512c8..HEAD
```

変更Go fileの `gopls check` を行い、integration fileは `GOFLAGS=-tags=integration` を使う。
cross compileは `CGO_ENABLED=0` でdarwin/linux/windows × amd64/arm64の
`./src/cmd/aidlc` buildと `./src/internal/steering` integration test binaryを作る。
これは各OS上でのnative実行証拠ではない。原稿は `docs/aidlc-content/SHA256SUMS` で確認する。

CI設定はPR #90のsteering integration対象を継承する。対象headのQuality/Buildが未開始、
pending、cancel、failureならmergeしない。成功後はmerge commit方式でmergeし、
mainへの反映とIssue closeを確認する。final後に対象差分を変更した場合は検証をstaleとする。

## 残余リスクと戻し方

Rootの正しい供給はcaller契約であり、内部APIに渡された任意のFSを強制sandbox化するものではない。
配置ファイルを途中で同時編集した場合の原子的snapshotはない。新しい永続形式や利用dataを
作らないため、問題は通常の修正PRまたはrevert PRで戻せる。

公開供給元とknowledge文字順の未決事項は既存マイルストーンRAMに残す。このIssueで
回答があったことにしたり、新しい公開契約を採用したりしない。

## 追補実施記録（2026-09-04、親finding対応、verification_mode=loop）

親の独立review前findingを受け、既承認の参照解決契約に適合する範囲だけを補正した。明示した
`rulesDir`はoperatorが選ぶ配置rootなので`.`、`./rules`、`../rules`、絶対path内のdot segment、
whitespaceを含む値をそのまま受け付け、`projectDir`も絶対性だけを検証する。graph由来の
`ReadPath`は従来どおりFS相対pathとして検証し、配置rootの相対`../`とは境界を分けた。
active Spaceと参照pathのcolonはOS固有の`filepath` localityへ委譲し、POSIXのcolon名を新たに
拒否せず、Windows nativeのdrive-relative／volume pathは拒否する。`rulesDir`についても
Windowsのdrive-relative／driveなしroot-relative指定をproject基準へ解釈し直さない。resolverは生成した
display `Path`とphysical `ReadPath`も`ReadResolvedRules`と同じEntry検証へ通す。

追加したRED/GREENの証拠は次のとおり。最初のpath-locality REDは修正前の未承認なdot/colon
rejectを確認したbehavior-first REDであり、生成EntryのREDは出力検証漏れを確認するため生成
`ReadPath`を一時的に不正値へ置いたnegative-control REDである。既存実装記録のAPI compile RED、
既存negative-control RED、および先行sliceでGREENになった補足testの分類は変更していない。

1. `GOTOOLCHAIN=go1.26.8 go test -count=1 -run 'TestResolveRulePathsAllowsExplicitRulesDirectory|TestResolveRulePathsAllowsDotSegmentProjectDirectory|TestResolveRulePathsNativeColonNames' ./src/internal/steering` は、修正前に `invalid project directory`、`rules directory is not safe`、POSIX `C:record` の `invalid active Space` で失敗した。修正後、`gofmt -w src/internal/steering/resolve.go src/internal/steering/resolve_test.go` と同コマンドを再実行し `ok github.com/sori883/ai-dd/src/internal/steering 0.335s`。
2. `GOTOOLCHAIN=go1.26.8 go test -count=1 -run '^TestResolveRulePathsEntriesPassReadResolvedRulesValidation$' ./src/internal/steering` は、生成memory `ReadPath`を一時的に `.../..` としたnegative-controlで `invalid resolved rule ... read path ... invalid argument` のRED。元の生成値へ戻し、生成後にPath/ReadPathを検証する最小実装を加えて、同testは `ok github.com/sori883/ai-dd/src/internal/steering 0.341s`。
3. `GOTOOLCHAIN=go1.26.8 go test -tags=integration -count=1 -run '^TestReadResolvedRulesIntegrationConnectsResolverAndOverrideRoot$' ./src/internal/steering` は、`ResolveRulePaths`のrelative override、`MemoryDir`の `os.OpenRoot`、返却Entriesの `ReadResolvedRules`接続、同じRootでの編集反映、override欠落時のdefault fallbackなしを実FSで確認し、`ok github.com/sori883/ai-dd/src/internal/steering 0.344s`。
4. `TestResolveRulePathsNativeDriveRelativeRulesDirectory` と `TestResolveRulePathsRejectsWindowsDriveLessRootedRulesDirectory` を追加し、Windowsでは `C:rules` と `\rules`／`/rules` をproject基準へ解釈しない拒否分岐、POSIXではcolon directoryを許可する分岐を保持した。今回のPOSIX実行では後者はskipされ、前者のPOSIX許可分岐は最終targetedで通過した。

docsのresolver説明にも、設定するrootの相対`../`と、root内FSへ渡す参照`ReadPath`の`../`が別の
検証境界であることを追記した。変更後は所有範囲のGo fileへgofmtを適用し、計画記載の最小
targeted testだけを再実行する。loopのため全package、race、vet、lint、cross build、公開E2Eは
実行しない。

## 実施記録（2026-09-04、verification_mode=loop）

単独writerとして計画の範囲内を実装した。`graph.Stage`へ`RulesInContext []Rule`を追加し、
`rules_in_context`の省略・明示空、順序・重複、exact lowercase field、必須値・scopeの検証を
行った。`Snapshot.Stages`、`stageplan`、`orchestrator.Directive.Stage`の3経路でsliceを複製した。
`steering.ResolveRulePaths`は入力をJoin前に検証し、active Spaceと相対・絶対rulesDir、最初の
`/memory/`、表示pathと物理読込pathの分離を純粋に解決する。`ReadResolvedRules`は全Entry検証、
display first-wins、使用sourceだけのnil/typed-nil検査、既存`ReadRules`委譲、template除外後の
表示path保持、fresh readとfail-closedを実装した。

実装・テスト対象は次のとおり。

- `src/internal/graph/graph.go`、`graph_test.go`
- `src/internal/stageplan/plan.go`、`plan_test.go`
- `src/internal/orchestrator/directive.go`、`directive_test.go`
- `src/internal/steering/resolve.go`、`resolve_test.go`、`resolve_integration_test.go`
- `docs/architecture.md`、`docs/development.md`

観測したRED/GREENは次のとおり。各GREEN後に変更Go fileへ`gofmt`を適用した。APIが存在しない段階のcompile RED、
実装を一時的に外して契約違反を確認するnegative-control RED、実装前の入力に対するbehavior-first REDを区別して記録する。

1. `TestLoadRulesInContext`: Stage/Rule未定義によるcompile RED（API欠落） → graph metadata実装後GREEN。
2. `TestLoadRejectsInvalidRulesInContext`: validationを一時除去したnegative-control RED（欠損・空・不正scopeを通過） → validation復元後GREEN。
3. `TestSnapshotReturnsDefensiveCopies`: RulesInContext cloneを一時除去したnegative-control RED（内部slice変更を観測） → clone追加後GREEN。
4. `TestPlanRulesInContextOwnership`、`TestDirectiveRulesInContextOwnership`: 各cloneを一時除去したnegative-control RED（caller変更漏れ） → 2経路へclone追加後GREEN。
5. `TestResolveRulePathsUsesActiveSpaceAndRulesDirectory`: API未定義によるcompile RED（API欠落） → resolver最小実装後GREEN。
6. `TestResolveRulePathsUsesFirstMemoryMarker`: last marker実装によるbehavior-first RED（誤表示path） → first markerへ修正後GREEN。
7. `TestResolveRulePathsRejectsUnsafeInput`: 検証未実装によるbehavior-first RED（unsafe入力を受理） → Join前検証後GREEN。
8. `TestReadResolvedRules`: API未定義によるcompile RED（API欠落） → first-wins、filter対応、fresh readの実装後GREEN。
9. `TestReadResolvedRulesAllowsUnusedFilesystems`、`TestReadResolvedRulesRejectsInvalidEntriesBeforeIO`、`TestReadResolvedRulesRejectsNilFilesystemWithoutPanic`、`TestReadResolvedRulesReportsDisplayPathOnReadFailure`: 使用FS・Entry検証・error契約を追加後GREEN（独立REDは取得せず、先行sliceの実装で確認）。
10. `TestReadResolvedRulesIntegration*`: caller-owned`os.Root.FS()`、Root継続利用、編集反映、内向き／外向きsymlinkを追加後integration GREEN（独立REDは取得せず、先行sliceの実装で確認）。

全Go変更へ最後の`gofmt`を適用した後のfresh targeted結果は次のとおり。

```text
GOTOOLCHAIN=go1.26.8 go test -count=1 -run 'TestLoadRulesInContext|TestSnapshotReturnsDefensiveCopies' ./src/internal/graph
ok   github.com/sori883/ai-dd/src/internal/graph  0.350s
GOTOOLCHAIN=go1.26.8 go test -count=1 -run '^TestPlanRulesInContextOwnership$' ./src/internal/stageplan
ok   github.com/sori883/ai-dd/src/internal/stageplan  0.325s
GOTOOLCHAIN=go1.26.8 go test -count=1 -run '^TestDirectiveRulesInContextOwnership$' ./src/internal/orchestrator
ok   github.com/sori883/ai-dd/src/internal/orchestrator  0.345s
GOTOOLCHAIN=go1.26.8 go test -count=1 -run 'TestResolveRulePaths|TestReadResolvedRules' ./src/internal/steering
ok   github.com/sori883/ai-dd/src/internal/steering  0.334s
GOTOOLCHAIN=go1.26.8 go test -tags=integration -count=1 -run '^TestReadResolvedRules' ./src/internal/steering
ok   github.com/sori883/ai-dd/src/internal/steering  0.344s
```

loopでは全package、race、vet、lint、cross build、公開E2Eを実行していない。Rootのopen/Closeと
正しい配置rootの供給はcaller契約であり、この内部APIは任意の`fs.FS`を完全sandbox化しない。
複数Markdown同時編集のatomic snapshot、公開CLI接続、knowledge順序、root探索は残余リスクまたは
計画対象外として維持する。外部Go module・tool、埋込み、cacheは追加していない。

### 追補後の最終targeted再確認

追加補正後に変更Go fileへgofmtを再適用し、計画に記載したloop範囲のtargeted testだけをfresh実行した。

```text
$ GOTOOLCHAIN=go1.26.8 go test -count=1 -run 'TestLoadRulesInContext|TestSnapshotReturnsDefensiveCopies' ./src/internal/graph
ok   github.com/sori883/ai-dd/src/internal/graph  0.333s
$ GOTOOLCHAIN=go1.26.8 go test -count=1 -run '^TestPlanRulesInContextOwnership$' ./src/internal/stageplan
ok   github.com/sori883/ai-dd/src/internal/stageplan  1.179s
$ GOTOOLCHAIN=go1.26.8 go test -count=1 -run '^TestDirectiveRulesInContextOwnership$' ./src/internal/orchestrator
ok   github.com/sori883/ai-dd/src/internal/orchestrator  0.757s
$ GOTOOLCHAIN=go1.26.8 go test -count=1 -run 'TestResolveRulePaths|TestReadResolvedRules' ./src/internal/steering
ok   github.com/sori883/ai-dd/src/internal/steering  0.450s
$ GOTOOLCHAIN=go1.26.8 go test -tags=integration -count=1 -run '^TestReadResolvedRules' ./src/internal/steering
ok   github.com/sori883/ai-dd/src/internal/steering  0.875s
```

POSIX上ではcolonを含むactive Space、参照path、明示rulesDirを受け付ける分岐と、dot segment／
whitespaceを保持する分岐が通過した。Windowsのdrive-relative拒否とcolonを含むnative path拒否は
同じコードのOS分岐としてテストに保持しているが、このloop環境ではWindows実行自体は行っていない。

### drive-less root補正後の最終targeted再確認

Windowsのdriveなしroot-relative `rulesDir` 拒否を追加した後、変更Go fileへgofmtを再適用し、
loopの狭いtargeted testだけをfresh再実行した。

```text
$ GOTOOLCHAIN=go1.26.8 go test -count=1 -run 'TestLoadRulesInContext|TestSnapshotReturnsDefensiveCopies' ./src/internal/graph
ok   github.com/sori883/ai-dd/src/internal/graph  0.344s
$ GOTOOLCHAIN=go1.26.8 go test -count=1 -run '^TestPlanRulesInContextOwnership$' ./src/internal/stageplan
ok   github.com/sori883/ai-dd/src/internal/stageplan  0.651s
$ GOTOOLCHAIN=go1.26.8 go test -count=1 -run '^TestDirectiveRulesInContextOwnership$' ./src/internal/orchestrator
ok   github.com/sori883/ai-dd/src/internal/orchestrator  0.953s
$ GOTOOLCHAIN=go1.26.8 go test -count=1 -run 'TestResolveRulePaths|TestReadResolvedRules' ./src/internal/steering
ok   github.com/sori883/ai-dd/src/internal/steering  1.234s
$ GOTOOLCHAIN=go1.26.8 go test -tags=integration -count=1 -run '^TestReadResolvedRules' ./src/internal/steering
ok   github.com/sori883/ai-dd/src/internal/steering  1.529s
```

POSIX環境ではWindows driveなしroot-relative拒否testがskipになり、他のnative分岐と全対象sliceは
通過した。Windows上のnative拒否自体はこのloopでは実行していない。

## 追補実施記録（2026-09-04、独立review P1対応、verification_mode=loop）

独立reviewで、POSIXの既存`workspace.localizeSpace`が許すliteral backslashを
`ResolveRulePaths`のactive Space検証だけが拒否していることが判明した。固定本家
`aidlc-lib.ts:585-586`のtoPosixと既存workspace契約に合わせ、表示名とFS参照pathの検証境界を分離した。
active Spaceと表示用Entry `Path`は`fs.ValidPath`、dot／component条件、`filepath.Localize`相当の
native検証を用い、POSIX literal backslashを保持する。元graph参照とEntry `ReadPath`は既存
`validateRulePath`／`ReadRules`のbackslash拒否を維持した。既存の承認範囲内の通常bug修正であり、
新しい仕様差分ではない。

RED/GREEN:

1. 旧headで `GOTOOLCHAIN=go1.26.8 go test -count=1 -run '^TestResolveRulePathsNativeBackslashActiveSpace$' ./src/internal/steering` を実行し、POSIX literal backslash Spaceを `invalid active Space` として拒否する意図したbehavior REDを確認した。
2. 旧headで `GOTOOLCHAIN=go1.26.8 go test -tags=integration -count=1 -run '^TestReadResolvedRulesIntegrationPreservesPOSIXBackslashSpace$' ./src/internal/steering` を実行し、同じ拒否をresolver接続fixtureで確認するintegration REDを得た。
3. active Space／表示Pathをnative検証へ分離し、`ReadPath`／元refのbackslash拒否を維持した最小GREEN後、`TestResolveRulePathsNativeBackslashActiveSpace`、`TestReadResolvedRulesNativeDisplayPath`、`TestReadResolvedRulesRejectsInvalidDisplayPathBeforeIO` は `ok .../steering 0.343s`、integration `TestReadResolvedRulesIntegrationPreservesPOSIXBackslashSpace` は `ok .../steering 0.334s` となった。
4. 既存`TestResolveRulePathsRejectsUnsafeInput`をOS別期待へ修正し、Windows native拒否・ReadPath／元ref backslash拒否・不正display path拒否を targeted suite で確認した。POSIX上のWindows拒否caseはskipであり、Windows実行はしていない。

今回の追補では`resolve.go`、resolver単体test／integration test、architecture/development、当計画RAMだけを変更し、
索引の既存Issue #91行は維持した。過去のRED/GREEN記録は削除・上書きしていない。

P1修正後のfresh targeted再確認は次のとおり。

```text
$ GOTOOLCHAIN=go1.26.8 go test -count=1 -run 'TestResolveRulePaths|TestReadResolvedRules' ./src/internal/steering
ok   github.com/sori883/ai-dd/src/internal/steering  0.172s
$ GOTOOLCHAIN=go1.26.8 go test -tags=integration -count=1 -run '^TestReadResolvedRules' ./src/internal/steering
ok   github.com/sori883/ai-dd/src/internal/steering  0.281s
```

POSIX環境ではliteral backslash Space／表示`Path`の回帰、元ref／`ReadPath`のbackslash拒否、
不正display pathのI/O前拒否、既存resolver／reader integrationが通過した。Windows分岐は
native Localize拒否をコードとOS別期待testへ保持しているが、loopではWindows実行を行っていない。
