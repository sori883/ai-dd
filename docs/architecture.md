# アーキテクチャ

## 方針

ai-ddは、単一のGo moduleと単一の実行ファイルを保ちながら、責務をpackage境界で分離するモジュラーモノリスです。現段階では抽象化を増やさず、Go標準ライブラリと手動dependency injectionだけを使用します。

```text
src/cmd/aidlc/main.go
  ├─ src/internal/buildinfo
  ├─ src/internal/cli (arguments, output, exit code)
  └─ src/internal/workspace (space・intentのread/write API)

src/internal/graph (stage graph・scope routingのread-only query)

src/internal/scope (scope Markdown frontmatterのread-only metadata query)

src/internal/memory (4層Memory sourceのread-only acquisition)

src/internal/workspace
  ├─ project root resolution
  ├─ active-space and space listing (read-only)
  ├─ active-intent and intent listing (read-only)
  ├─ Detect: caller-owned project Root → read-only workspace signals
  ├─ ReadSpaces: project → shared-cursor space list (read-only CLI)
  ├─ ReadIntents: project → active space → registry and record list (read-only CLI)
  ├─ ReadSelection: project → current space → intents (read-only, no CLI yet)
  ├─ CreateSpace: explicit creation within an existing project
  ├─ SwitchSpace: explicit shared space-cursor update within an existing project
  ├─ SwitchIntent: explicit shared intent-cursor update within the active space
  └─ CreateIntent: lock-backed core creation within an existing space (no CLI yet)
```

## Package境界

- `src/cmd/aidlc`: composition rootです。process引数、stdout、stderr、build情報と作成・一覧・切替callbackを組み立て、`cli.Run`の戻り値を`os.Exit`へ渡します。callbackが呼ばれたときだけcwdと環境変数を読みます。ドメイン判断や出力整形は置きません。
- `src/internal/cli`: CLIの引数解釈、stdout/stderrの分離、終了コード、help/version、space create/list/switch、bare space、intent list/switchとbare intentの表示契約を所有します。`io.Writer`とcallbackを受け取るため、process全体を起動せずにテストできます。
- `src/internal/buildinfo`: linkerが差し替える`Version`と`Commit`、およびそのsnapshotを所有します。既定値は`dev`と`unknown`です。
- `src/internal/workspace`: project rootの選択・path正規化、space・intentの読み取りとその接続、read-onlyのworkspace分析、spaceとIntent coreの新規作成、space・intentの共有cursor切替を所有します。root解決は受け取った候補だけで決定し、環境変数や現在directoryを直接参照しません。space一覧は`ReadSpaces`、intent一覧は`ReadIntents`を通じてCLIへ接続し、`ReadSelection`・`CreateIntent`・`Detect`は内部APIのままです。書込みは`CreateSpace`・`SwitchSpace`・`SwitchIntent`・`CreateIntent`の明示呼出しだけで行い、接続APIのRoot生存期間も内部で管理します。`Detect`は例外としてcaller所有の既存`*os.Root`を借り、Closeしません。
- `src/internal/graph`: data directory基準の`fs.FS`からcompiled stage graphとscope gridを読み、enabled stageとrouting actionのimmutable snapshotを返します。filesystem write、project root選択、scope metadata Markdown、state遷移、agent実行は所有しません。
- `src/internal/scope`: scopes directory基準の`fs.FS`から直下Markdownの狭いfrontmatter metadataを読みます。plugin選択、graph join、state、CLI、write、Root lifecycleは所有しません。
- `src/internal/memory`: Memory root基準の`fs.FS`から`org.md`、`team.md`、`project.md`、`phases/<phase>.md`を固定順で読む4層source acquisitionと、取得済みsourceからsubstantiveなbundleを作る純粋なfilterを所有します。merge・override・frontmatter parseなどのworkflow判断、workspace path解決、Rootのopen/Closeは所有しません。実filesystemの呼出側は`os.Root.FS()`を渡し、readerはRootをCloseしません。

`internal`配下はmodule外からimportできません。今後の機能も、CLIから直接filesystemやnetworkへ到達させず、責務ごとのpackageをcomposition rootで接続します。

## Memory source reader（内部API）

`memory.ReadSources(memoryFS fs.FS, phase string) ([]memory.Source, error)`は、callerがMemory
directoryをrootとする`fs.FS`を渡して、4層のsourceをread-onlyで取得します。返却する`Source.Path`は
Memory root相対のslash pathです。

```go
type Layer string
type Source struct {
    Layer Layer
    Path string
    Content string
}

func ReadSources(memoryFS fs.FS, phase string) ([]Source, error)
```

候補は`org.md` → `team.md` → `project.md` → `phases/<phase>.md`の順だけです。`phase`は
`^[a-z][a-z0-9-]*$`のsafe slugで、既知phase enumには限定しません。欠損fileはskipし、空fileも
1つのsourceとして返します。不正phase・nil FS・不正UTF-8・その他のread errorでは、I/Oまたは
後続候補へ進まず、結果をnilにしてerrorを返します。read errorのcauseとcandidate pathはerror chain・
contextに保持します。

内容はUTF-8として検証した後にbyte列からstringへ変換するだけで、CRLF、BOM、空行、frontmatter、
末尾改行を正規化しません。未知fileはwalkせず無視し、各呼出しはfresh readです。sourceのsliceは
呼出しごとに新しく作るため、callerが返却後に変更できます。

実filesystemでは`memory/` directoryを`os.OpenRoot`で開いた`*os.Root`の`FS()`を渡します。
これによりMemory root内の通常fileと相対symlinkは読めますが、root外・絶対symlinkは拒否され、外部の
bytesを返しません。`ReadSources`はcaller-owned Rootをcloseしません。`fs.FS`型だけではこの境界を
保証しないため、`os.DirFS`等をsandboxとは扱いません。

本家AI-DLC `2.6.123`ではMemory pathのsymlinkを通常Node filesystem経由で追跡し得ます。本実装は
project外の任意file読取を防ぐため、承認済み計画に従って`os.Root.FS()`を接続境界に採用します。
そのため外向きsymlinkはerrorになりますが、通常fileとroot内symlinkへの影響はありません。比較範囲は
ローカル実装の`aidlc-graph.ts:271-324,500-529,604-655`と`aidlc-steering.ts:85-107`です。
stage固有の第5層は本家v2.6.123でも予約・未実装のため、このAPIには含めません。

Go 1.26.0〜1.26.4のroot外leaf symlinkに関するGO-2026-4970を考慮し、CIと最終検証は修正版
Go 1.26.5以降を前提にします。固定候補pathには末尾slashがないため、このreaderのpath契約には
該当条件を持ち込みません。詳細な根拠、差分承認、未解決事項は[Memory source readerの参照契約](ram/research/2026-09-02-memory-source-reader-contracts.md)と
[実装計画](ram/decisions/2026-09-02-memory-source-reader-plan.md)を参照してください。

## Memory bundle filter（内部API）

`memory.BuildBundle(sources []memory.Source) []memory.Source`は、source acquisitionが返した
各layerの本文を本家AI-DLC v2.6.123のsubstantive判定へ通し、入力順のまま採用sourceだけを返します。
`Layer`、`Path`、`Content`は変更せず、duplicate pathもdeduplicateしません。nil、空、全件filter時も
caller-ownedのnon-nil空sliceを返し、入力sliceやglobal cache・I/O・その他の副作用は持ちません。

判定は本文からclosed HTML commentを`/<!--[\s\S]*?-->/g`相当でglobal・non-greedyに除去し、
`/\r?\n/`相当で行分割した各行をECMAScriptのtrim集合（U+0009、U+000B、U+000C、FEFF、Zs、
U+000A/U+000D/U+2028/U+2029）でtrimします。trim後に空、`#`開始、shipped template preambleの
12行、ASCII hyphenだけの3文字以上の行しかない本文はfilterし、それ以外の行が1つでもあれば
substantiveです。一般のblockquote、frontmatterのfield、変更済みpreambleは保持対象です。

このfilterはmerge、override、frontmatter解釈、layer間の優先順位付けを行いません。preambleの判定は
ローカルAI-DLC v2.6.123 `core/tools/aidlc-steering.ts:25-53`の文字列と処理を根拠にしており、今回の
Go移植で新たな利用者向けの意図的差分は採用していません。詳細な比較範囲と未確認事項は[Memory bundle filterの参照契約](ram/research/2026-09-02-memory-bundle-filter-contracts.md)と
[実装計画](ram/decisions/2026-09-02-memory-bundle-filter-plan.md)を参照してください。

## 手動DI

`main`が`os.Args[1:]`、`os.Stdout`、`os.Stderr`、`buildinfo.Current()`と
`cli.Dependencies`へ、space作成・切替の
`func(rawName, explicitDir string) (string, error)`、space一覧の
`func(explicitDir string) ([]workspace.Space, error)`、intent一覧の
`func(explicitDir string) (workspace.IntentListing, error)`、intent切替の
`func(target, explicitDir string) (workspace.IntentSelection, error)`をまとめて`cli.Run`へ渡します。
CLIは構文検証後にだけcallbackを1度呼びます。callback内で`os.Getwd()`、
`AIDLC_PROJECT_DIR`、`CLAUDE_PROJECT_DIR`を読み、flag値と合わせた`RootInput`を
`workspace.CreateSpace`、`workspace.ReadSpaces`、`workspace.ReadIntents`、`workspace.SwitchSpace`または
`workspace.SwitchIntent`へ渡します。
help/versionと構文エラーではcwd・環境・FSへ到達しません。
CLI packageはglobalなprocess I/Oを参照せず、各実行の出力と終了コードを決定的に検証できます。

`main`はnil可の`PrepareOutput func()`も渡します。CLIは認識済み`space create`、
`space list`、`space switch`、bare `space`、`intent list`、`intent switch`、bare intent一覧・切替で、構文エラーを含む最初の出力・callbackより前に1度だけ呼びます。`main`側のhookで
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

## Intent一覧CLIとread-only接続

`workspace.ListIntents(intentsFS, activeOverride) ([]Intent, error)`は、上記readerと同じく
選択済みspaceの`intents/`をrootとする`fs.FS`を受け取ります。`intents.json`のregistry行を
順序どおり保持し、record directoryと相関してから、registryがclaimしなかったdirectoryを
既存`ListIntentDirs`のUTF-16順でorphanとして末尾へ追加します。返す`Repos`は常にnon-nilです。

- `dirName`がnon-emptyならその完全一致だけを使います。一致しない場合にlegacy名へfallbackしません。
- `dirName`が欠損・null・空なら`<slug>-<lowercase hex>`をlegacy候補とし、hyphenを除いたUUIDの
  末尾が同じ長さのsuffixと完全一致するときだけ対応付けます。registry-only行と重複行も保持し、
  同じdirectoryを指す重複行は両方activeになり得ます。
- orphanのUUIDは空、statusは`unknown`、reposは空配列です。表示slugは先頭の6桁日付prefix、
  または末尾のlowercase hex suffixだけを除き、それ以外の名前を制限しません。
- `activeOverride == nil`のときだけ既存`ActiveIntent(intentsFS, "")`を使います。non-nilの空文字は
  明示的な未選択であり、cursorも1件fallbackも読みません。active判定はrecord directory名です。
- registryの欠損、読取error（部分dataを含む）、不正JSON、top-level非配列は空registryへfallbackし、
  disk上のorphanは列挙します。一方、有効なtop-level配列に1行でも構造不正があればfail-closedの
  errorとし、部分一覧を返しません。必須`uuid`・`slug`・`status`は値の意味を検証せず、存在する
  stringであることだけを要求します。任意`dirName`・`scope`は欠損/null/string、`repos`は
  欠損/null/string配列だけを受理し、未知fieldを無視します。

`workspace.ReadIntents(input RootInput) (IntentListing, error)`は、`ReadSelection`と同じroot解決、
space名の1 component検証、`filepath.Localize`、`os.Root`境界を使います。project Rootから
`aidlc/spaces/<space>/intents`を相対的に開き、そのRootのFSへ`ListIntents(..., nil)`を渡します。
初回project symlinkとproject内への相対child symlinkは許可しますが、外向き・絶対child linkは
拒否します。childが`fs.ErrNotExist`ならproject path・space名・non-nil空一覧を返し、その他の
open、query、Close失敗は原因を保持したerrorです。取得済みRootはintents、projectの逆順でCloseし、
複数errorを結合します。error時の`IntentListing`はzero valueで、作成・修復・切替・書込みはしません。

CLIは`intent list`とbare `intent`を受理します。human形式はspace名のheaderと、active marker、
`dirName ?? slug`、statusを表示します。空一覧では開始方法を1行で案内し、非空でactiveがなければ
末尾に切替案内を表示します。JSONはfield順を`active`、`space`、`intents`とし、各行を
`uuid`、`slug`、`status`、`repos`、`dirName`、`active`の順で1行出力します。active不在と
dirName不在はnull、reposは配列です。内部`scope`は公開しません。認識済み構文・query・Close・
出力失敗はexit 1で、stderrへ書ける場合はJSON errorを1行出力します。stdoutの部分出力は
残り得て、stderrも書けない場合はexit 1だけを保証します。予約済みの未実装
subcommandは従来形式・exit 2です。それ以外のbare `intent <target>`は下記の切替です。

### Intent一覧の意図的な差分

比較対象はローカルAI-DLC `2.6.123`の`listIntents`・record相関・一覧表示・public引数処理を、
[調査記録](ram/research/2026-09-01-intent-list-contracts.md)に記した範囲で静的確認したものです。
最新upstream、全workflow、全配布物のruntime parityは未確認です。

| 本家の挙動 | 採用した挙動 | 理由 | 利用者・互換性への影響 |
| --- | --- | --- | --- |
| registry行を実行時に寛容に扱い、構造不正が後段まで流れ得る | 有効な配列内の不正rowを1件でも検出したらfail-closed | 部分的・誤解を招く一覧を正常結果にしない | 不正registryは一覧全体がJSON error・exit 1。欠損・不正JSON・非配列のdisk fallbackとは区別 |
| public引数処理は余剰引数やflagを寛容に扱う箇所がある | 既存space CLIと同じstrict flag、値なしJSON、project-dir分離形・等号形を使う | typoをcallback前に検出して既存Go CLIと統一 | 以前黙認された構文はerror。flagはcommand前・途中・後に置ける |
| 通常のfilesystem操作でproject不在や任意symlinkをreader側の結果へ畳み込み得る | 既存projectを必須とし、projectとintentsを`os.Root`境界で開く | 接続失敗を隠さずproject外参照を制限 | project openはerror。外向き・絶対child linkはerror、初回project linkと内向き相対linkは許可 |
| 一覧のJSON/出力は本家runtimeのserializerとwrite挙動に従う | Go標準JSON encoderと明示的short-write検出を使う | 標準libraryだけで出力失敗をexit 1へ伝える | JSONのHTML escape等は本家runtimeと異なり得る。閉pipeを含むwrite失敗は正常終了しない |

承認と詳細は[intent一覧の実装計画](ram/decisions/2026-09-01-intent-list-plan.md)を参照してください。
`ListIntents`・`ReadIntents`自体は作成・切替・書込みをしません。session binding、intent作成、state本文、audit、registry修復は対象外です。各readerを別々に
呼ぶため並行更新中の一貫したsnapshotは保証せず、`os.Root`もmountやdeviceを防ぐ完全sandboxではありません。

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

## Intent切替

`workspace.SwitchIntent(input RootInput, target string) (IntentSelection, error)`は、shared
`active-space`が指すspaceの既存Intentを選び、その`intents/active-intent`へ
UTF-8の`<実際のdirName>\n`を保存します。`IntentSelection`は`SpaceName`と
`DirName`を持ち、APIのerror時はzero valueです。ただしRename後のRoot Close errorでは
cursorが変更済みの場合があり、CLIの成功出力が失敗した場合も同様です。
どちらも自動rollbackは行いません。

既存のroot優先順位と`os.Root`境界を使い、shared `ActiveSpace`を読んでから
space名を1 componentとして検証し、対象の`intents` Rootを開きます。その同じRootの
FSで`ListIntents(..., &emptyOverride)`を呼び、active-intent cursorの読取りと1件
fallbackを避けた一覧から対象を決めます。`dirName`のcase-sensitive完全一致が
最優先で、なければ`DirName != nil`のslug完全一致が一意な場合だけ選びます。
複数のslug一致は候補directory名を含むAmbiguous、0件はUnknownとして保存前に
失敗します。registry-only行は選択できず、markerを持つorphanは完全directory名か
一意な派生slugで選べます。targetはtrim・slugify・case変換しません。

target解決後にshared `aidlc/active-space`が不在なら、同じRoot内で書込みと
Close済みのstaging fileをhard linkし、`<space>\n`をno-replaceでbest-effort補完します。
競合で先に現れたcursorは上書きせず、補完失敗もIntent切替の成否へ昇格しません。
`active-intent`はSpace切替と共通のprivate cursor primitiveで、既存symlink・非regularを
拒否し、一時fileのwrite・short write・permission復元・Closeが成功してから
Renameします。親Rootとintents Rootは内部で逆順Closeし、先行errorと結合します。

CLIは`intent switch <target>`とbare `intent <target>`を同じcallbackへ接続します。
bare `list`は一覧、`switch`は明示subcommand、`create`・`archive`・`rename`・`show`・
`birth`・`help`は予約verbとしてtargetにしません。予約名のrecordも`intent switch <name>`なら
指定できますが、raw `help`・`-h`・空targetは明示形でも拒否します。`--json`は
一覧専用で、switchでは構文errorです。成功時はstdoutに
`Active intent → <dirName> (space: <space>)\n`の1行、stderr空、exit 0です。
認識済みswitchの構文・query・保存・Close・出力errorは、stderrに書ければ
JSON errorを1行出しexit 1です。stdoutの部分出力や保存済みcursorは残り得て、
stderrも書けなければexit 1だけを保証します。

### Intent切替の意図的な差分

比較対象はローカルAI-DLC `2.6.123`のpublic CLI・対象解決・共有cursor保存経路を、
[原典調査](ram/research/2026-09-01-intent-switch-contracts.md)に記した範囲で静的に確認したものです。
最新upstream全体や全workflowとの完全互換は主張しません。

| 本家の挙動 | 採用した挙動 | 理由 | 利用者・互換性への影響 |
| --- | --- | --- | --- |
| `active-intent`を直接best-effort write | 一時fileから置換し、保存失敗をerror通知 | 失敗を成功表示しない | directory書込権限が必要。error時にも置換済みの場合がある |
| 通常のpath操作 | 既存projectと`os.Root`境界、cursorの通常file制約 | root外linkや特殊cursorへの書込みを防ぐ | 外向き・絶対linkとcursor symlinkを拒否。初回project linkと内向き相対linkは許可 |
| 余剰引数・一部flagを無視し、helpを表示 | 厳格構文とJSON error・exit 1、project-dir等号形 | 誤入力をcallback前に検出し、既存Go CLIと揃える | 従来無視された入力やhelp要求がerrorになる |

active-spaceのno-replace補完とtarget解決順は本家に合わせます。session binding、
rebind offer、UUID stamp、auditは段階的な未実装範囲であり、恒久的な差分方針ではありません。
一覧と保存間の同時変更、active-spaceとの競合、全OSのatomic性、fsync・crash耐久、
multi-file transaction、owner・ACL・特殊mode・hardlink identityの保持は保証しません。
`os.Root`もmountやdeviceを防ぐ完全sandboxではありません。

詳細と承認は[Intent切替の実装計画](ram/decisions/2026-09-01-intent-switch-plan.md)、
構文・検証は[Intent切替CLI](development.md#intent切替cli)を参照してください。

## Intent作成の内部coreとworkspace lock

`workspace.CreateIntent(ctx, root, input) (CreatedIntent, error)`は、callerが選択・検証した
`SpaceName`、`Label`、任意の`Scope`・`Repos`を受け取り、既存SpaceのIntent coreを作成する
内部APIです。結果はUUID、正規化slug、実directory名、record path、Space名を返します。
scopeとreposの意味検証はcaller責務で、coreはreposの順序を維持し、空ならfieldを省略します。
公開CLI、session binding、audit、full state、workspace scanはまだ接続しません。

処理はproject identityを共有するWORKSPACE lockを取得してから、次の順で行います。

1. 既存projectとSpaceの`intents` Rootを開き、`intents.json`全体をstrictに検証します。
2. UUIDv7、24文字slug、予約語、UTC日付と`-2`から`-999`の衝突suffixを決めます。
3. record directoryと正確に`# AI-DLC State Tracking\n`だけを持つstate stubを排他的に作ります。
4. 既存registry rowのraw JSONと未知fieldを保持し、新規rowを2-space indent・末尾LFで
   sibling tempからRenameします。このRename成功を作成のcommit境界とします。
5. shared `active-space`が不在ならno-replaceで補完し、対象Spaceの`active-intent`を
   安全な一時file置換で保存します。

registry commit前のerrorではzero resultを返します。commit後のcursor、取得済みRootのClose、
lock releaseのerrorでは作成済み`CreatedIntent`とerrorを同時に返すため、callerは自動retryせず
結果を確認できます。stub・registry・cursorの途中成果物はrollbackしません。registryのRenameは
half-writeを避けますが、fsync、power-loss耐久、複数fileのatomicity、全OSでの同一semanticsは
保証しません。

lockは可能ならsymlinkを解決した絶対project path（Windowsは本家のECMAScript default lowercaseと
同じU+0130展開・Final Sigma文脈を適用）、NUL、
`__workspace__`から本家互換のMD5先頭8文字を作り、system tempの
`.aidlc-audit-<hash>.lock`を`Mkdir`で排他取得します。MD5はidentity互換用であり、security用途では
ありません。lowercase互換はBun Windows `1.3.14`のICU `73.2` / Unicode 15を基準とし、
GoのUnicode tableへUnicode 15 overlayを適用してtoolchain更新によるlock identityの変化を防ぎます。
Bun macOSのsystem ICU結果には追従せず、未監査のGo Unicode versionはtestで拒否します。
owner stampはPID、epoch milliseconds、random generation tokenを持ち、100ms間隔・
最大600 retriesで待ちます。callerのcancel/deadlineを優先し、自分のtokenと一致するgenerationだけを
解放します。stale・malformed・dead ownerは自動回収せず、診断後の手動復旧が必要です。

projectの初回path symlinkは既存root規則どおり追従します。Space名は正規化前にsingle componentとして
検証し、以後のchild accessは`os.Root`内で行います。内向き相対linkは許可され、外向き・絶対・
broken linkとregistry/`active-intent`のsymlink・特殊fileは拒否します。ただし`os.Root`はmountやdeviceを
防ぐ完全sandboxではありません。

### Intent作成coreの意図的な差分

比較対象はローカルAI-DLC `2.6.123`です。authored core、canonical Codex dist、配置済みCodex distの
作成関連`aidlc-lib.ts`と`aidlc-utility.ts`が同一であることを
[原典調査](ram/research/2026-09-01-intent-create-contracts.md)で静的に確認した範囲であり、
最新upstream全体との一致は主張しません。

| 本家の挙動 | 採用した挙動 | 理由 | 利用者・互換性への影響 |
| --- | --- | --- | --- |
| malformed・非配列registryを空listとして上書きし得る | 既存rowをstrict検証し、異常時は無変更でerror | registry消失を防ぐ | 壊れたworkspaceは作成前に修復が必要 |
| raw coreが不在Space treeも作成し得る | 既存Spaceを必須にする | typoによる暗黙Space作成を防ぐ | Spaceは既存APIで先に作成する |
| cursor failureをbest-effortで吸収 | commit済み結果とerrorを返す | 失敗通知と重複retry防止を両立する | errorでもIntentが作成済みの場合がある |
| 通常path操作 | `os.Root`、single component、通常file制約 | root外linkと特殊fileへの書込みを拒否する | 外向き・絶対linkやsymlink cursorは拒否される |
| owner generation付きstale-lock reaper | 既存lockを自動回収しない | 誤ったlock奪取を避ける | crash後は診断と手動復旧が必要 |

詳細と承認は
[Intent作成coreの実装計画](ram/decisions/2026-09-01-intent-create-core-plan.md)、
検証手順は[Intent作成core](development.md#intent作成core内部api)を参照してください。

## 読み取り専用workspace分析

`workspace.Detect(projectRoot *os.Root) ScanResult`は、callerが既に開いたproject Rootから
`ProjectType`、`Languages`、`Frameworks`、`BuildSystem`、`NestedRoot`、
`Submodules`を算出する内部APIです。Rootの選択・open・Closeはcallerが所有し、
scannerはfilesystemを書き換えず、個別のread・Stat・JSON parse errorをそのsignalの不在として
吸収します。空の`NestedRoot`はnested hitなし、non-nilの空`Submodules`は有効な
`.gitmodules` entryなしを表します。

root直下fileと既知source directoryを言語別に数え、frameworkは固定順、build systemは
本家の優先順でまとめます。`package.json`はplain objectだけをdocumentとして認め、
weakly typedなdependency fieldにはJavaScriptの`Object.keys`・object spread・truthinessの
必要範囲を適用します。rootにBrownfield signalがない場合だけ、除外directoryと
symlinkを飛ばして最大3階層のnested workspaceを探します。hit後はその下へ降りず、
nested pathだけをJavaScriptのUTF-16 code-unit順で並べ、`/`区切りで返します。

言語はcount降順のstable sortで、secondaryは
`max(1, floor(primary count * 0.2))`以上を残します。
systemのdirectory列挙順を保つため`Root.Open`と`File.ReadDir(-1)`を使い、同数に新しい
lexical tie-breakは追加しません。そのため、Bun・Go・OS・filesystemが異なる場合の
同数言語の表示順は完全互換を保証しません。`.gitmodules`は安全なpathの宣言順を保ち、
`<path>/.git`の存在だけで`Initialized`を決めます。Git commandやrepositoryの妥当性検証は
行いません。このAPIはstage graph、state、audit、Intent作成core、CLIにはまだ接続していません。

### Workspace分析の意図的な差分

比較対象はローカルAI-DLC `2.6.123`の`detectWorkspace`とそのhelperです。
authored implementation、canonical Codex dist、配置済みCodexの対象`aidlc-utility.ts`が
同一SHA-256であることを[2026-09-02の原典調査](ram/research/2026-09-02-workspace-detection-contracts.md)に
記録した範囲で確認しました。最新upstream全体との一致は主張しません。

| 本家の挙動 | 採用した挙動 | 理由 | 利用者・互換性への影響 |
| --- | --- | --- | --- |
| 通常のfilesystem APIがconfigやsubmoduleのroot外・絶対symlinkを追従し得る | 既存`os.Root`境界でroot外・絶対linkを拒否し、内向き相対linkだけをRootの規則で参照 | 承認済みworkspace APIの安全境界を継承し、scannerから緩和経路を作らない | 境界外configの本文signalは得られず、境界外のsubmodule `.git`は未初期化となる |

`os.Root`もmountやdeviceを防ぐ完全sandboxではなく、並行変更中の一貫したsnapshotも
保証しません。詳細と承認は
[読み取り専用ワークスペース分析の実装計画](ram/decisions/2026-09-02-workspace-detection-plan.md)、
検証手順は[読み取り専用workspace分析](development.md#読み取り専用workspace分析内部api)を参照してください。

## Stage graph・scope routing

`graph.Load(dataFS fs.FS) (Snapshot, error)`は、data directoryをrootとするread-only FSから
`stage-graph.json`と`scope-grid.json`を読みます。stageはJSON配列順を維持し、`enabled:false`を
公開snapshotから除外します。`Stage`はslug、number、name、phase、execution、lead agent、
support agents、mode、fallback用scopesを保持します。さらに、Stageが生成する必須成果物
`produces`、条件付き成果物`optional_produces`、入力成果物の`consumes`、依存・表示順序の
`requires_stage`をcompiled metadataとして保持します。`Consume`は`artifact`、`required`、
任意の`conditional_on`（`brownfield`または`greenfield`）を値として持ちます。

`produces`、`consumes`、`requires_stage`はJSON上の必須配列です。欠損・`null`・型不正はLoad
errorとし、空配列は有効です。`optional_produces`は欠損を許容し、存在する場合は配列として
読みます。`consumes`の`artifact`と`required`は必須で、`required:false`と欠損を区別します。
stageおよびconsume行のfield名は大小文字を含む完全一致で解釈し、未知fieldは無視します。

`requires_stage`は全graphの既知slugを参照し、参照先の`number`が当該stageより前であることと、
同一stage内の重複edgeがないことをLoad時に検証します。このmetadata検証は依存先stageを実行
closureへ自動追加せず、`enabled`やscope内外でedgeを絞りません。producer-consumerの意味検証と
scopeごとの実行計画への反映は、後続のStage Plan builderが所有します。

Snapshotは`Stages`、`ScopeNames`、`Scope`を、Scopeは`Action`と`Actions`を公開します。
未知scopeはbool false、partial action mapにstage cellがない場合は本家runtimeどおり`SKIP`です。
disabled stageへのgrid参照は全graphに存在するためvalidですが、公開`Actions`からは除外します。
`ScopeNames`はexplicit gridとfallbackのどちらも本家JavaScript互換のUTF-16 code-unit順で、
JSON objectの記述順には依存しません。
返すslice、map、Stage内sliceは防御的copyで、callerの変更はsnapshotへ反映されません。

scope gridのread errorまたはJSON構文errorでは、enabled stageの`scopes`をruntime
`loadScopeMapping`系と同じ純粋membershipで転置します。scope名を同じUTF-16順にsortし、各enabled stageに
`EXECUTE`または`SKIP`を作ります。compiler / designer側のinitialization特例はこのqueryの
対象ではありません。scopeのdescription、depth、keywords、test strategy等は
`.codex/scopes/*.md`側のmetadataであり、このpackageは読みません。

stage graphのread・decode error、必須field、`support_agents`、slug・number重複、execution enumを
fail-closedにします。Stage metadataの必須配列、consume行の必須field、conditional値、依存edgeの
既知slug・number順・重複も同じ境界で検証します。stage field名は本家`JSON.parse`と同じ完全一致で
読み、大小文字違いはunknown fieldとして無視します。gridは構文上validでもtop-level、scope entry、
必須`stages`が構造不正ならfallbackせずerrorにし、action enumと全graphへのstage参照も検証します。
unknown JSON fieldは将来互換のため無視します。nil FSはpanicせずerrorで、error時はzero Snapshotです。

### Stage routingの意図的な差分

比較対象はローカルAI-DLC `2.6.123`のruntime graph / scope loaderです。
[原典調査](ram/research/2026-09-02-stage-routing-contracts.md)に記したsourceとcanonical / 配置済み
Codex dataだけを静的に確認した範囲であり、最新upstream全体との一致は主張しません。

| 本家の挙動 | 採用した挙動 | 理由 | 利用者・互換性への影響 |
| --- | --- | --- | --- |
| JSON parse後のgraph/gridをruntime typeへcastし、構造・enum・参照をLoad境界では網羅検証しない | 必須stage field、metadata配列とconsume行、依存edgeの既知slug・number順・重複、execution/action enum、grid構造、stage参照をLoad時にfail-closed検証する | 壊れたroutingやStage Planの入力を正常snapshotとして後続へ渡さない | 本家で遅延errorや暗黙SKIPになり得るmalformed dataがLoad errorになる。正常data、grid read/syntax fallback、missing actionのSKIPは維持 |

`fs.FS`自体はsandboxを保証しません。供給FSのcontainmentとlifecycle、2 data fileの並行更新中の
一貫性、version migrationはcaller側の責務です。詳細と承認は
[Stage catalog metadataの参照契約](ram/research/2026-09-03-stage-catalog-metadata-contracts.md)、
[Stage catalog metadataの実装計画](ram/decisions/2026-09-03-stage-catalog-metadata-plan.md)、
[Stage routingの実装計画](ram/decisions/2026-09-02-stage-routing-plan.md)、検証手順は
[Stage graph・scope routing](development.md#stage-graphscope-routing内部api)を参照してください。

## Scope metadata reader

`scope.ReadAll(scopesFS fs.FS) ([]Metadata, error)`は、scopes directoryへroot化済みのread-only FSから
直下`.md`だけを読みます。filenameを本家JavaScript互換のUTF-16 code-unit順に処理し、filenameの
prefixやstemとfrontmatter `name`の一致は検証しません。duplicate nameはsort順の先後両filenameを
示して失敗します。error時にpartial resultを返しません。

Metadataはname、plugin、depth、description、keywords、test strategy、runner、skeleton、review cap、
freeform defaultを保持します。nameだけが必須です。runnerはexact true / falseだけをpointerへ保持し、
invalidは未指定です。skeletonとreview capは既知値以外をerrorにし、pluginの`aidlc-` prefixを
core runner pathとの衝突として拒否します。unknown fieldは無視します。

parserは本家2.6.123のzero-dependency frontmatter subsetだけを扱います。file先頭delimiter、LF / CRLF、
同一行scalar、quote除去、block marker空値、indent block list、single-line flow listを対象とし、一般YAML
parserではありません。scalarとflow値のtrimはECMAScript whitespaceに合わせ、U+FEFFを除去し、U+0085
は保持します。opening / closing delimiterではLF / CRLFを認識しますが、captureしたfrontmatter内部の
改行は変換しません。keywordsはfrontmatter全体で最初のvalid block sequenceをflowより優先し、blockが
なければ最初のflow sequenceを使います。block matcherはkeyとitem間のJavaScript whitespace行を許容し、
dash後が複数whitespaceだけのitemも本家regexどおり成立させて1文字のwhitespaceを保持します。horizontal
whitespace runの直後がlone CR / U+2028 / U+2029 / block終端の場合も、runが2文字以上ならinner regexの
backtrackingで最後のspace / tabをitemとして保持し、runが1文字ならemptyです。outer matcherはCR / LFだけを
item境界とし、inner extractorはCR / LF / U+2028 / U+2029をpayloadとして読みません。
outerのraw captureはoptional CR / LF terminatorと直後の反復itemを保持し、innerではCRLF / LFだけを
itemへ分割します。そのためlone CRで連結された反復itemはinnerで1行の不正値となりemptyです。matcher
成立後に抽出結果がemptyでもflowへfallbackしません。flowはquote内comma / bracketとterminal commentを
扱い、malformed listはemptyになります。loaderはcacheを持たず、毎回FSを読み、返すslice・keywords・
recordごとのrunner pointerをcallerが所有します。

### Scope metadata readerの意図的な差分

比較対象は[参照契約](ram/research/2026-09-02-scope-metadata-contracts.md)に記したローカルAI-DLC
`2.6.123`の範囲であり、最新upstream全体との一致は主張しません。

| 本家の挙動 | 採用した挙動 | 理由 | 利用者・互換性への影響 |
| --- | --- | --- | --- |
| root列挙の任意errorをemptyへ吸収 | `fs.ErrNotExist`だけempty、他errorはcause保持 | 欠損と読取不能を区別 | permission / I/O異常をcallerが検知できる |
| 空scalarが次fieldを値として読む場合がある | scalarは同一行だけを読む | 隣接fieldの誤読防止 | malformed inputが別field値に化けない。正常配布dataは不変 |
| global cacheをresetまで再利用 | cacheなしで毎回再読込しcaller所有値を返す | stale readと共有mutationを避ける | 呼出しごとのI/Oは増えるが変更を次回読取で観測する |

供給`fs.FS`自体のcontainmentとlifecycleはcallerの責務です。詳細と承認は
[Scope metadataの実装計画](ram/decisions/2026-09-02-scope-metadata-plan.md)、検証手順は
[Scope metadata reader](development.md#scope-metadata-reader内部api)を参照してください。

## 初期 aidlc-state.md builder

`state.BuildInitial(input Input) (Initial, error)`は、既に解決済みの`graph.Snapshot`、該当scopeの
`scope.Metadata`、workspace分析の専用`WorkspaceInfo`、開始日時、project descriptionを受け取り、
filesystemへ触れずに初期stateの内容を作ります。`state`はworkspace packageをimportせず、
`WorkspaceInfo`に`ProjectType`、`Languages`、`Frameworks`、`BuildSystem`だけを持たせます。

`Input.ProjectDescription`はsidecarへ保存するraw文字列、`Input.ProjectDescriptionPreview`はcallerが
解決済みの安全なsingle-line表示値です。builderはraw値をstate本文へ再利用しません。返却される
`Initial`は`StateContent`、`ProjectDescriptionJSON`、構造化`Routing`を持ち、sidecar内容は標準
`encoding/json`でJSON構文を生成した後、本家`JSON.stringify`がescapeしない`<`、`>`、`&`、
U+2028、U+2029の過剰escapeだけを安全に戻して末尾LFを付けます。raw値が空なら本家と同じ
`[Project description]`を使います。execute/skipの`StageRoute` sliceはgraphや相互の結果と共有しません。

depthは明示override > scope metadata、test strategyは明示override > scope metadata > effective depthで
解決し、3値をcase-insensitiveに`Minimal` / `Standard` / `Comprehensive`へcanonicalizeします。
review overrideは未指定または`adversarial`を空保存し、`advisory` / `none`だけcanonical lowercaseで保存
します。未知scope・無効設定・scope metadataとの不一致はerrorです。

stage routingはgraph順、missing cellは`SKIP`です。Greenfieldでraw scopeがreverse-engineeringをEXECUTE
しているときだけeffective mappingをSKIPへ補正し、skip末尾へ`number (reverse-engineering — greenfield)`
を追加します。incremental scopeで同条件の場合は構造化warning boolを返します。firstは補正後mappingの
最初のpost-init EXECUTE、該当なしは`intent-capture` / `IDEATION` / `aidlc-product-agent`へfallbackします。
nextは補正前raw mappingでfirstより後の最初のEXECUTE、該当なしは`none`です。

state本文は本家2.6.123のsection、field、コメント、phase順、空行、末尾LFを維持します。初期化stageは
`[x]`、それ以外は`[ ]`、補正後firstだけ`[-]`、phaseは`Verified` / `Active` / `Pending` / `Skipped`を
本家と同じ規則で出します。State Versionは`8`、Project Description Sourceは`project-description.json`
固定です。

本家の`handleIntentCreateStateBuild`が同時に担うworkspace scan、plugin選択、audit、state/sidecarの
filesystem write、Intent作成、CLI接続、stage本文実行はこの内部APIの責務ではありません。Goのpackage
分離と専用DTOは段階移植の依存方向であり、利用者が観測する意図的な本家差分ではありません。根拠と
確認範囲は[初期 state builderの参照契約](ram/research/2026-09-02-initial-state-builder-contracts.md)、
実装・targeted検証手順は[初期 state builder](development.md#初期-aidlc-state-builder内部api)を参照してください。

## 初期state永続化writer

`state.WriteInitial(recordRoot *os.Root, initial Initial) error`は、既にcallerが開いたrecord rootの
固定leafへbuilderの2つのpayloadを保存します。`project-description.json`へ
`Initial.ProjectDescriptionJSON`を保存してから、`aidlc-state.md`へ`Initial.StateContent`を保存します。
payloadは文字列をそのままbytesとして扱い、空payloadも有効です。record rootの選択・open・Close、lock、
record directoryの作成はcallerの責務であり、writerはrootをCloseしません。

既存の`aidlc-state.md`は、sidecarの保存後、stateの置換前に`Lstat`で通常fileであることを確認し、非truncateの
`O_WRONLY` open/closeをwrite barrierとして実行します。directory、symlink、その他の特殊file、または
barrierの失敗ではfail-closedし、stateを変更しません。sidecarは本家の保存順どおり既にcommit済みとなり、
rollbackしません。stateが不存在の場合はbarrierを省略して作成します。既存stubの存在は前提にしません。

各leafは同一directoryの一意なsibling temporary fileを`O_WRONLY|O_CREATE|O_EXCL`で作成し、全量write、
close、`Root.Rename`の順で置換します。short writeは失敗として扱い、close前のrenameは行いません。
衝突したtemporary fileは再試行するだけで削除せず、writerが取得したtemporary fileだけを失敗時にcleanup
します。Windowsで既存leafの原子的置換が保証されない場合もdelete-before-rename fallbackは行いません。

sidecarのrename成功が最初のcommit境界です。sidecarの作成・書込み・close・renameに失敗した場合はstateへ
進まず、stateの保存に失敗した場合はsidecarをrollbackせず、旧state（または不存在状態）を保持します。
したがって2ファイル全体の原子的commitや、crash後のdurabilityはこのAPIの保証ではありません。

### 初期state永続化writerの意図的な差分

比較対象はローカルAI-DLC `2.6.123`の`writeStateFile` / `writeFileAtomic`と初期state build経路であり、
最新upstream全体との一致は主張しません。成功bytes、保存順、partial commit、temporaryのclose-before-rename、
Windowsでの非atomic可能性は本家に合わせています。2026-09-02にユーザーから、nonregular stateを検出したら
異常として停止する方針の明示承認を得ています。
根拠範囲は`docs/実装_aidlc-workflows/core/tools/aidlc-lib.ts:16453-16478`の`writeStateFile`、同`:18166-18199`の
`writeFileAtomic`、および初期state build経路です。

| 本家の挙動 | 採用した挙動 | 理由 | 利用者・互換性への影響 |
| --- | --- | --- | --- |
| temporary cleanupのerrorを抑止 | 元の失敗原因とcleanup errorを`errors.Join`で返す | cleanup失敗も診断可能にする | `errors.Is`で元原因とcleanup原因を個別に検査でき、失敗時のerror文字列は追加情報を含む |
| `aidlc-state.md`の存在確認・write可否確認後、nonregular leafもatomic renameの対象になり得る | `Lstat`で通常file以外（symlink、directory、FIFO等）を検出したらfail-closedし、stateを変更しない | 異常なstate対象を黙って置換せず、FIFO等へのblockや意図しないleafの置換を避けて診断可能にする | 通常workspaceの挙動は不変。特殊なleafがあるworkspaceではsidecar commit後にerrorとなり、既存leafの種類・内容を保持する |

nonregular leafを復旧するforce/repair操作は現時点のinternal writer・Go CLIにはありません。明示確認後、利用者または
将来の上位CLIが異常leafを削除せず一意な別名へ退避し、同じ初期化処理を再実行することで通常stateを作成できます。
symlinkの場合はリンク本体を退避し、リンク先は変更しません。自動削除や低レベルwriterのforceは追加しません。

詳細な根拠、TDDの対象、確認範囲は[初期state永続化writerの実装計画](ram/decisions/2026-09-02-initial-state-writer-plan.md)を参照してください。

## ビルド情報

`Version`と`Commit`は通常のstring変数として安全な既定値を持ち、release buildでは`go build -ldflags -X`で差し替えます。build timestampは再現可能なbuildを損なうため保持しません。

## 現在の対象外

- stage遷移の実行、agent dispatch、stage definition・scope metadataの編集
- 設定ファイルとshell completion
- ancestor探索、project自体の自動作成、既存spaceの自動修復
- Intent作成の公開CLI・full handler、space・intentの削除、registry/state本文の解釈
- `ReadSelection`・intent読み取りの公開CLI/status接続、space/intent明示override、session binding
- Cobra、Viper、GoReleaserなどの外部依存
- release、署名、公証、installer
