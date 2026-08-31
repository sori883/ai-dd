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
  └─ active-space and space listing (read-only)
```

## Package境界

- `src/cmd/aidlc`: composition rootです。process引数、stdout、stderr、build情報を組み立て、`cli.Run`の戻り値を`os.Exit`へ渡します。ドメイン判断や出力整形は置きません。
- `src/internal/cli`: CLIの引数解釈、stdout/stderrの分離、終了コード、help/versionの表示契約を所有します。`io.Writer`を受け取るため、process全体を起動せずにテストできます。
- `src/internal/buildinfo`: linkerが差し替える`Version`と`Commit`、およびそのsnapshotを所有します。既定値は`dev`と`unknown`です。
- `src/internal/workspace`: project rootの選択・path正規化と、spaceの読み取りを所有します。root解決は受け取った候補だけで決定し、space readerはproject root基準の`fs.FS`を受け取ります。環境変数や現在directoryを直接参照せず、filesystemへ書き込みません。公開CLIにはまだ接続していません。

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
symlinkも参照し得ます。名前からpathへ到達する後続機能では、検証とfilesystem境界を
別途設計します。不正UTF-8のdecodeと、Stat失敗までのOS/runtime別列挙順による部分集合の
完全互換は保証しません。

承認と詳細は[共通space読み取りの初期契約](ram/decisions/2026-08-31-space-reading-contract.md)を参照してください。

## ビルド情報

`Version`と`Commit`は通常のstring変数として安全な既定値を持ち、release buildでは`go build -ldflags -X`で差し替えます。build timestampは再現可能なbuildを損なうため保持しません。

## 現在の対象外

- AI-DLCの33ステージとagent実行
- 設定ファイルとshell completion
- project rootの存在確認、ancestor探索、workspace filesystemの作成
- spaceの作成・切替・削除、名前からのpath解決、intent/state/statusの実装
- Cobra、Viper、GoReleaserなどの外部依存
- release、署名、公証、installer
