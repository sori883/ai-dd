# ビルドシステム、設定ファイル、依存関係

## ビルド方式

実装リポジトリは Bun 上で動く独自パッケージャーを採用している。入口は
`scripts/package.ts` で、一般的なbundlerで単一バイナリへまとめるのではなく、手書きソースを
ハーネス固有のディレクトリ構造へコピー・変換・生成する。

おおまかな処理順は次のとおり。

1. `core/` を各 `dist/<harness>/` へコピーし、`{{HARNESS_DIR}}` とルール配置名を変換する。
2. `harness/<name>/manifest.ts` のディレクトリ・ファイル対応を適用する。
3. ステージグラフ、スコープ表、モデル料金等のJSONデータをコンパイルする。
4. 有効ステージから単独実行用runnerを生成する。
5. `emit.ts` があるハーネスでは設定、adapter、agent TOML等を追加生成する。
6. 生成済みの索引表を更新する。

通常生成は `bun scripts/package.ts`、再現性検査は `bun scripts/package.ts --check`。
`--check` は一時ディレクトリへ再生成した結果とコミット済み `dist/` をbyte単位で比較する。

## npm scripts

`package.json` は開発専用のprivate packageで、公開パッケージの生成には使わない。

| script | 内容 |
| --- | --- |
| `bun run typecheck` | 3つのTypeScript設定で `tsc --noEmit` |
| `bun run lint` | `core harness scripts plugins tests` をBiome検査 |
| `bun run check` | 配布物parity → 型検査 → lintを直列実行 |

配布物そのものには `package.json` や `node_modules` は含まれない。TypeScriptのツールとフックを
利用者環境の `bun` で直接実行する。

## TypeScript設定

| ファイル | 対象 | 目的 |
| --- | --- | --- |
| `tsconfig.json` | `core/`、`harness/`、`scripts/`、`plugins/` | 手書き実装のstrict型検査 |
| `tsconfig.tests.json` | テストとplugin test | テストコードの型検査 |
| `tsconfig.adapters.json` | 生成済みadapter | 配布時だけ成立する隣接依存を含めた型検査 |

共通の重要設定は `target: ESNext`、`module: ESNext`、`moduleResolution: Bundler`、
`strict: true`、`noEmit: true`、`resolveJsonModule: true`、`types: ["bun"]`。
意図的に壊したfixtureはテスト用設定から除外される。

## パッケージ依存関係

| 依存 | 指定 | lock解決値 | 用途 |
| --- | --- | --- | --- |
| `@anthropic-ai/claude-agent-sdk` | `0.3.158` | `0.3.158` | Claude/TUI系の統合・live test |
| `@biomejs/biome` | `2.4.16` | `2.4.16` | lint |
| `@xterm/headless` | `^5.5.0` | `5.5.0` | terminal/TUIテスト |
| `bun-types` | `^1.3.13` | `1.3.14` | Bun型定義 |
| `node-pty` | `1.1.0` | `1.1.0` | pseudo terminalテスト |
| `smol-toml` | `1.7.0` | `1.7.0` | TOMLの読み書き |
| `typescript` | `^6.0.3` | `6.0.3` | 型検査 |

すべて `devDependencies` である。配布物の決定論的コアは、これらを利用プロジェクトに
インストールせずにBun標準APIと同梱ソースで動作する設計になっている。

## ドキュメントビルド依存

ドキュメントはアプリ実装とは別にPython系ツールを使う。

- Python: `>=3.12`
- Zensical: `0.0.51` 固定
- uv: CIで `0.11.28` 固定
- 設定: `zensical.toml`
- lock: `uv.lock`

CIでは外部ディレクトリへの相対リンクを `scripts/docs-rewrite-links.ts` でGitHub URLへ変換した後、
`uv run zensical build --strict` を実行する。

## 主な設定ファイル

| ファイル | 役割 |
| --- | --- |
| `package.json` / `bun.lock` | JavaScript開発依存と再現可能な解決値 |
| `tsconfig*.json` | 手書き、テスト、生成adapterの型検査境界 |
| `biome.json` | lint規則、対象、例外 |
| `knip.json` | 未使用ファイル・export等の探索設定 |
| `.markdownlint-cli2.yaml` | Markdown規則 |
| `.gitleaks.toml` / `.gitleaks-baseline.json` | secret scan設定と既知検出のbaseline |
| `pyproject.toml` / `uv.lock` | ドキュメント生成依存 |
| `zensical.toml` | ドキュメントサイト構成 |
| `harness/*/manifest.ts` | 配布先マッピングと生成フック |

## Codex配布設定

### `.codex/config.toml`

主な設定面は次のとおり。

- 1,000,000 tokenのcontext windowと高いreasoning effort。
- `workspace-write` sandboxとnetwork access。
- agent delegationの `max_depth = 1`。
- `request_user_input` を含む機能フラグ。
- 状態表示用TUI設定。
- `AIDLC_RULES_DIR` 等、実行時ルートを結ぶ環境設定。

実装内の正規 `dist/codex/.codex/config.toml` はAmazon Bedrock provider、AWS profile/region、
`openai.gpt-5.5` を有効にする。一方、配置された配布物はモデル、provider、AWS設定を
コメントアウトし、ChatGPT/Codex利用者側の既存設定を継承する。

### `.codex/hooks.json`

Codexのイベントを同梱TypeScript hookへ接続する。詳細は
[03-apis-contracts.md](03-apis-contracts.md) を参照。

### `.codex/rules/default.rules`

AI-DLC自身の `bun .codex/tools/`、`bun .codex/hooks/`、必要なGit操作等に対するCodexの
許可ルールである。`aidlc/spaces/.../memory/` にある方法論ルールとは別の仕組み。

## 実装と配布の再現性

分析時点では、`dist/codex/` と配置物はともに333ファイルで、相違は6ファイル。

- 5つのagent TOMLで、正規生成物の `openai.gpt-5.6-terra` が配置版では
  `gpt-5.6-terra` に変更されている。
- `.codex/config.toml` でBedrock向けのモデル・provider・AWS設定が配置版では無効化されている。

残る327ファイルは内容一致である。この差分はローカル運用には合理的だが、配布物を再生成すると
失われる。長期的には「正規生成物 + 明示的overlay」または「配置用post-process」として管理する
方が再現しやすい。

## 主要根拠

- `docs/実装_aidlc-workflows/package.json`
- `docs/実装_aidlc-workflows/bun.lock`
- `docs/実装_aidlc-workflows/scripts/package.ts`
- `docs/実装_aidlc-workflows/scripts/manifest-types.ts`
- `docs/実装_aidlc-workflows/harness/codex/manifest.ts`
- `docs/実装_aidlc-workflows/harness/codex/emit.ts`
- `docs/実装_aidlc-workflows/tsconfig*.json`
- `docs/実装_aidlc-workflows/pyproject.toml`
- `docs/実装_aidlc-workflows/.github/workflows/docs.yml`
- `docs/配布_ai-dlc/.codex/config.toml`
