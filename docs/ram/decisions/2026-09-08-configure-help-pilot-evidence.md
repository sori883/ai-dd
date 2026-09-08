# configure help実案件のTDD証拠

状態: 実装loop完了、親の境界確認・独立review・統合検証待ち。Issue #135。
work_unit_id `configure-help-pilot`、worktree `/private/tmp/ai-dd-configure-help-pilot`。
開始/終了HEADは `22a666c3134619790328616ab95f2e2b53efca8f`。
[直接承認](2026-09-08-configure-help-pilot-approved.md)と[計画](../../design/configure-help-pilot-plan.md)に従う。

## 実装結果

intent configure --helpへUnitなし/ありの完全な設定JSON例を追加した。
文字列・真偽値・配列の型、新規Unitのpending/空成果commit、現在HEADの40桁commitへの置換、
実projectのKnowledge/Scope/受入/テストへの置換、既存Unit進捗を戻さないことを説明する。
製品API/schema/実行挙動は変更していない。例のコピーだけで検証済みとは扱わない。

## TDD

1. `go test -count=1 ./src/internal/cli -run '^TestConfigureHelp'`
   - test先行のRED: exit1、JSON例を2件要求したが0件。
   - help実装後GREEN: exit0。2つのJSON構文、Unit fieldと初期値、型・置換・進捗説明を確認。
2. `go test -tags=integration -count=1 ./src/cmd/aidlc -run '^TestConfigureHelpExamples$'`
   - ALREADY_GREEN: exit0。実binaryのhelpから例を抽出し、fixtureの実HEADへ置換。
   - Unitなし/あり両例で公開configure成功、discoveryのfixture reviewからplanningへ進み、
     planningで再configureとSensor passを確認。現物Knowledgeを用意し、code版は実Git commitを使う。
   - test内reviewは決定的fixtureであり、実案件の独立AI reviewを代替しない。

末尾に上記両targetedを再実行しexit0。gofmt、git diff --checkも成功。
実装担当は全test/race/vet/crossbuild/live/commit/GitHub操作を実施していない。

実行ログ: `/tmp/configure-help-red.log`（RED）、`/tmp/configure-help-green.log`（GREEN）、
`/tmp/configure-help-examples.log`（実CLI初回）、`/tmp/configure-help-boundary.log`（末尾）。
これらは親が製品側TDD成果物として取り込める検証ログであり、実装担当は親所有のIntent/state/Knowledgeを編集していない。
配置済みskill/hooksと配布原稿を混在させず、他worktreeにも書き込んでいない。
四段階の最終実施結果は親が後続の統合検証・完了時に記録する。
