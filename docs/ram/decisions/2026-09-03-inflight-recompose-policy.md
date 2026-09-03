# in-flight recompose方針

- 状態: Accepted
- 決定日: 2026-09-03
- 関連Issue: #61（StartIntent接続）、将来のrecompose Issue
- 置換対象: [AI-DLC Go実装ロードマップ](2026-09-03-aidlc-implementation-roadmap.md)、
  [Stage Plan builder計画](2026-09-03-stage-plan-builder-plan.md)の「実行中はStage構成を固定する」記述

## 決定

本家AI-DLC `2.6.123`の確認範囲に合わせ、将来のin-flight recomposeではIntent開始時のPlan objectを永続runtime
sourceにしない。実効routingの正は、各Stage行へ保存された`aidlc-state.md`の`EXECUTE` / `SKIP` suffixとする。
`.aidlc-plan.json`、`.aidlc-stage-plan.json`などのGo独自sidecarは作らない。

recomposeは、人間がproposalをApproveまたはEditした後だけ実行可能とする。変更対象はcompiled graphに存在し、
current Stageより後ろにあるpending Stageに限る。completed、in-progress、jump-skipped、awaiting、revising、
current以前のStage、Current Stage、checkbox markerは変更しない。scope、Depth、Test Strategy、Review Overrideも
変更しない。変更後はsuffix、集計、phase progress、Next Stage、Last Updatedをcanonical stateへ反映する。

## 今回のStartIntentへの適用

今回のStartIntentは開始時の`state.BuildInitial`結果を`Initial.Plan`へin-memoryで返し、`state.WriteInitial`で全Stageの
suffixを保存するだけとする。Planを後続runtimeの固定照合へ保存せず、recompose本体、state parser、audit、Stage実行は
実装しない。write failureでは作成済みIntentと構築済みInitialを保持し、completion falseとする。

## 互換性と境界

初期stateの正常なsuffix、Stage番号順、Greenfield補正、off-path producer advisory、producer不在fail-closedは既存の
承認済み契約を維持する。recompose実装時にproposal前後のstrict validation、completed producerの扱い、dependency error、
walking-skeleton制約を別Issueで確定する。最新upstream全体との一致は未確認であり、この決定は固定snapshot `2.6.123`の
参照範囲に限定する。
