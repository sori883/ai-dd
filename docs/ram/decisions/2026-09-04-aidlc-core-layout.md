# 原稿の配置を本家core構成へ揃える

- 日付: 2026-09-04
- 状態: Accepted（ユーザーによる計画への直接承認）
- 対応Issue: [#87](https://github.com/sori883/ai-dd/issues/87)
- 基点: `06d31b729bb30d8efb20eff47e249c5cf8266499`（PR #86のmerge commit）
- 部分置換: [本文入り資産の初回配置](2026-09-04-aidlc-content-baseline.md)の配置先だけを置換。

## 背景と利用者が得る結果

ルール・知識・工程手順の本文は、PR #86で `src/assets/aidlc/` に配置した。
ユーザーは独自の置き場ではなく、本家と同じディレクトリ構成を希望した。
本家の開発用 `core/` とこのリポジトリの `src/core/` を対応させることで、
本家のパスから原稿をそのまま探せる状態にする。

これは開発用原稿の整理であり、Codexへのインストール先を変更するものではない。
140ファイルの内容、OKF（知識文書の形式）の適用を後で検討する順序、実行基盤と未接続である
境界は維持する。旧RAMの本文は当時の決定として保存する。

## 直接承認と基本方針

「本文を変えず `src/core/` へ移動し、一覧・リンク・ハッシュ記録を更新して一致を確認する」
計画を提示し、ユーザーは「はい、えっと基本的に本家準拠で良いと思います」と承認した。

今後も本家準拠を基本方針とする。本家とはローカルに固定した参照版の確認済み範囲を指し、
最新upstreamとの一致を推測しない。本家から意図的に仕様・挙動を変える必要がある場合は、
既存のAGENTS.mdに従い理由・影響を提示し、実装前に明示承認を得る。
この回答を、新機能・配布・権限・運用変更を無制限に許可したものとは解釈しない。

## 確認済みの根拠と配置

参照版は `docs/実装_aidlc-workflows/core/tools/aidlc-version.ts` の `2.6.123`。
指定6群の実体と、[構成分析](../../aidlc-analysis/01-package-architecture.md)を照合した。

| 本家の手書きソース | 新しい配置 | 件数 |
| --- | --- | ---: |
| `core/aidlc-common/` | `src/core/aidlc-common/` | 42 |
| `core/knowledge/` | `src/core/knowledge/` | 59 |
| `core/memory/` | `src/core/memory/` | 8 |
| `core/agents/` | `src/core/agents/` | 14 |
| `core/scopes/` | `src/core/scopes/` | 11 |
| `core/sensors/` | `src/core/sensors/` | 6 |

合計140件（139 Markdown文書と886 bytesの説明コメント入り `.gitkeep`）の全文・改行・
frontmatter（文頭の定義項目）を無変更で移動する。6群の内部階層とファイル名も保持する。

独自のREADME・INVENTORY（一覧）・SOURCE（出典）・SHA256SUMS（内容照合用ハッシュ）と
コピー元のLICENSEは `docs/aidlc-content/` にまとめる。本家のcore構造へ独自の案内を混ぜず、
「開発対象はsrc、開発用の説明文書はdocs」という既存規則に従う実装詳細である。
LICENSEは指定原稿の出典資料として保持し、Goコードのライセンスは変更しない。

原稿とLICENSEのSHA-256値は不変。ハッシュ一覧のパスは案内ディレクトリ基準へ更新し、
SHA256SUMSファイル自身の変更後ハッシュをSOURCEに記録する。

## 対象・所有権・実施順

- 対象: 旧 `src/assets/aidlc/` の145件、新 `src/core/` の140件、新 `docs/aidlc-content/` の5件、
  本RAMと `docs/ram/README.md` の索引。
- 親エージェントが単独writer。文書移動だけなのでGo実装担当は起動しない。
- 独立reviewerは固定base/headを読み取り専用で確認する。
- 旧配置の現在参照は案内とRAMだけで、実行時の依存はない。旧RAM本文は更新しない。
- 既存の未追跡 `docs/implementation-overview.html` は変更・追跡・削除しない。

1. Issueへ直接承認と計画を記録し、移動先未作成と旧配置の原本一致を確認する。
2. `apply_patch` で本文を無変更移動し、旧配置に重複コピーを残さない。
3. 案内・一覧・出典・ハッシュ記録を新配置へ合わせ、RAM索引の置換関係を更新する。
4. `loop` で移動前Git blob、本家の原稿、移動後ファイルを全件比較し、リンクと在庫を確認する。
5. 固定base/headの独立 `review` を通し、安定差分でread-only `final` を実施する。
6. 対応Issueを紐づけたPRの現在headでchecks成功を確認後、既存merge commit方式で自律mergeし、
   mainへの反映とIssue closeを確認する。

## 受け入れ条件と検証

- `src/core/` は指定6群の140件のみで、本家と相対パス・全byteが一致する。
- 全原稿とLICENSEは移動前Git blobとも一致し、旧配置にファイルを残さない。
- 一覧の140行のパス・リンク・byte数が一致し、作成案内のローカルリンクが実在する。
- `docs/aidlc-content/` のSHA256SUMSによる141件の照合が成功する。
- Goコード・設定・利用プロジェクトdata・旧RAM本文は変更しない。

文書移動のみで実行時挙動を変えないため、Go機能の新規TDDは非該当。
移動前の新配置未作成、移動後の全件一致を観測可能な前後確認とする。
`loop` と `review` は対象のbyte・リンク・ハッシュ・在庫・変更範囲だけを検証する。

`final` では全件比較、`docs/aidlc-content/` での `shasum -a 256 -c SHA256SUMS`、
一覧・案内リンク・在庫照合、`git diff --check`、`go test -count=1 -shuffle=on ./...`、
`go vet ./...`、`gofmt -l src` を実施する。原本LICENSEの既存末尾空行は修正せず、
移動のrename差分とbyte一致で確認する。原本のwhitespaceを正規化しない。
既存CIはpush/PRをpath制限なく対象とするため、Quality 2構成・Build 6構成も確認する。

## 境界・リスク・戻し方

新しい外部module/toolは追加しない。実行時仕様・挙動の意図的変更はない。
準拠を確認するのは固定参照版の指定6群であり、本家リポジトリ全体ではない。
Goコード、公開CLI、自動discovery、binary埋込み、install/update、`.codex/`、
`.agents/skills/`、利用プロジェクトの `aidlc/spaces/` へ接続しない。
本文中の命令は開発側AGENTS.mdを上書きせず、Bunコマンドや置換変数は展開しない。

古いファイルパスへの外部ブックマークは移動により変わる。新しい案内・一覧を入口とし、
移動履歴はGitで追跡する。コピー原文中の本家参照は本文保持のため変更しない。
必要なら通常のrevert PRで旧配置へ戻せる。利用プロジェクトdataの移行はない。
