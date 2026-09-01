# Intent切替の実装検証・配布E2E

- 実施日: 2026-09-01（JST）
- 結果: PASS。配布binaryの32起動すべてで期待exit・stdout/stderrと宣言済みの
  filesystem差分に一致した。cursor更新14回、無変更18回、skip・未実行caseなし。
- Issue: [#29](https://github.com/sori883/ai-dd/issues/29)
- 承認: [Intent切替の実装計画](../ram/decisions/2026-09-01-intent-switch-plan.md)
- source commit: `e4541fc2d65a0ad8eba79ded24debe27a90d085e`
- native実行: macOS / arm64、Go `1.26.4`。他OS向けcompileとは区別する。

## Artifactと再現条件

承認済みsandboxに未使用scenarioを排他的に作成した。既存scenario、通常の開発project、
本家snapshotは変更・削除していない。build時のrepository worktreeはcleanで、driverが
Go build infoのsource revisionと`vcs.modified=false`を確認した。この記録を含む後続commitは
文書だけで、binaryのsource commitとは区別する。

- [配布binary](/Users/const/sori883/haihu-aidlc/e2e/2026-09-01-intent-switch-JgRomm/aidlc)
- [実行driver](/Users/const/sori883/haihu-aidlc/e2e/2026-09-01-intent-switch-JgRomm/intent_switch_e2e_test.go)
- [全32起動の観測log](/Users/const/sori883/haihu-aidlc/e2e/2026-09-01-intent-switch-JgRomm/e2e.log)
- [build metadata](/Users/const/sori883/haihu-aidlc/e2e/2026-09-01-intent-switch-JgRomm/build.json)
- scenario: `/Users/const/sori883/haihu-aidlc/e2e/2026-09-01-intent-switch-JgRomm`
- binary size: `3,611,602 bytes`
- binary SHA-256: `b9746b1a64f165622d62bc71f17cf67c6994b4452f7e2204bc75b85d864da44b`
- driver SHA-256: `aa68b51bf85a3c26415e8b8917b6a161eca6760b06c555b04c26e0896d2b455c`
- observation log SHA-256: `642785c632c21a3ef30e4d83176577a72de1681dbc001c77cc7d0063198a8b06`
- build metadata SHA-256: `12fb8ea16508a4e6a49cb4ad35b1334582debab4013dc5467b979c2b802c8467`
- 注入version: `e2e-20260901-intent-switch`
- 注入commit: `e4541fc2d65a0ad8eba79ded24debe27a90d085e`

repository rootから次を実行した。

```sh
env CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -trimpath \
  -ldflags '-X github.com/sori883/ai-dd/src/internal/buildinfo.Version=e2e-20260901-intent-switch -X github.com/sori883/ai-dd/src/internal/buildinfo.Commit=e4541fc2d65a0ad8eba79ded24debe27a90d085e' \
  -o /Users/const/sori883/haihu-aidlc/e2e/2026-09-01-intent-switch-JgRomm/aidlc \
  ./src/cmd/aidlc
env AIDLC_E2E_EXPECTED_COMMIT=e4541fc2d65a0ad8eba79ded24debe27a90d085e \
  go test -count=1 -timeout=120s -v \
  /Users/const/sori883/haihu-aidlc/e2e/2026-09-01-intent-switch-JgRomm/intent_switch_e2e_test.go
```

driverは標準libraryだけを使う検証側のGo testである。子binaryは絶対pathで起動し、
環境は`PATH=`、空のroot変数、caseごとに必要なroot変数だけにした。各起動は10秒、
driver全体は120秒のtimeoutを持つ。fixture root、観測log、build metadataを排他作成するため、
同じscenarioでの再実行は意図的に拒否する。再確認時は既存artifactを消さず、新しいscenarioを使う。

## 配布E2Eの観測

exit 0は14回、exit 1は17回、予約済み未実装commandのexit 2は1回だった。
全32回でsignal終了はなく、期待した14回だけcursor差分を許可し、残り18回はfixture不変だった。

| 確認範囲 | 観測結果 |
| --- | --- |
| target解決 | exact directoryが重複slugより優先。一意slug、Ambiguous候補、registry-only拒否、orphan、case-sensitiveを一致 |
| CLI形式 | explicitとbareが同じ実directoryを選択。予約名`list`は明示switchで到達し、bareでは一覧を維持 |
| cursor | 同一targetをLFなし・0640から再保存し、`<dirName>\n`とmodeを維持。不在active-spaceは`default\n`だけ補完 |
| strict構文 | target欠落、raw help/-h、空target、余剰target、`--json`、重複project-dirをJSON error・exit 1で拒否 |
| root入力 | `--project-dir=`、明示 > AIDLC環境 > CLAUDE環境 > cwdを実binaryで一致 |
| project/link境界 | 初回project linkと境界内相対space linkは成功。外向き・絶対・broken space linkは拒否 |
| cursor型 | active-intent symlinkを拒否し、link先とprojectを無変更に維持 |
| 閉pipe | stdout閉鎖と両stream閉鎖ではexit 1でもcursor保存済み。stderr閉鎖の構文errorは無変更・exit 1 |
| build情報 | 注入version/commit、Go build infoのrevision、`vcs.modified=false`、binary SHA-256を一致 |

### 差分検査と保護fixture

各起動の前後にfixture root全体を再帰snapshotし、path集合、file種別・mode、通常fileの
内容SHA-256、symlink先を比較した。許可した`active-intent`と、不在時だけの`active-space`は
本文と必要なmodeを個別検査し、それ以外の差分やtemp残存を失敗にした。

保護fixtureにはregistry、全`aidlc-state.md`、knowledge audit、session binding、
current-session、rebind offer、UUID stamp、`.codex/config.toml`、無関係なproject data、
境界外link先を含めた。32回とも意図しない更新はなかった。別testでsnapshot検査器が
保護fileの内容変更を拒否することも確認した。

mtime、atime、owner、ACL、inode同一性はsnapshot比較の対象外である。permissionは
同一target再保存の0640で明示検査した。並行writerに対するsnapshot一貫性、mount/deviceを
含む完全sandbox、全OSのatomic性をこの結果からは主張しない。

## 最初のE2E driver失敗

最初の未使用scenario
`/Users/const/sori883/haihu-aidlc/e2e/2026-09-01-intent-switch-3kCw17` は、
製品不具合ではなくdriverの2件の誤りでFAILした。

1. 部分一致を検査する一覧出力にも空stdoutとの完全一致を要求していた。
2. 内向きspace link作成前に保護fixtureが同名directoryを作成していた。

失敗scenarioは上書き・削除せず、binary、driver、build metadata、部分観測logを保存した。
新しいscenarioで検査器を直し、検査器単独testを先にPASSさせてから32件を完走した。
失敗scenarioのsummary行はsubtest失敗前に到達したcase数だけを表し、PASS証拠には使わない。

## TDDと独立review

実装担当は13個のobservable sliceでassertion RED→GREENを報告した。内訳はworkspace 5、
CLI 6、main 2である。compile failure、構造整理、追加時点GREENのguardはRED件数へ含めない。
ただし#2〜#13のraw shell transcriptは永続化されておらず、test名・command形・失敗要旨の
turn要約だけが残る。最終GREENは再現可能なtest sourceとfresh検証で確認したが、過去の遷移を
独立再現した証拠とは扱わない。

初回固定head `e1f5e571bc7cfdf39577684cf29082f78ba27dbc` の独立reviewではP0/P1なし、
次のP2 1件とP3 2件を指摘した。

- 同一target再保存の回帰test不足。
- 記載したtargeted commandが共有cursorの一部失敗testを選択しない。
- `active-intent`だけを更新するという説明がactive-space補完と矛盾する。

同じ実装担当が、同一targetをLFなし・0640から再保存する追加時点GREENのguardと文書修正を行った。
固定head `e4541fc2d65a0ad8eba79ded24debe27a90d085e` の再reviewはNo findingsである。
実装担当、親、reviewerは通常・integration・shuffle/race・integration race、vet両構成、
tidy差分、gofmt、gopls、diff checkを分離して通過させた。

最終通常coverageはmain 92.6%、CLI 98.6%、workspace 83.6%、buildinfo 100%、全体88.0%。
profileは`/tmp/ai-dd-issue-29-final.Xm4Wnw/coverage.out`、SHA-256は
`ec40f3b6e47bc9d0e05625f710fa1bf0e35052831d1965ff88bc1a99f10472a9`である。

## 6構成cross compile

最終source commitから`CGO_ENABLED=0`で、darwin・linux・windowsのamd64/arm64について
CLIとworkspace integration test binaryをcross compileした。合計12 artifactで、
各OSでのnative実行証拠ではない。

| OS | amd64 | arm64 |
| --- | --- | --- |
| darwin | PASS | PASS |
| linux | PASS | PASS |
| windows | PASS | PASS |

artifact rootは`/tmp/ai-dd-issue-29-cross-final.RSDPcU/`。12 fileのSHA-256 manifestを
path順で連結して得たSHA-256は
`541d63f2180d36be03305d1645e656cc254a9b42364a69d7752c15e67a940d39`である。
一時directoryのため長期保持は保証しない。

## 本家との差分と限界

ローカル本家2.6.123からの承認済みの意図的な差分は、active-intentの安全な一時file置換と
保存失敗通知、既存projectと`os.Root`・cursor型の境界、strict CLIの3点である。
実装・review・E2Eを通じて新しい意図的な差分は追加していない。

session binding、rebind offer、UUID stamp、audit、Intent作成は段階的な未実装である。
一覧と保存間の並行変更、lock、multi-file transaction、rollback、全OS atomic、
fsync・crash耐久、owner・ACL・特殊mode・hardlink identity、mount/deviceを含む完全sandbox、
Linux/Windowsでのnative実行、最新upstream全体との一致は未検証である。
外部Go module/tool、CI変更は追加していない。
