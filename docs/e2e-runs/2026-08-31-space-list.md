# Space一覧の実装検証・配布E2E

- 実施日: 2026-08-31（JST）
- 結果: PASS。配布binaryの53起動で期待終了codeに一致し、全53回でfixtureの前後snapshotに差分なし。
- Issue: [#21](https://github.com/sori883/ai-dd/issues/21)
- 承認: [Space一覧の実装計画](../ram/decisions/2026-08-31-space-list-plan.md)
- source commit: `50fae80e4f7a05331af803c71d9d4c8c1b9d8781`
- native実行: macOS / arm64、Go `1.26.4`。他OS向けcross compileは下記で区別する。

## Artifactと再現条件

未使用scenarioを排他的に作成し、既存scenarioや通常の開発projectは変更・削除していない。
build時のworktreeはcleanで、`go version -m`でも上記revisionと`vcs.modified=false`を確認した。
後続の検証記録commitは文書のみで、検証binaryのsource commitとは区別する。

- [配布binary](/Users/const/sori883/haihu-aidlc/e2e/2026-08-31-space-list-URvnCc/aidlc)
- [実行driver](/Users/const/sori883/haihu-aidlc/e2e/2026-08-31-space-list-URvnCc/space_list_e2e_test.go)
- [全ケースの生ログ](/Users/const/sori883/haihu-aidlc/e2e/2026-08-31-space-list-URvnCc/e2e.log)
- scenario: `/Users/const/sori883/haihu-aidlc/e2e/2026-08-31-space-list-URvnCc`
- binary size: `3,081,762 bytes`
- binary SHA-256: `5eceb8015890ca2fe678a487a3ea7f3536a1ea1c4ca78bc99ac8a783dc7960f9`
- driver SHA-256: `c0d7324fbd888222a091c480f40cc7ad6ef6c7e630ce6216585d0b312ca5beed`
- log SHA-256: `48bbbc576c78e6ec2fac9ba106645f9b5ef2a229d2c9ae267fd471d726500dcc`
- 注入version: `e2e-20260831-space-list`、注入commit: `50fae80`

repository rootで次を実行した。

```sh
env CGO_ENABLED=0 go build -trimpath \
  -ldflags '-X github.com/sori883/ai-dd/src/internal/buildinfo.Version=e2e-20260831-space-list -X github.com/sori883/ai-dd/src/internal/buildinfo.Commit=50fae80' \
  -o /Users/const/sori883/haihu-aidlc/e2e/2026-08-31-space-list-URvnCc/aidlc \
  ./src/cmd/aidlc
env AIDLC_E2E_EXPECTED_COMMIT=50fae80 go test -tags=integration -count=1 -timeout=60s -v \
  /Users/const/sori883/haihu-aidlc/e2e/2026-08-31-space-list-URvnCc/space_list_e2e_test.go
```

driverは標準ライブラリだけを使う検証側のGoテストであり、配布binaryの実行依存ではない。
子binaryは絶対pathで起動し、環境は`PATH=`と各ケースに必要なroot/session変数だけにした。
この実行ではGo・Node等をPATHから呼べない状態で、binary単体の動作を確認している。
各起動に10秒timeout、driver全体に60秒timeoutを設定した。

driverは`fixtures/`を`os.Mkdir`で排他作成するため、既存scenarioでの再実行は意図的に拒否する。
再確認には新しいscenarioを用意し、既存fixtureを消したり上書きしたりしない。

## 配布E2Eの観測

通常・構文・接続・既存互換の45ケースと、実closed pipeの8ケースを実行した。
個々のargv・cwd・env・期待/実際の終了code・stdout・stderrは生ログの53件の`OBSERVATION`へ保存した。

| 確認範囲 | 観測結果 |
| --- | --- |
| 明示list、bare space、human/JSON、help/version | 期待する出力とexit 0、stderr空 |
| flagの前・途中・後配置、project-dirの分離形/等号形 | 同じproject・一覧へ到達 |
| default補完、未知cursor、欠落/空/読取不能cursor | 既存readerのfallbackを維持。未知cursorでは全行inactiveを保持 |
| UTF-16順、引用符・HTML文字を含む名前 | alphaがdefaultより前、emojiがU+E000より前。JSONのescape・field・boolean型・1行LFを確認 |
| flag > AIDLC_PROJECT_DIR > CLAUDE_PROJECT_DIR > cwd、相対指定 | それぞれ選択したprojectだけの一覧を返す |
| 初回project symlink、境界内相対symlink | 参照可能 |
| 外向きspaces/cursor symlink、project内を指す絶対symlink | 拒否された読取に対応するdefault fallback |
| 先頭のbroken entry symlink | Stat失敗で列挙を止め、後続entryを追加しない |
| session bindingを置いたshared cursor一覧 | 今回の段階どおりbindingではなくshared cursorを選ぶ |
| project不在/file、未知・重複・欠落・値付きflag、余剰引数 | 認識済み操作はJSON error・exit 1。低優先projectへ救済しない |
| createのJSON拒否・既存space拒否、未知command/subcommand | 既存契約を維持。未知commandは従来形式・exit 2 |
| stdout closed（human/JSON/bare） | exit 1、開いたstderrにJSON error |
| stderr closed（構文/bare/root）、両stream closed（human/JSON） | SIGPIPE終了でなくexit 1。閉じたstreamへの内容到達は保証しない |

未知cursorの実際のJSONは以下で、default行もfalseのままである（末尾LFあり）。

```json
{"active":"default","spaces":[{"name":"alpha","active":false},{"name":"default","active":false},{"name":"team-alpha","active":false}]}
```

全ケースで、projectと境界外のlink先を含むfixture全体のpath、file種別/mode、mtime、
通常fileの内容SHA-256、symlink先を比較した。53回とも無変更で、未作成projectも作られなかった。
読み取りに伴うatimeは比較しない。閉じたstreamはログの`closed_stream`で識別し、
captureが空でも「内容が相手に届いた」とは扱わない。

## TDDの証拠

実装担当が以下のassertion REDを観測し、各最小変更後に同じ対象を含むテストのGREENを確認した。
compile failureだけのREDではない。親はhandoffと対応するテストを確認し、上記配布E2Eを別に実行した。

`go test -count=1 -run '^TEST$' -v PACKAGE`を使用。
Wは`./src/internal/workspace`、Cは`./src/internal/cli`、Mは`./src/cmd/aidlc`。
3・4では`-tags=integration`を追加した。

| # | TEST / package | REDで観測した失敗 |
| --- | --- | --- |
| 1 | TestReadSpacesRejectsRelativeRoot / W | fs.ErrInvalidでなくnil error |
| 2 | TestReadSpacesProjectOpenError / W | open未実行・原因error欠落 |
| 3 | TestReadSpacesProjectClose / W | Close未実行・Root未Close・原因欠落 |
| 4 | TestReadSpacesSharedCursorListing / W | 期待一覧でなくnil |
| 5 | TestRunSpaceListHuman / C | 未知command、exit 2、callback未実行 |
| 6 | TestRunSpaceListJSON / C | --jsonが未知flag、exit 1 |
| 7 | TestRunSpaceBareAlias / C | human/JSONとも未知command、exit 2 |
| 8 | TestRunSpaceListExtraArguments / C | 余剰引数でcallback実行、exit 0 |
| 9 | TestRunSpaceListDuplicateJSON / C | 重複を受理、exit 0 |
| 10 | TestRunSpaceListShortStdoutWrite / C | human/JSONともshort writeを成功扱い |
| 11 | TestRunSpaceListOutputPreparation / C | 8経路でprepareが欠落 |
| 12 | TestRunHelpIncludesSpaceList / C | list/bareの構文表示が欠落 |
| 13 | TestSpaceListerRootInput / M | getwd/env/read未実行、返値nil |
| 14 | TestMainSpaceList / M | 実mainの未接続callbackによるpanic、exit 2 |

追加時点でGREENだったflag配置、FS境界、実pipe等のguardは14件へ数えない。
Unixのmain回帰では一覧の実pipe16ケースでexit 1と無変更、
既存help/version/未知command等8ケースで従来のSIGPIPE動作を確認した。

## Fresh検証と独立レビュー

実装担当はGoソースをfreezeした後、以下をすべて成功させた。
親によるその後の変更はRAM・文書の具体化だけで、Goソースを変更していない。

```sh
go test -count=1 ./src/internal/cli ./src/cmd/aidlc ./src/internal/workspace
go test -tags=integration -count=1 ./src/internal/workspace
go test -count=1 ./...
go test -count=1 -shuffle=on ./...
go test -count=1 -race -shuffle=on ./...
go test -tags=integration -count=1 -race -shuffle=on ./...
go test -count=1 -shuffle=on -coverprofile=/tmp/aidlc-space-list-coverage.MBzWD1 ./...
go vet ./...
go vet -tags=integration ./...
go mod tidy -diff
gofmt -l src
git diff --check
```

変更Goファイルのgopls通常/integration診断もなし。
gopls専用MCPは未公開のため、既存CLI `v0.23.0`を使った。
coverage付き実行でも実main回帰は成功した。子mainのcoverageは既存方式どおり別directoryへ隔離するため、
ここではそのcoverageを統合済み・全経路網羅とは扱わない。

別担当のread-onlyレビューは固定base `eb18da8943a9e09f388f5d5d1345e39ddebc96c9`から
head `50fae80e4f7a05331af803c71d9d4c8c1b9d8781`を対象とし、blocking findingなし。
追加Goテスト、承認計画・引数の具体化、docsを照合した。
独立実行したintegration込み全体race/shuffle、integration vet、main/CLIのcoverage付きテスト、
ReadSpaces targeted integration、固定diff-checkもPASS。
このレビューは本記録追加前の製品差分に対する結果で、実装時REDを別担当が再現したものではない。

## 6構成cross compile

親が同じsource commitから`CGO_ENABLED=0`で以下を成功させた。
各targetでCLIを`go build -trimpath`し、workspace（integration tag）・CLI・mainの3 test binaryを
`go test -c`で別にcompileした。合計6 CLIと18 test binaryであり、各OSで実行した証拠ではない。

| OS | amd64 | arm64 |
| --- | --- | --- |
| darwin | PASS | PASS |
| linux | PASS | PASS |
| windows | PASS | PASS |

artifactと実行command/outputは`/tmp/ai-dd-space-list-build-ESox7t/`に保存した。
[cross-build生ログ](/tmp/ai-dd-space-list-build-ESox7t/cross-build.log)のSHA-256は
`c8cf355082f9bcf1b87817c7f8b8b41d9f4a711fe95d8044afc7ebed9c6a9a42`。
一時directoryのため長期保持を保証しない。CIの6target buildとは別のローカル実行結果である。

## 限界

本家ローカル2.6.123との意図的な差分3件と影響は[承認済み差分](../architecture.md#space一覧の意図的な差分)を参照する。
reader内部のerror吸収・部分一覧、stdout失敗時の部分出力、Rootのmount/device等の限界は変えていない。
snapshot比較は、このfixtureに対する無変更の証拠であり、並行更新の一貫性や完全sandboxを保証しない。

Linux/Windowsでの配布binaryのnative実行、未導入のgovulncheckは未実施。
新規tool/moduleを追加していない。session binding、space/intent切替、intent/status、install資産展開や
完全なworkspace lifecycleは今回の実装・検証範囲外である。
