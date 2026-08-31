# Space creation distribution E2E — 最終確認（2026-08-31）

- 結果: Passed（基本23ケース＋closed-pipe 3パターン＋再試行2件、計28回のコマンド実行）
- source commit: `a5623a625a0fba80b9f0f8042600d9171c85927d`
- build時のworking tree: clean。この実行証跡だけを後続のdocs commitで追加する。
- build version: `e2e-20260831-space-create`、表示commit: `a5623a6`
- 環境: `Darwin 25.5.0 arm64`、Go: `go1.26.4 darwin/arm64`
- scenario（以下`<S>`）: `/Users/const/sori883/haihu-aidlc/e2e/2026-08-31-space-create-final-mZmL2f`
- binary: `<S>/aidlc`、Mach-O 64-bit executable arm64、3,007,410 bytes
- binary SHA-256: `8674a1fa42394cabe316072e89b1134f15329e1d025e46a166b82e7c882130fd`
- 基本検証source: `<S>/space_create_e2e_test.go`
- 同SHA-256: `cf7ae5fb80a8e0c00be8fd10332f2b15877e428f4875abb24e6b94e4ded268c1`
- pipe検証source: `<S>/closed_pipe_test.go`
- 同SHA-256: `576433b1dc8cd7a50d29343c00d20e29f207a41c69bf595e1e2a9591766fca5e`
- 観測ログ: `<S>/e2e.log`。実行後に保存し、binary・fixture・生成物も保持した。

[初回検証](2026-08-31-space-create.md)は基本23ケースが成功したものの、
追加のclosed-pipe検証でSIGPIPE終了を検出した。この記録はその修正後の再検証である。
初回・失敗再現のscenarioは変更せず、新しい未使用directoryを作成した。

## Build・実行

repository rootから次を実行した。

```sh
env CGO_ENABLED=0 go build -trimpath \
  -ldflags '-X github.com/sori883/ai-dd/src/internal/buildinfo.Version=e2e-20260831-space-create -X github.com/sori883/ai-dd/src/internal/buildinfo.Commit=a5623a6' \
  -o /Users/const/sori883/haihu-aidlc/e2e/2026-08-31-space-create-final-mZmL2f/aidlc \
  ./src/cmd/aidlc

go test -tags=integration -count=1 -v \
  /Users/const/sori883/haihu-aidlc/e2e/2026-08-31-space-create-final-mZmL2f/space_create_e2e_test.go \
  /Users/const/sori883/haihu-aidlc/e2e/2026-08-31-space-create-final-mZmL2f/closed_pipe_test.go
```

検証sourceはGo標準ライブラリだけを使用する。子プロセスは配布binaryを絶対pathで起動し、
環境には`PATH=`と、各caseに明記したroot環境変数だけを渡した。
Go/NodeのPATH解決なしで動作した。Goはhost側のbuild・検証toolであり、CLIのruntime依存ではない。
OS提供機能まで不要という主張ではない。

同じscenarioで再実行すると既存targetに衝突する。再確認は未使用scenarioへ初期fixtureを用意し、
既存artifactを削除・上書きせずに行う。

## Fixture・検証方法

初回と同じ空project、default orgと異なるteam/project/phase/template/CodeKB/knowledge、
既存space・intent cursor・active-space・相対symlinkを用意した。org本文は
`# Seed organization\nE2E only / 組織ルール\n`、orgのmodeは0640、orgとactive-spaceのmtimeは過去日時に固定した。
root優先順位用の4つの独立projectと、closed-pipe用の空`project-pipe`も用意した。
`must-stay-absent`は最後まで作成されなかった。

各コマンド前後にscenario全体のfile SHA-256、種別、permission、mtime、symlink参照先を比較した。
成功した生成先の祖先directoryについてだけmtime変化を許容し、新規pathもtargetと必要な祖先に限定した。
それ以外の既存dataは不変、通常のエラー・help/version・再試行ではsnapshot全体が不変だった。

新規spaceはすべて対象自身を含む7directory・6fileで、各本文も照合した。
default orgだけを継承し、team/projectは既定文、3つの`.gitkeep`は空、
`memory/phases/`と`intents/`は空directoryだった。既存cursorは変わらず、自動選択は行われなかった。

## 基本23ケースの観測結果

`<S>`は上記絶対pathの略記。環境欄の「なし」でも`PATH=`は渡している。
`\n`は実際の改行。エラー欄はstderr JSON 1行の`error`値であり、stdout空・末尾改行ありも確認した。
OS固有のerror本文の完全一致を公開契約にはしない。

| 入力（binary名は省略） | cwd / root環境 | 期待 / 実exit | stdout | stderrのerror値 |
| --- | --- | ---: | --- | --- |
| `--help` | `<S> / なし` | 0 / 0 | help、342 bytes | 空 |
| `--version` | `<S> / なし` | 0 / 0 | `aidlc e2e-20260831-space-create (commit a5623a6)\n` | 空 |
| `space create "My Space" --project-dir ./project-empty` | `<S> / なし` | 0 / 0 | `Space created: my-space\n` | 空 |
| `--project-dir=./project-seeded space create "Team Alpha"` | `<S> / なし` | 0 / 0 | `Space created: team-alpha\n` | 空 |
| `space --project-dir ./project-default create default` | `<S> / なし` | 0 / 0 | `Space created: default\n` | 空 |
| `space create --project-dir=./project-empty "Between Words"` | `<S> / なし` | 0 / 0 | `Space created: between-words\n` | 空 |
| `space create "İB" --project-dir ./project-empty` | `<S> / なし` | 0 / 0 | `Space created: i-b\n` | 空 |
| `space create "   " --project-dir ./project-empty` | `<S> / なし` | 0 / 0 | `Space created: intent\n` | 空 |
| `space create "Flag Winner" --project-dir <S>/root-flag` | `<S>/root-cwd / AIDLC_PROJECT_DIR=<S>/root-aidlc, CLAUDE_PROJECT_DIR=<S>/root-claude` | 0 / 0 | `Space created: flag-winner\n` | 空 |
| `space create "AIDLC Winner"` | `<S>/root-cwd / AIDLC_PROJECT_DIR=<S>/root-aidlc, CLAUDE_PROJECT_DIR=<S>/root-claude` | 0 / 0 | `Space created: aidlc-winner\n` | 空 |
| `space create "Claude Winner"` | `<S>/root-cwd / CLAUDE_PROJECT_DIR=<S>/root-claude` | 0 / 0 | `Space created: claude-winner\n` | 空 |
| `space create "CWD Winner"` | `<S>/root-cwd / なし` | 0 / 0 | `Space created: cwd-winner\n` | 空 |
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

## Closed-pipeと再試行の観測結果

pipeの読み口を先に閉じてから実binaryを起動した。下表の閉pipe欄は読み手がなく取得不可であり、
空のJSONを受信したという意味ではない。すべて通常の`exit status 1`で、SIGPIPE終了ではなかった。
stdout失敗・両方失敗では生成物を保持し、読み書き可能な通常stdioで同じ入力を再試行して重複拒否を確認した。

| 条件・入力 | 期待 / 実exit | stdout | stderr | 生成物・再試行 |
| --- | ---: | --- | --- | --- |
| `closed stdout / space create pipe-stdout --project-dir <S>/project-pipe` | 1 / 1 | 閉pipe（取得不可） | `write stdout: write /dev/stdout: broken pipe` | 完成7dir・6fileを保持、他の既存dataは無変更 |
| `retry after closed stdout / space create pipe-stdout --project-dir <S>/project-pipe` | 1 / 1 | 空 | `create space "aidlc/spaces/pipe-stdout": mkdirat aidlc/spaces/pipe-stdout: file exists` | 重複拒否、全snapshot無変更 |
| `closed stderr / space create` | 1 / 1 | 空 | 閉pipe（取得不可） | 全snapshot無変更 |
| `closed both / space create pipe-both --project-dir <S>/project-pipe` | 1 / 1 | 閉pipe（取得不可） | 閉pipe（取得不可） | 完成7dir・6fileを保持、他の既存dataは無変更 |
| `retry after closed both / space create pipe-both --project-dir <S>/project-pipe` | 1 / 1 | 空 | `create space "aidlc/spaces/pipe-both": mkdirat aidlc/spaces/pipe-both: file exists` | 重複拒否、全snapshot無変更 |

実行結果は`TestDistributedSpaceCreate (1.32s)`と`TestDistributedClosedPipe (0.19s)`がPASS、
`ok command-line-arguments 2.114s`、検証コマンドのexitは0だった。

## 検証範囲と残余リスク

修正後の同じGo sourceで通常・shuffle・race・integration・coverageテスト、vet、gofmt、tidy、
gopls診断が成功した。`CGO_ENABLED=0`でmacOS/Linux/Windows × amd64/arm64の
CLI・workspace integration test binary・main test binaryをcross compileし、6構成とも成功した。

- native配布E2EはmacOS arm64のみ。Linux/Windowsの配布binaryのnative実行は未実施。
- 同名競合、symlink境界、file I/O・Close失敗注入はrepository内のunit/integration testで確認し、
  この配布E2Eでは再実験していない。
- installer、Codex向け資産展開、切替・intent作成・state/status、完全なworkspace lifecycleは対象外。
- 部分/完成済みtargetが残る場合があり、自動rollback/repair・完全sandbox・crash durabilityは保証しない。
- govulncheckは未導入・未実施。新しい外部Go module/toolは導入していない。

これはspace作成CLIの配布・実行確認であり、完全なAI-DLC install/lifecycle E2Eではない。
保持したartifactを削除する場合は、対象を明示して別途承認を得る。
