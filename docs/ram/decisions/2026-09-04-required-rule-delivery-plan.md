# 配置ファイルから必須ルール本文を毎回読み込む

- 日付: 2026-09-04
- 状態: Accepted（知識供給マイルストーン内）
- Issue: [#89](https://github.com/sori883/ai-dd/issues/89)
- 基点: `91e476aa75776ec3bdc9723c40dda26abbdbf410`（PR #88）
- 実装許可: [配置Markdownによる知識供給](2026-09-04-file-based-knowledge-delivery.md)の第1段階。

## 背景・目的・結果

現在の `memory.ReadSources` は、存在する任意のMemory（組織・チーム・プロジェクト・工程区分のルール）を
集め、欠落ファイルを省略できる。担当AIへ「必ず渡す」と決まったルールには異なる契約が必要である。
本変更は、順序付きで指定された必須Markdownを毎回読み、欠損やencoding異常があれば
途中までの本文を返さない内部入口を追加する。利用側は再ビルド・再起動なしに次回の読込みから編集を反映できる。
これは本文取得部品の完成であり、Codexへの配信全体が完成したという意味ではない。

## APIと設計

`src/internal/steering` を作り、次の内部APIを置く。

```go
type RuleContent struct {
    Path string
    Text string
}

func ReadRules(rulesFS fs.FS, paths []string) ([]RuleContent, error)
```

1. `paths` は呼出側が解決した、FS root相対のslash pathの順序付き一覧。readerはphase推測、sort、
   root探索をしない。org→team→project→phaseの選択自体は後続の呼出側が担う。
2. 全pathをI/O前に検証し、`fs.ValidPath`でない値、`.`、backslashを含む値を`fs.ErrInvalid`をwrapして
   拒否する。nilおよびtyped-nil FSもpanicせず拒否する。空一覧は有効で、non-nil空sliceとI/O 0回になる。
3. 同一pathは最初の出現だけを読み、その順序を保持する。本文の上書き統合はしない。
4. 各ファイルを毎回 `fs.ReadFile` で読む。missingも必須ルールのerror。読込途中のerror、不正UTF-8は
   pathとcauseを保持し、結果をnilにする。不正UTF-8のcauseは `fs.ErrInvalid`。
5. 本家のfatal UTF-8 `TextDecoder`に合わせて先頭BOMを1つだけ除去する。それ以外のCRLF、空行、
   comment、frontmatter、途中や2つ目のBOMは正規化しない。
6. 既存 `memory.BuildBundle` に本文を渡し、実質的なルールを含まないtemplateを除外する。
   comment除去は判定にだけ使い、採用する本文自体のcommentを消さない。
7. 入力一覧を変更せず、結果sliceは呼出しごとに独立。cache、埋込み、切詰めや任意のsize capを追加しない。
8. 実filesystemの呼出側はMemory rootを `os.OpenRoot` で開いた `Root.FS()` を渡す。readerはRootを閉じない。
   root内の通常fileと相対symlinkを許し、root外・絶対symlinkから外部本文を返さない。

既存 `memory.ReadSources` を必須読込みへ変更する案は、任意ファイルの収集契約を破るため採用しない。
substantive判定（本文があるかの判定）を再実装せず `BuildBundle` を再利用することで、文字・改行の境界が
二重管理されないようにする。標準ライブラリだけを使う。

## 所有範囲

- `src/internal/steering/rules.go`、`rules_test.go`、`rules_integration_test.go`
- `.github/workflows/ci.yml` の既存Memory integration stepをsteeringにも拡張する箇所
- `docs/architecture.md`、`docs/development.md` のreader契約と対象検証案内
- 本計画、マイルストーンRAM、`docs/ram/README.md`の索引

親がIssue・計画・RAMを準備し、その後はGo実装担当1名だけが書き込む。既存の未追跡
`docs/implementation-overview.html` と本家原稿140件は変更・追跡・削除しない。
graph、orchestrator、CLI、state/audit形式、人間の承認取得元には変更しない。

## TDDと受入条件

`verification_mode=loop` で1 behaviorずつREDを観測し、最小GREENとrefactorを繰り返す。

1. 指定順、同一pathのfirst-wins、空一覧のnon-nil結果とI/O 0回。
2. missing、途中read failure、不正UTF-8をすべてerrorとし、部分本文が返らないこと。
3. BOMを先頭1個だけ除去、CRLF・comment・後続BOM等を保持すること。
4. 本家template・見出しだけの文書は除外、利用者が書いた本文やblockquoteは保持すること。
5. 同じFS/Rootへ配置したMarkdownを書き換えた後、同じAPIを呼ぶと新本文になること。
   入力・結果の所有権、nil/typed-nil、不正pathでpanicもI/Oも起こらないこと。
6. integration tag付き実FS testでRoot継続利用、通常file、root内相対symlink、外向きsymlinkを確認する。
   symlink skipは既存Memory testと同じWindows権限・未対応の理由付き条件だけにする。

targetedコマンドは `GOTOOLCHAIN=go1.26.8 go test -count=1 -run '^TestReadRules' ./src/internal/steering`。
実FSは同じコマンドに `-tags=integration` を付ける。`gofmt`適用はreview前のloopで行う。

## 独立review・final・merge

固定base/headを `verification_mode=review` で独立確認する。blocking findingを修正して差分が安定したら、
親がGo 1.26.8で次のread-only finalを1回開始する。

- `go test -count=1 -shuffle=on ./...`
- `go test -race -count=1 -shuffle=on ./...`
- `go test -tags=integration -race -count=1 -shuffle=on ./...`
- `go vet ./...` と `go vet -tags=integration ./...`
- `gofmt -l src`、`go mod tidy -diff`、`git diff --check`
- darwin/linux/windows × amd64/arm64のCLIとsteering integration test binaryのcross compile
- 対象Goファイルの `gopls check`、原稿の既存SHA256SUMS照合

内部部品なので本PRの配布CLI E2Eは非該当。実際の配信接続PRで別途行う。
全検証後に対象が変われば証拠を再取得する。現在headのGitHub Quality 2構成・Build 6構成と
新integration検証の成功後にmerge commit方式でmergeし、main反映とIssue closeを確認する。

## 本家根拠・境界・リスク

固定2.6.123の `docs/実装_aidlc-workflows/core/tools/aidlc-steering.ts:85-115` が必須読込み、
first-wins、fatal UTF-8、全結果破棄、filterの根拠である。readerではstage選択・配信を行わない。
新しい意図的な差分は採用しない。root外symlink拒否は
[既承認Memory境界](2026-09-02-memory-source-reader-plan.md)を継承する。

`fs.FS`だけではsymlinkを封じ込めず、実FS callerの `os.Root.FS()` が必要。複数ファイルの並行編集の
原子的snapshot、mount/device等に対する完全sandboxは保証しない。Go 1.26.5以降の既存最終検証条件を守る。
失敗時に製品dataの移行や削除は不要。問題は通常の修正またはrevert PRで戻せる。

## 実施記録（Issue #89）

- 実装日: 2026-09-04
- 実装担当: `go_tdd_implementer`（verification mode: `loop`）
- 変更範囲: `src/internal/steering`のreaderとunit/integration test、既存Memory integration stepへの追加、
  `docs/architecture.md`・`docs/development.md`の契約と検証案内
- 外部Go module・外部tool・埋込み・既存Memory/graph/orchestrator/CLIの変更はなし

### TDD実施

各sliceは対象packageの`ReadRules` testだけを実行し、失敗原因が新behaviorの欠落であることを確認してから
最小実装を追加した。

| slice | RED evidence | GREEN evidence |
| --- | --- | --- |
| 指定順 | package未実装時の `go test -count=1 -run '^TestReadRules' ./src/internal/steering` — `no non-test Go files` | 同コマンド — `ok` |
| duplicate first-wins | 同コマンド — duplicate pathが重複して返る失敗 | 同コマンド — `ok` |
| 空一覧 | 最小readerがnon-nil・無I/Oを既に満たしたため、追加assertionを同コマンドで確認 — `ok` | 同コマンド — `ok` |
| path事前検証とnil/typed-nil FS | 同コマンド — `fs.ErrInvalid`なし、I/O実行、typed-nil panicの失敗 | 同コマンド — `ok` |
| missing/read error・部分結果破棄 | 同コマンド — errorのpath contextがない失敗 | 同コマンド — `ok` |
| 不正UTF-8・途中失敗 | 同コマンド — 不正bytesが本文として返る失敗 | 同コマンド — `ok` |
| BOM保持境界 | 同コマンド — 先頭BOMが残る失敗 | 同コマンド — `ok` |
| template filter・本文comment保持 | 同コマンド — heading/comment/templateが返る失敗 | 同コマンド — `ok` |
| fresh read・caller ownership | 同コマンドで追加assertionを作成し、毎回の読込みと独立sliceを確認 | 同コマンド — `ok` |
| 実FS Root境界 | `GOTOOLCHAIN=go1.26.8 go test -tags=integration -count=1 -run '^TestReadRules' ./src/internal/steering`でfixtureを追加 | 同コマンド — `ok` |

変更後に`gofmt -w src/internal/steering/rules.go src/internal/steering/rules_test.go src/internal/steering/rules_integration_test.go`
を適用し、次のfresh targeted testを再実行した。

```text
GOTOOLCHAIN=go1.26.8 go test -count=1 -run '^TestReadRules' ./src/internal/steering
ok   github.com/sori883/ai-dd/src/internal/steering  0.169s

GOTOOLCHAIN=go1.26.8 go test -tags=integration -count=1 -run '^TestReadRules' ./src/internal/steering
ok   github.com/sori883/ai-dd/src/internal/steering  0.489s

GOTOOLCHAIN=go1.26.8 go test -tags=integration -count=1 -run '^TestSteeringSymlinkErrorClassification$' ./src/internal/steering
ok   github.com/sori883/ai-dd/src/internal/steering  0.601s
```

### 変更ファイルと残余リスク

- `src/internal/steering/rules.go`
- `src/internal/steering/rules_test.go`
- `src/internal/steering/rules_integration_test.go`
- `.github/workflows/ci.yml`
- `docs/architecture.md`
- `docs/development.md`
- 本記録（既存の承認・索引内容は維持）

複数ファイルの同時編集に対する原子的snapshot、`os.Root`で防げないmount/device等の完全sandbox、
Go 1.26.5未満、Node/Bunの全OS path解釈、最新upstream・全配布物との完全互換は未確認のままである。
integrationのsymlink作成は既存Memory testと同じくWindowsのpermission・privilege・unsupportedだけを
理由付きskipとする。readerは内部APIとして未接続で、CLIやstage選択・配信を変更しない。
