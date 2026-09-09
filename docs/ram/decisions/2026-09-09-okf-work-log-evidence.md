# OKF作業記録の実装証拠

Issue: #149。work_unit_id: `okf-work-log`、verification_mode: `loop`。
開始HEAD: `444fb6f9a2a488a7134a5fd06dcb6914bf11fd95`。
[直接承認](2026-09-09-work-log-okf-request.md)と[実装計画](../../design/okf-work-log-plan.md)の全項目を単独writerで実装した。

## 保存契約

差戻し記録を`aidlc/spaces/<space>/knowledge/log/<intent_id>-work-log.md`へOKFとして保存する。
初回の土台も正しいmetadataを持つ文書にし、途中保存でKnowledge検索を壊さない。
既存OKF serializerとmetadata builderを再利用し、未知metadataを保持する。
pendingには保存前の`log_hash`と完成後の`log_after_hash`を保存する。pending永続化後に固定された`at`で完成版を組み立て、
完成hash一致時の再試行は文書を書き直さずstateを確定する。pending保存前の失敗では有効な空本文の土台が残る場合があるが、要求は未保存であり、再試行で新しい時刻を選べる。旧pendingは完成hashがないため拒否する。
schema_version=4を維持し、旧記録の自動移行・削除・二重書込みは行わない。
文書全体の256 KiB検査は最初の保存前に行う。FIFOは開く前の通常ファイル検査で拒否する。

## TDD実測

1. 保存先・metadata・追記・検索
   - `go test -count=1 ./src/internal/flow -run '^TestOKFWorkLogDocument'`
   - RED: exit 1。`knowledge work-log missing: statat aidlc/spaces/default/knowledge: no such file or directory`。
   - GREEN: 同command exit 0。生成日時、type/intent_id、Intent名、未知metadata、2回追記、検索filter、旧記録非変更を確認。
2. 保存途中・再試行
   - `go test -count=1 ./src/internal/flow -run '^TestOKFWorkLogRecovery'`
   - RED: exit 1。fresh/existingのlog/final境界で`missing completed document hash`、旧pendingで`invalid pending hash accepted`。
   - GREEN: 同command exit 0。各保存境界の失敗、同一要求の1回追記、別要求拒否、完成文書bytesとpending保存後の時刻保持を確認。
   - 不正文書、別type/intent、directory/symlink、上限、改変・欠落への追加assertionは既存の正しい拒否を維持（ALREADY_GREEN）。
   - FIFO追加後RED: 同command exit 1、`reopen blocked opening a fifo`。leaf事前検査追加後GREEN: exit 0。
   - 既存reopen回帰fixtureを同容量の正しいOKFと新pathへ更新。
     `go test -count=1 ./src/internal/flow -run '^(TestOKFWorkLog|TestReopenLog)'`はexit 0。
3. 公開CLI・配布手順
   - `go test -count=1 ./src/internal/minimal ./src/internal/cli -run '^TestOKFWorkLog'`
   - 公開parserからreopen/search/showを実行しSpace/Intentを分離するtestはALREADY_GREEN（minimal exit 0）。検索production変更は不要。
   - helpはRED: cli exit 1。新path、type、search/show例、generated.at説明が欠落。更新後は両package exit 0。
   - `go test -count=1 ./src/internal/install -run '^TestOKFWorkLogInstalledGuidance'`
     は配布4 Stage/WORKFLOWのpath・検索例欠落でexit 1、手順更新後exit 0。
4. 結合fixture
   - `flow_journey_integration_test.go`を新pathへ更新し、実CLI検索とshowの内容一致assertionを追加。
   - loopでは結合testを実行していない。実行証拠は親のfinal検証で記録する。

## 作業単位末尾

上記3つのtargeted commandを再実行し全てexit 0。
`go test -count=1 ./src/internal/flow ./src/internal/minimal ./src/internal/cli ./src/internal/okfmemory ./src/internal/install`
は5 package全て成功、exit 0。変更Goファイルにgofmtを適用し、`git diff --check`はexit 0。
全project test/race/vet/cross-build/結合E2Eはloopでは実行していない。
Serenaを対象worktreeへ有効化した。gopls MCPは公開toolsに存在しないため、ローカルsource確認とtargeted Go testを使用し、追加toolを導入していない。

## 後続gate

独立review、親のread-only final、対象HEADのGitHub checks、PR mergeとIssue closeは親担当。
ファイルの存在はstate revision確定の代わりにならない。生成日時は人間の承認・検証済みを表さない。
FIFO fixtureはPOSIXのmkfifoを使用しWindowsではskipする。macOSの今回実測では実行して成功した。

## 独立レビュー後の文書修復

work_unit_id: `okf-work-log-doc-repair`、verification_mode: `loop`。
開始HEAD: `0b127b7a69185a850ef41cd6f9e124d078aed41f`。Issue #149の直接承認範囲で親が現在文書の所有範囲を拡張した。
P2指摘に対し、`docs/development.md`とRAM索引をKnowledgeのlog/配置と有効OKFの初期土台へ訂正した。
検索metadata・intent_id完全一致・showによる本文表示も明記した。計画と本証拠の時刻固定説明はpending永続化後へ限定した。
実装挙動や履歴記録は変更していない。文書だけの修復なので人工REDとGo testは実施しない。
`rg -n 'Intentのwork-log|空の通常file|Intent配下の作業記録Markdown' docs/development.md docs/ram/README.md`はexit 1（旧現行説明の一致なし）。
`git diff --check`はexit 0。全体検証は親のfinalへ残す。
