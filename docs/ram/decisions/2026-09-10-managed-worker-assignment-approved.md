# A案の作業割当管理とメインAIによる起動を採用する

状態: Accepted（A案の方針と保証範囲）。詳細契約は実装計画で確定する。2026-09-10。

## 合意

ユーザーは、エージェントをCLIから起動したくないことと、現行ではメインAIが標準ツールで起動していることを確認した。回答では、A案もこの起動方法を維持し、CLIは作業場所の登録と重複防止を担うこと、保証するのは管理された作業割当の重複防止であり実worker全稼働の制限ではないことを説明した。その後の「はい、一応、ではお願いします」をA案で進める承認として記録する。

1. メインAIがCodexの `spawn_agent` 等の標準ツールで子を起動する。Go CLIにエージェントの起動を代行させない。
2. 正規CLIによる作業割当で、同じ管理元の使用中worktreeを複数workerへ二重に割り当てない。別worktreeでの並列実装を維持する。
3. 当初の実worker全稼働を対象とする保証から、管理された作業割当の重複防止へ対象を変更する。登録を経由しない起動や残存processの全稼働を常時検知・停止する保証はしない。
4. 現在ステージの既存担当定義による起動前チェック、Unitなしの割当、1調整root内の全Space/Intent/sessionを対象とする方針を維持する。全サブエージェント人数の制限へ変更しない。
5. 結果提出と作業場所の解放を分け、`reported`、SubagentStop、時間経過だけで自動解放しない。

## 承認の扱いと後続作業

[A/B比較](2026-09-10-upstream-aligned-worker-control-options.md)と[CLIから起動しない制約](2026-09-10-native-agent-launch-required.md)に記載したA/B未選択の状態を、本記録で更新する。以前の記録は当時の履歴として保持する。

作業登録・停止確認・再開・保存失敗・Unit連携・配布の具体契約を既存仕様と照合し、自己完結した計画へ記載する。既存仕様と本合意で一意に決まる詳細は進める。未知の重要な公開API・永続data・復旧/互換性の選択を、この方針承認から一律に許可されたとは扱わない。専用起動CLI、外部Go module、複数管理元やPC間の分散排他へ拡張しない。

本家の比較対象はローカル固定AI-DLC 2.6.123の[確認済み範囲](2026-09-10-upstream-agent-dispatch-and-worktree-reference.md)。全ステージの担当照合、チーム運用外も含む管理root内の割当必須化、Unitなし割当、停止不明の場所を保持する点は、A案で示した意図的な追加である。利用者には開始前の登録と再利用時の明示確認が加わる。

## 具体計画と確認待ち

[メインAIによる起動を維持した担当・作業場所の管理](../../design/native-agent-assignment-plan.md)に、公開CLI案、ローカルruntime schema、Unit連携・部分保存、native task対応、単独writer、順序付きTDD、独立reviewとread-only final、配布・信頼・ロールバックを記載した。

通常の解放をメインAIの理由付き確認で認めるか（Q1）、初回管理開始および復元不能な管理情報の作り直しを人間確認にするか（Q2）は未回答。A案の選択を再質問せず、この運用責任の2点を具体計画とともに確認する。コード・設定・Issue・PRの変更やテスト・新しい実機実験にはまだ着手していない。

読み取り専用の計画確認で、read-only担当のPost欠落からの回復、releaseの親session引数、横断予約を持たない旧Unitの再開経路の3点を補正した。read-onlyは旧taskを再利用せず新しい名前で起動できる。releaseは親sessionと照合する。旧実行済みUnitへの予約後付け移行は設けず、新定義の新Intentで作業を登録する。これらは具体計画へ反映済みで、製品コードの検証結果ではない。

既存G0 rawの追加読取りでは、lifecycleのspawn/followupのPre/Postにsession_id、turn_id、tool_use_idがあり、spawn入力にagent_type/task_name、Post応答にcanonical task path、followup入力に相対task_nameのtargetがあることを確認した。原資料は `aidlc-agent-hook-probe-2918763566/lifecycle/events/`、既存の[結果記録](2026-09-10-agent-guard-preflight-result.md)から参照できる。これは固定Codex 0.153.4で観測した形式であり、canonical target指定の受理、名前再利用、全再試行の安全性を実証したという意味ではない。計画では親session内の登録と完全一致照合を使い、異常時は対応不明として保持する。製品としての動作は計画のTDD・限定liveで別途検証する。
