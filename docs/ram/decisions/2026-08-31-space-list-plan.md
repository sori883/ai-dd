# Space一覧をCLIへ接続する実装計画

- 日付: 2026-08-31
- 状態: Accepted
- GitHub Issue: [#21](https://github.com/sori883/ai-dd/issues/21)
- 前提: [PR #20](https://github.com/sori883/ai-dd/pull/20)はマージ済み。
  baseは `eb18da8943a9e09f388f5d5d1345e39ddebc96c9`、作業branchは `codex/space-list`。
- 関連: [space reader](2026-08-31-space-reading-contract.md)、
  [space作成](2026-08-31-space-creation-plan.md)、
  [今回の原典調査](../research/2026-08-31-space-list-contracts.md)、
  [意図的な差分の提示方針](2026-08-31-intentional-upstream-difference-reporting.md)

## 承認

space作成後の次の機能として、まず一覧表示を追加する案を提示した。
最初の実装依頼は詳細計画作成への了承として扱い、コード・設定・Issue・PRは変更しなかった。
その後、一覧とbare alias、JSON、shared cursorによる選択、専用の読取接続、
下記の意図的な差分、TDD・独立レビュー・配布E2E・PRまでの詳細計画を提示した。
ユーザーの「はい、じゃあえっと実装お願いしてもらっていいですか。」を明示承認として記録する。
自動マージはしない。過去のreaderや作成機能のAccepted契約は置換しない。

## 目的・CLIの受入条件

次の一覧操作だけを追加する。通常出力はstdout、成功時stderrは空、終了コードは0。

```sh
aidlc space list [--json] [--project-dir <path>]
aidlc space [--json] [--project-dir <path>]
```

- bare `space`は一覧のalias。切替を兼ねる `space <name>`は追加しない。
- `--project-dir PATH`と`--project-dir=PATH`は既存createと同じくcommandの前・途中・後に配置できる。
  空・欠落・重複を拒否する。分離値が `-`始まりなら欠落値として拒否し、実際のpathには
  `--project-dir=-dir`または`--project-dir ./-dir`を使う。
- `--json`は値なしのflagを1回だけ受理する。command前後に配置できる。
  `--json=true`、`--json=false`、分離した値、重複は許可しない。
- 一覧の余剰位置引数と未知flagを拒否する。構文エラーではcallback、cwd・環境参照、FS操作を行わない。
  未知command/subcommandの既存契約は維持する。
- 人間向け表示は `Spaces:\n` に続き、各行を `* <name>\n` または `  <name>\n` とする。
- JSONは1行＋改行、schemaは `{active,spaces:[{name,active}]}`。
  各行の順序・名前・activeはreader結果を保持する。active行がないときはトップのactiveだけ
  `default`にし、default行のfalseをtrueへ変更しない。JSON escapeは標準encoderを使う。
- 認識済み一覧操作の構文・読取接続・出力失敗は、stderrに1行のJSON `{"error":"..."}`、終了コード1。
  stderrも書けない場合は終了コードだけを保証する。stdout短いwriteや閉pipeも成功と扱わない。
- helpへ構文を追記し、既存help/version/create/未知commandの表示・終了コードの契約を維持する。
  一覧専用help、stdin、対話prompt、force、dry-run、新しいJSON fieldは追加しない。

## API・接続境界

```go
func ReadSpaces(input RootInput) ([]Space, error)
```

1. 既存 `ResolveRoot`で明示flag > AIDLC_PROJECT_DIR > CLAUDE_PROJECT_DIR > cwdの順に解決する。
2. 結果が絶対pathであることを確認し、projectを `os.OpenRoot`で開く。
3. `ListSpaces(projectRoot.FS(), nil)`を呼び、shared `aidlc/active-space`を選択元にする。
4. 取得したRootは内部で必ずCloseする。RootやFSを返値に含めない。

相対rootはfs.ErrInvalidを識別できるerror、project open失敗（不在を含む）とClose失敗は
原因を保持したerrorを返す。error時はnil sliceとし、不正rootを低優先候補で救済しない。
open成功後のreader内の読取失敗は、既存のfallback・列挙打切り契約を維持する。
不在と権限不足等をすべて区別できるAPIとは説明しない。

既存 `ActiveSpace`、`ListSpaces`、`ReadSelection`、root/intent readerは変更しない。
defaultの常時追加、JS trim/UTF-16順、未知cursorで全行inactiveになる契約を維持する。
cursor名の追加検証や名前を使ったpathアクセスはせず、intentやstateを読む前処理も追加しない。

mainが `func(explicitDir string) ([]workspace.Space, error)`のcallbackをCLIへ渡す。
callbackの中でだけcwdと環境変数を読み、RootInputを組み立てる。CLIはFSに直接触れず、
表示DTOへの変換・構文・終了コードを担当する。既存の出力準備hookは、createに加えて
認識済みlist/bareでも最初の出力・callback前に1回呼び、main側でSIGPIPEを扱う。
CLI packageにprocess-globalなsignal操作を移さず、help/version/未知commandには広げない。

## 承認済みの意図的な差分

比較はローカルAI-DLC 2.6.123のpublic parser・一覧・選択経路の静的確認に限定する。

| 本家の挙動 | 採用する挙動 | 理由・互換性への影響 |
| --- | --- | --- |
| 不在projectでもreaderがdefaultへfallbackし得る | 既存project必須。不在はerror | root指定ミスを隠さない。従来一覧だけ返せたpathでも失敗する |
| 通常のfilesystemでsymlinkを追従する | 初回OpenRootで開いたprojectを境界とし、外向き・絶対リンクを拒否する | 意図しない外部参照を防ぐ。該当リンクでfallbackや列挙打切りになり、一覧が省略され得る |
| 余剰引数や一部flagを無視し、project-dir重複は最後優先。publicでは=形式をglobal認識しない | 未知・余剰・重複・値付きjsonを拒否し、project-dirの=形式を受理する | 誤入力を明示し、既存Go createと揃える。黙って無視されていた入力はerrorになる |

初回指定project自体のsymlinkにはOpenRootが追従する。境界内への相対リンクは許可する。
Rootはmount・特殊file等まで遮断する完全sandboxではない。

session bindingは後続の未実装範囲であり、恒久的に本家と変える決定ではない。
今回の選択中表示はshared cursorに基づくため、bindingを使用中の本家とは表示が異なる場合がある。
作成・切替・修復・audit・session書込みを一覧経路に追加しない。

## 対象ファイル・所有権

実装writerは `go_tdd_implementer`の1名とする。

- 新規 `src/internal/workspace/space_read.go`と対応unit/integrationテスト。
- `src/internal/cli/cli.go`、`space.go`、一覧用 `space_list.go`と対応テスト。
- `src/cmd/aidlc/main.go`、対応main/Unix閉pipeテスト。
- `docs/architecture.md`、`docs/development.md`、`docs/e2e-testing.md`。

親はRAM・索引・E2E結果・GitHub・commitを担当し、実装担当との編集期間を分離する。
外部Go module/tool、store層、独自FS interface、CI設定変更は不要。
既存共有parserの小さな整理と `cli.Run`呼出し側の追従変更は範囲内だが、無関係なrefactorはしない。

## TDD・検証

targeted baselineを採り、1つの観測可能な動作ごとにRED→最小GREEN→GREEN上で整理する。

1. ReadSpacesの接続、root優先順位、相対root/open/close失敗。privateな関数注入で異常系を固定する。
2. human/JSON、bare alias、default未作成・非先頭、未知cursor、名前のescapeを表示側で確認する。
3. flag配置・厳格検証とcallback未実行。create/旧root commandの回帰を維持する。
4. mainからのRootInput受渡し、cwd失敗、正常構文でだけprocess入力を読むことを確認する。
5. 実FSの境界内/外/絶対/壊れたsymlink、既存dataのsnapshot無変更、実closed pipeのexit 1。

未実装APIのcompile失敗ではなく、最小のrunnable seamを通じた意図したassertion失敗をREDの証拠にする。
既存readerのテストや追加前からGREENのguardは、新機能のRED実績に数えない。
実FSテストはintegration tag、既存main subprocessによる閉pipe回帰は通常Unixテストに含める。

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

darwin/linux/windows × amd64/arm64の6構成をCGO_ENABLED=0でbuildし、必要なtest binaryもcross compileする。
配布E2Eは承認済み `/Users/const/sori883/haihu-aidlc/e2e/`の未使用scenarioへ配置し、
binaryのhash/source commit、stdout/stderr/exit、一覧実行前後のfixtureの無変更を記録する。
既存scenarioを上書き・削除しない。native実行とcross compileの証拠を混同しない。

## リスク・対象外・完了ゲート

並行更新中の一貫したsnapshot、全OSでの名前表現一致、不正UTF-8の完全互換は保証しない。
default行の存在は初期化済みを意味しない。readerのerror吸収で一覧が部分的になる場合がある。
撤回時は追加コード・文書を取り消すだけで、利用者dataのmigrationや削除は不要。
switch、intent、status、session binding、インストーラーや配布資産展開は今回の対象外。

Issue作成→単独TDD実装→固定base/headの独立レビュー→必要な修正・最終検証→配布E2E→PRと進める。
PRはIssueへ紐づけ、自動マージせずユーザーへ引き渡す。Issueはマージ後にCloseする。
