# 初回取得の独立review修復01

Issue #202、work_unit_id `consolidated-release-bootstrap-repair-01`、loop、開始HEAD
`22bc42e69968f5353662b1c1af8f91ec2cc2e154`。親は編集停止、単独writerがR1→R2の順で修復する。
元の直接承認と計画の範囲内のbug/test修復で、公開権限や仕様を拡張しない。

R1: 旧HEADのinstall.ps1は無条件exit。公開READMEのScriptBlock呼出しではホストへexit例外が伝わるため、
後続sentinelに到達しない。旧版を動的実行したという証拠ではない。
PowerShell公式v7.5.0 ScriptCommandProcessor.csのconstructorとexit処理、Microsoftの自動変数説明を確認した。
https://github.com/PowerShell/PowerShell/blob/v7.5.0/src/System.Management.Automation/engine/ScriptCommandProcessor.cs#L543
https://learn.microsoft.com/en-us/powershell/module/microsoft.powershell.core/about/about_automatic_variables?view=powershell-5.1
ファイル呼出しはMyInvocation.MyCommandのExternalScriptInfo、公開CreateしたScriptBlockはScriptInfoとなる。
ファイルではexitを維持し、公開経路では呼出元scopeのLASTEXITCODEへ結果を置いてreturnする。
ExecutionPolicyやhost終了による回避は追加しない。Windows 5.1/7で両経路の0/17/取得失敗と後続到達を検査する。

R2: 旧unsafe testは壊れたmanifestでも失敗し、展開拒否が壊れたことを検出できなかった。
正常archiveが展開できる対照とunpackBundle直接検査へ変更し、manifest検査に依存させない。
productionの誤受理を示すfindingではないため、正しいtestの初回成功をALREADY_GREENと記録する。


## 実施と検証
R1は先に5.1/7のfile/scriptblock × valid/exit17/取得失敗等を追加した。
公開経路ではsentinelが生成され、その内容がLASTEXITCODEの期待値になることを必須にした。
旧HEAD正本の末尾は`exit $exitCode`であり、公式v7.5.0の例外伝播によりsentinelへ到達しない。
ローカルPowerShellがないので動的REDは観測していない。
分岐とSet-Variable Scope 1を実装し、same-candidateも公開呼出し後の到達・0を検査する。
Scope 1が親scopeを指す根拠:
https://raw.githubusercontent.com/MicrosoftDocs/PowerShell-Docs/main/reference/5.1/Microsoft.PowerShell.Utility/Set-Variable.md

R2はtar/zip双方の正常対照、重複、symlink、親子衝突の両順序、mode、親参照、絶対path、
backslash、特殊fileをunpackBundleで直接検査する。tar hardlinkも検査し、ZIPにhardlink種別は作らない。
全体hashは完全bundleの対照からdigestだけを変えるtestへ移した。
初回targetedはexit 0、ALREADY_GREEN。production変更も人工REDもない。

末尾検証command:
- `go test -count=1 ./src/bootstrap -run '^TestBootstrap'`
- `go test -count=1 ./src/internal/release -run '^TestBundleArchive'`
- `go test -count=1 ./src/bootstrap ./src/internal/release`
- `go test -tags=integration -list '^TestBootstrapCandidateNative$' ./src/bootstrap`

PowerShell 5.1/7はmacOSでskip、Windows CIの両経路・各終了値・後続到達の実行が必須の未完gate。
integrationはcompile/listだけで、実候補起動は親finalへ残す。全package/race/vet/crossbuild/E2Eは未実行。

末尾の上記4commandは全exit 0。PowerShell subtestのskipは動的成功に含めない。
変更4 Go fileへgofmtを適用し、git diff --checkはexit 0。
