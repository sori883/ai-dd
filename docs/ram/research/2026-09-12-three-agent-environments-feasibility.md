# AI-DLC本体を3つのAI環境へ対応させる調査

確認日: 2026-09-12。状態: 対象の確認と対応案。製品実装・設定変更・実機試験は未実施。

## 確認した依頼

ユーザーが希望しているのは、GitHub用の任意skillだけでなく、このAI-DLC製品全体をCodex、Claude Code、GitHub Copilotで使えるようにすること。Copilotの対象は質問への回答により**VS CodeのGitHub Copilot**と確定した。Copilot CLIやクラウドのcoding agentへ読み替えない。

Goの単一バイナリ、Git不要の基本運用、共通のSpace・Intent・Knowledge、メインAIによる標準機能での子担当起動という既存方針を維持する案を検討する。CLIは進捗や割当を管理し、AI自体を起動しない。今回の対象確認を、新しい複数環境ロードマップ全体の実装承認とは扱わない。

## 現状と変更が必要な場所

mainはPR #182がマージされた`f8d9eb0d83143144bbcb2fc5b6a9dc5db80acf94`。調査したai-dd-namingのHEAD `ce55fda797495b9a3a8698c1034cea632921e7f1`は同じ内容である。元checkoutの未commit資材は変更しない。

| 現在の実装 | 役割と対応時の扱い |
| --- | --- |
| `src/internal/flow`、`src/internal/workspace`、`src/internal/okfmemory`、`src/internal/okf` | Space、Intentの実行計画・state・承認、Unit/Bolt、OKF文書、内容SHAとSensorを共通のGo処理として維持する。内容SHAの処理は`src/internal/flow/verification.go`にある |
| `src/core/workflow/stage-graph.json`、`stages/*.md` | 現在の6種類の工程と許可担当を共通の正本として維持する |
| `src/harness/codex/` | 現状の配布資材はCodex用のみ。共通の行動・工程skillと、環境固有のツール名・導入方法・agent定義を分離する必要がある |
| `src/internal/install/install.go`、`relocate.go`、`src/internal/cli` | install対象、配置先、hook登録、移転検査がCodex専用。環境別の配置と、共通資材を再初期化しない追加導入が必要 |
| `src/internal/app/session.go`、`hook.go`、`child_hook.go` | hook JSON、turn/tool/agent ID、Bash・apply_patch、子の起動・報告をCodex仕様で解釈している。環境別の変換処理が必要 |
| `src/internal/assignment/dispatch.go` | 子の結果を`/root/...`のtask pathへ結び付けるCodex固有部分がある。各環境の子ID・親IDと製品の割当を区別する必要がある |
| `src/internal/flow/procedure.go`の`requiredInputs` | 初期化Sensorが`.codex/hooks.json`と`.agents/skills/`の3入口を固定で検査している。利用中の環境に応じた配置検査へ分ける必要がある |

したがって、設定ファイルだけを増やして3環境対応済みにすることはできない。共通化できる基盤はあるが、hook・担当管理・初期化検査・配布とその回帰検証にまたがる変更になる。

## 一次資料で確認した接続点

| 環境 | 確認できた仕組み | 注意する相違 |
| --- | --- | --- |
| Codex | `.codex/agents/*.toml`、`.codex/hooks.json`、PreToolUseでの拒否、session/turn/tool ID | 既存実機記録は固定Codex 0.153.4での確認。現在のWeb資料や別versionと同一視しない |
| Claude Code | `.claude/skills/`、`.claude/agents/*.md`、settingsのhook、メインAIのAgent toolによる委譲 | 成功後と失敗後のhookが分かれる。子の起動応答と作業完了は別。現行資料のprompt_idはv2.1.196以後なので最低版候補と実機確認が必要 |
| VS CodeのCopilot | `.github/agents/*.agent.md`、skill、`agent/runSubagent`、`.github/hooks/*.json`、PreToolUseでの拒否 | hookはPreview。子への同一会話の追加依頼はできない。Codexのfollowup処理をそのまま移植しない |

主な根拠:

- [Codex hooks](https://learn.chatgpt.com/docs/hooks)、[custom agents](https://learn.chatgpt.com/docs/agent-configuration/subagents)。hook対象外の経路があり、全OS書込み経路の封鎖ではない。
- [Claude Code hooks](https://code.claude.com/docs/en/hooks)、[skills](https://code.claude.com/docs/en/skills)、[subagents](https://code.claude.com/docs/en/sub-agents)。prompt本文と識別情報をイベントから取得し、遅れて保存されるtranscriptから現在の回答を推測しない。
- [VS Code hooks（Preview）](https://code.visualstudio.com/docs/agent-customization/hooks)、[hook入力・出力](https://code.visualstudio.com/docs/agents/reference/hooks-reference)、[subagents](https://code.visualstudio.com/docs/agents/run/subagents)、[custom agents](https://code.visualstudio.com/docs/agent-customization/custom-agents)。

VS Codeは既定で`.claude/settings.json`も読む。ClaudeとCopilot用のhookを同じprojectへ置く場合、二重読込みを防ぐ設計が必要。VS Codeではmatcherが無視され、ツール名や引数名にも差があるため、Go側で対象操作を判定する。既存のユーザーhookを無断で一括無効化する案にはしない。

調査担当は各社の公式資料を参照し、親はContext7でもCodex・Claude・VS Codeを照合した。公式資料は将来変更され得る。記載機能の存在と、この製品が同じ保証で実際に動くことは別の確認である。

## 推奨する構成案

共通のGo本体に、AI環境との接続層を3つ設ける。接続層は「そのAIが発行するイベントやツール名を、製品の処理へ渡す部分」である。

```text
共通のAI-DLC
  Space / Intent / Unit・Bolt / state・承認
  OKF Knowledge・ADR・Rule / 内容SHA / Sensor
  工程手順と5種類の専門担当の責務
       │
       ├─ Codex接続
       ├─ Claude Code接続
       └─ VS Code Copilot接続
```

同じGo実行ファイルから各接続用hookを呼べる構成とし、追加の常駐サーバーや言語ランタイムは前提にしない。既存の日本語チェック用補助CLIは別配布のまま扱う。

共通にするものは工程・判定規則・文書・進捗であり、hostの識別情報を無理に同じ形の値だと見なさない。session/turn/tool/子の対応と故障時処理を環境ごとに確認して変換する。メインAIと5担当の責務を共有しつつ、各環境で提供されるツール名と権限へ写す。例えば調査担当もOKF検索CLIを使うため、単にterminalを全禁止する対応では足りない。

KnowledgeとIntentの正本は引き続き`aidlc/spaces/<space>/`。環境ごとに別の進捗を作らない案を推奨する。ローカルsession識別は環境間衝突を避け、worker予約は同じ管理root全体で共有する。別AIへ切り替える境界、動作中sessionの扱い、新しい保存項目はまだ契約未確定である。

導入先の候補はCodexが`.codex/`、Claudeが`.claude/`、Copilotが`.github/`。同じprojectに共存できるよう、AI-DLC所有ファイルだけを配置し、利用者の設定と共有Knowledgeを保持する。`install claude`や`install copilot-vscode`のような選択肢は設計例であり、現在使えるコマンドではない。

## 先に確認することと順序

1. **接続の前提検証**: 対象Codex・Claude Code・VS Code/Copilotの版を固定し、通常のtrust設定で、会話開始、ユーザー回答、正常・失敗tool、子起動・報告、中断・再開の実際のイベントを確認する。新しい環境の導入・設定変更・実機操作は具体計画と実装許可を確認してから行う。
2. **共通部分の分離**: Codexの観測可能な動作を維持して、共通の判定と環境ごとの入出力、配置・初期化検査を分ける。
3. **2環境の追加**: Claude、VS Code Copilotのagent・skill・hookを接続する。VS Codeで子を続けて呼べない場合は、既存の進捗と成果を次の新規担当へ渡す手順を具体化して確認する。
4. **製品としての完走検証**: 3環境それぞれで、初期化から承認済み実行計画、OKF保存・検索、TDD、独立レビュー、承認、完了までを通す。同rootの予約競合、保存失敗、古い承認の拒否、環境切替、配置の共存・移転も検査する。

会話の成果承認と、AI製品が出すツール実行の許可ボタンは別物として維持する。hook無効・故障時にhost自身が操作を継続する可能性を実測し、止められない経路を「拒否できる」と説明しない。対象条件が成立しない場合に製品CLIが進捗を進めないことと、hostの全編集を止めることを区別する。

実装計画で確定する重要な判断は、対応最低版と実機範囲、同一projectでのhook共存、AI環境の途中切替、識別・保存形式と復旧である。本記録は全実装を許可する確定計画ではなく、その計画を作るための調査結果である。

## 本家の参照範囲

本家の固定snapshot 2.6.123では、共通coreから環境別harnessを介して配布する構造を確認した。ai-dd-namingにはsnapshotが無いため、元checkoutの`docs/実装_aidlc-workflows/core/tools/aidlc-version.ts`と`harness/claude/manifest.ts`、`harness/copilot/manifest.ts`・`emit.ts`を限定して読んだ。

固定copilot manifestはCopilot CLIとVS Code両方への配布を記述するが、そのコメントの実測を今回のGo製品の検証結果として流用しない。今回のCopilot対象はユーザー指定のVS Codeのみ。33 Stage、全操作audit、旧memoryやTypeScript/Bunの配布内容を再導入する合意ではない。最新upstreamとの比較は行っていない。
