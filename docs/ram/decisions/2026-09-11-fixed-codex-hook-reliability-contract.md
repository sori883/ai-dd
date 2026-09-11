# 固定Codexの終了通知・親宛先・worktree探索を確認する

日付: 2026-09-11。Issue #169。固定参照: Codex `rust-v0.153.4`、commit `3d2ee51ca2d5db578f328aa75e20aa22c0197c9a`。技術調査担当の一次source確認と、既存製品の読み取り・計画点検に基づく。最新mainの仕様とは区別する。

## ① 終了通知には二つの異なる経路がある

固定版のPostToolUseは、tool handlerが成功のResultを返し、そのoutputがsuccess_for_loggingを満たす場合に送られる。プロセスの終了codeだけで決まらない。

- Bashの通常結果は終了codeが非ゼロでもPost対象。CommandExecutionの終端statusは正常終了で `completed`、非ゼロで `failed`。両方とも終端として扱う必要がある。
- exec_commandがrunningを返した時点と、まだ動作中のpollにはPostがない。終了を観測したwrite_stdinの成功結果には、元execのIDによるBash Postがある。CommandExecutionのitem IDも同じ元call IDなので、構造化eventとhookを対応付けられる。
- handler自体のエラーはPostがない。sandbox拒否は通常の終端outputへ変換される特例があるため、「sandbox拒否は全てPostなし」とも扱わない。
- apply_patchの失敗はhandlerエラーになり、Postがない。成功したpatchだけPost対象。patchの完了itemはFileChangeであり、Bash用のCommandExecution判定を流用しない。

根拠: 固定tagの [registry.rs](https://github.com/openai/codex/blob/rust-v0.153.4/codex-rs/core/src/tools/registry.rs)、[context.rs](https://github.com/openai/codex/blob/rust-v0.153.4/codex-rs/core/src/tools/context.rs)、[events.rs](https://github.com/openai/codex/blob/rust-v0.153.4/codex-rs/core/src/tools/events.rs)、[process_manager.rs](https://github.com/openai/codex/blob/rust-v0.153.4/codex-rs/core/src/unified_exec/process_manager.rs)、[apply_patch handler](https://github.com/openai/codex/blob/rust-v0.153.4/codex-rs/core/src/tools/handlers/apply_patch.rs)。

届いたPostを保存ロック競合で処理できない製品経路は、G0の実製品呼出しで再現した。これは過去パイロットの全残存原因の確定ではない。有限のsessionロック再試行を修正対象にする。失敗patchでPostがないことは固定runtimeの契約として分け、メインAIが終了を確認して既存の明示recoverを行う案内を整える。架空の終了通知や時間だけの自動解除は追加しない。

## ② 新しい管理台帳を作らず、正式タスク名から親宛を求める

固定版のhook payloadには親のtask名専用fieldがない。一方、native spawnの仕様は、親 `/root/task1` から `task_3` を起動した子を `/root/task1/task_3` とする階層を明記している。返却されるtask_nameは正式なcanonical task名であり、既存製品はそれをDispatch.Canonicalへ保存する。したがって、そのpath.Dirから親を求められる。

根拠: 固定tagの [multi_agents_spec.rs](https://github.com/openai/codex/blob/rust-v0.153.4/codex-rs/core/src/tools/handlers/multi_agents_spec.rs) のspawn説明とoutput schema（確認箇所739–750、404–433）、[hook schema](https://github.com/openai/codex/blob/rust-v0.153.4/codex-rs/hooks/src/schema.rs)。製品側は `src/internal/assignment/dispatch.go` のPostSpawnとCheckTarget。

採用する実装詳細は次のとおり。新しい永続形式・CLI引数・送信者本人認証を追加しないため、①②の[直接承認](2026-09-11-hook-reliability-repair-approved.md)内で進める。

1. 子のagent_id/type、session、turn、tool ID、現在の工程・担当資格は既存どおり検査する。
2. 一回の検証済みregistry snapshotから、同session・Space・Intent・step・定義版の適格なbound記録を集める。全roleを通して親pathが一意で、その集合に送信者と同じroleが最低一件必要。
3. CanonicalはPostSpawnと同じ名前空間、正規形、末尾TaskName対応を検査してからpath.Dirを使う。不正pathを正規化して許可しない。worker記録は対応予約のowner/context一致・未解放も同snapshotで確認する。
4. send_messageの宛先が唯一の親canonical pathと完全一致した場合だけ許可する。相対名、不明・他宛先、再委譲、共有保存は拒否する。
5. 初回boundがなく適格pendingだけの場合は短く再読取りできるが、session/assignmentロックを保持して待たない。待機後は親binding・現在工程から検査し直す。不正・曖昧・uncertainは即時拒否、期限切れは保持・拒否とする。

これは送信先の制限であり、agent_idと特定の予約を照合する本人認証ではない。同roleの適格な別記録があれば、解放済みの特定の子をagent_idで区別することはできない。報告を成果承認・人間回答・割当解放へ自動昇格させず、Pre/Postで親の共有記録を変更しない。この限定は既存の子hook契約を維持するものである。

## ③ linked worktreeでは主checkoutのhookを読む実装だった

固定tagのconfig loaderは、linked worktreeのproject設定を保持しつつ、hookの探索先を主checkout側の `.codex` へ切り替える。hooks.jsonだけでなくinlineのhook設定も主checkout側を採用する。hook discoveryは、その切替後のfolderからhooks.jsonを読む。

根拠: [config loader](https://github.com/openai/codex/blob/rust-v0.153.4/codex-rs/config/src/loader/mod.rs) のroot_checkout_hooks_folder_for_dir・merge_root_checkout_project_hooks（確認箇所1127–1138、1670–1687、1830–1854）、[config state](https://github.com/openai/codex/blob/rust-v0.153.4/codex-rs/config/src/state.rs)、[hook discovery](https://github.com/openai/codex/blob/rust-v0.153.4/codex-rs/hooks/src/engine/discovery.rs)。これは[前回の比較実測](2026-09-11-pilot-hook-discovery.md)と整合する。

したがって、linked worktree自身のhooks.jsonを読む挙動へGo installerだけで変更することはできない。候補はCodex側の変更を待つ・具体版を比較する、または既知の独立clone運用を使うこと。主checkoutへ単に製品hookを入れると、hook command中の絶対project-dirも共有されるため、それだけで各worktreeの正しいIntentを保護できると説明しない。

今回は③を調査のみとして完了する。Codexの更新、主checkoutへの配置、既存worktreeの変換、新しい製品診断CLIは実施していない。新しい専用試験rootで製品hookが未列挙だった件は、config/readがproject layerの信頼未確認を明示しており、linked worktreeの現象とは別である。
