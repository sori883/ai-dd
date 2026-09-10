# 委任した試験用回答と通常hookの実測

日付: 2026-09-11。Issue #163。実案件はrevision45で完走。開発側の最終検証とPRの結果は別gateとして管理する。
許可は [今回限定の承認役委任](2026-09-11-pilot-ai-approval-delegated.md)。
製品仕様は変えず、信頼済みのCodex 0.153.4で同じpilot会話を再開した。

## 承認待ちでの拒否

revision13、s02 discoveryで次を各1回だけ試した。

- `touch pilot-unapproved-probe.txt` は実PreToolUseにより `human approval pending` で拒否。
  直後の読取りと外側AIの確認でfile非存在。別経路の書込みは試さなかった。
- native `spawn_agent` の `aidlc-worker` / `discovery_denied_worker` は
  `agent is not allowed in current stage` で起動前拒否。担当一覧は空で、dispatchを作らなかった。

試験後も二つのpending要求とrevision13を維持し、Session.Toolは空だった。
当該Codex実行はexit0で終端を確認。実際の要求・拒否応答はtask専用rolloutから抽出した。
canaryの要求はcustom_tool_call内のexecで、native spawnはfunction_callだった。
最初の抽出scriptが両者をfunction_callとして扱ったassert失敗は、抽出処理を修正して解決した。
これは製品のREDやhook失敗として数えない。

## 試験用回答による再開

外側AIが提示済みの要件・計画・終了Sensor・独立reviewを確認し、新しい通常会話入力を送った。
引用は「AIによる試験用承認です。提示済みの計画とs02 discovery成果の両方を承認します。」。
実受信sessionは `01a08be5-f6fa-7490-9aca-4729726cd117`、turnは
`01a08c6a-a6cc-7df2-92f0-b5fb5f09ef1d`。

計画request `1e78d418a9299223118ad3bf6d064e30` はrevision14、
成果request `503d378cfd554a4db61aae03aa7213e6` はrevision15でapprovedとなった。
通常finishはrevision16でs02を完了し、s03 architecture-analysisへ進んだ。
親も実commandのexit0、保存quote/turnとhookによる入力捕捉を照合した。
人間本人が個々の成果を確認した証拠とは扱わない。

## 証拠と現在の限界

s03の試験用回答はrevision21、finishは22。受信turnは `01a08c83-2a84-7972-bdda-43bb01d371ae`。
s04はrevision23でbeginしたが、外側handoffがステージ選択専用aidlc-stage-plannerへ実装手順を依頼し、
実task `/root/implementation_plan` は担当範囲外として返却した。revision24で通常waitへ移り、Tool空で終了。
担当定義は既存どおりステージ採否・順序専用であり、planning全作業の専任という意味ではない。
外側の依頼先選択を訂正し、メインAIがImplementationPlan本文を作り独立reviewへ渡す回答を新turnで送った。
既存のメインAIの共有writer責務とplanning手順から決まる実施詳細であり、製品の役割や実行計画を変更しない。
この質問待ちを人間へ再度転送せず、今回の試験用利用者役が回答した。
実装用worktree `ai-dd-latest-pilot-worker` / branch `codex/latest-check-help-worker` はHEAD1ae3578で作成済み。
作成時cleanを確認し、workerの予約・起動・編集はTDD開始後に行う。

s03ではnative researcherとreviewerを起動し、実要求・応答とregistryの2dispatchを親も照合した。
CurrentAnalysis（hash `34410f7e7a427b8f8e716450e91649c7fac7eae0140d1f1d8a1f292e86b6e353`）と
Architecture（hash `6748baecef1a652397435f0f2107eefcbf88560328a07771233ff5689a97b25a`）をmemory CLIで保存。
終了Sensorと別rootのreviewはpass、指摘なし。両rootはHEAD1ae3578と830個の非aidlcファイルdigestが一致。
revision20でrequest `e12be7b69df701bf5e57eb535e073d1c`、target
`6b44922e82e9ec0ab684de721742a76ac935a77908abbecc8a0ded06a0d19892` の成果承認待ちとなった。
外側AIは2文書と実reviewを確認してs03限定の新しい試験用回答を送り、planningへ継続している。

s03途中ではRule再読込要求に通常bindで対応した。また、同じdraftへのDelete/Addを1patchで重ねた
形式エラーが終了した後にTool枠が残り、終端を確認した同一sessionの公開bind --recoverで復旧した。
この編集失敗はGoのREDではない。以後はdraftのUpdateを使う。後続操作・review割当は成功し、
当該Codex実行のexit0と最終Session.Tool空を確認した。通知の実採用や全process停止について、
文書の静的reviewだけで確認済みとはしていない。

専用rootの `aidlc/evidence/latest-pilot/27-denial-probes.*`、`27-denial-observations.json`、
`28-discovery-approval-architecture.*`、`28-approval-observations.json`、`28-native-dispatch-evidence.json` に生証拠を保持する。
後続段階は進行中。完走、Go実装、final成功をこの時点で主張しない。
並列読取りのTool残存と子の途中通知拒否という既知の課題も、解決済みとは扱わない。


## planning完了と実workerのTDD

s04は通常resumeでrevision25へ戻り、メインAIがImplementationPlanを作成した。
終了Sensorと別rootのnative reviewerはpass。外側AIが本文と実reviewを確認し、
新しい通常入力「AIによる試験用承認です。提示済みs04 planning成果を承認します。」を送った。
受信turnは `01a08c96-9d85-7721-b7ee-c081b991e646`。承認30、finish31、s05 begin32となった。
実装手順の依頼先訂正は既存役割の範囲内で行い、製品の担当定義や計画順序は変更していない。

メインAIがUnitなしのworker場所をreserveして、通常のnative spawn_agentでaidlc-workerを起動した。
割当は `5a4f3488cf72a9ebbb44c59d425f6c35`、workerは別worktreeの所有2fileだけを変更した。
Goの3項目は、段階と証拠、文書条件、完了手順とhelp形式の順に実行した。
各項目で新testを追加して実行可能なassertion failureを確認し、その後helpを変更して成功した。
compile failureや環境障害をREDへ数えていない。既存help形式等の成立はALREADY_GREENとして区別した。
末尾の見出し比較補強はtestだけの変更で、再実行は成功。source hashと実native tool時系列も親が照合した。

CLI package全体の検査は `Library/Caches/go-build` へのアクセス制限で開始できなかった。
workerは3項目の証拠と差分を保持し、commit未作成で返した。既知非同期shellは全て終端回収済み、
未回収backgroundなし。結果提出だけで予約を解放せず、親の残件確認までboundを維持した。

外側親は所有2fileの全差分・test先行時系列・実RED出力を確認した。
ソースを追加変更せず成果commit `21e755aa0ebe09699c5635a8403d5e7e1eb93e6a` を作成し、
一意の一時GOCACHEを指定して同commitのtargetedとCLI package検査を実行した。両方exit0、前後source hash不変。
これは環境内のキャッシュ場所変更であり、権限・hook trustの変更ではない。
調整rootへfast-forwardし、review rootも同commitへ同期した。
非aidlcの831fileのbytes/modeが一致することを確認して、通常会話から予約解放・実測登録・独立reviewへ進めた。
この時点でTDD成果承認・integration・finalは未完了。

## 子の実sessionと証拠の対応

native spawn応答はtask pathを返し、実child UUIDは返していない。
今回、同じ親sessionに属するローカルrolloutの先頭metadataだけを読み、
`source.subagent.thread_spawn` のparent_thread_id・agent_path・agent_roleから実child UUIDを一意に対応付けた。
workerは `01a08c9a-d23e-7ba0-be66-8e5d54721121`。native APIがUUIDを返したとは扱わない。
metadataのcwdは親rootなので、その値をworkerの実作業場所の証拠には使わない。
workerの実tool workdirとrecorderのcwd確認を別に照合した。
保存するmetadataは識別項目と先頭行hashに限定し、base instructionsや内部推論は抽出しない。

31の通常会話記録、`32-worker-tdd-tool-evidence.json`、`32-parent-boundary/`、
`32-review-checkout-manifest.json` を専用rootの証拠配下に保持する。
workerの各RED/GREENの実出力・sourceコピー・時刻・hashは `/tmp/ai-dd-check-help-hipulifz/` にある。
一時証拠は消失し得るため、PR前に必要な実測証拠を専用rootへ保全する。


## TDDの独立レビューと統合検証への遷移

native `/root/tdd_code_review` はcommit21e755aの所有2fileと、実RED/GREENの各snapshot、
元rolloutの13tool要求、親の成果commit上の再検査を照合し、pass・指摘なしとした。
workerの非同期shell7件は終端応答まで確認した。公開releaseはentry_revision4でreleasedとなった。
831fileのbytes/modeとHEADはreview開始・終了とも指定manifestに一致した。
この実reviewを開発側の独立コードレビューとしても採用する。
Issue本文の再取得はreviewer環境のAPI接続失敗で未確認だったため、親が取得した実本文JSONを
後続integrationのreviewへ渡す。その他の承認済み計画・RAM・受入文書・実承認は照合済みである。

s05はrevision35でrequest `7dc1c612a1d9c17031d63bd1c635663f`、target
`ad1d0a2622bfe2b8ae9b133904f541b8f4384f32c974e0c3fb0c3262fe599956` の成果承認待ちとなった。
親が実最終報告と成果を確認し、新しい通常入力で「AIによる試験用承認です。提示済みs05 tdd成果を承認します。」
を返した。approvalはrevision36、finishは37、s06 integrationはbegin38で開始した。

統合HEADも21e755a。targeted、CLI package、新binaryのbuild、2形式のnative check helpを実測し、
全5commandがexit0だった。helpのstdoutは一致しstderrは空、必須2段階・選択4段階と文書条件も確認した。
buildにはGoのstat cache書込み警告があり、そのまま実stderrへ保存した。権限やtrustは変更していない。
信頼済みhook用binaryを上書きせず、新helpは別名aidlc-check-helpで実行した。
Knowledgeはmemory CLIで保存し、intent_id指定検索とshowで取得した。現時点ではintegrationの
独立review・成果承認・finishと、開発側final・PR/mergeは進行中である。


## 最終到達点：同じIntentが6段階を完了

s06の独立reviewもpass・指摘なし。実測記録scriptと元rolloutの終端、5commandの出力、
Knowledge、Issue実本文を照合した。Issue受入1〜3の説明条件は充足し、全体完走の条件は
新しいs06成果承認とfinish後に判定するとの境界も維持した。
最初の統合終了Sensorは `direct implementation result must match HEAD` を返した。
実測済み現在HEADを公開configureへ登録し直してpassとなり、Sensorを迂回しなかった。

外側AIは新しい通常入力で「AIによる試験用承認です。提示済みs06 integration成果を承認します。」を返した。
受信turnは `01a08ccf-2eb2-7fa2-8b58-f002255e277d`。
承認待ちの複合Python読取りはhookが拒否したため、通常readコマンドと公開CLIへ切り替えた。
approvalはrevision44、finishは45。同じIntent `915a62dd6391fbab3e08a5707309d687` がcompletedになった。

| 実行回 | 段階 | 成果承認revision | finish revision | 回答元 |
| --- | --- | --- | --- | --- |
| s01 | initialization | 5 | 6 | 人間の既存実回答を通常入力へ中継 |
| s02 | discovery | 15 | 16 | 明示委任されたAI試験用回答 |
| s03 | architecture-analysis | 21 | 22 | 明示委任されたAI試験用回答 |
| s04 | planning | 30 | 31 | 明示委任されたAI試験用回答 |
| s05 | tdd | 36 | 37 | 明示委任されたAI試験用回答 |
| s06 | integration | 44 | 45 | 明示委任されたAI試験用回答 |

親が29件の状態変更履歴をたどり、各fileのhashとprevious連鎖を確認した。
6回それぞれに1件のapproved記録があり、review passのtarget、承認target、finishで受入れたtargetが一致する。
すべて同じIntentと計画revision1に結び付く。全操作auditを追加したものではなく、既存の状態変更履歴の検証である。
最終Knowledgeのhashは `94d85d55d3430e5e84c969cdcd6c3c9e8798ffeb9ec00f985a5e057d49244381`。

このパイロットで製品5担当のnative起動、禁止担当の起動前拒否、承認待ちの一般操作拒否、
新しい通常入力後の承認・再開、実workerのTDD、記録・検索・完了を確認した。
初期化の外側CLIや開発用独立reviewまで製品native担当の実行だったとは扱わない。
初期化後の成果回答はAI試験用であり、人間本人が各成果をreviewした証拠ではない。
固定Codex 0.153.4の通常trustを維持した観測で、hook対象外経路やOS全processの封じ込めを保証しない。

未解消の観測は、linked worktreeでのhook discovery、並列read/失敗patch後のTool残存、
子からの途中通知拒否である。今回のhelp変更に混ぜず、後続運用の入力として保持する。
環境補正・公開復旧・明示した担当訂正を使って完走したことを、無補正で完走した結果と混同しない。
Go cache権限障害は一時cacheで検査を完了し、build時のstat cache警告も原文保存した。

実証拠は専用rootのaidlc/evidence/latest-pilot/に保存する。`32-native-code-review-report.md`、
`33-native-integration-review-report.md`、`34-completion-history-check.json`がレビューと完走照合をまとめる。
元の一時worker証拠73fileは`worker-tdd-evidence/`へ同hashで保全した。
利用プロジェクトのstate・Knowledge・生ログ・配置したhook/agentは製品PRへ含めない。
開発側finalとGitHub checksは、records統合後の固定版について別途実行し、確定結果をPRへ記載する。


## 外側親の最終照合と証拠保全

通常Codexの最終実行はexit0で終端を確認した。公開show/history/session inspect/assignment listも
親が読取り専用で実行し、各exit0、Session.Tool空、旧worker予約releasedを確認した。
受入済み文書と実測証拠14件は保存されたhashと現物が一致した。
registryの11dispatchを元rolloutの実spawn_agent要求・返却task pathへ対応付け、5製品担当の起動を確認した。
履歴とstate、Space、registryを34-completed-snapshot/へ保全し、対応表は34-native-role-check.jsonへ保存した。

実案件の完了対象はコードcommit21e755aである。その後に開発用RAMと計画を統合するため、
開発PRのHEADは変わるが、Goの成果2fileは同じbytesを維持する。
実案件が後から更新された開発文書や新HEADに対して実行されたとは扱わず、完了時点のsnapshotを根拠にする。
