# パッケージ構成とモジュールの役割

## 全体像

実装は「単一の手書きコアを、ハーネスごとの薄い層を通して配布物へ投影する」構造である。

```text
core/ + harness/<name>/ + plugins/
              │
              └─ scripts/package.ts
                         │
                         ├─ dist/claude/
                         ├─ dist/kiro/
                         ├─ dist/kiro-ide/
                         ├─ dist/codex/
                         ├─ dist/cursor/
                         ├─ dist/opencode/
                         └─ dist/copilot/
```

Codex の配置物は、実装内 `dist/codex/` をベースにした実行可能なプロジェクト内配布形式である。

## 実装側のトップレベル責務

| パス | 役割 | 編集方針 |
| --- | --- | --- |
| `core/` | ハーネス非依存の方法論、状態機械、CLI、フック、知識、ルール | 主な手書きソース |
| `harness/<name>/` | 配置先、呼び出し方法、オンボーディング、ハーネス固有生成 | ハーネス固有変更時に編集 |
| `plugins/<name>/` | ステージ、スコープ、エージェント等の追加機構 | オプション機能の所有単位 |
| `scripts/` | パッケージ生成、manifest 契約、ドキュメント変換、CI補助 | ビルド系ソース |
| `dist/<harness>/` | 生成・コミット済み配布物 | 手編集禁止 |
| `tests/` | smoke/unit/integration/e2e、テスト支援、カバレッジ台帳 | 期待挙動の根拠 |
| `docs/` | 利用者ガイド、ハーネス開発ガイド、開発者リファレンス | 設計・操作説明 |

`harness/` には `claude`、`kiro`、`kiro-ide`、`codex`、`cursor`、`opencode`、
`copilot` の7種類がある。各ディレクトリの `manifest.ts` が投影規則を定め、必要なハーネスでは
`emit.ts` が追加の設定やアダプターを生成する。

## `core/` の責務

| サブディレクトリ | 主な内容 | 分析時のファイル数 |
| --- | --- | ---: |
| `agents/` | 14の専門・レビュー・構成エージェントのプロンプト | 14 |
| `aidlc-common/` | オーケストレーター、ステージプロトコル、33ステージ | 42 |
| `hooks/` | セッション、監査、ルール配信、状態遷移等のフック | 18 |
| `knowledge/` | エージェント別・共有の方法論知識 | 59 |
| `memory/` | 組織・チーム・プロジェクト・フェーズ別ルールの初期値 | 8 |
| `scopes/` | enterprise から express 等のスコープ定義 | 11 |
| `sensors/` | 必須節、出典、lint、型検査等の検証manifest | 6 |
| `skills/` | コスト、リプレイ、成果物パック等のセッションスキル | 4 |
| `templates/` | 各配布物へ展開するオンボーディングテンプレート | 1 |
| `tools/` | 状態機械、監査、graph、plugin、knowledge、CLI群 | 54 |

主要な制御は `core/tools/` に集中する。

- `aidlc.ts`: `/aidlc` 相当の入口とサブコマンドのルーティング。
- `aidlc-orchestrate.ts`: `next`、`continue`、`report`、`park`、`team-board` による進行制御。
- `aidlc-state.ts`: 状態の読み書きと遷移。
- `aidlc-graph.ts`: ステージグラフと次ステージの解決。
- `aidlc-audit.ts`: 91種類の監査イベントと追記契約。
- `aidlc-sensor.ts`: センサーの選択と実行。
- `aidlc-swarm.ts`: Construction の並列作業の調停。
- `aidlc-plugin.ts`: プラグインの選択、同期、検証、生成。
- `aidlc-knowledge.ts`: DocumentKB の同期・索引・関連付け。
- `aidlc-lib.ts`: 多数のツールが共有する基盤関数群。

## 方法論の構造

33ステージは5フェーズに分かれる。

| フェーズ | ステージ数 | 性質 |
| --- | ---: | --- |
| initialization | 3 | ワークスペース、意図、初期状態の準備 |
| ideation | 7 | 要求、利用者、価値、機会の整理 |
| inception | 9 | 設計、計画、リスク、実装準備 |
| construction | 7 | 実装、テスト、レビュー、統合 |
| operation | 7 | リリース、配備、運用、観測、改善 |

エージェントは11のドメイン専門家、2つのレビュー専用ロール、1つのadaptive composerで構成される。
スコープ、深度、テスト戦略、プラグイン選択により、実際に有効なステージ集合と詳細度が変わる。
正規の実行時ビューは生成済み `tools/data/stage-graph.json` と `aidlc --doctor` である。

## Codex 配布側の構成

| 配布パス | 役割 | 件数・補足 |
| --- | --- | --- |
| `.agents/skills/` | オーケストレーター、ステージランナー、補助スキル | 42ディレクトリ |
| `.codex/agents/` | Codex が起動するサブエージェント定義 | 14 TOML + 14 Markdown |
| `.codex/tools/` | Bun で動く決定論的CLIと生成済みdata | 69ファイル |
| `.codex/hooks/` | Codex hook adapter と共通フック | 19ファイル |
| `.codex/hooks.json` | Codexイベントからフックへの対応 | 1契約ファイル |
| `.codex/aidlc-common/` | オーケストレーターとステージ定義 | 42ファイル |
| `.codex/knowledge/` | 方法論知識 | 59ファイル |
| `.codex/scopes/` | スコープ定義 | 11ファイル |
| `.codex/sensors/` | センサー定義 | 6ファイル |
| `.codex/rules/` | Codexネイティブのコマンド許可ルール | AIDLC method memoryとは別物 |
| `.codex/config.toml` | モデル、sandbox、機能、agent、環境設定 | 配置版では利用者モデルを継承 |
| `aidlc/` | 実行時ワークスペースのseed | active-space と spaces 初期構造 |
| `AGENTS.md` | Codex向けオンボーディングと運用規約 | プロジェクトへコピー/統合 |
| `.gitignore` | 実行時一時データと個人カーソルの除外 | 共有記録は追跡する設計 |

## 実装から配布への対応

| 実装側 | Codex配布側 |
| --- | --- |
| `core/tools/` | `.codex/tools/` |
| `core/hooks/` | `.codex/hooks/` |
| `core/aidlc-common/` | `.codex/aidlc-common/` |
| `core/agents/*.md` | `.codex/agents/*.md` と生成TOML |
| `core/knowledge/` | `.codex/knowledge/` |
| `core/scopes/` | `.codex/scopes/` |
| `core/sensors/` | `.codex/sensors/` |
| `core/skills/` と `harness/codex/skills/` | `.agents/skills/` |
| `core/templates/onboarding.md` + Codex fill | `AGENTS.md` |
| compiled graph/scope/model data | `.codex/tools/data/*.json` |

## 配布物で保持するデータ

配布物はフレームワーク本体と、利用プロジェクト側に作る `aidlc/` ワークスペースを分離する。

- `aidlc/spaces/<space>/memory/`: org → team → project → phase → stage の加算的ルール。
- `aidlc/spaces/<space>/intents/`: 意図ごとの状態、監査、成果物、実行時記録。
- `aidlc/spaces/<space>/knowledge/`: チーム所有知識と DocumentKB。
- `aidlc/spaces/<space>/codekb/`: コードベース知識。
- `active-space` と `active-intent`: 利用者/cloneごとの現在位置。

共有可能な状態・監査・知識はGit追跡し、個人カーソル、clone id、センサーキャッシュ、
セッション一時領域等だけを `.gitignore` する方針である。

## 主要根拠

- `docs/実装_aidlc-workflows/AGENTS.md`
- `docs/実装_aidlc-workflows/README.md`
- `docs/実装_aidlc-workflows/core/`
- `docs/実装_aidlc-workflows/harness/*/manifest.ts`
- `docs/実装_aidlc-workflows/scripts/package.ts`
- `docs/配布_ai-dlc/AGENTS.md`
- `docs/配布_ai-dlc/.codex/tools/data/stage-graph.json`
