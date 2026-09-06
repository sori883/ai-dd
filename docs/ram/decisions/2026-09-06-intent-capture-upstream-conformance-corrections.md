# `intent-capture`の残存境界を固定本家へ揃える

- 日付: 2026-09-06
- 状態: Accepted
- 置換対象:
  - `2026-09-06-intent-capture-vertical-milestone.md`のPR 2にある、reject後の旧receiptを一律に
    無効化する記述のうち、Stage全体で1回だけ実施するlearningsまで再実行対象に含める意味
  - GitHub Issue #120とCodex receiver sourceにある、revision時にlearningsを再実行する記述
- 実装許可: 固定AI-DLC `2.6.123`へ新しい意図的差分を加えない包括承認枠と、2026-09-06の
  ユーザーによる「固定本家相当へ境界を狭めて続行する」直接確認

## 背景

`intent-capture` execution verticalの実装監査で、承認済み計画と固定本家`2.6.123`の間に、sensor以外にも
意味のずれが残っていることが分かった。固定本家の`stage-protocol.md`はlearnings ritualをStageごとに
1回だけ実施し、`Request Changes`後のrevisionでは再実行しない。一方、Issue #120と作業中のreceiverは、
summary、review、sensorと一緒にlearningsも再実行する記述になっていた。

また、作業中のGo実装はlearningをaudit eventへ記録するだけで、固定本家が正本とするproject/team memoryや
project-tier sensor manifestへ保存していなかった。summaryの`confirmed-content-v1`も質問file全体の生hashを
使い、固定本家の可視sectionと除外範囲を表すsemantic digestになっていなかった。review request/completionも、
declared artifact集合、既存terminal appendix、review challenge、固定audit field名を十分に束縛していなかった。

名称だけを固定本家と同じにして異なるauthority意味を持たせると、監査readerと利用者が実際より強い証拠だと
誤解する。このため、既に選択された固定本家準拠の範囲として訂正する。

## 決定

### Learnings

- mandatory learnings questionはStage全体で1回だけ実施する。
- revision時も、既に完了したlearning decision/answerはStage開始後の証拠として再利用する。
- `surface`は`<record>/<phase>/<stage>/memory.md`の固定見出しから候補を読み、Open Questionsは保存候補にしない。
- 選択されたpracticeはprojectまたはteam memoryの該当見出しへ安全かつ再実行可能に保存する。選択された
  sensorはproject-tier manifestとorigin Stage bindingを共有lock下で整合させる。
- durable writeと重複防止を確認できた場合だけ、固定fieldを持つ`RULE_LEARNED`または`SENSOR_PROPOSED`を記録する。
  何も選ばない回答でもmandatory questionの`QUESTION_ANSWERED`は残す。

### Stage epochとrevision freshness

- 通常の質問に対する`DECISION_RECORDED` / `QUESTION_ANSWERED`は、最新のcurrent-workflow
  `STAGE_STARTED(intent-capture)`からStage完了まで有効とし、`GATE_REJECTED` / `STAGE_REVISING`では一律に
  失効させない。revision中に追加したclarificationだけを新しいquestion pairとして記録する。
- summary receiptも同じStage epochを境界にする。ただし、確認対象questions contentのsemantic hashが変われば
  staleになり、変更後の内容に対する再確認を必要とする。内容が変わらないのにrejectだけを理由として再確認は
  要求しない。
- reviewは変更後artifactを対象にするため、revisionでは最新の`GATE_REJECTED` / `STAGE_REVISING`とartifact変更後の
  fresh request/completionを必要とする。
- approval応答はrevisionごとの最新`STAGE_AWAITING_APPROVAL`より後のfresh human turnへ束縛する。

### Summary confirmation

- `Hash Scope: confirmed-content-v1`は固定本家と同じsemantic questions digestを意味する。
- 可視な単一`## Consolidated Summary Confirmation`とexact `[Answer]: Looks correct`を必須にする。
- 改行を正規化し、末尾空白を除いたconfirmed contentをhashする。summary後の単一
  `## Assumption Confirmation`だけを除外し、後続の`Q<n>`と`Requested Changes Feedback`は含める。
- 重複section、重複question ID、その他の可視Markdownまたはraw HTML heading、曖昧な因果はfail closedにする。

### Review

- requestは全declared artifactのstable snapshotと、review artifactのterminal `## Review` appendix境界を束縛する。
- revision時に既存appendixがある場合は、requestを先に記録してchallengeを発行し、旧appendixを境界まで除去した
  後、同じchallengeを含む新しいterminal appendixを1つだけ受理する。
- `REVIEW_REQUESTED`と`REVIEW_COMPLETED`は固定本家のfield名と意味を使う。独自aliasをauthorityにしない。
- legacy pending-requestの`Upgrade` migrationは新規Go記録に不要なため、このverticalでは新しいreceiptとして
  誤認せずfail closedにする。通常のcurrent-attempt request/reviewとrevision challengeは対象に含める。

### Sensor

`2026-09-06-intent-capture-advisory-sensor-boundary.md`を維持する。3 sensorはgateで起動し、得られた結果を
表示・記録するが、terminal receipt、result、Note、findingはgate authorityにしない。sensorのMarkdown解析は、
固定本家を完全に判定できない入力をfalse failureへ倒し、false passを許さない。

## 実装・検証への影響

Issue #120、Codex receiver、gate read model、summary、review、sensor、learnings、fresh sandbox journeyをこの決定へ
揃える。公開`aidlc report`の4 result、`UserPromptSubmit`由来`HUMAN_TURN`の運用証拠境界、Go標準libraryのみ、
他Stageをunlockしない境界は変更しない。

独立reviewでは、固定fieldとsemantic digest、全artifact review binding、revision challenge、Stage-lifetime learning、
実際のmemory/manifest保存、sensor false-pass、公開grammar不変を確認する。差分安定後のfinal検証はIssue #120の
既存計画どおり1回だけ行う。

## 本家との差分

この訂正で新しい意図的差分は採用しない。revisionごとのlearnings再実行、audit-only learning、独自review field、
生questions hashは固定本家との差分になるため採用しない。

## 根拠

- `src/core/aidlc-common/protocols/stage-protocol.md` §1、§13
- `src/core/aidlc-common/protocols/stage-protocol-reviewer.md` §12a
- `src/core/knowledge/aidlc-shared/audit-format.md`
- `src/core/aidlc-common/stages/ideation/intent-capture.md`
- `docs/ram/decisions/2026-09-06-intent-capture-vertical-milestone.md`
- `docs/ram/decisions/2026-09-06-intent-capture-advisory-sensor-boundary.md`
- GitHub Issue #120
