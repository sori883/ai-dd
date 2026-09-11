# Gitなし配置で固定Codexの親子と製品hook入口を確認

日付: 2026-09-11。Issue #171の実装前gate G0は成立。本体のGit不要化はこの確認時点で未着手。

承認済みの[具体計画](../../design/git-independent-workflow-plan.md)に従い、Codex CLI 0.153.4、基準HEAD `9360348cf0512d168b8604c85c24da322f186186` のbinaryで、`.git` のない新規projectを配置した。試験場所は `/Users/const/sori883/ai-dd-validation/git-independent-171/g0/project`、証拠は同じ親の `evidence/`。

通常TUIのproject trustと `/hooks` を用い、配置済み5つの製品hookの絶対binary/root・内容を確認して信頼した。hook trustの迂回flagは使っていない。その後、新しい `codex exec --json --skip-git-repo-check -C PROJECT -s workspace-write` を起動した。`--skip-git-repo-check` は非Gitフォルダでexecを許可する入口であり、hookやtrustを無効化するものではない。既存のグローバルhookは変更していない。

実測は次のとおり。

- SessionStartで実sessionを受け、session bindによるRule読込み、procedureとshowが成功した。終了時も `.git` は存在しない。
- initializationで許可される `aidlc-stage-planner` をnative `spawn_agent`、`fork_turns=none` で起動できた。製品registryには同session/Intent/stepと `/root/g0_planner` のbound記録が残った。Pre/Post処理が実際に通った証拠である。
- 子の親宛MESSAGEとFINAL_ANSWERの到達、wait結果のcompletedを確認した。MESSAGE本文は一部暗号化されているため、そのbytesの復号照合は行っていない。nonceは平文の子finalと親の報告で一致しており、G0は親子・hook入口の到達を成立条件とする。
- 子が試した `intent plan --help --project-dir ...` は現行のchild hookが未分類として拒否した。CLI実行成功とは扱わない。親のbind前・一般tool同時実行・begin前にも製品hookの拒否が観測された。入口が未読込だった結果ではない。
- 比較として、hostから現行CLIを明示rootなしで実行すると `resolve Git worktree root: exit status 128` で失敗した。これは今回置換する製品側のGit依存である。

Codexの終了コードは0。子の完了を回収済みで、担当予約は作成していない。任意stage完了、成果承認、同root worker、SHAの失効はこのG0では確認していない。最終候補の実機gateで別に確認する。

主な証拠は `manifest.json`、`command.json`、`exec.jsonl`、`exec.stderr`、`native-relevant-transcript.json`、`registry.json`、`baseline-no-explicit-root.json`、`exit.json`。その内容hashを `sha256.json` に記録した。生の会話本文をRAMへ転載せず、実測結果と限界だけを残す。
