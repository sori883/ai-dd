# 委任した試験用回答と通常hookの実測

日付: 2026-09-11。Issue #163。進行中。
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
