# Intent作業記録をOKF検索対象へ移す依頼

状態: Accepted。ユーザーが保存先をknowledge/log/へ補正した後、実装計画への「はい」を確認。直接承認として実装する。

ユーザーはwork-log.mdをKnowledge配下へ移し、ファイル名を
`<intent_id>-work-log.md`にしてOKF検索したいと指定した。
従来の[Intent配下の差戻し記録](2026-09-09-stage-rework-log-document-outputs.md)の保存先を置換する。
記録対象を全作業や全操作へ拡張する依頼ではない。

確定した保存先は`aidlc/spaces/<space>/knowledge/log/<intent_id>-work-log.md`。
frontmatterにtype: work-log、intent_id、Intent名を含むtitle、description、tags、
generatedを設定し、日時をCLIが生成する。reopenが既存の差戻し理由を本文へ追記する。
検索は現在のmetadata検索とintent_id絞り込みを使い、本文全文検索は追加しない。
state.jsonはIntent配下に維持し、記録途中の保存失敗・同一再試行・改変検出を維持する。
新規Intent向けとし、既存ファイルの削除・自動移行は行わない。

対象はflowのreopen保存処理と回帰テスト、OKF検索テスト、配布ステージ手順とhelp、
CLI結合テスト。承認に基づきIssueを作成し、単独writerのTDD、独立レビュー、
全体test/race/vet/結合テスト/配布ビルドを通してPRへ進む。外部Go moduleは追加しない。

当初のKnowledge直下案はユーザー指定でlog/配下へ置換した。
詳細な計画は[OKF作業記録の実装計画](../../design/okf-work-log-plan.md)を参照する。
