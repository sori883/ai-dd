# 知識一覧の既存実装を補強して採用する

- 日付: 2026-09-05
- 状態: Accepted（修正・検証・採用・mergeまでの直接承認）
- Issue: [#93](https://github.com/sori883/ai-dd/issues/93)
- 置換対象: [手順不備と回復方針](2026-09-04-knowledge-roster-tdd-recovery.md)の採用確認待ち。
- 関連: [元計画](2026-09-04-knowledge-roster-plan.md)、[検証範囲](2026-09-04-knowledge-validation-scope.md)、[段階別handoff](../../tdd-handoff.md)。

## 背景・承認・得られる結果

配置Markdownから担当AI向けの知識一覧を選ぶ内部APIに、知識の誤除外、警告順の不具合、容量境界・
日本語・実ファイル更新反映のテスト不足が残る。大半の既存testが実装後に追加された手順不備もある。
「既存コードを残して新しい手順で修正・検証し、再レビュー後に採用・mergeする」提案に対し、
ユーザーは「はい、じゃあお願いします」と回答した。この回復方針への直接承認である。

既存コードを保持し、実際の不具合には修正前に回帰testを失敗させる。既に正しい条件は初回成功
（ALREADY_GREEN）と記録する。過去に実装前REDがあったことにせず、完成コードを消して人工REDを作らない。
各RED/GREENの最終返却後に親が差分・同じtestを確認し、親の受入記録と全対象fileの機械的hash照合で進める。

## 契約と境界

本家は固定AI-DLC 2.6.123を参照する。Minimal（知識を絞る設定）は対象stage/owner表がなければ全保持。
警告はplugin metadata→persona→各directoryの深さ優先読込の発生順とする。
根拠は本家core/tools/aidlc-orchestrate.ts:2740–2787,2943–3060。これらへの修正は新しい意図的差分ではない。
承認済みUTF-16固定順は維持する。本家localeCompareより再現性が高い一方、日本語・大文字小文字等で
順序と8 KiB内に残る文書が変わる可能性は維持される。

容量は8,192/6,144 bytesの既存規則を機械的に適用し、詰め直し等を加えない。供給元は利用先.codex/・
aidlc/spaces/で、毎回読み直す。埋込み・cache・src/core直接参照・OKF変換・外部moduleは加えない。
開発testはMac/Linux fixtureを許容し、製品のWindows対応は削らない。cross compileとnative実行を区別する。
今回の採用は内部の知識一覧部品であり、Codexで本文を読む後続接続まで完成したとはしない。

## 依頼単位と所有権

Goはgo_tdd_implementerが単独writerとなる。各行は1つの振る舞いのREDから始め、親が失敗を再実行して
確認した場合だけ同じ行のGREENを別依頼にする。初回成功なら本体変更なしで返す。追加scaffoldは不要。
表のfile名はsrc/internal/knowledge/配下。GREENでは受入済みtestを緩めない。

| slice_id | 期待動作・test名 | RED所有 | GREEN候補 |
| --- | --- | --- | --- |
| minimal-table-boundary | 対象表なしのstage/ownerでは無効plugin知識も保持。TestBuildRosterMinimalKeepsPluginKnowledgeOutsideTable | minimal_test.go | minimal.go |
| warning-source-order | plugin→persona→DFSの警告順と容量内に残る先頭。TestBuildRosterWarningsFollowSourceOrder | roster_test.go | roster.go、read.go |
| path-budget-boundary | 8,192 bytes一致・1 byte超過、特殊文字、正確な省略件数、後続の詰め直しなし。TestBuildRosterPathBudgetExactBoundary | budget_test.go | budget.go（失敗時のみ） |
| warning-path-text | 警告の表示pathは特殊文字を二重escapeせず本家と同じ生文字列を保持。TestBuildRosterWarningPathsPreserveLiteralCharacters | roster_test.go | roster.go、read.go、plugins.go |
| warning-budget-boundary | 6,144 bytes一致・1 byte超過、要約予約・省略件数、特殊文字。TestBuildRosterWarningBudgetExactBoundary | budget_test.go | budget.go（失敗時のみ） |
| space-fresh-read | 同じ借用Framework/Space Rootで日本語文書の本文編集・追加・削除を次回へ反映。TestBuildRosterIntegrationRefreshesJapaneseSpaceKnowledge | read_integration_test.go | read.go（失敗時のみ） |

公開BuildRosterのPaths/Warningsを検査する。容量の期待値は実装のサイズ計算helperではなく独立したJSON
wire文字列で求め、引用符・制御文字・<>&・U+2028/U+2029・日本語・補助平面文字を含める。
commandはGOTOOLCHAIN=go1.26.8 go test -count=1 -run '^<test名>$' ./src/internal/knowledge。
実Rootの行だけ-tags=integrationを追加する。gofmtは各返却前に所有fileだけへ適用してtestを再確認する。
関連する既存targeted testだけは追加実行できる。予期しない別の振る舞いが必要なら親へ返す。

親は本記録・RAM索引・元計画の追記・Issue/PRを管理する。既存CI integration接続とarchitecture/development
説明は元計画の範囲を維持する。src/core原稿、既存公開API、CLI、人間承認・state/audit、無関係なHTML、
RAM整理は変更しない。

## 完了条件・検証・リスク

全sliceを親が確認した後、base=a54da90a6f09d73c6dd195e91172ab232acd66d7と固定headで独立reviewする。
安定後に親が元計画のread-only finalを行う: 全package test/race/integration race、通常/tag付きvet、
go mod tidy -diff、format check、git diff --check、変更Go fileのgopls check、6構成CLI/test cross compile、
原稿140件とLICENSEのhash確認。Goは1.26.8。公開経路未変更なので公開CLI配布E2Eは対象外。
現headのGitHub push/PR全16checks成功後にmergeし、main反映・Issue closeを確認する。
新しい不一致は承認済み契約内で1件ずつRED/GREENへ戻す。新しい仕様や権限の選択は停止する。
永続data移行はなく、問題時は通常の修正PRで戻せる。過去の手順不備と今回の実測証拠は区別してPRに残す。

## 同じ承認範囲内で判明した警告表示の補修

固定head 094fec0の独立した限定reviewで、6箇所のwarningが表示pathをGoの`%q`で再escapeする
不具合を確認した。本家2.6.123はaidlc-orchestrate.ts:2760,2779,2899,2935,2993,3002で
二重引用符内にpathを生補間する。引用符、backslash、制御文字、U+2028/U+2029で返却文字列と
6 KiBに収まる警告が変わるため、warning容量testの前に独立したwarning-path-textを追加する。
公開BuildRoster経由の6種類の警告を完全一致でRED確認し、外側の引用符を保つ文字列補間へ直す。
validation、path変換、error本文、容量algorithmは変更しない。元計画の「本家形式の警告」と
特殊文字の容量契約を満たす通常bug修正であり、新しい意図的差分や追加承認を要する設計変更ではない。
