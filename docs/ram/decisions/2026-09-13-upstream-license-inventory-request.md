# 参考プロジェクトのライセンスを同じ開発者向け文書へ整理する

2026-09-13、ユーザーは参考にしたプロジェクトのライセンスも考慮したいとして、一覧への追記を依頼した。[初回版と文書確認の合意](2026-09-13-v0-1-0-developer-inventory-request.md)を補足する依頼であり、初回`0.1.0`、ユーザーが文書を確認してからリリースへ進む順序を維持する。製品全体のライセンス選択、実tag・Draft・Release作成、Claude対応の再開は含まない。

## 文書整備の計画と許可

[Issue #190](https://github.com/sori883/ai-dd/issues/190)。今回の直接依頼を許可の根拠として、mainの`742da9457b7d836a144ac3c77bb16d91e0677312`を基準に、[既存の開発者向けMarkdown](../../developer-references-and-dependencies.md)、本記録、RAM索引の3ファイルだけを変更する。説明の本体は一枚を維持する。

親エージェントが唯一のwriterとなり、調査担当は原典の読み取り確認だけを行う。順序は固定版のLICENSE・NOTICEと現行資材の照合、本文執筆、独立したread-only review、親のread-only final、PR/checks/mergeとする。reviewは事実・根拠・初心者への説明・公開境界を確認する。finalはローカルリンク、固定版、変更範囲、`git diff --check`を対象にする。文書だけのためGoのTDDやローカル全体testは実施せず、対象PRで起動する既存CIを確認する。取り消す場合は文書差分のrevertで戻せる。

## 確認結果と残る判断

- 本家AI-DLCの保存版2.6.123はMIT No Attribution（MIT-0）で、通常のMITにある表示保持条件と区別する。元commitは未確認。保持済みの原典資料は現在の製品配布一覧ではない。
- OKF仕様v0.2の固定commitはApache-2.0で、同版のrepoにNOTICEはない。OKF Agent Memoryの初期比較版と後発skill参考版はMIT。`aidlc-okf`には上流LICENSE・出典の配布表示が未整備である。
- mattpocock、owainlewis、mblode、obra、cojiの固定版はMIT。12skillの原典表示と翻案記録を照合した。標準15skillすべての表示が整ったとは扱わない。
- YAML v3.0.5はファイルごとにMITとApache-2.0が適用され、自由選択ではない。同版には2011–2016 Canonical LtdのNOTICEがあり、LICENSE・Apache-2.0全文とともに配布で引き継ぐ整備が残る。
- Kagome・辞書共通・uni moduleはMITで、UniDicデータはBSD-3-Clause。日本語補助CLIには原典・解析器・辞書の5許諾文を梱包する実装がある。GoのLICENSEや実公開方法の整備は別に残る。
- GoのBSD-3-Clauseと追加のPATENTS文書を区別する。PATENTS同梱がBSD条文の直接の要求だとは説明しない。実build版の追加表示を含む配布全体の網羅確認は未実施。
- 開発用samber skillの取得commitは未記録。現行mainのMITを、取得当時の原文を確認できた証拠にしない。別途起動する開発ツールを本製品の同梱ライブラリへ数えない。

原文・固定版のリンクと、現在どこに表示があるか、受取人へ届けるための残対応を本文第6節へ集約した。ソースに許諾文があること、binaryに埋め込むこと、archiveや配置後に読めることを区別する。製品ライセンスを選んでも原典の条件は置き換わらない。LICENSES等の配布整備は案であり、このタスクで製品コード・設定・依存・許諾資材を変更しない。
