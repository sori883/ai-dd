# アーキテクチャ

## 方針

ai-ddは、単一のGo moduleと単一の実行ファイルを保ちながら、責務をpackage境界で分離するモジュラーモノリスです。現段階では抽象化を増やさず、Go標準ライブラリと手動dependency injectionだけを使用します。

```text
src/cmd/aidlc/main.go
  ├─ src/internal/buildinfo
  └─ src/internal/cli
       └─ src/internal/buildinfo.Info

src/internal/workspace
  └─ project root resolution (CLI integration is deferred)
```

## Package境界

- `src/cmd/aidlc`: composition rootです。process引数、stdout、stderr、build情報を組み立て、`cli.Run`の戻り値を`os.Exit`へ渡します。ドメイン判断や出力整形は置きません。
- `src/internal/cli`: CLIの引数解釈、stdout/stderrの分離、終了コード、help/versionの表示契約を所有します。`io.Writer`を受け取るため、process全体を起動せずにテストできます。
- `src/internal/buildinfo`: linkerが差し替える`Version`と`Commit`、およびそのsnapshotを所有します。既定値は`dev`と`unknown`です。
- `src/internal/workspace`: AI-DLCが対象にするproject rootの選択とpath正規化を所有します。環境変数や現在directoryを直接参照せず、process境界から受け取った候補だけで決定する読み取り専用packageです。公開CLIへの接続は`status`の契約確定後に行います。

`internal`配下はmodule外からimportできません。今後の機能も、CLIから直接filesystemやnetworkへ到達させず、責務ごとのpackageをcomposition rootで接続します。

## 手動DI

`main`が`os.Args[1:]`、`os.Stdout`、`os.Stderr`、`buildinfo.Current()`を`cli.Run`へ明示的に渡します。これがprocess境界の手動DIです。CLI packageはglobalなprocess I/Oを参照しないため、各実行の出力と終了コードを決定的に検証できます。

外部DI containerは使用しません。依存が増えた場合も、まずconstructorまたは関数引数による注入を維持し、変更理由が明確になった時点で再評価します。

## ビルド情報

`Version`と`Commit`は通常のstring変数として安全な既定値を持ち、release buildでは`go build -ldflags -X`で差し替えます。build timestampは再現可能なbuildを損なうため保持しません。

## 現在の対象外

- AI-DLCの33ステージとagent実行
- 設定ファイルとshell completion
- project rootの存在確認、ancestor探索、workspace filesystemの作成
- Cobra、Viper、GoReleaserなどの外部依存
- release、署名、公証、installer
