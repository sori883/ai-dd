# 配信継続token用のprivate key lifecycleを実装する

- 日付: 2026-09-05（Asia/Tokyo）
- 状態: Accepted（知識供給の包括承認内）
- GitHub Issue: [#107](https://github.com/sori883/ai-dd/issues/107)
- 実装許可: [ルール・知識のAI供給を個別承認なしで完了まで進める](2026-09-05-context-delivery-autonomous-authorization.md)
- verification mode: `loop`（独立review後の`final`は親エージェントが固定HEADで1回だけ実施）

## 背景と利用者が得る結果

長い必須ルールを複数partへ分ける継続tokenは、第三者が再計算できない秘密鍵で署名する必要がある。
Issue #105／PR #106でHMAC-SHA256 token codecは実装したが、利用projectごとの鍵を安全に生成・再利用する
I/O層はまだない。

このIssueでは、workflow stateがある場合はactive Intent record直下、stateがない場合はclone-localな
`aidlc/.aidlc-sessions/`から、`.aidlc-steering-token-key`を毎回読み取る内部APIを追加する。初回だけ
暗号学的乱数32 bytesを排他的に作成し、同時生成ではwinnerの鍵へ収束する。破損、危険なleaf、I/O失敗では
利用可能な鍵を返さない。

このPRだけではtokenの一回限り消費やpublic配信を完成扱いにしない。鍵は既存codecへ直結する独立した安全基盤だが、
active-directive cursorを簡略schemaで先行させるとfixed本家のsession、attempt、delivery、revision、harness、
human-gate authorityを失い、exactly-onceを実装済みに見せる危険がある。そのため、鍵、canonical run-stage構成、
完全なcursor transactionの順に分ける。

## 固定本家2.6.123の根拠

比較対象はrepository内の固定AI-DLC 2.6.123であり、最新upstreamとの一致は主張しない。

- `core/tools/aidlc-orchestrate.ts:3427-3437`: state fileが存在すれば同じrecord directory、存在しなければ
  `aidlc/.aidlc-sessions/`へkeyを置く。
- 同`:3440-3498`: machine-local runtime key、32 bytes、canonical base64url、`wx` exclusive create、
  file mode `0600`、`EEXIST`時のwinner再読込。
- 同`:3501-3534`: keyをHMAC-SHA256 continuation tokenへ渡す。
- `tests/unit/t248-steering-content-delivery.test.ts:331-394`: 同じprojectでの再利用、別projectとの分離、
  32 bytes、canonical base64url、非Windowsで`0600`。
- 同`:480-518`: stateful workflowではrecord直下のkeyとactive-directive cursorを使用する。
- 同`:580-624`: 公開project pathから導出できるkeyではpart skip偽造を防げない。

通常の有効配置では固定本家と同じ結果にする。Go実装全体で採用済みの`os.Root` containmentと通常file検査を
維持し、root外linkやspecial fileから秘密鍵を読まない。project path由来fallback、keyring、暗黙rotate、cache、
外部moduleは追加しない。

## 実装範囲、package、API

単独implementerは次を所有する。

- `src/internal/steering/key.go`（新規）
- `src/internal/steering/key_test.go`（新規）
- `src/internal/steering/key_integration_test.go`（新規）
- Unix固有mode testが必要なら`src/internal/steering/key_integration_unix_test.go`（新規）

親エージェントは本計画、RAM索引、architecture／development文書を所有する。既存`token.go`、
`continuation.go`、`orchestrator.Next`の観測可能な契約は変更しない。

```go
var ErrInvalidContinuationKeyFile = errors.New(
    "steering: invalid continuation key file",
)

func ReadOrCreateContinuationKey(
    projectRoot *os.Root,
    recordRoot *os.Root,
) ([]byte, error)
```

`projectRoot`は必須でcaller所有、`recordRoot`はactive recordがある場合だけ渡せるcaller所有Rootとする。APIは
成功・失敗ともRootをCloseしない。`recordRoot`が非nilなら、その直下の`aidlc-state.md`をLstat／Statして、
regular fileとして存在するときだけrecord側を選ぶ。state不存在または`recordRoot == nil`ではproject側sessionを
選ぶ。stateのpermission／identity／special-file errorを「不存在」とみなしてfallbackせずfail-closedにする。

stateful key pathはrecord Root相対`.aidlc-steering-token-key`、session key pathはproject Root相対
`aidlc/.aidlc-sessions/.aidlc-steering-token-key`である。session directoryだけを必要時に`0777 & umask`相当で
`MkdirAll`し、内部で開いたchild Rootだけを内部でCloseする。

## key file契約とcommit境界

key fileは次の1行だけである。

```text
<32-byte keyのunpadded base64url>\n
```

- 新規keyは`crypto/rand.Reader`からexact 32 bytesを`io.ReadFull`で得る。
- fileは`O_WRONLY|O_CREATE|O_EXCL`、mode `0600`で作る。既存fileをtruncateしない。
- 全byte writeとFile Closeが成功して初めて生成成功とする。失敗時はkeyを返さず、破損fileを自動修復しない。
- `EEXIST`は、別process／goroutineが生成に勝ったものとして既存fileを1回freshに読み直す。loserが生成した
  独自keyは返さない。winnerがまだ書込途中なら一時的な破損としてfail-closedにし、callerのfresh `next`再実行に委ねる。
- 既存fileは毎回single-descriptorで読み、regular fileであること、bounded size、全byte、Closeを確認する。
- UTF-8として前後のECMAScript whitespaceを既存steering判定でtrimし、RawURLEncodingでdecodeした結果がexact
  32 bytes、再encodeがtrim済み文字列と完全一致する場合だけ受理する。padding、別alphabet、余分な文字、
  invalid UTF-8、空、短い／長いkeyを拒否する。
- 既存keyをchmod、rotate、cacheせず、呼出し間の正当なfile差替えは次回readへ反映する。
- 返却`[]byte`はcaller所有で、production package globalへ保持しない。
- corrupt／nonregular／root外symlink、short write、read／write／Close errorではzero keyとerrorを返す。

native filesystemの失敗を再現するprivate helperには、production公開interfaceやmutable globalを追加せず、具体的な
private operation集合を引数として渡す。通常APIは実operationを直接選ぶ。

## 後続compositionとの接続境界

今回接続できる範囲は次だけである。

```text
recordまたはsessionのprivate key file
  → ReadOrCreateContinuationKey
  → EncodeContinuationToken / DecodeContinuationToken
```

初回配信では、後続facadeが`orchestrator.Next`のread-only結果を受け、配置済み`.codex/`、active Space、state、
graph、rules、knowledge、stage Markdown、artifactsをfreshに読み、canonical run-stage／route／bundleを計算する。
今回のAPIでkeyを得てfirst-part tokenを作った後、active-directive cursorをcommitできた場合だけpart 1をpublishする。

continuationではfresh keyでtokenをdecodeして全sourceとhashを再構成し、`AdvanceContinuation`後、cursorの
`continue_token_sha256`が提示tokenと一致するときだけ次partまたはfinal markerへ原子的に進める。cursor commit後だけ
directiveをpublishする。今回のkey APIを`orchestrator.Next`内部やpublic CLIへ直接接続しない。

現在不足するcanonical run-stage DTO／wire／hash、route projection、stage content reader、consume present／absent、
active-directive v2 markerとtransaction、publication facadeは後続Issueで実装する。特に現行`graph.Stage`をそのまま
JSON化して固定本家のroute hashとみなさない。

## TDD単位

各項目は同一implementerへ別のRED依頼、親のexact再実行とhash受入、別のGREEN依頼として渡す。最初のREDだけ、
`key.go`へ予定sentinel、function signature、常に未実装errorを返すcompile-only stubを置ける。random、path選択、
I/Oをstubへ入れない。後続testが先行実装で通る場合は`ALREADY_GREEN`とし、人工的なfailureを作らない。

1. `TestReadOrCreateContinuationKeyCreatesRecordKey`
   - stateがあるとrecord直下だけにcanonical 32-byte keyを作り、既存codecでencode／decodeできる。
   - `GOTOOLCHAIN=go1.26.8 go test -tags=integration -count=1 -run '^TestReadOrCreateContinuationKeyCreatesRecordKey$' ./src/internal/steering`
2. `TestReadOrCreateContinuationKeyUsesSessionFallback`
   - state不存在ではrecord側へ作らず、projectのsession pathへ作る。
   - `GOTOOLCHAIN=go1.26.8 go test -tags=integration -count=1 -run '^TestReadOrCreateContinuationKeyUsesSessionFallback$' ./src/internal/steering`
3. `TestReadOrCreateContinuationKeyReusesFreshCanonicalFile`
   - 既存keyを上書き／chmodせず、呼出し間の正当な差替えを次回readへ反映する。
   - `GOTOOLCHAIN=go1.26.8 go test -count=1 -run '^TestReadOrCreateContinuationKeyReusesFreshCanonicalFile$' ./src/internal/steering`
4. `TestReadOrCreateContinuationKeyRejectsCorruptFile`
   - padding、長さ、alphabet、余分な文字、invalid UTF-8、emptyをtable testで拒否する。
   - `GOTOOLCHAIN=go1.26.8 go test -count=1 -run '^TestReadOrCreateContinuationKeyRejectsCorruptFile$' ./src/internal/steering`
5. `TestReadOrCreateContinuationKeyRereadsConcurrentWinner`
   - createが`fs.ErrExist`となったloserがwinnerのcanonical keyを読み、独自keyを返さない。
   - `GOTOOLCHAIN=go1.26.8 go test -count=1 -run '^TestReadOrCreateContinuationKeyRereadsConcurrentWinner$' ./src/internal/steering`
6. `TestReadOrCreateContinuationKeyPropagatesIOFailure`
   - random／short write／write／Close／winner read errorでzero keyと原因を返す。
   - `GOTOOLCHAIN=go1.26.8 go test -count=1 -run '^TestReadOrCreateContinuationKeyPropagatesIOFailure$' ./src/internal/steering`
7. `TestReadOrCreateContinuationKeyIntegrationRejectsUnsafeLeaf`
   - nil required Root、nonregular state／key、root外symlinkを拒否し、Rootを閉じず、別pathを変えない。
   - `GOTOOLCHAIN=go1.26.8 go test -tags=integration -count=1 -run '^TestReadOrCreateContinuationKeyIntegrationRejectsUnsafeLeaf$' ./src/internal/steering`
8. `TestReadOrCreateContinuationKeyIntegrationUses0600`
   - `integration && (darwin || linux)`で新規keyのpermissionを確認する。
   - `GOTOOLCHAIN=go1.26.8 go test -tags=integration -count=1 -run '^TestReadOrCreateContinuationKeyIntegrationUses0600$' ./src/internal/steering`

loopでは各exact testと変更Go fileのgofmtだけを実行し、全package、race、vet、cross compileを繰り返さない。

## review、final、受け入れ条件

固定baseと実装headの独立reviewで、path選択、`crypto/rand.Reader`、32-byte key、exclusive create、EEXIST winner、
canonical base64url、0600、bounded regular-file read、Root ownership、error時zero、cache／fallback／cursor／public CLIの
非混入を確認する。

blocking finding解消後、親が固定headでread-only finalを1回だけ実施する。

- 通常／integrationの全package test、race、vet。
- `go mod tidy -diff`、`gofmt -l`、`gopls check`、`git diff --check`。
- CLIとsteering test binaryをdarwin／linux／windows × amd64／arm64へcross compile。

native filesystem integrationはこのPRで実施する。cross compileは各OSで実行した証拠ではなく、public配信経路がないため
Codex配布E2Eは後続へ残す。

受け入れ条件は、正しいrootへprivate keyを初回だけ作り毎回freshに再利用できること、並行loserがwinnerへ収束すること、
破損・unsafe leaf・全I/O failureで署名可能なkeyを返さないこと、caller Rootと既存workflow stateを変更しないこと、
existing codecとの接続が確認でき、replay-safeやpublic配信を実装済みと主張しないことである。

## リスクと停止条件

最大のリスクはstate inspection errorを不存在と誤認してsession fallbackすること、concurrent loserが独自keyを返すこと、
corrupt keyをrotateして途中配信を黙って無効化すること、cursor未実装の状態をreplay-safeと見せることである。

正常なrecord／session以外の永続場所、簡略化した独自cursor schema、project path由来key、credential store、外部module、
追加権限が必要になった場合は停止する。現時点では固定本家と既存Go boundaryから一意に実装できるため、包括承認内で進める。
