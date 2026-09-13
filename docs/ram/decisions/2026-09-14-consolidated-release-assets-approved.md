# 7個の配布物と自動選択による導入を実装する

2026-09-14。6環境の一式archiveと共通checksumで43配布物を7個に減らし、
5つのCLIは別々の実行ファイルとして維持する提案に、ユーザーは「はい。お願いします」と回答した。
続けて「インストーラーがダウンロードするアセットを自動選択するの？」と確認した。

実装の直接承認として扱う。利用者が実行する初回取得のsh/PowerShellがOS・CPUを判定し、
対象の一式を取得する。Go製`aidlc-install`も実行環境と内部manifestを確認し、
同梱runtime・資材を既存の配置処理へ渡す。取得した一式を`--release-dir`で引き継ぎ、
同じarchiveを再取得しない。既にinstallerを持つ場合の通常ダウンロードも自動選択する。

採用する配布名は`ai-dd_VERSION_OS_ARCH.tar.gz`（Windowsは`.zip`）。
各archiveに5binary、共通/Codex原稿、schema 2の内部manifest、原典の許諾文書を入れ、
外側は6archiveと`SHA256SUMS`だけにする。製品名AI-DDをarchive名に使い、CLI名は変更しない。
利用先に保存する日常用binaryは従来の`aidlc`、`okf`、`natural-japanese-go`の3つ。

[具体計画](../../design/consolidated-release-bootstrap-plan.md)はこの承認内の実装詳細を定める。
対象は梱包、取得、照合、Codex新規配置・同版移転、薄い初回取得script、CIと文書。
Go製本体への処理集約を維持し、sh/PowerShellは最初の取得に限る。
外部Go moduleや開発用外部toolを追加しない。state、hook、工程・skill本文の仕様は変更しない。
独立review、read-only final、PR checks、mergeまで進める。

[前の未承認提案](2026-09-14-consolidated-release-assets-proposal.md)の実装許可待ちを解消する。
43件を維持したまま初回取得だけを追加する旧案は検討履歴として保持する。
公開済みv0.1.1のtag・43添付を変更しない。旧43形式を新installerへ互換実装しない。
新版をv0.1.2として公開する可否は質問中で、実装許可と公開許可を区別する。

作業場所は`/Users/const/sori883/ai-dd-release`、branchは`codex/consolidated-release-assets`、
基準mainはPR #201のmerge commit `a3514ecc6ac1900a93e064e70aa1ce30efdf19e7`。
以前の未commit計画・RAMを保全して同じ作業へ含める。
対応する実装記録は[Issue #202](https://github.com/sori883/ai-dd/issues/202)。
