# Intent一覧の実装検証・配布E2E

- 実施日: 2026-09-01（JST）
- 結果: PASS。配布binaryの45起動すべてで期待exit・stdout/stderrと
  filesystem不変に一致した。skip・未実行caseなし。
- Issue: [#25](https://github.com/sori883/ai-dd/issues/25)
- 承認: [Intent一覧の実装計画](../ram/decisions/2026-09-01-intent-list-plan.md)
- source commit: `fb016aa28e83902df785ee82795c9c156daabcdf`
- native実行: macOS / arm64、Go `1.26.4`。他OS向けcompileとは区別する。

## Artifactと再現条件

承認済みsandboxに未使用scenarioを排他的に作成した。
既存scenario、通常の開発project、本家snapshotは変更・削除していない。
build時のworktreeはcleanで、`go version -m`とdriver内の`debug/buildinfo`の両方で
source revisionと`vcs.modified=false`を確認した。
この記録を含む後続commitは文書のみで、binaryのsource commitとは区別する。

- [配布binary](/Users/const/sori883/haihu-aidlc/e2e/2026-09-01-intent-list-MGe8u6/aidlc)
- [実行driver](/Users/const/sori883/haihu-aidlc/e2e/2026-09-01-intent-list-MGe8u6/intent_list_e2e_test.go)
- [全45起動の観測ログ](/Users/const/sori883/haihu-aidlc/e2e/2026-09-01-intent-list-MGe8u6/e2e.log)
- [build metadata](/Users/const/sori883/haihu-aidlc/e2e/2026-09-01-intent-list-MGe8u6/build.log)
- scenario: `/Users/const/sori883/haihu-aidlc/e2e/2026-09-01-intent-list-MGe8u6`
- binary size: `3,577,074 bytes`
- binary SHA-256: `2ce20b1a6917587338e912eda1f007164c6df0b5e4426c127d0ab1d0b9a19b4f`
- driver SHA-256: `ffafcb6fff9debac8e088ce7be03bcf3de1294c3d5c8858ead065798930cf577`
- observation log SHA-256: `503ea17f8d610f8fa23efdab9c5795bdef3bd11f019b98d86ee9a3c2883f2a9e`
- build log SHA-256: `db18aaf12a8ad3bcaef3aeaef677cc9eab57ad747181b54fc570b359e479a8a6`
- 注入version: `e2e-20260901-intent-list`、注入commit: `fb016aa`

repository rootから次を実行した。

```sh
env CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -trimpath \
  -ldflags '-X github.com/sori883/ai-dd/src/internal/buildinfo.Version=e2e-20260901-intent-list -X github.com/sori883/ai-dd/src/internal/buildinfo.Commit=fb016aa' \
  -o /Users/const/sori883/haihu-aidlc/e2e/2026-09-01-intent-list-MGe8u6/aidlc \
  ./src/cmd/aidlc
env AIDLC_E2E_SCENARIO=/Users/const/sori883/haihu-aidlc/e2e/2026-09-01-intent-list-MGe8u6 \
  AIDLC_E2E_EXPECTED_COMMIT=fb016aa28e83902df785ee82795c9c156daabcdf \
  go test -count=1 -timeout=120s -v \
  /Users/const/sori883/haihu-aidlc/e2e/2026-09-01-intent-list-MGe8u6/intent_list_e2e_test.go
```

driverは標準ライブラリだけを使う検証側のGo testである。
子binaryは絶対pathで起動し、環境は`PATH=`、空のroot変数、caseに必要なroot変数だけにした。
各起動は10秒、全driverは120秒のtimeoutを持つ。
fixture rootと観測logは排他作成するため、同じscenarioでの再実行を意図的に拒否する。
再確認時は既存artifactを削除せず、新しいscenarioを使用する。

## 配布E2Eの観測

通常・fallback・構文・接続・link・既存互換の40ケースと、
実closed pipeの5ケースを実行した。
exit 0は27回、exit 1は15回、未知commandのexit 2は2回、
SIGPIPE終了を維持する未知commandは1回だった。全45回が期待どおりである。

| 確認範囲 | 観測結果 |
| --- | --- |
| human / JSON / alias | `intent list`とbare `intent`が同じ結果。空、activeなし、末尾LF、field順を完全一致 |
| registry相関 | registry順、exact、legacy、duplicate claim/active、registry-onlyを一致 |
| orphan | date-prefix表示slugと、Go rune順とは異なる補助平面/私用領域のUTF-16順を一致 |
| JSON projection | top-level active、null、repos常時array、scope非公開、row field順を一致 |
| fallback | registry不在、不正JSON、非array、read errorをdisk orphanへfallback |
| fail closed | valid array内の`repos:["api",null]`を部分表示せずJSON error・exit 1 |
| root入力 | flagの前・途中・後・等号形、明示 > AIDLC環境 > legacy環境 > cwd、相対flagを一致 |
| strict構文 | 未知/重複/欠落/空flag、値付きJSON、余剰位置引数をJSON error・exit 1 |
| 未実装command | `intent create`と`intent target`を従来形式のunknown・exit 2として維持 |
| link境界 | 初回project linkと内向き相対intents linkは成功。外向き相対/絶対linkは拒否 |
| child不在 | intents directory不在とbroken内向きlinkをspace metadata付き空一覧として成功 |
| closed pipe | 認識済みhuman/JSONのstdout、構文errorのstderr、両stream閉鎖でexit 1 |
| SIGPIPE非回帰 | 未知intent commandは出力準備hookを使わず、閉stderrでSIGPIPE終了を維持 |
| help/version | workspaceを読まず、注入build情報と更新済みhelpを正確に表示 |

### 全fixture不変の検査

各起動の直前直後にfixture root全体を再帰snapshotした。
path集合、file種別・mode、size、mtime、通常file内容SHA-256、symlink先を比較し、
45回すべてで差分なしだった。

保護fixtureにはactive-space、active-intent、intents.json、全aidlc-state.md、
audit shard、session binding、project notes、境界外target、project/intents symlinkを含む。
registry fallbackやfail-closed、error出力、closed pipeでも作成・修復・切替・監査追記はなかった。
別testで同じsnapshot検査器がfile内容変更を検知して失敗することも確認した。

atime、owner、ACL、inode同一性は比較対象外である。
snapshotは一回の起動前後比較で、並行writerに対する一貫したsnapshotや
mount/deviceを含む完全sandboxを保証しない。

## TDDと独立review

実装担当は初期15件で実際のrunnable assertion REDを観測し、GREENを確認した。
compile failure、Dependenciesの挙動不変refactor、追加時点でGREENのguardは件数に含めない。

初回固定head `2470882ea660600932c9f14738e0222cebd81762` の独立reviewで、
`repos:["api",null]`を空文字として受理するP1を発見した。
現headで同caseがerrorなし・`["api",""]`となる16件目のREDを観測し、
RawMessageごとのstring token検証を追加した。修正後はnull/number要素を拒否し、
missing/null/空/string配列を維持した。

- 初期TDD log: `/tmp/ai-dd-issue-25-tdd.log`、
  SHA-256 `cb46423e45266ba5e1b82ade540816d7d01ffd32e3f9f25f4ed1d16f4ff2cf61`
- 初期fresh verification: `/tmp/ai-dd-issue-25-verification.log`、
  SHA-256 `2e6377c31b9ee462584c0e11507161a80644fc5a749e34fb1b800bef5eb158ef`
- review修正TDD/verification: `/tmp/ai-dd-issue-25-review-fix.log`、
  SHA-256 `26f70a7d2312b00495a069feeb98cf09f9a467fd8c297ab12e53774a14285fb1`
- 最終coverage profile: `/tmp/ai-dd-issue-25-final-coverage.out`、
  SHA-256 `d411d1c8be30d5ff09763cf36bc8114c4b8f3b5719794f74906f124aec42bc8a`

最終headに対して実装担当、親、独立reviewerが分離して検証した。

```sh
go test -count=1 ./src/internal/workspace ./src/internal/cli ./src/cmd/aidlc
go test -tags=integration -count=1 ./src/internal/workspace
go test -count=1 -shuffle=on ./...
go test -count=1 -race -shuffle=on ./...
go test -tags=integration -count=1 -race -shuffle=on ./...
go vet ./...
go vet -tags=integration ./...
go mod tidy -diff
gofmt -l src
git diff --check
```

すべてPASSし、goplsの通常/integration診断もなかった。
最終coverageはmain 90.9%、CLI 98.3%、workspace 81.0%、buildinfo 100%、
全体86.1%だった。

最終独立reviewの比較範囲は
base `d770479230b7a3ca2d5eb572207330de06809156` から
head `fb016aa28e83902df785ee82795c9c156daabcdf`。
P1解消後はblocking findingなし、追加P0-P2なしだった。
外部配布E2Eと過去のTDD遷移はreviewerが独立再実施したものではない。

## 6構成cross compile

同じsource commit・clean worktreeから`CGO_ENABLED=0`で成功させた。
各targetのCLIを`go build -trimpath`、workspace integration・CLI・mainの
3 test binaryを`go test -c`でcompileした。合計6 CLIと18 test binaryである。

| OS | amd64 | arm64 |
| --- | --- | --- |
| darwin | PASS | PASS |
| linux | PASS | PASS |
| windows | PASS | PASS |

artifact rootは`/tmp/ai-dd-intent-list-build.vwFp7r/`。
cross-build log SHA-256は
`79455dde43722fa74ebaaa7afd5ca28b83f5fce6849dfec720a39d42aa812938`。
一時directoryのため長期保持を保証しない。
この結果はcompile可能性であり、Linux/Windowsでのnative実行証拠ではない。

## 本家との差分と限界

比較対象はローカルAI-DLC 2.6.123の静的確認範囲である。
承認済みの意図的な差分は、不正registry rowのfail-closed、
strict CLIとproject-dir等号形、既存project/os.Root境界、
Go標準JSON encoderとshort-write検出の4点である。
今回、新しい意図的差分は追加していない。

session binding、audit、intent作成・切替、state/status修復、registry migrationは
段階的な未実装範囲である。表示文が未実装commandを案内する点は本家表示互換を維持した。
Linux/Windows native実行、並行更新中の一貫性、mount/deviceを含む完全sandbox、
最新upstream全体との一致、未導入のgovulncheckは未検証である。
外部Go module/tool、CI変更は追加していない。
