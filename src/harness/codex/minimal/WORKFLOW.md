# 共通操作の索引

入口で指定した実行ファイルを `A`、Intentを `ID`、Spaceを `SPACE` と記す。
`A intent procedure ID --space SPACE` が現在段階の手順全文、担当、文書参照と遷移候補を返す。
各turnのbindが返す必須Ruleを読み、段階変更・再開後は現在手順を取り直す。
現在のrevisionは `A intent show ID --space SPACE` で確認する。

- 実行計画: `A intent plan --help`、`A intent plan-approval --help`
- 成果承認・完了・履歴: `A intent approval --help`、`A intent finish --help`、`A intent history --help`
- 設定と型・JSON例: `A intent configure --help`
- 検査・開始: `A intent check --help`、`A intent begin --help`
- 独立review: `A intent review --help`
- Unit割当・回収: `A unit claim --help`、`A unit result --help`、`A unit integrate --help`
- 質問待ち・再開・差戻し: `A intent wait --help`、`A intent resume --help`、`A intent reopen --help`
- 本文草稿からKnowledge保存: `A memory create --help`、`A memory update --help`
- clone移転: `A install codex --help`、`A unit reassign --help`

定義JSONと全段階MDのpath・本文bytesはIntent作成時に固定される。変更時は元版へ復元するか新Intentを作る。
既設配置を上書き更新しない。欠落時に旧手順や内包版を代用しない。
差戻し理由は`aidlc/spaces/<space>/knowledge/log/<intent_id>-work-log.md`へCLIがOKF（type: work-log）で保存し、設計判断のADRとは区別する。
`A memory search work-log --space SPACE --intent-id ID`で検索し、`A memory show log/ID-work-log --space SPACE`で本文を読む。
stateのrevisionが要求revisionを超えて確定するまで、記録の存在だけで成功扱いしない。generated.atは内容更新時のUTC日時であり、承認・検証済みを意味しない。
途中保存は成功ではない。同じexpect・decisionで再試行する。logが変わった場合は元版へ復元する。
非同期toolは終端までpollし、失敗終了を確認した同session/Intent/Spaceだけsession bind --recoverを使う。

## 別cloneへ移ったとき

移転先のAI会話を開始する前に端末で `aidlc install codex --help` を確認し、
`aidlc install codex --relocate --project-dir NEW_ROOT --from-project-dir OLD_ROOT --from-binary OLD_BINARY`
を実行する。旧pathは配置済み参照と一致する絶対文字列を指定する。既存SKILL原稿の編集や版更新を兼ねない。
新しい `.codex/hooks.json` の絶対pathとCodex hook trustを利用者が確認する。認証設定を自動変更しない。
部分失敗のPaths/Pendingを読み、同じ引数で全件を再検査する。未知編集を推測で上書きしない。

旧workerの終了を確認し、`unit reassign --help` のJSONで同じIntent/Unitを新しい別worktree/sessionへ割り当てる。
runningなら既存pause/resumeでneeds_confirmationにする。`previous_run_stopped: true` は確認済みの場合だけ指定し、
CLI成功まで新workerを開始しない。runtimeはGit共有しない。保存途中は他のstate更新（configureや別Unit操作など）が拒否される。同じexpect/JSONで再試行し、
既にrunningならshowと現在assignmentで成功を確認する。新run_idと現在HEADで再テスト後result/integrateする。
レビューは新root/sessionへassignし、現在targetの独立reviewを受け直す。

## 文書の選択と保存

段階の入力は `intent procedure` に示す metadata 条件と解決済み path を確認する。出力は具体 path と metadata に従う。可変の文書は `intent documents ID --space SPACE` で読み、`--expect REV --file DOCUMENTS.json` を付けて inputs/outputs 両一覧を置換する。詳細と JSON の型は `intent documents --help` で確認する。新しい出力は未存在でも登録できる。実行結果 test_results は実行後の存在するファイルだけを登録する。
現行知識の保存は `codekb/NAME`。共有解析は `codekb/current-analysis`、構成図は `codekb/architecture`。
ADR の保存は `adr/NAME`、type は `adr`。Knowledge は現行の what/how、adr は判断の why。既存共有文書の ID/日時を形式だけのために付け直さない。

初回はinitialization→discovery。他4段階の採否・順序はIntentの承認済み計画に従う。各実行のstep_idを文書・Unit・実測結果へ記し、過去の同stage成功を流用しない。計画承認と成果承認は別のrequestで、両者を提示した後の回答だけを共有できる。reopenは--stepで指定し、承認後に新IDで再実行する。

## 実行と承認の共通操作


入口の実行ファイルを `A`、Intentを `ID`、Spaceを `SPACE` とする。
各turnで `A intent switch --id ID --space SPACE --session SESSION` が返す必須Ruleを全文読む。
`A intent procedure ID --space SPACE` で現在のstep_id、手順、入力と出力を取得する。段階名だけで実行回を判断しない。
`A intent show ID --space SPACE` のrevisionを `R` とし、変更は `--expect R`。競合時は再読込する。
開始Sensorを `A intent check ID --space SPACE --boundary start` で確認し、`A intent begin ID --space SPACE --expect R` で開始する。

計画は `A intent plan ID --space SPACE` で確認する。変更は同操作へ `--expect R --file PLAN.json` を付ける。
初回initializationとdiscoveryだけは未承認のbootstrapとして開始できる。実行済み・承認済みとは扱わない。
discovery終了までに任意4段階の採否・省略理由を提示し、計画承認を得る。追加・並べ替え・省略も毎回承認する。
稼働workerは停止を確認してから計画を変更する。計画整理・読取り・質問回答は承認待ちでも行える。

終了Sensorは `A intent check ID --space SPACE`。別root・別sessionのread-only reviewerへ現在targetと実証拠を渡す。
コードを扱う段階ではreviewerのcheckoutを調整rootと同じ版・bytesにする。初期化は配置の実物を確認する。
`A intent review ID --space SPACE --expect R --file REVIEW.json` でassign後、実報告をacceptする。
assign例: {"action":"assign","coordinator_session":"main","session":"reviewer","root":"別rootの絶対path"}
accept例: {"action":"accept","session":"reviewer","root":"別rootの絶対path","target":"表示されたhash","status":"pass","summary":"実際の検証根拠"}
未実施やfailをpassへ書き換えない。対象変更後は再検査・再レビューする。

計画の判断は `A intent plan-approval ID --space SPACE --expect R --file DECISION.json`、成果の判断は `A intent approval ID --space SPACE --expect R --file DECISION.json`。
DECISIONはrequest_id、target、decision(approve/reject)、session、turn、quoteの文字列。実際のUserPromptSubmit回答から引用する。
AI自身の文を人間の回答として作らない。提示後の回答だけが対象で、過去turnや別requestへ流用しない。
discoveryの計画と成果を同時に提示した場合は同じ回答をそれぞれへ記録できる。後から作ったrequestには使えない。
`A intent finish ID --space SPACE --expect R` は現在のSensor・独立review・人間の成果承認を再確認する。
最後の選択回を完了するとIntentが完了する。計画承認だけで実行を完了しない。

## 文書と実測結果

inputsはmetadataの完全一致で選ぶ。countはone=1件、optional=0〜1件、many=1件以上。
acceptedは最も近い前の該当実行回のpath/hashを保持する。共有current文書は更新でき、同回のoutput更新も許容する。
`A intent documents ID --space SPACE --expect R --file DOCUMENTS.json` でinputs/outputs両一覧を登録する。
各要素はstep_id、stage、Space内Markdownのpath、metadata(type/title/description必須)。既存実行の成果を新しいstep_idに付け替えない。
outputsは文書のみで空も許容する。コード・テスト・ADR要否の条件は省略しない。

本文だけをsessionのdraftへ書き、metadataはCLIで生成する。引数・型は `A memory create --help` / `A memory update --help`。
`A memory create codekb/NAME --space SPACE --body-file FILE --actor process:codex --type Knowledge --title TITLE --description DESCRIPTION`
`A memory show codekb/NAME --space SPACE` のcontent/hashを確認して `A memory update codekb/NAME --space SPACE --body-file FILE --actor process:codex --expect HASH`。
一般知識は命令権限を持たない。Knowledgeは現行what/how、adrは設計判断のwhy・代替案・影響。
ADRのfolder/typeは小文字adr、新規ADRのIntent IDを保持する。既存共有文書のID/日時を形式だけのために付け直さない。
要件はdesign/ID/requirements、計画はdesign/ID/implementation-plan、共有解析はcodekb/current-analysis、構成図はcodekb/architecture。

TDD/integrationは対象commit、実測command、exit_code、非空output_pathをtest_resultsへ登録する。
結果JSON例: {"step_id":"現在の実行回ID","stage":"tdd","runs":[{"command":"実行したcommand","commit":"実際の40桁commit","exit_code":0,"output_path":"aidlc/evidence/result.txt"}]}
現在存在する結果だけを指定し、未来のファイルを事前登録しない。別stepの成功は現在回の成功として数えない。
TDDはdirect_commitまたは各UnitのResultCommit、integrationは現在HEADで必要commandの成功を確認する。
Unitはstep_id、id、bolt、base_commit、depends_on、scope、testsを明示し、新規statusはpending、結果は空。
調整役が別worktreeへworkerを起動する。依存統合・非重複範囲を確認し、claim/result/integrateのJSONにもstep_idを含める。
run_id・session・root・commitを実測と照合する。CLIはworker起動やGit統合を代行しない。

## 再開と履歴

質問待ちはwait、中断はpause、回答・条件確認後resume。非同期toolは終端までpollする。
中断workerは実runを確認してconfirmし、不明なrunを自動再実行しない。
`A intent reopen ID --space SPACE --expect R --step STEP_ID --reason TEXT` は指定回の再実行を変更案として提示する。
承認前は適用せず、承認後は旧完了回を残し新IDで実行する。計画・条件変更でEntry・review・成果承認を失効させる。
`A intent history ID --space SPACE` で計画・承認・進捗の履歴を読む。全操作auditではない。
理由はknowledge/log/ID-work-log.mdへCLIがOKF(type: work-log)で保存する。
`A memory search work-log --space SPACE --intent-id ID`、`A memory show log/ID-work-log --space SPACE` で読む。
保存途中は同じexpect・decisionを再試行する。durable pending後は時刻・前後hashが固定され、記録改変時は元版を復元する。
stateのrevisionが要求revisionを超えて確定するまで、記録の存在だけで成功扱いしない。generated.atは内容更新時刻で承認を意味しない。
定義変更時は元版へ復元するか新Intentを作る。既設ファイルを自動移行・上書きしない。

## CLI例の参照

Concept IDは拡張子なし。`adr/NAME` はtype adrで保存する。
`A memory create codekb/NAME --space SPACE --body-file FILE --actor process:codex --type TYPE --title TITLE --description DESCRIPTION`
`A memory search QUERY --space SPACE [--intent-id ID]`
Unitの実操作とJSONは `A unit claim --help`、`A unit result --help`、`A unit integrate --help`。
必要な実装計画を確認し、独立レビューは `A intent review --help` のassign/acceptを使う。

## 5担当の依頼と結果回収

メインAIがaidlc-requirementsへ目的・要件・受入条件の整理、aidlc-researcherへ根拠の調査を依頼する。
両結果が揃ったらaidlc-stage-plannerへIntent/Space、state/計画、6段階カタログ・手順、Rule、要件、調査結果、制約と成果物を渡す。
同担当はread-onlyで採否・順序・理由・省略理由・期待する文書（なければなし）・不足情報とCLI適合PLAN.json案を返す。
プログラム・テストコード・commitを文書outputsへ列挙しない。検証証拠の必要性は文書outputsとは別に説明する。
途中の追加・省略・並べ替え・reopenでも同担当へ変更理由と現在の実行回を渡し、案を回収する。
メインAIがユーザーへ説明し、最終決定を受けてintent plan/plan-approvalへ記録する。担当自身は保存・承認・子起動を行わない。
実装手順とUnit詳細はplanning、実装はaidlc-worker、独立reviewはaidlc-reviewerが担当する。
workerだけがworkspace-write、他4担当はread-only。共有state/Knowledge保存と担当起動はメインAIが行う。
