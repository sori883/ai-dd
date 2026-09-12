# AI-DLC用OKFスキルの追加計画

## 背景と実装許可

OKFは、知識の本文と種類・説明・作成者などの情報をMarkdownへ保存する形式である。
現在の製品には `aidlc memory` の検索・読取り・作成・更新・検査があるが、AI向けの操作案内は
他の操作と一緒に `aidlc-cli` に置かれている。ユーザーは本家の専用skillと現在の実装の違いを
確認したうえで「このプロジェクトに適用したokfのskillsを作成してほしい」と直接依頼した。
この依頼を、既存のCLIと承認・保存契約に合わせた製品用skill、その配布・参照接続の実装許可とする。
旧ロードマップの包括承認は流用しない。

利用者は `aidlc-okf` を読むことで、必要な知識の探し方、保存対象の判断、保存先、CLIによる
安全な更新を確認できる。新規配置時は `.agents/skills/aidlc-okf/SKILL.md` へ自動配置される。
`aidlc` は工程・承認、`aidlc-cli` は操作選択・stateと文書宣言、`aidlc-okf` は知識の検索・保存を担当する。

## 確認した根拠と境界

- 開始main: `951a5e2d642b5167c830ef31e82af54d848ca689`。PR #158でskill分離が導入済み。
  Open Issue・PRは0件。元checkoutの未commit変更は保持し、実装は `ai-dd-naming` で行う。
- OKF Agent Memoryの参照: `a09e04918aa84d275b784374b5236d9eeac56c9e` の
  `.agents/skills/okf-memory/SKILL.md`、`discovery.md`、`remember.md`。
  [本家skill](https://github.com/okf-memory/okf-agent-memory/blob/a09e04918aa84d275b784374b5236d9eeac56c9e/.agents/skills/okf-memory/SKILL.md)。
  ユーザー指定のローカル資料 `docs/実装_okf-agent-memory/` も確認したが、その元commitは未確認。
- 本家は既定 `knowledge/`、専用 `okf` CLI/MCPと詳細な検索・関連付け・検査を案内する。
  本製品は承認済みのSpace配置とGo単一binaryを使い、既存 `aidlc memory` へ合わせて独自に文章を作る。
  本家のコマンドや検索機能を全移植したとは説明しない。外部tool・moduleは追加しない。
- AI-DLC本家との新しい工程・承認・データ仕様差分を追加する作業ではない。
  既存の必須Rule、単独の共有writer、stage Sensor、独立review、人間承認を維持する。
  固定本家AI-DLCの全体再調査や最新upstreamとの一致確認は行わない。
- 新skillの読取りは既存の選択・Rule読込み・実行中toolの保護を通る。起動前の無条件免除や
  任意ファイルの読取り許可は追加しない。既存の承認待ち中のskill読取り対象へ1ファイル追加する。
- 新規配布と同一版内の移転へ接続する。旧配置へ上書きinstall、旧記録の移行、個人用skill導入は行わない。
  重要な未解決選択はない。結果を変える新しい選択が見つかった場合は範囲を広げず確認する。

## スキルの内容

1. Spaceの `aidlc/spaces/<space>/knowledge/` が検索範囲。まず検索し、説明から必要なConceptだけshowする。
   現行検索はtitle/description/tagsの語句照合であり本文全文・BM25検索ではない。
   `--intent-id` は完全一致。共有文書を探す時はIntent条件を外す。既知IDはshowできる。
2. 同じ話題の現行Knowledgeは更新を優先する。新しい設計判断のwhy・代替案・影響は現在Intentの新規ADRに残し、
   過去の判断を上書きして消さない。将来の利用に必要な事実・確定要件を残し、未確認の推測と事実を区別する。
3. Concept IDは拡張子なし。`codekb/`、`design/<intent_id>/`、`adr/`、`rules/rule`、
   CLI管理の `log/<intent_id>-work-log` を説明する。共有current-analysisとarchitectureはSpace共有。
   ADRのtypeは小文字 `adr`。工程が要求するtype等はprocedureの宣言と一致させる。
4. 本文草稿だけをsessionのdraftへ書き、メインAIがmemory create/updateで保存する。サブ担当は本文案を返す。
   generated日時はCLI自動、actor・title・description・type・tags等は引数。正確な値・型はhelpを参照する。
   sourcesとverifiedは実際の根拠を使い、人間確認を捏造しない。Ruleは利用プロジェクト共通ルール。
5. 更新前にshowの本文とhashを読み、--expectで競合を検知する。同じ内容を日時だけ更新しない。
   保存後にshow/search/checkで必要な結果を確認する。checkの成功は工程や承認の合格ではない。
   部分保存では成功と断定せず、返されたpath/hashと実ファイルを確認し、盲目的にcreate/updateを再試行しない。
6. 作業ログ・index/logはCLI管理。通常の進捗はstate、差戻し理由はintent操作で記録する。
   終了時には変更した機能のKnowledgeと必要なADRが現状と一致するかを確認する。

## 所有範囲と実装順序

親が本計画・RAM・Issue/PRを管理する。実装開始後の対象ツリーwriterは1名とし、全項目を
`work_unit_id=aidlc-okf-skill`、`verification_mode=loop` で同じGo実装担当へ渡す。

- 新規 `src/harness/codex/aidlc-okf/SKILL.md`。
- `src/harness/codex/SKILL.md`、`aidlc-cli/SKILL.md`、`assets.go`: 記録規約の移管、相互参照とembed。
- `src/internal/install/install.go`、`relocate.go` と対応test: 新skillの配置、3skill+hooksの4ファイル移転。
- `src/internal/app/hook.go` とskill読取り関連test: 狭い許可path追加。
- `src/internal/flow/procedure.go` と関連test: initializationで新skillの存在も確認する。
- `src/internal/install` の既存文書参照test、`src/cmd/aidlc/relocation_integration_test.go` と
  `memory_live_integration_test.go`:
  新構成の期待値・実読込み証拠へ追従。
- `src/docs/user-guide.md`、`docs/distribution.md`、必要な現行参照 `docs/architecture.md`・`docs/development.md`・CLI help:
  skillの責任分担、3skillの移転を説明。履歴RAMの旧配置は改変しない。

順序付きTDDは次の通り。単なる文章の一致には人工REDを作らず、実際の配布・拒否条件を検査する。

1. **配置**: 新skillが存在しbinary参照とリンクが解決する、既存fileを上書きしない。
   `go test -count=1 ./src/internal/install -run '^TestOKFSkillInstall'`
2. **移転**: 全3skillとhooksの既知参照だけ変更。未知編集・欠落を拒否。部分失敗・同一要求の再試行を確認。
   `go test -count=1 ./src/internal/install -run '^Test(OKFSkillRelocate|Relocate)'`
3. **読込み**: 開始前・承認待ちで許可されたskillを読め、別path・連結コマンド・Rule未読・in-flightを拒否。
   `go test -count=1 ./src/internal/app -run '^TestOKFSkillRead'`
   initializationが新skill欠落を拒否する: `go test -count=1 ./src/internal/flow -run '^TestOKFSkillInitialization'`。
4. 既存参照・文書・実機fixtureを追従。実機証拠の判定を変更する場合は記録なし/拒否/成功の
   人工イベントを使うtargeted testで先に検証する。
   `go test -tags=integration -count=1 ./src/cmd/aidlc -run '^TestMemoryMetadataCommandEvidence$'`
   skillのcat成功と出力が配置した本文と一致する実読込み証拠を検証し、欠落・拒否・異なる本文を成功扱いしない。

末尾はinstall/appの関連test群、変更Goへのgofmt、`git diff --check` を一度まとめて確認する。
CLIの機能・保存形式・hook matcherは変更しない。新規skillを読めるpath追加は必要な接続に限る。

## 独立reviewとfinal

別agentが `verification_mode=review` で新skillと移管元・コードを照合する。共有文書更新、別IntentのADR、
未検証の人間確認、保存競合などの例を使って指示が既存契約を守るか確認する。
blocking finding修正後、親がread-only `final` を開始する。

- `go test ./...`、`go test -race ./...`、`go vet ./...`、`go mod tidy -diff`、`gofmt -l src`、`git diff --check`。
- `go test -tags=integration -count=1 ./...`（実機専用のskipは成功と数えず、下記で明示実行）。
- 新規配布した3skillを既存 `skill-creator/scripts/quick_validate.py` で検証する。
- 固定済みCodex 0.153.4で既存 `TestMemoryMetadataLive` を実行し、新skillの実読込み、help、
  CLI作成・更新とmetadata保持を確認する。隔離した試験用projectだけを使い、実利用環境のtrust等は変更しない。
  この試験はKnowledge操作の確認であり、全工程完走やhook全経路の保証とは説明しない。
- Goクロスbuildや配布archiveの全対象確認は対象PRで起動する既存CIも確認する。
  全checks成功・review・final成立後、既存運用に従いPRをmergeし、main反映とIssue closeを確認する。

## 更新・復旧の扱い

新skillを含む同一版の配置一式で利用する。既存配置の更新は従来のstaging比較手順を使い、
3skillと対応binaryを組で扱う。`--relocate` は版更新ではなく同一資材の絶対path補正である。
移転失敗時はPaths/Pendingを確認して同じ引数を使い、未知編集は保全する。
問題があれば作業を止め、保管した同じ版のbinary・製品資材を組で復元する。Space文書とstateは変更しない。
