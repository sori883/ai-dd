# Space creation distribution E2E — 2026-08-31

- 結果: Passed（23ケース、作成成功10・help/version 2・エラー拒否11）
- 判定範囲: 初回の基本23ケース。後述の追加closed-pipe検証はFailedで、このsourceは完了版ではない。
- 種別: local distribution feature E2E
- source commit: `c7cdba318f9a594b8e4226ddcd1a6ce84d6d1617`
- build時のworking tree: clean。後続のE2E記録はbuild対象に含めていない。
- build version: `e2e-20260831-space-create`、表示commit: `c7cdba3`
- 環境: `Darwin 25.5.0 arm64`、Go: `go1.26.4 darwin/arm64`
- scenario（以下`<S>`）: `/Users/const/sori883/haihu-aidlc/e2e/2026-08-31-space-create-y2yRKy`
- binary: `<S>/aidlc`、Mach-O 64-bit executable arm64、2,989,330 bytes
- binary SHA-256: `f228cea181566f2e8036a3a5243c9d99949c0980866a2b29f701879ed2440223`
- 検証source: `<S>/space_create_e2e_test.go`、Go標準ライブラリのみ
- 検証source SHA-256: `40ea90e7e41a9803114e1af772d2dd6c572c068e1c23c7638e9cb7bd8b75c75f`
- 観測ログ: `<S>/e2e.log`。実行後に記録し、既存artifactは削除・上書きしていない。

## Build・実行

承認済みsandboxの未使用子directoryを`mktemp -d`で作り、repository rootからbuildした。

```sh
env CGO_ENABLED=0 go build -trimpath \
  -ldflags '-X github.com/sori883/ai-dd/src/internal/buildinfo.Version=e2e-20260831-space-create -X github.com/sori883/ai-dd/src/internal/buildinfo.Commit=c7cdba3' \
  -o /Users/const/sori883/haihu-aidlc/e2e/2026-08-31-space-create-y2yRKy/aidlc \
  ./src/cmd/aidlc

go test -tags=integration -count=1 -v \
  /Users/const/sori883/haihu-aidlc/e2e/2026-08-31-space-create-y2yRKy/space_create_e2e_test.go
```

検証用Goプロセスは配布binaryを絶対pathで起動した。子プロセスへ渡した環境は`PATH=`と、
root優先順位caseで明記した`AIDLC_PROJECT_DIR`・`CLAUDE_PROJECT_DIR`だけである。
Go/Node等をPATHで解決せずに全コマンドが動作した。Goはbuild・検証のためのhost側toolであり、
配布binaryの実行時依存ではない。OS提供機能まで不要という意味ではない。

実行済みscenarioでテストを再実行すると重複targetになるため、そのまま再実行しない。
再確認するときは別の未使用scenarioへbinary・検証source・初期fixtureを用意する。

## 初期fixtureと検証方法

- `project-empty`・`project-default`は空の既存directory。
- `project-seeded`はdefault org、異なるteam/project、phase/template、CodeKB/knowledge、
  default/legacyのintent cursor、`aidlc/active-space`、既存spaceと相対symlinkを持つ。
- default orgは`# Seed organization\nE2E only / 組織ルール\n`。
  既存orgのpermissionを0640にし、orgとactive-spaceのmtimeを過去日時へ固定した。
- `root-flag`・`root-aidlc`・`root-claude`・`root-cwd`は独立した空の既存directory。
- `must-stay-absent`は存在させず、不正入力・不存在projectの検証後も不在を確認した。

各コマンド前後にscenario全体をsnapshotし、既存fileのSHA-256、file種別・permission、
mtime、symlink参照先を比較した。成功時に限り、生成先の祖先directoryのmtime変更を許容した。
追加pathはそのtarget配下と必要な祖先だけに限定し、生成targetのtreeと各file本文を別途照合した。
失敗・help/versionでは、祖先を含めsnapshot全体が同一であることを確認した。

すべての生成spaceで対象自身を含む7directory・6fileが一致した。
`memory/org.md`だけをdefaultから継承し、それ以外は既定文・空fileだった。
`memory/phases/`と`intents/`は空で、余分な`.gitkeep`はなかった。
既存default/legacyの内容・metadata・cursorは変わらず、自動選択も行われなかった。

## コマンドの観測結果

`<S>`は上記scenario絶対pathの略記。環境欄の「なし」でも`PATH=`は渡している。
stdout欄の`\n`は実際の改行。全エラーはstdout空、stderrに`error`だけを含むJSON 1行、
末尾改行ありだった。下表はそのJSONの`error`値を記載する。OS固有の本文一致を公開契約にはしない。

| 入力（binary名は省略） | cwd / root環境 | 期待 / 実exit | stdout | stderrのerror値 |
| --- | --- | ---: | --- | --- |
| `--help` | `<S> / なし` | 0 / 0 | help、342 bytes | 空 |
| `--version` | `<S> / なし` | 0 / 0 | `aidlc e2e-20260831-space-create (commit c7cdba3)\n` | 空 |
| `space create "My Space" --project-dir ./project-empty` | `<S> / なし` | 0 / 0 | `Space created: my-space\n` | 空 |
| `--project-dir=./project-seeded space create "Team Alpha"` | `<S> / なし` | 0 / 0 | `Space created: team-alpha\n` | 空 |
| `space --project-dir ./project-default create default` | `<S> / なし` | 0 / 0 | `Space created: default\n` | 空 |
| `space create --project-dir=./project-empty "Between Words"` | `<S> / なし` | 0 / 0 | `Space created: between-words\n` | 空 |
| `space create "İB" --project-dir ./project-empty` | `<S> / なし` | 0 / 0 | `Space created: i-b\n` | 空 |
| `space create "   " --project-dir ./project-empty` | `<S> / なし` | 0 / 0 | `Space created: intent\n` | 空 |
| `space create "Flag Winner" --project-dir <S>/root-flag` | `<S>/root-cwd / AIDLC_PROJECT_DIR=<S>/root-aidlc, CLAUDE_PROJECT_DIR=<S>/root-claude` | 0 / 0 | `Space created: flag-winner\n` | 空 |
| `space create "AIDLC Winner"` | `<S>/root-cwd / AIDLC_PROJECT_DIR=<S>/root-aidlc, CLAUDE_PROJECT_DIR=<S>/root-claude` | 0 / 0 | `Space created: aidlc-winner\n` | 空 |
| `space create "Claude Winner"` | `<S>/root-cwd / CLAUDE_PROJECT_DIR=<S>/root-claude` | 0 / 0 | `Space created: claude-winner\n` | 空 |
| `space create "CWD Winner"` | `<S>/root-cwd / ` | 0 / 0 | `Space created: cwd-winner\n` | 空 |
| `space create "Team Alpha" --project-dir ./project-seeded` | `<S> / なし` | 1 / 1 | 空 | `create space "aidlc/spaces/team-alpha": mkdirat aidlc/spaces/team-alpha: file exists` |
| `space create typo --force --project-dir ./project-seeded` | `<S> / なし` | 1 / 1 | 空 | `unknown flag "--force"` |
| `space create typo extra --project-dir ./project-seeded` | `<S> / なし` | 1 / 1 | 空 | `space create requires exactly one name` |
| `--project-dir ./project-seeded space create typo --project-dir ./must-stay-absent` | `<S> / なし` | 1 / 1 | 空 | `duplicate --project-dir` |
| `space create --project-dir ./project-seeded` | `<S> / なし` | 1 / 1 | 空 | `space create requires exactly one name` |
| `space create typo --project-dir=` | `<S> / なし` | 1 / 1 | 空 | `--project-dir requires a nonempty path` |
| `space create typo --project-dir` | `<S> / なし` | 1 / 1 | 空 | `--project-dir requires a nonempty path` |
| `space create typo --project-dir --force` | `<S> / なし` | 1 / 1 | 空 | `--project-dir requires a nonempty path` |
| `space create SHOW --project-dir ./project-seeded` | `<S> / なし` | 1 / 1 | 空 | `reserved space name "show": invalid argument` |
| `space create "" --project-dir ./project-seeded` | `<S> / なし` | 1 / 1 | 空 | `invalid space name ""` |
| `space create typo --project-dir ./must-stay-absent` | `<S> / なし` | 1 / 1 | 空 | `open project root "<S>/must-stay-absent": open <S>/must-stay-absent: no such file or directory` |

検証結果は`--- PASS: TestDistributedSpaceCreate (0.89s)`、
`ok command-line-arguments 1.290s`、終了コード0だった。

## 関連検証と未検証範囲

同じGoソースの通常テスト、integration付きrace/shuffle、vet、gofmt、module tidy検証も成功した。
別途`CGO_ENABLED=0`のmacOS/Linux/Windows × amd64/arm64で、CLIとintegration付きworkspace
test binaryのcross compileはすべて成功した。ただし、この配布E2Eのnative実行はmacOS arm64のみ。

- Linux/Windowsでの配布binaryのnative実行は未実施。
- この配布E2EではI/O・Close・出力失敗注入、同時作成やsymlink境界を再実験していない。
  これらはrepository内のunit/integration testで検証している。
- installer、Codex向け資産展開、切替・intent作成・state/status、完全なworkspace lifecycleは対象外。
- 途中失敗時のrollback・修復、完全sandbox、crash durabilityは保証しない。

この結果はspace作成CLIの配布・実行確認であり、完全なAI-DLC install/lifecycle E2Eではない。
artifactと生成spaceはsandbox内へ保持し、削除する場合は別途対象を明示して承認を得る。

## 独立レビュー後の追加closed-pipe検証

同じ`c7cdba3`のbinaryについて、別の未使用scenario
`/Users/const/sori883/haihu-aidlc/e2e/2026-08-31-space-create-pipe-0LrrWK/`で
読み口を先に閉じたpipeを接続した。検証sourceは`closed_pipe_test.go`、観測ログは`red.log`。

```sh
go test -tags=integration -count=1 -v \
  /Users/const/sori883/haihu-aidlc/e2e/2026-08-31-space-create-pipe-0LrrWK/closed_pipe_test.go
```

stdoutへ閉pipeを接続して正常作成した場合と、stderrへ閉pipeを接続して名前を欠落させた場合の
両方で、`ProcessState=signal: broken pipe`・Goの`ExitCode=-1`となった。
期待するexit 1に対するassertionが失敗し、検証コマンドのexitは1だった。
stdout側は作成済み`pipe-target`のorg本文も確認でき、生成物は保持された。

これはmock writerでは検出できなかったUnixのSIGPIPE境界であり、基本23ケースのPassedとは
分けて記録する。初回artifactと失敗証跡は残し、修正後は別の新規scenarioで再確認する。
