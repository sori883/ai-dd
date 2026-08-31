# アーキテクチャ

## 方針

ai-ddは、単一のGo moduleと単一の実行ファイルを保ちながら、責務をpackage境界で分離するモジュラーモノリスです。現段階では抽象化を増やさず、Go標準ライブラリと手動dependency injectionだけを使用します。

```text
src/cmd/aidlc/main.go
  ├─ src/internal/buildinfo
  └─ src/internal/cli
       └─ src/internal/buildinfo.Info

src/internal/workspace (CLI integration is deferred)
  ├─ project root resolution
  ├─ active-space and space listing (read-only)
  ├─ active-intent and intent listing (read-only)
  └─ ReadSelection: project → current space → intents (read-only)
```

## Package境界

- `src/cmd/aidlc`: composition rootです。process引数、stdout、stderr、build情報を組み立て、`cli.Run`の戻り値を`os.Exit`へ渡します。ドメイン判断や出力整形は置きません。
- `src/internal/cli`: CLIの引数解釈、stdout/stderrの分離、終了コード、help/versionの表示契約を所有します。`io.Writer`を受け取るため、process全体を起動せずにテストできます。
- `src/internal/buildinfo`: linkerが差し替える`Version`と`Commit`、およびそのsnapshotを所有します。既定値は`dev`と`unknown`です。
- `src/internal/workspace`: project rootの選択・path正規化、space・intentの読み取りとその接続を所有します。root解決は受け取った候補だけで決定します。space readerはproject root基準、intent readerは選択済み1spaceのintents directory基準の`fs.FS`を受け取ります。`ReadSelection`は実filesystemとの接続とRootの生存期間を内部で管理します。環境変数や現在directoryを直接参照せず、filesystemへ書き込みません。公開CLIにはまだ接続していません。

`internal`配下はmodule外からimportできません。今後の機能も、CLIから直接filesystemやnetworkへ到達させず、責務ごとのpackageをcomposition rootで接続します。

## 手動DI

`main`が`os.Args[1:]`、`os.Stdout`、`os.Stderr`、`buildinfo.Current()`を`cli.Run`へ明示的に渡します。これがprocess境界の手動DIです。CLI packageはglobalなprocess I/Oを参照しないため、各実行の出力と終了コードを決定的に検証できます。

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
今回の対象外であり、既存機能を削除した非互換とは区別します。作成・切替・削除や保存形式の
変更もありません。将来consumerが返された名前を再びpathへ使う場合は、改めて検証と安全な
FS境界を用意してください。返されたboolは選択だけを表し、stateの妥当性や返却後の存在を
保証しません。

各readerは別々に読むため、並行更新中の一貫したsnapshotは保証しません。`os.Root`もmountや
特殊file/deviceを防ぐ完全なsandboxではありません。不正UTF-8、OS別の名前制約、Node/Bunの
path解釈を含めた完全互換は対象外です。

承認した差分・詳細は[workspace読み取り接続の実装計画](ram/decisions/2026-08-31-workspace-reading-composition-plan.md)を参照してください。

## ビルド情報

`Version`と`Commit`は通常のstring変数として安全な既定値を持ち、release buildでは`go build -ldflags -X`で差し替えます。build timestampは再現可能なbuildを損なうため保持しません。

## 現在の対象外

- AI-DLCの33ステージとagent実行
- 設定ファイルとshell completion
- ancestor探索、workspace filesystemの自動作成・修復
- space・intentの作成・切替・削除、registry/state本文の解釈
- workspaceの公開CLI/status接続、`ReadSelection`のspace/intent明示override
- Cobra、Viper、GoReleaserなどの外部依存
- release、署名、公証、installer
