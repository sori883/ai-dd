# Stage Graphと段階別手順への分割案

状態: Proposed。2026-09-09のユーザー依頼は構成・反復の設計検討。コード・設定変更や実装は未承認。
check help改善の対象選択は保留し、この案を先に検討する。既存4段階とSensor・独立レビューを維持する。
後続のユーザー指定により、差戻し理由はIntentの作業記録Markdownへ保存し、outputsはドキュメントだけに限定する。
根拠は[補足合意](../ram/decisions/2026-09-09-stage-rework-log-document-outputs.md)。

## 目的

毎回WORKFLOW.md全文を読む負担を減らす。AIは小さな全体索引から現在の段階の手順だけを読み、
必要な共通操作はCLI helpを参照する。必須Ruleと今回の入力成果物は引き続き読む。
分割により入力量を減らせる見込みはあるが、実際のtoken削減量や運用品質はまだ測定していない。

## 推奨する責務

| 定義 | 正本とする内容 |
| --- | --- |
| stage-graph.json | 段階ID・表示名・手順MD参照、通常の前進経路、完了位置、見直しの戻り先ルール |
| stages/*.mdのfrontmatter | 担当agent、前提成果物と参照版、終了時のドキュメント、必要なSensorの識別子 |
| stages/*.mdの本文 | 現在段階の具体的手順、調査/要件整理/実装の担当依頼、判断例、検査失敗時の対応 |
| Go CLI | 定義を読み込み検査する処理、Sensorの実処理、独立review照合、stateの安全な更新 |
| state.json | 今回Intentの現在位置、待機/中断等の状態、参照版、合格版、Unit進捗 |
| Intentの作業記録Markdown | 差戻し元・戻り先・差戻し理由。ADRとは分離する |

遷移をJSONとMDの両方へ手入力しない。MDにnext_stage/requires_stageを重ねず、
必要な成果物とその合格版をinputsで表す。概要をMDに説明するとしても、機械的な遷移先の正本はJSONだけにする。
Goへ同じ遷移表を残してJSONを説明専用にすると二重管理が残るため、最終構成ではGoもJSONを参照する案を推奨する。
Sensorの検査コードや共通安全条件はGoに残す。本文や任意shell式が機械判定を上書きする仕組みにはしない。
以下のfield名は設計例であり、現在のCLIが受理する形式ではない。

## 配布先の構成例

```text
aidlc/workflow/
  stage-graph.json
  stages/
    discovery.md
    planning.md
    tdd.md
    integration.md
```

短いCodex入口SKILLからこの索引・現在手順を参照する。これらは製品の実行手順であり、
利用者のKnowledge/ADR/Ruleは従来どおりaidlc/spaces/<space>/knowledge/配下に置く。
実装する場合の製品原稿はsrc/配下とし、Goバイナリへ同梱して配布する。

## 全体JSON例

```json
{
  "schema_version": 1,
  "start_stage": "discovery",
  "completion_stage": "integration",
  "stages": [
    {"id": "discovery", "name": "目的整理＋深掘り", "procedure": "stages/discovery.md"},
    {"id": "planning", "name": "実装計画", "procedure": "stages/planning.md"},
    {"id": "tdd", "name": "TDD", "procedure": "stages/tdd.md"},
    {"id": "integration", "name": "統合検証", "procedure": "stages/integration.md"}
  ],
  "advance": [
    {"from": "discovery", "to": "planning"},
    {"from": "planning", "to": "tdd"},
    {"from": "tdd", "to": "integration"}
  ],
  "reopen": {"allow_current": true, "allow_ancestors": true}
}
```

ancestorsはadvance経路を逆に辿って到達できる前段階を意味する。advanceは循環を許さず、
反復はreopenとして区別する。初期案は現在の4段階・一つの通常進行先を維持する。
複数の通常進行先、工程skip、任意の新しい段階や並列stageを実行する汎用engineまでをこの例から推定しない。

共通条件は各edgeへ繰り返さない。advance/完了は現在段階の終了Sensorと独立reviewが合格したときだけ。
reopenは合格を前提とせず、明示理由をIntentの作業記録Markdownへ記載し、戻り先とその後続の合格・開始確認を無効化する。
差戻しという作業上の理由だけではADRを作らない。アーキテクチャ変更の判断が別途ある場合は、その設計理由をADRへ記す。
その後は戻り先の開始Sensorから再確認する。waiting/paused/cancelled/completedはstageのノードにせずIntentのstatusを維持する。

## TDDのfrontmatter例

この例のknowledge_rootはaidlc/spaces/<space>/knowledge、intent_idは今回IntentのID。
config.*参照は既存stateの宣言を意味し、任意の式評価を意味しない。

```yaml
---
stage_id: tdd
agents:
  - role: implementation
    agent: aidlc-worker
  - role: independent_review
    agent: aidlc-reviewer
inputs:
  - path: "${knowledge_root}/design/${intent_id}/requirements.md"
    version: accepted
    accepted_at: discovery
  - path: "${knowledge_root}/design/${intent_id}/implementation-plan.md"
    version: accepted
    accepted_at: planning
  - path: "${knowledge_root}/knowledge/current-analysis.md"
    version: current
    required_when: exists
  - path: "${knowledge_root}/knowledge/architecture.md"
    version: current
    required_when: exists
  - refs: config.adr.refs
    version: current
    required_when: adr_required
outputs:
  - role: architectural_decisions
    refs: config.adr.refs
    required_when: adr_required
sensors:
  start: tdd-start
  end: tdd-end
---

# TDD

1. 現在のstateと必須Ruleを読み、開始Sensorとbeginを通す。
2. 計画に沿って担当workerへ依頼する。Unit分割時は依存と別worktreeを守る。
3. テストの失敗を確認し、最小実装と再テストを繰り返す。
4. 実測した成果版・command・終了コード・出力を提出する。
5. 終了Sensor合格後、固定成果を独立reviewerへ渡す。
6. 合格後はadvance。計画変更ならplanning、要件変更ならdiscoveryへreopenし、差戻し理由をIntentの作業記録Markdownへ記載する。
```

検査IDと参照構文は実装前に既存Go Sensorとの対応を定める必要がある。
共通の必須Rule読込みや入力文書のOKF検査を省略できる意味にはしない。
agents配列は起動順・時点の区別が必要。workerとreviewerは同時起動せず、reviewは成果固定後。
discoveryは調査が必要な場合にresearcher、調査結果から要件をまとめるrequirementsを使う。
調整役はユーザー対話と共有state/文書の唯一のwriterとして共通手順に定義する。

## 作業記録とoutputsの範囲

Intent作業記録の配置案はaidlc/spaces/<space>/intents/<intent_id>/work-log.md。
Intent配下のMarkdownという保存先はユーザー指定で、work-log.mdというファイル名は提案。
差戻し元・戻り先・理由を短く記載する。例:

```markdown
## 差戻し: tdd → planning
理由: 当初の分割では同じファイルを複数Unitが変更するため、担当範囲を見直す。
```

今回の追加要求は差戻し理由の記録であり、全tool操作や全作業の日誌を義務化する合意とは扱わない。
進捗の正本はstateで、Markdownを読み直して現在stageを決める構成にはしない。
差戻し時にstateとMarkdownが片方だけ保存された場合の失敗・再試行契約は、実装計画で具体化する。
現在のCLIはstateのreasonへ保存する実装であり、このMarkdown記録はまだ未実装。

段階frontmatterのoutputsには、要件書・実装計画書・現状解析・構成図・機能説明・必要ADR等の文書だけを宣言する。
プログラムやテストソースの一覧、config.scopeやconfig.test_resultsをoutputsへ展開しない。
TDDで更新が必要な文書がなければoutputsは空配列でもよく、一覧を埋めるための文書は作らない。
コードの担当範囲は既存Unit/config、実行結果は既存テスト証拠とSensorで確認する。
outputsを文書だけにすることは、コード・テストの検証を省く変更ではない。
Intent作業記録はreopen時の共通記録とし、各段階で必ず生成するoutputsには重複列挙しない。

## 反復の検討結果

| ケース | 操作と根拠 |
| --- | --- |
| TDDのテストが失敗し実装修正 | tdd内で反復。段階の移動・開始記録の取り直しは不要 |
| TDDで実装計画の変更が必要 | planningへreopen。planning以降の旧合格を無効化 |
| 統合で要件の誤りが判明 | discoveryへreopen。全後続の旧合格を無効化 |
| 統合で計画内の実装修正が必要 | tddへreopen。tdd/integrationの旧合格を無効化 |
| 質問の回答待ち | stageを保ってwaiting。回答後resume |
| 合格済み共有文書を必要に応じ更新 | currentとして再検査。Intent別要件/計画のaccepted版と混同しない |

前提段階だけでTDD→planningの戻しを表すと、planningがTDDを待つ循環依存になり初回に開始できない。
nextだけでは、その移動が合格後の前進なのか見直しなのか分からない。経路と前提成果物を分けることで両方を避ける。

## 読込みと整合性

入口では小さなgraphと現在手順を読む。再開・段階変更・定義変更時には必要な手順を読み直す。
共通操作の詳細は必要なCLI helpだけを読む。Ruleの読込みは既存の会話turnの契約を維持する。
将来のCLIで現在stageに対応する手順と遷移候補を返す案は有効だが、コマンド名/出力契約は実装計画で定める。

実装前に、重複stage/存在しない参照・forward循環・未対応agent/Sensor・不明frontmatter key・
危険なpath・手順欠落の拒否を具体化する。手順がないとき古い全体文書で黙って代用しない。
動作中Intentに対するgraph/手順変更の版管理と再承認、配布済み環境の更新方法は実装前に確定が必要。
現在の保存版を黙って新graphへ付け替えることや、定義変更だけで旧passを利用することは避ける。

## 固定AI-DLC参照との関係

本家ローカルsnapshotは2.6.123。version原稿と分析索引の一致を確認した。
reverse-engineering.mdのfrontmatterにはlead_agent/support_agents、produces/consumes、requires_stage、sensors等がある。
分析上、stage-graph.jsonは生成された実行時ビューで、runtime再compile hookはaudit eventを参照する。
確認範囲はこの段階定義・分析・compile hook冒頭のみで、全生成経路や最新upstreamは未確認。

本案では承認済み4段階を維持し、JSONを遷移の正本、MDを局所手順・成果物契約の正本とする分担を提案する。
本家の33段階・receipt/audit連動生成を復元しない。理由は読み込み量と重複管理の削減。
この新しい定義・配布・実行時読込み方式は未承認の意図的設計案であり、実装前に影響を含む計画を提示する。
