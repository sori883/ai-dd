# 固定Codexでの担当起動制限の実測結果

状態: 調査結果。製品guardの契約確定は保留。2026-09-10、Issue #159。

## 結論と今回の変更

現在ステージで許可されていない担当の起動と、同じworktreeでのworker重複を防ぐため、起動前hookと子の寿命を調べた。**正常なhook応答による起動前拒否は実測できた。一方、予定worktreeとの構造化した対応、確実な終了・再開、hook故障時の拒否は、製品の要求を満たす契約にできていない。**

追加したのはGoの試験用hook、証拠検査器、一時プロジェクトで実Codexを呼ぶ任意実行の試験と本記録である。製品の担当制限やworker排他を有効化する変更ではない。通常のGo testではモデルを呼ばない。既存の製品Goコード・配布hook・利用者設定・外部Go moduleは変更していない。

実施範囲は[直接承認](2026-09-10-stage-agent-worker-guard-approved.md)と[G0作業単位](../../design/agent-guard-preflight-work-unit.md)に基づく。製品guardへの移行条件が満たされていないため、[条件付き実装案](../../design/stage-agent-worker-guard-plan.md)は実装しない。専用起動方式や公開runtime形式の採用には別の契約確定が必要である。

## 固定した環境と証拠

Codex CLI **0.153.4**、macOS arm64、gpt-6-astra / xhighで実測した。既存CLIと通常の認証経路を使い、一時Gitプロジェクトとworktree、試験専用hook設定だけを作った。hook trust bypassはこの隔離試験限定であり、製品の通常導入手順ではない。

以下のA・Bはソースと実験を固定して完了した観測である。後からfixtureの判定器を修正しても、これらのrawを上書きしたり、新しいソースの実行結果へ読み替えたりしない。PRでは最終ソースの独立reviewとfresh finalの証拠を別に示す。

| 観測 | 試験ソースHEAD | ローカルraw保存先 |
| --- | --- | --- |
| A: 初回。Stop後のprocess継続 | `4a8dc3fbb0b3fed5ed491c24f915c5c8af937060` | `/var/folders/9w/921pjkys39q28sk4xsc0hs000000gn/T/aidlc-agent-hook-probe-579906995` |
| B: 実hook名修復後。起動拒否・故障・追加依頼 | `05df6d1e951eb22e38b8a674079f1a9e6040cb20` | `/var/folders/9w/921pjkys39q28sk4xsc0hs000000gn/T/aidlc-agent-hook-probe-3375108718` |

rawは試験した親子sessionだけを収集したローカル資料であり、Gitには収録しない。一時保存先は恒久配布物ではない。再実測コマンドと記録形式、各修復のRED/GREEN、A・Bの詳細は[実装証拠](2026-09-10-agent-guard-preflight-evidence.md)に残す。試験runnerのexit 0と製品要件の成立は区別する。

## 前提gateごとの判断

| Gate | 実測と判断 | 製品への影響 |
| --- | --- | --- |
| G0-1 起動前拒否 | BのdenyはPreでdenyを返し、実tool結果が拒否を通知。子Startと試験印は0件。allowは子Startと試験印が各1件。限定した経路では手動raw照合でpass | 起動前の担当照合に使える根拠。ただし再開や故障時まで保証するものではない |
| G0-2 要求・子・rootの対応 | task_nameと子metadataを使う対応候補は得たが、構造化した予定rootの指定は確認できていない。inconclusive | 自然文や親cwdからworkerの予約先を推測してはいけない |
| G0-3 追加依頼・再開 | followup_task等のPre/Postと、idleだった子が再稼働する例を観測。実行中割込みとcloseは未実証。gate全体はinconclusive | 初回spawnだけの検査では要件を満たしたことにならない |
| G0-4 終了 | Aで子Stop後も同じ子が開始したprocessが約10.953秒継続。Stopだけによる完全終了判定は成立しない | Stop、結果提出、spawnのPostだけでworker枠を解放しない。安全な解放条件は未確定 |
| G0-5 Unitとroot境界 | Bで複数の子は動いたが、Unit/root割当との対応は未確認。実Unit APIは操作していない。別調整rootの排他も未実証。inconclusive | Unitあり/なしの具体的な予約契約と1調整root境界が必要 |
| G0-6 故障 | Bの応答欠落・非ゼロ終了・保存失敗・timeoutで子開始を観測。故障時に必ず起動を止めるという仮定は成立しない | hookが壊れた場合も排他を維持できるとは説明できない |

### 起動要求と子の識別

実hook名は `collaborationspawn_agent` だった。直接tool名の `spawn_agent` とは表記が異なる。Pre/Postは同じ `tool_use_id` で照合でき、Postの結果は `task_name` を含むJSON文字列だったが、子の `agent_id` は含まなかった。

SubagentStartの子IDとtranscript path、子transcript先頭の `session_meta` にあるID・agent_path・明示parent IDを照合できた。`fork_turns="none"` では `forked_from_id` が省略される。この非公開の履歴形式を、安定した製品APIと見なさない。同じtask_nameの再使用、再試行、並行開始、通知逆転時の一意性は未確認である。

親の作業場所や、子が文章で指定先へ移動したという報告は、起動前に予定worktreeを拘束した証拠ではない。二つのworktreeを試験用に作れたことも、二つの子へそれぞれ割り当てられた証拠にはならない。

### 終了と追加依頼

Aの子 `01a0874b-6c42-7dd1-b974-a17305f4773a` が開始したhelperを、Bash Preのcommand、manifestのnonce、process記録で対応付けた。Stop記録は `2026-09-09T17:51:16.770721Z`、同じhelperの終了は `17:51:27.723715Z` だった。任意のprocessを止める実験はしていない。

Bでは `followup_task`、`send_message`、`interrupt_agent` のPre/Postが記録された。followup_taskはidleの子を再稼働させた。send_messageによるidleからの開始は観測していない。interrupt_agentの結果は既にcompletedの子を示したため、実行中の割込みを確認できたとは言わない。調べたschemaには同等のclose操作がなく、割込みや一turn完了で代替していない。

### hookの故障

Bのmissing-response、nonzero-exit、save-failure、hook-timeoutは全てtask_nameを返し、子Startが各1件、process印が各1件あった。試験hookの記録上の予定exitはそれぞれ0、2、74、0である。これは特にtimeoutで実際に返却されたexitを意味しない。

timeout設定1秒に対してhookは記録後3秒待つ。spawnのPre記録とPost記録の差は約1.102秒で、hookの完了前に起動が進んだ。明示timeout診断やhook processへのOSのkillは確認していない。非ゼロ終了・保存失敗・timeoutを、正常なdeny JSONと同じ拒否手段として扱えない。

## 次に確定すべき契約と選択肢

推奨する次の作業は、**workerの起動前に構造化した割当を登録し、終了を確認してから予約を解放する手順を具体化すること**。例えばIntent、任意Unit、正規化root、担当、要求IDをCLIで登録し、実際の起動・子ID・再開と対応させる。ただし既存のspawn引数で安全に結べるか、hook故障時にもその手順を守れるかは未解決であり、この案だけで排他が実現すると断定しない。

| 選択肢 | 得られるものと制約 |
| --- | --- |
| 既存の起動経路を維持し、明示的な割当・停止確認の契約を追加する案を設計する（推奨） | 現在の並列worktree運用を維持しやすい。hook故障時やID対応の限界を利用条件として合意できるかが必要。停止不明の予約は保持する |
| 起動から終了まで管理する専用CLI経路を検討する | 起動前のroot指定とprocess寿命を管理する余地がある。既存の「schedulerを作らない」境界を変更するため追加承認が必要。Codexや子process全体の停止まで管理できるかは別途検証する |
| 起動時の担当拒否だけを先行する | 最初の起動への対策は作りやすいが、再開の検査とworker重複防止が残る。元の2要件からの範囲縮小なので、黙って採用しない |

現時点ではいずれも新規の実装許可へ読み替えない。運用手順への依存、公開CLI、runtime保存形式、復旧時の確認者、既存配置の更新を含む具体案を示してから、必要な選択をユーザーへ確認する。

## 本家との差分と確認範囲

本家AI-DLCはローカル固定 **2.6.123** のCodex adapter、配布hook、子終了記録に限って確認した。今回の変更は検証用コードと文書だけであり、製品挙動の意図的変更はない。本家の全scheduler、worker排他、全ハーネスや最新upstreamを調べた結果ではない。

[公式hook仕様](https://learn.chatgpt.com/docs/hooks)のPreToolUse拒否等は設計の根拠だが、本記録のCLI実測をDesktopの全tool経路や将来版へ一般化しない。最終的な製品契約と本家から採用する差分は、次の計画で確定する。
