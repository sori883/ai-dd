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

procedureのmetadata条件と解決path/版を読み、documentsでinputs/outputs両一覧を置換する。新文書は未存在でも宣言できるが、実測test_resultsは実行後に存在する結果だけを指定する。accepted入力は前回合格のpath/hashを保持し、変更にはreopenが必要。共有currentと同回outputは更新できる。各宣言・Unit・実測へ現在step_idを使う。
本文だけをsessionのdraftへ書き、memory CLIでmetadataを生成する。Concept IDは拡張子なし。Knowledgeはcodekb/NAME、ADRはadr/NAMEでtype adr。要件はdesign/ID/requirements、実装計画はdesign/ID/implementation-plan、共有解析はcodekb/current-analysis、構成図はcodekb/architecture。新規ADRのIntent IDを保持する。update前にshowのcontent/hashを確認する。
TDD/integrationは現在回のcommit・command・exit_code・output_pathをconfigureへ記録する。TDDはdirect_commitまたはUnitのResultCommit、integrationは現在HEADを検証する。Sensorは形式・版・command成功を確認し、真正性とRED/GREENの意味はreviewerが確認する。

## 承認と復旧

reviewは別root/sessionへassignし、実報告をacceptする。コードを扱う段階のreviewer checkoutは調整rootと同じ版・bytesにする。plan-approval/approvalは表示されたrequest_id/targetと実回答のsession/turn/quoteを使う。計画整理・読取り・質問回答は承認待ちでもできる。
reopenは--stepで対象を指定する。保存途中は同じexpect・decisionを再試行する。durable pending後は時刻と前後hashが固定され、log改変時は元版を復元する。記録の存在だけで成功扱いせずstateのrevision確定を確認する。generated.atは更新日時で承認を意味しない。work-logはmemory search work-log、memory show log/ID-work-logで読む。
移転前に端末からinstall codex --helpを読み、--relocateへ旧root/binaryの配置済み絶対文字列を渡す。両skillとhooksの既知参照だけを移し、版更新や未知編集の上書きを兼ねない。利用者が新hooksの絶対pathとCodex trustを確認する。部分失敗はPaths/Pendingを見て同じ引数で再検査する。
Unit移転は旧run停止を確認してからreassignする。runningはpause/resumeでneeds_confirmationにし、previous_run_stoppedは確認時だけtrue。成功まで新workerを開始せず、保存途中は同じexpect/JSONで再試行する。新run_idとHEADで再テストし、reviewも新root/sessionへassignし直す。runtimeはGit共有しない。CLIはworker起動やGit統合を代行しない。

## native担当の登録

CLIは起動しない。メインAIがprocedureの担当と登録task_nameをnative spawn_agentへ渡す。fork_turnsが提供される場合はnoneとし、Rule全文と必要な入力だけを渡す。worker登録は同一管理root全Space/Intent/sessionで競合を検査する。別の実在worktreeを用意し、Unitなしreserveも開始条件を満たす。Unit claim/reassignにはregistry_epoch/request_id/coordinator_sessionを含める。担当のsessionは割当識別子で実child IDではない。
追加依頼はcheck後、応答確認済みの相対task_nameへfollowup_task/send_messageを使う。Post欠落では再依頼・同名再spawnをせず不明として保持する。interruptは停止要求だけで解放ではない。メインAIが追加依頼終了・既知コマンドとbackground終了・成果残件回収を確認してreleaseする。不明なら人間へ確認する。init/resetは既知作業を整理した人間の回答と理由を記録する。原本復元を優先し、旧Unitへ予約を後付けしない。
