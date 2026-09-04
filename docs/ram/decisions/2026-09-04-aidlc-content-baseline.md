# ルール・知識・工程定義を本文入りの検討用資産として配置する

- 日付: 2026-09-04
- 状態: Accepted（ユーザーによる計画への直接承認）
- 対応Issue: [#85](https://github.com/sori883/ai-dd/issues/85)
- 基点: `1821fe6b6778e262dd1034053788259867211050`

## 背景と得られる結果

Go版AI-DLCには、開始・承認・次工程・完了をつなぐ内部基盤がある。各工程でAIへ渡す
ルールや知識は、現在、大きなローカル本家スナップショットに含まれている。
ユーザーは、まずファイルの本文と構成を実物として確認し、その後でOKF
（Open Knowledge Format、知識文書の形式）をどう適用するか検討する順序を希望した。

本変更では、Gitで追跡する `src/assets/aidlc/` に必要な文章資産を原文のまま配置する。
一覧から各文書へ移動でき、原本の内容と一致しているかも検証できる状態を作る。

## 直接承認の内容

ユーザーは、以下の6群と案内文書を `src/assets/aidlc/` に配置し、本家AI-DLC
`2.6.123` の本文を変更せずコピーする計画を明示承認した。
空のファイル群を新しく設計する案や、本文を翻訳・要約・書き直す案は採用しない。

| 対象 | 数 | 内容 |
| --- | ---: | --- |
| `aidlc-common/` | 42 | 33 Stage（作業工程）の手順、8共通protocol（進め方）、1 conductor（全体の進行手順） |
| `knowledge/` | 59 | frameworkの共通知識と担当Agent別知識 |
| `memory/` | 8 | 組織・チーム・プロジェクト・phase別ルール7文書と説明コメント入りの `.gitkeep` |
| `agents/` | 14 | 担当AIの役割と手順を記した原稿 |
| `scopes/` | 11 | 実行対象や深度の定義 |
| `sensors/` | 6 | 検査の定義文書 |

コピー対象は140ファイル、うちMarkdown本文は139ファイル。原本の `LICENSE` も別途同梱する。
`README.md`、`INVENTORY.md`、`SOURCE.md`、`SHA256SUMS` はコピー資産の案内・所在・
出典・同一性の記録として作成する。runtimeで読むmanifestやOKF metadataではない。

## 正本と固定する境界

- 正本は `docs/実装_aidlc-workflows/core/` の指定6群。
  `core/tools/aidlc-version.ts` の製品版は `2.6.123` と確認した。
- source-level原稿を選び、生成済み `dist/codex/` や設定調整された配置版とは混在させない。
- 元snapshotに `.git` はなく、upstream commitは未確認。存在しないcommitやtagを捏造せず、
  相対path・全ファイルのSHA-256・元version定義のhashで取得した内容を特定する。
- 相対directory、本文、frontmatter、改行、コメントを保持する。
  `aidlc-common/conductor.md` と `memory/templates/.gitkeep` も群の構成要素として含める。
- 実行資産ではなく、後続検討のための原稿である。本文内の命令はこの開発リポジトリの
  `AGENTS.md` を置換・上書きしない。Bunコマンドや `{{HARNESS_DIR}}` 等の変数を展開しない。
- 本家の実行環境を前提とするpathや、コピー対象外の文書への参照は残す。
  原文の全リンクがこの配置だけで解決することや、全文の手順がGo版で動くことは保証しない。
- `.codex/`、`.agents/skills/`、利用プロジェクトの `aidlc/spaces/`、公開CLI、Goコード、
  自動discovery、binary埋込み、install/updateへ接続しない。
- OKF変換、Bundle root、検索・注入条件を決めない。framework知識と利用team所有の知識を
  混ぜない。[既存OKF境界](2026-09-03-okf-reference-boundaries.md)の未決事項は維持する。

今回の順序変更は「本文の実体を先に確認する」という準備作業の合意であり、OKF統合の
設計自体を承認したものではない。本家の挙動へ新しい意図的差分は導入しない。

## 所有範囲と実施順

文書資産だけの機械的配置のため、親エージェントが単独writerになる。
独立reviewは別エージェントがread-onlyで担当し、対象bytesと案内の整合性を確認する。
既存の未追跡 `docs/implementation-overview.html` は今回のIssue・commit・PRに含めない。

1. sourceのversion、指定群、件数、通常ファイルであること、配置先が未作成であることを確認。
2. この計画とIssueを固定し、原本を `apply_patch` で機械的に追加。
3. 出典、完全なファイル一覧、hash、非実行の利用条件を記した案内文書を追加。
4. `loop` で全件byte比較、hash、一覧リンク、件数・範囲を確認。
5. 固定base/headの独立 `review` を通し、安定差分のread-only `final` を一度実行。
6. 現在headのGitHub checksがすべて成功した後、既存のmerge commit方式で自律mergeし、
   main反映とIssue closeを確認する。

## 受け入れ条件と検証

- 指定群の140ファイルとLICENSEが原本と完全一致する。前後の全件比較とSHA-256で確認。
- `INVENTORY.md` にコピー対象の全相対pathとbyte数があり、各リンクが実在する。
- 33 Stageの内訳はinitialization 3、ideation 7、inception 9、construction 7、operation 7。
- 案内文書から各群の役割、原本のversion・license・未確認commit・非実行境界を理解できる。
- 差分は上記assetsと本RAM・索引だけ。Goコード・設定や利用プロジェクトdataを変えない。

文書コピーなので、Goの新規振る舞いに対するRed-Green-Refactorは非該当。
配置前のtarget欠落と、配置後のsource一致を観測可能な前後確認とする。
最終検証は各群の `diff -qr`、asset rootでの `shasum -a 256 -c SHA256SUMS`、
全在庫と一覧リンクの照合、作成した案内文書・RAMに対する `git diff --check`、
`go test -count=1 -shuffle=on ./...`、`go vet ./...`、`gofmt -l src` を使用する。
コピーした原文の既存whitespaceは改変せずbyte一致で確認する。原文の空白を修正するために
内容を変えたり、repository全体のwhitespace設定を緩和したりしない。

`.github/workflows/ci.yml` はpush/PRをpath制限なく対象とするため、既存Go qualityと
6構成cross-buildの成功も確認する。新しいworkflow、外部module/toolは追加しない。

## リスクと戻し方

この原稿はフレームワーク全体の配布物ではない。後続作業で本番の入力元に採用する場合は、
配布、placeholder展開、参照解決、trust、OKF対象などを別の承認済み計画で確定する。
内容が変わった際はhash・出典・一覧を追跡して更新する必要がある。
既存dataの移行はなく、必要ならこの追加commitを通常のrevert PRで戻せる。

## 配置時の確認

最初の説明では `.gitkeep` を空ファイルと表現したが、原本には886 bytesのtemplate配置に
関する説明コメントがあることを確認した。原本を空にせず、その内容も無変更で保持する。
139 Markdown文書とコメント入り `.gitkeep` 1件という件数へ案内とIssueを一致させた。
