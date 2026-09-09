# Intentごとの承認済み実行計画でステージを実行する案

状態: Accepted。設計案提示後の「hai」でユーザーが本案を実装範囲として承認した。実装用の詳細計画とIssueへ具体化して進める。

## 目的と現在地

現行main（29f4344、PR152）はdiscovery→planning→tdd→integrationの固定順で、目的整理の中に現状解析を含む。利用目的が調査や検証だけでも、固定の前段成果やTDDを要求する箇所がある。ユーザーは初期化→目的整理だけを必須とし、残る段階をIntentの目的に応じて選択・並べ替え・追加したいと依頼した。

この案は共通の段階定義を利用可能なステージのカタログにし、実行順・採否・進捗はIntentのstate.jsonにまとめる。計画の追加・省略・並べ替えは毎回ユーザー承認を待つ。過去の会話を読まずに実行計画と進捗を再開できることを目標とする。

[要求・回答のRAM](../ram/decisions/2026-09-09-intent-execution-plan-request.md)。今回の選択制・永続形式・CLIを含む具体案への直接承認を確認済み。従来の33 Stageや4段階実装への承認を本案へ流用しない。

## ステージの責務

| ID | 名前 | 実行 | 主担当 | 文書成果 |
|---|---|---|---|---|
| initialization | Space等の初期化 | 必須・先頭 | 対話の調整役 | 導入済み設定・Ruleの確認。作業用ダミー文書は不要 |
| discovery | 目的整理と深掘り | 必須・2番目 | 要件整理、必要な調査、調整役 | design/ID/requirements.md、state内の実行計画 |
| architecture-analysis | 現状の構成分析 | 選択 | 調査、調整役 | codekb/current-analysis.md、codekb/architecture.md |
| planning | 実装計画 | 選択 | 調整役 | design/ID/implementation-plan.md |
| tdd | TDD | 選択 | worker、調整役 | 文書outputsは空でも可。実測テストは別の証拠契約 |
| integration | 統合検証 | 選択 | 調整役、必要なworker | 必要な現行仕様のcodekb文書。検証対象に応じてoutputsを宣言 |

各段階の終了Sensor・独立レビュー・人間の成果承認は維持する。計画承認と成果承認は異なる対象であり、計画への賛成だけで未実行段階を合格にしない。discoveryの要件と実行計画を一緒に提示し、両者が承認対象だと明示すれば同じ回答を根拠として記録できる。

discoveryでは目的を決めるための読取りや調査を行えるが、現状解析と構成図の作成を一律に要求しない。本格的な解析・図の作成はarchitecture-analysisが担当する。構成分析の成果は引き続きSpace共有で、Intentごとの重複コピーにしない。

## 初期化とIntent作成

保存先であるSpaceがないとIntentを保存できないため、入口で既存Spaceを選択するか正規CLIで新規作成する。その後にIntentを作り、initializationを未完了の先頭実行として記録する。配置・選択Space・必須Rule・workflow・CLI参照などが正しいことを開始/終了検査で確認して完了する。過去の作業実績を捏造して完了扱いにしない。
既存Spaceなら再作成せず利用条件を確認する。現在のAI-DLC準拠のSpace作成と配布を利用し、利用者のファイルを上書きしない。

## 共通定義とIntentの保存形式

stage-graph.jsonにはステージID・名前・手順Markdownと必須先頭順を定義する。固定advance辺と固定completion_stageは廃止する。個別MDは担当・文書inputs/outputs・開始/終了Sensor・手順を持ち、固定の次ステージは持たない。

state.jsonを唯一の更新元にする。配置は`aidlc/spaces/<space>/intents/<intent_id>/state.json`。計画と進捗を別ファイルへ二重保存しない。共通カタログ/手順のdefinition hashと、Intent内の計画版・承認対象hashは役割を分ける。

概念例（説明用の抜粋であり現行CLIの入力形式ではない）:

```json
{
  "execution_plan": {
    "revision": 1,
    "approval": "approved",
    "steps": [
      {"id": "s01", "stage": "initialization", "status": "completed"},
      {"id": "s02", "stage": "discovery", "status": "completed"},
      {"id": "s03", "stage": "architecture-analysis", "status": "pending"},
      {"id": "s04", "stage": "planning", "status": "pending"},
      {"id": "s05", "stage": "tdd", "status": "pending"},
      {"id": "s06", "stage": "integration", "status": "pending"}
    ],
    "omitted": []
  },
  "current_step_id": "s03"
}
```

実装では承認の回答出典・引用・対象hash・日時も保存する。実行回IDはCLIが生成し再利用しない。同じtddを追加してもs05とs08を区別し、s05の合格をs08の代わりにしない。Entry、Sensor、review、人間承認、文書宣言、テスト結果と受入済み版を実行回IDへ結び付ける。省略する任意ステージには理由を残し、選択漏れと意図した省略を区別する。

stepの進捗はpending（未開始）、active（作業中）、awaiting_approval（成果承認待ち）、completed（合格済み）。質問待ち・中断・中止は既存のIntent状態と実行回を組み合わせる。省略は実行計画の採否であり、架空のcompletedを作らない。変更前の計画と判断経緯は承認・状態変更の履歴で追えるようにし、全ツール操作auditは作らない。

## 実行と強制の境界

メインAIはprocedure CLIで現在の実行回・担当・入力と出力を取得する。beginは必須先頭、承認済み計画、順序、入力、開始Sensorを確認する。完了操作は終了Sensor・独立レビュー・現在版の成果承認が揃った実行回だけをcompletedにする。次に開始可能なのは承認した順序で未完了の先頭である。最後の選択実行回が完了し未承認の変更案がなければIntentを完了でき、integrationを必須にしない。

公開CLIは既存begin/check/procedure/reviewへ実行回IDを対応させ、固定順を進めるadvanceを実行回完了のfinishへ置き換える案とする。実行計画の読取り・変更提案と、版を指定した承認操作を加え、helpに引数・型・例を記載する。state直編集による進行を通常AI操作の正規経路にはしない。

hookは計画承認待ち・順序違反・未開始作業を止め、読取り、help、質問回答、正規の計画修正・承認操作を残す。サブエージェントの起動と結果回収はメインAIが担い、CLIは起動を代行しない。機械的検査とレビューによる通常運用の漏れ防止であり、OS権限やAIの内面まで保証しない。

## 順序・省略と品質条件

必須prefix以降には固定の順序制約を追加しない。例えばintegration→tddは既存コードの検証後に修正する用途として選択できる。ただし各実行に必要な入力や受入条件が存在することは確認する。

- initialization→discoveryだけで調査方針の整理を完了できる。任意4段階の省略理由を承認する。
- planning省略でtddを選ぶ場合も、実装範囲・受入条件・テスト方法の承認は必要。discoveryの成果または既存の有効な計画から具体化し、ImplementationPlan文書だけを一律必須にはしない。
- tdd省略のintegrationは既存成果を検証できる。対象コード版・検証command・実行結果を要求し、新規TDDの受入を要求しない。
- architecture-analysisを省略したdiscoveryに解析/図を要求しない。後段が必要とする入力が欠ける場合は、計画修正・追加実行を提案する。
- 文書outputsは文書のみ、成果文書がない実行は空でもよい。コードや実測検証の必要条件をoutputsの空で省略しない。

文書はOKF検索で選び、採用したpath/hashと実行回の対応を保持する。既存の共有currentと受入済み版の扱いを維持し、文書全量snapshotを追加しない。

## 途中変更とやり直し

追加・省略・並べ替えごとに変更理由、前後の実行順、省略理由、再検査対象を提示してユーザー承認を待つ。変更案と承認済み計画をstate内で区別し、未承認案で作業を進めない。読取りと計画整理は可能。進行中のworkerがある場合は停止を確認してから実行回や担当を変更する。

完了済み実行回を削除して過去を消さない。追加は新しい実行回IDで未実行部分へ挿入する。現在の条件を変える変更はbegin・Sensor・レビュー・成果承認を失効させる。過去の要件・受入済み入力を変える場合は対象実行回を指定してreopenし、影響する後続を再検査する。計画に必要な再実行を示し、ユーザー承認後に進む。

例: tdd実行後に設計見直しが必要ならplanningを追加し、続くtddとintegrationの必要性・順序も含む変更案を承認してから実行する。旧tddの合格を新tddへ流用しない。戻る理由は既存の`knowledge/log/<intent_id>-work-log.md`に残し、設計変更のwhyは別途adrへ記録する。

## 実装範囲と検証案

主な対象はsrc/internal/workflow（カタログ・MD解析）、src/internal/flow（state/transition/procedure/boundary/sensor/documents/review/reopen）、src/internal/minimal（hook/session）、src/internal/cli（引数・help）、src/core/workflow（6段階）、配布Rule/WORKFLOW、関連install/workspace/cmd tests、docs/developmentとRAMである。外部Go moduleを追加しない。

単独writerで、次を順序付きTDDとして実装する案とする。
1. カタログ6段階・必須prefix・stateの計画版/実行回ID・採否の保存検証。
2. begin/finish/procedureと実行列、初期化→discoveryだけで完了、同一段階の複数実行、順序飛ばし拒否。
3. 選択制Sensorと文書・テスト証拠の実行回対応。planningなしTDD、TDDなしintegration、解析の分離。
4. 計画と成果それぞれの人間承認、変更案の隔離、旧承認/合格流用拒否、hookで未承認作業拒否。
5. 差戻し・進行中worker停止・履歴・保存途中の同一再試行と競合検出。
6. CLI/help、6手順、配布、実CLI journey。

実装開始前にこれらを自己完結したIssueとexact targeted commandsへ具体化する。loopでは対象testのみ、独立review後の固定HEADで全体test/race/vet/format/tidy、結合journey、6構成buildをfinalへ集約する。承認待ち・部分保存・同stage別実行の取り違えを回帰ケースにする。

人間承認はIssue146で未merge・中断中。既存の会話承認方式と状態変更履歴へのユーザー判断を保持し、本案の計画版/実行回へ結び直す必要がある。未修正の旧実装をそのまま取り込まない。新schemaは明示して旧stateを誤読せず、既存ファイルの削除・自動移行・二重運用は作らない。旧配置の自動上書きも行わない。

この案への承認は6段階とIntent実行計画・承認・Sensorの一体変更を対象とする。未知の新ステージの無制限な追加や、全操作audit、別の配布方式への変更を含めない。結果が変わる未解決の契約が実装詳細化で見つかった場合は、その点だけ確認する。
