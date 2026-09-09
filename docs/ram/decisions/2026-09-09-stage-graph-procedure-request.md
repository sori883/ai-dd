# 全体WORKFLOWの読込みをStage Graphと段階別Markdownへ分割する検討

状態: Requested / Proposed。ユーザーは毎回WORKFLOW.mdを読み込む負担を減らすため、
stage-graph.jsonへ段階ID/名称/手順参照を、段階別Markdownのfrontmatterへagent/入力/出力を置く案を依頼した。
次段階か前提段階か、TDDから計画へ戻る反復をどう表すかも検討対象。今回の依頼は設計検討であり実装許可ではない。

[具体案とケース検討](../../design/stage-graph-procedure-proposal.md)を作成した。
推奨はJSONへ通常進行/見直しを集約し、MDへ前提成果物と版・出力・担当・手順を置く方式。
TDD内のRED/GREENは内部反復、計画/要件変更はreopenで前段へ戻し後続の合格を無効化する。
Goが新定義を参照すること、稼働中Intentの定義版管理、配布更新は実装前に計画を具体化する。

[help改善の実案件案](2026-09-09-new-sensor-pilot-request.md)は対象未選択のまま保留し、今回の検討を優先する。
過去の4段階・Sensor・独立review・共有Knowledge/ADR/Ruleの責務を置換承認した記録ではない。
