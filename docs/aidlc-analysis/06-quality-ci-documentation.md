# コード品質指標

## 総評

生成物parity、strictな型検査、決定論的deep test、semantic coverage ratchet、secret/SAST/audit、
strictなドキュメントbuildまで複数層の品質ゲートがある。特に「生成済み配布物もコミットし、
手書きソースからbyte単位で再現できること」をCIで保証する点が強い。

一方、一般的な行・分岐カバレッジ、未使用コード検査、全Markdownへのlint、formatter統一、
live-model testの常設CIは確認できない。強い独自契約検査と、一般的な静的指標の空白が同居する。

## lintと型検査

### TypeScript

- 3つの `tsconfig` が手書き実装、テスト、生成adapterを分けて `strict: true` で検査する。
- `noEmit` により型検査だけを行い、配布生成は独自packagerへ一本化する。
- 壊れていること自体を試すfixtureだけを明示除外する。

### Biome

`biome check --error-on-warnings core harness scripts plugins tests` をCIで実行する。

- linterは有効、formatterは無効。
- import自動整理は無効。
- `dist/` は生成物なので対象外。
- testとtool系ではnon-null assertion等の一部規則を緩和。
- `aidlc-knowledge.ts` は、ファイル変更をatomic wrapperへ集約するため `node:fs` の直接mutationを禁止。

warningも失敗扱いにする一方、整形とimport順は品質ゲートに含めない設計である。

### Knip

`knip.json` はentry、project、fixture例外を定義するが、`knip` packageは `package.json` に宣言されず、
scriptやGitHub Actionsからの実行も確認できない。設定は存在するが、現状の必須ゲートではない。

## CI/CD

| workflow | trigger | 主なgate |
| --- | --- | --- |
| `ci.yml` | `v2` 向けPR、手動 | dist parity、型、lint、smoke/unit、決定論的integration/e2e、CHANGELOG |
| `security-scanners.yml` | `v2` のPR/push、手動 | Gitleaks、Semgrep、Bun audit |
| `markdownlint.yml` | `v2` のPR/push、手動 | rootの主要Markdown |
| `docs.yml` | docs関連pathのPR/push、手動 | link変換、Zensical strict build、Pages deploy |

GitHub Actions参照はcommit SHAで固定され、Bun、uv、Gitleaks、Semgrepも明示版を使う。
同一refの古いrunをcancelするconcurrency設定がある。production docs deployだけは直列化し、途中cancelしない。

### 配布物parity

`bun scripts/package.ts --check` が全ハーネスの配布物を一時生成し、コミット済み `dist/` と比較する。
これにより次を検出する。

- `dist/` の手編集。
- 手書きソース変更後の再生成漏れ。
- stage graph、scope、runner、agent設定等の生成drift。
- harness manifestと配布形状の不一致。

### CHANGELOG gate

利用者向け変更では製品版、README badge、CHANGELOGを同期する。unit testが同一性と重複見出しを検査し、
PR workflowはbase commitに存在したversion見出しの削除も防ぐ。純粋な文書、内部refactor、testのみの
変更はversion bump対象外と明記される。

## セキュリティ品質

### Gitleaks

- `8.30.1` のarchiveをchecksum検証して導入。
- shallow checkoutではなく全Git履歴をscan。
- 独自設定とbaselineを使用。
- SARIFをartifact保存し、GitHub code scanningへupload。

### Semgrep

- `1.157.0` を固定。
- `core harness scripts plugins` の手書きTypeScriptを対象。
- OSS ruleset `p/typescript` のERROR severityをblocking。
- SARIFの形式自体も検査する。

### dependency audit

- `bun audit --json` を実行しartifactを保存。
- JSON形式不正やscan失敗を検出。
- critical advisoryはblocking。
- Claude Agent SDK依存グラフの既知high severity advisoryは可視化するがblockingしない。

## ドキュメント品質

文書は読者別に分かれる。

| 場所 | 読者・目的 |
| --- | --- |
| `README.md` | 製品概要、対応ハーネス、quick start |
| `docs/guide/` | 利用者向けworkflow、scope、agent、操作、troubleshooting |
| `docs/harness-engineering/` | 設定拡張、plugin、別ハーネスへのport |
| `docs/reference/` | architecture、protocol、hooks、testing、contributing |
| `assets/*Specification.pdf` | AI-DLC 2.0の方法論基準 |
| `CHANGELOG.md` | 利用者向け変更履歴 |

Zensical siteは `--strict` でbuildし、repository外相当の相対リンクをGitHub blob URLへ変換するscriptが
リンク先の存在も検査する。ファイル・command・flagの追加、削除、改名時は `docs/` とREADMEの
古い参照を同一commitで更新するpolicyがある。

markdownlint workflowの対象は `AGENTS.md`、`CLAUDE.md`、`CODE_OF_CONDUCT.md`、
`CONTRIBUTING.md`、`README.md` のroot文書だけで、`docs/**/*.md`、`core/**/*.md`、
配布物Markdownは直接のmarkdownlint対象ではない。docs site buildは別途行われるが、対象と規則は同一ではない。

## 監査可能性と運用品質

- 91種類の監査イベントをclone別shardへ追記し、並列作業後にmergeできる。
- session、human turn、tool write、state transition、approval、reviewをhookで記録・保護する。
- blocking sensorのoverrideは明示的に監査される。
- stateとartifactをGit管理し、再開と判断根拠を残す。
- model token/costを決定論的runtime summaryから集計する補助skillがある。
- generated dataとrunnerにdrift guardがある。

これらはソースコードの静的品質とは別に、AI agentが行う作業プロセスの再現性と説明可能性を高める。

## 品質指標のまとめ

| 指標 | 状態 |
| --- | --- |
| 配布再現性 | byte parity gateあり |
| 型安全性 | TypeScript strict、3境界で検査 |
| lint | Biome warningをblocking |
| formatter | 無効 |
| 自動test | 468 test files、4level |
| semantic coverage | 817面を列挙、520 covered、class別ratchet |
| 行/分岐coverage | gate未確認 |
| secret scan | 全履歴Gitleaks |
| SAST | Semgrep ERROR rules |
| dependency audit | criticalのみblocking |
| doc build | Zensical strict |
| Markdown lint | root主要文書のみ |
| unused code | Knip設定はあるが必須実行なし |
| release traceability | version/README/CHANGELOG同期gate |

## 主要根拠

- `docs/実装_aidlc-workflows/package.json`
- `docs/実装_aidlc-workflows/biome.json`
- `docs/実装_aidlc-workflows/knip.json`
- `docs/実装_aidlc-workflows/.github/workflows/ci.yml`
- `docs/実装_aidlc-workflows/.github/workflows/security-scanners.yml`
- `docs/実装_aidlc-workflows/.github/workflows/markdownlint.yml`
- `docs/実装_aidlc-workflows/.github/workflows/docs.yml`
- `docs/実装_aidlc-workflows/AGENTS.md`
- `docs/実装_aidlc-workflows/tests/.coverage-registry.json`
