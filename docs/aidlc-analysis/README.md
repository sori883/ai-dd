# AI-DLC v2 技術分析インデックス

## 目的

このディレクトリは、AI-DLC v2 の「実装リポジトリ」と「Codex 向け配布物」を、
コードベース全体を毎回コンテキストへ投入せずに参照するための索引である。
最初に該当する観点の文書を読み、詳細な確認が必要な場合だけ根拠ファイルへ進む。

分析日は 2026-08-29、ローカルスナップショットの製品バージョンは `2.6.123`。
上流は [awslabs/aidlc-workflows の v2 系列](https://github.com/awslabs/aidlc-workflows/tree/v2)、
方法論の基準文書は [AI-DLC Workflows 2.0 Specification](https://github.com/awslabs/aidlc-workflows/blob/v2/assets/AI-DLC-Workflows-2.0-Specification.pdf) である。
ただし、本分析の事実認定は外部の最新状態ではなく、次のローカルスナップショットを基準にする。

- 実装: `docs/実装_aidlc-workflows/`
- 配布: `docs/配布_ai-dlc/`
- 実装内の正規 Codex 生成物: `docs/実装_aidlc-workflows/dist/codex/`

## 結論

AI-DLC v2 は、33 ステージ、14 エージェント、状態・監査・センサー・承認ゲートを持つ
AI 開発ライフサイクルを、単一の harness-neutral な `core/` から7種類のCLIハーネスへ生成する
TypeScript/Bun 製フレームワークである。配布物は単なる文書セットではなく、Codex のスキル、
エージェント設定、フック、決定論的CLI、ルール、知識、実行時ワークスペースの初期構造を含む。

実装側の `dist/codex/` と配置された配布物は各333ファイルで、差分は6ファイルだけだった。
5つはエージェントTOMLのモデル名、1つは `.codex/config.toml` のモデルプロバイダー設定であり、
配置版は Bedrock 固定値をコメントアウトして利用者設定を継承する形へ調整されている。
したがって「実装に含まれる正規生成物」と「実際に参照する配布物」は区別して扱う必要がある。

## 読み分け

| 知りたいこと | 最初に読む文書 |
| --- | --- |
| 全体構造、責務、実装から配布への対応 | [01-package-architecture.md](01-package-architecture.md) |
| 生成方法、設定、依存関係、再現性 | [02-build-config-dependencies.md](02-build-config-dependencies.md) |
| CLI、フック、JSON/Markdown 契約、内部API | [03-apis-contracts.md](03-apis-contracts.md) |
| Bun、TypeScript、Biome、各ハーネス等のバージョン | [04-frameworks-libraries.md](04-frameworks-libraries.md) |
| テスト階層、実行方法、カバレッジ | [05-testing-coverage.md](05-testing-coverage.md) |
| lint、CI/CD、セキュリティ、ドキュメント品質 | [06-quality-ci-documentation.md](06-quality-ci-documentation.md) |
| 優先度付きの技術負債候補 | [07-technical-debt.md](07-technical-debt.md) |

## 参照の優先順位

矛盾がある場合は、用途に応じて次の順序で判断する。

1. 実行時の挙動を知りたい場合は `docs/配布_ai-dlc/`。
2. 設計意図や変更箇所を知りたい場合は `core/`、`harness/`、`plugins/`、`scripts/`。
3. Codex へ本来生成される内容を知りたい場合は `dist/codex/`。
4. 期待挙動の裏付けには `tests/` と `docs/reference/`。
5. 方法論の意図には仕様PDFと `docs/guide/`。

生成物と配置物が異なる場合は差分を消さず、「正規生成値」と「ローカル運用値」を併記する。

## スナップショット規模

| 対象 | ファイル数 | 補足 |
| --- | ---: | --- |
| 実装スナップショット | 3,364 | `.git`、`node_modules`、`.DS_Store` を除外 |
| 実装内 `dist/` | 2,201 | 7ハーネスの生成済み配布物を含む |
| 配置された Codex 配布物 | 333 | `.DS_Store` を除外 |
| TypeScript テスト | 468 | `*.test.ts` の実ファイル数 |
| Codex 配布スキル | 42 | `.agents/skills/` の直下ディレクトリ数 |
| Codex 配布エージェント | 14 | `.codex/agents/*.toml` |

ファイル数は分析時点の棚卸し値であり、後続バージョンでは変わり得る。

## 参照スキルからの利用

リポジトリ内の `.agents/skills/aidlc-reference/` に、この索引を利用する
`$aidlc-reference` スキルを配置している。スキルは次の progressive disclosure を採用する。

1. この `README.md` から質問を1〜2個の観点へ分類する。
2. 対応する観点別文書だけを読む。
3. 文書内の「主要根拠」から必要な原典だけを読む。
4. 実行時の質問では配置物、変更設計では実装側を優先する。
5. バージョン差や設定差が関係する場合だけ `dist/codex/` と配置物を比較する。

これにより、巨大な `dist/` や `aidlc-lib.ts` を無条件に読み込まずに済む。

## 分析上の注意

- `package.json` の `0.0.0` は開発用パッケージの版であり、製品版 `2.6.123` とは別物。
- `dist/` は生成・コミットされるが、手編集禁止。正規性は `bun scripts/package.ts --check` が判定する。
- 配置版は正規生成物から6ファイル変更されているため、そのままでは上記 parity check の対象外になる。
- カバレッジ値は行・分岐カバレッジではなく、公開面を列挙した独自の semantic coverage である。
- 静的な件数と依存解析はテキストベースの棚卸しであり、動的 import や実行時生成を完全には表さない。

## 管理方針

この分析結果はGitで管理する。一方、容量が大きく更新頻度も異なる2つの参照スナップショットは
`.gitignore` で個別に除外する。更新時は参照スナップショットを差し替え、製品バージョン、件数、
`dist/codex/` との差分、依存バージョン、カバレッジ値を再計測する。
