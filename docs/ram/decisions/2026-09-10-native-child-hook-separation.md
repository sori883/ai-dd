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
