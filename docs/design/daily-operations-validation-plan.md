# 日常運用を実CLIで確認する計画

状態: Accepted。ユーザーは残対応の「1.実案件で1 Intent完走」「2.日常運用の確認」に
「1番と2番をやってもらっていいですか」と直接依頼した。本計画は2の検証範囲を具体化する。
1の実案件は対象を質問中であり、対象選定後に別の作業単位で扱う。

## 背景と結果

四段階フローとKnowledge CLIはPR #131/#133で導入済み。従来の一周テストに加え、
複数の仕事、別会話・別cloneへの引継ぎ、競合、保存失敗を公開CLIから確認する。
利用者が現在の制約と復旧手順を理解できる検証結果と、再実行できるGo integration testを残す。
対象の基準はmainの22a666c3134619790328616ab95f2e2b53efca8f。

## 受入条件と順序

1. TestOperationsMultiIntent: A/Bを別sessionで選択し、Aの設定・中断がBへ混ざらない。
   同名の複数候補を勝手に選ばない。paused/waitingを新sessionで選択し同じIDで明示再開できる。
2. TestOperationsGitHandoff: state・Knowledge・ADRをGit保存して別cloneへ引継ぐ。
   ID・進捗・本文が保持され、runtimeは共有されない。旧割当やreviewを根拠なしに有効と扱わない。
   clone後に現在root/sessionを使って必要な選択・再確認を行う。
3. TestOperationsConcurrentCAS: 同じrevisionで2 processを同時起動。成功は一つ、他方は競合、
   revision増加は一回、JSONは正常。現物を再読込した明示再試行は成功する。
4. TestOperationsUnitConflicts: 公開CLIで同一Unit二重claim、重複scope、未統合依存の起動を拒否。
   正当な別worktreeへの割当は保持される。
5. TestOperationsSaveRecovery: 一時fixtureで実filesystemの保存権限を一時的に外す。
   実CLIが失敗し、元state bytes/revisionが不変。権限復元・再読込後の再試行は成功。
   障害が効かなかった環境は成功と扱わず理由を明示する。Knowledgeの保存・補助索引失敗も
   既存契約に従い、保存前失敗と保存後部分成功を区別して再読込・復旧を確認する。
6. TestOperationsGitConflict: 同じstateを別branchで更新し、Git競合を検出する。
   未解消の競合をCLIが正常stateとして受理しない。fixtureの明示した採用内容で解消後に再検査する。
   fieldの自動合成やCLIの新しい競合解消APIは作らない。

## 実施範囲

単独Go実装担当が新規 src/cmd/aidlc/operations_integration_test.go と必要な同prefix test fileを所有する。
既存journeyのbinary build、fixture helperを利用できる。失敗検証はstdout/stderr/exitを分けて捕捉し、
入力、state hash/revision、成功・期待失敗をtest出力へ記録する。必要な比較は実ファイルから行う。
検証記録は docs/ram/decisions/2026-09-08-daily-operations-evidence.md と索引、
実施手順は docs/development.md へ追記する。

今回の直接依頼は現行製品の運用検証を許可する。製品API、state形式、配布設定は変更しない。
契約違反や未対応の運用が判明した場合は、現状と影響を報告し、製品修正は具体案を提示して扱う。
Go単一バイナリ、既存依存のみ。本家AI-DLCの既承認Space/配布境界を維持し、新しい意図的差分を加えない。
一時repo/clone/worktree内でだけ障害を起こし、権限はcleanupで復元。ユーザーのAGENTS差分、
参照資料、他worktree、利用先データを保全する。認証やCodex設定を変更しない。

## 検証と完了

work_unit_id=daily-operations-validation、verification_mode=loop。順番に各testを先に追加し、
既存契約が満たされていればALREADY_GREENと記録する。人工的なREDや製品の破壊変更を作らない。
各項目のtargetedは `go test -tags=integration -count=1 -v ./src/cmd/aidlc -run '^TestOperations名前$'`。
末尾で `go test -tags=integration -count=1 -v ./src/cmd/aidlc -run '^TestOperations'` を実行し、
gofmtとdiff checkを確認する。親が末尾差分/targetedを一度確認し、独立reviewへ渡す。

安定後のread-only finalは全package test、race/shuffle、vet、tidy-diff、format/diff、integration全体。
製品binaryは無変更なのでcrossbuild・四段階長時間liveは再実行しない。
実CLIのfixture結果と実AIの証拠は区別する。既存liveのGit操作はhostが補助しており、今回も
Git操作は検証runnerが実施する。実案件での操作は別途観測する。

Issue分類は製品挙動を変えない運用検証としてユーザーリクエスト。独立review・final・CI成功後に
通常merge commitでPRをマージする。追加testと文書はGitで戻せる。
