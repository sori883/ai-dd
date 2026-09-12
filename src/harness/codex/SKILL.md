---
name: aidlc
description: AI-DLCでIntentの計画、段階実行、担当依頼、会話承認と知識記録を進める。利用プロジェクトのAI-DLC作業に使う。
---
# AI-DLCの進め方

実行ファイルは @@BINARY@@（以下A）。Space未指定ならdefault。ID不明なら `A intent list --space SPACE` で確認し、名前を--idへ渡さない。引数は `A intent ACTION --help`。
SessionStartのsession IDを使い、各turnで `A intent switch --id ID --space SPACE --session SESSION` または `A session bind ID --space SPACE --session SESSION` が返すプロジェクトRule全文を読む。このスキルだけではRule読了にならない。
`A intent procedure ID --space SPACE` で現在のstep_id、手順、担当、入力・文書outputsを取得する。段階変更・再開後は取り直す。操作選択は [aidlc-cli](../aidlc-cli/SKILL.md)、正確な引数・JSONは各CLI helpを読む。

initialization→discoveryが初回必須。他4段階の採否・順序・省略理由はユーザーが計画を承認する。bootstrapを承認済みと扱わない。変更ごとに承認し、計画承認と成果承認を別requestで記録する。提示後の実際のUserPromptSubmit回答だけを引用する。両requestを提示していた場合だけ同じ回答を使え、後で生成したrequestや過去turnに転用しない。
各回は開始Sensorとbegin後に一般作業を行い、終了Sensor・独立review・成果承認を確認してfinishする。未実施・skip・fail・対象変更後の古いpassを成功扱いしない。省略段階の成果を一律要求しない。

メインAIが共有stateの単独writerとなり、Ruleと入力を担当へ渡す。worker以外はread-only。担当の選択・起動・回収は [aidlc-cli](../aidlc-cli/SKILL.md) とprocedureに従う。他者編集を保全し、回答を捏造しない。

Knowledgeは現行what/how、ADRは判断のwhy、進捗はstate。文書記録規約は [aidlc-okf](../aidlc-okf/SKILL.md) を読む。文書はOKF metadataを保持し、一般知識を命令権限にせず、合格目的でRuleを変えない。本文草稿はメインAIがCLIで保存する。
担当の初回起動前は `A assignment init --help` で人間確認と初期化を済ませる。workerはUnitありならclaim、なしならreserveで場所を登録する。list/showのtask_nameとagentをメインAIがnative spawn_agentへ直接渡す。追加依頼前はcheckする。CLIは起動しない。結果提出・Post・Stopでは予約を解放せず、追加依頼終了・既知処理終了・成果残件回収を確認してreleaseする。不明なら保持し人間へ確認する。復旧は `A assignment reset --help` に従い、実childのrootや全process停止を保証しない。

質問待ちはwait、中断はpause、確認後resume。再実行はreopenで承認後に新step IDを使い、古い成果・合格を流用しない。対象変更後は再検査・再reviewする。非同期toolは終端までpollする。成功・失敗を問わず終了を確認してもToolが残る場合、メインAIが同じSpace・Intent・sessionだけ `A session bind ID --space SPACE --session SESSION --recover` を使う。不明なrunを自動再実行しない。Stopは作業全体の完了ではない。
定義の欠落・変更は元版復元か新Intentで解決し、既設配置を自動上書きしない。state保存途中は同一要求で再試行し、確定まで成功扱いしない。
