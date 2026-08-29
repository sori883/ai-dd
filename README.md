# ai-dd

AI-DLCを単一バイナリで実行するためのGo製CLIです。現在は、今後のワークフロー実装を支えるCLI基盤として、help、version、終了コード、ビルド情報の契約を提供します。

## 必要環境

- Go 1.26以上

実行時の追加ランタイムは不要です。Goの外部moduleにも依存していません。

## ビルド

```sh
mkdir -p bin
go build -o bin/aidlc ./src/cmd/aidlc
```

versionとcommitはlink時に設定できます。

```sh
go build \
  -ldflags "-X github.com/sori883/ai-dd/src/internal/buildinfo.Version=v0.1.0 -X github.com/sori883/ai-dd/src/internal/buildinfo.Commit=abcdef0" \
  -o bin/aidlc \
  ./src/cmd/aidlc
```

ビルド情報を指定しない場合は、versionが`dev`、commitが`unknown`になります。再現可能性を保つため、build timestampは埋め込みません。

## 使い方

```text
$ ./bin/aidlc --help
AI-DLC command-line interface

Usage:
  aidlc <command>

Commands:
  help       Show help
  version    Show version information

Flags:
  --help     Show help
  --version  Show version information
```

`help`と`--help`はhelpをstdoutへ出力し、`version`と`--version`はversion情報をstdoutへ出力します。stdoutへの書き込みに失敗した場合はstderrへ診断を試み、終了コード1を返します。不正な引数は診断とusageをstderrへ出力し、終了コード2を返します。

## 開発

ローカル検証手順は[docs/development.md](docs/development.md)、配布先での確認は[docs/e2e-testing.md](docs/e2e-testing.md)、package境界と手動DIは[docs/architecture.md](docs/architecture.md)を参照してください。

AI-DLC v2参照実装の分析資料は[docs/aidlc-analysis/README.md](docs/aidlc-analysis/README.md)にあります。

開発上の意思決定、制約、調査結果は[docs/ram/README.md](docs/ram/README.md)から参照できます。
