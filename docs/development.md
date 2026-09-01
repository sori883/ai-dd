# 開発手順

## 前提

- Go 1.26以上
- Git

このmoduleはGo標準ライブラリだけを使用します。外部Go moduleや追加の開発toolを導入する場合は、先に必要性と設計理由を提示し、明示的な承認を得てください。

## ローカル検証

repository rootで次を実行します。

```sh
gofmt -w src
go test -count=1 ./...
go test -race -shuffle=on ./...
go vet ./...
go mod tidy -diff
git diff --check
```

coverageを確認する場合は、repository外の一時fileへ出力します。

```sh
coverage_file="$(mktemp)"
go test -shuffle=on -coverprofile="$coverage_file" ./...
go tool cover -func="$coverage_file"
```

### Workspaceのspace・intent readerと接続

内部readerの単体テストと、実filesystemでの統合テストを分けて実行します。

```sh
go test -count=1 ./src/internal/workspace
go test -tags=integration -count=1 ./src/internal/workspace
go test -count=1 -shuffle=on ./...
go test -race -shuffle=on ./...
go test -tags=integration -race -shuffle=on ./...
go vet -tags=integration ./...
```

単体テストは`fstest.MapFS`と最小の失敗注入stubを使い、cursor、明示override、
部分データ付きエラー、途中Stat失敗、JavaScriptのtrim・UTF-16 sort互換を固定します。
cwdや環境変数は変更しません。

intentではさらに、非空explicitの未検証pass-throughとFS未アクセス、cursor優先、
候補0・1・複数のfallback、無効cursorを正規化・Statする前の拒否、nested pathと`.`、
registry/state本文の未読を確認します。途中Stat失敗で後続も調べる点は、spaceと異なります。
`ActiveIntent`のboolは選択の有無であり、未検証explicitを後続path操作へそのまま渡してよい
という意味ではありません。

spaceの統合テストは`os.DirFS(t.TempDir())`に対して、未配置・空・通常file・directory、
directory/fileへのsymlink、broken link、root外symlinkを検証します。
intentの統合テストは、`t.TempDir()`を`os.OpenRoot`で開いた`Root.FS()`を使用します。
cursorとmarkerの双方で、内側の相対linkの受入れ、外側・絶対・壊れたlinkによるfallbackや
候補除外を検証します。reader単体テストでは`Root`のopen/closeをfixtureが管理し、
接続APIの`ReadSpaces`、`ReadIntents`、`ReadSelection`では製品内部が管理します。
`fs.FS`自体、`os.DirFS`、`fs.Sub`にはsymlink封じ込め保証がない点に注意してください。

readerと接続APIの統合テストで、読取前後のpath、内容、mode、更新時刻、symlink先を比較し、
製品処理による作成・変更がないことを確認します。fixtureの作成は製品への作成機能追加とは別です。
Windowsでsymlink作成権限が不足する場合は、そのケースだけ理由付きでskipします。

`ReadSelection`だけを確認する場合は次を実行します。

```sh
go test -count=1 -run '^(TestReadSelection|TestLocalizeSpace)' ./src/internal/workspace
go test -tags=integration -count=1 -run '^(TestReadSelection|TestLocalizeSpace)' ./src/internal/workspace
```

registryとrecord directoryの相関、および公開一覧接続だけを確認する場合は次を実行します。

```sh
go test -count=1 -run '^(TestListIntents|TestReadIntents)' ./src/internal/workspace
go test -tags=integration -count=1 -run '^(TestListIntents|TestReadIntents)' ./src/internal/workspace
```

接続の単体テストではroot候補の優先順位・絶対path、spaceの正規化前検証とLocalizeのOS差を
確認します。open・child-open・closeのエラーは非公開helperの関数引数から注入し、権限不足を
chmodや実行ユーザーの権限だけに依存させません。実Rootが必要な経路はintegration側でcaptureし、
正常・不在・無効space・接続失敗・Close失敗でも取得済みRootが逆順に閉じられること、
複数の原因を`errors.Is`で識別できること、error時にmetadataがzero valueになることを確認します。

実FSではdefault/custom/未知space、未作成の各階層、候補0・1・複数、cursorの優先を確認します。
projectの初回symlinkは追従しますが、childはproject内の相対linkだけを許可し、外向き・絶対linkを
接続error、壊れた相対linkを不在として扱います。`active-space`の拒否されたリンクはdefaultへ
fallbackします。さらに、同じproject内でもintents外のcursor/markerはfallback・除外となることを
固定します。接続テストも外側fixtureを含む読取前後snapshotを比較し、書込みや自動作成が
ないことを検証します。cursor・ReadDir・Statのエラー吸収は既存readerのままです。

本家ローカル`2.6.123`との差分は[接続境界と互換性への影響](architecture.md#本家との差分と影響)、
合意は[Accepted計画](ram/decisions/2026-08-31-workspace-reading-composition-plan.md)を参照してください。
将来のconsumerは`Selection`のboolを安全性・state内容・存在継続の保証とせず、名前を再利用する
path操作に独自の検証とFS境界を用意する必要があります。

既存CIのquality jobでも統合テストを実行します。ただしCIの実行OSはUbuntuであり、
6 targetのcross buildを各OSの実行証拠とは扱いません。space一覧は`ReadSpaces`、intent一覧は
`ReadIntents`、intent切替は`SwitchIntent`経由で公開CLIに接続していますが、
`ReadSelection`、Intent作成の内部core、sessionは公開CLIへ未接続です。help/version smokeを一覧や切替の
検証とせず、space/intentの機能E2Eも完全な
workspace lifecycleの検証とは区別します。

### Space一覧CLI

既存projectのspaceをread-onlyで表示します。bare `space`は`space list`のaliasです。

```sh
go run ./src/cmd/aidlc space list --project-dir /path/to/existing-project
go run ./src/cmd/aidlc space --json --project-dir=/path/to/existing-project
```

rootは明示`--project-dir`、`AIDLC_PROJECT_DIR`、`CLAUDE_PROJECT_DIR`、cwdの順に選びます。
相対pathはcwd基準です。選んだprojectが開けなくても低優先rootへ切り替えません。
project自体は既存である必要がありますが、`aidlc/spaces`やintent・stateの準備は不要です。
合成`default`の表示は初期化を意味せず、一覧はdirectory作成・修復・切替を行いません。
現在spaceはshared cursorだけから決め、session bindingやspace overrideはまだ受け取りません。

`--json`は値なしで1回だけ指定できます。`--json space list`、`space --json list`、
`space list --json`は同じ操作です。`--project-dir path`と`--project-dir=path`も
commandの前・途中・後に配置できます。重複・欠落・空値、未知flag、list後の余剰位置引数は
callback前に拒否し、cwd・環境変数・FSを読みません。`--json=true`や`--json=false`も無効です。
分離形のproject-dirは次tokenが`-`始まりなら値欠落とします。
`--project-dir --force`をpathとして扱わず、実pathが`-dir`なら
`--project-dir=-dir`または`--project-dir ./-dir`を使います。

bare aliasはflagを除いた位置引数が`space`だけの場合です。未知subcommandは従来の診断を保ちます。

| 入力（`aidlc`省略） | 結果 |
| --- | --- |
| `space --json false` | `space false`が未知subcommandとなり、従来形式・exit 2 |
| `space list --json false` | 認識済みlistの余剰位置引数としてJSON error・exit 1 |
| `space --json=false` | bare一覧の未知flagとしてJSON error・exit 1 |

成功時はstdoutのhuman一覧またはJSON 1行、stderrは空、exit 0です。全行inactiveならJSONの
top-level `active`だけが`default`となり、行のfalseは保持されます。認識済み一覧の構文・接続・
Close・出力失敗はexit 1で、stderrが書ける場合にJSON errorを1行出力します。
stdout失敗では部分出力が残り得ます。stderrも書けない場合はexit 1だけを保証します。
reader内部の欠損・読取エラーは既存fallbackを保つため、すべてがCLI errorになるわけではありません。

```sh
go test -count=1 -run '^TestReadSpaces' ./src/internal/workspace
go test -tags=integration -count=1 -run '^TestReadSpaces' ./src/internal/workspace
go test -count=1 ./src/internal/cli ./src/cmd/aidlc
```

単体テストはroot優先順位・open失敗の原因保持、flag配置・厳格検証・callback未実行、human/JSONの
行・escape・short write、lazy main入力を確認します。実RootのClose失敗は関数注入し、
Close済みであることもintegrationで確認します。実FSでは未配置・fallback、初回projectリンクと
内部相対・外向き・絶対・brokenリンクを確認し、外側fixtureを含む前後snapshotを比較します。
Unixの通常テストでは実mainのhelper subprocessに閉じたstdout/stderr pipeを接続し、list/bareの
human/JSONがSIGPIPE終了でなくexit 1となること、既存help/version/未知commandには影響しないことを
確認します。非Unixでは実pipe回帰は対象外です。全体・race・coverage・vetは上記の手順も実行します。

詳細と本家ローカル`2.6.123`との差分は[一覧CLIの契約](architecture.md#space一覧cliとread-only接続)、
[承認済み差分表](architecture.md#space一覧の意図的な差分)を参照してください。

### Intent一覧CLI

現在spaceのintent registryとrecord directoryをread-onlyで相関して表示します。bare `intent`は
`intent list`のaliasです。一覧操作自体はintentの作成・切替やstate本文の読取りを行いません。

```sh
go run ./src/cmd/aidlc intent list --project-dir /path/to/existing-project
go run ./src/cmd/aidlc intent --json --project-dir=/path/to/existing-project
```

rootは明示`--project-dir`、`AIDLC_PROJECT_DIR`、`CLAUDE_PROJECT_DIR`、cwdの順です。
既存projectを必須とし、低優先rootへfallbackしません。現在spaceはshared `active-space`から決め、
space名を1 componentとして検証してから`os.Root`でその`intents/`を開きます。intents rootが
未作成ならspace名を保持した空一覧です。registryの欠損、不正JSON、非配列、読取errorは
disk上のrecord directoryだけを表示しますが、有効な配列内に構造不正rowがあれば一覧全体を
errorにします。重複・registry-only・orphanを保持し、registry順の後にorphanをUTF-16順で並べます。

human形式はactiveを`*`、inactiveを空白で示し、`dirName`がなければslugを表示します。
空一覧は開始方法を案内し、非空でactiveがなければ切替方法を案内します。JSONは1行と末尾LFで、
top-levelに`active`・`space`・`intents`、各行に`uuid`・`slug`・`status`・`repos`・`dirName`・
`active`をこの順で出力します。active/dirName不在はnull、reposは常に配列で、内部scopeは出力しません。

`--json`は値なしで1回だけ指定できます。`--json intent list`、`intent --json list`、
`intent list --json`は同じ操作です。`--project-dir path`と`--project-dir=path`もcommandの
前・途中・後に置けます。重複・欠落・空値、未知flag、list後の余剰位置引数はcallback前に拒否し、
cwd・環境変数・FSを読みません。分離形の次tokenが`-`始まりなら値欠落です。`-dir`という実pathは
`--project-dir=-dir`または`--project-dir ./-dir`で指定します。専用subcommand helpはありません。

bare aliasはflagを除いた位置引数が`intent`だけの場合です。

| 入力（`aidlc`省略） | 結果 |
| --- | --- |
| `intent --json false` | bare切替target `false`への`--json`指定としてJSON error・exit 1 |
| `intent list --json false` | 認識済みlistの余剰位置引数としてJSON error・exit 1 |
| `intent --json=false` | bare一覧の未知flagとしてJSON error・exit 1 |
| `intent switch` | 認識済みswitchのtarget欠落としてJSON error・exit 1 |
| `intent create` / `intent help` | 予約または専用verbの従来形式・exit 2 |

成功時はstdoutだけへ一覧を出してexit 0です。認識済み一覧の構文・接続・query・Close・出力失敗は
exit 1で、stderrへ書ける場合はJSON errorを1行出力します。stdout失敗では部分出力が残り得ます。
stderrも書けない場合はexit 1だけを保証します。Unixの閉pipeでも同じで、help/version/未知commandの
SIGPIPE挙動は変更しません。

```sh
go test -count=1 -run '^(TestListIntents|TestReadIntents)' ./src/internal/workspace
go test -tags=integration -count=1 -run '^TestReadIntents' ./src/internal/workspace
go test -count=1 -run '^(TestRunIntentList|TestRunHelpIncludesIntentList)' ./src/internal/cli
go test -count=1 -run '^(TestIntentLister|TestMainIntentList|TestMainRootCommandsKeepSIGPIPE)' ./src/cmd/aidlc
```

integrationでは初回project link、内向き相対child link、外向き・絶対・broken child linkと
外側fixtureを含む前後snapshotを確認します。Unixの通常testでは実mainへ閉じたstdout/stderr pipeを
接続します。非Unixでは実pipe回帰は対象外で、Windowsのsymlink権限不足では該当caseだけ理由付きskipです。
本家ローカル`2.6.123`との比較範囲と承認済み変更は
[Intent一覧の契約](architecture.md#intent一覧cliとread-only接続)・
[差分表](architecture.md#intent一覧の意図的な差分)を参照してください。

### Intent切替CLI

shared `active-space`が指すspaceの既存Intentを選び、通常はそのspaceの
`intents/active-intent`を更新します。shared `active-space`が不在のときだけ
`aidlc/active-space`も補完します。明示形とbare形は同じ操作です。

```sh
go run ./src/cmd/aidlc intent switch build-auth --project-dir /path/to/existing-project
go run ./src/cmd/aidlc intent build-auth --project-dir=/path/to/existing-project
```

rootは明示flag、`AIDLC_PROJECT_DIR`、`CLAUDE_PROJECT_DIR`、cwdの順です。既存projectと
対象spaceの`intents/`が必要で、open失敗で低優先rootへfallbackしません。targetは
trim・slugify・case変換せず、まず`dirName`のcase-sensitive完全一致、次に一意な
slug完全一致で解決します。slugが複数directoryに一致すればAmbiguous、0件なら
Unknownとして保存前に拒否します。registry-only行は選択できませんが、markerを
持つorphanは完全directory名か一意な派生slugで選べます。

bare `intent`だけと`intent list`は一覧です。bareの`list`・`switch`・`create`・`archive`・
`rename`・`show`・`birth`・`help`はtargetにしません。`list`等の名前を持つrecordは
`intent switch list`のように明示形で指定できます。rawの空文字・`help`・`-h`は
明示形でも拒否します。caseの異なる`Help`等は通常targetです。

`--project-dir path`と`--project-dir=path`はcommandの前・途中・target後に置けます。
重複・欠落・空値、未知flag、余剰target、`--json`はcallback前に拒否し、cwd・環境変数・
FSを読みません。分離形の次tokenが`-`始まりなら値欠落です。`-dir`という実pathは
`--project-dir=-dir`または`--project-dir ./-dir`で指定します。`--json`は一覧専用です。

成功はstdoutに`Active intent → <dirName> (space: <space>)\n`だけを出し、stderr空、
exit 0です。認識済みswitchの構文・query・保存・Close・出力失敗はexit 1で、
stderrに書ければJSON errorを1行出します。stdout失敗では部分出力と保存済み
cursorが残り得ます。stderrも書けなければexit 1だけを保証します。

shared `active-space`が不在なら、完成済みstaging fileとhard linkで`<space>\n`を
no-replace補完します。競合で先に作られた値は上書きせず、補完失敗はbest-effortです。
`active-intent`は一時fileから置換し、既存symlink・非regularを拒否します。既存permissionの
9bitだけを復元し、owner・ACL・特殊mode・hardlink identityは保持保証しません。
Rename前の失敗でも補完済みactive-spaceやcleanup失敗のtempは残り得ます。Rename後、
Root Close、出力失敗ではactive-intentも保存済みの場合があり、自動rollbackしません。

```sh
go test -count=1 -run '^(TestSwitchIntent|TestResolveIntentTarget|TestCompleteCursorNoReplace|TestReplaceCursor|TestReplaceSpaceCursor|TestSaveCursorInRoot)' ./src/internal/workspace
go test -tags=integration -count=1 -run '^TestSwitchIntent' ./src/internal/workspace
go test -count=1 -run '^(TestRunIntentSwitch|TestRunHelpIncludesIntentSwitch)' ./src/internal/cli
go test -count=1 -run '^(TestIntentSwitcher|TestMainIntentSwitch)' ./src/cmd/aidlc
```

unit testはexact・unique slug・Ambiguous・Unknown、非公開seamでのopen・write・short write・
Chmod・Close・Rename・cleanup失敗、no-replaceの順序と競合を固定します。integrationは
初回project link、内向き相対child link、外向き・絶対・broken link、cursor型とmode、
session・registry・state・audit・configの無変更を確認します。Unixの通常testは実mainに
閉じたstdout/stderr pipeを接続し、SIGPIPE終了でなくexit 1に到達することと、
出力errorで保存済みcursorを取り消さないことを確認します。

制約とローカルAI-DLC `2.6.123`からの承認済み変更は
[Intent切替](architecture.md#intent切替)・[差分表](architecture.md#intent切替の意図的な差分)を参照してください。
session binding、rebind offer、UUID stamp、audit、Intent作成の内部coreはまだ公開CLIとして一連で動きません。

### Intent作成core（内部API）

`workspace.CreateIntent`は、callerが決定した既存Space、label、任意scope・reposを受け取り、
workspace lock下でrecord、registry、共有cursorを作成します。公開CLIはまだ接続していないため、
`go run ./src/cmd/aidlc intent create ...`は利用手順ではありません。full state、session binding、audit、
workspace scanもこのcoreの対象外です。

通常の検証は次のコマンドで行います。

```sh
go test -count=1 -run '^(TestUUIDV7|TestIntentSlug|TestNormalizeIntentLabel|TestIntentDirBase|TestResolveIntentDirName|TestCreateIntent|TestDecodeIntentRegistryForMutation|TestReadIntentRegistryForMutation|TestWriteIntentRegistry|TestWorkspaceLock|TestAcquireWorkspaceLock|TestInitializeWorkspaceLock|TestReleaseWorkspaceLock|TestWaitForWorkspaceLock)' ./src/internal/workspace
go test -tags=integration -count=1 -run '^(TestCreateIntentIntegration|TestCreateIntentHelperProcess)' ./src/internal/workspace
go test -tags=integration -race -shuffle=on ./...
go vet -tags=integration ./...
```

unit testはUUIDv7、slug・予約語・UTC日付・suffix、正確なstub、strict registry decode、未知field保持、
atomic writer、write・short write・Close・Rename・cleanupの原因保持を確認します。lock testは本家互換identityと
owner stamp、600回上限、context優先、自分のgenerationだけのrelease、stale・malformed lockの
fail-closedを固定します。上の正規表現を変更する場合は`go test -list`で対象testを確認してください。

integration testは実`os.Root`で既存Space、invalid registryの前後snapshot、registry/`active-intent`の
symlink・特殊file、project/Space link境界を確認します。helper subprocessは同じprojectへ同時作成する
2 processがlockで直列化され、UUIDとdirectoryが重複せず、registryに2 row残ることを実証します。
Windowsでsymlink作成権限が不足する場合は、そのcaseだけ理由付きでskipします。

registry tempのRename成功がcommit境界です。そこより前のerrorはzero result、以後のcursor・Root Close・
lock release errorは作成済み結果とerrorの両方を返します。自動retryやrollbackはせず、途中directory・
stub・tempが残る場合があります。fsync、power-loss耐久、複数fileのatomic性は保証しません。
約60秒のlock待機より短いdeadlineが必要なら、callerが`context.Context`へ設定します。stale lockは
自動回収しないため、lock pathを含むerrorを確認してから手動で診断・復旧してください。

本家ローカル`2.6.123`からの承認済み変更と安全境界は
[Intent作成の内部core](architecture.md#intent作成の内部coreとworkspace-lock)・
[差分表](architecture.md#intent作成coreの意図的な差分)を参照してください。公開CLIを変更しないため、
CLI buildや配布E2Eをこの内部APIのnative実行証拠として扱いません。

### Space作成CLI

既存projectへ新しいspaceを作ります。複数単語の名前は引用してください。

```sh
go run ./src/cmd/aidlc space create "Team Alpha" --project-dir /path/to/existing-project
```

`--project-dir path`と`--project-dir=path`はcommandの前・途中・名前の後に配置できます。
省略時のroot優先順位は`AIDLC_PROJECT_DIR`、`CLAUDE_PROJECT_DIR`、cwdです。
明示flagが最優先で、相対pathはcwd基準です。project自体は新設しません。
余剰位置引数、未知flag、重複flag、欠落・空のpath値、空の名前、作成位置の`help`・`-h`は
callback前に拒否します。分離形の次tokenが`-`始まりなら値欠落とし、例えば
`--project-dir --force`をpathとして飲み込みません。`-`始まりの実pathは
`--project-dir=-dir`または`--project-dir ./-dir`で渡せます。

成功時はstdoutに`Space created: team-alpha\n`、stderrは空、exit 0です。
認識済み`space create`の失敗はstderrのJSON 1行とexit 1、rootの未知commandは従来どおりexit 2です。
認識済み作成はUnixの閉じたstdout/stderr pipeでもexit 1を返します。stderrが書ける場合だけ
JSONを出力し、stdoutの出力失敗で作成済みspaceを取り消しません。
help/version/未知commandのSIGPIPE挙動は変更しません。
help/versionと構文エラーではcwd・環境変数・FSへアクセスしません。作成時も現在のspace/intentを
読まず、自動切替しません。本文・生成treeと境界は[Space作成](architecture.md#space作成)を参照してください。

```sh
go test -count=1 ./src/internal/cli ./src/cmd/aidlc ./src/internal/workspace
go test -tags=integration -count=1 -run '^TestCreateSpace' ./src/internal/workspace
go test -count=1 -race -shuffle=on ./...
go test -tags=integration -count=1 -race -shuffle=on ./...
go vet ./...
go vet -tags=integration ./...
```

単体テストは名前のUnicode小文字化・48文字切詰め、flag全位置と`=形式`、callback未実行、
stdout/stderr・終了コード、lazyなmainのroot入力を固定します。open/read/write/Closeの異常は
小さい関数seamから注入し、権限エラーをchmodだけに依存させません。
Unixでは通常テスト内でhelper subprocessから実際の`main`を呼び、読み口を閉じた実pipeを
stdout/stderrへ接続します。mock Writerだけでは検出できないSIGPIPE終了、生成物の保持、
再試行拒否、既存commandへの非影響を確認します。CIの全package通常テストでも実行され、
子processのcoverage出力は専用一時directoryへ隔離します。非Unixではこの回帰テストは対象外です。
integrationでは7 directory・6 fileと本文、default orgだけの継承、空org、不在fallback、
既存targetの全種別と同名競合、内部・外部・絶対・broken link、部分writeと複数Close失敗を確認します。
snapshotで既存default・他space・cursorの内容・mode・mtimeを比較し、対象追加に必要な親directoryの
mtime以外を変えないこと、境界外に書き込まないことを確認します。

errorが返っても生成物が残る場合があり、再試行は既存targetとして拒否されます。
自動cleanupや修復は行わず、利用者が内容を確認して次の対応を決めます。
本家との承認済みの6つの意図的な変更は[差分表](architecture.md#space作成の意図的な差分)を参照してください。
space作成の配布E2Eはread-only APIや完全なworkspace lifecycleの検証とは区別します。

### Space切替CLI

既存spaceを選び、共有`aidlc/active-space`だけを更新します。明示的な`switch`が必要で、
bare `space <name>`は受理しません。

```sh
go run ./src/cmd/aidlc space switch "Team Alpha" --project-dir /path/to/existing-project
go run ./src/cmd/aidlc space list --json --project-dir=/path/to/existing-project
```

rootは明示flag、`AIDLC_PROJECT_DIR`、`CLAUDE_PROJECT_DIR`、cwdの順で、相対pathはcwd基準です。
projectは既存である必要があり、open失敗で低優先rootへ切り替えません。名前は作成と同じslug化を
行いますが、作成専用の予約名拒否は使いません。raw空文字・`help`・`-h`は拒否し、`Help`からの
`help`や`list`等は一覧にあれば選べます。合成`default`は実directoryがなくても選べます。
一覧にない名前では書き込まず、自動作成・修復しません。readerがerrorを吸収するため、
この拒否だけで対象の物理的な不在を断定できません。同じtargetでも`<slug>\n`へ再保存します。

`--project-dir path`と`--project-dir=path`はcommandの前・途中・名前の後に配置できます。
重複・欠落・空値、余剰位置引数、未知flag、`--json`をcallback前に拒否し、cwd・環境変数・FSを
読みません。分離形の次tokenが`-`始まりなら値欠落です。実pathが`-dir`なら
`--project-dir=-dir`または`--project-dir ./-dir`を使います。専用subcommand helpはありません。
既存の`space --json false`は引き続き未知subcommandの従来形式・exit 2です。

成功はstdoutに`Active space → team-alpha\n`、stderr空、exit 0です。認識済みswitchの構文・保存・
Close・出力失敗はexit 1で、stderrが書ける場合はJSON errorを1行出力します。stdout失敗では
部分出力が残り得て、stderrも書けない場合はexit 1だけを保証します。Unixの閉pipeも同様です。
help/version/未知commandのsignal挙動は変更しません。

既存cursorがsymlink（danglingを含む）や非regularなら拒否します。初回project linkと境界内の
相対linkは許可しますが、外向き・絶対linkの祖先を経由した保存は拒否します。新規directoryは0777、
新規cursorは0666をumask付きで作ります。既存cursor更新はtemp0600からpermissionの9bitだけを
継承する置換であり、directory書込権限が必要です。owner/ACL等の保持は保証しません。
Rename前の失敗では、並行writerがいなければ旧cursorを保持しますが、親directoryやtempが
残り得ます。Rename以降・Close・出力失敗では切替済みの場合があり、自動rollbackしません。
errorだけで未変更と判断せず、一覧やcursorを確認してから次の対応を決めてください。

```sh
go test -count=1 -run '^(TestSwitchSpace|TestReplaceSpaceCursor|TestSaveCursorInRoot)' ./src/internal/workspace
go test -tags=integration -count=1 -run '^TestSwitchSpace' ./src/internal/workspace
go test -count=1 -run '^(TestRunSpaceSwitch|TestRunHelpIncludesSpaceSwitch)' ./src/internal/cli
go test -count=1 -run '^(TestSpaceSwitcher|TestMainSpaceSwitchClosedPipes)' ./src/cmd/aidlc
```

異常系は小さい関数seamへopen/write/short write/Chmod/Close/Rename/cleanup失敗を注入し、
原因のjoinと順序、自分のtempだけのcleanupを確認します。権限エラーをchmodだけに依存させません。
integrationでは実RootのClose、保存前後の内容・mode、内側・外側・絶対・broken linkを確認します。
cursorと保存に必要な親directory以外のsession・intent・registry・harness等はsnapshotで無変更を確認します。
Unixの通常テストは実mainのhelper subprocessへ閉pipeを接続し、exit 1と保存済みcursorの保持を
検証します。子のcoverage出力は専用一時directoryへ隔離します。非Unixでは実pipeテストは対象外、
Windowsのsymlink権限不足では該当caseだけ理由付きskipです。全体・race・coverage・vetも上記手順で実行します。

保存の限界と本家ローカル`2.6.123`との承認済みの3つの意図的な変更は
[Space切替](architecture.md#space切替)・[差分表](architecture.md#space切替の意図的な差分)を参照してください。
spaceの作成→一覧→切替→一覧は確認できますが、intentやsessionを含む完全なworkspace lifecycleではありません。

## 実行fileの確認

```sh
build_dir="$(mktemp -d)"
go build -o "$build_dir/aidlc" ./src/cmd/aidlc
"$build_dir/aidlc" --help
"$build_dir/aidlc" --version
```

linkerからbuild情報を設定する例です。

```sh
go build \
  -ldflags "-X github.com/sori883/ai-dd/src/internal/buildinfo.Version=v0.1.0 -X github.com/sori883/ai-dd/src/internal/buildinfo.Commit=abcdef0" \
  -o "$build_dir/aidlc" \
  ./src/cmd/aidlc
"$build_dir/aidlc" version
```

## Cross build

CIは`CGO_ENABLED=0`で次の6ターゲットのCLIをbuildします。

- `darwin/amd64`
- `darwin/arm64`
- `linux/amd64`
- `linux/arm64`
- `windows/amd64`
- `windows/arm64`

ローカルでは次のloopで、CLIとintegrationタグ付きworkspaceテストバイナリを確認できます。
mainはspace作成・一覧・切替とintent一覧・切替のためworkspaceをimportしますが、CLI buildはworkspaceの`_test.go`や
integrationタグ付きテストをcompileしないため、テストバイナリも別に確認します。
いずれもコンパイルの確認であり、各OSでのテスト実行とは区別します。

```sh
build_dir="$(mktemp -d)"
for target in darwin/amd64 darwin/arm64 linux/amd64 linux/arm64 windows/amd64 windows/arm64; do
  goos="${target%/*}"
  goarch="${target#*/}"
  suffix=""
  if [ "$goos" = "windows" ]; then suffix=".exe"; fi
  CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" \
    go build -trimpath -o "$build_dir/aidlc-$goos-$goarch$suffix" ./src/cmd/aidlc
  CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" \
    go test -c -tags=integration -o "$build_dir/workspace-$goos-$goarch.test$suffix" ./src/internal/workspace
done
```

## 配布E2E

repository外の承認済みlocal sandboxへ実行物を配置して確認する手順と安全規則は、
[配布E2Eテスト](e2e-testing.md)を参照してください。実行ごとの証跡は
[`e2e-runs/`](e2e-runs/)へ記録します。

## 変更手順

1. GitHub Issueと承認済み計画でscopeを固定します。
2. observable behaviorを表す失敗testを追加してREDを確認します。
3. 最小実装でGREENにし、testがgreenの間だけrefactorします。
4. 上記のローカル検証を実行します。
5. Issueへ紐づくPRを作成します。自動mergeは行いません。
