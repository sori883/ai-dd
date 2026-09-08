# 日常運用の実CLI検証結果

状態: loop完了、親boundary・独立review・final待ち。Issue #134、work_unit_id `daily-operations-validation`。
開始/終了HEAD `22a666c3134619790328616ab95f2e2b53efca8f`。製品code/API/state/配置設定は変更していない。
[直接依頼](2026-09-08-pilot-and-daily-operations-request.md) と
[受入計画](../../design/daily-operations-validation-plan.md) に従った。

## 実測結果

| test suffix | 実測と判定 |
| --- | --- |
| MultiIntent | ALREADY_GREEN。Aの設定/中断がBのbytesとsessionへ混入せず、paused/waitingから新sessionで同ID再開。同名曖昧選択はexit2で旧選択不変 |
| GitHandoff | ALREADY_GREEN。Git保存/cloneでID・進捗・Knowledge/ADR・配置bytes保持。runtime不在、新session選択/resume成功。旧Unit confirmと旧review acceptは割当runtime欠落の具体診断でexit1 |
| ConcurrentCAS | ALREADY_GREEN。同revisionの2 processを両方Start後にWait。一成功、一競合exit2。現物JSON正常、revision一回増加。旧revision再送はrevision conflict、新revisionで明示再試行成功 |
| UnitConflicts | ALREADY_GREEN。二重claim・scope重複・未統合依存はexit2でstate bytes不変。別worktree/別sessionの独立Unitは両方running、割当root/session/run ID保持 |
| SaveRecovery | ALREADY_GREEN。実chmodでstate/Knowledge保存前失敗exit1、元bytes不変。権限復元と再読込後に保存成功。索引をdirectoryにした障害はConcept保存済JSON/hashとexit2、障害復旧後にmetadata変更で索引更新とcheck成功 |
| GitConflict | ALREADY_GREEN。同stateを別branch更新しmerge exit1と競合markersを実測。CLI showはJSON不正のexit1で未変更。fixtureが左側state全体を明示採用後、ID/revision/本文・競合解消を再確認 |

既存挙動へのtest追加なので人工REDや製品変更はない。以下の誤ったfixture期待を
INVALID_TEST_FIXTUREとして訂正し、製品REDに数えていない。

- 欠落runtimeのfilesystem errorをexit2とした期待を、exit1かつ該当units/reviews path診断へ訂正。
- 旧reviewを試すcheckoutは版不一致が先に拒否されるため、cloneの現在版checkoutへ揃えて割当欠落を観測。
- 同時processの敗者がrevision照合前の排他lock競合になることがある。既存診断のlock名・file existsを
  限定照合して受理し、別途旧revision拒否を必須検査した。
- 索引directory障害はpath名でなく `Concept saved; bookkeeping failed: not regular` と診断される。
  保存済hashと実体を比較し、exit2とこの具体診断へ期待を訂正した。

## 運用上の制約

Gitで共有する正本とruntimeは別であり、clone後に旧worker/reviewerの割当を根拠なく再利用できない。
旧Unitのneeds_confirmationは維持されるが、runtimeの失われたcloneでconfirmの成功までは実証していない。
配置hookは元の絶対rootを保持していることを現物で確認し、移転後rootへ書き換えていない。
従って直接CLIの引継ぎと、配置済AI環境の移転完了を同一視しない。これらは既存制約で、今回新APIを導入しない。
実AIの会話は起動せず、Git操作は一時fixture内でtest runnerが実施した。
権限はcleanupでも復元し、障害が効かなければテストは失敗する。ユーザーAGENTS/参照資料は保全した。

## 再実行

各suffixについて以下を個別に実行し、末尾でprefix全体も実行した。すべてexit0、skip/0件なし。

```sh
go test -tags=integration -count=1 -v ./src/cmd/aidlc -run '^TestOperationsMultiIntent$'
go test -tags=integration -count=1 -v ./src/cmd/aidlc -run '^TestOperationsGitHandoff$'
go test -tags=integration -count=1 -v ./src/cmd/aidlc -run '^TestOperationsConcurrentCAS$'
go test -tags=integration -count=1 -v ./src/cmd/aidlc -run '^TestOperationsUnitConflicts$'
go test -tags=integration -count=1 -v ./src/cmd/aidlc -run '^TestOperationsSaveRecovery$'
go test -tags=integration -count=1 -v ./src/cmd/aidlc -run '^TestOperationsGitConflict$'
go test -tags=integration -count=1 -v ./src/cmd/aidlc -run '^TestOperations'
```

gofmtとgit diff --checkを実施。全package/race/vet/長時間live/commit/GitHub操作は実施していない。

## 独立review修正: Gitのlocale依存を除去

P2: mergeの人間向け出力に英語CONFLICTが含まれるという条件を除去した。
exit1、git ls-files -uの未解消エントリ、保存stateの競合markersを全て必須として維持する。
fixture判定の訂正なので人工REDは作らず、TestOperationsGitConflictのtargeted成功を確認した。
gofmtとdiff checkも成功。製品挙動の変更はない。
