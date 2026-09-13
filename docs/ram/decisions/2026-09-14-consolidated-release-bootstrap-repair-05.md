# Windows directory別表記のtest修復05

Issue #202 / PR #203、loop、work_unit consolidated-release-bootstrap-repair-05、開始HEAD
f869557eae37821dda19927e5ba4d9bc7760324c。親停止中の単独writer。testと計画/RAMだけを変更する。
Windows CI34772300383でnative成功、PowerShellの終了値0/17・公開caller復帰・不正系拒否は成功した。
正常系のargs比較がRUNNER~1とrunneradminの同じdirectoryの別表記を誤拒否した。
Get-Item.FullNameの正規化は契約どおりで、productionは修正しない。
実在directoryのos.Stat/IsDir/os.SameFileで同一性を比較し、引数の順・個数・版などは維持する。


test helperへ従来のEqualFold比較を移した上で、同一・別表記・別directory・不足pathを先に検査した。
`go test -count=1 ./src/bootstrap -run '^TestBootstrapProjectDirectory$'`で、
同一の別表記の誤拒否と、両方存在しない同じ文字列の誤受理のRED exit 1を確認。
os.Stat/IsDir/os.SameFileへ変更して同command GREEN exit 0。
PowerShell argsの実在directory同一性と絶対pathを検査し、他のassertionは維持した。

候補bootstrapを調べたところ、Goのfilepath.EvalSymlinksでrootを正規化した後、
同じGo installerのEvalSymlinksによる配置bytesと比較している。
Go1.26.4のpath/filepath/symlink_windows.go:108以降はtoNormとFindFirstFileに基づく名前へ揃える。
未正規化t.TempDirの直接比較ではなく、同じ誤仮定は見つからなかったため候補testを変更していない。
Unix既存testも期待値をEvalSymlinks済みで、今回のWindows短名の修復対象外。

Windowsの修正後動的GREENは未確認。新HEADのCIが必須であり、ローカル成功を代用しない。
末尾はbootstrap通常、coverage付き、候補integration-listとgofmt/diff-check。
全package/race/vet/候補E2Eはloopで実行しない。

末尾の実行は全exit 0:
- go test -count=1 ./src/bootstrap
- go test -count=1 -coverprofile=/tmp/ai-dd-repair05-cover.out ./src/bootstrap
- go test -tags=integration -list '^TestBootstrapCandidateNative$' ./src/bootstrap
変更3 Go test fileへgofmt適用済み、git diff --checkはexit 0。
