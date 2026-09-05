# Codexのcontext読込をGoの安全境界へ移す

- 日付: 2026-09-05（Asia/Tokyo）
- 状態: Implemented（ユーザーが直接承認。Issue #115の独立review修正）
- 対応Issue: [#115 Codex receiverで配信本文を実読込する](https://github.com/sori883/ai-dd/issues/115)
- 置換対象: [Codex receiverで配信本文を実読込する](2026-09-05-codex-receiver-read-plan.md)のshell直接読込、
  live prompt、短いfixtureに関する設計
- 前提: [Codex向け配信transactionと公開next／continueを接続する](2026-09-05-delivery-publication-plan.md)
- 基準: リポジトリ固定AI-DLC 2.6.123

## 背景

Issue #115の最初の実装では、Codex skillが`aidlc next`／`aidlc continue`で受け取った
`run-stage`のpathをshellから直接読んだ。通常系のlive Codex実読込は成功したが、独立reviewで次を確認した。

1. `consumes_absent[].expected:false`は「本来必要な入力が予想外に存在しない」ことを表すが、skillがこの値を
   判定しないため、必須入力が欠けたまま`context ready`へ進める。
2. project内に見えるpathがsymlinkや読込中の差替えでproject外を指しても、shellへの注意書きだけでは
   読込を機械的に阻止できない。
3. live testのpromptがskillの読込順と反復処理を重ねて指示していたため、skill単独の回帰を隠せる。
4. 短い一行fixtureでは全文読込を、実行canaryのないStageでは「読んだが実行していない」ことを十分に示せない。

これは事故が発生した証拠ではなく、正常系だけでは検出できない契約上の欠陥である。次の作業項目で本番Stageを
縦に通す前に、receiverをfail-closedな知識入力面へする。

## ユーザー承認

ユーザーは、問題と必要性、以下の公開`aidlc read-context`方式が項目1の範囲内であること、固定本家の
shell直接読込から安全なGo境界へ移す意図的差分であることの説明を受け、2026-09-05に「はい、では対応して」
と実装を直接承認した。

この承認は新しい公開CLI契約、安全なfile読込、skill・test・文書の修正、Issue／PR、品質gate後の自律mergeを
許可する。Stage実行、artifact生成、review／sensor／report、人間承認、一般installer／update、外部Go module、
追加credentialや追加権限は許可範囲に含めない。

## 採用する公開契約

Codexは`run-stage`のpathをshellで直接読まず、次のread-only commandだけを呼ぶ。

- 開始: `aidlc read-context --project-dir .`
- 継続: `aidlc read-context continue "<opaque read token>" --project-dir .`

開始にtokenはなく、継続は直前の成功応答が返したtokenを解析・再構成せず一つの引数として渡す。
file path、slot、partはcallerから指定できない。Go側が必ず、全inline context、Stage file、存在する全consumeの
順に一つずつ選ぶ。

成功時はstdoutだけへcanonicalな一行JSONを返し、改行を含めて8192 bytes以下にする。主なfieldは次とする。

- `kind`: `context-chunk`
- `stage`: activeなStage slug
- `slot`: `inline-context`、`stage-file`、`consume`のいずれか
- `index`: 同じslot内の1-based file順
- `part`／`parts`: 同じfile内の1-based chunk順と総数
- `content_sha256`: 読込んだfile全体のSHA-256
- `text`: UTF-8の本文chunk
- `read_continue_token`: 次chunkがある場合だけ返すopaque token
- `complete`: 全contextを返し終えた最終chunkだけ`true`

JSON escaping、field、token、末尾改行を含めて上限内とし、UTF-8 runeの途中で分割しない。同じtokenの再送は
同じchunkを返すread-only replayとし、cursorを永続化しない。成功も失敗もstate、audit、artifact、active markerを
変更しない。

syntax errorはstdoutを空にしてstderrへ説明しexit 2、実行時の不整合はstdoutを空にしてstderrへ説明しexit 1とする。
実行時の不整合には、marker不在・古い／改ざんtoken・未配信または消費済みdirective・予想外の必須入力欠落・
unsafe file・不正UTF-8・I/O失敗を含む。失敗時に次のfileへ進めず、自動`next`や`report`もしない。

8192-byte上限はCodexのtool出力で扱いやすい保守的な初期値であり、Codex CLIの保証値とは断定しない。修正後の
live E2Eで切捨てなく全partを受け取れることを検証する。より小さい上限が必要と判明した場合は、新しい値を
推測で採用せず契約を再検討する。

## active directiveへの拘束

各chunkは`recordlock.With`のtransaction内で次を検証してから返す。

1. `ComposeRunStageWithGuard`で現在の`run-stage`をfreshに再構成する。
2. `.aidlc-active-directive.json`を読み直し、version 2、Codex harness、issued／delivered、rehydrate不要、
   settled attempt、未消費・未supersedeであることを確認する。
3. project、space、intent、state、marker revision、現在のwire digestをmarkerへ照合する。
4. 全fileを開く前に`consumes_absent`を検査する。`expected:false`が一つでもあれば停止し、
   `expected:true`は予定された不在として読込対象から外す。
5. Go側がtokenに拘束された次のslot／file／partを一つだけ選び、安全に全文を読む。
6. 出力直前にmarkerを再読し、revision、kind、wire digestが変わっていないことを確認する。

現在の新規active markerは`ResultSHA256`を省略する場合があるため、配信facadeは新規／継続いずれのpublicationでも
canonical wireのdigestとresult revisionを記録する。既存のdigestなしmarkerは`read-context`で拒否し、利用者が
改めて`aidlc next`を呼ぶとfresh markerへ更新できる。schema fieldは既存なので永続形式のversion migrationは行わない。

read tokenは既存record private keyからdomain-separated HMACで認証し、project、space、intent、marker revision、
wire digest、Stage、次slot／file／part、同じfileの`content_sha256`を拘束する。tokenはopaqueで永続化しない。

## file安全性

- project root相対の通常fileだけを許可し、absolute path、`..`、directory、device、FIFO、socketを拒否する。
- `os.Root`でancestor symlinkを含むproject外escapeを拒否し、leaf symlinkも明示的に拒否する。
- confined openの前後でpath情報と開いたdescriptorを照合し、読込中の差替えやfile内容変更を拒否する。
- UnixではFIFO等によるblockを避けるためnonblocking openを使い、Windows／その他の対象OSはbuild-tag別の
  標準ライブラリ実装にする。
- bind mountは`os.Root`だけでは防げないため、利用者が管理するproject mountを信頼境界とする。
- 複数file全体のatomic snapshotは作らない。各chunkでfresh directiveを再検証し、同一fileの複数chunkは
  content digestで変更を検出する。

## skillとE2Eの修正

product skillは、全`load-steering`本文をactive bundleへ順番に蓄積し、tokenがある限り反復する。その後は
`read-context`を`complete:true`まで反復し、読込失敗時に停止する。shellによるpath直接読込は削除する。
通常成功は`context ready`、検証用schemaが明示された場合だけread receiptを返す。

live promptは「このprojectで`$aidlc` skillを明示利用し、skillが定める検証receiptだけをschemaどおり返す」
という要求だけにし、読込順、反復、path処理を重ねて教えない。

fixtureは予測不能なbegin／middle／endを持つ長い複数sectionとし、少なくとも一fileを複数context chunkへ分割する。
Stage本文には、実行されれば検出できるcanaryを置く。実行前後でstate、audit、artifactをsnapshotし、配信で必要な
key／active marker以外が変わらず、canary出力が存在しないことを確認する。

最初のlive成功は旧skill／prompt／fixtureに対する履歴として保持するが、本変更後の完了証拠には使わない。
non-live修正、独立review、対象差分の安定後、Codex account利用量を消費し得るlive commandを再実行する直前に、
ユーザーへ別途明示確認する。

## 本家との意図的な差分

- 本家の挙動: 固定AI-DLC 2.6.123のCodex skillは、directiveが示すpathをCodexのfile／shell toolで直接読む。
- 採用する挙動: Go版では同じcontext本文を、active directiveへ拘束された`aidlc read-context`だけから返す。
- 理由: 必須入力欠落、symlink escape、読込中の差替えをLLMへの注意書きではなく決定論的に拒否するため。
- 影響: Codexは任意pathを読めず、bounded JSONをtokenで反復する。Stage内容や順序は変えず、より厳しく停止する。
- 確認範囲: リポジトリ固定snapshot 2.6.123の配置済みCodex skillと分析索引。最新upstreamとの一致は未確認。

## 単独writer所有権

1人の`go_tdd_implementer`が1 work unitで次を所有する。

- `src/internal/delivery/`: safe context reader、read token、active marker publicationとtest。
- `src/internal/cli/`: `read-context` grammar、JSON stdout、exit codeとtest。
- `src/cmd/aidlc/`: project root adapter、main wiring、fresh／live integration test。
- `src/harness/codex/skills/aidlc/`: product skillと通常test。
- `docs/architecture.md`、`docs/development.md`、`docs/e2e-testing.md`、本記録とRAM索引。

既存user変更を戻さず、Issue／PR操作とlive Codex実行は親が行う。外部Go moduleは追加しない。

## TDD work unit

`work_unit_id=codex-safe-context-read`として、次を順に実装する。新しい公開APIをrunnableなREDへ到達させるため、
testが参照する型、constant、function signatureだけのcompile-only scaffoldを許可する。

1. `active-run-stage-binding`
   - 新しいpublicationがwire digest／revisionをmarkerへ保存し、readerが現在のissued run-stageだけを受け入れる。
   - test: `src/internal/delivery/facade_test.go`、`src/internal/delivery/context_read_test.go`
   - command: `go test -count=1 -run 'Test(Next|Continue|ReadContext).*' ./src/internal/delivery`
2. `absent-input-preflight`
   - `expected:false`を最初のfile open前に拒否し、`expected:true`は予定された不在として扱う。
   - test: `src/internal/delivery/context_read_test.go`
   - command: `go test -count=1 -run 'TestReadContext.*Absent' ./src/internal/delivery`
3. `ordered-bounded-chunks`
   - inline全件→Stage→consume全件、UTF-8 rune-safe、8192-byte以内、token改ざん／stale／content変更拒否、replay同値。
   - test: `src/internal/delivery/context_read_test.go`
   - command: `go test -count=1 -run 'TestReadContext.*(Order|Chunk|Token|Replay|Change)' ./src/internal/delivery`
4. `root-confined-files`
   - 通常fileを読める一方、不正UTF-8、leaf／ancestor symlink、差替え、FIFO／special fileをblockせず拒否する。
   - test: `src/internal/delivery/context_read_test.go`とOS別integration test
   - command: `go test -count=1 -run 'TestReadContext.*(File|Symlink|Race|FIFO|UTF)' ./src/internal/delivery`
5. `public-cli`
   - 開始／continue構文、opaque token、stdout／stderr、exit 0／1／2、root cleanupを公開入口から保証する。
   - test: `src/internal/cli/*_test.go`、`src/cmd/aidlc/*_test.go`
   - command: `go test -count=1 -run 'Test.*ReadContext' ./src/internal/cli ./src/cmd/aidlc`
6. `receiver-skill`
   - skill単独で全rule蓄積と両continue loopを指示し、直接file read、Stage実行、reportを行わない。
   - test: `src/harness/codex/skills/aidlc/skill_test.go`
   - command: `go test -count=1 ./src/harness/codex/skills/aidlc`
7. `fresh-receiver-journey`
   - fresh projectの長い多section本文、複数chunk、最小live prompt、Stage canary、state／audit／artifact不変を検証する。
   - test: `src/cmd/aidlc/codex_receiver_integration_test.go`
   - command: `go test -tags=integration -count=1 -run '^TestCodexReceiver(FreshPlacementJourney|ReadsDeliveredContext)$' ./src/cmd/aidlc`

work unit末尾では上記non-live command、影響4 packageのtest、変更Go fileの`gofmt`、skill validator、
`git diff --check`を実行する。loop中に全package、race、vet、cross compile、live Codexを実行しない。

## reviewとfinal

親はwork unitの全差分とtargeted testを確認し、固定base/headで独立reviewを行う。blocking findingは一つの
repair work unitへ戻し、回帰testを先行する。差分が安定した後だけ、read-only finalとして次を一度実行する。

- `go test -count=1 -shuffle=on ./...`
- `go test -tags=integration -count=1 -shuffle=on ./...`（live環境変数なし）
- 通常／integrationの`go test -race`と`go vet`
- `go mod tidy -diff`、`gofmt -l src`、変更Go fileの`gopls check`、`git diff --check`
- skill validator
- darwin／linux／windows × amd64／arm64のCLIと対象test binary cross compile
- fresh配置journey

live E2Eはユーザーの再承認後に一度だけ実行し、その結果をRAM／PRへ記録する。live証拠を記録した変更後は、
Go実装へ変更がないことを確認し、文書差分を含む現在headで必要なread-only checkを更新する。

## 受け入れ条件

- 必須入力が予想外に欠けている場合、fileを一つも読まずに停止する。
- Codexは任意path／slotを指定できず、active directiveが選んだ通常fileだけをproject内から読む。
- 全contextを固定順・全文・UTF-8のまま、8192-byte以下のcanonical JSON chunkで受け取れる。
- tokenの改ざん、stale、同一file変更を拒否し、同じtoken replayは同じchunkを返す。
- 読込成功／失敗ともworkflow state、audit、artifactを変更せず、Stageを実行しない。
- product skill単独で両continue loopが成立し、live promptがskillの欠落を補わない。
- 通常／integration／race／vet／format／lint／cross compile／fresh E2E、独立review、GitHub checksが成功する。
- 再承認されたlive Codex 1回が全文、順序、複数chunk、Stage未実行を証明する。
- PRをdefault branchへmergeし、Issue #115をcloseして項目1を完了する。

## リスクとrollback

主なリスクは、8192-byte境界でもlive tool出力が切り捨てられること、OS別file openの意味が異なること、
marker再検証とfile差替えの間に残る競合である。bounded output、cross-platform test、descriptor／path再照合、
live E2Eで抑える。bind mountと複数file全体のatomic snapshotは明示したtrust境界として残る。

問題時は本Issueのsafe reader、CLI wiring、skill／testを同じPR単位でrevertできる。PR #114の配信cursorや
state schema migrationは不要で、既存の`next`／`continue`契約を壊さない。

## 実装証拠（Issue #115）

承認済み契約を`work_unit_id=codex-safe-context-read`の一つのloop work unitとして実装した。新規publicationは
`.aidlc-active-directive.json`のsettled attemptへcanonical run-stage wireの`result_sha256`と`result_revision`を保存し、
`ReadContext`／`ContinueContext`はfresh composition、issued／delivered marker、revision、wire digest、state、identityを同じ
record lock内で再検証する。digestなしmarker、consumed／superseded marker、改ざん・stale tokenは拒否し、次回`next`でfresh回復する。

readerは全`consumes_absent`をfile open前にpreflightし、`expected:false`を拒否、`expected:true`を読込対象から除外する。inline、
stage、consumeをこの順に全文読込し、UTF-8 rune境界のbounded chunkをcanonical JSON（改行込み8192 bytes以下）へ返す。record private
keyのdomain-separated HMAC tokenはproject、space、intent、marker revision、wire digest、stage、slot、file、part、content
digestを拘束し、同じtokenのreplayはread-only同値となる。`os.Root`相対のregular non-symlinkだけを許可し、ancestor／leaf symlink、
FIFO・special file、不正UTF-8、descriptor/path/content raceをfail-closedにし、Unixではnonblocking openを使う。

公開CLIは`aidlc read-context [continue <opaque-token>] [--project-dir <path>]`を実装し、path／slot／partをcallerへ公開しない。
成功はstdout canonical JSON一行、syntax errorはstdout空・stderr・exit 2、reader／I/O／root cleanup failureはstdout空・stderr・exit 1。
receiver skillはshellによるraw path readを削除し、`read-context`を`complete:true`まで反復する。fresh non-live journeyはrepository外
projectへskillをbyte-identical配置し、予測不能で複数chunkのcontext本文、inline→stage→consume順、read前後snapshot不変、Stage canary
不在を確認する。live Codex testは`AIDLC_CODEX_EXEC_LIVE=1`が外部で設定された場合だけ実行し、test自身は設定しない。

親reviewの追加repair work unit `codex-safe-context-read-parent-repair`では、verification-only receiptのschema意味をskillへ固定した。
`rules`は受領順の各`rules_content` entryの末尾非空行、`inline_context`／`stage_file`／`consumes`はそれぞれslot/index/part順で
各fileの全chunkを連結した全文を表す。live promptはskill invocationと定義済みreceipt/schema要求だけへ縮小し、routing、読込順、stop、
canaryの指示を重ねない。fixtureのstage・consumeは予測不能なBEGIN/MIDDLE/ENDを持ち、stage実行時だけproject rootの
`stage-execution-canary.txt`へrandom sentinelを書き込む明示指示を含めた。live前後snapshotはdirectory mtimeを無視してregular fileの
mode/bodyだけを比較し、transport上必要な`.aidlc-active-directive.json`と`.aidlc-steering-token-key`だけを除外する。Codex liveはこのrepair後
未実施で、環境変数なしのintegration testはskipする。

また、read-contextのcontinuation key取得は新設したread-only `steering.ReadContinuationKey`へ統合した。既存key lifecycleと同じ
4 KiB+1 bounded read、regular non-symlink、UTF-8、ECMAScript whitespace trim、canonical unpadded base64url、exact 32-byte検証を再利用し、
read-context中のkey／session directory作成・変更を許可しない。

loop targeted evidence:

```text
go test -count=1 -run 'Test(Next|Continue|ReadContext).*' ./src/internal/delivery
go test -count=1 -run 'TestReadContext.*Absent' ./src/internal/delivery
go test -count=1 -run 'TestReadContext.*(Order|Chunk|Token|Replay|Change)' ./src/internal/delivery
go test -count=1 -run 'TestReadContext.*(File|Symlink|Race|FIFO|UTF)' ./src/internal/delivery
go test -count=1 -run 'Test.*ReadContext' ./src/internal/cli ./src/cmd/aidlc
go test -count=1 ./src/harness/codex/skills/aidlc
go test -tags=integration -count=1 -run '^TestCodexReceiver(FreshPlacementJourney|ReadsDeliveredContext)$' ./src/cmd/aidlc
go test -count=1 -run 'TestReadContext.*Key' ./src/internal/delivery
go test -count=1 -run '^TestReadContinuationKeyIsReadOnlyAndUsesCanonicalBounds$' ./src/internal/steering
env -u AIDLC_CODEX_EXEC_LIVE go test -tags=integration -count=1 -run 'TestCodexReceiver(LivePrompt|LiveCommand).*' ./src/cmd/aidlc
env -u AIDLC_CODEX_EXEC_LIVE go test -tags=integration -count=1 -run '^TestCodexReceiver(FreshPlacementJourney|Fixture.*|StableSnapshot.*|ReadsDeliveredContext)$' ./src/cmd/aidlc
go test -count=20 -run '^TestReadContextTokenTamperReplayAndContentChange$' ./src/internal/delivery
```

本家固定snapshot AI-DLC 2.6.123はdirective pathをCodex file／shell toolから直接読む。本実装は承認済み意図的差分として、同じ本文を
active directiveへ拘束されたGo `read-context`からのみ返す。理由は必須入力欠落、symlink escape、読込中差替えをLLMへの注意書きでなく
決定論的に拒否するためで、利用者は任意pathを指定できず、Stage本文と順序は維持される。
