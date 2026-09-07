# M0最小契約を具体化した案と、未承認のM1境界

- 日付: 2026-09-07
- 状態: Proposed。ユーザーから許可された設計・記録作業。契約採用とM1実装は未承認。
- 詳細: [M0最小契約案とM1実装計画](../../design/minimal-product-contract-m0.md)

後続指示: [Space knowledgeへのOKF集約](2026-09-07-space-knowledge-okf-unification.md)により、下記の分離配置・Space非依存・KDR専用形式の提案は置換された。本文は当初の検討履歴として保持する。

## 依頼と現在地

ユーザーは製品最小化の作業を引き継ぎ、M0の残りである最小契約を一つの具体案へまとめるよう依頼した。
対象はIntent/KDR対応、テンプレートとCLI、hookによる記録漏れ判定、例外・故障、OKF導入とrules、M1の対象file・受入条件・検証・許可範囲である。
M1全体・新ロードマップの実装は未承認で、旧33 Stageの包括承認を流用しないことも明示された。

GitHubを再確認し、mainとHEADは `fc8bd7ea633f0c1ad8cc605233572a6106b3b911`、
[PR #127](https://github.com/sori883/ai-dd/pull/127)はMERGED、[Issue #126](https://github.com/sori883/ai-dd/issues/126)はCLOSED。
Open Issue・PRは各0件。直近PRのfiles、commits、本文とchecksを取得し、現行help・hook・CIも確認した。
新設計とRAMの既存未コミット差分は保持した。Go全検証・live E2Eは今回行っていない。

## 今回の推奨案

- ランダムIDと `docs/kdr/<id>.md` の一対一対応にし、KDRをIntentの唯一の記録とする。別registry・旧Spaceへの依存を作らない。
- AIが配置テンプレートを読み、CLIで作成・期待hash付き更新・修復を行う。同じ目的の再開では同じIDを使う。
- hookはKDRの存在・必要項目と、今回の記録更新を分けて確認する。
  session限定の未記録表示、実行中tool最大1件、rules読込確認に必要な一時情報だけを候補とする。
  全操作履歴、工程番号、独自JSON snapshot、receipt、transcript解析は要求しない。
- M1は一つのworktreeの単独writer、Codex 0.153.4/macOS arm64を候補にし、対応hookを先頭で実証する。
  未対応ならversion変更を勝手に採用せず確認する。Stopは補完を一回求め、修復不能なら警告して中断できる。
- OKF Agent Memoryはv0.1.2 / `d4c523ed5ce916fa207fe314851b98721421c891` を固定候補とする。
  release assetのSHA-256を設計へ記録し、専用配置の外部CLIとして使う。新しいGo moduleは追加しない。
- bundleは利用repositoryの `docs/knowledge/` 一つ。必須Concept IDを短い入口fileへ明示し、検索順位で選ばず本文を読む。
  導入はinit、OKFの知識変更logは維持する。AGENTS/skill等も変更するbootstrapは採用しない案。
- OKFの本文/index/log保存は一体の原子操作ではない。M1の知識更新は単独writer・小さな承認済み変更とし、部分失敗を現物確認して復旧する。
- M1のfile候補、1 Issue/PR・1 work unitのTDD順序、独立review、read-only final、GitHub checksとmerge後確認までを詳細計画へ記した。

この一時情報・直列化、配置、公開CLI、固定tool導入はユーザーの回答だけから一意に確定するものではない。
設計の推奨であり、Acceptedへ変更していない。特に、一時情報を持たず存在だけ確認する代案では更新漏れを検出できないことを比較表に示した。

## 調査・計画確認

Serenaを有効化して初期指示を確認した。GitHubはgithub-pr-workflow、外部仕様はtechnical-research、計画はimplementation-planningの手順を参照した。
read-onlyのtechnical_researcherへ固定OKF、project_plannerへM1のfile・testと競合境界を委譲した。
Context7には指定OKFの一致libraryがなかったため、指定commitの公式ソースへfallbackした。
Codex hooksはContext7と2026-09-07の公式文書を照合し、現在のCLI version表示だけをローカル確認した。
動作実証前なので「0.153.4で新hook動作確認済み」とは扱わない。
参照URL・asset hash・保証の限界は詳細設計に保持する。

## 前の決定との関係

- [製品方向性](2026-09-07-okf-kdr-minimal-product-direction.md)と[Intent KDR/hookの境界](2026-09-07-intent-kdr-hook-boundary.md)を具体化する後続提案。
- [既存移行不要](2026-09-07-minimal-product-no-legacy-migration.md)は維持し、既存dataを削除しない。
- [マイルストーン案](2026-09-07-minimal-product-milestones.md)のM0契約未確定部分に、具体案ができたことを追記する。
- [成果物中心案](2026-09-07-artifact-centered-workflow-proposal.md)のsnapshot/receiptを承認済み要件として持ち込まない。
- 固定AI-DLC 2.6.123の工程state/auditを使う挙動から意図的に方向転換する理由と互換性影響を詳細設計に明示した。
  過去の承認を削除・上書きせず、M1を許可する後続の回答があればその範囲を別途記録する。

## 残る確認と許可

まず具体化したM0契約案の採用可否をユーザーへ確認する。採用だけをM1の実装許可と解釈しない。
M1開始を明示された場合の対象は、詳細計画の新規Intent一周、固定okfの専用配置、対象fileと必要なIssue/PR・検証・mergeである。
M2/M3、外部Go module、追加権限、旧経路拡張、既存data削除は含めない。
今回変更したのは設計・RAM・索引だけで、コード・設定・Issue・PRの変更や外部tool導入はしていない。
M0は具体化済み・採用待ちであり、全体完了とはしない。

文書検証では新規文書の相対リンク先の存在、code fenceの対応、`git diff --check`を確認した。
これは設計資料の確認であり、提案CLI・hookの動作証拠ではない。
