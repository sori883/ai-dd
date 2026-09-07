# OKF・初期資産の内包、OKF Space作成、rule.md継承を採用する

- 日付: 2026-09-08
- 状態: Accepted（3点へのユーザー直接回答）

## 回答

[確認した3点](2026-09-08-single-binary-and-conformance-questions.md)に対し、ユーザーは
「1: A」「2: A」「3: A、org.mdではなく、rule.mdとして配置してほしいです」と回答した。
3点とも回答済みとして扱い、再確認しない。

## 採用する契約

1. OKF処理と初期資産をGoのaidlcシングルバイナリへ内包する。
   導入時にskill・hook設定・Rule等をAI-DLCの配置先へ展開する。別okf実行ファイルや別配布runtimeを必須にしない。
   内包するのは導入用資産であり、通常の読込みでは利用先の配置済み本文を使う。編集済みRuleを内包版で上書きしない。
2. Spaceのコマンド、名前の扱い、作成と選択の分離は本家基準を維持する。
   生成内容は新OKF構成へ変更し、Space作成時にknowledge bundleと初期Ruleを用意する。
   新Space用に旧memory/intents/codekb構造を生成してから別操作でOKF化する案は採用しない。
3. 組織Ruleをdefault Spaceから作成時にコピーする。ファイル名はorg.mdではなくrule.md。
   合意済みrules配置に従い、コピー元は `aidlc/spaces/default/knowledge/rules/rule.md`、
   コピー先は `aidlc/spaces/<space>/knowledge/rules/rule.md`。OKF Concept IDは `rules/rule`。
   作成後は各Spaceで独立して編集し、defaultの後続変更を自動反映しない。

一般的な作業手順Rule等を追加する場合もknowledge/rules内のOKF文書とし、rule.mdのコピー範囲を全Ruleへ勝手に拡張しない。
初回default初期化、コピー元欠落・不正、保存の部分失敗時の扱いは本家の根拠とこの契約に従って計画へ具体化する。

## 本家との意図的差分

比較対象はリポジトリ固定AI-DLC 2.6.123の確認済み配布・Space作成契約。

| 本家の挙動 | 採用する挙動 | 理由・利用者への影響 |
| --- | --- | --- |
| release CLIは隣接runtime treeを使う | Go単一バイナリがOKF処理・初期資産を内包し配置する | 単一バイナリ配布を継続し、利用者の別tool導入を不要にする |
| Space作成でmemory/intents/codekbと空knowledgeを生成 | 新SpaceをOKFのKnowledge・設計・KDR・Rule配置で初期化 | 新方式を作成後から使える。旧生成形式との互換性は要求しない |
| default/memory/org.mdを新Spaceへコピー | default/knowledge/rules/rule.mdを同じ相対pathへコピー | 組織RuleもOKFとして統一。作成後に独立する本家の継承方式は維持 |

本家最新upstreamの全体一致を主張しない。
根拠: [配布調査](../research/2026-08-29-existing-distribution-format.md)、[Space作成調査](../research/2026-08-31-space-creation-contracts.md)。

## 置換対象と許可

外部okf実行ファイルの取得案、初期資産の別配布案、OKF初期化を別操作とする案を置換する。
[配布準拠](2026-09-07-minimal-layout-distribution-conformance.md)・[Space作成準拠](2026-09-07-minimal-space-creation-conformance.md)の未確定範囲はこの回答で具体化する。
過去の記録は履歴として残し、[M0設計](../../design/minimal-product-contract-m0.md)を現在の判断へ合わせる。

今回の承認は上記設計選択。M1全体の実装開始、外部Go module追加、既存org.mdや旧dataの移動・削除は含まない。
設計・RAM・索引だけを更新する。
