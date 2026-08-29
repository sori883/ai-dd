# テストディレクトリ、テストフレームワーク、カバレッジ設定

## テスト構成

テストはすべてTypeScriptで、Bunの `bun:test` を利用する。分析時点の `*.test.ts` は468ファイル。

| 場所 | ファイル数 | 主な責務 |
| --- | ---: | --- |
| `tests/smoke/` | 14 | 配布形状、最低限の起動、重要契約の短時間確認 |
| `tests/unit/` | 259 | parser、schema、状態遷移、個別toolの決定論的検査 |
| `tests/integration/` | 114 | 複数tool、生成物、監査、plugin等の連携 |
| `tests/e2e/` | 78 | journey、TUI、live harnessを含む外側からの検証 |
| `tests/harness/` と `tests/lib/` | 3 | test harness自体の校正・支援 |

`bun:test` のimportは約474箇所ある。テストは手書き `core/` だけでなく生成済み配布物も対象にし、
特にClaude配布の `aidlc-lib.ts` を参照するtest importが多数ある。これによりpackaging後の挙動も
検証対象になる。

## テストランナー

正規入口は `bun tests/run-tests.ts`。`tests/run-tests.sh` はPOSIX互換wrapperとして残る。

| profile/指定 | 実行範囲 |
| --- | --- |
| 既定 | smoke + unit + integration |
| `--ci` | credential不要のCI向け構成 |
| `--release` / `--all` | e2eを含む全level |
| `--no-llm` | 全live-model gateを明示的に閉じて決定論的subsetを実行 |
| `--filter` | test file名で絞り込み |
| `--parallel N` | 大きいlevelの並列数を指定 |
| `--debug` | 詳細traceを保存・表示 |

smokeとunitは原則serial、integrationとe2eは並列化できる。runnerは失敗したtest fileを集約し、
JUnit XMLと実行metadataを生成できる。

## CIでのテスト

`.github/workflows/ci.yml` はPR先が `v2` の場合に次を実行する。

1. `bun install --frozen-lockfile`。
2. `bun run check` によるdist byte parity、3系統の型検査、Biome lint。
3. `bun tests/run-tests.ts --smoke --unit --parallel 8`。
4. `bun tests/run-tests.ts --integration --e2e --no-llm --parallel 8`。
5. CHANGELOGの既存version見出しを削除していないか検査。

deep test jobは90分timeout。`--no-llm` はcredentialがないため偶然skipする方式ではなく、
Claude SDK、TUI、Kiro、Codex exec等のlive gateを明示的に閉じ、残った決定論的journeyを実行する。

## semantic coverage registry

このリポジトリの主要なカバレッジ指標は行・分岐率ではなく、公開面やworkflow面を列挙した
独自のL-SURFACE semantic coverageである。

- 生成器: `tests/gen-coverage-registry.ts`
- 台帳: `tests/.coverage-registry.json`
- 低下防止: `tests/.coverage-ratchet.json`
- test側の宣言: 各ファイルの `covers:` header
- 検査: `bun tests/gen-coverage-registry.ts --check`

分析時点の値は次のとおり。

| unit class | 列挙数 | covered | 率 |
| --- | ---: | ---: | ---: |
| function | 519 | 299 | 57.6% |
| audit | 91 | 49 | 53.8% |
| scope | 11 | 11 | 100.0% |
| stage | 33 | 11 | 33.3% |
| hook | 18 | 18 | 100.0% |
| subcommand | 138 | 125 | 90.6% |
| render-surface | 7 | 7 | 100.0% |
| 合計 | 817 | 520 | 63.6% |

合計率は単純合算であり、classごとに要求する最低mechanismが違うため、単一の品質スコアとしては
扱わない。ratchetは各classのcovered件数が既存値から無断で低下することを防ぐ。
実台帳とのdrift検査はunit testにも含まれるため、CIのunit jobから間接的に実行される。

## 一般的なコードカバレッジ

`bun test --coverage`、LCOV、Istanbul/c8等の行・分岐カバレッジ設定やCI gateは確認できなかった。
したがって次はsemantic registryだけでは判断できない。

- 条件分岐の未実行経路。
- 例外処理やplatform別分岐の実行率。
- 大きなmodule内で、列挙対象になっていない内部処理の実行率。
- 変更行に対する差分カバレッジ。

## skipとlive test

`test.skip` 系のテキスト出現は76件ある。ただし多くはOS、TTY、credential、live model等の条件付きで、
未実装テストが76件あるという意味ではない。CIはlive familyを `--no-llm` で除外し、これらを
local pre-mergeまたはrelease concernとしている。

この方針はCIの決定性を高める一方、実モデル、provider、Codex/Kiro/Claudeの実行環境との互換性は
別の検証実績に依存する。

## fixtureとテスト自己検証

- `tests/fixtures/` に成功・失敗・破損・遅延等の意図的fixtureがある。
- 一部の壊れたTypeScript fixtureは通常の型検査から明示除外される。
- coverage generator自体にunit testがあり、実台帳のstalenessも検査する。
- test suite drift testが古いpathやlegacy package名を検出する。
- version/changelog同期、stage-runner生成、plugin compose、配布parityもテスト対象。

## 主要根拠

- `docs/実装_aidlc-workflows/tests/README.md`
- `docs/実装_aidlc-workflows/tests/run-tests.ts`
- `docs/実装_aidlc-workflows/tests/gen-coverage-registry.ts`
- `docs/実装_aidlc-workflows/tests/.coverage-registry.json`
- `docs/実装_aidlc-workflows/tests/.coverage-ratchet.json`
- `docs/実装_aidlc-workflows/tsconfig.tests.json`
- `docs/実装_aidlc-workflows/.github/workflows/ci.yml`
- `docs/実装_aidlc-workflows/docs/reference/09-testing.md`
