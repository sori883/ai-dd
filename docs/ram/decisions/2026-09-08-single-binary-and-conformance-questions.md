# Goシングルバイナリ継続とAI-DLC準拠範囲の確認

- 日付: 2026-09-08
- 状態: Resolved。シングルバイナリ継続・不明点の確認方針はAccepted。下記は質問時の履歴。

3点ともA、組織Ruleのファイル名はrule.mdとの回答を受けた。[採用記録](2026-09-08-single-binary-space-rule-accepted.md)を参照する。

ユーザーは「その他AI-DLC準拠するか不明な所は聞いてください。またGoのシングルバイナリは継続です」と指定した。
Goの単一実行ファイルという製品制約を維持し、本家の配布準拠を理由に撤回しない。
前回の配布方針を、隣接runtime treeが必須の配布へ決定したと解釈した部分は訂正する。
AI-DLC準拠の範囲を根拠から一意に決められない場合は、代案と影響を提示してユーザーへ確認する。

## 回答待ちの質問

1. 単一実行ファイルにOKF処理と初期資産も内包し、skill・hook設定・Rule等を本家の配置先へ展開するか。
   推奨は内包。代案は実行ファイルをaidlc一つにし、初期資産を別途配布する方式。
   外部okf実行ファイルを利用者の必須依存とする先行案は、そのまま採用しない。
2. Spaceの名前・コマンド・作成と選択の分離を本家準拠とし、生成物は新方式のOKF構成へ変更するか。
   推奨は生成物を新方式へ変更。代案は本家のmemory/intents/codekbと空knowledgeを残し、OKF初期化を別操作にする方式。
3. 本家がdefault Spaceからコピーする組織ルールを、新方式でも作成時にコピーするか。
   推奨はコピーによる独立。代案はdefault Ruleの共有参照、または継承せず配布初期Ruleから開始する方式。

これらは本家固定2.6.123の配布・Space作成と、新製品のOKF配置を両立するための設計選択。
質問を提示しただけで回答済みにしない。型名・新CLI・Intent識別・hookの詳細も、この回答要求だけで採用済みにはしない。
外部Go module追加なし、既存Intent移行不要、既存data削除なし、M1実装未承認を維持する。

## 置換・根拠

- [前回の配布方針](2026-09-07-minimal-layout-distribution-conformance.md): 配布先・生成物管理の準拠は維持し、単一バイナリの解釈を訂正。
- [Space作成方針](2026-09-07-minimal-space-creation-conformance.md): OKF初期化・org継承との接続を今回質問。
- [固定版の配布調査](../research/2026-08-29-existing-distribution-format.md)、[Space作成調査](../research/2026-08-31-space-creation-contracts.md)。
- [現行設計案](../../design/minimal-product-contract-m0.md)。
