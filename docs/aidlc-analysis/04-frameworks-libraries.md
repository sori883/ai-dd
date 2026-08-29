# フレームワーク、ライブラリとそのバージョン

## バージョンの読み方

このスナップショットには3種類の版がある。

| 種類 | 値 | 意味 |
| --- | --- | --- |
| AI-DLC製品版 | `2.6.123` | 利用者が参照するフレームワーク版 |
| 開発用npm package版 | `0.0.0` | private tooling用で、製品版ではない |
| 仕様世代 | `2.0` / v2 | 方法論とGit branchの系列 |

製品版は `core/tools/aidlc-version.ts`、README badge、CHANGELOGの最新見出しで同期され、
テスト `tests/unit/t68-version-changelog-sync.test.ts` が整合性を検査する。

## 実行・開発基盤

| 技術 | バージョン/制約 | 使用箇所 |
| --- | --- | --- |
| Bun | CIは `1.3.14` 固定 | packager、CLI、hooks、tests、TypeScript直接実行 |
| TypeScript | `^6.0.3`、lockは `6.0.3` | 全決定論的ツールとテスト |
| ECMAScript modules | `package.json` の `type: module` | モジュール形式 |
| Node互換API | Bunが提供 | `node:fs`、process、child process等 |
| Python | `>=3.12`、CIは `3.12` | ドキュメントサイトのみ |
| uv | CIは `0.11.28` | Python依存のlock/install |
| Zensical | `0.0.51` 固定 | ドキュメント生成 |

配布物のruntime前提はBunだが、配置物自体にBunのlockやinstallerは含まれない。
そのため実行再現性は利用者環境のBun版に依存し、CIの `1.3.14` が検証済み基準になる。

## JavaScript/TypeScriptライブラリ

| ライブラリ | 宣言 | lock値 | 分類 |
| --- | --- | --- | --- |
| `@anthropic-ai/claude-agent-sdk` | `0.3.158` | `0.3.158` | Claude harness/live integration test |
| `@biomejs/biome` | `2.4.16` | `2.4.16` | 静的解析 |
| `@xterm/headless` | `^5.5.0` | `5.5.0` | TUI/terminal test |
| `bun-types` | `^1.3.13` | `1.3.14` | 開発時型定義 |
| `node-pty` | `1.1.0` | `1.1.0` | pseudo terminal test |
| `smol-toml` | `1.7.0` | `1.7.0` | TOML処理 |
| `typescript` | `^6.0.3` | `6.0.3` | compiler/type checker |

`@anthropic-ai/claude-agent-sdk` は `@anthropic-ai/sdk >=0.93`、MCP SDK `^1.29`、
Zod `^4` 等を依存グラフへ持つ。security workflowには、このグラフに既知のhigh severityな
transitive advisoryがあることが明記されている。

## 品質・セキュリティツール

| ツール | バージョン | 実行位置 |
| --- | --- | --- |
| Biome | `2.4.16` | `bun run lint` / CI |
| markdownlint-cli2 action | `v22.0.0` | root文書のCI |
| Gitleaks | `8.30.1` | 全Git履歴のsecret scan |
| Semgrep | `1.157.0` | 手書きTypeScriptのERRORルール |
| Bun audit | Bun `1.3.14` 同梱 | lock依存の脆弱性監査 |
| Knip | schemaはv6 | 設定のみ。依存宣言・script・CI実行は未確認 |

GitHub Actionsはtagだけでなくcommit SHAにpinされている。コメントに対応するaction release版も
記録されている。

## 対応ハーネスと最低版

ローカルREADME/オンボーディングが明示する主な条件は次のとおり。

| ハーネス | 最低版または記載 |
| --- | --- |
| Codex CLI | `>= 0.145.0` |
| Kiro CLI | `>= 2.6` |
| GitHub Copilot CLI | `>= 1.0.74` |
| GitHub Copilot VS Code | `>= 1.130` |
| opencode | `>= 1.17` |
| Claude Code | このスナップショットの一覧に数値の最低版なし |
| Kiro IDE | このスナップショットの一覧に数値の最低版なし |
| Cursor | このスナップショットの一覧に数値の最低版なし |

Codex `0.145.0` の下限理由はcompact後のSessionStart復元であり、`0.139.0` より前には
subagent role attributionとhyphenated agent TOML解決にも制限がある、と配布 `AGENTS.md` に記載される。

## モデルとprovider

モデルはアプリ依存ライブラリではなく、ハーネス設定の一部である。

- 正規Codex生成物: session/judgment tierに `openai.gpt-5.5`、balanced/templated agentの一部に
  `openai.gpt-5.6-terra`、providerはAmazon Bedrock。
- 配置されたCodex配布物: project設定のprovider/model指定をコメントアウトし、利用者のCodex設定を継承。
- 配置版の5 agent TOML: `gpt-5.6-terra` を直接指定。
- READMEのKiro向け推奨: Claude Opus 4.8。

モデルIDはprovider固有の名前空間差があるため、文字列一致だけで互換性を判断しない。
将来の参照スキルは「正規生成値」「配置時のoverlay」「実際の利用者設定」を分けて回答すべきである。

## 標準ライブラリ優先の傾向

runtime CLIは外部runtime packageを同梱せず、Bun/Node互換のファイル、process、crypto、
child process、fetch APIを中心に実装される。外部依存は主に開発・lint・live test用途に限定される。
一方、単一の大きな内部共通モジュールへ機能が集中しており、外部依存の少なさが内部結合の少なさを
意味するわけではない。

## 主要根拠

- `docs/実装_aidlc-workflows/package.json`
- `docs/実装_aidlc-workflows/bun.lock`
- `docs/実装_aidlc-workflows/core/tools/aidlc-version.ts`
- `docs/実装_aidlc-workflows/README.md`
- `docs/実装_aidlc-workflows/pyproject.toml`
- `docs/実装_aidlc-workflows/uv.lock`
- `docs/実装_aidlc-workflows/.github/workflows/*.yml`
- `docs/配布_ai-dlc/AGENTS.md`
- `docs/配布_ai-dlc/.codex/config.toml`
- `docs/配布_ai-dlc/.codex/agents/*.toml`
