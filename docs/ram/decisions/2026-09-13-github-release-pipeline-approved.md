# 単一CLIのGitHub Releases配布機構を整備する

2026-09-13。ユーザーは、OS・CPU別の `aidlc` に配置資材を同梱し、GitHub Releasesからバージョンを選んで取得する方式と、`aidlc install codex` / `aidlc install claude` の引数で配置するAI環境を選ぶ方式に同意した。配布処理をこれから整備する説明に「はい、お願いします」と回答したため、[具体計画](../../design/github-release-pipeline-plan.md)の実装、検証、Issue・PRを進める。

利用者向けCLIは `aidlc` 一つを維持する。開発用 `aidlc-dist` は既存の梱包ツールであり、利用者が別途導入するものではない。資材だけを別バージョンで取得する機能や、Codex・Claude Code本体のインストールは追加しない。

配布先は `sori883/ai-dd` のGitHub Releases。既存の6対象のarchive、manifest.json、SHA256SUMSを再利用する。macOS・Linux・Windowsで、梱包した候補そのものを展開して版表示・help・新規配置を検査する。その受渡しのためにActions artifactへ候補8ファイルだけを1日保存する。検証のみの実行でも一時artifactは取得可能になる。これは[旧配布範囲](2026-09-11-distribution-packaging-scope.md)のuploadなしという境界を、今回の候補転送と手動指定時のRelease下書き作成について更新する決定である。

手動実行では、mainに含まれる既存tagのcommitを選ぶ。既定では検証までとし、明示指定したときだけ全配布検証の成功後にReleaseの下書きを作成できる。下書き作成jobだけに、その操作に必要な `contents: write` を付ける。自動publish、tag作成、既存Release・添付物の上書き、失敗時の自動削除は行わない。新しい外部Go module・ローカルtool・個人tokenは不要。

正式バージョン、Go製品のライセンス、初回公開の内容は未確定であり、この実装依頼を実tag作成や一般公開の承認として扱わない。完成した処理と候補の内容を確認可能にしてから、初回公開を具体化する。候補の生成やPRのCI成功は、実Release作成の成功とは区別する。

Claude対応のIssue #185は、残高不足による実機確認中断を受け、ユーザーが未完了のまま保留するよう指定している。追加試験・独立review・final・PR・mergeは再開せず、`/Users/const/sori883/ai-dd-naming` の未commit変更を保持する。今回の基準はPR #184がmerge済みのmain `5970c99a057077b2edd62726fcd8c4f1245a2c7d` で、Codexのみが利用できる。実装場所は独立した `/Users/const/sori883/ai-dd-release`、branchは `codex/github-release-distribution`。

本家の根拠は既存分析の固定2.6.123に限る。共通原稿と環境別配置定義は既存実装を再利用する。Go単一binaryへの同梱は既承認の方針であり、本家全体や最新upstreamの公開手順との一致を新たに主張しない。


実装では、同tagの下書きjobを直列化し、認証済みRelease一覧の全ページを確認して既存Draftも拒否する。再実行で部分Draftや添付を自動再利用しない。候補検証は同じSHAの配置資材と照合し、実Codex hookの成功証拠とは分ける。具体的な検証方法は[計画](../../design/github-release-pipeline-plan.md#実装時の具体化)に記録する。
