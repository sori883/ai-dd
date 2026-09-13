# Go trimpathと配布候補検証の修復02

Issue #202、loop、work_unit_id `consolidated-release-bootstrap-repair-02`、開始HEAD
`d5c76cb04d6e5d5a39b6e46ddd260290cdbf4f68`。親停止中の単独writerで、承認済み検証側bugをV1→V2で修復する。
final-01の候補・ログは保全し、production、build flags、依存、公開物形式を変更しない。

Go 1.26.4の正本GOROOT/src/cmd/go/internal/load/pkg.go:2424-2435を確認した。
-trimpath時はlinker引数に含まれるpathの秘匿のため-ldflagsをBuildInfoへ保存しない。
公式根拠は https://go.dev/issue/52372 。final-01の実binaryに-ldflagsがないことは異常ではない。
全30構成では実在するPath・GoVersion・GOOS/GOARCH・CGO_ENABLED・trimpathを厳密照合する。
manifestの版/commit、外側checksum、memberhash、原稿・許諾正本、梱包入力対応の検査は維持する。
実行可能なnative5CLIは版表示を完全一致で確認する。4CLIはproduct/version/commitを確認する。
natural-japanese-goの既存表示はproduct/versionだけであり、commitを確認したとは主張しない。
3OSで同一候補を実行しても全6CPUの版表示を実行した証拠にはならない。


## RED/GREEN証拠と末尾
V1: -ldflagsを持たず実際のPath・CGO・trimpathを持つ正常fixtureで、
`valid trimpath binary without ldflags rejected binary identity differs`のrunnable RED exit 1を確認。
validatorを実在する項目へ変更して同じcommandがGREEN exit 0。
Path・Go版・OS・CPU・CGO・trimpathの違い/必要設定不足も拒否する。

V2: 完全一致helperのcompile-only scaffoldとtestを先に追加し、全5製品の誤product/version、
4製品の誤commit、前後の追加dataと改行欠落を受理するrunnable RED exit 1を確認。
厳密な文字列照合へ実装しGREEN exit 0。実候補Native呼出しもこのhelperへ接続した。

両項目と末尾のexact targeted command:
`go test -tags=integration -count=1 ./src/cmd/aidlc-dist -run '^TestReleaseCandidate(MetadataValidation|NativeSelection|ProjectDirectory)$'`
影響通常test: `go test -count=1 ./src/cmd/aidlc-dist`。
末尾の両command、変更Goのgofmt、git diff --checkを確認する。
実候補起動、crossbuild、全体test/race/vetはloopで未実行。旧final-01を再成功扱いせず、親が新finalで確認する。

末尾targetedと影響通常testは両方exit 0。変更Goへgofmt適用済み、git diff --checkはexit 0。
