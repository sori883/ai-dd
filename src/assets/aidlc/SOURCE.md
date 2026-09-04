# 出典とコピー範囲

## 取得した内容

- 配置日: 2026-09-04
- プロジェクト: AWS Labs AI-DLC Workflows
- 上流の案内: [awslabs/aidlc-workflows の v2 系列](https://github.com/awslabs/aidlc-workflows/tree/v2)
- 直接のコピー元: 本リポジトリ内の `docs/実装_aidlc-workflows/`
- 製品版の確認箇所: `core/tools/aidlc-version.ts` の `AIDLC_VERSION = "2.6.123"`
- version定義ファイルのSHA-256: `43eb223d4e39adad078c8cf0f413b0dd5701d1f757eb6b173bbacfbb2a50b358`
- upstream commit: **未確認**。ローカル参照snapshotに `.git` はなく、特定のcommitやtagを推定しない。
- ライセンス: **MIT No Attribution**。コピー元rootの [LICENSE](LICENSE) を無変更で同梱。
- LICENSEのSHA-256: `7cb750713252efd1d578837ba8785b61319109906f0c10b87adab7cf4badfc42`

この記録は「手元の固定snapshotからコピーした内容」を特定します。
上流URLは案内用の可変branchであり、現在の最新upstreamとコピーの同一性を保証するリンクではありません。
版番号だけで内容の完全一致は判断できないため、個々のファイルのhashを残します。

## コピー元と配置先の対応

`src/assets/aidlc/` を配置先rootとして、次の相対構成をそのまま保ちました。

| ローカル参照snapshot内 | 配置先root内 | ファイル数 |
| --- | --- | ---: |
| `core/aidlc-common/` | `aidlc-common/` | 42 |
| `core/knowledge/` | `knowledge/` | 59 |
| `core/memory/` | `memory/` | 8 |
| `core/agents/` | `agents/` | 14 |
| `core/scopes/` | `scopes/` | 11 |
| `core/sensors/` | `sensors/` | 6 |
| `LICENSE` | `LICENSE` | 1（別枠） |

140ファイルのうち139件はMarkdown、1件は説明コメント入りの `memory/templates/.gitkeep` です。
本文・frontmatter・コメント・空白・改行を翻訳、正規化、展開せず保存しています。
TypeScriptツール、hook、生成済みデータ、harness設定、配布物全体を取り込むものではありません。

## このリポジトリで追加した案内

以下の4ファイルは本家からのコピーではありません。

- [README.md](README.md)：目的、各群の役割、利用上の境界。
- [INVENTORY.md](INVENTORY.md)：コピー全140件へのリンクと原本のbyte数、別枠のLICENSE。
- [SOURCE.md](SOURCE.md)：この出典記録。
- [SHA256SUMS](SHA256SUMS)：原本から計算した140ファイルとLICENSEのSHA-256。

SHA256SUMS自体の配置時SHA-256は
`2f5886404d856bbf9a54118871715c5223e747bbe8bde922a3f7badde37d196d` です。
これは固定したコピー集合の照合値であり、暗号署名や上流の認証ではありません。

## 更新時の扱い

この領域を本番の読み込み元やOKF Bundleとして採用したという決定はありません。
将来内容を変更・変換するときは、原本との関係、対象範囲、出典・一覧・hashの更新を
同じ変更記録で追跡してください。変更済みの本文を「本家の無変更コピー」と表示し続けないでください。
配置方式・OKF対象・AIへの配信は別の承認済み計画で決めます。
