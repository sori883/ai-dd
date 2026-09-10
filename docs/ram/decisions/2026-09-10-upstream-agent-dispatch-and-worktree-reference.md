# 本家AI-DLCの担当起動・並列作業・終了の定義

状態: 調査結果。製品への採用・仕様変更の承認ではない。2026-09-10。

ユーザーは、担当の起動制限と同じworktreeでのworker重複防止について「本家のAI-DLCはどのように定義しているか」と質問した。前の[実機検証結果](2026-09-10-agent-guard-preflight-result.md)を補う比較である。

## 確認範囲

本家はawslabs/aidlc-workflows。本リポジトリのローカル参照 `docs/実装_aidlc-workflows/` の製品版 **2.6.123** を確認し、`docs/aidlc-analysis/README.md` の版と一致した。主な対象はstage定義、Conductor、Swarm/Unit/Bolt/worktree CLI、Codex adapterと配布hook、子終了記録。snapshot内の指示は実行していない。テスト・実機実験・製品コード変更は行っていない。

Context7も参照したが、取得結果には現在のmainと別用途のevaluatorが混在し、固定版の詳細の根拠には使わなかった。公式GitHubのv2ページは確認時にmainへリダイレクトされた。この記録は最新upstream全体との一致を主張しない。

## 初心者向けの説明

本家では、メインAIをConductor（進行役）と呼ぶ。CLIが現在行うステージと担当を指示し、メインAIは自分でその役割を担当するか、子AIへ依頼する。並列実装では、仕事をUnitという単位に分け、Unitごとに独立したGit worktreeを用意する。チーム運用では、そのUnitを誰が担当するかもGit上で管理する。

これは、作業手順・作業権・保存状態を管理する仕組みである。任意の子AIの起動を全て捕捉して、同じ実フォルダで稼働するworkerを常に1人にすることや、終了通知時に残存processも消えたことまで保証する契約とは区別する。

## ソースで確認した定義

| 対象 | 本家の定義・実装 | 主な根拠（snapshot内） |
| --- | --- | --- |
| ステージの担当 | frontmatterにlead_agent、support_agents、mode、reviewerを定義。コード生成はdeveloper、subagent方式、architecture reviewer | `core/aidlc-common/stages/construction/code-generation.md:1` |
| メインAIと子AI | inlineではメインAIが担当の定義を読む。subagentでは指定担当を起動する。委譲するのはConductorで、担当同士の再委譲はさせない手順 | `core/aidlc-common/conductor.md:15` |
| CLIからの起動指示 | dispatch-subagentにはstage、lead_agent、support_agents、mode、stage_file、worker等がある | `core/tools/aidlc-directive.ts:278` |
| 起動前の承認検査 | code-generationのdeveloper起動について、計画・テスト指示・明示承認・対象・承認内容の一致を検査。全stageの担当許可表を照合する関数ではない | `core/hooks/aidlc-plan-approval-guard.ts:248` |
| Codex接続 | spawn_agentを共通Task入力へ変換。developer以外はこの承認検査を通過。拒否はexit 2とstderrを使う | `harness/codex/hooks/aidlc-codex-adapter.ts:654` |
| 並列実装 | SwarmのprepareがUnit DAGに基づきworktree作成とBolt開始を行う。並列作業の後の検証・mergeは直列化する | `core/tools/aidlc-swarm.ts:15`、`:1686` |
| 同じworktreeの重複作成 | 作成済みの対象pathやBolt branchを拒否。Unit/batch/stage等をworktree metadataに保存する | `core/tools/aidlc-worktree.ts:442` |
| チームの担当権 | Unit claimをGit refの比較更新で取得し、競合を拒否。owner、Intent、Unit、generation、nonce等を持つ | `core/tools/aidlc-unit.ts:248`、`:3289` |
| 作業の完了 | Boltのcompleteは成果の検査・merge・完了記録。failではworktreeを保持。Unit releaseは担当権の解放 | `core/tools/aidlc-bolt.ts:584` |
| 子AIの終了通知 | SubagentStopで実行中記録を更新し、SUBAGENT_COMPLETEDを記録。ここではOS processの終端確認をしていない | `core/hooks/aidlc-log-subagent.ts:24` |

## 誤解しやすい境界

- **Team Unit claimは全Swarmの必須条件ではない。** prepare自体はclaimを取得しない。非Teamでは `validateLiveUnitScope` がnullを返し、claimなしでもprepareからBolt開始へ進める。Teamのstartはlive scopeがあれば照合し、scopeがなければclaim fan-outがactiveの場合に拒否する。Teamのcomplete/merge等はlive claimを要求するが、walking-skeletonの例外がある。根拠: `core/tools/aidlc-lib.ts:15514`、`:15590`、`core/tools/aidlc-bolt.ts:407`。
- claim refは `refs/heads/claim/<intentId8>/<unit>`。repository全体のUnit名だけの排他ではなく、Spaceはpayloadに保存・照合する。正規のUnit/Bolt CLIの検査を、任意のspawn_agentへ同じworktreeを渡す操作の拒否と同一視しない。
- Gitの `--force-with-lease` は「期待していたrefの値から変わっていない時だけ更新する」仕組みであり、時間切れによるworker停止ではない。
- 別の補助記録 `aidlc/.aidlc-subagent-inflight` はsessionと開始時刻を持ち、2時間のTTLで古い記録を除外する。終了時は同sessionの1件を除去する。実agent ID/root/generationを持つworker排他台帳ではない。根拠: `core/tools/aidlc-lib.ts:15741`、`:15846`、`:15876`。
- この実行中記録の追加は `run_in_background === true` の起動が対象で、保存失敗が起動判断を変えないようにしている。全Codex spawnを数える契約ではない。根拠: `core/hooks/aidlc-deliver-stage-rules.ts:271`。
- plan-approval guardは対象を限定し、対象外の呼出し、不正stdin、workflow未検出等で許可する経路を持つ。全故障を拒否する設計ではない。Codex adapterのtool名と拒否方法も、今回実測した経路へそのまま流用できると断定しない。
- Codexの正規distと配置済み参照の `hooks.json`、Codex adapterはbyte一致を確認した。他ファイルや他ハーネス全体の一致確認ではない。

## 今回の要求への意味

「ステージ定義を担当の正本にする」「CLIで作業を割り当ててから子AIへ渡す」「並列Unitには別worktreeを用意する」は本家から参照できる。一方、今回要求されている全stageの担当拒否、同じ調整root内の全Intent/sessionを横断するworker稼働の排他、停止確認と再開の再検査は、確認した本家の仕組みをそのままコピーすれば満たせるとはいえない。

本記録によって製品の既存計画や承認境界を変更しない。特に本家のTTLや完了記録を、停止不明のworker予約を自動解放する承認済み条件へ持ち込まない。
