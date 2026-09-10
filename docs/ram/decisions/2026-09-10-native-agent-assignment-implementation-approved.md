# 担当・作業場所管理の具体計画と通常解放・復旧条件を承認

状態: Accepted（具体計画全体の直接実装承認）。2026-09-10。

## ユーザー確認と結論

親は[具体計画](../../design/native-agent-assignment-plan.md)を提示し、通常の作業終了はメインAIが担当と既知の処理の終了を確認してCLIで予約を解放し、不明なら保持して人間へ確認する案（Q1）と、初回管理開始・復元不能な管理記録の作り直しは人間が残作業を確認する案（Q2）を示した。「この運用を含む計画で、実装を進めてよいですか」と確認したところ、ユーザーは「はい、えっと実装お願いしてもいいですか」と回答した。

Q1・Q2を推奨どおり採用し、同計画全体の公開CLI、ローカルruntime、Unit連携、native hook、配布・復旧・検証を実装する直接承認とする。[A案の方針承認](2026-09-10-managed-worker-assignment-approved.md)の確認待ちを本記録で解消する。過去の記録は当時の状態として保持する。

メインAIが標準ツールで直接エージェントを起動する。CLIは起動せず、管理元内での作業場所の二重割当防止を担う。結果提出・SubagentStop・時間経過だけで予約を解放しない。実childの全稼働・全process停止・hook故障時の完全遮断を保証しない。

1調整rootの全Space/Intent/session、別worktreeでの並列実装、Unitなし割当、Goシングルバイナリ、外部Go moduleなしを維持する。旧Unitへの予約後付け移行・新旧二重運用は追加せず、既存dataは保全する。固定本家2.6.123に対する追加の担当検査・横断予約等の差分は具体計画の提示内容を承認対象とする。

## 実装と品質gate

親が1 Issue/PRを管理し、`work_unit_id=native-agent-assignments` を単独Go実装担当へ渡す。順序付きtest-first loop、独立review、固定差分のread-only final、PRで起動するchecksの成功を確認して自律mergeする。manual mergeの指定はない。

新しい重大な仕様選択や、計画に定めたnative task照合が実機で成立しない場合は、保証を黙って変更せず結果を提示する。合意済みの通常詳細・明らかなbug・review findingは、この範囲で修正する。
