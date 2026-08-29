# 外部・内部API

## APIの性質

AI-DLC v2 はHTTPサービスではない。外部との主な境界は、ローカルCLI、Codex hook protocol、
agent/skill設定、Git管理されるMarkdown・JSON・TOMLファイルである。実装内にアプリケーション
サーバーやREST/GraphQL endpointは確認できなかった。

例外的なnetwork出力として、任意設定のメトリクスendpointへのbest-effort HTTP POSTがある。

## 利用者向けCLI

Codexでは `$aidlc` または `/skills` からorchestratorを起動する。配布物のCLI helpで確認できる
上位操作は次のとおり。

| コマンド | 契約の概要 |
| --- | --- |
| `next` | 現在状態から次に行う処理を決定 |
| `report` | ステージ結果を記録し、承認・遷移を反映 |
| `park` | 実行中の作業を中断可能な状態へ保存 |
| `compose` | タスクやscan reportから適応的なstage planを提案 |
| `status` | 現在のspace、intent、stage、進捗を表示 |
| `doctor` | 配置、バージョン、設定、生成物の整合性を診断 |
| `version` | AI-DLC製品版を表示 |
| `recompose` | 実行中workflowの未完了部分を再構成 |
| `intent` | intentの一覧・選択・管理 |
| `space` | workspace spaceの一覧・選択・管理 |
| `config get/set/list` | AI-DLC設定の参照・変更 |
| `plugin select/list/sync/validate/build` | pluginの選択と配布物反映 |
| `knowledge` | DocumentKBのonboard/sync/list/show/関連付け等 |

さらに内部ルーター `core/tools/aidlc.ts` は、`unit`、`state`、`audit`、`graph`、`runtime`、
`sensor`、`swarm`、`bolt`、`worktree`、`jump`、`learnings`、`validate`、`scope`、`gen`、
`workspace`、`hook`、`statusline`、`adapter` 等の決定論的ツールへ転送する。

各stage-runner skillは `aidlc-orchestrate next --stage <slug> --single` を呼び、main workflowの
`Current Stage` を進めず、単独attemptだけを開始・完了する。

## オーケストレーター契約

`aidlc-orchestrate.ts` の外部サブコマンドは次の5つに絞られている。

- `next`: graph、状態、approvalを評価して次のdirectiveを返す。
- `continue`: hook等から再開指示を運ぶ内部transport。
- `report`: 実行結果と証拠を受け、attemptを閉じる。
- `park`: checkpointを作り安全に中断する。
- `team-board`: Construction teamの読み取り専用照会。

判定をLLMだけに委ねず、状態遷移、監査、stage graph、sensor、approval gateをTypeScript CLIで
検証することが中核設計である。

## Codex hook契約

`.codex/hooks.json` はCodexイベントと同梱hookを次のように対応付ける。

| Codexイベント | 主なAI-DLC処理 |
| --- | --- |
| `SessionStart` | session開始とcompact後のmission復元 |
| `UserPromptSubmit` | human turnの記録 |
| `PreToolUse` | shell binding、stage rule配信、状態遷移・review・plan approval guard |
| `PostToolUse` | human turn、変更監査・sensor、`update_plan` とworkflow stateの同期、graph再生成 |
| `PreCompact` | compact前の状態検証 |
| `SubagentStop` | subagent終了記録 |
| `Stop` | workflow継続判定 |

Codex固有adapterがイベントpayloadを共通hook形式へ変換する。CodexにはSessionEndがないため、
閉じられていないsessionは次回SessionStart時に推論した `SESSION_ENDED` として調停される。

## 永続化ファイル契約

| 契約 | 形式 | 主な役割 |
| --- | --- | --- |
| stage definition | frontmatter付きMarkdown | phase、execution、mode、agent、sensor、遷移 |
| `stage-graph.json` | JSON | 有効stage集合、順序、依存、runner生成元 |
| scope definitions / grid | Markdown + JSON | stageごとの有効性、depth、test strategy |
| `aidlc-state.md` | 構造化Markdown | current stage、status、approval、checkpoint |
| audit shards | JSONL相当の追記記録 | 91イベント、clone別書込み、merge可能な監査証跡 |
| sensor manifest | frontmatter付きMarkdown | trigger、blocking/advisory、実行command |
| `harness.json` | JSON | harness、plugin選択、生成時設定 |
| `model-rates.json` | JSON | token/cost集計用モデル料金 |
| plugin manifest | `.aidlc-plugin/plugin.json` | plugin identity、contribution、target |
| agent definition | Markdown + Codex TOML | role prompt、model、reasoning、sandbox |
| skill definition | `SKILL.md` + optional YAML | 呼出条件、手順、表示metadata |
| DocumentKB | `index.json` + `metadata.json` + `content.md` | 文書identity、抽出内容、関連付け、tombstone |

`core/tools/aidlc-stage-schema.ts` はphase、execution、mode等を検証する。
`aidlc-sensor-schema.ts`、`aidlc-rule-schema.ts`、`aidlc-directive.ts`、`aidlc-graph.ts`、
`scripts/manifest-types.ts` が、その他の主要な境界型とvalidatorを提供する。

## DocumentKB契約

`knowledge/documents/` は利用者所有の原本、`knowledge/documentkb/` はツール所有の派生catalog。
`index.json` を失っても各文書の `metadata.json` から再構築できるが、`documentkb/` 全体を削除すると
identityとtombstoneは復元できない。ツールは利用者原本を削除する `remove` を意図的に持たない。

抽出された文書本文は「データであり命令ではない」と明示され、prompt injection境界として扱われる。

## Plugin契約

pluginは `.aidlc-plugin/plugin.json` と、`stages/`、`contributions/`、`sensors/`、`tools/` 等の
core相似treeを持つ。pluginは既存要素を上書きするより、stageやagent等を追加する。
install側の `tools/data/harness.json` が選択pluginを保持し、compose/build時にgraph、scope、runnerへ反映する。

`scripts/manifest-types.ts` の主要型は次の境界を表す。

- `DirMap` / `FileMap`: 実装から配布へのコピー対応。
- `HarnessManifest`: ハーネス識別子、出力root、ファイル対応、emit hook。
- `EmitContext`: 生成処理へ渡すroot、graph、scope等。
- `OnboardingSpec`: 共通テンプレートへ埋めるハーネス固有値。
- document extractor: 外部抽出器のargvと入出力契約。

## 内部モジュールAPI

手書きTypeScriptはES moduleのexport/importで連携する。特に `aidlc-lib.ts` は約23,333行、
`export` 宣言が約580あり、49前後の手書きモジュールから参照される中心的API面になっている。
次いで監査、graph、runtime path、manifest typeが高いfan-inを持つ。

テキストベースの静的import解析では、`aidlc-lib`、`aidlc-audit`、`aidlc-graph` と複数schemaの間に
循環依存群が見える。ただし動的importやtype-only edgeを厳密に分類していないため、これは
設計上の結合シグナルであり、runtime cycleの断定ではない。

## 外部network API

`core/tools/aidlc-metrics.ts` は、`AIDLC_METRICS_ENDPOINT` が指定された場合だけ
`text/plain` のHTTP POSTを行う。追加headerは `AIDLC_METRICS_HEADERS` から受け取り、
3秒timeoutの子プロセスとしてbest-effort送信する。失敗しても本体workflowを止めない。

この他に、フレームワーク本体が固定の業務API endpointをlistenまたはcallする実装は確認していない。
モデル呼出しやMCPは各ハーネスとagent runtimeの責務であり、AI-DLCの決定論的CLI契約とは分離される。

## 不明点・動的要素

- 有効なstage、agent、pluginは配置内容と `harness.json` で変化するため、固定件数だけで判断しない。
- Codex hook payloadの最終仕様は利用中のCodex版にも依存する。
- MCP serverは配布時に未設定で、利用プロジェクトの `.codex/config.toml` に委ねられる。
- live-modelの応答契約はモデル/provider依存であり、リポジトリ内の型だけでは完結しない。

## 主要根拠

- `docs/実装_aidlc-workflows/core/tools/aidlc.ts`
- `docs/実装_aidlc-workflows/core/tools/aidlc-orchestrate.ts`
- `docs/実装_aidlc-workflows/core/tools/aidlc-stage-schema.ts`
- `docs/実装_aidlc-workflows/core/tools/aidlc-sensor-schema.ts`
- `docs/実装_aidlc-workflows/core/tools/aidlc-audit.ts`
- `docs/実装_aidlc-workflows/core/tools/aidlc-metrics.ts`
- `docs/実装_aidlc-workflows/scripts/manifest-types.ts`
- `docs/実装_aidlc-workflows/docs/guide/12-cli-commands.md`
- `docs/配布_ai-dlc/.codex/hooks.json`
- `docs/配布_ai-dlc/.codex/tools/data/`
