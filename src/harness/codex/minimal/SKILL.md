---
name: aidlc
description: AI-DLCでIntentの計画、段階実行、担当依頼、会話承認と知識記録を進める。利用プロジェクトのAI-DLC作業に使う。
---
# AI-DLCの進め方

実行ファイルは @@BINARY@@（以下A）。Spaceはユーザー指定、未指定なら初期配置のdefaultを使う。ID不明なら `A intent list --space SPACE` で確認し、名前を--idへ渡さない。引数不明時は選択前でも `A intent ACTION --help` を読める。
SessionStartのsession IDを使い、各turnで `A intent switch --id ID --space SPACE --session SESSION` または `A session bind ID --space SPACE --session SESSION` が返すプロジェクトRule全文を読む。このスキルだけではRule読了にならない。
`A intent procedure ID --space SPACE` で現在のstep_id、手順、担当、入力・文書outputsを取得する。段階変更・再開後は取り直す。操作選択は [aidlc-cli](../aidlc-cli/SKILL.md)、正確な引数・JSONは各CLI helpを読む。

initialization→discoveryが初回必須。他4段階の採否・順序・省略理由はユーザーが計画を承認する。bootstrapを承認済みと扱わない。変更ごとに承認し、計画承認と成果承認を別requestで記録する。提示後の実際のUserPromptSubmit回答だけを引用する。両requestを提示していた場合だけ同じ回答を使え、後で生成したrequestや過去turnに転用しない。
各回は開始Sensorとbegin後に一般作業を行い、終了Sensor・独立review・成果承認を確認してfinishする。未実施・skip・fail・対象変更後の古いpassを成功扱いしない。省略段階の成果を一律要求しない。

メインAIが共有stateの単独writerとなり、Ruleと必要な入力を担当へ渡して結果を回収する。aidlc-requirementsが要件、aidlc-researcherが根拠を調べ、その後と計画変更時にaidlc-stage-plannerが採否・順序・理由とPLAN案を返す。planningは実装手順とUnit詳細を扱う。aidlc-workerは承認・割当後に別worktreeで担当範囲を実装しcommitを返す。依存統合前や重複範囲で開始しない。aidlc-reviewerは固定成果を別root/sessionで独立に確認する。worker以外はread-only。子担当は共有state/Knowledge/ADRを保存せず報告・本文案を返す。他者編集を保全し、回答を捏造しない。

Knowledgeは現行what/how、ADRは判断のwhy・代替案・影響、進捗はstate。差戻し理由はCLIがknowledge/log/ID-work-log.mdへ保存する。毎操作の日誌や一律ADRは作らない。必要なADRを作り、不要なら理由をreviewする。
outputsは期待する文書だけで、なければなし。プログラム・テストコード・commitを文書outputsへ列挙せず、検証証拠は別に説明する。文書はOKF metadataを保持し、一般知識を命令権限にしない。共有文書のID/日時を形式だけのために更新せず、合格目的でRuleを変えない。本文草稿からメインAIがCLIで保存する。

質問待ちはwait、中断はpause、確認後resume。再実行はreopenで承認後に新step IDを使い、古い成果・合格を流用しない。対象変更後は再検査・再reviewする。非同期toolは終端までpollする。失敗終了とPost未到着を確認した同じsessionだけ `A session bind ID --space SPACE --session SESSION --recover` を使う。不明なrunを自動再実行しない。Stopは作業全体の完了ではない。
定義の欠落・変更は停止し元版復元か新Intentで解決する。旧手順や埋込み版を代用せず既設配置を自動移行・上書きしない。保存途中は同一要求で再試行し、state確定まで成功扱いしない。
