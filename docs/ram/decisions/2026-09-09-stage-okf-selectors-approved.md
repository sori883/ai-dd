# ステージ入力をOKF検索し、出力は保存先とmetadataを定義する

ユーザーは、inputsをfrontmatter条件で検索し、outputsへ保存先pathとmetadataを併記する案に「はい、お願いします」と回答した。

## 採用する方向

- inputsは文書のtypeやintent_idで対象を特定する。必須入力が0件なら開始不可、一意な入力を期待して複数見つかったら勝手に選ばない。
- 開始時に実ファイルのpathと内容の版を確認し、レビューで対象を取り違えない。
- outputsは保存先pathと、文書に求めるmetadataを併記する。保存先を検索条件だけで代用しない。
- 今回のIntentのADRと、調査で参照する過去のADRを区別する。過去ADRは関連検索後に採用した具体的文書を確定する。
- ステージ定義からconfig.adr.refsとconfig.feature_knowledgeという間接参照をなくす。
- [小文字化の指定](2026-09-09-lowercase-adr-request.md)を反映し、配置はknowledge/adr/、typeはadrとする。他typeの小文字化は追加しない。

この決定は[実ファイル名指定の依頼](2026-09-09-explicit-stage-document-paths-request.md)を、入力の検索と出力の明示という方式へ具体化する。OKF自体の規格変更ではなく、この製品のステージ定義と検査の契約である。

## 実装前に具体化する点

機能別Knowledgeと判断別ADRは名前と件数が可変である。共通ステージ定義における保存先の宣言方法はまだ具体化していない。全Intentで同じ名前へ統一すること、既存の機能別共有文書をIntent別の配置へ変更することは承認されていない。

現在のSensorには固定pathやconfig参照を使う検査もある。Markdownだけを変更して完了とはせず、検索・版照合・必須文書の検査を一貫させる計画を作る。既存の共有文書の更新と、前工程で合格済みの要件・計画の保全を区別する。

人間承認機能Issue #146は中断を維持する。今回の変更は別の実装計画・Issueで扱い、外部Go moduleの追加は行わない。現時点は計画整理であり、コード・配布定義は未変更。

## 出力一覧とmetadataの追加承認

ユーザーは、具体pathとmetadataをIntentごとの出力一覧へ登録する案へ「はい、良いと思います。type以外に指定が必要なものもあれば追加しておいてください」と回答した。出力先の一覧を持つ方式は確定した。type/title/description、文書の用途に応じたintent_idを期待条件とし、任意status/tagsを指定できる。生成日時はmemory CLIが引き続き管理する。

[実装計画](../../design/stage-okf-documents-plan.md)へ検索条件・出力登録CLI・state保存・共有文書/合格版の扱い・TDDを具体化した。mainから独立したworktreeで実装し、人間承認Issue146を再開しない。
