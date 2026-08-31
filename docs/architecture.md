# アーキテクチャ

## 方針

ai-ddは、単一のGo moduleと単一の実行ファイルを保ちながら、責務をpackage境界で分離するモジュラーモノリスです。現段階では抽象化を増やさず、Go標準ライブラリと手動dependency injectionだけを使用します。

```text
src/cmd/aidlc/main.go
  ├─ src/internal/buildinfo
  ├─ src/internal/cli (arguments, output, exit code)
  └─ src/internal/workspace (CreateSpace / ReadSpaces / SwitchSpace callbacks)

src/internal/workspace
  ├─ project root resolution
  ├─ active-space and space listing (read-only)
  ├─ active-intent and intent listing (read-only)
  ├─ ReadSpaces: project → shared-cursor space list (read-only CLI)
  ├─ ReadSelection: project → current space → intents (read-only, no CLI yet)
  ├─ CreateSpace: explicit creation within an existing project
  └─ SwitchSpace: explicit shared-cursor update within an existing project
```

## Package境界

- `src/cmd/aidlc`: composition rootです。process引数、stdout、stderr、build情報と作成・一覧・切替callbackを組み立て、`cli.Run`の戻り値を`os.Exit`へ渡します。callbackが呼ばれたときだけcwdと環境変数を読みます。ドメイン判断や出力整形は置きません。
- `src/internal/cli`: CLIの引数解釈、stdout/stderrの分離、終了コード、help/version、space create/list/switchとbare spaceの表示契約を所有します。`io.Writer`とcallbackを受け取るため、process全体を起動せずにテストできます。
- `src/internal/buildinfo`: linkerが差し替える`Version`と`Commit`、およびそのsnapshotを所有します。既定値は`dev`と`unknown`です。
- `src/internal/workspace`: project rootの選択・path正規化、space・intentの読み取りとその接続、spaceの新規作成・共有cursor切替を所有します。root解決は受け取った候補だけで決定し、環境変数や現在directoryを直接参照しません。space readerはread-onlyの`ReadSpaces`を通じてCLIへ接続し、intent readerと`ReadSelection`は未接続です。書込みは`CreateSpace`・`SwitchSpace`の明示呼出しだけで行い、接続APIのRoot生存期間も内部で管理します。

`internal`配下はmodule外からimportできません。今後の機能も、CLIから直接filesystemやnetworkへ到達させず、責務ごとのpackageをcomposition rootで接続します。

## 手動DI

`main`が`os.Args[1:]`、`os.Stdout`、`os.Stderr`、`buildinfo.Current()`と
作成・切替それぞれの`func(rawName, explicitDir string) (string, error)`、
`func(explicitDir string) ([]workspace.Space, error)`を`cli.Run`へ渡します。
CLIは構文検証後にだけcallbackを1度呼びます。callback内で`os.Getwd()`、
`AIDLC_PROJECT_DIR`、`CLAUDE_PROJECT_DIR`を読み、flag値と合わせた`RootInput`を
`workspace.CreateSpace`、`workspace.ReadSpaces`または`workspace.SwitchSpace`へ渡します。
help/versionと構文エラーではcwd・環境・FSへ到達しません。
CLI packageはglobalなprocess I/Oを参照せず、各実行の出力と終了コードを決定的に検証できます。

`main`はnil可の`prepareSpaceOutput func()`も渡します。CLIは認識済み`space create`、
`space list`、`space switch`、bare `space`で、構文エラーを含む最初の出力・callbackより前に1度だけ呼びます。`main`側のhookで
`SIGPIPE`を無視し、Unixの閉じたstdout/stderr pipeへのwriteもerrorとして扱い、exit 1へ到達させます。
signalのprocess-global設定は`main`だけが所有し、CLI package自身には置きません。
help/version/未知commandではhookを呼ばず、従来のsignal・出力挙動を維持します。

外部DI containerは使用しません。依存が増えた場合も、まずconstructorまたは関数引数による注入を維持し、変更理由が明確になった時点で再評価します。

## Space読み取りの互換契約

`workspace.ActiveSpace(projectFS)`と`workspace.ListSpaces(projectFS, activeOverride)`は、
AI-DLC `2.6.123`の共通readerに合わせた内部APIです。`projectFS`には、呼出側が用意した
project root基準の`fs.FS`を渡します。独自FS interfaceやstore層は追加していません。

- `ActiveSpace`は`aidlc/active-space`を読み、JavaScript相当のtrim後の名前を返します。
  U+FEFFは除去し、U+0085は保持します。空・欠損・任意の読取エラーでは`default`です。
  エラーと一緒に返された部分データは使用しません。
- `ListSpaces`は`aidlc/spaces`直下をStatし、directoryだけを列挙します。再帰しません。
  未作成でも`default`を必ず1件含め、重複を除き、JavaScriptのUTF-16コード単位順で
  整列します。`default`を先頭に固定するわけではなく、その存在は初期化済みを意味しません。
- overrideがnilの場合だけcursorを読みます。非nilの空文字も明示値とし、overrideをtrimせず、
  名前の完全一致で`Space.Active`を決めます。未知名では全件inactiveになり得ます。
- ReadDirのエラーでは部分entriesも捨てて`default`だけを返します。子のStat失敗では
  そこで列挙を打ち切り、既収集分と`default`を整列して返します。

このAPIは互換性のため読取エラーを吸収するので、fallbackだけでは未作成と権限不足等を
区別できません。cursorの形式や指すdirectoryの存在も検証せず、その名前を使った追加の
pathアクセスは行いません。spaceの作成・切替・修復は別の責務です。

symlinkは供給FSのStatに従って追従します。`os.DirFS`はsandboxではなく、root外への
symlinkも参照し得ます。下記の`ReadSelection`は、名前からpathへ到達する接続側で
検証とfilesystem境界を適用します。不正UTF-8のdecodeと、Stat失敗までのOS/runtime別列挙順による部分集合の
完全互換は保証しません。

承認と詳細は[共通space読み取りの初期契約](ram/decisions/2026-08-31-space-reading-contract.md)を参照してください。

## Space一覧CLIとread-only接続

`workspace.ReadSpaces(input RootInput) ([]Space, error)`は、既存`ResolveRoot`で選んだprojectを
絶対pathと確認して`os.OpenRoot`で1つだけ開き、`ListSpaces(root.FS(), nil)`を呼んでCloseします。
相対解決結果は`fs.ErrInvalid`、project open失敗は不在を含めerrorとし、低優先rootへfallbackしません。
RootのClose失敗も原因を保持したerrorとして返し、error時の一覧はnilです。
返値にRootやFSは含めず、呼出側のCloseは不要です。異常系は非公開helperのopen/close関数へ
注入し、mutable globalや独自FS/store interfaceは追加しません。

既存space readerのエラー吸収は維持します。未作成の`aidlc/spaces`は合成`default`を返しますが、
directoryを作成したという意味ではありません。cursor参照の境界拒否・読取失敗は`default`へfallbackし、
ReadDir失敗は`default`だけ、途中Stat失敗は既収集分と`default`を返します。
このため、成功した一覧だけでは欠損・権限不足等を区別できません。

選択はshared `aidlc/active-space`だけを参照します。cursor名を検証・pathとして再利用せず、
intent、state、registry、sessionの存在を要求しません。初回project pathのsymlinkは追従します。
project内の相対symlinkは許可し、外向き・絶対linkは`os.Root`で拒否されるため、読取箇所に応じて
既存readerのfallback・途中打切りになります。作成・切替・修復・書込みは行いません。

CLIは`space list`とbare `space`を受理し、`--json`がなければ`Spaces:\n`に続けて
active行を`* <name>\n`、その他を`  <name>\n`として出力します。
JSONは`active`と`spaces`（各行は`name`・`active`）だけの1行と末尾LFです。
top-level `active`は最初のactive行の名前、全行inactiveなら`default`です。その場合も
各行のfalseは書き換えません。readerの順序・名前を保持し、JSON escapeは標準encoderに任せます。

成功はexit 0、認識済み一覧の構文・root・Close・出力失敗はexit 1です。
stderrが書ける場合はJSON errorを1行出力します。stdout失敗時は途中までの出力が残り得て、
stderrも書けない場合はexit 1だけを保証します。既存の未知command/subcommandは従来形式・exit 2です。
引数の配置・厳格検証は[Space一覧CLIの利用手順](development.md#space一覧cli)を参照してください。

### Space一覧の意図的な差分

比較対象はローカルAI-DLC `2.6.123`の`printSpaceListing`、共通reader、public引数処理です。
[調査記録](ram/research/2026-08-31-space-list-contracts.md)のcore実装を静的に確認し、
該当する3ファイルと配置版のSHA-256一致を確認した範囲です。
正規配布を含む配布物全体のparity、最新upstreamや全workflowの一致は未確認です。

| 本家の挙動 | 採用した挙動 | 理由 | 利用者・互換性への影響 |
| --- | --- | --- | --- |
| project不在でもreaderがdefaultを返し得る | 既存project必須、project open失敗をerrorにする | 誤ったproject指定や接続失敗を隠さない | 未作成projectはJSON error・exit 1。存在するprojectの未配置spacesは従来のfallback |
| 通常のFS操作で任意symlinkを参照し得る | projectの`os.Root`境界を適用 | project外の参照を制限する | 外向き・絶対linkは読取箇所に応じfallback・途中打切り。初回project linkと内部相対linkは許可 |
| 余剰引数・flagの解釈が寛容で、public経路のproject-dir形式に差がある | 未知・重複・値付きJSON・余剰位置引数を拒否し、project-dirの分離形と等号形を受理 | typoをcallback前に検出し、既存createと揃える | 以前黙認された構文はerror。flagはcommand前・途中・後に置ける |

承認は[space一覧の実装計画](ram/decisions/2026-08-31-space-list-plan.md)を参照してください。
session binding、space override、intent/status表示は今回の未実装範囲であり、恒久的な差分ではありません。
`os.Root`もmountや特殊file/deviceを防ぐ完全sandboxではありません。不正UTF-8やOS別の名前表現、
列挙途中エラーの部分集合の完全互換、並行更新中の一貫したsnapshotは保証しません。

## Intent読み取りの契約と安全性の差分

`workspace.ListIntentDirs(intentsFS)`と`workspace.ActiveIntent(intentsFS, explicit)`は、
選択済み1spaceの`aidlc/spaces/<space>/intents/`をrootとする、non-nilの`fs.FS`を受け取ります。
project root基準のFSではありません。space選択、名前からのFS接続、`os.OpenRoot`と`Close`の
管理はreader自身ではなく呼出側の責務です。下記の`ReadSelection`ではこの責務を内部で
完結させます。readerを単独で利用する場合は、引き続き呼出側が管理します。

- `ListIntentDirs`は直下entryの`<entry>/aidlc-state.md`へのStatが成功すれば候補にします。
  entryのdirectory判定やmarkerの通常file判定は行いません。命名形式を制限せず、現行の
  `YYMMDD-label`、旧形式、その他の名前をUTF-16コード単位順で返します。再帰はしません。
- ReadDirのエラーでは部分entriesも捨て、non-nilの空sliceを返します。個別Statの失敗では
  その候補だけを除外して後続を調べます。space readerの途中打切りとは異なる契約です。
- `ActiveIntent`の非空explicitは、空白やunsafeなpathもtrim・検証せずそのまま`true`と
  返します。この分岐ではFSへ一切アクセスしません。空explicitはcursor・候補選択へ進みます。
- cursorは`active-intent`を読み、space readerと同じJavaScript相当のtrimを行います。
  読取エラーでは部分データも捨てます。trim後の値が`fs.ValidPath`を満たし、markerへの
  Statが成功すればその値を選択します。候補一覧への所属は必要ありません。
- cursorが利用できなければ、候補がちょうど1件の場合だけその名前と`true`を返します。
  0件・複数件なら`("", false)`です。いずれのreaderもregistryの`intents.json`やstate本文を
  読まず、作成・切替・修復・書き込みを行いません。

AI-DLC `2.6.123`との意図的な差分として、cursorのpathを正規化する前に`fs.ValidPath`で
検証します。`../other`、`a/../b`、絶対path、空component等はcursor先をStatせずfallbackへ
進みます。一方、`nested/name`と特殊値`.`は有効です。`.`はrootの`aidlc-state.md`を確認し、
返値も`.`のままです。backslashとcolonは`fs.FS`の名前文字として扱い、OSの区切りへ
独自変換しません。

実filesystemの供給方法には`os.Root.FS()`を採用し、統合テストでその境界を固定しています。
内側の相対symlinkには追従しますが、越境・絶対symlinkは拒否されるため、本家が参照できる
リンクでもfallbackまたは候補除外になり得ます。`fs.FS`という型自体にはこの封じ込め保証が
ありません。`os.DirFS`や`fs.Sub`をsandboxとせず、供給FSの選択は呼出側で行います。
`os.Root`もmount、特殊file、device fileまで遮断する完全なsandboxではありません。

返すboolは選択の有無だけを表し、名前の安全性や存在の保証ではありません。特にexplicitは
未検証であるため、後続consumerはその値をpathに使う前に検証し、安全なFS境界を用意する
必要があります。`ReadSelection`は未検証space名からintentsのFSへの接続を担当しますが、
intentの明示overrideは受け取りません。既存`ActiveIntent`のexplicit契約は変更しません。
エラー吸収により欠損と権限不足等は区別できません。不正UTF-8の完全互換、各OSの名前制約・
Node/BunのOS別path解釈、並行更新中の一貫したsnapshotは保証しません。

承認と詳細は[intent読み取りの実装計画](ram/decisions/2026-08-31-intent-reading-plan.md)を参照してください。

## Workspace読み取りの接続

`workspace.ReadSelection(input RootInput) (Selection, error)`は、現在選択された1 spaceの
metadataを返します。`Selection`は`ProjectRoot`、`SpaceName`、`IntentDirs []string`、
`ActiveIntent`、`HasActiveIntent`を持ちます。RootやFSを返さず、成功時の空一覧はnon-nil、
error時は全fieldがzero value（一覧はnil）です。`ProjectRoot`は`ResolveRoot`で解決したpathを
保持し、symlink展開後の実体pathには置き換えません。

既存`ResolveRoot`の優先順位（明示root、AIDLC、Claude、WorkingDir）と正規化はそのまま使い、
結果が絶対pathであることを確認して`os.OpenRoot`します。取得したprojectの`Root.FS()`で
`ActiveSpace`を一度だけ呼びます。返された名前を追加trim・Cleanの前に`fs.ValidPath`で
検証し、`.`とslashを含む値を拒否します。`filepath.Localize`でhost OSに表現できるpathへ
変換してから、project Rootの`OpenRoot`で相対的に`aidlc/spaces/<space>/intents`を開きます。
ASCII slugやspace一覧への所属は要求しません。

開いたintentsの`Root.FS()`を既存`ListIntentDirs`と`ActiveIntent(..., "")`へ渡します。
project内の相対symlinkは子を開く際に許可しますが、project外・絶対symlinkは拒否します。
intent読取りはさらにintents Root内へ限定するため、同じprojectの別directoryへの
cursor/markerリンクもfallback・候補除外になります。最初のproject path自体はsymlinkを
追従します。`os.DirFS`、`fs.Sub`、path文字列のprefix判定を封じ込めとしては使用しません。

| 接続時の状態 | 結果 |
| --- | --- |
| 解決結果が相対path、space名やLocalizeが無効 | `errors.Is(err, fs.ErrInvalid)`で識別できるerror。無効spaceではchildを開かない |
| project open失敗 | 不在を含めすべてerror。低優先順位のrootへ切り替えない |
| child openが`errors.Is(err, fs.ErrNotExist)` | rootとspace名を保持した正常な空一覧・現在intent未選択 |
| childのその他open失敗 | 操作・対象を添え、原因を`%w`で保持したerror |
| RootのClose失敗 | 先行errorや他のClose失敗と`errors.Join`で結合したerror |

子の不在には祖先directoryの未作成や壊れた相対リンクも含まれ、「未初期化」と診断した
わけではありません。自動作成・修復は行いません。取得済みRootはintents、projectの逆順で
必ずCloseし、空結果を返せる経路でもClose失敗を無視しません。異常系テストには非公開helperへ
open・child-open・closeの関数を注入し、mutable globalや独自FS/store interfaceを増やしません。
open後のcursor・ReadDir・Statエラーは既存readerが吸収するため、すべてのI/O障害を
通知するAPIではありません。

### 本家との差分と影響

比較対象はローカルAI-DLC `2.6.123`の[共通helper](実装_aidlc-workflows/core/tools/aidlc-lib.ts)
（`activeSpace`、`intentsDir`、`listIntentDirs`、`activeIntent`）です。実装・正規Codex配布・
配置版の`aidlc-lib.ts`が一致する範囲で確認しており、全fileや最新upstreamとの一致は未確認です。

| 本家の挙動 | 本実装の挙動 | 差分の理由 | 利用者・互換性への影響 |
| --- | --- | --- | --- |
| 共通helperは未検証space名をpathへ結合する | 接続前に1 componentとLocalizeを検証する | 名前を使うpath操作の境界を明確にする | traversal・nested名・`.`・OSで表現不能な名は接続error。大文字・Unicode等は一律に拒否しない |
| 通常のFS操作で任意のsymlinkを追従し得る | projectとintentsのRoot境界を順に適用する | project外や選択intents外の参照を防ぐ | 外向き・絶対childリンクはerror。cursor/markerの拒否は既存readerのfallback・除外。初回projectリンクと内向き相対リンクは許可 |
| 共通readerがcursor・列挙等のI/Oエラーを吸収する | 接続はproject失敗をerror、child不在だけを空結果にする | 接続不能と現在intent未選択を区別する | 従来の空結果等に代わり一部接続障害を呼出側が処理する。reader内部のエラー吸収は維持 |
| 比較対象helperはpathや値を返し、Rootを保持しない | metadataだけを返し、内部でRootを逆順Closeしてerrorを結合する | Goでのresource所有権を単一呼出し内に収める | 呼出側のClose不要。Close失敗時も部分metadataは返さない。既存Go reader APIは変更しない |

space/intent override、space一覧、公開CLI/statusへの接続、registry/session/state本文は
`ReadSelection`接続の対象外であり、既存機能を削除した非互換とは区別します。
`ReadSelection`は作成・切替・削除や保存形式の変更を行いません。将来consumerが返された名前を再びpathへ使う場合は、改めて検証と安全な
FS境界を用意してください。返されたboolは選択だけを表し、stateの妥当性や返却後の存在を
保証しません。

各readerは別々に読むため、並行更新中の一貫したsnapshotは保証しません。`os.Root`もmountや
特殊file/deviceを防ぐ完全なsandboxではありません。不正UTF-8、OS別の名前制約、Node/Bunの
path解釈を含めた完全互換は対象外です。

承認した差分・詳細は[workspace読み取り接続の実装計画](ram/decisions/2026-08-31-workspace-reading-composition-plan.md)を参照してください。

## Space作成

`workspace.CreateSpace(input RootInput, rawName string) (string, error)`は、既存project内へ
新しいspaceを作り、成功時に正規化名、error時に空文字を返します。既存`ResolveRoot`の
優先順位をそのまま使います。spaceやintentの選択は不要で、`ReadSelection`やcursor readerを
前処理として呼びません。作成後の自動切替やcursor・registry・state・auditの更新もありません。

名前は本家のslugify順序に合わせ、小文字化、ASCII英数字以外の連続をhyphen化、端のhyphen除去、
48文字切詰め、末尾hyphen除去、必要な`intent-`prefix付加を行います。prefixは切詰め後なので
最終名は48文字を超え得ます。Unicodeの`İB`は`i-b`、`K`は`k`です。非空の空白・記号・
非ASCII文字だけの名前は`intent`になります。空入力、rawの`help`・`-h`、正規化後の
`help/list/switch/create/archive/rename/show/birth`は拒否します。`default`は禁止名ではありません。
名前と`filepath.Localize`の検証はFSアクセス前に行い、Windowsのdevice名等は
`errors.Is(err, fs.ErrInvalid)`で識別できます。

生成先の`aidlc/spaces/<name>/`を含め、7 directory・6 fileを作ります。

```text
<name>/
  ├─ memory/
  │    ├─ org.md
  │    ├─ team.md
  │    ├─ project.md
  │    ├─ phases/
  │    └─ templates/.gitkeep
  ├─ intents/
  ├─ codekb/.gitkeep
  └─ knowledge/.gitkeep
```

orgだけを`default/memory/org.md`から継承し、確認できた不在時だけ
`# Organization defaults\n`にします。既存の空fileもそのまま継承し、read/Closeエラーを
fallbackで隠しません。teamは`# Team practices\n`、projectは`# Project overrides\n`、
3つの`.gitkeep`は空です。他のspaceやdefaultのteam/project/template/knowledge等はコピーしません。
新規directoryは`0777`、fileは`0666`を指定し、実permissionはprocessのumaskに従います。
既存defaultの内容・permission・mtimeは変更しません。

project自体は`os.OpenRoot`で開ける既存directoryが必要です。初回project pathのsymlinkは
追従しますが、以後のread/writeはそのRoot内へ限定します。内部の相対linkは許可し、
その先の不足する祖先directoryも作れます。外向き・絶対linkは拒否します。
targetを単独の`Mkdir`で確保するため、既存directory・file・symlink（danglingを含む）は
`fs.ErrExist`となり、mergeや修復はしません。fileも`O_CREATE|O_EXCL|O_WRONLY`で作ります。

取得したfileとRootはCloseし、先行エラーとCloseエラーを`errors.Join`で保持します。
途中write・Close・成功メッセージ出力の失敗では、部分生成物や完成済みspaceが残り得ます。
自動cleanup・rollback・上書き再開はせず、再実行も既存targetとして拒否します。同名の本CLI同士では
1件だけが作成権を得ますが、その処理の成功やcrash durabilityは保証しません。
敵対的directory差替え、mount、特殊file/deviceまで防ぐ完全sandboxでもありません。

### Space作成の意図的な差分

比較対象はローカルAI-DLC `2.6.123`の`handleSpaceCreate`、名前・引数処理です。
[調査記録](ram/research/2026-08-31-space-creation-contracts.md)に実装・正規Codex配布・配置版の
該当fileの一致と根拠を記録しています。最新upstreamや全workflowの一致は未確認です。

| 本家の挙動 | 採用した挙動 | 理由 | 利用者・互換性への影響 |
| --- | --- | --- | --- |
| 不足project自体も再帰mkdir | 既存project必須 | 指定誤りによるproject新設を防ぐ | 利用者が先にproject directoryを用意する |
| 通常のpath操作で任意linkを参照し得る | project Rootで境界を適用 | 外部への書込み・copyを防ぐ | 外向き・絶対linkはerror、内部相対linkは許可 |
| 事前存在確認後に再帰mkdir、非排他的write | targetの単独Mkdirとfileの排他作成 | 同名競合時の再利用・上書きを防ぐ | dangling targetも拒否し、同名作成の片方だけが進む |
| orgのexistsSyncがfalseならstub | 確認できた不在だけstub、他はerror | 継承失敗を隠さない | 読めるorgか不在が必要。部分生成物が残り得る |
| 余剰引数や未使用flagを厳格検証しない | 不明・余剰・重複・空flag値をcallback前に拒否 | typoによる作成を防ぐ | 以前黙認された構文はerrorになる |
| 成功3行と切替案内、名前欠落時usage | 成功1行、認識済み作成失敗はJSON・exit 1 | 機械判読可能にし、作成導入時点で未実装だった切替を案内しない | 成功出力と名前欠落時の形式が変わる |

承認とscopeは[space作成の実装計画](ram/decisions/2026-08-31-space-creation-plan.md)を参照してください。
session・audit・legacy alias等は今回の未実装範囲であり、恒久的な非互換としては扱いません。

## Space切替

`workspace.SwitchSpace(input RootInput, rawName string) (string, error)`は、既存spaceを選び、
共有`aidlc/active-space`へUTF-8の`<正規化名>\n`を保存します。成功時は正規化名、error時は
空文字を返します。ただしerrorでもcursorが保存済みの場合があります。作成の成功出力・自動切替しない
契約は変更しません。intent cursor、session binding、harness include、audit、registry、state、
spaceの内容は更新しません。

rawの空文字・`help`・`-h`を拒否し、作成と同じprivate slug helperで正規化します。
Unicode小文字化、ASCII化、48文字切詰め、数字開始のprefix、空白・記号だけなら`intent`という
順序は共通ですが、作成専用の予約名拒否は流用しません。`Help`からの`help`や`list`、`create`等も
一覧にあれば選べます。同じtargetでも末尾LF付きで再保存します。

既存`ResolveRoot`の優先順位で選んだ絶対pathを`os.OpenRoot`で開きます。既存projectが必要で、
不在・open失敗を低優先rootへfallbackしません。同じRootのFSで`ListSpaces`を呼び、非nilの
overrideで現在cursorの不要な読取りを避けて所属を確認します。targetだけのStatには置き換えません。
合成`default`は実directoryなしで選べます。未知名は書込み前に拒否しますが、readerの途中Stat失敗等で
一覧から漏れた場合も含むため、「物理的に存在しない」という診断ではありません。

project内に必要な`aidlc/`親だけを作り、別のRootとして開きます。初回project pathのsymlinkと
境界内の相対linkは許可しますが、外向き・絶対linkの祖先を経由した保存は拒否します。
既存cursorはLstatで検査し、symlink（danglingを含む）や非regular fileなら拒否します。
同じaidlc Root内で`crypto/rand.Text`による一時名を`O_CREATE|O_EXCL`で確保し、衝突は最大10回で
打ち切ります。pathを再解決する`os.CreateTemp`や、旧cursorの先行削除は使いません。

新規directoryは`0777`、新規cursorは`0666`をumask付きで使用します。既存通常cursorの更新では
tempを`0600`で作り、write・短いwriteの確認、既存permissionの9bitだけを継承するFile.Chmod、
File.Closeの順に成功してからRoot.Renameします。owner、ACL、特殊mode、hardlink同一性等は
保持保証しません。置換にはdirectory書込権限が必要で、直接上書きとは権限・metadataの扱いが異なります。

自分で作成したtempだけをcleanupし、write・Chmod・Close・Rename・cleanupの原因をwrap/joinします。
取得したRootはaidlc、projectの逆順に閉じ、Close失敗も保持します。Rename呼出前の失敗は、
他のwriterがいなければ旧cursor内容を保護しますが、新しい親directoryやcleanup失敗したtempは残り得ます。
Rename呼出以降、Root Close、成功出力の失敗では切替済みの場合があり、自動rollbackしません。
排他lock、一貫したsnapshot、敵対的並行差替えへの完全transaction、全OSのatomic更新、
fsyncによるcrash/powerloss耐久性は保証しません。Rootもmountやdeviceを防ぐ完全sandboxではありません。

### Space切替の意図的な差分

比較対象はローカルAI-DLC `2.6.123`のpublic CLI・共有cursor保存経路です。
[原典調査](ram/research/2026-09-01-space-switch-contracts.md)に基づく静的確認であり、
最新upstream全体や全ハーネスとの完全互換は主張しません。

| 本家の挙動 | 採用した挙動 | 理由 | 利用者・互換性への影響 |
| --- | --- | --- | --- |
| cursorを直接上書きし、mkdir/write失敗を吸収 | 一時fileから置換し、保存失敗をerror通知 | 失敗を成功と表示しない | directory書込権限が必要。owner/ACL等の保持は保証しない |
| 通常path操作でlinkを追従し、不在projectも作り得る | 既存projectのRoot境界とcursor型検査 | 外部・非regular配置を更新しない | 従来追従できた外向き・絶対linkやcursor symlinkも拒否する |
| 余剰引数・一部flagを無視し、raw help/-hはhelp表示 | 厳格検証とJSON error・exit 1。project-dir等号形式は既存Go方針を継続 | 誤入力を隠さず既存Go CLIと揃える | 従来無視された入力やhelp要求はerrorになる |

承認とscopeは[space切替の実装計画](ram/decisions/2026-09-01-space-switch-plan.md)、
構文・出力は[Space切替CLI](development.md#space切替cli)を参照してください。
bare `space <name>`、session binding、harness include、audit等は段階的な未実装範囲であり、
恒久的な非互換方針ではありません。

## ビルド情報

`Version`と`Commit`は通常のstring変数として安全な既定値を持ち、release buildでは`go build -ldflags -X`で差し替えます。build timestampは再現可能なbuildを損なうため保持しません。

## 現在の対象外

- AI-DLCの33ステージとagent実行
- 設定ファイルとshell completion
- ancestor探索、project自体の自動作成、既存spaceの自動修復
- intent作成・切替、space・intentの削除、registry/state本文の解釈
- `ReadSelection`・intent読み取りの公開CLI/status接続、space/intent明示override、session binding
- Cobra、Viper、GoReleaserなどの外部依存
- release、署名、公証、installer
