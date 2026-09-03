# Stage完了可否を読み取り専用で判定する実装計画

- 日付: 2026-09-03
- 状態: Accepted
- GitHub Issue: [#71](https://github.com/sori883/ai-dd/issues/71)
- 実装許可: [薄いライフサイクルマイルストーン](2026-09-03-thin-lifecycle-milestone.md)とロードマップ包括承認内

## 現状と目的

現在のGo実装は、保存済みstateとStage graphから実行対象を解決し、通常Stageのrequired artifactが少なくとも1件存在するかを確認できる。しかし、Stageを完了またはapproval gateへ進めてよいかを、本家順序で評価する一つのread-only境界がない。

本変更では、Stageの完了条件metadataと証拠を受け、最初の未充足条件を決定的に返す内部APIを追加する。後続の永続化処理は、このAPIが成功した場合だけmarker変更を検討できる。

## 設計

- `graph.Stage`へ、固定stage definitionから生成される完了条件metadataのうち、この判定に必要なfieldを追加する。
- graph decoderはfieldの存在、型、許可値を検証し、sliceは既存と同様にowned copyとして保持する。
- `orchestrator`に入力証拠と判定結果の小さなvalue typeを置く。結果のzero valueは成功を意味しない。
- 判定は現在Stageとgraph Stageの整合を確認し、artifact、summary、pipeline、review、sensor、blockingの順で最初の不足を返す。
- artifact確認は`artifact.HasRequiredOutput`を呼び、通常Stageだけを対象にする。
- per-unit、CodeKB、未実装のreceipt／dispatcherを必要とする条件は、理由を明示してfail-closedにする。
- filesystem、state、audit、clock、lockを変更しない。

## 対象ファイル

- `src/internal/graph/graph.go`
- `src/internal/graph/graph_test.go`
- `src/internal/orchestrator/completion.go`
- `src/internal/orchestrator/completion_test.go`
- `docs/architecture.md`
- `docs/development.md`
- `docs/ram/README.md`
- 本計画、マイルストーン、調査記録

実装担当は上記を単独所有し、他の作業者の変更をrevertしない。

## TDD slice

1. graph metadataの正常値、欠落、型不正、許可値違反を公開`graph.Load`経由でRED/GREENにする。
2. 通常Stageでartifact不足と存在を既存artifact API経由で判定する。
3. summary、pipeline、review、sensor、blocking条件を固定順序で判定する。
4. per-unit、CodeKB、Stage不一致、不正なzero inputをfail-closedにする。
5. caller-owned input、filesystem、stateが変更されないことを確認する。

loopでは次だけを実行する。

```text
go test -count=1 -run '^(TestLoad|TestEvaluateStageCompletion)' ./src/internal/graph ./src/internal/orchestrator
```

## final検証

独立reviewでblocking findingがない固定headに対し、unit、integration、race、vet、`go mod tidy -diff`、`gofmt -l src`、`git diff --check`、6 OS/archのCLI buildと対象test binary compileをread-onlyで1回実行する。

## 互換性・依存・リスク

外部Go moduleは追加しない。固定AI-DLC 2.6.123の確認済み完了順序を採用し、新しい意図的差分はない。未実装能力を要求するStageが進まないことは、安全な段階的実装境界であり、恒久的に条件を省略する仕様ではない。

最大のリスクは、stage graph生成物のfield欠落を既定値で成功扱いすることである。必須性が判定できないfieldはfail-closedにし、既存fixtureも明示的なmetadataへ更新して回帰を可視化する。

