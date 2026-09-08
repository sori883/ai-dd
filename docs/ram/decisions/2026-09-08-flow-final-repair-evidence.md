# 四段階final失敗の修正証拠

Issue #130、承認済み四段階計画内の通常bug修正。work_unit_id=flow-final-repair、verification_mode=loop。
開始HEADは9769657c215f25ac2419cc3f76e12be761b0855c。親の委譲により単独writerで実施した。

## F1 未知Intent操作と閉pipe

親finalのTestMainRootCommandsKeepSIGPIPEは、unknownという名前のfixtureに現在有効なintent createを使っていた。
初回失敗はINVALID_TEST_FIXTUREであり製品REDに数えない。親確認後intent unknownへ訂正してもexit 2となる有効REDを観測した。
isMinimalが未知actionにもPrepareOutputを適用してSIGPIPEを無効化するためだった。
intentの公開actionだけを認識し、未知actionはroot診断の元のSIGPIPE境界を維持した。
既知actionの引数不正や実行経路は維持する。

- RED/GREEN: `go test -count=1 ./src/cmd/aidlc -run '^TestMainRootCommandsKeepSIGPIPE$'`、fixture訂正後exit 1 → 修正後exit 0。

## F2 同じ実行ファイルの別path表記

親live raw: `/private/var/folders/9w/921pjkys39q28sk4xsc0hs000000gn/T/aidlc-flow-live-309540584`。
両coordinatorは配置skillに記された/var/.../aidlcでcreate/listを実行したが、Preで全て拒否された。
minimalCommandは実行ファイルをEvalSymlinksで/private/var/...へ正規化し、hookは文字列完全一致だけで例外を認識していた。
SessionStartは発火しており、bind集計の問題ではない。一般cat拒否は選択前の既存境界として正しい。

絶対path同士のos.Stat/os.SameFileで同じ通常fileである場合だけ既存のCLI例外へ進むよう修正した。
別file、basenameのみ、解析不能なshellは従来の拒否を維持する。権限、Rule準備条件、証拠閾値は変更していない。
TestHookBootstrapBinaryIdentityで同一path/aliasのcreate/list許可と別file/basename拒否を検査した。

- RED/GREEN: `go test -count=1 ./src/internal/minimal -run '^(TestFlow|TestHook|TestSession)'`、aliasでexit 1 → 修正後exit 0。

## 境界検査と残件

以下は修正後exit 0。gofmtとgit diff --checkも成功。

- `go test -count=1 ./src/internal/cli -run '^(TestMinimal|TestFlow|TestParse)'`
- `go test -count=1 ./src/internal/minimal -run '^(TestFlow|TestHook|TestSession)'`
- `go test -count=1 ./src/cmd/aidlc -run '^(TestMainRootCommandsKeepSIGPIPE|TestFlowCommand.*)$'`

rawにはbootstrap以降の実行証拠がなく、後続の実機成功は未観測。実機/full/race/vetは親finalで再実行する。
fixture/prompt/検証閾値の変更は不要だった。過去の独立review修正を保持し、AGENTS.mdと未追跡参照資料は変更していない。

## 親boundary後の公開show/check回帰修正

F1で作ったaction一覧に、他familyと共用caseだったintent show/checkを落とした不備を親が発見した。
TestMinimalPublicCommandsへ両操作の実Run→Minimal callback検査を先に追加し、code 2 / calls 0の有効REDを観測。
一覧へshow/checkを戻してGREEN。`go test -v -count=1 ./src/internal/cli -run '^(TestMinimal|TestFlow|TestParse)'`
はTestFlowGrammar、TestMinimalPublicCommands、TestMinimalRejectsInvalidとその子testを実行しexit 0。
このprefix正規表現に末尾の`$`を追加すると名前と一致しないため、0件の結果は証拠に含めない。
`go test -count=1 ./src/cmd/aidlc -run '^TestMainRootCommandsKeepSIGPIPE$'`もexit 0。
gofmtとgit diff --check成功。先の修正証拠はこの追加差分を含む親boundary/review/finalで更新する。
