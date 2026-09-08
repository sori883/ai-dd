# 4 stage の実装詳細契約（Issue #130）

[直接承認済み計画](four-stage-workflow-implementation-plan.md)とGit共有・調整役AIの回答を具体化する。
これは新しい運用選択ではなく、同計画の型、flag、保存と検証対象の定義である。

## 共有state

`aidlc/spaces/<space>/intents/<32桁hex ID>/state.json` を UTF-8 JSON として保存する。
未知field/重複key、不正schema/ID/Space、破損、末尾の別JSONを拒否する。schema_version=1、revisionは1から増加。
IDと作成時Spaceは不変。名前は空白だけを拒否し同名を許容するが、名前解決は候補が1件だけのとき成功する。
更新は `--expect <revision>` による比較とruntime内lock、同directoryの一時fileからrenameで保存する。
失敗時は直前の正本を維持し、呼出し側へerrorを返す。lockは勝手に横取りしない。

共有fieldは `schema_version,id,space,name,revision,stage,status,reason,resume_condition,config,sensor,review`。
stageはdiscovery/planning/tdd/integration。statusはactive/waiting/paused/completed/cancelled。
configはobjective、scope、acceptance、unknowns、plan、adr、artifacts、units、code_revision。
adrはrequired、reason、refs。artifactsはpath（project相対）、kind（Knowledge/ADR/test）、stage。
unitsはid、bolt、depends_on、scope、tests、base_commit、status、result_commit、integrated_commit。
Unitのstatusはpending/running/waiting/needs_confirmation/reported/integrated。
空配列は空集合でありnullも入力では空集合として扱う。文書上限は256 KiB。

sensor/reviewは現段階のtarget、status(pass/fail)、summary。過去結果をappendする台帳は作らない。
state revision・status・結果自身はtargetへ含めず、stage、configの目的/計画/参照/Unit計画/成果commitと
実際の成果物bytes・コード版を含める。進捗statusや割当だけの変化ではtargetを古くしない。
コード版は実際のGit HEADとworktree内容から確認し、config.code_revisionはHEADの40桁commitと一致する必要がある。
共有stateとruntime自体はコード対象hashから除外し、指定された成果物は別に内容を含める。

## 段階とgate

discoveryは目的・範囲・受入条件・実装計画を妨げる未確定事項の解消、現行Knowledge参照、ADR要否とその理由を検査する。
planningは上記に加え実装計画、Unit計画（scope/test/base_commit/bolt）、未知依存/循環/重複IDを検査する。
tddは全Unit成果の統合、テスト成果物参照を要求し、integrationも同じ成果物と最終コード版を検査する。
ADR required=trueなら少なくとも1つの `aidlc/spaces/<space>/knowledge/ADR/*.md` を参照しtype ADRを検証する。
不要なら非空reasonをレビュー対象に含める。Knowledgeは同じSpaceのOKF形式と非空typeを検査する。artifact区分とOKF typeは同一視しない。
形式/存在/版の検査はSensor、意味の合格判定は独立reviewerが担う。
checkは読取り専用でtargetと不足を返す。advance時にSensorを再実行し、同じtargetのreview passを必須にする。
review fail/unknown/旧targetでは進めない。discovery→planning→tdd→integration→completedの一段階だけ進む。
`--expect`により同じadvanceの再送を拒否する。reopenは理由付きで明示stageへ戻しgateを無効化する。
waiting/pausedからresumeは理由・確認内容を要求する。paused時のrunning Unitはneeds_confirmationにする。

## ローカル担当と独立review

`aidlc/.runtime/flow/` に選択session、review割当、Unit run ID/session/worktreeを保存しGit管理から除外する。
共有state/Knowledge/ADRのwriterは指定した調整rootの調整役一人。workerの別worktreeでコピーstateを更新しない。
review割当は調整役sessionと異なるreviewer session、別の絶対root、現在targetに結び付ける。
受理は割当session/root/target一致、pass/failと非空summaryを確認する。Goはagentを起動しない。
これはAI運用上の独立性でありOS権限相手の著者認証ではない。

Unit claimはtdd/active、pending、依存全てintegrated、別worktree、基準commit一致を要求する。
同じscopeの同時claimを拒否し、run IDを生成する。結果は同じrun/session/worktreeでのみ受理する。
担当の終了だけでは後続を起動せず、調整rootで結果commitの統合を確認してintegratedにする。
Git commitは実在し、結果commitはworker基準commitの子孫、統合commitは調整root HEADと一致して結果commitを含む。
中断後はunit confirmで実在worktree/commitとrun情報を再確認する。状態消失だけで再claimしない。

## 公開CLI

共通: `--project-dir <調整root>`（省略時Git root）、`--space <space>`。
`<id>`は固定Intent ID、`<revision>`は正の十進数。flagsは重複/未知を拒否し、JSON fileの未知fieldも拒否する。
stdoutはJSON（template/rulesだけMarkdown）、stderrは診断。成功0、入力/競合2、I/O故障1。

```text
aidlc install codex --project-dir <root>
aidlc space create <name> [--project-dir <root>]
aidlc space list|switch [name] [--project-dir <root>]
aidlc intent create <name> --space <space>
aidlc intent list --space <space>
aidlc intent switch <name> --space <space> --session <session>
aidlc intent switch --id <id> --space <space> --session <session>
aidlc intent show <id> --space <space>
aidlc intent configure <id> --space <space> --expect <revision> --file <config.json>
aidlc intent check <id> --space <space>
aidlc intent review <id> --space <space> --expect <revision> --file <review-request.json>
aidlc intent advance <id> --space <space> --expect <revision>
aidlc intent pause|resume|reopen <id> --space <space> --expect <revision> --reason <text> [--stage <stage>]
aidlc unit claim|result|integrate|confirm <id> --space <space> --expect <revision> --file <unit-request.json>
aidlc memory create <concept-id> --space <space> --body-file <draft> --actor <actor> --type <type> --title <title> --description <description>
aidlc memory update <concept-id> --space <space> --body-file <draft> --actor <actor> --expect <hash>
aidlc memory show|search|check|rules ...
aidlc session inspect --session <session>
aidlc session bind <id> --space <space> --session <session> [--recover]
```

review-requestのactionはassign/accept。assignはcoordinator_session,session,root。
acceptはsession,root,target,status,summary。Unit requestはunit,session,root,run_id,commitを操作に応じ指定する。
claimはunit/session/root、resultはそれらとrun_id/commit、integrateはunit/commit、confirmはunit/session/root/run_id/commit。resultとconfirmのcommitは現在のworker HEADと一致する40桁のcommitが必須。
configureはconfig全体の置換だが実行中UnitのID/基準/依存/範囲を無断変更しない。
pauseに加えwaitingは `intent pause --stage waiting` で表現せず、`intent wait` をreason/resume_condition付きで提供する。
`intent cancel`はreason付き、completed/cancelledから通常advanceしない。

hookは選択と必須Rule全文hash、同時一般操作を検査する。毎操作の記録やKDR dirty要求はない。
対応Postで実行中slotを解放し、失敗Postが来ない編集は終了確認後の同session/Intent bind --recoverで復旧する。
Stopは作業日誌を要求せず、実行中slotがある場合だけ確認を要求する。
記録・読取り・質問・中断・復旧は固定CLI例外で妨げない。stage進行は常に上記CLIのgateを通す。

## 旧製品削除

新入口接続後、旧KDRと33 Stage専用CLI/配布原稿/package/testを依存閉包で除去する。
名前正規化、Spaceの非上書き保存、OKF metadata、CLI pipe安全性等の共通回帰検査は維持する。
参照snapshot・過去RAM・利用先dataは削除しない。移行処理・schedulerは追加しない。

## Sensor の通常詳細

`config.unknowns` は実装計画を妨げると明示された未確定事項だけを置く。一般的な疑問や将来課題はKnowledgeへ残せる。
Unit未分割の場合は `config.acceptance` と `config.tests` で直接実装の受入・検証を定義し、
TDD以降は `config.direct_commit` に検証したコード版を設定する。Unit 0件を自動合格しない。

## 固定Codex liveのGit支援

固定 workspace-write / approval=never の実証では、テストhostが専用fixtureのworktree作成・
worker編集bytesのcommit・統合を担当する。モデルは実編集と実test、調整役は実CLIによるstate・
割当・review・advanceを担当する。モデル自身のGit操作成功とは報告しない。
通常の調整役AIは実行環境で許可されたGit操作を使う。製品Goに起動・Git管理機能は追加しない。

review.status の `pending` はローカル担当を割り当て、まだ結果を受理していない状態を表す。

## 独立レビュー修正で明確化した境界

configureはUnitの計画を扱う。新Unitはpending/空結果だけで、既存のstatus/result_commit/integrated_commitは
専用Unit操作の結果を保持する。実行中・要確認・結果未回収のUnitを変更・除去しない。
Unit IDは英数字で始まる英数字・`_`・`-`、80文字以内の単一componentとする。
後続Unitのbase_commitには依存先の実integrated_commitが含まれていなければならない。

review assign/acceptはいずれも、別Git worktreeのHEADと非ignoredコードbytesを調整rootと照合する。
`aidlc/` の共有文書は調整rootから読むため、review rootへ全複製しない。
artifactはkind/stage/pathを検査し、現在までの段階だけ存在・内容を要求する。
将来stageの定義も対象hashに含むが、まだ存在しなくてよい。state/runtimeはartifactにできない。

配置skillは日本語の4KiB以下の入口と、選択・Rule全文読込後に通常file読込で到達する
`.agents/skills/aidlc/WORKFLOW.md` に分ける。上限超過時の切捨てや上限緩和はしない。

test-only liveは、Pre時点のreview requestとモデルJSONLのcommand_executionの実exit/outputを照合する。
実review報告のsession/root/target/status/summaryとCLI受理結果を一致させる。
rawにstale診断文字列があるだけ、echo出力、別報告や別対象は証拠にしない。
reviewer checkoutのcommitとbytes hashを保持し、review中の変化も拒否する。
