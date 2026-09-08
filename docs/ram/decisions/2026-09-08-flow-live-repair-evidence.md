# 四段階liveの再開と証拠集計を修正する

Issue #130の承認済み四段階計画内の通常bug修正。work_unit_id=flow-live-repair、verification_mode=loop。
開始HEADは7529251efad42308a09c0c6c4f1975d21c7e0092。親がfinal2の失敗を受けて単独writerへG1〜G3を委譲した。

## 観測した事実

親final2は1640.37秒で不合格。生証拠は
`/private/var/folders/9w/921pjkys39q28sk4xsc0hs000000gn/T/aidlc-flow-live-1084706664`、
集計は同ディレクトリの`proof-1788839211881796000-378205993.json`。
初回会話ではdiscoveryのreview fail→修正→pass、四段階の完了、a/bの稼働時間重複と別worktree、
実test RED/GREEN、統合後のc実行を観測した。これは再会話での再完了を含む受入全体の成功ではない。

二会話目では既存completed Intentのlist/switchとRule全文読込は成功した。
その後、配置SKILL/WORKFLOWのcatをinactive状態の一般操作として拒否したため、完全手順を読めなかった。
reopenの試行には--to、--id、--session、--helpが混ざり、必須--expectも欠けていた。
正規文法のreopenが拒否された証拠ではなく、手順読込の循環が原因だった。

## G1: 失敗時の架空State出力を除く

実CLI subprocessのstale reviewとrevision conflictで、exit 2と診断に加えzero Stateをstdoutへ出すREDを観測。
executeFlowは失敗結果をencodeせずerrorを返す。checkの正しい不合格Sensorはtarget/summaryを含むJSONを維持する。
TestFlowCommandFailureOutputで両拒否のstdout空・exit 2・診断完全一致と、failed Sensor保持を検査した。
途中で追加した同名create拒否caseは、同名を固定IDで区別する既存契約と矛盾していたためINVALID_TEST_FIXTURE。
親確認後そのcaseだけ除き、stale/conflictの有効REDとは区別した。

- `go test -count=1 ./src/cmd/aidlc -run '^TestFlowCommand'`: 有効RED exit 1 → GREEN exit 0。
- `go test -count=1 ./src/internal/minimal -run '^TestFlow'`: GREEN exit 0。

## G2: 完了・待機・中断から配置手順を読んで再開する

TestFlowInactiveWorkflowReadAndResumeはcompleted/waiting/pausedの3状態で配置手順のcat拒否をREDとして観測。
選択済み・当該turnのRule読込・Rule現物hash一致・非稼働slotの確認後、literalの
`cat .agents/skills/aidlc/WORKFLOW.md`（配置SKILLとの併読も可）だけをinactiveでも許可する。
正常なtool slot取得/Post解放を維持。一般write、他file、redirect、compound、symlink、未選択、
新しいturn、変更Rule、missing fileは拒否する。正規resume/reopenのCLI文法を拡張せず検査した。
配置SKILLに具体catと既存--expect付き文法を記載し、実配布本文の4KiB capを維持した。

- `go test -count=1 ./src/internal/minimal -run '^(TestFlow|TestHook|TestSession)'`: RED exit 1 → GREEN exit 0。
- `go test -count=1 ./src/internal/install -run '^TestFlowInstall'`: 配置文法不足のRED exit 1 → GREEN exit 0。

## G3: CAS拒否と再会話の完了を実証する

旧rawのstale拒否は診断＋zero Stateが混在し、stdout/stderrの順序も一定でなかった。
その前にはExpect=7/Pre revision=6の正しいCAS拒否もあった。
G1後の診断単独出力を前提に、実review requestと実jobが一致し、exit 2、診断完全一致、
非zeroのPre revisionとExpectの不一致が揃ったCAS拒否だけを既知拒否として扱う。
これはレビュー合格やstale拒否の代用ではなく、別途実stale拒否と実受理を必須とする。
TestFlowCommandConflictRequiresObservedRevisionで正常拒否を誤失敗するREDと、未観測revision=0を許すREDを修正。

旧verifierは2会話のbindだけでは再完了の不在を検出しなかった。
TestFlowCommandRequiresSecondSessionCompletionで不足証拠を誤受理するREDを観測後、
会話別のbound state観測を採取し、初回完了→二会話目の同ID完了済み選択→activeへの再開→
新しいreviewerの実pass受理→それより新しいrevisionで再完了を必須にした。
判定閾値は緩めず、旧proofを成功へ書き換えていない。新しい実機証拠が必要。

- `go test -count=1 ./src/cmd/aidlc -run '^TestFlowCommand'`: 各回帰RED exit 1 → GREEN exit 0。

## 境界と親final入口

親許可によりlive専用deadlineを30分から45分に変更。既測27分20秒の実行に再開・再reviewの時間を確保するため。
製品timeout、モデルgpt-6-astra/medium、sandbox、認証、hook trust条件、host Git支援の範囲は維持。
親finalのliveコマンドは以下へ更新した。

```sh
AIDLC_FLOW_LIVE=1 go test -tags=integration -v -count=1 -timeout=50m ./src/cmd/aidlc -run '^TestFlowJourneyLive$'
```

末尾は上記targeted全てと、親許可の`go test -tags=integration -count=1 ./src/cmd/aidlc -run '^TestFlowCommand'`で
live harnessのcompileと非live回帰だけを検査する。gofmtとgit diff --checkを実行する。
全体test/race/vet/liveは実施せず、修正後の実機再完了は親finalに残る。

末尾結果: 上記5本のtargeted（integrationタグ付き同prefixを含む）は全てexit 0。
gofmt適用済み、git diff --checkもexit 0。HEADは開始値のまま。ユーザーのAGENTS.md差分と未追跡参照資料を保持した。
