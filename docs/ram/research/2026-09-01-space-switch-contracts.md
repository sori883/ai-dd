# Space切替の参照契約と保存API

- 日付: 2026-09-01
- 状態: Current for local AI-DLC 2.6.123 snapshot
- 関連: [承認済み計画](../decisions/2026-09-01-space-switch-plan.md)、
  [一覧の参照契約](2026-08-31-space-list-contracts.md)

## 確認範囲

analysis indexと `core/tools/aidlc-version.ts` は2.6.123で一致した。
計画時にtechnical_researcherがauthored coreと配置版の
`aidlc.ts`・`aidlc-utility.ts`・`aidlc-lib.ts`・`aidlc-includes.ts` のSHA-256一致を確認した。
全配布物の一致や最新upstreamとの一致は主張しない。
参照snapshotのcommand・hook・installer・テストは実行せず、静的に読んだ。

## 本家のCLI・対象名

- publicは `aidlc space <name>` と `aidlc space switch <name>` の両方を受理する。
  public parserは必要なutility引数を再構成し、余剰位置引数・一部の後続flagを落とす。
- globalの `--project-dir PATH` は前・途中・後に置け、重複時は最後を使う。
  `=PATH` はそのglobal経路で認識しない。
- `space switch` の名前欠落はusage/exit 1。空文字もutility側で失敗する。
  `space ""` は一覧であり、明示switchとは異なる。
- switch成功のJSON schemaはなく、`--json` は一覧用。
- targetは `slugify(raw)` の結果を `listSpaces` の名前と比較する。
  createの予約名validatorやvalidSpaceFlagは使わない。
- raw `help`・`-h` はhelp表示となり、書き込まない。一方 `Help` は正規化後の
  `help` が既存なら選べる。明示switchでは既存 `list/create/switch/archive/rename/show/birth`
  も選択可能である。
- Unicode小文字化、ASCII slug、48文字切詰め、数字開始へのprefix等は作成と同じ。
  非空の空白・記号だけなら `intent` になる。

listSpacesはspaces directoryがない/読めない場合にも合成defaultを含めるため、
defaultの実directoryやmemory/intentsは切替の必須条件ではない。
非defaultはdirectory所属のみ確認し、directory symlinkも通常のFSとして追従する。
途中Stat失敗で列挙から漏れたtargetはUnknownとなる。

主要根拠（実装snapshot基準）:

- `docs/実装_aidlc-workflows/core/tools/aidlc.ts:653`、`:874`
- `docs/実装_aidlc-workflows/core/tools/aidlc-lib.ts:732`、`:754`、`:2173`、`:2419`
- `docs/実装_aidlc-workflows/core/tools/aidlc-utility.ts:6217`、`:6243`

## 保存と後続処理

`setActiveSpaceCursor` はaidlc親をrecursive mkdirし、
`aidlc/active-space` へUTF-8の `<name>\n` を直接writeする。
mkdir/writeの例外をすべて吸収し、同じtargetでも再writeする。
atomic置換、lock、rollbackはない。cursor/祖先のsymlinkは通常のpath操作として追従する。
したがって、本家の成功表示はcursorの保存成功を保証しない。

順序は対象確認→workflow/session解決→cursor保存→session関連更新→harness更新→成功表示。
session衝突ならcursor書込み前に失敗し得る。
session IDはselectionを優先し、なければcurrent-sessionを読み、
targetのactive intentをbindingへ反映し、rebind offerやintent UUID stampを更新する。
targetのactive-intent cursor、registry、state/stageを直接書き換える処理ではない。

Codexでは既存 `.codex/config.toml` の `AIDLC_RULES_DIR` を
`aidlc/spaces/<target>/memory` へ差し替える。現在のharnessだけが対象。
同内容なら書かないが、他spaceからdefaultに戻す場合は更新される。
読取/変換失敗はskipし得る一方、includeのwrite/rename失敗はthrowし得る。
既に更新されたcursor/sessionを巻き戻す処理はない。

成功は `Active space → <target>\n`、include更新時だけ追加行、通常exit 0。
Unknown等はJSON stderr/exit 1。解決可能なstateがあるerror経路では、
条件付きのERROR_LOGGED監査追記も試みる。

主要根拠:

- `docs/実装_aidlc-workflows/core/tools/aidlc-lib.ts:2531`、`:18166`、`:22207`
- `docs/実装_aidlc-workflows/core/tools/aidlc-utility.ts:6253`、`:6273`
- `docs/実装_aidlc-workflows/core/tools/aidlc-includes.ts:147`、`:270`
- 名前/Unknownのテスト: `tests/integration/t165-intent-create-p4.test.ts:990`
- session更新のテスト: `tests/integration/t311-session-binding-writers.test.ts:178`
- include更新のテスト: `tests/unit/t-active-space-includes.test.ts:266`

最後の3つのtest pathは実装snapshot配下。合成defaultへの実切替、cursor書込失敗、
同一targetの再write、並行writeの直接テストは今回の限定確認では見つからなかった。
これらの契約はソースからの判断であり、本家実行による確認ではない。

## Go保存APIの一次資料

Context7を先に参照した。解決された標準ライブラリ資料は
`/websites/pkg_go_dev_go1_25_3` であり、プロジェクトのGo1.26.4そのものではない。
親がローカルGo1.26.4の `go doc` と該当標準ライブラリsourceで照合した。

- os.Rootは境界外/絶対symlinkを拒否するが、mount/device等を含む完全sandboxではない。
- Root.OpenRootでaidlc directoryを開き、以降の一時fileと置換を同じRootに限定できる。
- RootにCreateTempはない。Root.OpenFileのO_CREATE|O_EXCLとcrypto/rand.Textの予測不能名で
  排他作成する案なら、os.CreateTempに絶対pathを渡す再解決を避けられる。
- Root.Renameは通常Renameの契約に従い、既存非directoryを置換する。
  非Unixでは同じdirectory内でもatomicとは保証されない。
- 新規fileのmodeにはumaskが適用される。既存fileのpermissionを置換後も残すには、
  新規一時fileのFile.Chmod等が必要で、owner/ACL等の完全なmetadata保存とは別である。
- File.Close、Root.Close、Remove等はerrorを返すため、primary errorと合わせて原因を残す。
  Rename後のClose/出力失敗は保存済みでも起き得る。

参照:

- [os.Root](https://pkg.go.dev/os#Root)
- [os.Rename](https://pkg.go.dev/os#Rename)
- [Traversal-resistant file APIs](https://go.dev/blog/osroot)
- [crypto/rand.Text](https://pkg.go.dev/crypto/rand#Text)

webのosページは確認時Go1.27.0を表示したため、Go1.26.4の証拠はローカルdoc/sourceと区別した。
標準ライブラリsourceの確認箇所は `os/root.go`、`root_openat.go`、
`root_unix.go`、`root_windows.go`、`internal/syscall/windows/at_windows.go`。

## 推奨と限界

共有cursorだけを初回範囲とし、保存失敗をerror通知する。
一時fileへの書込み/Closeを先に完了させることで、Rename呼出前の失敗による旧cursor破損を避ける。
Cursor型検査とRoot境界を追加し、strict CLIは既存Go版へ揃える。
詳細な採用理由・互換影響・非保証は承認済み計画を参照する。

同時切替の排他、全OS atomic、敵対的差替え、crash耐久性は検証済みとはしない。
session/harness/auditは後続段階へ分け、共有cursor更新を本家の全切替処理と同一視しない。
