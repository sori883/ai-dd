# 配信chunk継続tokenとfreshness検証を純粋APIとして実装する

- 日付: 2026-09-05（Asia/Tokyo）
- 状態: Accepted（知識供給の包括承認内）
- GitHub Issue: [#105](https://github.com/sori883/ai-dd/issues/105)
- 実装許可: [ルール・知識のAI供給を個別承認なしで完了まで進める](2026-09-05-context-delivery-autonomous-authorization.md)
- verification mode: `loop`（独立review後の`final`は親エージェントが1回だけ実施）

## 背景と利用者が得る結果

長い必須ルールは複数の`load-steering`へ分割される。継続tokenは、受け手が次に要求できるpartと、
配信開始時点のStage、scope、rule bundle、完成形run-stage、route、workflow stateを結び付ける署名付きの値である。
単なるpart番号では、受け手や第三者が途中を飛ばしたり、Markdownやstateが変わった後に古いruleと新しい
run-stageを混ぜたりできる。

このIssueでは、callerが現在値と32-byte秘密鍵を渡す純粋な内部APIを追加する。HMAC-SHA256で改ざんと
別keyのtokenを拒否し、freshな値が署名済みclaimsと一致した場合だけ次chunkまたは最終完了境界を返す。
filesystem、clock、process global、workflow stateを変更しないため、後続の配信compositionがfresh readと
一回限りcursorを安全に接続する前提部品になる。

## 固定本家2.6.123の根拠

比較対象はリポジトリ固定AI-DLC 2.6.123の次の確認範囲であり、最新upstreamとの一致は主張しない。

- `core/tools/aidlc-orchestrate.ts:2038-2058`: version 1 token payloadのfieldと型。
- 同`:3413-3420,3501-3608`: 32-byte key、HMAC-SHA256、base64url envelope、constant-time MAC比較、
  version・part・nullable／optional field・gate・unit gate rhythmの検証。
- 同`:3614-3650`: Stage、scope、次part、bundle、directive hash、route hash、state hashと再構成入力の結合。
- 同`:3653-3726`: ruleのfresh read後のbundle／directive比較、part進行、最終run-stage境界。
- 同`:8343-8373`: continuation時のstate、Stage存在、route再検証。
- 同`:8440-8451`および`core/tools/aidlc-lib.ts:5337-5643`: exactly-onceはtoken codecではなく、
  別のactive-directive cursor transactionが保証する。
- `tests/unit/t248-steering-content-delivery.test.ts:331-394,496-518,580-697`: private keyのproject分離、
  token再利用拒否、part skip改ざん、rule／state／scope route変更の拒否。

固定本家tokenに発行時刻・失効時刻はなく、TTLは追加しない。Stop Hookの`probe` envelopeはCodex向け通常配信の
tokenではないため、この段階では受理しない。公開pathからkeyを導出するfallbackも追加しない。

## 実装範囲とAPI

所有packageは、既存の`ChunkRules`、`BundleDigest`、`MarshalLoad`と同じ`src/internal/steering`とする。

- `src/internal/steering/token.go`（新規）
- `src/internal/steering/token_test.go`（新規）
- `src/internal/steering/continuation.go`（新規）
- `src/internal/steering/continuation_test.go`（新規）

既存fileは原則変更しない。compile-only stubに必要な最小変更があっても、既存APIの観測可能な契約を変えない。

```go
type GateValue uint8
type UnitGateRhythm string

type OptionalNullableString struct {
    Present bool
    Value   *string
}

type ContinuationClaims struct {
    Version         int
    Stage           string
    Scope           string
    NextPart        int
    Bundle          string
    DirectiveHash   string
    RouteHash       string
    StateAware      bool
    Unit            *string
    UnitKind        *string
    ForcePersona    bool
    Gate            GateValue
    NextStage       OptionalNullableString
    Single          bool
    UnitSpecific    bool
    Wave            bool
    SwarmSettled    *bool
    UnitGate        UnitGateRhythm
    StateHash       *string
}

type ContinuationFreshness struct {
    Stage         string
    Scope         string
    Bundle        string
    DirectiveHash string
    RouteHash     string
    StateHash     *string
}

type ContinuationStep struct {
    Complete     bool
    Part         int
    Parts        int
    RulesContent []RuleContent
    Next         ContinuationClaims
}

func EncodeContinuationToken(key []byte, claims ContinuationClaims) (string, error)
func DecodeContinuationToken(key []byte, token string) (ContinuationClaims, error)
func AdvanceContinuation(claims ContinuationClaims, current ContinuationFreshness, chunks [][]RuleContent) (ContinuationStep, error)
```

最終の定数名、private helper、error型はGoの命名規則に従ってよい。`GateValue`はJSON上のboolean
`false`／`true`と文字列`"unresolved"`を型付きで区別し、zero valueは不正とする。`UnitGateRhythm`は
空をabsent、`per-stage`、`unit-end`だけを許容する。`OptionalNullableString`はabsent、JSON null、stringを
区別する。`SwarmSettled`はnilをabsent、非nilをJSON booleanとする。pointer由来の値はencode時に保持せず、
decode／step結果もcallerから独立したcopyにする。

## token wireとvalidation

- keyはexact 32 bytesとし、空・短い・長いkeyを拒否する。key生成・保存・permissionは後続のI/O責務である。
- payloadのJSON field順は固定本家の`v,s,c,i,b,d,r,a,u,k,f,g,n,x,p,w,z,q,h`とする。`n`、`z`、`q`は
  absent状態ならfield自体を省略する。
- envelopeは`p`、`m`の順とし、`m`はcanonical payload bytesのHMAC-SHA256をunpadded base64urlにする。
  envelope全体もunpadded base64urlにする。
- 文字列は既存`appendJSONString`を使い、HTML escapeを追加しない`JSON.stringify`相当のbyte列にする。
  日本語、emoji、U+2028／U+2029、control character、quote、backslashを固定fixtureで照合する。
- 全stringの不正UTF-8、versionが1以外、`NextPart < 1`、不正nullable／optional表現、gate／rhythmの
  未知値を拒否する。tokenやMACのbase64url、JSON、schema、MACが不正なら同じinvalid token境界とする。
- MACは`hmac.Equal`で比較する。base64urlは暗号化ではないため、claimsへ秘密情報やrule本文を入れない。
- `probe` fieldを含むenvelopeは拒否する。独自TTL、project path由来key、暗号化、network依存は追加しない。

## freshnessとpart進行

最初の`load-steering part=1`は後続compositionが作り、そのtokenの`NextPart`は1になる。
`AdvanceContinuation`は全bindingを比較してから次を返す。

- `NextPart < len(chunks)`: `chunks[NextPart]`を1-basedの`part=NextPart+1`として返し、`Next.NextPart`を
  1増やす。返却rulesは入力chunkと共有しない。
- `NextPart == len(chunks)`: `Complete=true`とし、後続compositionがfreshに再構成したrun-stageを
  解放できる境界を返す。rulesとNextはzero valueにする。
- `NextPart > len(chunks)`、空chunks、算術overflowは存在しないpartとして拒否する。

比較対象はStage、scope、bundle、directive hash、route hashである。`StateAware=true`ならrequired nullableの
state hashもexact一致させ、nilと空文字を区別する。`StateAware=false`なら固定本家どおりstate hashの現在値は
freshness判定へ使わない。stale errorはどのbindingが変わったかをtyped reasonまたは`errors.Is`可能なsentinelで
識別でき、error時はzero `ContinuationStep{}`を返す。

途中Markdown変更は、callerが配置Markdownを毎回読み直し、既存`BundleDigest`で現在bundleを計算して渡すことで
検出する。このAPI自身はfileを読まず、古い`RuleContent`をcacheしない。directive hashとroute hashのcanonicalな
投影方法は後続compositionが決めるため、このPRで推測しない。

## I/O、cursor、Nextとの境界

このPRは純粋部品に限定し、次を実装しない。

- `.aidlc-steering-token-key`の作成・fresh read・0600 permission・破損検査。
- `.codex/`、`aidlc/spaces/`、state、graphのfresh readとdigest算出。
- active-directive cursorの永続化、lock transaction、同一tokenの一回限り消費、並行winner。
- run-stageのcanonical composition、public `next`／`continue` command、Codex receiverへの公開。

したがって同じvalid tokenを純粋関数へ複数回渡すと同じstepを返す。replay防止を実装済みとは主張せず、
後続cursor接続までは公開経路へ出さない。既存`orchestrator.Next`はstate、audit、cursor、registryを変更しない
read-only契約のままとする。

cursorの永続schemaとtransactionは固定本家のactive-directive subsystemと密結合しているため、後続Issueで
現行Goのrecord identity、`recordlock`、`os.Root`境界と照合して別計画を作る。未対応のStop Hook probe、unit、
swarm、legacy IDE approvalを実行可能扱いにせず、claimsは互換な型を保持するだけである。

## TDD単位

各sliceは別のRED／GREEN依頼とする。最初のREDでは、最終APIをtestから呼べるtype、sentinel、function signatureと
常に未実装errorを返すcompile-only stubだけを置ける。暗号、比較、part進行をstubへ入れない。後続testが現実装で
通る場合は`ALREADY_GREEN`とし、人工的に不具合を入れない。

1. `TestContinuationTokenRoundTripMatchesFixedWire`
   - 全field、absent／null、日本語、emoji、control文字を含むknown keyのexact token fixtureとround-trip。
2. `TestContinuationTokenRejectsInvalidSchema`
   - key長、base64url、envelope、version、part、UTF-8、nullable、gate、rhythmをtable testで拒否。
3. `TestContinuationTokenRejectsTampering`
   - part／MAC改ざんと別32-byte keyをinvalid tokenとして拒否。
4. `TestAdvanceContinuationRejectsStaleBinding`
   - Stage、scope、bundle、directive、routeを1 fieldずつ変え、typed stale reasonとzero resultを確認。
5. `TestAdvanceContinuationChecksStateWhenStateAware`
   - state-awareの変更・missingを拒否し、非state-awareでは現在state hashを比較しない。
6. `TestAdvanceContinuationRejectsChangedRuleMarkdown`
   - 旧／新ruleへ実際に`BundleDigest`を適用し、途中本文変更をstale bundleとして拒否。
7. `TestAdvanceContinuationReturnsEachPartThenCompletes`
   - `ChunkRules`の複数chunkで順序、1-based part、Next claims、final、範囲外partを確認。
8. `TestAdvanceContinuationDoesNotConsumeToken`と`TestAdvanceContinuationOwnsReturnedContent`
   - 純粋APIがreplayを消費しない境界、反復決定性、input／result sliceとpointer値の非共有を確認。

targeted commandは各test名をexactに指定する。

```sh
GOTOOLCHAIN=go1.26.8 go test -count=1 -run '^TestContinuationTokenRoundTripMatchesFixedWire$' ./src/internal/steering
GOTOOLCHAIN=go1.26.8 go test -count=1 -run '^TestContinuationTokenRejectsInvalidSchema$' ./src/internal/steering
GOTOOLCHAIN=go1.26.8 go test -count=1 -run '^TestContinuationTokenRejectsTampering$' ./src/internal/steering
GOTOOLCHAIN=go1.26.8 go test -count=1 -run '^TestAdvanceContinuationRejectsStaleBinding$' ./src/internal/steering
GOTOOLCHAIN=go1.26.8 go test -count=1 -run '^TestAdvanceContinuationChecksStateWhenStateAware$' ./src/internal/steering
GOTOOLCHAIN=go1.26.8 go test -count=1 -run '^TestAdvanceContinuationRejectsChangedRuleMarkdown$' ./src/internal/steering
GOTOOLCHAIN=go1.26.8 go test -count=1 -run '^TestAdvanceContinuationReturnsEachPartThenCompletes$' ./src/internal/steering
GOTOOLCHAIN=go1.26.8 go test -count=1 -run '^TestAdvanceContinuationDoesNotConsumeToken$' ./src/internal/steering
GOTOOLCHAIN=go1.26.8 go test -count=1 -run '^TestAdvanceContinuationOwnsReturnedContent$' ./src/internal/steering
```

fixtureはGo標準libraryだけを使う。loopでは該当exact testと変更Go fileのgofmtだけを行い、全package、race、
vet、cross compile、配布E2Eを繰り返さない。

## review、final、受け入れ条件

固定base/headの独立reviewで、本家field順とschema、JSON escaping、HMAC、constant-time比較、freshness、part境界、
alias、error時zero、I/O／cursor非実装の境界を確認する。blocking finding解消後、親がread-only finalを1回だけ行う。

- steering packageと全packageのtest、race、integration tag付きtest／race。
- 通常・integrationのvet、`go mod tidy -diff`、対象fileの`gofmt -l`と`gopls check`。
- `git diff --check`。
- token codecとpure continuationを含むsteering test binaryのdarwin／linux／windows × amd64／arm64 cross compile。

受け入れ条件は、known keyのfixed wireが一致し、valid tokenが全claimsを所有してround-tripし、改ざん・別key・
不正schemaを拒否すること、freshness不一致と途中rule変更をpart返却前に拒否すること、全chunkを順に返して
final境界を一度も飛ばさないこと、I/O・global state・既存Nextを変更しないことである。

cross compileは各OS上で実行した証拠ではない。後続のkey／cursor永続化とCodex受領E2Eで各OS固有I/Oと
一回限り消費を検証する。
