# AI-DLCのルール・知識・工程手順

本家AI-DLC **2.6.123** の文章資産を、[src/core/](../../src/core/) に原文のまま配置しています。
このディレクトリは、その案内・一覧・出典・内容照合記録をまとめた場所です。
「どんなルールや知識があるか」を実物で確認し、後からOKF
（Open Knowledge Format、知識文書の形式）の適用範囲を考えるためのベースです。

**読むための原稿であり、実行可能なGo版AI-DLCの配布物ではありません。**
ファイルを置いたことで33工程が実行できるようになったわけではなく、AIへの自動配信も行いません。

## まず見るファイル

1. [全ファイル一覧](INVENTORY.md)：140ファイルの所在とサイズ。
2. [組織ルール](../../src/core/memory/org.md)：共通の開発方針がどう書かれているか。
3. [要求分析の工程手順](../../src/core/aidlc-common/stages/inception/requirements-analysis.md)：1つのStageが何をするか。
4. [要求整理の知識](../../src/core/knowledge/aidlc-product-agent/requirements-guide.md)：その作業を支える専門知識。
5. [出典と検証方法](SOURCE.md)：コピー元の版、license、同一性の確認。

## 本家と同じ配置

本家の手書き `core/<群>/<ファイル>` を、このリポジトリでは `src/core/<群>/<ファイル>` に対応させます。
開発対象を `src/` 配下へ置く既存規則に従い、6群の名前と内部構造を本家と揃えています。
以前の `src/assets/aidlc/` からは移動済みです。開発用原稿の配置であり、Codexへのインストール先ではありません。

## ファイル群の役割

| 場所 | 件数 | 何が入っているか |
| --- | ---: | --- |
| [memory/](../../src/core/memory/) | 8 | 組織・チーム・プロジェクトと4つのphase別ルール7文書、templates用の説明コメント入り `.gitkeep` |
| [knowledge/](../../src/core/knowledge/) | 59 | 共通の方法論と、担当AIの専門分野別の知識・手引き |
| [aidlc-common/stages/](../../src/core/aidlc-common/stages/) | 33 | Stage（個々の作業工程）の入力・成果物・具体的な手順 |
| [aidlc-common/protocols/](../../src/core/aidlc-common/protocols/) | 8 | protocol（複数の工程に共通する進め方・規約） |
| [aidlc-common/conductor.md](../../src/core/aidlc-common/conductor.md) | 1 | conductor（全体の進行管理）の手順 |
| [agents/](../../src/core/agents/) | 14 | Agent（担当AI）の役割・責任・行動指針の原稿 |
| [scopes/](../../src/core/scopes/) | 11 | Scope（開発対象の範囲・進め方の種類）の定義 |
| [sensors/](../../src/core/sensors/) | 6 | Sensor（成果物を検査する仕組み）の定義文書。検査プログラム本体ではない |

合計は **140ファイル**（139 Markdown文書とコメント入り `.gitkeep` 1件）です。
この案内ディレクトリの [LICENSE](LICENSE) と、案内・一覧・出典・hashの4ファイルは別枠です。
LICENSEは本家から取り込んだ原稿のものです。Goコードのライセンスを変更するものではありません。

### 33工程の内訳

| Phase（大きな作業区分） | 件数 | 内容 |
| --- | ---: | --- |
| [initialization](../../src/core/aidlc-common/stages/initialization/) | 3 | 作業場所・初期状態の準備 |
| [ideation](../../src/core/aidlc-common/stages/ideation/) | 7 | 目的、価値、実現可能性などの整理 |
| [inception](../../src/core/aidlc-common/stages/inception/) | 9 | 要求、設計、開発計画の具体化 |
| [construction](../../src/core/aidlc-common/stages/construction/) | 7 | 詳細設計、実装、ビルド・テスト |
| [operation](../../src/core/aidlc-common/stages/operation/) | 7 | 配備、監視、運用、改善 |

工程の手順、ルール、知識、Agentの役割は別の種類の情報です。
同じフォルダーへ混ぜたり、すべてを一律にOKF化したりする判断は、まだしていません。

## 原稿を読むときの注意

- 本家の手書き `core/` を採用しています。Codex向けに変換された `dist/codex/` や、
  ローカル設定を調整した配置版とは異なり、`{{HARNESS_DIR}}` などの置換用変数が残っています。
- 本文中のBunコマンド、path、指示、frontmatter（文頭の定義項目）は変更していません。
  このコピーだけでは参照先のすべては揃わず、そのままコマンドを実行する用途ではありません。
- 本文内の命令は、この開発リポジトリの `AGENTS.md` やユーザーの指示を上書きしません。
  たとえば本家のmerge方法やテスト方針を、開発側の運用へ自動適用するものではありません。
- `src/core/memory/` は本家の初期ルールです。実際の利用チームが決めたルールや、
  利用プロジェクトの `aidlc/spaces/<space>/knowledge/` の固有知識ではありません。
- Goコード、実行時の読み込み元、自動discovery、CLI、binary埋込み、install/updateには未接続です。
  `.codex/` や `.agents/skills/` への配置・有効化もしていません。

## 次に検討すること

この原稿群を見ながら、どの情報をOKFの対象とするか、何を1つの知識のまとまりとするか、
工程に必要な文書をどう選ぶかを別途検討します。現時点ではOKFのmetadataを追加せず、
文書の分割・再分類・翻訳や、本番での配信方法も決めていません。

「ファイルを先に用意し、OKF適用は後で考える」という合意は
[初回配置の記録](../ram/decisions/2026-09-04-aidlc-content-baseline.md)、
本家のcore構成へ配置を揃える承認は
[配置変更の記録](../ram/decisions/2026-09-04-aidlc-core-layout.md)にあります。

## 内容の確認

コピーした140ファイルとLICENSEのSHA-256を [SHA256SUMS](SHA256SUMS) に記録しました。
`docs/aidlc-content/` で次を実行すると、`src/core/` の原稿とこの場所のLICENSEが
記録された内容と一致するか確認できます。

```sh
shasum -a 256 -c SHA256SUMS
```

hashは記録されたファイルの内容比較に使うもので、作者や配布元の署名ではありません。
全在庫は [INVENTORY.md](INVENTORY.md)、コピー元との対応は [SOURCE.md](SOURCE.md) を参照してください。
