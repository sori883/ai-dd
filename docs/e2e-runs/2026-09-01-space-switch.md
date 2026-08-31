# Space切替の実装検証・配布E2E

- 実施日: 2026-09-01（JST）
- 結果: PASS。配布binaryの76起動すべてで期待exitとfilesystem差分に一致した。
  予定した更新は32回（space作成1回を含む）、無変更は44回。skip・未実行caseなし。
- Issue: [#23](https://github.com/sori883/ai-dd/issues/23)
- 承認: [Space切替の実装計画](../ram/decisions/2026-09-01-space-switch-plan.md)
- source commit: `39f1f58acdb60f6e3225e6e432cc9040406fa6f0`
- native実行: macOS / arm64、Go `1.26.4`。他OS向けcompileとは区別する。

## Artifactと再現条件

未使用scenarioを排他的に作成した。既存scenario、通常の開発project、本家snapshotは変更・削除していない。
build時のworktreeはcleanで、`go version -m`でも上記revision・`vcs.modified=false`を確認した。
この実施記録等の後続commitは文書のみで、binaryのsource commitとは区別する。

- [配布binary](/Users/const/sori883/haihu-aidlc/e2e/2026-09-01-space-switch-oCkM67/aidlc)
- [実行driver](/Users/const/sori883/haihu-aidlc/e2e/2026-09-01-space-switch-oCkM67/space_switch_e2e_test.go)
- [全ケースの生ログ](/Users/const/sori883/haihu-aidlc/e2e/2026-09-01-space-switch-oCkM67/e2e.log)
- [build command・metadata](/Users/const/sori883/haihu-aidlc/e2e/2026-09-01-space-switch-oCkM67/build.log)
- [snapshot検査器の単独テスト](/Users/const/sori883/haihu-aidlc/e2e/2026-09-01-space-switch-oCkM67/oracle.log)
- scenario: `/Users/const/sori883/haihu-aidlc/e2e/2026-09-01-space-switch-oCkM67`
- binary size: `3,332,178 bytes`
- binary SHA-256: `02507c579938d2b0ee030cd42b26db7fd1f19deef6313c030186c9a2abb7708d`
- driver SHA-256: `4af5f79a6a9109ce0338bd36d3d7f213b77e1a0febe85da1ac1b075d85a11465`
- log SHA-256: `8986d2b75554db3cad1fc27fa50db273975719c61c7848806b9465ed7d0329c6`
- 注入version: `e2e-20260901-space-switch`、注入commit: `39f1f58`

repository rootから次を実行した。full runの前に、同じdriverの
`TestSnapshotDeltaOracle`だけを実行し、検査器自体の回帰も確認した。

```sh
env CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -trimpath \
  -ldflags '-X github.com/sori883/ai-dd/src/internal/buildinfo.Version=e2e-20260901-space-switch -X github.com/sori883/ai-dd/src/internal/buildinfo.Commit=39f1f58' \
  -o /Users/const/sori883/haihu-aidlc/e2e/2026-09-01-space-switch-oCkM67/aidlc \
  ./src/cmd/aidlc
env AIDLC_E2E_EXPECTED_COMMIT=39f1f58 go test -tags=integration -count=1 -timeout=120s -v \
  /Users/const/sori883/haihu-aidlc/e2e/2026-09-01-space-switch-oCkM67/space_switch_e2e_test.go
```

driverは標準ライブラリだけを使う検証側のGoテストであり、配布binaryの実行依存ではない。
子binaryは絶対pathで起動し、環境は`PATH=`と各caseに必要なroot変数だけにした。
Go・Node等をPATHから呼べない状態で、binary単体の動作を確認している。
各起動に10秒、driver全体に120秒のtimeoutを設定した。

driverは`fixtures/`を`os.Mkdir`で排他作成する。既存scenarioでの再実行は意図的に拒否する。
再確認には新しいscenarioを用意し、既存fixtureを消したり上書きしたりしない。

## 配布E2Eの観測

通常・構文・接続・既存互換の71ケースと、実closed pipeの5ケースを実行した。
argv・cwd・env・期待/実際のexit・stdout・stderr・期待更新pathを76件の`OBSERVATION`へ保存した。
exit 0は37回、exit 1は36回、従来の未知commandのexit 2は3回で、すべて期待どおりだった。

| 確認範囲 | 観測結果 |
| --- | --- |
| create→list→switch→list→default→list | 作成で自動切替せず、明示切替後だけ選択を反映。正確な出力・cursor本文 |
| 合成default・未知名 | 空projectでもdefaultを保存でき、space雛形は作らない。未知名は無変更 |
| 名前 | Unicode小文字化、48文字上限、数字prefix、空白のintent fallback、既存list/create/switch/Helpを確認 |
| 現在cursor | 未知cursorでも切替可能。同じtargetの余白付きcursorを正確なLF付き本文へ再保存 |
| project-dirとroot入力 | flagの全位置・等号形・相対指定、明示 > AIDLC環境 > legacy環境 > cwdを確認 |
| 初回project link・内向き相対space/aidlc link | 選択したRoot境界内で成功 |
| 外向き/絶対space・aidlc link、broken entry | 拒否・無変更。列挙途中エラーでは実在する後続targetもUnknown |
| cursorの内向き/外向き/絶対/dangling link、directory | 拒否・無変更 |
| hardlink cursor | cursorを置換し、別pathの旧内容を変更しない |
| aidlc/spacesが通常file | 合成defaultの保存は可能。aidlc自体が通常fileなら拒否 |
| aidlc親の書込権限なし | 非root実行でpermission error・exit 1、旧cursorと周辺dataは無変更 |
| project不在/file、不正引数、raw help/-h、JSON指定 | JSON error・exit 1。低優先rootへの救済や保存なし |
| bare alias未実装、space --json false、未知root command | 従来形式のstderr・exit 2。既存createの予約名拒否も維持 |
| stdout closed、両stream closedで正常な切替 | exit 1だがcursorは保存済み。開いたstderrにだけJSON errorが届く |
| stderr closedで構文失敗、両stream closedで未知target | exit 1・無変更。閉じたstreamへの内容到達は保証しない |
| 成功時に未使用のstderrだけclosed | stdoutの成功1行・exit 0、cursorを保存 |

### 差分検査と保護fixture

fixture全体についてpath、file種別/mode、mtime、通常fileの内容SHA-256、symlink先を前後比較した。
成功時も単にcursorと親を比較対象から外すのではなく、指定したfile本文・permission、
新規directoryのmode、保存に必要な親directoryのmtimeだけを許可した。その他の差分やtemp残存は失敗になる。
新規modeは0666/0777のprobeでその実行環境のumaskを観測し、既存cursorは0640の維持を確認した。
`TestSnapshotDeltaOracle`では、無関係な編集・想定外temp・誤ったcursor・permission変更を
検査器が拒否することを確認した。process-globalなumaskは変更していない。

保護fixtureは以下を含む。

- `aidlc/.aidlc-sessions/.current-session`、binding JSON、rebind offer、拡張子なしUUID stamp。
- `aidlc/spaces/team-alpha/intents/active-intent`、intentの`aidlc-state.md`。
- `aidlc/spaces/team-alpha/intents/intents.json`、intent配下の`audit/fixture.md`。
- `.codex/config.toml`、無関係なproject data、境界外link先。

保存先は[本家保存先調査](../ram/research/2026-09-01-space-switch-contracts.md#配布e2eで保護する本家の保存先)に基づく。
state本文・audit・offer等は無変更検証用の合成fixtureで、本家のsession/harness処理を実行した証拠ではない。
76回とも期待差分に一致し、意図しない更新やtemp残存はなかった。
atime、owner/ACL、inode同一性はsnapshot比較の対象外。hardlink caseも内容保持の確認で、
metadataや全OSの原子的置換を保証するものではない。

## TDDの証拠

実装担当が17 sliceで実際のassertion REDを観測し、最小変更後のGREENを確認した。
compile失敗や追加時点でGREENだったguardはRED件数に含めない。
親・独立レビュー担当も保存ログを照合したが、過去の実装時遷移を独立再現したとは扱わない。

生ログは`/tmp/aidlc-space-switch-tdd-DjWwmK/`の番号付き`*-red.log`・`*-green.log`。
focused commandは`go test -count=1 <package> -run '^<test>$'`、Iのみ`-tags=integration`を追加した。
W/Iはworkspace、CはCLI、Mはcmd/aidlc。GREENの一部は累積suiteへ拡大した。

| # | Test / package | REDで観測した失敗 |
| --- | --- | --- |
| 01 | TestSwitchSpaceInvalidNameBeforeOpen / W | 無効名でnil error |
| 02 | TestSwitchSpaceRejectsRelativeRoot / W | 相対rootでnil error |
| 03 | TestSwitchSpaceProjectOpenError / W | open未実行・原因error欠落 |
| 04 | TestSwitchSpaceSavesNormalizedName / I | 名前空・保存0回 |
| 05 | TestSwitchSpaceUnknownDoesNotSave / I | unknownが保存へ到達 |
| 06 | TestSwitchSpaceCreatesOnlySharedCursor / I | cursor不在 |
| 07 | TestSwitchSpaceRejectsCursorSymlink / I | linkを受理しsnapshot変更 |
| 08 | TestReplaceSpaceCursorPreservesPermissions / W | 置換tempが0666 |
| 09 | 同testへ順序assertion追加 / W | Chmod未実行 |
| 10 | TestReplaceSpaceCursorShortWrite / W | short writeでRenameへ到達 |
| 11 | TestReplaceSpaceCursorRetriesCollision / W | 初回衝突で終了 |
| 12 | TestRunSpaceSwitch / C | 未知command・exit 2 |
| 13 | TestRunSpaceSwitchInvalidRawName / C | 空名/helpでcallback実行 |
| 14 | TestRunSpaceSwitchShortStdoutWrite / C | short stdoutでexit 0 |
| 15 | TestRunHelpIncludesSpaceSwitch / C | helpに構文なし |
| 16 | TestSpaceSwitcherRootInput / M | 入力・callback未実行 |
| 17 | TestMainSpaceSwitchClosedPipes / M | 4ケースでSIGPIPE・exit -1 |

13の`-h`caseは元からGREENだったため、RED実績には含めない。
追加guardでは、最大10回のcollision retry、各error causeと複数Join、自己temp限定cleanup、
保存前の旧cursor保持、cleanup失敗時の残存、保存後Close失敗、Root逆順Close、link境界、
名前・root入力・strict args・hook順序・周辺data不変を確認した。

## Fresh検証と独立レビュー

実装担当はGoソースをfreezeした後、以下をすべて成功させた。
親も対象3packageとworkspace integrationを別に再実行し、freeze時の13 Go fileのSHA-256が
candidate commit前に変わっていないことを照合した。

```sh
go test -count=1 ./src/internal/workspace ./src/internal/cli ./src/cmd/aidlc
go test -count=1 -tags=integration ./src/internal/workspace
go test -count=1 ./...
go test -count=1 -shuffle=on ./...
go test -count=1 -race -shuffle=on ./...
go test -count=1 -tags=integration -race -shuffle=on ./...
go vet ./...
go vet -tags=integration ./...
go mod tidy -diff
gofmt -l src
git diff --check
go test -count=1 -shuffle=on -coverprofile=/tmp/aidlc-space-switch-tdd-DjWwmK/coverage.out ./...
```

通常の変更Go12 fileと、integration設定での13 fileへ`gopls check`も実行し、診断なしだった。
これら14ゲートの結果は`fresh-*.log`、親の別実行は
[parent-targeted.log](/tmp/ai-dd-space-switch-build-mT69DQ/parent-targeted.log)に保存した。
gopls専用MCPは未公開のため既存CLI `v0.23.0`を使用した。

通常coverageはmain 88.2%、CLI 99.2%、workspace 72.4%、buildinfo 100%。
integrationを含むcoverageとは区別する。main helper subprocessのcoverageは既存方式の
専用一時directoryへ隔離し、親のcoverageへ統合済みとは扱わない。

別担当のread-onlyレビューは固定base `7f9001260d4c93e7156a7b13d075466ba72b34ad`から
head `39f1f58acdb60f6e3225e6e432cc9040406fa6f0`を対象とし、blocking findingなし、P0/P1/P2指摘なしだった。
独立実行のintegration込み全体race/shuffle、integration vet、tidy、gofmt、固定diff-check、
通常/integrationのgoplsもPASS。integration込みcoverageはworkspace 99.0%、CLI 99.2%、main 88.2%だった。
このレビュー結果は本記録追加前の製品差分に対するもので、後続artifactの確認とは区別する。

## 6構成cross compile

親が同じsource commit・clean worktreeから`CGO_ENABLED=0`で成功させた。
各targetのCLIを`go build -trimpath`、workspace（integration tag）・CLI・mainの3 test binaryを
`go test -c`でcompileした。合計6 CLIと18 test binaryであり、各OSでの実行証拠ではない。

| OS | amd64 | arm64 |
| --- | --- | --- |
| darwin | PASS | PASS |
| linux | PASS | PASS |
| windows | PASS | PASS |

artifactとcommand/outputは`/tmp/ai-dd-space-switch-build-mT69DQ/`。
[cross-build生ログ](/tmp/ai-dd-space-switch-build-mT69DQ/cross-build.log)のSHA-256は
`f81d1201167c0f807ee9ed47876db2cbe7c524d3af8c29f52ba9163c93fa34be`。
一時directoryのため長期保持を保証しない。CIの6target buildとは別のローカル実行結果である。

## 本家との差分と限界

本家ローカル2.6.123のpublic CLI・共有cursor保存経路との承認済み差分は
[Space切替の意図的な差分](../architecture.md#space切替の意図的な差分)に記録した。
temp置換と保存error通知、既存projectのRoot境界とcursor型検査、strict argsの3点で、
それぞれdirectory書込権限・metadata、一部link配置、受理する入力の互換性に影響する。

Rename呼出前の失敗では旧cursor内容を保持するが、親directoryやcleanup失敗したtempは残り得る。
Rename以降・Root Close・stdout失敗では保存済みの場合があり、自動rollbackしない。
このE2Eでもstdout/両stream失敗後の保存済みcursorを観測した。
write/Chmod/Close/cleanup等の決定的な失敗注入はunit/integrationの別証拠であり、配布binaryへ
故障注入したとは扱わない。排他lock、並行更新の一貫性、全OS atomic、crash耐久性、
mount/deviceを含む完全sandboxは保証しない。

Linux/Windowsでの配布binaryのnative実行、未導入のgovulncheckは未実施。
外部module/toolは追加していない。bare switch alias、session/harness/audit連携、
intent作成・切替、status、install資産展開、完全なworkspace lifecycleは今回の対象外である。
