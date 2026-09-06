# `intent-capture`のsensorを固定本家どおりadvisoryとして扱う

- 日付: 2026-09-06
- 状態: Accepted
- 置換対象: `2026-09-06-intent-capture-vertical-milestone.md`のPR 2にある、
  `intent-capture`のsensor terminal receiptまたはsensor結果を人間承認gateへ到達するための
  必須証拠とした箇所
- 実装許可: 2026-09-06のユーザーによる、固定AI-DLC本家`2.6.123`と同じadvisory境界を
  採用して実装を続ける直接承認

## 背景

承認済みmilestoneは、`claim-sources`、`required-sections`、`upstream-coverage`の3 sensorについて、
fire eventとterminal eventを対にして検証し、その証拠がなければ`intent-capture`を人間承認gateへ
進めない計画だった。

固定AI-DLC `2.6.123`の配置graphでは、この3 sensorはいずれも`fire_on: gate`、
`enforcement: advisory`である。固定実装のgate処理は対象sensorを起動するが、advisory sensorでは
blocking sensor向けのexit、terminal receipt、verdict、Note検査へ進まず、sensor processの失敗、
budget override、terminal receiptの欠落をgate authorityに使わない。したがって、terminal receiptを
必須にする旧計画は、固定本家より強いfail-closed境界になる。

## 決定

`intent-capture`の3 sensorは、固定本家`2.6.123`と同じくadvisoryとして扱う。

- gateへ進むときは、適用される3 sensorをすべて起動する。
- 取得できた`SENSOR_PASSED`、`SENSOR_FAILED`、`SENSOR_BUDGET_OVERRIDE`、Note、findingは、
  品質判断の材料として監査記録と人間向け表示へ残す。
- sensorの結果、exit、terminal receiptの有無、fire/terminalのpair成立は、gate authorityにしない。
- sensor processの起動失敗、script error、budget override、terminal receipt欠落、advisory findingは、
  それ自体では`STAGE_AWAITING_APPROVAL`または人間のapprove/rejectを妨げない。
- gate authorityとしてfail closedにするのは、成果物、質問回答、summary確認、reviewer実行、
  learnings、人間応答など、このStageで別途必須とされた証拠に限る。
- sensorの内部入口はcaller supplied booleanを受け取らず、現在のStageとgraphから対象sensorを
  freshに導出する。ただし、この所有境界はsensor結果を承認権限へ昇格させるものではない。

起動を試みた事実と得られた結果は運用上観測できるようにするが、advisory sensorが壊れたときに
workflow全体を止めない。品質情報が存在する場合は承認画面で人間へ見せ、見えなかったことを
「問題なし」とは表現しない。

## 置換範囲

旧milestoneの次の意味だけを置換する。

- PR 2のsensor fire/terminal pairをgate用authority evidenceとして検証する計画
- sensor terminal receiptをgate到達の必須条件とする記述
- 3 sensorの「必要な証拠」がなければgateへ進めない受け入れ条件

3 sensorの起動、得られた結果の記録と表示、3成果物、summary確認、advisory reviewer、learnings、
freshな`HUMAN_TURN`、reject後の旧receipt無効化、公開`report`の4 result限定、2 PR構成は維持する。

## 本家との差分

新しい意図的な仕様・挙動差分は採用しない。sensor terminal receiptを必須にする案は、固定本家より
強い意図的差分になるため採用しなかった。

## 利用者・運用への影響

sensor故障時にもStageを人間承認gateへ進められるため、厳格なfail-closed品質保証より可用性を優先する
境界になる。一方、sensorが生成した警告や失敗は隠さず、人間が承認判断へ利用できる。

この境界はsensorに限る。`HUMAN_TURN`、summary confirmation、reviewer receipt等の必須証拠を緩めず、
advisory sensorの実行結果を承認済み品質の証明とも扱わない。

## 根拠

- `docs/ram/decisions/2026-09-06-intent-capture-vertical-milestone.md`
- `docs/配布_ai-dlc/.codex/tools/data/stage-graph.json`
- `docs/実装_aidlc-workflows/core/tools/aidlc-state.ts`
- `docs/実装_aidlc-workflows/core/tools/aidlc-sensor.ts`
