# Windows curl選択・起動後処理の修復03

Issue #202 / PR #203、loop、work_unit_id `consolidated-release-bootstrap-repair-03`、開始HEAD
`f41ff05c474d471fcffbc53c87949d339a32d9b3`。親は編集停止、単独writerが承認済みbug/test修復を行う。
Go production、buildflags、依存、公開形式は変更しない。旧final-02の資材・ログは保持する。

Distribution run34771558215でpackage/Ubuntu/macOS成功、WindowsのGo native成功後、PowerShellの
両shell・両呼出しのvalid/exitがcode1で失敗した。診断はKillのNo process associatedであり、Start失敗後の
finallyが元errorを隠すことは確定した。元Start診断そのものは未観測。
PowerShell v7.5.0のGetCommandCommand.cs:963-970はCommandType明示時に列挙を継続する。
https://raw.githubusercontent.com/PowerShell/PowerShell/v7.5.0/src/System.Management.Automation/engine/GetCommandCommand.cs
複数curl.exeが返りSourceが単一pathにならなかったことは、公式分岐とCI環境からの推論であり、
新HEADのWindows CIで確認する。TotalCount 1でPATHの最初のApplicationInfoを選び、Pathを起動する。
Start成功をboolで追跡し、開始前のHasExited/Killを避ける。

候補testはEncodedCommandのstderr進捗CLIXMLをJSONへ混ぜないよう、stdoutとstderrを分離する。
非zero終了は引き続き失敗とし、stderrは失敗診断へ残す。製品のTLS/ExecutionPolicy等を緩めない。


## 実装と証拠
R3aでは先に正常helperの次へ起動不能なcurl.exeを置くfixtureと、先頭自体が不正exeのcaseを追加した。
既存の0/17/取得失敗・両呼出し・sentinelを維持し、Start失敗caseはcode1、installer未起動、
Cannot start curl.exeから始まる元診断とcleanup診断への非置換を要求する。
その後TotalCount 1、ApplicationInfo.Path、started boolの分岐を実装した。
CI run34771558215の失敗が観測RED。ローカルPowerShellはないため新caseの動的RED/GREENは未観測。

R3bでは候補の既存CombinedOutput経路をtest helperへ移す形でtestを先に追加し、
JSON stdoutとCLIXML風stderrが混ざるrunnable RED exit 1を、0/17両終了で確認した。
別々のbufferとcmd.Runへ修正して同じtestがGREEN exit 0。候補JSON解析はstdoutだけ、
終了失敗と解析失敗にはstderrも診断として含める。
追加targetedは`go test -count=1 ./src/bootstrap -run '^TestBootstrapOutputStreams$'`。

末尾command:
- `go test -count=1 ./src/bootstrap -run '^TestBootstrap'`
- `go test -count=1 ./src/bootstrap`
- `go test -tags=integration -list '^TestBootstrapCandidateNative$' ./src/bootstrap`

PowerShell 5.1/7の修正後GREENと実候補bootstrapは、新HEADのWindows CI/親finalで必須確認する。
ローカルUnix成功やcompile/listをWindows成功に読み替えない。全package/race/vet/crossbuild/候補E2Eは未実行。

末尾3commandは全exit 0。変更4 Go fileへgofmt適用済み、git diff --checkはexit 0。
PowerShell skipは動的成功の証拠に含めない。
