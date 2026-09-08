# Issue #130 実装作業単位

計画: [4ステージ実装計画](four-stage-workflow-implementation-plan.md)。許可は同計画とRAMの直接回答。
Issue: https://github.com/sori883/ai-dd/issues/130 （機能開発）。
work_unit_id: four-stage-product-cutover。verification_mode: loop。
workdir: /Users/const/sori883/ai-dd。
starting_head: 68430a80ea6edff87f651ddeabcf7cbfc7aec3aa。
単独writerはgo_tdd_implementer。親はhandoff後に対象ファイルを編集しない。

## 所有範囲

srcの計画対象package、移行後不要になる旧製品package・原稿・専用testと、それを呼ぶCI。
docs/designの本計画と具体契約、docs/architecture.md、docs/development.md、docs/ramの新実装証拠と索引。
Go module追加禁止。AGENTS.mdのユーザー差分、docs/実装_okf-agent-memory/、参照資料、過去RAM本文は変更・削除禁止。
既存の今回の設計差分は保持する。新方式へ切替後の旧経路削除は計画と依存調査の根拠を記録する。
Goの全project検証・cross build・live実行はloopで行わない。配布E2Eのtest実装は所有範囲だが実行は親のfinal。

## 確定した運用境界

- stateはGit共有の `aidlc/spaces/<space>/intents/<id>/state.json`。
- 調整役AIが担当と独立reviewerを起動。Goは起動せず、現在割当・受理結果・gateを管理。
- 共有stateのwriterは調整役一人。Unit担当は別worktreeで担当範囲を実装し、成果を返す。
- reviewは運用上の独立レビュー。受理したsession等を確認するが同一OS権限相手の完全な著者認証は主張しない。
- 各ステージ境界と統合検証→完了にSensor＋review。未実施・fail・unknown・対象変更後の旧passでは進めない。
- stateの進捗更新自体でreview対象を古くしない。計画・成果物・コード版を対象に含め、結果欄は除く。
- ADR不要時は理由をreviewする。毎操作の作業日誌・一律ADR生成・全操作audit・独自snapshot/receipt台帳は禁止。
- 新規Intentが対象。旧KDR/33 Stageの互換・移行不要。利用データの自動削除はしない。

## 順序付きのbehaviorとtargeted command

新APIのcompile-only scaffoldは、計画の意味に沿う型・signature・空の返値だけを先に置いてよい。
公開CLIの詳細flag・JSON schemaは、計画の操作責任と既存parser/安全保存の慣例を維持して具体化し、
test実装前に本計画の付録または別契約文書へ記録する。新しい運用選択が必要なら停止して親へ返す。

| slice | 振る舞い・主なfile | exact targeted command |
| --- | --- | --- |
| S1 | flow document/store、必要な共通filestore、ID/Space/形式/CAS/破損/保存失敗 | `go test -count=1 ./src/internal/flow -run '^TestFlowStore'` |
| S2 | flow sensor、必要成果物/ADR要否/参照/対象版/循環と未知Unit | `go test -count=1 ./src/internal/flow -run '^TestFlowSensor'` |
| S3 | flow review、独立担当情報と対象版、pass/fail、stale判定 | `go test -count=1 ./src/internal/flow -run '^TestFlowReview'` |
| S4 | flow transition、4段階/待機/中断/再開/戻し/完了、飛越し・重複遷移拒否 | `go test -count=1 ./src/internal/flow -run '^TestFlowTransition'` |
| S5 | flow unit、割当/重複/依存/結果/統合/中断後確認 | `go test -count=1 ./src/internal/flow -run '^TestFlowUnit'` |
| S6 | minimal/cli、公開操作とhook、KDR dirty義務を廃止し必須Rules等を接続 | `go test -count=1 ./src/internal/minimal ./src/internal/cli -run '^TestFlow'` |
| S7 | install/workspace/okfmemory、ADR配置・配布手順・検索・Spaceの維持 | `go test -count=1 ./src/internal/install ./src/internal/workspace ./src/internal/okfmemory -run '^TestFlow'` |
| S8 | cmd main接続/旧入口除去と専用依存削除、新一周test・live harness作成 | `go test -count=1 ./src/cmd/aidlc -run '^TestFlowCommand'` |

各commandは実際のtestを実行させる。compile failure/no tests/skippedをREDとしない。
変更前の実装で失敗するrunnable assertionを確認してから最小GREEN。既に成立するものはALREADY_GREEN。
S8の依存整理では必要な共通処理と安全性の回帰testを維持し、単に失敗testを削ることをしない。

## boundaryと報告

末尾にS1–S8のtargeted群、存在する影響package（flow/filestore/minimal/cli/install/workspace/okfmemory）の
package test、変更Goのgofmt、git diff --checkを実行する。全project・race・vet・integration journeyは実行しない。
RED/GREENは /tmp/ai-dd-flow-*.log 等へ実行証拠を残し、RAMに各sliceのtest名・理由・exit codeをまとめる。
WORK_UNIT_READYまたは真のBLOCKED時だけ返す。Issue/PR、commit、push、merge、他agent起動は親が担当する。
親は末尾で全差分とtargeted群を一度確認し、独立review→final→PR/CI/mergeへ進む。
