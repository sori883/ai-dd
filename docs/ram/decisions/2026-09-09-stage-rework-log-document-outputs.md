# 差戻し理由のIntent作業記録と文書だけのoutputs

状態: Accepted requirements。Stage Graph設計検討へのユーザー補足。実装全体の承認ではない。

- 戻る理由はADRではなく、Intent配下の作業記録Markdownへ記載する。
- 段階定義のoutputsはドキュメントだけにする。プログラムを含めると一覧が大きくなるため。

[直前の設計案](../../design/stage-graph-procedure-proposal.md)を更新した。
配置名work-log.mdは提案であり、ユーザーが指定した確定名ではない。
進捗state、アーキテクチャのwhyを扱うADR、現行what/howを扱うKnowledgeの責務は維持する。
差戻しの作業理由とアーキテクチャ判断が両方ある場合も、同じ記録として混同しない。

後続のこの合意は、[4段階のフローとADR・進捗state](2026-09-08-four-step-flow-adr-and-progress-state.md)にある
作業記録不要という方針を、差戻し理由のMarkdown記録に限って補足・置換する。全操作auditの導入は含まない。
[Stage Graph案](2026-09-09-stage-graph-procedure-request.md)のoutputsへコード範囲・実行証拠を列挙する例も置換する。
コードやテスト実行証拠の既存Sensor検査は維持し、outputsへの列挙とは分ける。
state遷移とMarkdown保存の失敗時整合は実装計画で具体化する。コード・設定・Issueは変更していない。
