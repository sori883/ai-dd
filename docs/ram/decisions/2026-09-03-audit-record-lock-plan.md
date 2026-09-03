# 最小audit ledgerとrecord lockの実装計画

- 日付: 2026-09-03
- 状態: Accepted
- GitHub Issue: [#77](https://github.com/sori883/ai-dd/issues/77)
- 実装許可: [薄いライフサイクルマイルストーン](2026-09-03-thin-lifecycle-milestone.md)とロードマップ包括承認内

## 現状と目的

state patcherとatomic update writerにより、単一processは安全なreplacement bytesを構築・保存できる。しかし複数processが同じIntentを同時更新するとlost updateが起こり得る。また、承認とStage遷移を判断可能にするappend-only audit receiptの保存境界がまだない。

本変更では、project root・space・intentから一意なrecord identityを作るcross-process lockと、同じidentityのGuardを要求する最小audit ledgerを追加する。後続transactionは、一つのcritical sectionでstateを再読込し、auditを先にappendしてからstateを置換できる。

## 設計

### record lock

- identityはcanonical project path、space slug、intent record名の組で構成し、曖昧・空・path separator・control characterを拒否する。
- project pathはsymlinkを解決できる場合に解決し、platformのpath比較規則に合わせて正規化したidentityをhashし、system temp配下のrecord単位lock directoryへ写像する。
- `context.Context`を受け、`mkdir`によるcross-process排他を有限retryする。cancel、timeout、non-`ErrExist`を原因付きで返す。
- owner token、PID、開始時刻をlock directoryへ保存し、release時にtoken一致を確認する。別ownerのlockやfileを削除しない。
- callbackへidentity-bound Guardを渡す。後続のnested処理はGuardを明示的に再利用し、同じprocessでlockを再取得しない。Goの並行goroutineを暗黙のreentrant callerとして扱わない。
- callback errorとrelease errorは`errors.Join`し、panic時もreleaseしてからpanicを再送出する。

### audit ledger

- 固定2.6.123と同じ`audit/<normalized-host>-<clone-id>.md` shardをrecord root配下に使用する。
- clone IDはproject rootの`aidlc/.aidlc-clone-id`に12文字の小文字hexとして保存し、既存値を検証する。初回競合はexclusive createと再読込で一つのon-disk tokenへ収束させる。
- shard名とpath componentをallowlist検証し、`audit` directory・shard leafのsymlinkやnon-regular targetを拒否する。
- 本マイルストーンで必要なeventだけをallowlistする: `HUMAN_TURN`、`STAGE_AWAITING_APPROVAL`、`GATE_APPROVED`、`GATE_REJECTED`、`STAGE_REVISING`、`STAGE_COMPLETED`、`PHASE_COMPLETED`、`PHASE_VERIFIED`、`PHASE_STARTED`、`STAGE_STARTED`、`WORKFLOW_COMPLETED`。
- event heading、UTC RFC3339 timestamp、`**Event**`、fieldをcanonical Markdown blockとしてrenderする。field keyは固定pattern、`Timestamp`と`Event`は予約し、値の全line terminatorをliteral `\\n`へescapeする。
- batch全体を事前検証・renderしてから、identity一致かつ現在heldなGuardのもとで一度のappend sessionに書く。空fileには`# AI-DLC Audit Log` headerを先に書く。
- 全byte writeを保証し、short/no-progress、open、write、close failureを原因付きで返す。append-only fileの途中writeはrollback可能と主張しないため、後続state transactionはaudit error時にstateを変更しない。
- caller-owned project/record RootをCloseしない。public CLI、human authority minting、state mutation、audit shard merge/read modelは追加しない。

## 対象ファイル

- 新規`src/internal/recordlock/lock.go`
- 新規`src/internal/recordlock/lock_test.go`
- 必要なplatform別fileとintegration test
- 新規`src/internal/audit/ledger.go`
- 新規`src/internal/audit/ledger_test.go`
- 新規`src/internal/audit/ledger_integration_test.go`
- `docs/architecture.md`
- `docs/development.md`
- `docs/ram/README.md`
- 本計画

実装担当はこれらを単独所有し、他の作業者の変更をrevertしない。

## TDD slice

1. record identity validation、canonical path、同一identityの同一lock path、異なるspace/intentの分離を固定する。
2. acquire、owner初期化、contention retry、context cancel、owner mismatch、callback/release error、panic releaseをRED/GREENにする。
3. goroutineとhelper subprocessで同一recordが直列化され、異なるrecordが独立することを確認する。
4. clone IDの既存値、初期生成、競合収束、不正値、project Root ownershipとhost正規化を確認する。
5. event/field validation、heading、timestamp、改行escape、header bootstrap、複数event順序をexact bytesで確認する。
6. Guardのidentity不一致、未保持・release済みGuard、audit directory/shard symlink・non-regular、short/no-progress writeを拒否する。
7. 実`os.Root`で複数appendが欠落せず、同一recordの並行appendがraceなく直列化され、Rootを継続利用できることを確認する。

loopでは`recordlock`と`audit`のtargeted unit/integration testだけを実行する。全package、race、vet、cross buildは独立review後のfinalへ集約する。

## 受け入れ条件

- 同一record identityのwriterをprocess間で直列化し、異なるrecordを同じlockへ衝突させない。
- cancel・取得失敗・owner mismatchで他ownerのlockを削除しない。
- audit appendは同じidentityのheld Guardがなければ拒否する。
- canonical headerとevent blockをappend順どおり保存し、field/event line forgeryを拒否する。
- audit append失敗を呼び出し元へ返し、後続がstate更新前に停止できる。
- per-clone shard名とclone IDを固定形式で扱う。
- caller-owned RootをCloseせず、外部Go moduleを追加しない。

## 互換性・リスク

固定AI-DLC 2.6.123のrecord/audit identity、per-clone shard、Markdown block、audit-firstの前提へ準拠し、新しい意図的差分は採用しない。Goでは自動reentrancy depthの代わりにidentity-bound Guardをnested処理へ明示的に渡す。これはgoroutineを誤ってnested callとして扱わず、同じcritical sectionを再利用する内部構造上の選択であり、外部挙動は同じrecordの直列化である。

process強制終了後のstale owner自動reap、全shard merge-sort reader、監査rollback、`fsync`は本sliceの保証外とする。authority-bearing eventはこの内部ledgerだけが受けるが、信頼済み`HUMAN_TURN` adapterは後続でもpublic化せず、agentがCLI引数からhuman authorityをmintする経路は作らない。
