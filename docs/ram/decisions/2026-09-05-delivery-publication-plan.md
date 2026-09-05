# Codex向け配信transactionと公開next／continueを接続する

- 日付: 2026-09-05（Asia/Tokyo）
- 状態: Implemented（知識供給の包括承認内、Issue #113）
- 対応Issue: [#113 Codex向け配信transactionと公開next／continueを接続する](https://github.com/sori883/ai-dd/issues/113)
- 実装許可: [ルール・知識のAI供給を個別承認なしで完了まで進める](2026-09-05-context-delivery-autonomous-authorization.md)
- 前提: [canonical run-stage composition](2026-09-05-run-stage-composition-plan.md)
- 基準: リポジトリ固定AI-DLC 2.6.123

## 背景と利用者が得る結果

Go版AI-DLCは、配置済みのrule、knowledge、工程手順、入力成果物と保存stateを毎回読み、
canonical `run-stage` JSON、必須ruleのchunk、継続token用claimsとfreshnessを組み立てられる。
HMAC-SHA256 token codecと、recordごとのprivate key lifecycleも実装済みである。

しかし、これらはまだ公開入口へ接続されていない。tokenを発行しても、どのtokenが現在有効かを
永続cursorで一回限り進めなければ、同じtokenの再利用や並行実行で同じchunkを複数回成功扱いにできる。
また、cursorへ記録する前にJSONを出力すると、利用者へ見えたdirectiveと永続状態が一致しない。

本Issueでは、Go単一binaryの`aidlc next`と`aidlc continue <token>`を公開し、固定本家の
`.aidlc-active-directive.json` version 2をrecord lock下で更新する。利用者はpart 1から必須ruleを順番に
受け取り、全partを一回ずつ受領した後だけ、既存composerとbyte一致する`run-stage`を得られる。
途中でrule、state、route、directiveが変わった場合や、古いtokenを再利用した場合はfail-closedにする。

## 作業ゴール内の位置と実装許可

ユーザーは「Codexへの配信・実読込を完成」を現在の作業ゴールとし、一般向けinstaller・updateは
その後の第5段階に置いた。このため本作業ゴールは、次の2つの小さなIssue／PRで進める。

1. 本計画: Go側の配信transaction、active-directive cursor、公開`next`／`continue`。
2. 後続計画: Codex receiverの配布用sourceと、fresh projectへbinaryとreceiverを明示配置して本文を
   実読込するE2E。一般向けinstall/update、既存設定の上書き・移行・rollback契約は扱わない。

[知識供給の包括承認](2026-09-05-context-delivery-autonomous-authorization.md)は、残るGo側配信、
Codex側の受領と実読込、一連の検証、Issue、PR、品質gate後の自律mergeまでを直接承認している。
本計画は固定2.6.123の既存public verbと既存永続形式を移植し、新しい公開形式や永続schemaを選ばないため、
個別の人間承認を待たず実装できる。

## 維持する境界

- runtime入力は利用先の`.codex/`、`aidlc/spaces/<active>/`、active recordから毎回読む。
  開発用`src/core/`、binary埋込み、fallback、永続cacheを追加しない。
- 既存`orchestrator.Next`のread-only、record binding、製品の人間承認gate、未対応Stageのfail-closedを
  迂回しない。ruleを読めることをStage完了や承認の証拠にしない。
- active-directive cursorは固定本家version 2のfieldを保持する。小さな独自schemaを作らず、未知version、
  破損、64 KiB超過、identity／state不一致、symlinkやnon-regular fileを拒否する。
- Go標準ライブラリだけを使用する。外部Go module、service、認証情報、追加権限は導入しない。
- 本IssueはStage本体の実行、成果物生成、`report`、sensor、review、人間応答、installer/updateを変更しない。
  これは後続能力を列挙するためではなく、公開権限・永続化・承認境界を誤認しないための制約である。

## 確認済みの固定本家契約

- `core/tools/aidlc-orchestrate.ts:3653-3728`: fresh `next`はpart 1の`load-steering`を返し、
  `continue`はstage、bundle、directive hash、route hash、state hashをfreshに照合する。最終partを
  continueした後だけ`run-stage`を返す。
- `core/tools/aidlc-orchestrate.ts:8343-8458`: 不正token、state／route変更、古いpartを拒否し、
  prepared successorをcursor transactionでcommitできた場合だけ公開する。
- `core/tools/aidlc-lib.ts:4140-4219,4700-5395`: intent単位のactive-directive version 2 marker、
  64 KiB上限、atomic replacement、revision、delivery、token digest、一回限りのcursor進行。
- `tests/unit/t248-steering-content-delivery.test.ts:496-697`と
  `tests/unit/t249-copilot-adapter.test.ts:1863-1888`: same-token exactly-once、rule／state／route変更、
  replay拒否の観測可能な契約。

固定本家のCodex skillは`next`／`continue`が返すtyped JSONを受ける。`load-steering`はtransportであり
`report`せず直ちに継続し、`run-stage`後の本文読込は後続receiver Issueで接続する。

## 設計

### record lockとguard-aware read

配信開始と継続は、既存の`recordlock.With`でactive recordを一つのtransactionとして固定する。
現在の`orchestrator.Next`は内部で同じlockを取得するため、外側のtransactionから再利用できる
guard-aware helperを追加する。`delivery.ComposeRunStage`にも同じhelperを渡す入口を追加し、公開済みの
read-only APIは既存挙動を維持する。

lock内でactive Space／Intent、state、graph、artifact、knowledge、ruleをfreshに構成し、keyの読込／初回作成、
token検証、cursor比較とatomic replacementまでを完了する。JSON stdoutへの書込みはcursor commit後だけ行う。
同じtokenを並行して提示した場合はrecord lockで直列化され、最初の1回だけcursorのtoken digestと一致する。

### active-directive marker

active record直下の`.aidlc-active-directive.json`を使用する。version 2の既知fieldと未使用のoptional fieldを
構造体で保持し、更新時にも固定2.6.123の情報を落とさない。現在のCodex通常Stageでは、project／intent／state、
revision、harness、kind、stage、part／parts、continue tokenとSHA-256、deliveryを使用する。

読込はsingle descriptor、最大64 KiB、valid UTF-8、JSONの単一値、既知schema、regular leafを検査する。
保存はrecord Root内に`0600`の排他的temporaryを作り、全byte writeとClose成功後にRenameする。
失敗時は旧markerを保持し、所有するtemporaryだけを安全に片付ける。caller所有Rootはcloseしない。

### 公開facade

`delivery.Next`相当はfresh compositionから最初のtokenを署名し、part 1のcanonical `load-steering`を作る。
markerに同じstage、part、token digest、state digestをcommitしてから返却する。chunkがなければ
`run-stage`を直接commit・返却する。

`delivery.Continue`相当は同じprivate keyでopaque tokenをdecodeし、fresh compositionを作り直す。
claims、freshness、現在marker、提示token digestが全て一致した場合だけ`AdvanceContinuation`を1段進め、
次の`load-steering`または最終`run-stage`をmarkerへcommitする。commit前の失敗はmarkerを変えない。
古いtoken、別recordのtoken、改ざん、途中変更はtyped errorとして識別し、正常directiveを返さない。

CLIは`aidlc next`と`aidlc continue <token>`を受け、workflowとして表現できる結果をstdoutへ
改行付きtyped JSONで1つだけ出す。正常時は`load-steering`または`run-stage`、改ざんtoken、
stale／superseded token、part／cursorの不一致は`{"kind":"error","message":"..."}`を出してexit 0とする。
これはCodex receiverが全結果を同じdirective dispatchで解釈する固定本家契約である。引数数などのCLI構文errorは
stderrとexit 2、directiveを安全に構成できないI/Oや内部failureはstderrとexit 1とし、stdoutへpartial JSONを
残さない。公開facadeはworkflow errorとinternal failureをtypedに分類し、CLIが出力契約を一意に選べるようにする。

### 出力失敗

marker commit後にstdout書込みが失敗しても、cursorを過去へ巻き戻さない。`next`は常にfresh sequenceを
part 1から再発行できる。`continue`でcommit済みの古いtokenは再利用を拒否し、利用者はfresh `next`で
再開する。高度なsession rehydrateや複数windowのattempt共有は、ユーザーが第5段階に置いた復旧・並列の
実用化で扱い、本Issueでは安全なrestartを保証する。

これは固定本家のCodex通常経路で観測される「古いtokenを再利用せずfresh nextから再開」に従う。
Copilot固有attempt共有、legacy Kiro approval、swarm/unit authorityを通常Codexで実行可能にはしない。

### Review resolution: 破損cursorとfresh `next` recoveryの区別

固定本家のtransactionは、markerが存在しない場合、version 1のlegacy markerの場合、または
regular fileの内容が破損して読めない場合、fresh markerをbaseにして`next`を再発行できる。
この場合はrevision 0のfresh baseからpublication revision 1を作り、atomic replacementが成功した後に
directiveを返す。markerのsymlinkやnon-regular pathはRoot境界違反として置換せず、内部failureにする。

一方、`continue`は現在のmarkerを継続cursorの証拠として必要とするため、missing、legacy、破損、改ざん、
superseded markerをworkflow errorとしてtyped `error` directiveへ変換し、markerを置換しない。
したがって受け入れ条件の「破損cursorでfail-closed」は継続処理に適用し、fresh `next`の安全なrecoveryは
同じ固定契約内の別経路として扱う。この区別は新しい永続形式を追加するものではなく、2.6.123の
missing/corrupt/legacy parse結果とpublication transactionの既存挙動を明文化するものである。

## 対象fileと単独writer所有権

1人のGo実装担当が次を所有する。

- `src/internal/orchestrator/next.go`, `next_test.go`: guard-aware read-only helper。
- `src/internal/delivery/run_stage.go`, `run_stage_test.go`: guard-aware composition入口。
- `src/internal/delivery/active_directive.go`, `active_directive_test.go`: version 2 markerの検証とatomic I/O。
- `src/internal/delivery/facade.go`, `facade_test.go`, `facade_integration_test.go`: `next`／`continue` transaction。
- `src/internal/cli/cli.go`, `cli_test.go`, `delivery.go`, `delivery_test.go`: public grammar、DI、出力・exit code。
- `src/cmd/aidlc/main.go`, `delivery.go`, `main_delivery_integration_test.go`: production Root／identity接続とjourney。
- `docs/architecture.md`, `docs/development.md`, `docs/e2e-testing.md`: 公開契約と検証手順。
- 本記録と`docs/ram/README.md`。

実装担当はIssue・PRを操作せず、同じ作業treeの他の変更を戻さない。

## 1つのwork unitで行うTDD

`work_unit_id=delivery-publication-v2`として、次を順番にtest-firstで実装する。新APIのrunnable testへ
到達するため、計画した型、sentinel、function signatureと常に未実装errorを返すcompile-only scaffoldだけを
最初のsliceで許可する。

1. `guard-aware-next-compose`
   - 外側record lockの同じGuardで`Next`とcompositionを再ロックせず実行し、既存public APIは同じ結果を返す。
   - `go test -count=1 -run '^(TestNextWithGuard|TestComposeRunStageWithGuard)$' ./src/internal/orchestrator ./src/internal/delivery`
2. `active-directive-v2`
   - canonical round-trip、全optional field保持、破損・過大・symlink・nonregular・別identity／state／version拒否、
     atomic replacement失敗時の旧marker不変を確認する。
   - `go test -count=1 -run '^TestActiveDirective' ./src/internal/delivery`
3. `delivery-next`
   - part 1、署名token、canonical load wire、commit-before-return、chunkなしのrun-stageを確認する。
   - `go test -count=1 -run '^TestDeliveryNext' ./src/internal/delivery`
4. `delivery-continue-exactly-once`
   - 1段だけ進み、直列replayを拒否する。並列2要求は一方だけ成功し、commit failureでは同じtokenを再試行できる。
   - `go test -count=1 -run '^TestDeliveryContinue' ./src/internal/delivery`
5. `delivery-freshness-final`
   - rule、state、route、directiveを個別変更すると古いtokenを拒否し、最終part後のrun-stageは既存wireとbyte一致する。
   - `go test -count=1 -run '^TestDelivery(RejectsStale|PublishesRunStage)' ./src/internal/delivery`
6. `public-cli`
   - 構文、callback 1回、正常／workflow errorの単一JSON stdout、internal failureのstderr、exit code、
     short writeを確認する。
   - `go test -count=1 -run '^TestRun(Next|Continue)' ./src/internal/cli`
7. `distribution-journey`
   - repository外のfresh sandboxで単一binaryを起動し、`next`から複数`continue`を経て`run-stage`へ到達する。
     同token replay、途中rule変更、必須欠落もfail-closedにする。
   - `go test -tags=integration -count=1 -run '^TestDeliveryJourney$' ./src/cmd/aidlc`

work unit末尾では上記targeted command、影響package test、変更Go fileの`gofmt`、`git diff --check`を行う。
loop中は全package、race、vet、cross compile、配布E2E全体を繰り返さない。

## 独立reviewとfinal検証

固定base/headの独立reviewでは、lock順、marker schema保持、symlink／path境界、write failure、tokenの定数時間検証、
same-token直列・並列競合、freshness、commit-before-publication、Root ownership、CLIのmachine-readable stdout、
unsupported能力を有効化していないことを確認する。

blocking findingがなく差分が安定した後、親がread-only finalを1回実行する。

- `go test -count=1 -shuffle=on ./...`
- `go test -tags=integration -count=1 -shuffle=on ./...`
- 通常／integrationの`go test -race`と`go vet`
- `go mod tidy -diff`
- `gofmt -l src`
- 変更Go fileへの`gopls check`
- `git diff --check`
- CIと同じdarwin／linux／windows × amd64／arm64のCLIと該当test binary cross compile
- repository外fresh sandboxのpublic `next`→`continue` distribution E2E

final後に対象fileが変わった場合は証拠をstaleとし、targeted loop、再review、fresh finalへ戻す。

## 受け入れ条件

- `aidlc next`がcursor commit後にpart 1のcanonical `load-steering`を1つだけ出す。
- valid `aidlc continue <token>`が一度だけ次partへ進み、同tokenの直列・並列replayは正常directiveを得ない。
- 改ざん、stale、superseded、part／cursor不一致はstdoutの単一`error` directiveで終了し、
  Codex receiverがstderr解析なしで安全に停止できる。
- 全rule partの受領前に`run-stage`を出さず、最終part後のJSONが`ComposeRunStage`のwireとbyte一致する。
- rule、state、route、directive、selectionの変更、改ざん、別record token、破損cursorでfail-closedになる。
- cursorはversion 2、64 KiB、0600、atomic replacement、Root confinementを守り、失敗時に旧値を壊さない。
- 既存`orchestrator.Next`はkey、cursor、state、auditを変更しないread-onlyのままである。
- fresh sandboxで単一binaryの公開journeyが追加runtimeなしに成功する。
- 外部Go moduleと新しい意図的な本家差分を追加しない。

## リスクとrollback

最大のリスクは、state lockとcursor更新の順序不整合、partial write、replay、別record tokenの混入である。
全公開処理を既存record identityの単一lockへ集約し、Root内atomic replacement、token／marker／freshnessの全照合、
error時zero resultで抑える。問題時は本Issueのfacade／CLI／cursor追加をrevertでき、既存のread-only
`ComposeRunStage`と内部ライフサイクルはそのまま残る。cursorにはworkflow stateを進める権限がないため、
revert時にstate migrationは不要である。

## 後続receiverへの引継ぎ

本Issueのmerge後、別IssueでCodex配布用receiver sourceを`src/`配下に追加する。receiverは固定本家どおり、
`load-steering`本文を順序どおり保持して直ちにcontinueし、`run-stage`受領後に全`inline_context_paths`を
最初に実読込し、その完了後に`stage_file`と存在する全`consumes`を本文まで読む。fresh projectへbinaryと
receiverを明示配置するE2Eでsentinel本文の読込を検証する。一般向けinstaller/updateは第5段階まで扱わない。

## 実装記録

2026-09-05に、計画した7 sliceを単一writerのRED／GREEN loopで実装した。公開`aidlc next`／
`aidlc continue <token>`、guard-awareなfresh composition、固定2.6.123のactive-directive v2、
HMAC tokenとmarkerを結ぶ一回限りのcursor進行、commit後だけのdirective返却、全rule chunk後だけの
`run-stage`公開、CLIのtyped error分類、repository外fresh sandboxの配布journeyを接続した。

独立reviewで見つかった固定schemaとの不一致は、optionalなzero／false／`resume: null`と未知nested fieldの
lossless保持、`units[]`のUnit名制約（top-level `unit`は固定parserどおりtrim後nonempty）、stage／token digest
binding、sessionless attempt、初回revision 1、既存baseのcounter／resume保持、directive固有fieldの消去、
state変更時のfresh `next` base継承、markerとattemptのownership epoch整合、temporary fileの差替え検知と
所有inodeだけのcleanup、予約flag表記を含むopaque tokenの解釈として回帰testを先に追加して修正した。
破損markerのfresh `next` recoveryと、同じmarkerを使う`continue`のfail-closedは上記review resolutionどおり
区別した。外部Go moduleと新しい意図的差分は追加していない。
