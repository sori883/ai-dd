# 保存済み aidlc-state.md reader の実装計画

- 日付: 2026-09-03（Asia/Tokyo）
- 状態: Accepted（Issue #63、ユーザー明示承認済み）
- GitHub Issue: [#63](https://github.com/sori883/ai-dd/issues/63)
- 承認: 2026-09-03、ユーザーがstrict parsing、nonregular leaf拒否、file size上限なし、標準ライブラリのみの計画を明示承認
- base: `fe50297b27907cd8e9658f174ef5d005bbfc3488`（PR #62 merge）
- 作業branch: `codex/state-reader`
- verification mode: `loop`（独立review後のfinal検証は親agentが担当）
- 関連: [参照契約](../research/2026-09-03-state-reader-contracts.md)、[in-flight recompose方針](2026-09-03-inflight-recompose-policy.md)

## 背景と目的

AI-DLCのIntent recordには、現在のStage、workflow status、phaseの表示状態、各Stageのcheckbox、承認済み
Planを表す`EXECUTE` / `SKIP` suffixを持つ`aidlc-state.md`が保存される。既存のGo実装には初期stateを
構築する`state.BuildInitial`と保存する`state.WriteInitial`があるが、保存済みstateを後続処理で安全に
利用するreaderがない。

この計画では、callerが既に選択して開いたrecord `*os.Root`から固定leafをread-onlyで読み、State Version 8の
canonical Markdownをtyped `State`へ変換する。readerはstateを更新せず、graphやaudit、CLI、stage実行へは
接続しない。保存済みsuffixを後続のrouting/recomposeが利用できる境界だけを確立する。

## 実装範囲と所有権

実装担当が変更するファイルは次のとおりである。

- `src/internal/state/state.go`（新規: typed State、enum、value accessor）
- `src/internal/state/read.go`（新規: `Read`、`Parse`、strict parser）
- `src/internal/state/read_test.go`（新規: parser unit test）
- `src/internal/state/read_integration_test.go`（新規: `//go:build integration`、実`os.Root` test）
- `docs/ram/research/2026-09-03-state-reader-contracts.md`（新規: 固定本家snapshotの参照契約）
- `docs/ram/decisions/2026-09-03-state-reader-plan.md`（本計画と実施記録）
- `docs/ram/README.md`、`docs/architecture.md`、`docs/development.md`（索引・責務・検証手順）

`initial.go`、`write.go`と既存test fixtureは、round-tripに必要な最小変更以外は変更しない。Issue、PR、
commit、push、merge、独立review、final検証は親agentが担当する。外部Go module/toolは追加しない。
作業treeにある他者の変更はrevert・上書きしない。

## APIとtyped model

公開する内部APIは次のとおりである。

```go
func Read(recordRoot *os.Root) (State, error)
func Parse(content []byte) (State, error)
```

`State`の保存fieldは非公開とし、value accessorからVersion、Scope、ProjectType、WorkflowStatus、
LifecyclePhase、CurrentStage、NextStage、Summary、canonical 5件のPhaseProgress、document順のStagesを
取得できる。`Summary`は`TotalStages int`、`Completed int`、`InProgress string`を持つ。

各`StageProgress`は、documentの`Slug`、`CheckboxMarker`、trim済みraw `Suffix`、markerから導いた
`CheckboxState`、suffix先頭のword boundary付きaction wordから導いた`PlanAction`を持つ。checkbox stateはpending、in-progress、
awaiting-approval、revising、completed、skipped、PlanActionはEXECUTE、SKIPである。両者は直交し、
`[S] stage-slug — EXECUTE`も有効とする。enumのzero/unknown valueはparse成功値にしない。

slice accessorはdefensive copyを返し、Parseは入力byteのbacking arrayを保持しない。成功時だけ非zero Stateを
返し、どのerrorでも`State{}`を返す。

## Parse契約

次の構造・値をstrictに検証する。

- invalid UTF-8、lone CR、`\r\r\n`、先頭header不一致を拒否する。先頭UTF-8 BOMは最大1個を除去し、2個目はheader mismatchとする。
- LF、CRLF、混在、末尾改行なしを受け入れる。`bufio.Scanner`の64 KiB制限は使わず、file size上限を追加しない。
- 先頭は`# AI-DLC State Tracking`に一致し、`Project Information`、`Execution Plan Summary`、`Phase Progress`、`Stage Progress`、`Current Status`を各1回要求する。section順とfield順は固定せず、未知追加sectionは許容する。
- Project Informationでは`Project Type`、`Scope`、`State Version`、Execution Plan Summaryでは`Total Stages`、`Completed`、`In Progress`、Current Statusでは`Lifecycle Phase`、`Current Stage`、`Next Stage`、`Status`をそれぞれ対応section内で一意に要求する。
- 必須fieldはtrim後nonemptyとし、`none`は通常の文字列値として許容する。State Versionはbare token `8`だけ、整数はASCII decimalの`0`または先頭zeroなしでint overflowなしだけを許容する。workflow/phase/phase statusはexact caseの既知enumだけを許容する。
- Phase Progressは`Initialization`、`Ideation`、`Inception`、`Construction`、`Operation`のexact順を要求し、各statusを`Pending`、`Active`、`Verified`、`Skipped`から選ぶ。
- Stage Progress内だけをdocument順に走査し、rowは`- [marker] slug — suffix`とする。markerは`[ ]`、`[-]`、`[?]`、`[R]`、`[x]`、`[S]`、separatorはU+2014 EM DASHとする。dash前後のhorizontal whitespaceは柔軟に扱い、slugはnonemptyかつwhitespaceなしとする。
- suffixはtrim後nonempty、先頭action wordがexact `EXECUTE`または`SKIP`であることをword boundaryで確認し、後続説明をraw Suffixへ保持する。`EXECUTE: reason`・`SKIP: reason`は受理し、`EXECUTEfoo`・`SKIP_foo`などword継続は拒否する。duplicate slug、Stageらしく`- [`で始まるmalformed row、Stage row 0件を拒否し、section外のdecoyは無視する。
- `Completed <= Total`、`[x]`件数、Current Stageとmarkerの一致、graph membershipやcanonical slug grammarなどのcross validationは行わない。

## Read契約

`Read`はnil rootをI/O前に`fs.ErrInvalid`で拒否する。record root相対の固定leaf `aidlc-state.md`だけを
`Lstat`し、regular fileであることを確認してから同じRootで全量readし`Parse`へ渡す。symlink、directory、FIFO、
deviceなどはregularでないため拒否する。caller-owned rootをCloseせず、state bytes、mode、mtimeを変更しない。
通常readに伴うatimeの不変までは保証しない。missing、permissionなどのI/O causeは`%w`でerror chainへ保持し、
全errorでStateをzeroにする。Lstat/read間のTOCTOUはこのsliceでは完全解消しない。

## 本家AI-DLCとの意図的な差分

比較対象はrepositoryに固定された本家AI-DLC `2.6.123`の確認範囲だけであり、最新upstreamとの一致は未確認で
ある。以下はGoへの移植上の不可避な差分ではなく、ユーザーが2026-09-03に明示承認したfail-closed判断である。

| 本家の挙動 | Goで採用する挙動 | 変更理由 | 利用者・互換性への影響 |
| --- | --- | --- | --- |
| `getField`はdocument全体の最初の一致を使い、sectionを限定しない | 必須fieldを対応section内で一意に要求し、重複を拒否 | decoyや曖昧な重複がrouting値へ混入するのを防ぐ | legacy・手編集の曖昧stateは本家より早く停止し、修復が必要 |
| `parseCheckboxes`はdocument全体をregex走査し、不一致のmalformed rowを無視し得る | Stage Progress内だけを読み、`- [`で始まるmalformed rowを拒否 | section外decoyを無視しつつ、部分破損を見逃さない | 破損stateはpendingへ黙って変換されず、早期に修復要求となる |
| duplicate Stageは一覧に残り、suffix Mapでは後勝ちになり得る | duplicate slugを拒否 | PlanActionの一意性を保つ | duplicateを含むstateは修復が必要 |
| 通常`readStateFile`はsymlinkを追跡し得る | `Lstat`でregular leafだけを許可 | record root外のbytesをstateとして取り込まない | symlink共有stateは拒否される |
| reader契約にinvalid UTF-8・不正CRの明示拒否がない | invalid UTF-8、lone CR、`\r\r\n`を`fs.ErrInvalid`で拒否 | byte列・行境界の曖昧な解釈を防止 | 不正encoding・改行stateは修復が必要 |

根拠は[参照契約](../research/2026-09-03-state-reader-contracts.md)に固定し、未確認の本家挙動を差分なしと断定しない。

## TDDと検証手順

`verification_mode=loop`で、観測可能なbehaviorごとにRED→最小GREEN→green上のrefactorを行う。各Go変更後に
対象Go fileへ`gofmt`を適用し、影響するtargeted testを再実行する。loopでは全package test、race、vet、lint、
cross compile、配布E2Eを実行しない。

1. canonical Stateと`BuildInitial`のround-trip、全marker/action、document order、`[S]` + `EXECUTE`、accessor ownershipを固定する。
2. UTF-8/BOM/newline/header、unknown追加section、64 KiB超入力の境界を固定する。
3. section-scoped field、duplicate/missing/empty、scalar canonicality、version、enum、overflowを固定する。
4. phase/stage order、decoy、duplicate、malformed、raw suffix、graph/cross-validation非実施を固定する。
5. 実`os.Root`でsuccess、missing、nil、directory、symlink、invalid content、Root継続利用、filesystem非変更を固定する。

loopのtargeted commandは次のとおりである。各commandは、それぞれcanonical parser、parser全体、accessor、
実Root境界だけを確認する。

```sh
go test -count=1 -run '^TestParseCanonicalState$' ./src/internal/state
go test -count=1 -run '^TestParse' ./src/internal/state
go test -count=1 -run '^TestState' ./src/internal/state
go test -tags=integration -count=1 -run '^TestRead' ./src/internal/state
```

## 実装時点のloop証拠

実装前のbaselineは、上記target patternがいずれも`ok [no tests to run]`であった。最初のcanonical testを
追加した時点では、次のcompile failureがREDになった。

```text
go test -count=1 -run '^TestParseCanonicalState$' ./src/internal/state
undefined: Parse
undefined: WorkflowStatusRunning
undefined: LifecyclePhaseIdeation
undefined: PhaseProgress
...
```

`state.go`、`read.go`とunit testを追加してcanonical round-tripをGREENにし、encoding/newline、section/value、
phase/stage strictness、ownership、semantic non-validationの各testを追加した。integration testは、regular
fileの成功、missing、nil、directory、symlink、invalid contentを最初に失敗させ、`Lstat` barrierとRoot readを
実装してGREENにした。文書化時点のfresh結果は次のとおりである。

各sliceの代表的なRED/GREEN commandと失敗理由は次のとおりである。REDのうち最初のcompile error以外は、
テスト追加直後にまだ該当parser境界を満たしていないことを表すbehavior failureである。

| slice | RED commandと失敗理由 | 最小GREEN command |
| --- | --- | --- |
| canonical model / round-trip | `go test -count=1 -run '^TestParseCanonicalState$' ./src/internal/state` — `Parse`、enum、typed modelが未定義でcompile failure | 同じcommand。canonical State、全marker/action、order、`BuildInitial` round-tripが成功 |
| encoding / newline / header | `go test -count=1 -run '^TestParseEncodingAndHeader$' ./src/internal/state` — BOM、invalid UTF-8、不正CR、長いunknown sectionの境界未実装 | 同じcommand。指定encodingと64 KiB超入力の挙動が成功 |
| section fields / scalar / enum | `go test -count=1 -run '^TestParseSectionScopedFieldsAndValues$' ./src/internal/state` — section外decoy、duplicate、canonical decimal、enum/overflowの期待とparserが不一致 | 同じcommand。section-scoped uniqueとvalue validationが成功 |
| phase / stage strictness | `go test -count=1 -run '^TestParseStageProgressStrictness$' ./src/internal/state` — malformed row、duplicate slug、order、raw suffixの境界未実装 | 同じcommand。phase/stage strictnessとsemantic non-validationが成功 |
| Root integration | `go test -tags=integration -count=1 -run '^TestRead' ./src/internal/state` — `Read`、regular-leaf barrier、Root ownership境界が未実装 | 同じcommand。success/missing/nil/nonregular/invalid contentが成功 |

```text
go test -count=1 -run '^TestParseCanonicalState$' ./src/internal/state
ok   github.com/sori883/ai-dd/src/internal/state

go test -count=1 -run '^TestParse' ./src/internal/state
ok   github.com/sori883/ai-dd/src/internal/state

go test -count=1 -run '^TestState' ./src/internal/state
ok   github.com/sori883/ai-dd/src/internal/state

go test -tags=integration -count=1 -run '^TestRead' ./src/internal/state
ok   github.com/sori883/ai-dd/src/internal/state
```

変更Go fileはgofmt済みで、`gofmt -l`は出力なし、`git diff --check`も成功している。docs追加・更新後は
文書差分に対して`git diff --check`を再実行する。独立reviewのfinding修正とloop確認が完了し、対象差分が安定した
後の全package test、race、vet、format、lint、cross compileなどのfinal gateは親agentが一度だけ実施する。

### 独立review finding修正のloop証拠

2026-09-03の独立reviewで、Stage suffixのword boundary、slug内EM DASH、required sectionテスト、filesystem snapshot、
Windows symlink fixtureに関するfindingを受けた。Finding 1/2では先に次の回帰テストを追加し、旧実装が失敗することを確認した。

```text
go test -count=1 -run '^TestParseStage(SuffixWithColonExplanation|SlugMayContainEmDash)$' ./src/internal/state
--- FAIL: TestParseStageSuffixWithColonExplanation
    ... stage suffix must begin with exact EXECUTE or SKIP token
--- FAIL: TestParseStageSlugMayContainEmDash
    ... stage suffix must begin with exact EXECUTE or SKIP token
FAIL
```

最小修正は、`EXECUTE` / `SKIP`直後のASCII word継続だけを拒否するprefix判定と、EM DASH separator候補を右から評価して
有効なslug・suffixを選ぶ処理である。`EXECUTE: reason`、`SKIP: reason`、内部EM DASH、action風substringを含むgreedy slugを
受理し、`EXECUTEfoo`・`SKIP_foo`を拒否することを同じpatternでGREENにした。Finding 3は初回実装がすでにsection順不同と
required section重複を正しく扱っていたためproduction REDはなく、全required sectionのmissing/duplicateとsection・field
reorderをテストで固定した。Finding 4/5は統合テストを修正し、metadata snapshotとsymlink fixtureの失敗分類を固定した。

```text
go test -count=1 -run '^TestParseStage(SuffixWithColonExplanation|SlugMayContainEmDash)$' ./src/internal/state
ok   github.com/sori883/ai-dd/src/internal/state
go test -count=1 -run '^TestParseRequiredSectionsMayBeReorderedButMustBeUnique$' ./src/internal/state
ok   github.com/sori883/ai-dd/src/internal/state
go test -tags=integration -count=1 -run '^TestReadIntegrationRejectsNonRegularLeaf$' ./src/internal/state
ok   github.com/sori883/ai-dd/src/internal/state
```

最終loop確認では`go test -count=1 -run '^TestParse' ./src/internal/state`、`go test -count=1 -run '^TestState' ./src/internal/state`、
`go test -tags=integration -count=1 -run '^TestRead' ./src/internal/state`、およびmetadata判定の`-count=10`を実行し、すべて
成功した。通常readに伴うatimeは保証せず、snapshotではdirectory entries、state bytes、mode、mtimeだけを比較する。

### 再review finding修正のloop証拠

再reviewでは、EM DASH候補探索の計算量、required sectionテストのfixture独立性、`Read` godocのmetadata契約が確認対象になった。
大量の不正Stage rowを公開`Parse`へ渡す回帰testを先に追加し、旧実装で次のREDを確認した。

```text
go test -timeout=3s -count=1 -run '^TestParseLargeMalformedStageRowScalesLinearly$' ./src/internal/state
panic: test timed out after 3s
...
github.com/sori883/ai-dd/src/internal/state.containsWhitespace
FAIL
```

test inputは50,000個のEM DASH候補、action不正なsuffix、同数の末尾空白を含む。最小修正では、末尾trim位置を一度だけ求め、
前向き走査中に各dash位置、suffix開始位置、dash以前のslug妥当性を記録する。候補は最後から評価するが、suffixのtrimは共通の
終端と各候補直後の空白runを使い、可変長suffixやslug prefixを候補ごとに再走査しない。このため前向き走査、末尾走査、逆順の
定数長action判定、最終slug trimの合計は入力長に対して線形である。

```text
go test -timeout=3s -count=1 -run '^TestParseLargeMalformedStageRowScalesLinearly$' ./src/internal/state
ok   github.com/sori883/ai-dd/src/internal/state
```

required section testはproductionの`requiredSections`を参照せず、5件のliteral一覧をtest側に保持するよう修正した。duplicate
fixtureは有効なsection blockを元sectionより前に複製し、duplicate guardを一時的に外すと5件すべてが`Parse() error = <nil>`に
なって失敗することを確認した。guardを復元した後、次のtestはGREENである。

```text
go test -count=1 -run '^TestParseRequiredSectionsMayBeReorderedButMustBeUnique$' ./src/internal/state
ok   github.com/sori883/ai-dd/src/internal/state
```

`Read`のgodocはstate bytes・mode・mtimeを変更せず、通常readによるatimeは保証しないと明記し、既存のarchitecture、development、
参照契約と一致させた。変更後も対象Go fileをgofmtし、parser/accessor/Rootのtargeted testと`git diff --check`を再実行する。

## 対象境界と残余risk

このIssueではstateの更新・advance・recompose本体、graphとのjoin、summary/checkboxの意味検証、audit、CLI、
migration、Plan専用sidecarを追加しない。将来のrecomposeでは保存済みStage suffixをroutingの正本として扱い、
人間承認後にcurrentより後ろのpending Stageだけを変更する方針を別記録から参照する。

最新upstreamとの差分、future State Version、crash durability、Lstat/read間TOCTOU、極端に大きな入力のメモリ
使用量、OSごとのsymlink挙動は未解決である。Windowsでsymlink作成権限がない場合は該当integration testだけを
理由付きでskipする。record rootのcontainmentはcallerが開く`*os.Root`を接続境界とし、任意の`fs.FS`をsandboxと
みなさない。
