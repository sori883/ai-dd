# 保存済み state reader の参照契約

- 調査日: 2026-09-03（Asia/Tokyo）
- 状態: Current（このrepositoryに固定された本家AI-DLC `2.6.123`の確認範囲）
- 対象Issue: [#63](https://github.com/sori883/ai-dd/issues/63)

## 背景と確認範囲

AI-DLCは、1件の作業単位であるIntentの進行状況を、Intent record直下の
`aidlc-state.md`へ保存する。このファイルには、scope、現在のStage、workflowの状態、各Stageのcheckbox、
承認済み計画を表す`EXECUTE` / `SKIP` suffixが含まれる。Go側では、初期stateを構築する
`state.BuildInitial`と保存する`state.WriteInitial`が先に実装されているため、次に保存済みstateを安全に
typed valueへ戻す低水準readerの契約を固定する。

本家の確認は、このrepositoryに置かれたAI-DLC `2.6.123`の実装を、state生成・field読取り・Stage suffix
読取りに必要な範囲だけ静的に照合した結果である。最新upstream全体との一致や、列挙していないworkflowの
挙動との一致は主張しない。

確認した根拠は次のとおりである。

- `docs/実装_aidlc-workflows/core/tools/aidlc-utility.ts:5809-5968`
  - 初期`aidlc-state.md`のsection、field、phase順、Stage row、State Version 8を生成する。
- `docs/実装_aidlc-workflows/core/tools/aidlc-lib.ts:16370-16378`
  - `readStateFile`が固定state pathを`existsSync`で確認し、`readFileSync(..., "utf-8")`で読む。
- `docs/実装_aidlc-workflows/core/tools/aidlc-lib.ts:16477-16490`
  - `getField`がsectionを考慮せず、最初に一致したfieldをtrimして返す。
- `docs/実装_aidlc-workflows/core/tools/aidlc-lib.ts:16693-16734`
  - `parseCheckboxes`がdocument全体をregexで走査し、regexに一致しないrowを取り込まずに進める。
- `docs/実装_aidlc-workflows/core/tools/aidlc-lib.ts:21917-21935`
  - `parseStateStageSuffixes`がdocument全体からsuffixを読み、同じslugのMap値を後勝ちで置き換える。
- `docs/実装_aidlc-workflows/core/tools/aidlc-lib.ts:23258-23270`
  - 保存済みstateのschema versionを`CURRENT_STATE_VERSION = "8"`として扱う。

## 本家で観測した形式と挙動

初期stateの先頭は`# AI-DLC State Tracking`である。標準生成物には次の必須sectionがあり、ほかに
`Scope Configuration`、`Workspace State`、`Runtime State`、`Session Resume Point`などの追加sectionも
存在する。

- `Project Information`: `Project Type`、`Scope`、`State Version`
- `Execution Plan Summary`: `Total Stages`、`Completed`、`In Progress`
- `Phase Progress`: `Initialization`、`Ideation`、`Inception`、`Construction`、`Operation`
- `Stage Progress`: phase見出しの下にStage rowを並べる
- `Current Status`: `Lifecycle Phase`、`Current Stage`、`Next Stage`、`Status`

生成側はPhaseをこの5件の順に出し、statusには`Pending`、`Active`、`Verified`、`Skipped`を使う。Stage
rowは`- [marker] slug — suffix`であり、初期生成ではsuffixへ`EXECUTE`または`SKIP`を保存する。checkboxの
markerは`[ ]`、`[-]`、`[?]`、`[R]`、`[x]`、`[S]`で、これは実行計画のsuffixとは別の情報である。

本家の通常readerはpathの存在確認後にNodeの通常file readを行うため、sectionの構造、UTF-8の妥当性、
leafのfile typeをtyped stateの境界で保証しない。`getField`、`parseCheckboxes`、
`parseStateStageSuffixes`も、state全体の意味的一貫性を検証するvalidatorではなく、必要な文字列を投影する
helperである。

## Goで固定する観測契約

Goの内部APIは次の2つに分ける。

```go
func Read(recordRoot *os.Root) (State, error)
func Parse(content []byte) (State, error)
```

`Read`はcaller-ownedのrecord `*os.Root`から固定leaf `aidlc-state.md`を読む。nil rootはI/O前に
`fs.ErrInvalid`で拒否し、`Lstat`でregular fileだけを受け入れる。symlink、directory、FIFO、deviceなどは
拒否し、rootをCloseせず、state bytes、mode、mtimeを変更しない。通常readに伴うatimeの不変までは保証しない。
missingやpermissionなどのI/O原因はerror chainへ保持する。

`Parse`は入力byteを保持せず、invalid UTF-8、不正なlone CR、`\r\r\n`、header不一致を拒否する。先頭BOMは
最大1個だけ除去し、LF・CRLFの混在と末尾LFなしを受け入れる。`bufio.Scanner`の既定64 KiB制限は使わない。
header、必須section、section内必須fieldは一意に要求し、section順とfield順は固定しない。未知の追加sectionと
未知fieldは保持・解釈せずに許容する。

成功した`State`はfieldを非公開にし、value accessorから次を提供する。

- Version、Scope、ProjectType、WorkflowStatus、LifecyclePhase、CurrentStage、NextStage
- `Summary{TotalStages int, Completed int, InProgress string}`
- canonical 5件のPhaseProgress（phase順）
- document順のStageProgress（Slug、CheckboxMarker、derived CheckboxState、trim済みraw Suffix、derived PlanAction）

必須fieldの重複・欠落・空値、State Version以外のversion、非canonical decimal、未知enumを拒否する。
`Completed <= Total`、`[x]`件数、Current Stageのmarker、graph membershipなどの意味的な相互検証はこの
低水準readerの責務にしない。Stage suffixの先頭action wordだけをword boundary付きで`EXECUTE`または`SKIP`として解釈し、
後続の説明はSuffixへ保持する。checkbox stateとPlanActionは直交するため、`[S] stage-slug — EXECUTE`を受理する。

separator候補を含むStage rowは、dash位置・suffix開始位置・slug妥当性を一度の前向き走査で記録し、候補を右から評価する。
size上限は設けないが、候補探索で同じslug prefixやsuffix全体を繰り返し走査しないため、1 rowのCPU時間は入力長に対して線形とする。

返却sliceはcallerが変更できる独立コピーとし、error時はpartial Stateを返さずzero Stateにする。保存済み
`aidlc-state.md`の`EXECUTE` / `SKIP` suffixは、先頭action wordをword boundaryで判定し、`EXECUTE: reason`・
`SKIP: reason`のような説明もraw suffixへ保持する。`EXECUTEfoo`・`SKIP_foo`などword継続は拒否する。suffixは
将来のin-flight recomposeでroutingを判断する正本である。
readerはgraphを再計算したり、stateを書き換えたりしない。

## 本家AI-DLCとの差分

比較対象は上記の固定snapshot `2.6.123`だけであり、最新upstreamとの差分は未確認である。次の挙動はGoで
意図的に変更し、Issue #63の承認記録へ引き継ぐ。

| 本家の挙動 | Goで採用する挙動 | 変更理由 | 利用者・互換性への影響 |
| --- | --- | --- | --- |
| `getField`はdocument全体の最初の一致を返し、sectionを限定しない | 必須fieldを対応section内でexact label一意に要求する | decoyや重複で別sectionの値がroutingへ混入することを防ぐ | 手編集・legacyの曖昧なstateは早く拒否され、修復が必要になる |
| `parseCheckboxes`はdocument全体を走査し、regexに一致しないStageらしい行を無視し得る | `Stage Progress`内だけをdocument順に読み、`- [`で始まるmalformed rowを拒否する | section外decoyを無視し、部分破損をpendingへ黙って寄せない | 破損した手編集stateは本家より早く停止する |
| duplicate Stageは一覧に残り、suffix Mapでは後の値が勝つ | duplicate slugを拒否する | 同一slugのPlanActionを一意にできない状態で進めない | 重複rowを持つstateは修復が必要になる |
| 通常の`readStateFile`はNodeの通常file readでsymlinkを追跡し得る | `Lstat`後にregular leafだけを読み、symlinkを拒否する | record root外の内容をstateとして取り込まない | symlink共有stateは本家より早く停止する |
| reader契約にinvalid UTF-8や不正CRの明示拒否がない | invalid UTF-8、lone CR、`\r\r\n`を`fs.ErrInvalid`で拒否する | byte列の曖昧な文字列化や行境界の誤読を防ぐ | 不正encoding・改行のstateは修復が必要になる |

これはGoへの単純な型変換で不可避な差分ではなく、曖昧なstateを誤ったroutingへ進ませないために承認された
fail-closed方針である。正常な本家生成stateのsection、field、marker、suffixの意味は維持する。

## 未確認事項と残余risk

- 本家の最新upstream全体、未列挙workflow、将来のState Versionとの互換性は確認していない。Go readerはv8だけを受け入れ、migrationは行わない。
- `Lstat`とreadの間にleafが置換されるTOCTOUは、このsliceでは完全に解消しない。read後にParseすることで、少なくとも取得したbytesの構造検証は行う。
- `*os.Root`のcontainmentやsymlinkのOS差はGo/runtimeとOSの実装に依存する。Windowsのsymlink作成権限不足は該当integration caseだけを理由付きでskipする。
- file size上限は設けない。64 KiB超の未知sectionを受け入れる代わりに、極端に大きな入力のメモリ使用量は上位callerの責務である。
- graph join、summaryとcheckboxの整合性、state mutation、audit、CLI、advance、recompose本体はこのreaderの未実装境界である。
- `fs.FS`単体をsandboxとして扱うのではなく、recordのcontainmentはcallerが開いた`*os.Root`で担保する。
