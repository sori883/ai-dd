# KDRでもOKFの標準metadataを利用する

- 日付: 2026-09-08
- 状態: Clarified（既存方針の説明補足。generatedのテンプレート案を具体化）

## 背景

Intent識別の説明で `type`、`intent_id`、`title` だけを例示したため、ユーザーから
OKF Agent Memoryの `description`、`tags`、`generated`、`status` は使わないのか質問を受けた。
3項目だけに制限する意図はない。既存設計例にもdescription・tags・statusは含まれていたが、
generatedの例と用途の説明が不足していた。

## 設計への反映

Knowledge・KDR・RuleにOKFの標準metadataを使う方針を維持する。
KDRの例はtype・title・description・tags・generated・statusに、合意済みのintent_idを加えて示す。
titleは名前選択、descriptionとtagsは一覧・検索、generatedは内容の作成者と最終の意味ある変更日時、
statusは文書の成熟度に使う。generatedはbyとatの二つの子項目であり、byatという項目ではない。

generated.byには実際に内容を作成・更新したactorを記す。例のモデル名を固定値としてコピーしない。
generated.atはUTC offset付きISO 8601日時とする。正確なactorを得るCLI入力の契約は実装計画で具体化し、
不明なモデル名や実施していない人間の確認を推定して記録しない。
これらのmetadataだけの変更でhookの未記録判定を解消しない。本文に判断・結果等を書く契約を維持する。

sources（根拠）、verified（実際の確認者と日時）、stale_after（見直し時期）、resource（対象URI）も、
必要な文書で使い、読込・更新時に落とさない。全KDRで空の項目を一律に必須化するものではない。
status: stableはIntentの完了・テスト合格・人間の承認を意味しない。

## 確認した根拠

ユーザー指定のローカルOKF Agent Memory参考実装では、
[Concept型](../../実装_okf-agent-memory/pkg/okf/types.go)がこれらのmetadataを持ち、
[更新処理](../../実装_okf-agent-memory/pkg/okf/mutate.go)はactorと日時をgeneratedへ設定する。
[Reviewの例](../../実装_okf-agent-memory/examples/books/reviews/cognitive-bias-review.md)もgeneratedを含む。
元commitが不明なローカル実装参考であり、最新upstreamの確認とは扱わない。
標準項目の意味は[固定OKF v0.2の分析](../../okf-analysis/02-frontmatter-trust-lifecycle.md)を参照した。

この記録はmetadataの扱いを補足するものであり、全操作auditや工程state、新しい外部依存の採用を含まない。
