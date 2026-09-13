# 製品を五つのCLIに分ける

2026-09-13。ユーザーは`aidlc-install`、`aidlc`、`okf`、
`natural-japanese-go`、`aidlc-dist`の五つの実行ファイルにする希望を示し、実現可能かを確認した。
[本体とインストーラーの分離](2026-09-13-separate-installer-and-runtime.md)を具体化する指定である。
インストーラー名は`aidlc-install`とし、OKFも独立したCLIとして扱う。

| CLI | 役割 |
| --- | --- |
| `aidlc-install` | 利用者向けの導入。本体・関連CLIと、対象AI環境の手順・設定を配置する |
| `aidlc` | Intent、工程、進捗、センサー、承認、担当管理、hookを扱う本体 |
| `okf` | Knowledge・ADR・Rule・作業記録の検索・作成・更新・検証 |
| `natural-japanese-go` | 日本語文章の検査 |
| `aidlc-dist` | 開発者用の配布ファイルの梱包。通常の利用先には不要 |

現行では`aidlc install codex`と`aidlc memory`が本体の公開CLIにあり、
知識処理は`src/internal/okfmemory`と`src/internal/app/command.go`に分かれている。
現在のGo実装を独立した`okf`の入口へ接続する方法で、五つのCLIへの分離は可能である。
本体のセンサー・Rule読取り・作業記録もOKF処理を使うため、単なるファイル名変更にはしない。
共有するGo処理、hookの実行ファイル識別、各CLIの引数、skillと配布の参照を具体計画で整理する。
各CLIはGoの単一バイナリとしてbuildし、共通コードのコピーは増やさない方針とする。

この実現方法は現行Go実装の切り出しを想定した説明である。
原典OKF Agent Memoryの実行ファイルをそのまま配布する決定や、原典CLIとの完全互換の確定ではない。
取得元・版の結び付け・配置先・インストーラーの引数と、0.1.1の添付内容は、
改訂する実装・公開計画で確定する。公開準備中の旧一体型構成の計画をそのまま実行しない。

本記録時点では分離コード・配布設定・Issue・PR・tag・Releaseを変更していない。
0.1.1の公開は中断中である。
