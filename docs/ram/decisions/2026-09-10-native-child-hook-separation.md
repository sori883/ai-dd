# native子エージェントへ親の会話検査を誤適用しない

日付: 2026-09-10。Issue #161の承認済み計画内の不具合修正。

## 実機で分かったこと

`cb07593c4dae0b86adc757517c41f687ad9d445c`の全package test、race、integration、vet、整形・module確認、6種類のbuildは成功した。しかし、固定Codex CLI 0.153.4/macOS arm64/astra xhighの実機では、登録済み2workerのspawn Pre/Postは成功しても、両方の通常コマンドが親向けRuleTurn検査で拒否された。有限processの実行記録は0件で、実並列の受入条件は未達である。モデルの成功終了はこの判定を置き換えない。

子のhookには親と同じsession_id、子固有のturn_id、agent_id、agent_typeが届く。親のspawnにはagent_idがなく、入力のagent_type/task_nameで担当を指定する。この違いを捨てたまま同じ会話stateへ通していたことが原因である。[公式hooks仕様](https://learn.chatgpt.com/docs/hooks)と固定実機の生入力で確認した。cwdは親のディレクトリであり、実workerの作業rootの証明には使えない。

証拠は `/private/var/folders/9w/921pjkys39q28sk4xsc0hs000000gn/T/aidlc-assignment-live-1170376627/allow-parallel/`。終了したcaseのmanifest・transcriptとhook eventsを保存した。検証全体の記録は `/var/folders/9w/921pjkys39q28sk4xsc0hs000000gn/T/ai-dd-native-assignment-final-zdz_0a1f/`。不具合を確認したため残りの実機caseを中断した。この差分でfinal合格、実並列成功とは扱わない。

## 修正と許可の根拠

[直接承認](2026-09-10-native-agent-assignment-implementation-approved.md)は、メインAIが直接起動し、子は渡されたRuleと別worktreeで作業し、共有state/Knowledge/承認の保存をメインAIが担当する構成を含む。子へ親の会話state更新を求めず、この既存の役割分担を実現する修正である。独立した計画点検でも、以下の保護を残す範囲は既承認内と確認した。子を一律許可する方式にはしない。

- 公開agent_id/typeで子を識別し、子と識別済みなのにtypeが欠落・未知なら起動前検査を拒否する。
- 子の通知を親の会話更新より前に分離する。子のPost/Stop/UserPromptSubmit等で親のRuleTurn、Tool、承認回答、予約、task対応を更新しない。
- 子のPreでは親が選択した現在IntentのProcedure.Agentsとagent_typeを照合する。workerには既存の承認・開始Sensor検査を適用する。
- 子からのnative再委譲と、認識済み製品CLIによる状態・承認・Knowledge・Unit・予約・sessionの変更を拒否する。helpと読取り・診断は利用できる。既存の保護対象へのapply_patch拒否も維持する。
- 子の通常Bash/apply_patchだけは親のRuleTurn/Toolを使わず、既存の担当指示とsandboxに従って実行する。未知コマンドを読取り確認済みとは呼ばない。任意shellや全ファイル編集経路の網羅的な制限を追加しない。

agent_idと予約の新しい対応表、実rootの本人確認、全process停止の保証は作らない。親の起動・追加依頼の担当/予約検査、明示解放の契約も変更しない。CLIによるagent起動は追加しない。

## 修正と検証の順序

同じ単独writer/work_unit_idで、まず観測した親session共有・子turnの入力を回帰テストにし、通常作業の誤拒否と親state汚染が失敗することを確認する。担当不一致、worker未承認、子の管理CLI・再委譲、保護patchを拒否する対照と、親経路が維持される対照を追加する。

対象はminimalのhook/sessionと子用処理・test、必要な配布担当手順、実機fixture、計画と本RAMの実装証拠に限定する。loopはtargeted検証と影響packageだけ。独立review後、差分を固定してread-only finalを最初から再実行し、実機のprocess記録で並列作業を確認する。旧finalの成功部分を修正後codeの証拠として流用しない。

## 実装loopの証拠（開始d6e2c16）

同じ`work_unit_id=native-agent-assignments`で単独writerが次の順に実装した。

| 順 | exact command | RED → GREEN |
| --- | --- | --- |
| 1 子通知の親state分離 | `go test -count=1 ./src/internal/minimal -run '^TestChildHookNotifications$'` | exit 1: SessionStart/UserPromptSubmit/Postが親filesを変更し、Stop等が親向け制御を返した。公式トップレベルagent_id/agent_typeを保持しwithSessionより前へ分離してexit 0 |
| 2 子の担当・開始条件 | `go test -count=1 ./src/internal/minimal -run '^TestChildHookEligibility$'` | exit 1: 正しいworker/read-onlyの通常作業も拒否。現在Procedure.Agentsと既存AssignmentStage/CheckWorkを読み取り利用しexit 0 |
| 3 子の管理変更拒否 | `go test -count=1 ./src/internal/minimal -run '^TestChildHookCommandBoundary$'` | exit 1: 子のbind/release/Unit/Knowledge/承認変更・分類不能CLI・保護patchが許可。明示read/diagnostic allowlistと既存protectedPatchを適用しexit 0 |
| 4 有限process測定 | `go test -count=1 ./src/cmd/aidlc -run '^TestAssignmentProcessRendezvous$'` | 宣言のみのhelperからexit 1: peer不在でも成功しreadyなし。有限待機、peer到着後の作業区間、nonce重複拒否を実装しexit 0 |

子Preでは公式識別・tool/turn・親の現在選択・担当適格性を確認する。子通知は承認回答やnative Post対応を含め親の保存処理へ通さない。回帰では親sessionのRuleTurn/ToolとIntent/registry等の全bytesが不変であることを確認し、既存pending taskに対する子native Postも無視する。
worker開始条件は既存CheckWorkを利用し、子のRuleTurnやToolを親と一致させない。実測のagent_idからtask_name/予約を推測する処理はない。
認識した製品binaryの分類不能操作（space create等を含む）は親へ返す拒否とし、一般Bashへ戻さない。未知の一般shellをread-only認証したとは扱わない。親経路の既存検査は維持する。

配布workerとaidlc-cliへ、子は渡されたRuleを使い親のsession bind/intent switchを代行しない旨を追加した。bootstrapを子へ返して親会話を更新させる運用にはしない。

## fresh実機への引渡し

旧cb07593実機ではspawn対応、相対/canonical task pathへのfollowup、Unit reported後の競合拒否、両release、新reserveは実測された。一方、子process印は0件で実並列は不合格だった。この修正の成功証拠へ流用しない。

4case、固定モデル、全体25分・case 5/7分の予算を維持する。allow-parallelだけ`TestAssignmentProcess`を子が実行する。各子は固有nonce（childa/childb）をO_EXCLで確保し、`.ready`を作成する。peer readyを最大90秒待ち、到着後15秒の有限作業を行う。timeoutも失敗として終了・記録する。nonce重複で以前のrecordを上書きしない。
`.process.json`にはNonce/Peer/PID/Cwd/StartedAt/ReadyAt/WorkStartedAt/EndedAt/Errorを記録する。親がprocess印を作らず、両子の実WorkStartedAt〜EndedAtの重なりで並列を判定する。readyの存在だけでは合格にしない。中断でrecordが未完なら不明として残す。
helperは最大105秒で完了し、追加依頼は短い応答だけで再実行しない。live promptは両spawnを先に行い、子は自身のprocess sessionを終端までpollする。親会話更新は依頼しない。

末尾 `go test -count=1 ./src/internal/minimal -run '^TestChildHook'` とhelper targetedはexit 0。
影響確認 `go test -count=1 ./src/internal/minimal ./src/internal/install` はexit 0。
このloopではintegration/live、全project/race/vet/buildを実行していない。独立review後の親fresh finalで実機を確認する。
