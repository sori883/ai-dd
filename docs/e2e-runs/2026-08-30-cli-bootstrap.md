# CLI bootstrap distribution smoke E2E — 2026-08-30

- 結果: Passed
- 種別: local manual distribution smoke E2E
- source commit: `62e126c4aac64495357b008987421af657e7813e`
- build version: `e2e-20260830`
- 環境: `Darwin 25.5.0 arm64`
- Go: `go1.26.4 darwin/arm64`
- 配布先: `/Users/const/sori883/haihu-aidlc/e2e/2026-08-30-cli-bootstrap/aidlc`
- artifact: Mach-O 64-bit arm64、2,493,730 bytes
- SHA-256: `12a9675fef6ffe25757975803f218a7adee9794c6da7b665c8366e74de7fdeef`

## Build

repository rootから次の条件で、repository外の新規scenario directoryへ直接buildした。

```sh
go build -trimpath \
  -ldflags "-X github.com/sori883/ai-dd/src/internal/buildinfo.Version=e2e-20260830 -X github.com/sori883/ai-dd/src/internal/buildinfo.Commit=62e126c" \
  -o /Users/const/sori883/haihu-aidlc/e2e/2026-08-30-cli-bootstrap/aidlc \
  ./src/cmd/aidlc
```

## 観測結果

すべて配布先directoryをworking directoryとして実行した。

| 入力 | 期待exit | 実exit | stdout | stderr | 結果 |
| --- | ---: | ---: | --- | --- | --- |
| 引数なし | 0 | 0 | help、197 bytes | 空 | Passed |
| `--help` | 0 | 0 | help、197 bytes | 空 | Passed |
| `--version` | 0 | 0 | `aidlc e2e-20260830 (commit 62e126c)` | 空 | Passed |
| `unknown` | 2 | 2 | 空 | unknown arguments診断とhelp、234 bytes | Passed |

## 判定

repository外へ配置した単一binaryが追加runtimeなしで起動し、現行CLI契約どおりに
help、version、invalid inputを処理したため、このscenarioはPassedとする。

## 未検証範囲

- Codex向けagent、skill、hook、rule等の資産展開
- install、update、既存fileとのmerge、rollback
- project root resolverのCLI接続
- space、intent、state、auditを含むworkspace lifecycle
- LinuxとWindowsでのnative実行

これらは現行Go版に公開機能がないため、今回の合格判定には含めない。
