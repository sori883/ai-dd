# Intent実行計画のトップhelp残存一覧修復

Issue153、work_unit_id=intent-execution-plan-review-repair-2、verification_mode=loop。
開始HEAD: `2d4821299c3dcbf1777b5f4b5f42caf6c4b0a965`。親が再reviewのP2を既存直接承認範囲の通常修復として許可した。

トップhelp末尾に残った旧4段階一覧を、initialization→discovery必須、残るarchitecture-analysis/planning/tdd/integrationは計画選択とする説明へ修正した。
完了条件もSensor・独立review・成果の会話承認と明記した。旧snapshot期待値も同じ文言へ追従した。

`TestExecutionPlanReviewHelp` を先に拡張し、3入口（引数なし/help/--help）に旧一覧が残ることのrunnable REDを確認した。
正確なcommand: `go test -count=1 ./src/internal/cli -run '^TestExecutionPlanReviewHelp'`。
RED: tool chunk71976b、exit 1、各入口で `stale help "Stages: discovery, planning, tdd, integration."`。
GREEN: 同command、tool chunkc8c599、exit 0。

所有はcli実装・help snapshot・回帰test、本RAMとindexだけ。実機/E2E/全体/race/vetは未実行。
終了HEADは本記録を含む修復commitとして親へ報告する。

末尾確認: 指定command exit 0（0.337s）、`go test -count=1 ./src/internal/cli -run '^TestRun_Help$'` exit 0（0.169s）。
gofmt -lは空、git diff --check exit 0。変更sourceのSHA256:

```text
0a5de6c93e5801c47683d6f50f916ade92d7adad5b74cefcb709b184bbd11ad6  src/internal/cli/cli.go
6c791b8d8fe8ee3116b61f52c9c4e10592baab6950f46d66fb27928695f8d239  src/internal/cli/cli_test.go
4b0923acd7305fb68531f5e2d621b59f38fba61e190b1d18306b72fd9574460d  src/internal/cli/execution_plan_review_test.go
```
