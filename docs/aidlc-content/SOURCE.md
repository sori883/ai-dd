# 出典と配置の対応

## 取得した内容

- 初回配置・配置変更日: 2026-09-04
- プロジェクト: AWS Labs AI-DLC Workflows
- 上流の案内: [awslabs/aidlc-workflows の v2 系列](https://github.com/awslabs/aidlc-workflows/tree/v2)
- 直接のコピー元: 本リポジトリ内の `docs/実装_aidlc-workflows/`
- 製品版の確認箇所: `core/tools/aidlc-version.ts` の `AIDLC_VERSION = "2.6.123"`
- version定義ファイルのSHA-256: `43eb223d4e39adad078c8cf0f413b0dd5701d1f757eb6b173bbacfbb2a50b358`
- upstream commit: **未確認**。ローカル参照snapshotに `.git` はなく、特定のcommitやtagを推定しません。
- ライセンス: **MIT No Attribution**。コピー元rootの [LICENSE](LICENSE) を無変更で保持。
- LICENSEのSHA-256: `7cb750713252efd1d578837ba8785b61319109906f0c10b87adab7cf4badfc42`

この記録は「手元の固定snapshotからコピーした内容」を特定します。
上流URLは案内用の可変branchであり、現在の最新upstreamとの一致を保証しません。
LICENSEは指定原稿の出典資料であり、このリポジトリのGoコードのライセンスを変更するものではありません。

## 本家のソースと配置先の対応

本家の開発用 `core/` に対応する場所を `src/core/` とします。
既存の「開発対象はsrc配下」という規則を維持し、指定6群の内部構造は本家と揃えます。

| ローカル参照snapshot内 | このリポジトリ内 | ファイル数 |
| --- | --- | ---: |
| `core/aidlc-common/` | `src/core/aidlc-common/` | 42 |
| `core/knowledge/` | `src/core/knowledge/` | 59 |
| `core/memory/` | `src/core/memory/` | 8 |
| `core/agents/` | `src/core/agents/` | 14 |
| `core/scopes/` | `src/core/scopes/` | 11 |
| `core/sensors/` | `src/core/sensors/` | 6 |
| `LICENSE` | `docs/aidlc-content/LICENSE` | 1（別枠） |

初回配置の `src/assets/aidlc/` から移動したもので、140ファイルとLICENSEの内容は不変です。
140件のうち139件はMarkdown、1件は説明コメント入りの `memory/templates/.gitkeep` です。
本文・frontmatter・コメント・空白・改行を翻訳、正規化、展開せず保存しています。

今回揃えるのは指定6群の開発用ソース配置です。本家リポジトリ全体のコピーではなく、
TypeScriptツール・hook・生成済みデータ・harness設定を含む配布物全体ではありません。
Codexへのインストール先である `.codex/`、`.agents/skills/` 等の構成とは区別します。

## このリポジトリで追加した案内と照合記録

本家のcore構成へ独自文書を混ぜないため、案内は `docs/aidlc-content/` に分離しています。

- [README.md](README.md)：目的、各群の役割、利用上の境界。
- [INVENTORY.md](INVENTORY.md)：全140件へのリンクと原本のbyte数、別枠のLICENSE。
- [SOURCE.md](SOURCE.md)：この出典・配置記録。
- [SHA256SUMS](SHA256SUMS)：140ファイルとLICENSEのSHA-256。

ハッシュ一覧のパスは `docs/aidlc-content/` を基準にしています。
移動に伴い対象パスのみ更新し、各原稿とLICENSEのハッシュ値は初回配置時の値を保っています。
SHA256SUMS自体の配置変更時SHA-256は
`cc4d12f506f8ad8c693e56afb9d062dadc9681798fa7921af180f13c9da1b0c6` です。
これは固定したコピー集合の照合値であり、暗号署名や上流の認証ではありません。

## 更新時の扱い

この領域を本番の読み込み元やOKF Bundleとして採用したという決定はありません。
将来内容を変更・変換するときは、原本との関係、対象範囲、出典・一覧・hashの更新を
同じ変更記録で追跡してください。変更済みの本文を「本家の無変更コピー」と表示し続けないでください。

本家に準拠する配置と原文保持の承認は
[配置変更の記録](../ram/decisions/2026-09-04-aidlc-core-layout.md)にあります。
初回配置の経緯は [旧記録](../ram/decisions/2026-09-04-aidlc-content-baseline.md)に残しています。
OKF適用やAIへの配信は別の承認済み計画で決めます。
