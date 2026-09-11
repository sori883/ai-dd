# worktreeでhookを使うための次の対応案

日付: 2026-09-11。状態: Proposal（製品・環境変更は未承認）。Issue #169の③について、ユーザーは調査結果を受けて「これはどうするの？」と質問した。

## 今すぐ使う方法

調整AIを動かす検証環境は、実測済みの独立cloneを使う。独立cloneはGit管理情報を別に持つコピーで、その場所のhookを通常の信頼確認で有効にできる。これは暫定運用であり、既存worktreeを自動変換する指示ではない。一つの調整役が管理する進捗・担当登録を、workerごとのcloneへ分散する案でもない。

## 恒久対応として検討する方法

固定Codex 0.153.4がhookを探す主checkoutを共通の入口にし、AI-DLCの管理先を明示する案を推奨候補とする。「hookファイルを置く場所」と「Intent・session・担当登録を保存する調整root」を分けて設計する。現在のinstallerはhook commandへ固定の `--project-dir` を埋め込むため、主checkoutに単に現行hookを配置するだけでは、別の調整rootを正しく扱えない。

workerの作業場所は既存assignmentに登録し、進捗と担当の重複検査は調整rootの一つのregistryで維持する。本件でworkerの全ファイル編集先を網羅的に制限する機能まで追加しない。

次の実装計画では、主checkout入口から許可された調整rootを選ぶ対応関係、未登録・移動済みrootでの拒否、既存hookとの共存、通常trust、復旧・rollbackを具体化する。配置先と管理先の関係が新しい運用契約になるため、③の調査承認だけでは実装しない。

## cwdだけの自動振分けは採用しない

固定版のhook JSONにある `cwd` は、各コマンドへ指定した作業場所ではなく、turn/sessionの基準ディレクトリである。Pre/Postは `turn_context.cwd` を送信し、Bashの `workdir` は別に解決する。子の会話rootが親と同じままBashだけworkerの場所で実行される場合、hookのcwdから実際のworker作業場所を特定できない。

したがって、cwdを全書込み先やworker本人の証明と扱わない。入口と調整rootの明示的な関係、session・agent情報、既存assignmentとの対応を、固定Codexの実測で確認してから採否を決める。

根拠は固定 `rust-v0.153.4` の [hook_runtime.rs](https://github.com/openai/codex/blob/rust-v0.153.4/codex-rs/core/src/hook_runtime.rs#L184-L203)、[turn_context.rs](https://github.com/openai/codex/blob/rust-v0.153.4/codex-rs/core/src/session/turn_context.rs#L218-L225)、[exec_command.rs](https://github.com/openai/codex/blob/rust-v0.153.4/codex-rs/core/src/tools/handlers/unified_exec/exec_command.rs#L185-L203)。主checkoutへ探索先を切り替える根拠は[既存調査](2026-09-11-fixed-codex-hook-reliability-contract.md)を参照。固定snapshotを最新Codexと同一視しない。

## 実装前に確認すること

隔離した主checkoutとlinked worktreeで、hookの列挙元、通常trust、実際の発火を確認する。そのうえで親・子のsession/agent/cwdと登録rootを照合し、別worktreeから操作しても進捗と担当登録が正しい調整rootに保存されること、別rootの記録を誤更新しないことを検証する。並行worker、未登録root、移動・削除・symlinkによる別名も対象にする。識別情報が不足する場合は、保護できると断定せず計画の未解決事項として残す。

Codex更新を解決策とするには、更新先の具体版で同じ試験が必要である。今回の回答では新しい版の比較・導入は行わず、独立cloneの暫定利用と共通入口案の検討を分けて説明する。
