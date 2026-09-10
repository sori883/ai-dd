# 通常hookで初期化を完了し、製品担当で目的整理を進める

日付: 2026-09-11。Issue #163。対象は `intent check --help` を初心者向けに改善する実案件。
変更予定は `src/internal/cli/help.go` と新規 `src/internal/cli/check_help_test.go` の2file。
本記録時点では製品コードを変更しておらず、実利用フローの要件と計画を整理している。

## 初期化の実承認

ユーザーの「OKです。」は、既に提示していた初期化成果・専用環境の通常hook信頼・初回担当管理の
開始への回答として記録した。出典は [承認RAM](2026-09-11-pilot-initialization-approved.md)。
assignment initは実際に成功し、初回epochは `a2e13b2a2fa345a1c5a90d0c0388b3cd`。

hook探索問題を解決するため、専用環境を同じpathの独立Gitコピーへ補正した。内容・mode・symlinkの
1,339fileとHEADを維持し、通常 `/hooks` で製品5イベントを確認・信頼した。
経緯は [探索と補正](2026-09-11-pilot-hook-discovery.md)。元checkoutの未コミット資料は変更していない。

通常Codex 0.153.4 / gpt-6-astra xhighの実会話がSessionStartを受け、Ruleを読んでIntentをbindした。
実回答を説明済みの方式で中継し、次の実UserPromptSubmitを根拠としてapproval CLIへ保存した。

- 専用root: `/Users/const/sori883/ai-dd-latest-pilot`。
- Space: `workflow-pilot`、Intent: `915a62dd6391fbab3e08a5707309d687`。
- 受信session: `01a08be5-f6fa-7490-9aca-4729726cd117`。
- 受信turn: `01a08bea-5fb7-7513-9526-9257d878bff8`。
- 初期化request: `82fbc4a54de98997c7252ea3e2338224`、引用: `OKです。`。
- revision5で承認、revision6のfinishでs01がcompleted、s02 discoveryがpendingになった。

これは出典付きの回答中継の実測であり、Codexが元の人間本人を認証したとの意味ではない。
synthetic hook、state直接編集、trust bypassは使っていない。後から作る承認要求へ回答を転用しない。

## 製品担当による目的整理

同じ実会話でdiscoveryを開始し、メインAIがnative spawn_agentを使って次を起動した。
製品CLIは担当を起動していない。実runtimeのdispatchは現在Intent/step s02/定義hashへ結び付き、
Postでcanonical task pathがboundとなった。

| 製品担当 | task path | 役割 |
| --- | --- | --- |
| aidlc-researcher | `/root/discovery_research` | 現行Sensor・help・段階定義の文書契約を調査 |
| aidlc-requirements | `/root/discovery_requirements` | 目的・範囲・要件・受入条件を整理 |
| aidlc-stage-planner | `/root/discovery_stage_plan` | 全6段階を使う理由と順序・文書outputを提案 |
| aidlc-reviewer | `/root/discovery_review` | 固定した成果を別rootで独立review |

子はread-onlyで本文案・判断を返す。共有stateと文書の保存はメインAIがCLIだけで行う。
要件は `aidlc/spaces/workflow-pilot/knowledge/design/915a62dd6391fbab3e08a5707309d687/requirements.md`。
type Requirements、現在intent_id、generatedの日時等はmemory CLIが生成した。
本文は6段階の文書型、metadata、必須見出し、共有版・受入済み版、実測結果と人間承認を説明する。
ADRはアーキテクチャ判断を変更しないため不要とした。新しい製品仕様の選択はない。

資料登録で開始時の入力記録を取り直し、開始Sensorとbeginを再実施した。
要件と文書宣言・段階計画を保存後、終了Sensorはpass。コード版とbytesの一致を検査するreview assignも成功。
独立review rootは `/Users/const/sori883/ai-dd-latest-pilot-review`、対象HEADは `1ae3578`。

計画案はs01初期化、s02目的整理、s03構成分析、s04実装計画、s05TDD、s06統合検証。
全6段階を選ぶのは今回の実案件で既存機能の組合せを観測するためで、help変更一般に強制する仕様ではない。
計画requestは `1e78d418a9299223118ad3bf6d064e30`、targetは
`6e5db64fba0e2d7a8acc25054b324a94aae107bdaf31e179c6fdde50614103b1`。

独立reviewは固定targetでpass、指摘なし。reviewerは同じHEADとcode digest、要件・資材のhash、
終了Sensorを確認した。先行担当の実行ログを再reviewしたとの主張や、後続実装の検証成功は含めない。
子の実session UUIDはnative toolから公開されず、review登録sessionはtaskに対応する独立識別子として扱った。

revision13、s02 discoveryは成果承認待ち。計画と成果の二つの承認要求がpendingである。

- 計画request: `1e78d418a9299223118ad3bf6d064e30`。
- 計画target: `6e5db64fba0e2d7a8acc25054b324a94aae107bdaf31e179c6fdde50614103b1`。
- 成果request: `503d378cfd554a4db61aae03aa7213e6`。
- 成果target: `5006d0f53f75c8e2cb6c54b7d68b1bddacc8f3cd9bdd37f9b9210a8e7c7ad412`。

両要求を要件文書・実行順・理由とともに人間へ提示し、新しい回答を待つ。
初期化に対する回答を再利用せず、後続成果の承認も各回で行う。
後続の構成分析・planning・TDD・統合検証、helpコード変更、final、PR/mergeは未実施。

## 観測した運用上の課題

読取りcommandを並列に呼ぶとsession lock競合が発生し、completed/exit0のcatがTool枠に残った。
実transcriptのtool IDと終端を照合してから、同sessionの公開bind --recoverで復旧した。
以後のshell呼出しは直列で完了させている。Post不達とPost処理失敗は今回の記録だけでは区別できない。
並列読取りの製品側修正は未実施であり、日常運用の改善候補に残す。

子担当から親への途中通知もhookに拒否されたが、最終報告は回収できた。
実際の通知経路と意図した子の制限の関係は追加調査が必要。これを通常の中間報告成功とは数えない。
開発用agent3件のinvalid transportとLinear MCPの再認証要求は、製品aidlc-*の起動とは区別する。
外部tool導入、認証変更、Codex更新、Go module追加は行っていない。

## 開発記録と検証対象の分離

製品Sensorはaidlc外のGit追跡対象・未追跡fileとHEADをtargetに含める。レビュー後に同じrootのRAMを
書き換えると承認対象が変わるため、この後続RAMは同repoの別checkout
`/Users/const/sori883/ai-dd-latest-pilot-records`（branch `codex/latest-pilot-records`）へ保存する。
現在の調整rootとreview rootは固定し、承認待ちの対象を維持する。
実案件完了後に開発PRへこの記録branchを統合し、製品の実測履歴と開発記録を混在させない。
これは同時writerの回避と証拠保全のための実施詳細で、製品のstate仕様は変更しない。

生証拠は専用rootの `aidlc/evidence/latest-pilot/20-*`（bind）、`21-*`（tool終端）、
`22-*`（実回答・承認・finish）、`23-*`（製品担当とdiscovery）へ保存。
新規製品コードのTDDや全体final検証の成功を、この記録で主張しない。
