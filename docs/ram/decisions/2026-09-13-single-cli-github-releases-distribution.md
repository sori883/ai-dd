# 単一CLIと同梱資材をGitHub Releasesで配布する

2026-09-13。ユーザーは、資材を内蔵した実行ファイルをGitHub Releasesでバージョン別に配布する案に同意し、一つのCLIの引数でインストール対象を選ぶ構成を希望した。

利用者が使う製品CLIは `aidlc` 一つとする。`aidlc install codex --project-dir ROOT` と `aidlc install claude --project-dir ROOT` で、AI-DLCの配置資材を選択する。Codex本体・Claude Code本体の取得やインストールを代行する機能を追加する判断ではない。

実行ファイルはmacOS・Linux・WindowsとCPUに応じてビルドする。AI環境ごとに実行ファイルを分けず、共通core資材と環境別adapter資材を同梱する。選んだ製品バージョンの実行ファイルに、その版のskill・agent・hook・工程定義が入る。installのたびにGitから資材を取得する機能や、資材だけ別バージョンへ切り替える機能は追加しない。

公開先は `sori883/ai-dd` のGitHub Releasesとし、既存のOS/CPU別archive、manifest.json、SHA256SUMSを利用する。開発者用の `aidlc-dist` は梱包ツールであり、利用者へ別のインストーラーとして要求しない。任意の日本語チェックCLIの別配布方針は変更しない。

mainで公開CLIへ接続済みなのはcodex。claudeの引数・adapterはIssue #185のローカル変更にある。[Claude対応を未完了のまま保留する決定](2026-09-13-claude-adapter-paused-incomplete.md)を維持し、この合意を再開や未検証変更のmergeの許可に使わない。

この回答で確定したのは配布先と単一CLIの構成である。GitHub Releasesへの公開処理はまだ実装されていない。正式バージョン、Go製品のライセンス、初回に実際に公開する成果物は未確定なので、具体化してから公開する。過去の[配布範囲](2026-09-11-distribution-packaging-scope.md)にある「公開先未確定」を本記録で更新し、過去の検証結果は保持する。
