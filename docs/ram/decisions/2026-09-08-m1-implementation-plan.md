# M1: 単一バイナリでOKF・KDR・hookの作業を一周させる実装計画

- 日付: 2026-09-08
- 状態: Accepted / 実装着手
- 対応Issue: [#128](https://github.com/sori883/ai-dd/issues/128)
- 実装許可: 本タスクで最小契約を具体化し、ユーザーが名前によるIntent操作、標準OKF metadata、intent_id検索まで確認した後の「はい、では進めてください」という直接承認。

## 目的と現在地

現在のGo CLIにはSpace管理、旧工程の処理、保存・CIの基盤がある。一方、新方式のKDRとOKF Agent Memory、記録漏れを防ぐhookは未実装である。
開始時のmainとHEADは `fc8bd7ea633f0c1ad8cc605233572a6106b3b911`。PR #127までmerge済みで、Open Issue・PRは各0件。

M1では一つのaidlcバイナリから新しい利用先を用意し、Space作成、名前によるIntent作成・選択、必須ルール読込、試作・テスト・修正、独立レビュー、結果記録、別の会話からの再開まで一周できるようにする。
Intentは一つの目的を持つ作業、KDRはその目的・判断・結果・検証・残件を記録する一つのMarkdownである。

## 許可範囲と準拠先

この計画はM1全体への直接承認を根拠にする。旧33 Stageロードマップの包括承認は使わない。
GoシングルバイナリにOKF処理と初期資産を内包し、利用先では展開済みの文書を読む。外部Go moduleを追加しない。
既存依存 `go.yaml.in/yaml/v3 v3.0.5` は利用可能。新しい実行環境やokf別バイナリを要求しない。

配布先とSpaceの公開作成操作はリポジトリ固定AI-DLC 2.6.123を基準にする。
OKF形式は固定v0.2仕様 `ad30107c31c06aec8a7d5636e0d1058118604e6f`、Agent Memoryの版の基準はv0.1.2 / `d4c523ed5ce916fa207fe314851b98721421c891`。
ユーザー指定の `docs/実装_okf-agent-memory/` は元commit不明のGo実装参考として選択的に照合する。最新upstreamとは同一視しない。
参照コードを取り込む場合はMITの著作権・license表示を保持する。参考ディレクトリ全体を製品へコピーしない。

旧Intent・stateは読込・変換・移行せず、既存ファイルも削除しない。M2/M3の一般更新・rollbackや旧経路の縮小は許可に含めない。
コード・設定・Issue・PRの変更、独立レビュー、検証、checks成功後の通常mergeとIssue closeは本計画の実行範囲である。

## 利用者の操作と保存契約

Spaceの `aidlc/spaces/<space>/knowledge/` がOKF bundle（知識文書のまとまり）である。
`knowledge/`、`design/`、`kdr/`、`rules/` に全Knowledge・設計・KDR・Ruleを配置し、index.mdとlog.mdで案内と知識変更履歴を保つ。
KDRの保存先は `kdr/<intent_id>.md`。IDは32桁の小文字16進数でCLIがcrypto/randから生成する。
frontmatterにintent_idとtitleを持ち、IDはファイル名と一致する。タイトル変更・反復・再開ではIDとpathを変えない。
type・description・tags・generated・status等の標準OKF metadataを使う。generated.byは明示actor、atは意味ある内容変更時のUTC日時。モデル名を推測しない。
sources・verified・stale_after・resourceと未知metadataを読込・更新で保持し、未確認のverifiedを作らない。statusは工程stateや合格判定ではない。

| 公開操作 | 利用者が得る結果 |
| --- | --- |
| `aidlc install codex --project-dir <root>` | fresh利用先へ内包資産を展開。既存対象fileは上書き・mergeしない |
| `aidlc space create <name> [--project-dir <path>]` | 本家準拠の名前正規化・予約名・重複検査で新OKF Spaceを作成。作成で選択は変えない |
| `aidlc intent create <name> --space <space> --file <draft> --actor <actor>` | titleとの一致を検査し、新IDとKDRを作成。選択は別操作 |
| `aidlc intent list --space <space>` | KDRからIDと名前を列挙 |
| `aidlc intent switch <name> --space <space> --session <session>` | 名前を完全一致解決し、内部で会話と結び付けてKDR・必須Ruleを読む |
| `aidlc intent switch --id <id> --space <space> --session <session>` | 同名候補を選んだ場合のAI用解決経路 |
| `aidlc kdr template/create/list/show/update/check/repair` | テンプレート読込、同じKDRの更新、検査、修復。下位操作の文法はM0設計の契約を使う |
| `aidlc session bind/inspect` | 内部の対応・復旧操作。一般利用者へIDやbindの手入力を要求しない |
| `aidlc memory search [query] [--intent-id <id>] --space <space>` | 指定SpaceのOKF検索。IDはfrontmatter完全一致、検索語との併記はAND、0件は正常 |
| `aidlc memory show <concept-id>` / `rules` / `check` | 本文表示・必須Rule全文・OKF検査 |
| `aidlc memory create/update <concept-id> --file <draft> --actor <actor>` | 合意された共有知識の追加・更新。KDR配下は専用操作へ案内。更新は期待hashを要求 |

新方式にはSpaceを明示する。共通のproject-dir省略時はGit worktree rootを解決するが、既存Space操作のroot解決契約は維持する。
新方式のIntent操作は--space付き経路として接続し、旧cursorを参照・更新しない。旧公開経路の削除はM3で扱う。
名前が複数に一致した場合は候補とIDを返し、勝手に選ばない。ID検索で本文・タグ内の文字列を代用せず、検索だけで会話の選択や未記録表示を変えない。
検索結果はConcept ID・intent_id・title・description・相対pathを返す。空や形式不正の明示IDは入力エラー。

## 導入とSpace生成

内包資産はsrc/core/minimalとsrc/harness/codex/minimalそれぞれにembed入口を持たせ、installerが本家対応の.codex・.agents・aidlc等への配置mappingを組み立てる。
go:embedの親directory参照は禁止されるため、製品資産を別の重複原稿へコピーしない。
fresh導入でdefault SpaceのOKF初期資産を作る。新Spaceはdefaultのknowledge/rules/rule.mdを作成時だけコピーする。
コピー元不存在時だけ内包の最小OKF Ruleを使う。存在するが不正・読取不能なら失敗し、編集済みRuleを置換しない。
途中失敗は保存済みpathと失敗箇所を返し、部分treeを自動削除しない。利用時には内包の古い本文へfallbackしない。

## 保存、記録漏れ、復旧

KDRはOKF frontmatterとM0設計の6本文見出しを持つ。目的は具体文、他節は未実施等の理由付き記録を許す。
空欄・案内文の残存を拒否し、通常updateでは判断と結果・検証とレビュー・残件と再開のいずれかの実質変更を要求する。
metadata・日時だけの更新や同一本文で未記録を解消しない。文章の真偽・十分性はAIと独立レビューが確認する。

KDRは256 KiB、UTF-8、regular file、path封じ込め、ID一致を検査する。
session lock→Space bundle lockの順で排他し、期待SHA-256をlock内で比較する。各fileは同じdirectoryの一時fileから置換する。
保存順は本文→親index→log→sessionの記録済み表示。index/log共有のためKDR単位lockだけでは済ませない。
部分失敗時はID・path・現在hash・失敗箇所を返す。create失敗後に別IDを自動生成しない。
repairは欠落・破損本文に加えindex/log不整合も対象にする。正常本文を保持し、実際に行った修復を新しいlog項目へ書く。
失われた原操作の履歴を推測して復元せず、repairだけで記録済みにしない。復旧経緯は通常updateで記録する。
未知metadataの意図しない脱落は拒否して再提出を求める。全file transactionや独自receiptは追加しない。

会話ごとの一時textにSpace・ID・turn・未記録の真偽・実行中tool最大1件・Rule読込印とhashだけを上書き保存する。
配置はworktree固有 `aidlc/.runtime/{sessions,drafts,locks}/` とし、runtime内の.gitignoreで管理対象外にする。CLIとAIのdraft編集が通常sandbox権限で使えるようGit管理領域から具体化した。正本の配置は変えない。根拠: [一時保存先の具体化](2026-09-08-minimal-runtime-workspace-path.md)。工程state、全操作audit、過去tool履歴、独自JSON snapshot、receiptは保存しない。
UserPromptSubmitで未記録にし、SessionStart/compactでRule読込印を無効化する。SessionStartにturn_idはない。
SessionStartは配置済み最小skillの開始手順（4 KiB以内）をadditionalContextへ渡し、初回の一般cat拒否との循環を防ぐ。必須Rule本文の代用にはせず、読込済み印も付けない。欠落/超過時は診断し内包fallbackしない。根拠: [初期案内](2026-09-08-minimal-bootstrap-context.md)。
PreToolUseはKDR・必須Ruleと会話の対応を確認し、一般操作を未記録として実行中slotを持つ。PostToolUseの対応終端だけがslotを外す。
固定版liveでBashのPost本文に終了コードがないことを確認したため、成功/失敗分岐は不要とし対応Post自体を終端とする。拒否したPreにはPostがないため、slotを取得するのは検査を通った許可経路だけとする。根拠は[実機確認](../research/2026-09-08-minimal-hook-live-preflight.md)。
失敗したテストも未記録のまま。一般操作を直列にし、長時間toolはpollが完了するまで更新・別操作を拒否する。
Stopは補完を一回要求し、再入でも未解消なら警告して止まる。初回bindは未記録、同じIDへの再bindでも保持する。
未記録の別Intent切替を拒否し、残留slotの明示recoverも未記録で再開する。

記録・読取りの例外は固定aidlc実行ファイルの既知の単独CLIと会話専用draftのpatchだけ。シェル結合・置換等は例外にしない。
これは対応する通常AI経路での記録漏れ防止であり、OS権限による全書込み禁止ではない。故障・無効なhookが必ず拒否する保証は置かず、導入診断と故障時の作業停止を定める。
必須Ruleはrules/entry.mdの明示リンク順に全文を読み、合計16 KiBを超えたら切り捨てずエラーにする。
reviewerは別checkout・別root会話のread-only sandboxで同じKDR・Rule・コードを読み、writerが結果をKDRへ反映する。

## 単独writerの対象とTDD

実装担当はgo_tdd_implementer一人。親とread-only planner/reviewerはproduction・testを同時編集しない。
所有範囲は新規src/internal/{install,okfmemory,kdr,minimal}、src/core/minimal、src/harness/codex/minimal、src/internal/workspaceのSpace作成と関連test、src/internal/cli、src/cmd/aidlc、.github/workflows/ci.yml、docs/development.md・architecture.md、実装証拠RAM。
既存の未commit設計・RAMとユーザーの参照ソースは保全する。変更が必要な旧testは新Space契約に直接関係する期待値だけ更新する。

最初の技術gateとして、固定codex-cli 0.153.4/macOS arm64の実イベントをfresh sandboxで確認する。
ここだけは製品全体を実装する前に真偽を確かめる必要があるため、work unitをpreflightと本体に分ける。同じwriterを継続使用する。
preflight担当はGo製test harnessを用意し、親が既存auth・既定モデル・workspace-writeでliveを実行する。
検査済み専用test hookに限り一回限りのhook trust bypass引数を使える。通常の利用設定・保存済みtrust・sandboxを弱めない。
testイベントの観測logは試験fixtureであり製品の全操作auditにしない。証拠は実際のhook入力と副作用の有無で確認し、AIの自己申告だけに依存しない。

| 順序 | work unit / 受入条件 | targeted command |
| --- | --- | --- |
| 0 | m1-hook-preflight: 開始・turn入力、実行前拒否、成功/失敗/非同期終端とtool ID対応、Stop一回再入 | `go test -count=1 ./src/cmd/aidlc -run '^TestMinimalHookProbe'`、親liveは同名のLive testを明示envで起動 |
| 1 | m1-minimal-intent-journey: 内包資産、fresh配置、衝突時保全 | `go test -count=1 ./src/internal/install -run '^TestInstall'` |
| 2 | 新Space・Ruleコピーと独立性・失敗 | `go test -count=1 ./src/internal/workspace -run '^TestCreateSpace'` |
| 3 | 標準metadataと未知項目保持、検索、検査、ID一致、index/log | `go test -count=1 ./src/internal/okfmemory -run '^Test(Parse|Search|Validate|Bookkeeping)'` |
| 4 | KDR、名前曖昧性、改名、競合、部分保存と修復 | `go test -count=1 ./src/internal/kdr -run '^Test(Document|Resolve|Store|Repair)'` |
| 5 | 公開CLI、actor、不正入力、出力 | `go test -count=1 ./src/internal/cli -run '^Test(Install|Intent|KDR|Memory|Minimal)'` |
| 6 | session、Rule更新、hook、Stop、非同期競合、読取無変更 | `go test -count=1 ./src/internal/minimal -run '^Test(Session|Hook|Rules)'` |
| 7 | 配置から名前操作、反復、独立レビュー、知識一件追加、別会話再開 | `go test -tags=integration -count=1 ./src/cmd/aidlc -run '^TestMinimalJourney'` |

本体は1 Issue/PRの全項目を一つのwork unitで順番にtest-first実行する。
新APIにはrunnable assertion到達に必要な型・signature・空返値だけのscaffoldを許可する。compile failureをREDに数えない。
初回から通るtestはALREADY_GREENとして実測記録し、人工的に壊さない。loopはtargeted/affected package確認に限定する。
実装担当はloopのgofmtと差分checkを完了して一回返し、親が全差分とtargeted test群を一回確認して独立reviewへ進む。

## 最終検証と完了

独立reviewのblocking finding解消後、親がread-only finalを一回開始する。
`go test -count=1 ./...`、`go test -race -shuffle=on -count=1 ./...`、`go vet ./...`、`gofmt -l src`、`git diff --check`、`go mod tidy -diff`を行う。
既存CIのintegrationと新規filesystem/journey、CGO_ENABLED=0のdarwin/linux/windows×amd64/arm64 buildを行う。
Linux非live一周はGitHub CIで、macOS arm64/Codex固定版のfresh live一周は既存authで検証し、実際のモデルを結果へ記録する。
liveではKDRなし拒否、記録後の終了、後続変更の再要求、質問待ち、中断・別会話再開、Rule欠落、CLI故障、長時間toolと更新競合を確認する。
修正があればfinal証拠を更新する。対象PRのchecksが実際に成功してから通常mergeし、main反映とIssue closeを確認する。

## 意図的差分、リスク、停止条件

本家2.6.123はStage graph・registry・cursor・承認/完了marker・auditを使う。新方式はOKF KDR正本と会話単位の対応で反復する。
変更理由は製品構成と記録運用の簡素化。利用者の名前操作は保つが保存形式・再開方法は旧方式と互換にしない。
Space生成内容とRule名もユーザー指定の新OKF配置へ変更する。最新upstream全体との一致は主張しない。
hook非対応や終端判別不能、新しい外部module・権限、承認範囲外の重大差分が判明した場合だけ停止して確認する。
戻す場合は専用sandboxの使用を止め、必要なら通常revertする。利用者KDR・bundleを自動削除しない。

根拠: [詳細なM0契約](../../design/minimal-product-contract-m0.md)、[3点のA回答](2026-09-08-single-binary-space-rule-accepted.md)、
[名前とfrontmatter](2026-09-08-intent-name-frontmatter-accepted.md)、[標準metadata](2026-09-08-kdr-okf-metadata-clarification.md)、
[intent_id検索](2026-09-08-okf-intent-id-search.md)、[本家配布](../research/2026-08-29-existing-distribution-format.md)、
[本家Space作成](../research/2026-08-31-space-creation-contracts.md)。
