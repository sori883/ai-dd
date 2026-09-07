# M1 最小 Intent 実装の loop 証拠

対象は Issue #128、work unit `m1-minimal-intent-journey`、`verification_mode=loop`。
実装許可は [直接承認済み計画](2026-09-08-m1-implementation-plan.md)。開始 HEAD は
`fc8bd7ea633f0c1ad8cc605233572a6106b3b911`、終了時も同じ。実装担当は単独で編集し、commit・GitHub操作・live model 起動は行っていない。

## 実装と TDD の記録

以下の command は各行の runnable な assertion RED（exit 1）から GREEN（exit 0）を確認し、
work unit 末尾にも同じ targeted command で exit 0 を確認した。

| 項目 | 観測した初期 RED | exact command |
|---|---|---|
| 1 配置 | fresh assets 不在、既存配置/不正 symlink を受理 | `go test -count=1 ./src/internal/install -run '^TestInstall'` |
| 2 Space | 新 OKF tree/Rule がない、不正 default Rule を受理 | `go test -count=1 ./src/internal/workspace -run '^TestCreateSpace'` |
| 3 OKF | metadata roundtrip、検索、予約 path、bookkeeping が空実装 | `go test -count=1 ./src/internal/okfmemory -run '^Test(Parse\|Search\|Validate\|Bookkeeping)'` |
| 4 KDR | 六節、同一 ID/CAS、同名解決、repair が空実装 | `go test -count=1 ./src/internal/kdr -run '^Test(Document\|Resolve\|Store\|Repair)'` |
| 5 CLI | 新 command が runtime に渡らない、不正引数を厳密拒否しない | `go test -count=1 ./src/internal/cli -run '^Test(Install\|Intent\|KDR\|Memory\|Minimal)'` |
| 6 session/hook | bind・未記録・tool slot・必須 Rule の振る舞いが未実装 | `go test -count=1 ./src/internal/minimal -run '^Test(Session\|Hook\|Rules)'` |
| 7 一周 | 実 binary の install が minimal runtime unavailable | `go test -tags=integration -count=1 ./src/cmd/aidlc -run '^TestMinimalJourney'` |

表の Markdown escape `\|` は shell command では `|` として使用する。
Space の既存 integration のうち、契約変更に直接対応する tree/default Rule 期待だけを更新し、
`go test -tags=integration -count=1 ./src/internal/workspace -run '^TestCreateSpace'` も exit 0。

追加の実 RED→GREEN は、予約 index/log 検証、Rule 更新後の再読込、正本 KDR 直接 patch 拒否、
SessionStart の配置済み skill 読込、KDR show の読みやすい本文欠落、memory show の原本 hash 欠落。
`TestMinimalJourneyEvidence` の必要 event 欠落、`TestMinimalJourneyRejectsMissingDenial` も
runnable assertion RED→GREEN。モデル自己申告ではなく、受信 event と session 前後状態を検査する。

限定的な手順逸脱として、一般 memory create/update の service 実装は独立した直接 test より先行した。
実装を戻して RED を演出していない。後から追加した `TestSessionMemoryWritesAndSearchPreserveSelection`、
`TestSessionMemoryBoundaries` は ALREADY_GREEN（exit 0）。hash 競合、metadata 脱落拒否、KDR 配下拒否、
Space 隔離、本文保存後の index failure と部分結果を直接検査する。
`TestSessionMemoryShowOriginalHash` は実際に原本 bytes と hash の欠落で RED となり、修正後 GREEN。
`TestSessionDirtySwitchAndRecovery`、`TestMinimalJourneyBoundaries` も既存機能に対する ALREADY_GREEN。
独立 reviewer はこの実装先行の限定範囲も確認する。

## 受入項目と検証入口

| 受入 | test |
|---|---|
| fresh install/default Rule、名前正規化、新 OKF tree | TestInstallFresh、TestCreateSpaceOKF |
| metadata/名前/予約/duplicate/未知 field、検索と選択分離 | TestParseRoundTrip、TestParseRejectsInvalid、TestSearchIntentID、TestValidateReservedFiles、TestSessionMemoryWritesAndSearchPreserveSelection |
| 名前で作成/一覧/切替、同名候補、CAS、同じ ID、repair | TestResolveAmbiguousAndRename、TestStoreCreateUpdateAndConflict、TestRepairPartialAndMissing、TestMinimalJourney |
| 必須 Rule 全文、再読込、skill bootstrap | TestRulesFullTextAndLimit、TestRulesUpdateRequiresFreshRead、TestSessionStartLoadsPlacedSkillOnly |
| 無選択拒否、実行許可、同じ hook ID の解放、Stop 再入 | TestHookGuardsAndTerminal、TestMinimalJourneyBoundaries |
| 質問待ち記録、後続入力の再要求、中断/別会話再開 | TestMinimalJourneyBoundaries |
| Rule 欠落、CLI 消失の診断・未記録維持・復旧 | TestMinimalJourneyBoundaries |
| 長時間 process 中の更新競合、誤 ID Post 拒否、終端 | TestMinimalJourneyBoundaries（実 subprocess）、親実行済み TestMinimalHookProbeLive（Codex async wire） |
| 非live RED/GREEN 試作、知識1件、別session再開 | TestMinimalJourney |
| 実 model と配置済み skill、canary、独立 read-only reviewer、再開 | TestMinimalJourneyLive（親が実行） |

live を起動しない通常確認は前表の command 群。親が使用する実 model command は次のとおり。

```sh
AIDLC_MINIMAL_JOURNEY_LIVE=1 go test -tags=integration -v -count=1 -timeout=20m ./src/cmd/aidlc -run '^TestMinimalJourneyLive$'
```

固定 Codex 0.153.4 / macOS arm64 / gpt-6-astra / medium。1 model 呼出は最大5分。
writer は workspace-write / approval never、reviewer は別 Git checkout / read-only。
HOME/CODEX_HOME と認証は継承し、ユーザー config/trust は変更しない。
実際に配置した製品 hook を test 専用 Go relay が呼び、返す判断は一切変えない。
relay は raw JSON、製品出力、前後 session を一時 evidence に残す。stdout/stderr とレビュー本文も保持する。
CLI 引数の trust は canonical root に対する map 全体指定。ユーザー hook が併存し得るが、親が確認済みの
Stop 音声等を test evidence と取り違えない。

本体 live は未実行であり、実モデルでの成功は未観測。質問待ち・故障・競合は deterministic integration で検査し、
live による hook transport の成立は既存 preflight 証拠に分離している。実際の model が全手順を完遂することは
この loop の成功として主張しない。live raw evidence は親が確認する。

## 保存と互換性の境界

本文・index・log は別保存で、途中 failure を診断して同じ ID で repair する。
一時状態は `aidlc/.runtime/`、正本は Space `knowledge/`。製品 transcript/audit 依存はない。
故障 hook の OS 強制停止保証は置かず、導入時の発火確認と故障時停止・復旧を利用手順へ記載した。

本家固定 AI-DLC 2.6.123 の Stage/state/marker/audit 中心の方式に対し、承認済み M1 は OKF KDR と
会話単位の記録へ変更する。利用者の名前操作は維持し、新しい Space 内容と保存・再開形式は旧方式へ自動移行しない。
この意図的差分と理由は計画に直接承認されている。旧 code/data 経路は削除していない。
既存 yaml module 以外の追加 dependency はなく、参照実装の source copy は行っていない。

## 境界確認

変更 Go files へ gofmt 適用。全7 targeted と Space affected integration は末尾で exit 0。
全package test、race、vet、cross-build、配布E2E、live model は loop では起動していない。
既存 CI gate を保持し、TestMinimalJourney prefix の非live integration を追加した。
独立 review、read-only final、GitHub checks、PR/merge は親が担当する。

## 独立 review 修正: m1-review-repairs

開始 HEAD `362d311cf44acae13ca0189413fd1d684de61e3b`、Issue #128、同計画の直接承認内。
以下は前節の初期 live 判定器および「境界は deterministic のみ」の説明を置換する。
`verification_mode=loop` の単独 writer による修正であり、live model は起動していない。

1. P2: `checkBookkeeping` の単なる substring 判定を修正した。
   `TestSessionRejectsCommentOnlyBookkeeping` は comment 内の ID だけで check/bind が通ることを
   runnable assertion RED（exit 1）で確認し、実 link 行と日付付き action を確認する実装後 GREEN（exit 0）。
   `TestSessionBookkeepingActiveEntries` は正常な Creation、未日付、不正日付、未知 action、fence、
   未閉鎖 comment を区別する。未閉鎖 comment の拒否と同 ID repair 復旧も実 RED→GREEN。
   修復項目を comment の外へ置き、`TestBookkeepingRootIndexFrontmatter` の RED→GREEN で
   root index の OKF frontmatter を先頭に保つことも確認した。独自 audit は追加していない。
2. P1: live main の成功条件を強化した。
   `TestMinimalJourneyRejectsWeakEvidence` は旧判定器が空の二会話を成功にする RED を確認。
   `TestMinimalJourneyEvidence` は実行 RED/GREEN 欠落、test 不在、exit 不一致、source 不変、test 差替え、
   第二 session の bind/update/clean Stop 欠落、異なる ID、誤 Post ID を RED→GREEN で拘束する。
   metadata だけの変更と再開時の本文差替えも `TestMinimalJourneyRejectsMetadataOnlyRecord`、
   `TestMinimalJourneyRejectsDifferentResumeContent` で実 RED→GREEN。
   model が固定 Go helper executable を Bash から起動し、helper 内の実際の
   `go test -json -count=1 -run '^TestAdd$' .` の exit/stdout、source/test bytes を JSON で返す。
   同じ固定 command の実 Pre/Post と raw Post stdout に結び付け、TestAdd の run/fail/pass を確認する。
   helper の観測は製品の権威・判断・永続状態には使わない。モデル自己申告の検証結果は採用しない。
   `TestMinimalJourneyRunner` は実 subprocess で RED/GREEN の process 証拠を確認（ALREADY_GREEN）。
3. 計画の live 境界を `TestMinimalJourneyBoundariesLive` に実装した。
   6 checkpoint は「質問待ち」「同 session resume による回答・記録後の追加操作と再要求」
   「Rule 欠落」「CLI executable 消失」「同 session の復旧・20秒 Bash と KDR update 競合」
   「別 session の同 ID 再開」。各 model 呼出は最大5分で、故障は専用 temp だけへ注入する。
   relay は製品の判断を変更せず、失敗時も raw 入力・process 診断・前後状態を記録する。
   `TestMinimalJourneyBoundaryEvidence` は phase 欠落、同 session 違反、早期終端、ID 不一致、競合欠落を
   runnable assertion RED→GREEN で検査。回答後の clean Stop 欠落も RED→GREEN。
   `TestMinimalJourneyRelayDiagnostics` は実行不能 CLI の raw 診断と未記録保持を実 subprocess で確認した
   （ALREADY_GREEN）。OS 強制停止保証は置かない。

最初の boundary fixture の試験時に unused import による compile failure が一度あり、RED として数えていない。
削除後に実行された assertion failure を上記 RED 証拠とした。

親が行う live command は二つに分ける。

```sh
AIDLC_MINIMAL_JOURNEY_LIVE=1 go test -tags=integration -v -count=1 -timeout=20m ./src/cmd/aidlc -run '^TestMinimalJourneyLive$'
AIDLC_MINIMAL_JOURNEY_LIVE=1 go test -tags=integration -v -count=1 -timeout=35m ./src/cmd/aidlc -run '^TestMinimalJourneyBoundariesLive$'
```

前者は実 RED→実装変更→GREEN、独立 reviewer、二 session の同じ本文/ID の bind と内容更新・終了を確認する。
後者は計画の質問・再要求・Rule/CLI 故障・長時間競合・再開を実 Codex hook 環境で確認する。
いずれも未実行であり、成功とは主張しない。不明な payload/不足 event は失敗として evidence を残す。

修正末尾の affected targeted command:

```sh
go test -count=1 ./src/internal/okfmemory -run '^Test(Parse|Search|Validate|Bookkeeping)'
go test -count=1 ./src/internal/kdr -run '^Test(Document|Resolve|Store|Repair)'
go test -count=1 ./src/internal/minimal -run '^Test(Session|Hook|Rules)'
go test -tags=integration -count=1 ./src/cmd/aidlc -run '^TestMinimalJourney'
```

修正末尾の上記4 command はすべて exit 0（okfmemory 0.725s、kdr 0.528s、minimal 2.100s、
cmd/aidlc 11.070s）。gofmt 適用と `git diff --check` 成功。HEAD は開始時のまま。
全体 test/race/vet/cross-build/live は実行していない。

## final で判明した既存期待値の更新: m1-final-expectations

開始 HEAD は `3fb1cbc`、Issue #128 の新 help/Space 契約に直接対応する test expectation だけを更新した。
`go test -count=1 ./src/internal/cli -run '^TestRun_(Help|HelpWriteError|UnknownArguments)$'` は旧 help 全文との差で
exit 1 を再現し、明示 literal の期待値へ新公開 command を追加後 exit 0。
`go test -count=1 ./src/cmd/aidlc -run '^TestMainSpaceCreateClosedPipes$'` は旧 scaffold 期待で exit 1 を再現し、
新 OKF の6 directory・6 file の本文を検査する期待へ更新後 exit 0。
閉 pipe の exit 1、出力境界、完成 Space 保持、再試行の EEXIST と全 tree 不変の検査は維持した。
production/hook/live harness は変更していない。gofmt 適用、diff check 成功。

main live の read-only 原因確認では、第二 session は同じ KDR の更新後に一般 Bash の git status/diff を実行し、
再び未記録となった。その後の再 update がなく Stop(false) は block、Stop(true) は warning で終わった。
したがって `sessions=1` は第二 session の clean 終了欠落を正しく拒否した結果であり、live 成功とは扱わない。

## 編集失敗からの継続: m1-edit-failure-recovery

開始 HEAD `0f82410f7ebf4b474a40815cdc8c2857de08cf28`、Issue #128。
[直接承認された補足計画](2026-09-08-edit-failure-remains-in-progress.md)に従い、単独writerが loop で修正。

1. `TestHookEditFailureRecovery` は同じ session/Space/Intent の明示 recover が残留 slot に拒否される
   runnable RED（exit 1）を観測し、固定単独CLIだけの例外追加後 GREEN（exit 0）。
   通常 bind、別対象、混在 shell、一般操作の拒否を維持。復旧後の未記録、変更された Rule の再読込、
   再試行・同一 KDR 更新まで確認した。Rule 再読込の追加確認は既存Serviceで ALREADY_GREEN。
2. `TestHookRecoveryDiagnostic` と `TestInstallRecoveryGuidanceAndContextLimit` は正確な文法・状態表示・
   対応event限定 context limit の欠落で RED→GREEN。配置skillは原稿3086 bytesで4 KiB以内。
   AIが失敗終了を確認し同じ対象へ recover、Bashはpoll、最後の一般操作後に保存、追加操作後は再保存と案内する。
   Stop一回block/再入warnは維持。未記録のまま保存済み・完了とは主張しない。
3. `TestMinimalJourneyEditRecoveryEvidence` は失敗応答、Post不在、正しいsession復旧、未記録維持、
   再試行・file変更、KDR保存、clean Stop の不足を runnable RED→GREEN で拒否する。
   `TestMinimalJourneyAcceptsExplicitEditRecovery` はmain判定器に残る失敗編集のpendingを
   正しい明示復旧後に完了扱いできない RED→GREEN。復旧をPost受信と取り違えない。
   boundary live の既存recovery checkpoint内に意図したpatch検証失敗と再試行を追加した。
   外側call IDで結び付いた固定版の raw failure response と、製品 hook ID のPost不在・復旧を別々に検査する。
   raw transcript の複製はtest evidenceのみで、製品の状態や判断に依存させない。

loopではliveを起動していない。親の起動入口は引き続き次の2件。

```sh
AIDLC_MINIMAL_JOURNEY_LIVE=1 go test -tags=integration -v -count=1 -timeout=20m ./src/cmd/aidlc -run '^TestMinimalJourneyLive$'
AIDLC_MINIMAL_JOURNEY_LIVE=1 go test -tags=integration -v -count=1 -timeout=35m ./src/cmd/aidlc -run '^TestMinimalJourneyBoundariesLive$'
```

新API/永続状態/依存module/権限は追加していない。終了を推測する自動解除も追加していない。

末尾targetedは `go test -count=1 ./src/internal/minimal -run '^Test(Session|Hook|Rules)'`、
`go test -count=1 ./src/internal/install -run '^TestInstall'`、
`go test -tags=integration -count=1 ./src/cmd/aidlc -run '^TestMinimalJourney'` が全て exit 0
（1.558s、0.393s、9.970s）。gofmt・diff check 成功。全final・live・GitHub操作・commitは未実施。

## final2 と memory 文法案内: m1-memory-cli-guidance

開始 HEAD `de97182`、Issue #128、既存承認範囲の配置案内修正。
親のfinal2では全nonlive検証が成功し、境界liveは全6 phaseを543秒で完了してPASSした。
編集失敗→recover→retry、async更新競合、同会話の回答・別会話の再開を実機確認できた。
境界evidence: `/private/var/folders/9w/921pjkys39q28sk4xsc0hs000000gn/T/aidlc-minimal-boundaries-765011879`。

mainは外側timeout（308秒、exit 1）で未合格。
`/private/var/folders/9w/921pjkys39q28sk4xsc0hs000000gn/T/aidlc-minimal-journey-3707071674` のrawを
read-only確認したところ、memory文法探索で4回の入力エラーがあり、create/show/updateと最後のKDR保存に時間を使った。
最新raw末尾にはDirty=falseのStopとturn.completedも残るが、外側timeoutの失敗を成功へ読み替えない。

`TestInstallMemoryCommandGuidance` は完全な文法と拡張子なしConcept ID説明の欠落によるrunnable RED
（exit 1）を確認した。配置skillへ create/update/show/search の文法、`knowledge/addition-test` の例、
actor/draft、showのcontent/hashからupdateのexpectへ渡す方法を追加後GREEN（exit 0）。
skill原稿3691 bytes、実行ファイルpathを展開した実配布本文も4096 bytes以内で検査する。
製品policy、live判定器、timeoutは変更していない。live/全final/GitHub/commitはこのloopでは未実施。
