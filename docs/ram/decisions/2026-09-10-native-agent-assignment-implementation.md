# native担当・作業場所管理の実装loop

2026-09-10。Issue [#161](https://github.com/sori883/ai-dd/issues/161)、work_unit_id `native-agent-assignments`、verification_mode `loop`。
開始HEADは`23488f91b681cfe30f80bdeb21ade70c351662d2`、branchは`codex/native-agent-assignments`。
[具体計画](../../design/native-agent-assignment-plan.md)全体とQ1/Q2の[直接承認](2026-09-10-native-agent-assignment-implementation-approved.md)に基づく単独writer作業。

## 実装と境界

新assignment packageは明示初期化・schema 1・epoch・横断予約・native task対応・解放・復旧を管理する。
flowはUnitあり/なしを共通予約へ接続し、予約→Unit runtime→progressの順で保存する。
部分保存は同一要求だけを復旧させ、Save/Begin等の共通guardも元revisionを保護する。旧Unit runtimeだけの結果提出・予約後付けは認めない。
minimalは現在Rule/Intent/step/担当を検査し、workerだけ開始条件と予約を要求する。read-only担当をworker開始Sensorで一律に止めない。
native Pre/Postは親sessionとtool_use_idで対応し、実測済みtask_name応答だけを保存する。製品はtranscriptを読まない。
同じ名前の別起動、未応答taskへの追加依頼、別stepや解放後の再依頼を拒否する。interrupt・Stop・Postは解放しない。

repository所属は共有履歴で具体化した。UnitなしはIntentのCodeRevisionがworker HEADの祖先であることをflow層で検査する。
Unitの既存base/HEAD/依存/scopeは維持する。共通git-dirやremote URLの一致を必須化せず、cloneによるworker移転を維持する。
管理rootをcloneしてruntimeを失った場合、既存Unitに予約を後付けせず、新しいIntentへ必要な計画を登録してclaimする。

init/resetのhuman_confirmedと理由、releaseの停止・追加依頼終了・成果回収理由は申告として保存する。
新規受付はJSON escapeで膨らむ理由も含め未解放予約ごと16384byte、未応答taskごと1024byteの余裕を残す。
容量不足で新規受付を止め、受理済みPostと解放を可能にする。自動GC/TTLは追加しない。
旧Skill/matcherは既知bytesだけを参照移転し、旧構成を新しい保護へ自動upgradeしない。

## TDD証拠

各行のcommandをtest先行で実行した。新APIは宣言・未実装errorだけの承認済みscaffoldでrunnableにした。
各REDはexit 1、対応するGREENは同じcommandのexit 0。途中のfixture不正をREDに算入していない。

| slice | exact command | runnable REDの観測 |
| --- | --- | --- |
| Registry | `go test -count=1 ./src/internal/assignment -run '^TestRegistry'` | 明示Initが未実装。追加回帰で欠落root/未知status/重複IDを含む保存済みrecordを受理していた |
| Reservation | `go test -count=1 ./src/internal/assignment -run '^TestReservation'` | 予約未実装、別process成功0件。追加回帰でreserveのrequest_idをreleaseへ再使用できていた |
| UnitAssignment | `go test -count=1 ./src/internal/flow ./src/internal/assignment -run '^TestUnitAssignment'` | progress失敗時に予約0件、Unitなし未実装。reassignが旧予約保持/新予約未作成。Save/Beginがpending予約中の変更を許可。旧runtime resultが予約なしで成功 |
| AssignmentStage | `go test -count=1 ./src/internal/flow ./src/internal/minimal -run '^TestAssignmentStage'` | stage API未実装、開始Sensor前の許可read-only担当が拒否されていた |
| AssignmentDispatch | `go test -count=1 ./src/internal/assignment ./src/internal/minimal -run '^TestAssignmentDispatch'` | dispatch API未実装、応答後の相対target依頼が不許可担当扱い。未知control文字を含むcanonical pathを受理していた |
| AssignmentRecovery | `go test -count=1 ./src/internal/assignment ./src/internal/minimal -run '^TestAssignmentRecovery'` | release/init応答喪失retry失敗、容量到達後のPost失敗、reset未実装、JSON escapeされた有効な理由でrelease保存失敗 |
| AssignmentContract | `go test -count=1 ./src/internal/cli ./src/internal/minimal ./src/internal/install -run '^TestAssignmentContract'` | assignment CLI/実行未対応、native matcher欠落。旧Skill/matcher移転対照は既存動作としてALREADY_GREEN |

AssignmentStageのtestでEntryを直接Saveするfixtureは既存の「entry and accepted are CLI owned」で拒否された。
親の指示で公開Reopen→CaptureApproval→DecidePlanを行う既存helperへ修正し、新stepのEntry欠落を確認した。
install fixtureのtemp root別名もEvalSymlinksで揃えた。どちらも受入期待・製品境界の変更ではない。
既存Unit回帰はtest専用helperで明示init/epoch/request/coordinatorを指定し、reassignは実claimを済ませてから行う形に追従した。
製品に旧入力の迂回を追加していない。

## 最終検証への引渡し

loopの許可対象は上記targeted群と次のaffected package commandである。affected群は一度全packageがexit 0になり、
その後の末尾修正に対する最終結果は単独writer返却と固定commitを照合する。

`go test -count=1 ./src/internal/assignment ./src/internal/flow ./src/internal/minimal ./src/internal/cli ./src/internal/install ./src/internal/workflow`

integration/live、全project/race/vet/crossbuildはloopでは実行していない。独立reviewと親のread-only finalが必要。
新しい公開CLIのdeterministic fixtureは`TestAssignmentJourney`（integration tag）。固定実機は次の明示opt-inのみ。

`AIDLC_ASSIGNMENT_LIVE=1 go test -tags=integration -count=1 ./src/cmd/aidlc -run '^TestAssignmentLive$' -timeout 30m`

既存Codex0.153.4/macOS arm64/gpt-6-astra xhigh、専用temp管理root/worktrees、通常CLI認証（認証file読取/コピーなし）を使う。
許可並列、拒否、Post欠落、保存失敗を個別caseで試みる。全体25分、case 5〜7分、有限helperは15秒である。
`aidlc-assignment-live-*`にmodel transcript、hook生入力/出力/exit、process印、case manifestを残す。
fixtureの終了やモデルexit 0を製品gateのpassとは扱わず、未実行・対応不明はinconclusiveとして親がraw確認する。
本loopでは新製品の実機成功を主張しない。旧G0の成功や故障観測は新製品codeの成功証拠として流用しない。

実機rawの判定は次を対照とする。SubagentStart/Stopはfixtureが記録するだけで製品へ渡さず、製品の解放処理には使わない。

| case | raw確認点 |
| --- | --- |
| allow-parallel | Unit claimとUnitなしreserveが異なるrootを保持。実spawn Pre/Postのsession/tool ID/task名がregistry Dispatchと一致。childa/childbの有限process印で実行区間の重なりを確認。相対/canonical追加依頼の実Pre/Post、unit reported後の競合拒否、理由付きrelease後の新予約を確認 |
| deny | 実unregistered worker spawnのPreがdeny JSON/exit 0を返し、実tool結果が拒否。子Startとprocess印が生じないことを確認。単にspawn未実行ならinconclusive |
| missing-post | 実spawnを行いfixtureのpost_deliberately_omitted=trueを確認。registryはpendingのまま、同じtaskへの追加依頼Preがdeny、旧予約が保持されることを確認 |
| save-failure | 実spawn Pre保存中だけregistry親directoryを書込不可にした試験印を確認。製品の保存失敗診断・Pre拒否、予約がreservedのまま、実tool結果/子Start/process有無を対照とする。hook故障を成功扱いしない |

model exitだけ、Postだけ、Stopだけ、自然文によるroot自己申告だけではこれらをpassにしない。未発火・未実行は明記する。

## 末尾の原子性修正

`TestUnitAssignmentReassignAdmissionFailure`は、新rootが既に予約済みのとき旧予約だけがreleasedになる
runnable RED（上表UnitAssignment command、exit 1）を検出した。旧解放と新予約を同じregistry lock内で
組み立て、admission確認後に1回だけ保存する`Replace`へ変更し、同commandはexit 0となった。
保存失敗で旧予約が残り、同一要求の再試行が同じ新IDを返す
`go test -count=1 ./src/internal/assignment -run '^TestReservationReplacementSaveFailure$'`
は追加時からexit 0（ALREADY_GREEN）だった。競合や書込失敗で旧予約の解放だけを永続化しない。
160文字の公開request_idを使うreassign回帰では内部release IDの長さ超過を検出し、
元request_idのhashから内部IDを作ることで公開上限を維持した。

末尾7 targetedは全command exit 0。affectedの既存`TestRun_Help`完全一致fixtureは
追加したassignment案内に未追従でexit 1となったため、承認済みhelp契約へ期待文を追従した。
製品の追加変更や受入条件の緩和は行っていない。

このfixture追従により、引数なしの起動だけassignment案内がない分岐差も検出した。
`go test -count=1 ./src/internal/cli -run '^TestRun_Help$'`の同一help期待を維持し、
Help内で引数なしも同じ公開helpへ統一してexit 0とした。これはhelp契約の通常修正である。
未知引数のstderr回帰は既存の基本helpを期待するため、公開helpの期待定数と分けて維持した。
`go test -count=1 ./src/internal/cli`はexit 0。help fixture修正に人工REDは数えない。

最終境界: 上表7 targetedはすべてexit 0、help修正後のAssignmentContract再確認もexit 0。
affected 6 packageの単一commandも最終exit 0（assignment 8.259s、flow 66.257s、
minimal 7.229s、cli 0.989s、install 1.430s、workflow 1.387s）。変更Goへgofmtを適用し、
`git diff --check`はexit 0。integration/live/full-project/race/vet/crossbuildは未実行のまま親へ渡す。
