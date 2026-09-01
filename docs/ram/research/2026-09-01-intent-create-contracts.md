# Intent作成coreの参照契約

- 日付: 2026-09-01
- 状態: Current for local AI-DLC 2.6.123 snapshot
- 関連: [承認済み計画](../decisions/2026-09-01-intent-create-core-plan.md)、
  [Intent切替の参照契約](2026-09-01-intent-switch-contracts.md)

## 確認範囲

analysis indexと`core/tools/aidlc-version.ts`でローカルsnapshotのversion 2.6.123を確認した。
technical_researcherがauthored core、canonical Codex dist、配置済みCodex distを静的に比較し、
作成処理に関係する次のfileが3形式で一致することを確認した。

| file | SHA-256 |
| --- | --- |
| `aidlc-lib.ts` | `3b0ff3dda8abfbead8ff7e4a61b2ce334d1d3e78977995d1bd249668fc99db09` |
| `aidlc-utility.ts` | `c0a27b957f121d45c47f104eed1f7648406fc4756679e2edb71679fc046d4458` |

command、hook、installerは実行せず、最新upstream全体との一致も主張しない。親セッションには
SerenaとContext7が公開されなかったため、`rg`、`gopls`、ローカル原典で補完した。外部libraryや
APIを採用しないため、Context7から補う仕様はなかった。

## 決定論的core契約

本家の`createIntent(projectDir, label, space, scope?, repos?, sessionId?)`は、次の順で
`{uuid, slug, dirName, recordDir, space}`を作る。

1. UUIDv7を生成する。
2. labelを24文字上限でslugifyし、予約語を拒否する。
3. UTCの`YYMMDD-slug`をbaseとして、衝突時は`-2`から`-999`を探索する。
4. record directoryとheader-onlyの`aidlc-state.md`を作る。
5. `intents.json`へ`uuid`、`slug`、`dirName`、任意の`scope`・`repos`、
   `status: "in-flight"`を追記する。
6. shared `active-space`を不在時だけ補完し、対象spaceの`active-intent`を更新する。
7. session bindingをbest-effortで更新する。

UUIDv7は48-bit Unix millisecond、version 7、暗号学的random tail、RFC variantであり、同一ms内の
単調順は保証しない。slugはASCII英数字以外をcollapsed hyphenへ変換し、空なら`intent`、先頭が
英字でなければ`intent-`を付ける。予約語は`help`、`list`、`switch`、`create`、`archive`、
`rename`、`show`、`birth`である。state stubは正確に`# AI-DLC State Tracking\n`である。

`scope`と`repos`のtrim、検証、dedupe、sortはpublic handlerの責務であり、coreは決定済みの値を
保存する。空のreposはfield自体を省略する。shared `active-space`を指定spaceへ切り替えず、cursorが
不在ならshared selectionのfallback値だけをmaterializeする。

主要根拠:

- `docs/実装_aidlc-workflows/core/tools/aidlc-lib.ts:941-951`
- `docs/実装_aidlc-workflows/core/tools/aidlc-lib.ts:2147-2248`
- `docs/実装_aidlc-workflows/core/tools/aidlc-lib.ts:2328-2377`
- `docs/実装_aidlc-workflows/core/tools/aidlc-lib.ts:2491-2527`
- `docs/実装_aidlc-workflows/core/tools/aidlc-lib.ts:3876-3948`
- `docs/実装_aidlc-workflows/core/tools/aidlc-utility.ts:5440-5700`

## Registry、lock、部分失敗

本家は`intents.json`をread-modify-writeし、writer固有のsibling tempを排他的に作ってrenameする。
JSONは2-space indentと末尾LFである。renameはhalf-writeを防ぐがlost updateを防がないため、全ての
registry mutationはcaller-held WORKSPACE lockを必要とする。

WORKSPACE lock identityは、可能ならrealpathした絶対project path（Windowsはlowercase）とNUL、
`__workspace__`の連結である。そのMD5 hex先頭8文字からsystem temp配下の
`.aidlc-audit-<hash>.lock`を決める。Intent作成handlerは100ms間隔、600 retries、最大約60秒で
owner-stamped directory lockを待つ。本家はowner PID、generation、random token、reap gateを使い、
死んだgenerationだけを自動回収する。

本家はrollbackしない。stub失敗ではdirectory、registry失敗ではdirectoryとstub、cursor以降の
失敗ではregistryまでの成果物が残り得る。cursorとsessionの失敗は成功結果へ反映されない。
registry不存在・malformed・非配列は空listとして扱うため、新規rowだけで上書きされ得る。

主要根拠:

- `docs/実装_aidlc-workflows/core/tools/aidlc-lib.ts:16831-16916`
- `docs/実装_aidlc-workflows/core/tools/aidlc-lib.ts:17333-18016`
- `docs/実装_aidlc-workflows/core/tools/aidlc-lib.ts:18155-18187`
- `docs/実装_aidlc-workflows/core/tools/aidlc-lib.ts:18617-18689`
- `docs/実装_aidlc-workflows/core/tools/aidlc-utility.ts:268-270`
- `docs/実装_aidlc-workflows/core/tools/aidlc-utility.ts:5512-5700`

## Go版への示唆

現行Go版はroot解決、Space作成・一覧・切替、Intent読み取り・一覧・切替、`os.Root`境界、安全な
cursor stagingを持つ。再利用できるのは`spaceSlug`のJS小文字化補正、new-file writer、
`completeActiveSpaceCursor`、`saveIntentCursor`、strict registry row検証である。

不足しているのはUUIDv7、Intent用24文字slug、UTC dateと衝突解決、未知fieldを保持するregistry
writer、cross-process workspace lock、CreateIntent API、failure injectionと並行process testである。
これらはGo標準libraryだけで実装できる。registry既存行はtyped structへ再encodeせず、
`json.RawMessage`として検証・保持する必要がある。

本家testはUUID・slug、予約語、full creation、active cursor、repos、2 process concurrencyを主に固定する。
Go版ではUTC境界、`-999` exhaustion、malformed registry無変更、各write/close/cursor失敗、owner違いの
lock解放拒否、context cancellation、partial artifactも追加で固定する。

