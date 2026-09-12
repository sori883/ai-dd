---
name: aidlc-cli
description: AI-DLC CLIの操作目的からコマンドとhelpを選ぶ。Intent、文書保存、Unit、復旧や配置移転の操作時に使う。
---
# 操作を選ぶ

実行ファイルは @@BINARY@@（以下A）。進行と承認の規約は [aidlc](../aidlc/SKILL.md)、現在工程は `A intent procedure ID --space SPACE`。以下から目的を選び、変更前に該当helpで引数・型・値・JSON例を確認する。実際のID/Space/sessionとshowのrevisionを使い、競合時は再読込する。

| 目的 | help |
| --- | --- |
| Space作成・選択、Intent作成・選択 | `A space --help`、`A intent create --help`、`A intent list --help`、`A intent switch --help` |
| 現在状態と手順 | `A intent show --help`、`A intent procedure --help` |
| 段階の採否・順序と計画承認 | `A intent plan --help`、`A intent plan-approval --help` |
| 設定・文書宣言・実測結果 | `A intent configure --help`、`A intent documents --help` |
| 開始/終了Sensorと開始 | `A intent check --help`、`A intent begin --help` |
| 独立review・成果承認・完了 | `A intent review --help`、`A intent approval --help`、`A intent finish --help` |
| 質問待ち・中断・再開・差戻し・履歴 | `A intent wait --help`、`A intent pause --help`、`A intent resume --help`、`A intent reopen --help`、`A intent history --help` |
| worker場所の登録・起動前照合・解放・復旧 | `A assignment --help`、`A assignment init --help`、`A assignment reserve --help`、`A assignment check --help`、`A assignment release --help`、`A assignment reset --help` |
| Unit割当・回収・統合・中断確認 | `A unit claim --help`、`A unit result --help`、`A unit integrate --help`、`A unit confirm --help` |
| Knowledge/ADRの作成・更新・読取り | `A memory create --help`、`A memory update --help`、`A memory show --help`、`A memory search --help` |
| session読込み・復旧、配置・移転、Unit移転 | `A session bind --help`、`A install codex --help`、`A unit reassign --help` |

## 文書と実測

Knowledgeは現行what/how、ADRは判断のwhy・代替案・影響、進捗はstate。毎操作の日誌や一律ADRは作らない。必要なADRを作り、不要なら理由をreviewする。
outputsは期待する文書だけで、なければなし。プログラム・テストコード・commitを文書outputsへ列挙せず、検証証拠は別に説明する。共有文書のID/日時を形式だけのために更新しない。

procedureのmetadata条件と解決path/版を読み、documentsでinputs/outputs両一覧を置換する。新文書は未存在でも宣言できるが、実測test_resultsは実行後に存在する結果だけを指定する。accepted入力は前回合格のpath/hashを保持し、変更にはreopenが必要。共有currentと同回outputは更新できる。各宣言・Unit・実測へ現在step_idを使う。
本文だけをsessionのdraftへ書き、memory CLIでmetadataを生成する。Concept IDは拡張子なし。Knowledgeはcodekb/NAME、ADRはadr/NAMEでtype adr。要件はdesign/ID/requirements、実装計画はdesign/ID/implementation-plan、共有解析はcodekb/current-analysis、構成図はcodekb/architecture。新規ADRのIntent IDを保持する。update前にshowのcontent/hashを確認する。
TDD/integrationではverification_pathsへコード・テスト・設定・共通部品の適切な範囲を指定する。`A intent hash ID --space SPACE` →テスト→hashの前後一致を確認し、step_id/stage/verification_scope/verification_sha256/runsを持つ結果JSONをaidlc/evidenceへ保存してtest_resultsへ登録する。runはunit_id/command/exit_code/output_path。Unit結果はトップレベルunit_id/run_idも対応させる。最終検証は最新Intent全体SHAの各Unit+commandの成功を要求する。個別ファイルSHAは登録しない。

## 承認と復旧

reviewは別session（同root可）へassignし、実報告をacceptする。コードを扱う段階の別reviewer rootは同じverification_pathsの集合SHAにする。plan-approval/approvalは表示されたrequest_id/targetと実回答のsession/turn/quoteを使う。計画整理・読取り・質問回答は承認待ちでもできる。
reopenは--stepで対象を指定する。保存途中は同じexpect・decisionを再試行する。durable pending後は時刻と前後hashが固定され、log改変時は元版を復元する。記録の存在だけで成功扱いせずstateのrevision確定を確認する。generated.atは更新日時で承認を意味しない。work-logはmemory search work-log、memory show log/ID-work-logで読む。
移転前に端末からinstall codex --helpを読み、--relocateへ旧root/binaryの配置済み絶対文字列を渡す。両skillとhooksの既知参照だけを移し、版更新や未知編集の上書きを兼ねない。利用者が新hooksの絶対pathとCodex trustを確認する。部分失敗はPaths/Pendingを見て同じ引数で再検査する。
Unit移転は旧run停止を確認してからreassignする。runningはpause/resumeでneeds_confirmationにし、previous_run_stoppedは確認時だけtrue。保存途中は同じexpect/JSONで再試行する。新runで再検証し、現在verification_sha256と結果を提出する。CLIはworker起動やファイル移送を代行しない。

## native担当の登録

メインAIが共有stateの単独writerとなり、Ruleと必要な入力を担当へ渡して結果を回収する。aidlc-requirementsが要件、aidlc-researcherが根拠を調べ、その後と計画変更時にaidlc-stage-plannerが採否・順序・理由とPLAN案を返す。planningは実装手順とUnit詳細を扱う。aidlc-workerは承認・割当後に登録された通常rootで担当範囲を実装し、verification_sha256と結果を返す。依存統合前や重複範囲で開始しない。aidlc-reviewerは固定成果を別sessionで（同rootを許可）独立に確認する。worker以外はread-only。子担当は共有state/Knowledge/ADRを保存せず報告・本文案を返す。他者編集を保全し、回答を捏造しない。

子担当は渡されたRule全文と入力を使い、親のsession bind/intent switchや共有state更新を代行しない。子の通常作業は担当指示とsandboxに従い、管理変更や追加担当の必要はメインAIへ返す。

CLIは起動しない。メインAIがprocedureの担当と登録task_nameをnative spawn_agentへ渡す。fork_turnsが提供される場合はnoneとし、Rule全文と必要な入力だけを渡す。worker登録は同一管理root全Space/Intent/sessionで競合を検査する。実在する通常ディレクトリを使い（管理rootも可）、同一・親子rootは順次割り当て、、Unitなしreserveも開始条件を満たす。Unit claim/reassignにはregistry_epoch/request_id/coordinator_sessionを含める。担当のsessionは割当識別子で実child IDではない。
追加依頼はcheck後、応答確認済みの相対task_nameへfollowup_task/send_messageを使う。Post欠落では再依頼・同名再spawnをせず不明として保持する。interruptは停止要求だけで解放ではない。メインAIが追加依頼終了・既知コマンドとbackground終了・成果残件回収を確認してreleaseする。不明なら人間へ確認する。init/resetは既知作業を整理した人間の回答と理由を記録する。原本復元を優先し、旧Unitへ予約を後付けしない。

子からの途中報告は、メインAIが渡した親の正式task名（例 `/root`、`/root/phase`）をsend_messageのtargetへ完全一致で指定する。現在工程の同じroleのbound記録と親宛の一意性を検査する宛先制限であり、agent_idと特定予約の本人認証ではない。親以外への通知、再委譲、共有保存は許可しない。初回Post待ちで拒否された場合は、親のbinding完了後に同じ子から報告を再送し、再spawnや自動解放はしない。

一般Toolが残る場合は、非同期処理を終端までpollし、成功・失敗を問わず実際の終了を確認する。同じSpace・Intent・sessionでメインAIが `A session bind ID --space SPACE --session SESSION --recover` を実行できる。固定Codex 0.153.4では失敗patchにPostがないため、終了確認とこの明示復旧を使う。時間やStopだけで解除せず、workerの割当解放とは別に扱う。

Gitは利用時に不要。明示--project-dirを優先し、省略時は上方向のaidlc/workflow/stage-graph.jsonで管理rootを探す。候補が複数なら明示する。検証集合は管理aidlcと全深さ.gitを除き、symlink・特殊file・範囲外参照を拒否する。10,000ファイル/256 MiBが上限。範囲の十分性を計画と独立reviewで確認する。レビュー中は編集を止める。
