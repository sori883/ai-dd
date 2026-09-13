# 本体と利用者向けインストーラーを別の実行ファイルにする

2026-09-13。ユーザーは0.1.1の公開を依頼した後、公開準備を止め、
本体とインストーラーが同じ実行ファイルかを確認し、別々にするよう指定した。
現在は`aidlc install codex`と日常のIntent・Knowledge・hook処理が同じ`aidlc`にある。

この指定により、[単一CLIの配布方針](2026-09-13-single-cli-github-releases-distribution.md)と
[Release機構の方針](2026-09-13-github-release-pipeline-approved.md)のうち、
利用者向けの本体と配置操作を一つのCLIへ統合する部分を置き換える。
共通手順を一か所で管理し、AI環境との差分をハーネスへ置く構造は維持する。

合意した方向は、導入時に使うインストーラーと、AI・hookが日常的に呼ぶ本体を分けること。
本体の現名は`aidlc`。インストーラーの仮名を`aidlc-install`として説明するが、
名称・引数・取得方法・配置場所は、分離の具体計画で確定する。
Goの単一実行ファイルという性質は、それぞれのCLIについて維持する。
任意の日本語補助CLI`natural-japanese-go`、開発者用梱包CLI`aidlc-dist`とは役割を区別する。

0.1.1の公開作業は分離後の構成を確認するまで中断する。
公開版名は以前の0.1.0指定から0.1.1へ変更する。
独自部分のライセンス・著作権者表記と、日本語補助CLIのRelease添付範囲は回答待ちのままである。
本記録時点では製品コード・配布設定・Issue・PR・tag・Releaseを変更していない。
作業先は`/Users/const/sori883/ai-dd-release`、branchは`codex/release-0-1-1`、
基準mainは`64833d7e94203be0284c18abbf4a35b23810cf77`。
