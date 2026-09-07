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
