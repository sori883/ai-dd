# Space切替を共有カーソルへ接続する実装計画

- 日付: 2026-09-01
- 状態: Accepted
- GitHub Issue: [#23](https://github.com/sori883/ai-dd/issues/23)
- 前提: [PR #22](https://github.com/sori883/ai-dd/pull/22)はマージ済み、Issue #21はClosed。
  baseは `7f9001260d4c93e7156a7b13d075466ba72b34ad`、作業branchは `codex/space-switch`。
- 関連: [space作成](2026-08-31-space-creation-plan.md)、
  [space一覧](2026-08-31-space-list-plan.md)、
  [原典・保存API調査](../research/2026-09-01-space-switch-contracts.md)、
  [意図的な差分の提示](2026-08-31-intentional-upstream-difference-reporting.md)

## 承認

space一覧のマージ後、次は共有カーソルを切り替える最小機能を作る案を提示し、
ユーザーは「はい。良いと思います」と実装順序を了承した。
その段階では調査と詳細計画に留め、コード・文書・Issue・PRを変更しなかった。

続いて、明示的なswitch構文、合成default、保存と失敗時の境界、本家との意図的な差分、
単独TDD・独立レビュー・配布E2E・PRまでの計画を提示した。
「この内容で実装を始めてよいですか？」に対するユーザーの「はい」を明示承認として記録する。
外部module/tool、対象外機能、未提示の意図的な仕様変更は承認されていない。
既存reader・create・listのAccepted契約は置換しない。自動マージはしない。

## 目的・対象外

Go単一binaryで既存spaceを選び、共有 `aidlc/active-space` に選択名を保存できるようにする。
作成→一覧→切替→一覧をCLIで確認できることを受入条件とする。

bare `space <name>`、session binding、harness include、audit、intent、state/status、
spaceの新設・修復・削除・rename、配布資産の展開は対象外。
これらは段階的な未実装範囲であり、恒久的な非互換方針ではない。
外部Go module/tool、store層、CI変更は追加しない。

## CLI契約

```sh
aidlc space switch <name> [--project-dir <path>]
```

- 名前はちょうど1つ。空文字・rawの `help`・`-h` は拒否し、切替を行わない。
- `--project-dir PATH` と `--project-dir=PATH` を既存create/listと同じ位置で受理する。
  空・欠落・重複、分離値の `-` 始まり、余剰位置引数、未知flagを拒否する。
- switch成功の `--json` は追加しない。構文エラーではcallback、cwd/env参照、FS操作をしない。
- 成功はstdoutに `Active space → <正規化名>\n`、stderr空、exit 0。
- 認識済みswitchの失敗はstderrへ1行のJSON `{"error":"..."}`、exit 1。
  stderrも書けない場合は終了コードだけを保証する。短いwriteと閉pipeも成功にしない。
- 既存の出力準備hookを認識済みswitchのcallback・最初の出力前に1回呼ぶ。
  SIGPIPE操作はmain側に保ち、help/version/未知commandへ広げない。
- root helpに構文を追加する。専用subcommand helpや対話promptは追加しない。
- 既存のbare一覧や未知commandの契約を維持する。
  特に `space --json false` は引き続き未知subcommand、従来形式のstderrとexit 2。

## API・名前・対象確認

```go
func SwitchSpace(input RootInput, rawName string) (string, error)
```

成功時は正規化名、失敗時は空文字と原因を返す。errorでも保存済みの場合がある。
CLIにはcreateと同形の `func(rawName, explicitDir string) (string, error)` を注入する。
mainはcallbackの中でだけcwd/envを読み、既存RootInputを組み立てる。

1. raw空文字/help/-hを拒否する。本家と同じslug化で名前を正規化する。
2. `normalizeSpaceName` からslug部分だけを共有private helperへ抽出し、
   create専用のraw・予約名検証は変えない。switchへcreateの予約名拒否を流用しない。
3. 既存ResolveRootの明示flag > AIDLC_PROJECT_DIR > CLAUDE_PROJECT_DIR > cwdを維持し、
   絶対path・既存projectを `os.OpenRoot` で開く。不正指定を低優先rootで救済しない。
4. 同じproject RootのFSで `ListSpaces` を呼び、名前が一覧に含まれるか確認する。
   明示的な非nil overrideで共有cursorの不要な読取りを避ける。active値は対象確認に使わない。
5. 一覧に含まれない名前は書込み前に拒否する。targetだけのStatへ置換しない。

合成defaultは実directoryがなくても選べる。非defaultは一覧のdirectory判定に従い、
memory/intentsの完全性を新たな前提にしない。途中Stat失敗等で一覧から漏れる場合があるため、
Unknownを「物理的に存在しない」と断定するAPIにはしない。

Unicode小文字化、ASCII化、48文字切詰め、数字開始のprefix、
非空の空白・記号だけなら `intent` になる順序を維持する。
raw `Help` から正規化された `help` や、既存の `list/create/switch` 等は一覧にあれば選べる。
同じtargetでも再保存する。ReadSpaces/ReadSelectionを経由せず、raw reader契約は変えない。

## 保存・安全性・失敗

- UTF-8の `<slug>\n` を `aidlc/active-space` へ保存する。他のcursor、session、
  harness、registry、space雛形などは更新しない。
- project Root内で必要な `aidlc/` 親だけ作り、そのdirectoryを別のRootとして開く。
  project自体は作らない。初回project pathのリンクと境界内相対リンクは既存方針どおり扱う。
- 検査時の既存cursorがsymlink（dangling含む）または非regularなら拒否する。
  外向き・絶対リンクの祖先を経由した保存もRoot境界で拒否する。
- 同じaidlc Root内に予測不能な名前の一時fileを `O_CREATE|O_EXCL` で作る。
  `crypto/rand.Text` とRoot.OpenFileを使い、名前衝突は有限回で打ち切る。
  Root.CreateTempがないため、絶対pathを再解決するos.CreateTempへ境界を戻さない。
- 新規directoryは0777、新規cursorは0666をumask付きで使用し、既存の作成方針に揃える。
  既存通常cursorの更新では一時fileを0600で作り、書込み後にFile.Chmodで
  既存permissionの9bitだけを継承する。owner、ACL、特殊mode、hardlink同一性等は保持保証しない。
- writeのerror・短いwrite、Chmod、Closeを確認してからRoot.Renameでcursorへ置換する。
  旧cursorを先に削除しない。自分が作成した一時fileだけcleanupし、失敗原因もwrap/joinして返す。
- 取得したfileとRootは閉じ、Close失敗を無視しない。

| 失敗段階 | 保証と残り得る状態 |
| --- | --- |
| Rename呼出前 | 他のwriterがいなければ旧cursor内容を保持。新しいaidlc親やcleanup失敗した一時fileは残り得る |
| Rename呼出以降、Root Close、成功出力 | cursorが切替済みの場合がある。成功を偽装せずerrorを返すが、自動rollbackしない |

置換はdirectory書込権限を必要とし、直接上書きとは権限・metadataの扱いが異なる。
排他lock、一覧と保存の一貫したsnapshot、敵対的な並行差替えへの完全transaction、
全OSでのatomic更新、fsyncによるcrash/powerloss耐久性は保証しない。
Rootはmountやdeviceまで制限する完全sandboxではない。

## 承認済みの意図的な差分

比較対象はローカル本家2.6.123のpublic CLI・共有cursor保存経路の静的調査。
最新upstream全体や全ハーネスとの完全互換は主張しない。

| 本家の挙動 | 採用する挙動 | 理由・互換性への影響 |
| --- | --- | --- |
| cursorを直接上書きし、mkdir/write失敗を吸収 | 一時fileから置換し、保存失敗をerror通知 | 失敗を成功と表示しない。directory書込権限が必要となり、owner/ACL等の保持は保証しない |
| 通常path操作でリンクを追従し、不在projectも作り得る | 既存projectのRoot境界とcursor型検査 | 誤った外部・非regular配置を更新しない。従来追従できた配置でも拒否する |
| 余剰引数・一部flagを無視し、raw help/-hはhelp表示 | 厳格検証とJSON error 1。project-dir等号形式は既存Go方針を継続 | 誤入力を隠さず既存Go CLIと揃える。従来無視された入力やhelp要求はerrorになる |

## 対象ファイル・所有権

実装writerは `go_tdd_implementer` の1名。

- 新規 `src/internal/workspace/space_switch.go`、対応unit/integrationテスト。
- `src/internal/workspace/space_create.go`: 名前のslug部分の共有化のみ。
- 新規 `src/internal/cli/space_switch.go` と対応テスト。
- `src/internal/cli/cli.go`、`space.go`、`src/cmd/aidlc/main.go` と関連テスト。
  callback追加に必要な既存call site更新とUnix閉pipeテストを含む。
- `docs/architecture.md`、`docs/development.md`、`docs/e2e-testing.md`。

親はRAM・索引・E2E driver/証跡・GitHub・commitを担当し、writerとの編集期間を分離する。
privateな関数注入・小さなwrite seamと既存test helperを再利用する。
独自の汎用FS interface、mutable global、無関係なrefactorは追加しない。

## TDD・検証・完了ゲート

baselineを採り、1つの観測可能な動作ごとにrunnableなassertionのRED→最小GREEN→refactorを行う。
compile失敗や追加前からGREENのguardをRED証拠には数えない。

1. createのGREENを維持してslug部分を共有化する。
2. 名前・予約名区別・Unknown・合成default・root入力を固定する。
3. 正確な保存、同一targetの再保存、cursor以外の非更新を固定する。
4. open/write/short write/chmod/Close/Rename/cleanup失敗と残存状態を関数注入で固定する。
5. strict args、callback未実行、lazy cwd/env、stdout/stderr/閉pipeを固定する。
6. integration tagで実FSのリンク境界を確認する。

```sh
go test -count=1 ./src/internal/workspace ./src/internal/cli ./src/cmd/aidlc
go test -count=1 -tags=integration ./src/internal/workspace
go test -count=1 -shuffle=on ./...
go test -count=1 -race -shuffle=on ./...
go test -count=1 -tags=integration -race -shuffle=on ./...
go vet ./...
go vet -tags=integration ./...
go mod tidy -diff
gofmt -l src
git diff --check
```

既存6target（darwin/linux/windows × amd64/arm64）をCGO_ENABLED=0でbuildし、
必要なtest binaryもcross compileする。cross-buildを各OSの実行成功とは扱わない。
配布E2Eは承認済み `/Users/const/sori883/haihu-aidlc/e2e/` の未使用scenarioで、
binaryのsource/hash、入力・stdout/stderr/exit、cursorと周辺dataの前後を記録する。
既存scenarioは上書き・削除しない。

Issue→単独TDD→固定base/headの独立レビュー→必要な修正と再検証→配布E2E→PRへ進める。
PRはIssue #23へ紐づけ、自動マージせず引き渡す。IssueのCloseはマージ後。
撤回時も利用者のcursorを自動で復元・削除せず、必要な復旧は利用者の判断に委ねる。

承認記録時点ではswitchの新規テスト、実装、独立レビュー、配布E2Eは未実施。
後続の検証結果は証跡への参照とともに追記する。

## 実装・検証結果

2026-09-01、Issue #23の承認scopeで実装を完了した。
製品差分を `39f1f58acdb60f6e3225e6e432cc9040406fa6f0` に固定し、
base `7f9001260d4c93e7156a7b13d075466ba72b34ad` からの独立read-onlyレビューで
blocking findingはなかった。承認済みの意図的な差分3点は変更していない。

- 実際のassertion RED→GREENは17 slice。追加時点でGREENのguardは区別した。
- 通常・shuffle・race・integration・coverage、vet、tidy差分、gofmt、goplsを通過。
  既存create/listと未知commandの契約も回帰確認した。
- 同じclean sourceから6構成のCLIと18 test binaryをcross compileした。
  各OSでの実行成功とは扱わない。
- macOS/arm64で配布binaryを76回起動し、期待exitとfilesystem差分に全件一致した。
  予定更新32回（space作成1回を含む）、無変更44回。Go/NodeをPATHから呼べない環境で検証した。
- stdout/両streamの失敗でexit 1でも保存済みとなるcaseを確認し、自動rollbackは行わない。

具体的なsource、artifact、TDD、生ログ、比較条件・未検証範囲は
[Space切替の実装検証・配布E2E](../../e2e-runs/2026-09-01-space-switch.md)へ記録した。
後続の証跡追記commitは文書のみで、検証binaryのsource commitとは区別する。
外部module/tool、store層、CI変更は追加していない。
bare alias、session/harness/audit、intent/status、installは引き続き後続段階へ分離する。
PRはIssue #23へ紐づけ、自動マージしない。IssueのCloseはマージ後とする。
