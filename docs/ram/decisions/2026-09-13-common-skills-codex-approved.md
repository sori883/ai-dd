# 共通手順とCodex設定を分離しClaude未完了作業を破棄する

2026-09-13。ユーザーは、共通手順を一か所に置き、ハーネスには環境固有の違いを置く具体計画へ「はい、じゃあそれで、うんうん、まあお願いします。」と回答した。これは[実装計画](../../design/common-skills-codex-plan.md)全体への直接承認である。旧33 Stageや3環境対応の包括承認を流用しない。

## 採用する構成

共通の15 skillを `src/core/skills/`、5担当の役割本文を `src/core/agents/` にまとめる。承認・単独writer・入力等の共通部品を一か所に置き、必須境界は生成物へ含める。長い方法論はskillを必要時に読む。`src/harness/codex/` はCodex固有のtool・event・権限・形式・配置処理を持ち、既存のManifestを使って配置する。原稿・参照資料・ライセンスを移す。生成済み本文を第二の正本にしない。

Go単一バイナリ、外部module数、CLI名、配置先、state、Sensor、承認・担当管理を維持する。配置済み工程MDはbytesとdefinition hashを維持する。通常のソース整理であり、本家2.6.123の参照済み範囲へ新しい製品挙動差分を加えない。未確認のupstream全体との一致を主張しない。

## Claude作業の置換と削除範囲

[Claude未完了保留](2026-09-13-claude-adapter-paused-incomplete.md)の「実装を保持して再開する」を置換する。Issue #185「CodexとClaude Codeを環境別アダプターとして新規配置する」、local branch `codex/claude-adapter`、専用worktree `/Users/const/sori883/ai-dd-naming` の削除を承認済み。調査時点で同branchのremoteとPRはない。D1の共通Manifestはmainへmerge済みなので保持し、未commitのD2コード・runtime schema変更を今回へ持ち込まない。

削除前に未保存だった15件のRAMと[Claude接続計画](../../design/claude-code-connection-plan.md)を内容を変えず回収し、索引へ追加した。これらは当時の判断・実測の履歴である。Claudeの部分成功やCredit balance too lowを全機能の成功・無料プランの制限と説明しない。Claude再開はCodex確認後の新計画で決める。Copilot保留・fresh配置・単一CLI配布の関連判断も回収した。

READMEのAI-DD見出しと既存RAM、他のworktreeを保持する。README見出し修正だけの「Issue不要」はその修正に適用する。今回の共通化はリポジトリの通常Issue／PR運用で管理する。新規Release・tag公開はこの承認に含めない。

## 検証と完了

単独実装担当が順序付きTDDを行い、独立review後にread-only finalを行う。配置・relocate・hook読取り・梱包・既存工程の回帰と固定Codexの実読込みを確認する。自動fixtureのtrustと通常trustを区別し、未実施・skipを成功にしない。実装結果・削除結果・検証証拠は本記録の後続へ追記する。
