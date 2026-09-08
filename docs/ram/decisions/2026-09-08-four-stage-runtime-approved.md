# Git共有stateと調整役AIによる起動を採用する

状態: Accepted。ユーザーは確認中だった2点に「1.git共有します。2.推奨でお願いします」と回答した。

## 確定した選択と許可

1. Intent・Unitの進捗はSpace配下のファイルとしてGit共有する。担当sessionや実行中情報はローカルruntimeへ分ける。
2. 調整役AIがCodexの機能で並列担当・独立レビュー担当を起動する。Go CLIは割当・依存・結果・遷移を管理する。
   Go CLI自身がCodexを起動するschedulerは作らない。結果の受理は通常AI操作の運用保証であり、
   同一OS利用者からの完全な著者認証を意味しない。

[実装依頼](2026-09-08-four-stage-implementation-request.md)に残った確認事項はこの回答で解消した。
[提示した実装計画](../../design/four-stage-workflow-implementation-plan.md)をこの回答に合わせて確定し実装する。
許可範囲は4ステージ、Intent/Unit state、必須Sensorと独立レビュー、Knowledge/ADR分担、
配布・hook接続と一周確認、不要になった旧製品経路の削除。旧データ移行・互換性・新旧二重運用は不要。
ユーザーの未commit変更・利用先データ・開発RAM・他worktree・参照資料は削除しない。
Go単一バイナリ・外部Go module追加なしを維持する。

## 計画の具体化に用いる根拠

進捗の正本は計画で提示した `aidlc/spaces/<space>/intents/<id>/state.json` とする。
ADRと現行KnowledgeはOKF文書として分け、進捗・未確定事項・計画を二重の正本へ保存しない。
調整役の作業rootを一つ明示し、そこで共有stateを更新する。Unit worktreeのstateコピーから
並列の調整役を無条件起動する分散schedulerは要求しない。
Git共有はGitで保存・引継ぎできる意味であり、複数PC間のリアルタイム排他を保証しない。

統合検証後もSensorとレビューに合格してからIntent完了にする。
型・flagの具体名や標準ライブラリでの構造は、提示した意味を維持して計画の詳細契約へ記載する。
新しい重大な運用選択・本家との意図的差分が発生した場合だけ確認へ戻す。
