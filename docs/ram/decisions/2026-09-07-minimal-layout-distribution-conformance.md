# Space内の配置案を採用し、配布の仕組みはAI-DLCに準拠する

2026-09-08追記: [Goシングルバイナリ継続](2026-09-08-single-binary-and-conformance-questions.md)の再指定を優先する。
下記の本家runtime treeの説明は参照事実であり、Go版で別runtime配布を必須とする合意ではない。

- 日付: 2026-09-07
- 状態: Accepted（提示した配置案と配布方針へのユーザー直接回答）

## 合意

Spaceの `aidlc/spaces/<space>/knowledge/` 配下に `knowledge/`、`design/`、`kdr/`、`rules/` と
index.md/log.mdを置き、KDR・RuleもOKF文書として検索・表示・検証対象にする案に対して、
ユーザーは「はい、それでいいです」と回答した。
続けて「配布の仕組みはAI-DLCに準拠するので、その点も考慮しておいてください」と指定した。

[前回の配置決定](2026-09-07-space-knowledge-okf-unification.md)で提案扱いだった下位分類を採用する。
同じIntentでは同じKDRを更新する。型名、CLI文法、保存処理、OKFの外部CLI利用かGo組込みかは、
この回答だけで確定したものとは扱わない。M1の実装開始も未指示である。

## 配布設計への反映

参照基準はリポジトリ固定AI-DLC `2.6.123`。今回も参照元のversion定数と分析索引の一致を確認した。
最新upstream全体への一致は主張しない。

確認した本家の仕組みは、共通coreとハーネス別資産から配布treeを生成し、利用先へ配置するもの。
Codexでは `.codex/`、`.agents/`、`aidlc/` と入口文書等を配置対象とする。
release用CLIも隣接runtime treeを使うため、厳密な単一ファイル配布だけを前提にしない。

新方式では次を配布計画に含める。

- 手書きの製品資産をsrcに保持し、ハーネス別の生成・配置対応を明示する。生成物を正本として手編集しない。
- CLI、hook、最小skill・agent、KDRテンプレート、OKF初期Ruleを配布構成として一緒に検証する。
- 初期Ruleは対象Spaceのknowledge/rulesへOKF文書として配置し、利用中のKnowledge・KDR・Ruleと配布元原稿を区別する。
- 配布更新で利用者のKDR・知識・編集済みRuleを無条件に上書きしない。既存設定との統合方法を配置計画へ含める。
- M1のfresh sandboxでも本家に沿った配置先から起動する。正式配布の生成再現性、初回導入、更新、戻し方はM3の対象として維持する。

本家の配布手順に含まれる旧Stage/state/auditの全資産を新製品へ復活させる意味ではない。
Goで実装する制約も維持する。新しい配布方式を独立に作る前に、本家の生成・配置・更新境界との対応を計画へ示す。
OKF実行ファイルを別途取得する先行案は配布契約との整合が必要であり、採用済みの利用者向け導入手順とは扱わない。

## 根拠と許可の範囲

- [固定配布形式の調査](../research/2026-08-29-existing-distribution-format.md)
- [配布・ビルド分析](../../aidlc-analysis/02-build-config-dependencies.md)
- [本家version定数](../../実装_aidlc-workflows/core/tools/aidlc-version.ts)
- [更新するM0設計](../../design/minimal-product-contract-m0.md)

今回の回答は配置案の採用と配布方針の指定。M0のその他の未確定契約やM1全体への包括承認に拡張しない。
今回行うのは設計・RAM・索引への反映であり、製品コード・設定・Issue・PR・既存dataは変更しない。
