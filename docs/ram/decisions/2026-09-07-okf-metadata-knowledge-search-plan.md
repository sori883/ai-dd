# Space固有knowledgeをOKF metadataで段階的に検索する

- 日付: 2026-09-07（Asia/Tokyo）
- 状態: Accepted（ユーザーによる直接承認）
- 実装基点: `819279c1945c0e831057881db190d19eacb1abb9`（2026-09-07時点の最新`origin/main`）
- work unit: `okf-metadata-knowledge-search-v1`
- GitHub Issue: [#124 Space固有knowledgeをOKF metadataで段階的に検索する](https://github.com/sori883/ai-dd/issues/124)
- 比較対象: リポジトリ固定AI-DLC `2.6.123`
- OKF根拠: Open Knowledge Format v0.2、固定commit `ad30107c31c06aec8a7d5636e0d1058118604e6f`
- 実装許可: ユーザーは2026-09-07、本文の実装契約、固定本家との差分、
  `go.yaml.in/yaml/v3 v3.0.5`の追加を一つの計画として明示承認した。

## 背景と目的

現在のGo版AI-DLCは、工程と担当AIに応じてframeworkのpersona／knowledgeとactive Spaceの
`aidlc-shared/`・agent別knowledgeを列挙し、`run-stage`の`inline_context_paths`へ入れる。
Codex receiverはその一覧を安全な`read-context`から全件読む。この経路は固定AI-DLC 2.6.123の
path単位のknowledge供給を移植したもので、Space固有knowledgeが増えると、今回の作業に不要な本文も
自動的にcontextへ入る。

本計画では、active Spaceが明示的にOKF Bundleを置いた場合だけ、プログラムがYAML frontmatterの
標準metadataを小さく検索し、AIが返された安全なpathから必要なMarkdownだけを通常のfile readerで
段階的に読む。プログラムはConcept本文を検索結果や`run-stage`へ自動注入しない。検索はCLI processごとに
freshなfilesystemを走査して破棄し、永続indexやcacheを作らない。

利用者は、既存Spaceを変更せず利用し続けられる一方、`knowledge/okf/`を作ったSpaceでは、目的に合う
少数のConcept候補をmetadataだけで選び、必要な本文だけを読むことができる。

## 置換する過去の決定

この記録は過去の記録を削除せず、次の未決事項または旧挙動を置換する。

- [OKF v0.2参照基盤と初期統合境界](2026-09-03-okf-reference-boundaries.md)で未決だった
  OKF化するknowledge領域、Bundle root、context上限、完全YAML moduleを確定する。
- [AI-DLCでの検索とcontext制御](../../okf-analysis/04-aidlc-retrieval-guidance.md)に残した検索field、
  不正Concept、順位、本文選択方法の候補を、本計画の初版契約で置換する。
- [工程・担当AIに応じて配置知識ファイルを選択する計画](2026-09-04-knowledge-roster-plan.md)のうち、
  Space固有`aidlc-shared/`・agent別knowledgeを全件rosterへ入れる挙動を、`knowledge/okf/`が存在する
  active Spaceに限って置換する。framework persona／knowledgeとlegacy fallbackは維持する。
- [Codexのcontext読込をGoの安全境界へ移す](2026-09-05-codex-safe-context-read-contract.md)の
  `run-stage`が宣言したcontextを`read-context`で順番に読む契約は維持する。OKF検索後のConcept本文は、
  検索CLIが検証済みBundle相対pathを返した後、AIが通常のfile readerで選択的に読む別経路とする。

## Bundleのopt-inと互換性

- 初版のBundle rootはactive Spaceの
  `aidlc/spaces/<space>/knowledge/okf/`だけとする。
- `okf/`が存在しない場合だけ、既存どおりSpaceの`aidlc-shared/`・agent別knowledgeを
  `run-stage`へ渡す。既存projectは移行なしで互換動作を続ける。
- `okf/`が存在する場合は、legacy Space knowledgeを自動投入しない。Bundleが空、全Conceptが不正、
  または検索結果が0件でもlegacyへ黙って戻らない。利用者のopt-inを曖昧に反転させないためである。
- framework persona／knowledge、protocol、Stage file、`consumes`、必須ruleは現在の経路を維持する。
- `okf/`というleafがsymlink、通常directory以外、または安全に開けない場合は、存在しないものとして
  fallbackせずruntime errorにする。ancestor symlinkによるproject外escapeは`os.Root`で拒否する。
- `index.md`は不要で、生成、要求、検索をしない。`log.md`もConcept検索対象外とし、いずれも階層を問わず除外する。
- Markdown linkは本文で自由に使えるが、scannerは解析、追跡、存在確認、rankingに利用しない。
  broken linkはConceptまたはBundleの拒否理由にしない。
- 永続検索index、cache、migration、既存knowledgeの自動変換は作らない。framework knowledge、DocumentKB、
  Stage／protocol原稿も初版ではOKFへ移さない。

## OKF scannerの契約

`src/internal/okf`へ、特定のAI-DLC pathを知らないpureなscanner／search APIを追加する。callerが
root化した`fs.FS`、表示prefix、検索条件、1検索内で固定する評価時刻を渡し、APIはcwd、環境変数、
filesystem mtime、永続状態を読まない。将来、別のknowledge領域も同じAPIへ明示rootを渡せる形にする。

### Conceptとfrontmatter

- Bundle内の予約名以外のexact `.md`通常fileをConcept候補とする。Concept IDはBundle rootからの
  slash形式相対pathから末尾`.md`を除いて決定的に導出する。
- ConceptはUTF-8で、byte 0から始まる`---`単独行、YAML frontmatter、閉じる`---`単独行、
  その後の自由なMarkdown本文を持つ。CRLFとLFを受理し、本文は返さない。
- frontmatterは1 Conceptあたり64 KiB以下とする。閉じdelimiterを含めて上限を越えたConceptはwarningで
  除外し、本文を無制限にmemoryへ保持しない。本文には製品上のbyte数・行数上限を設けず、固定bufferで
  最後までUTF-8妥当性だけを検査する。
- `go.yaml.in/yaml/v3 v3.0.5`で完全YAMLを解析する。Go標準libraryには完全なYAML parserがないため、
  frontmatterのmapping、sequence、timestamp、quoted scalar、anchor等を独自に再実装しない。
  `Decoder.KnownFields(true)`は使わず、未知fieldを拒否しない。
- YAML rootはmapping必須、`type`は空でないstring必須とする。未知`type`、未知field、任意field欠落は許容する。
- 標準field `title`、`description`、`resource`はstring、`tags`はstring列、`sources`はstandard mapping列、
  `generated`は`by`必須のmapping、`verified`はmapping 1件またはmapping列、`status`は
  `draft|stable|deprecated`、`stale_after`と各標準timestampは明示UTC offset付きISO 8601として解析する。
  `generated.at`、`verified[].at`、`sources[].last_modified`も同じtimestamp型を検証する。
  標準fieldが存在して型または標準形が不正なら、そのConceptだけをwarningで除外する。
- `sources`、`verified`、未知fieldは検索結果へraw出力しない。`verified`から`unverified`、
  `machine-confirmed`、`human-reviewed`のtrust tierだけを導出する。
- 不正UTF-8、frontmatter delimiter、YAML、root、`type`、標準field型、symlink、directory symlink、
  device／FIFO／socket等のspecial fileは、Bundle内display pathと安定した理由を持つwarningとして除外する。
  1件の不正で他のConcept検索を止めない。
- Bundle rootの列挙または安全なopen自体に失敗した場合は、部分結果を成功として返さずerrorにする。

### 上限とwarning

- 1回のscanでConcept候補は最大4,096件。4,097件目を確認した時点で、部分検索結果を返さず
  明示errorにする。途中までの順位だけを成功として見せないfail-closed境界である。
- warningは`path`と`reason`を持つ。既存knowledge rosterと同じJSON互換6,144-byte予算を使い、
  先頭から安定順に保持し、残りは省略件数を示す最後のsummary warningへまとめる。
- directory entryと最終結果の同点順は既存規約と同じECMAScript／UTF-16 code-unit順とする。
- 入力sliceと返却sliceを共有せず、結果の`tags`とwarningを含めてcaller所有のdefensive copyを返す。

## 検索条件、順位、出力

### filter

公開commandは次とする。

```text
aidlc knowledge search [--tag <tag>]... [--type <type>]... [--query <text>] [--limit <1..100>] [--project-dir <path>]
```

- `--tag`はrepeatableで、指定値をcase-sensitiveな完全一致ANDとして扱う。同じ値の重複は意味を変えない。
- `--type`はrepeatableで、指定値をcase-sensitiveな完全一致ORとして扱う。同じ値の重複は意味を変えない。
- `--query`はtype、title、description、tagsだけを検索する。Unicode letter／numberの連続をtokenとし、
  GoのUnicode tableで小文字化する。句読点とwhitespaceはseparatorで、substring一致や本文検索はしない。
  Unicode正規化moduleは追加せず、異なるcode point列を同一視しない。
- query候補は、正規化したdistinct query tokenのうち1個以上がmetadata tokenへ完全一致したConceptとする。
  一致度は一致したdistinct query token数で、field重みや出現回数は加えない。queryが空token列へ
  正規化される場合はusage errorにする。
- tag、type、queryという異なるfilter群を同時指定した場合はANDとする。
- tag、type、queryがすべて無い場合はusage errorとする。`--limit`の既定は4、範囲は1..100である。

### lifecycleと順位

- `status`省略は`stable`。`deprecated`は検索結果から除外する。
- `stale_after`は1回の`Search`へ注入した同じ`now`で評価し、`now >= stale_after`をstaleとする。
- 順位は、query一致度の降順、次に`stable fresh`、`stable stale`、`draft fresh`、`draft stale`、
  次に`generated.at`降順、日時なしを後、最後にConcept IDのUTF-16 code-unit昇順とする。
- filesystem mtime、`verified.at`、`sources.last_modified`は本文更新時刻または順位へ使用しない。
- query未指定時は全候補のquery一致度を0とし、lifecycle以降で順位を決める。

### canonical JSON

成功時はstdoutへ改行で終わる一行のcanonical JSON、stderrへ何も出さない。struct field順を固定し、
HTML用の追加escapeやmap順へ依存しない。top-levelは次の順で常に非nil配列を持つ。

```json
{"results":[],"warnings":[]}
```

各resultは次の順で全fieldを返す。

1. `concept_id`
2. `path`（`aidlc/spaces/<space>/knowledge/okf/`から始まるslash形式の安全なproject相対display path）
3. `type`
4. `title`
5. `description`
6. `resource`
7. `tags`（常にarray）
8. `status`
9. `generated_at`（欠落時`null`、存在時は入力のinstantをUTCのRFC 3339文字列へcanonical化）
10. `stale`
11. `trust_tier`

warningは`path`、`reason`の順とする。本文、raw `sources`、raw `verified`、未知fieldは返さない。
syntax／flag errorはstdout空、stderrの`aidlc:`診断、exit 2、Bundle／filesystem／scan errorはstdout空、
stderrの`aidlc:`診断、exit 1とし、short writeもexit 1にする。

## run-stage cutoverとreceiver

- `ComposeRunStage`はactive Spaceの`knowledge/okf`を毎回`Lstat`してopt-inを判断する。
  不在時は既存のSpace knowledge sourceを`BuildRoster`へ渡す。存在時は安全なdirectory rootを開けることを
  確認したうえでSpace knowledge sourceを渡さず、framework persona／knowledgeだけでrosterを構成する。
- OKF modeは既存`narration`へ必要最小限のknowledge利用指示を加える。AIは必要に応じて
  `aidlc knowledge search`へ自分で選んだ`--tag`、`--type`、`--query`を渡し、結果の`path`だけを
  通常のfile readerで段階的に読む。Stageやagent identityから検索条件を自動生成しない。
- receiver skillにも同じ境界を記載する。検索は繰り返せる。既定4件は1回のresult countであり、
  Stage全体またはagent lifetimeのhard capではない。製品CLIは選択後の本文byte／行数、Stage全体の
  読込文書数を制限しない。これはaccess controlではない。
- `read-context`はframework persona／knowledge、protocol、Stage file、`consumes`の既存安全読込を維持する。
  OKF本文を自動でそのstreamへ混ぜない。

## 所有権と実装計画

単独の新規`go_tdd_implementer`（`gpt-6-astra` / `low`）が、1 Issue／PR、1 work unitとして
次を所有する。親は本記録と索引、Issue、gate、差分確認、review、final、PR、mergeを管理する。

- 新規`src/internal/okf/{frontmatter,scan,search,warning}.go`と対応する`*_test.go`、
  `scan_integration_test.go`
- `src/internal/knowledge/roster.go`と対象test（Space OKF cutoverを表す内部結果だけ）
- `src/internal/delivery/{run_stage,wire,presentation}.go`と対象unit／integration test
- 新規`src/internal/cli/knowledge.go`、`src/internal/cli/cli.go`、対応test
- 新規`src/cmd/aidlc/knowledge.go`、`src/cmd/aidlc/main.go`、対応unit／integration test
- `src/harness/codex/skills/aidlc/SKILL.md`とskill contract test
- `.github/workflows/ci.yml`の既存content／fresh journey対象への追加
- `go.mod`、`go.sum`
- `docs/architecture.md`、`docs/development.md`、`docs/e2e-testing.md`

既存の無関係な変更を戻さず、同じworktreeへ別writerを入れない。新APIをrunnableなREDへ到達させる
ため、各sliceのtestが参照する型、constant、function signature、zero resultだけのcompile-only scaffoldを許可する。

## TDD work unit

`work_unit_id=okf-metadata-knowledge-search-v1`として次を順に実装する。loopではexact targeted testと
影響package test、変更Go fileの`gofmt`、`git diff --check`だけを実行し、全package、race、vet、
cross compile、配布E2Eは実行しない。

1. `frontmatter`
   - 完全YAML、frontmatter／body分離、unknown field、任意field欠落、bare／list `verified`、標準field、
     invalid UTF-8／YAML／root／type／field型、64 KiB境界を固定する。
   - test: `src/internal/okf/frontmatter_test.go`
   - implementation: `src/internal/okf/frontmatter.go`
   - command: `go test -count=1 -run '^TestParseConcept' ./src/internal/okf`
2. `bundle-scan`
   - Concept ID、reserved file、UTF-16 DFS、不正Concept隔離、symlink／special file、root error、
     4,096件成功と4,097件fail-closed、bounded warning、defensive ownershipを固定する。
   - test: `src/internal/okf/scan_test.go`、`scan_integration_test.go`
   - implementation: `src/internal/okf/scan.go`、`warning.go`
   - command: `go test -count=1 -run '^TestScanBundle' ./src/internal/okf`
3. `search`
   - tag AND、type OR、異種filter AND、Unicode／日本語query token、部分一致禁止、query一致度、
     lifecycle、deprecated除外、注入now、generated.at、UTF-16同点、limit、deep ownershipを固定する。
   - test: `src/internal/okf/search_test.go`
   - implementation: `src/internal/okf/search.go`
   - command: `go test -count=1 -run '^TestSearch' ./src/internal/okf`
4. `space-cutover`
   - `okf/`不在だけlegacyへfallbackし、存在時はlegacy Space knowledgeを自動投入せず、空／全不正でも
     fallbackしない。framework、protocol、Stage、consumeを維持し、fresh detectionとnarrationを確認する。
   - test: `src/internal/knowledge/roster_test.go`、`src/internal/delivery/run_stage_test.go`、
     `run_stage_integration_test.go`
   - implementation: `src/internal/knowledge/roster.go`、`src/internal/delivery/run_stage.go`、
     `wire.go`、`presentation.go`
   - command: `go test -count=1 -run 'Test.*(OKF|KnowledgeCutover)' ./src/internal/knowledge ./src/internal/delivery`
5. `public-cli`
   - repeatable flag、required filter、limit、`--project-dir`、active Space、canonical JSON、warning、
     stdout／stderr、exit 0／1／2、root cleanup、Windows display pathを固定する。
   - test: `src/internal/cli/knowledge_test.go`、`src/cmd/aidlc/knowledge_test.go`
   - implementation: `src/internal/cli/knowledge.go`、`cli.go`、`src/cmd/aidlc/knowledge.go`、`main.go`
   - command: `go test -count=1 -run '^Test.*KnowledgeSearch' ./src/internal/cli ./src/cmd/aidlc`
6. `receiver-journey`
   - receiverがOKF modeで明示filter検索を必要に応じて繰り返し、返されたpathだけを通常file readerで読み、
     自動条件生成・本文自動注入・hard lifetime capを行わないことを構造testとfresh non-live journeyで固定する。
   - test: `src/harness/codex/skills/aidlc/skill_test.go`、
     `src/cmd/aidlc/knowledge_search_integration_test.go`
   - implementation: `src/harness/codex/skills/aidlc/SKILL.md`、CLI／delivery adapter、文書、CI
   - command: `go test -count=1 ./src/harness/codex/skills/aidlc && go test -tags=integration -count=1 -run '^TestKnowledgeSearchFreshNonLiveJourney$' ./src/cmd/aidlc`

work unit末尾では上記6 command、`go test -count=1 ./src/internal/okf ./src/internal/knowledge ./src/internal/delivery ./src/internal/cli ./src/cmd/aidlc ./src/harness/codex/skills/aidlc`、
変更Go fileの`gofmt`、`git diff --check`を実行する。

## reviewとfinal

親はwork unit返却後に全差分とtargeted commandを一度確認する。固定base/headに対し、
`independent_reviewer`を`verification_mode=review`で起動する。blocking findingは同じ単独writerへ
一つのrepair work unitとして戻し、観測可能な修正は回帰testのREDを先行してから再reviewする。

差分が安定した後、親が対象fileを変更しないread-only `final`を一度実行する。

```text
go test -count=1 -shuffle=on ./...
go test -race -count=1 -shuffle=on ./...
env -u AIDLC_CODEX_EXEC_LIVE go test -tags=integration -race -count=1 -shuffle=on ./...
go vet ./...
go vet -tags=integration ./...
gofmt -l src
go mod tidy -diff
go mod verify
gopls check <変更Go file...>
govulncheck ./... または利用可能な同等のreachability check
git diff --check 819279c1945c0e831057881db190d19eacb1abb9..HEAD
darwin/linux/windows × amd64/arm64の`src/cmd/aidlc` cross compile
darwin/linux/windows × amd64/arm64の該当test binary cross compile
fresh sandboxの`TestKnowledgeSearchFreshNonLiveJourney`
```

外部live Codexは本計画のfinalで実行しない。final後に対象fileが変われば証拠をstaleとし、targeted loop、
再review、fresh finalへ戻る。PRでは現在headで起動するGitHub checksの開始と成功を確認し、保護を迂回せず、
既存のmerge commit方式で自律mergeする。merge後に`origin/main`反映とIssue closeを確認する。

## 本家AI-DLCとの意図的な差分

- 本家の挙動: 固定AI-DLC 2.6.123の確認済み`aidlc-orchestrate.ts`とCodex receiverは、active Spaceの
  `aidlc-shared/`・agent別knowledgeをpath単位で選び、`run-stage` contextとして全件読む。
- 採用する挙動: `knowledge/okf/`が存在するSpaceだけ、標準OKF metadataを検索して候補pathを返し、AIが
  必要なMarkdownを通常のfile readerで段階的に読む。legacy Space knowledgeの自動投入は行わない。
- 変更理由: Space固有knowledgeを増やしても不要な本文を自動注入せず、検索条件と選択をAIが観測可能な
  小さいmetadata結果へ分離するためである。
- 利用者・互換性への影響: `okf/`のない既存Spaceは従来動作を保つ。`okf/`を作ると明示opt-inとなり、
  空または不正Bundleでもlegacyへfallbackしない。検索結果4件は1回の既定値で、繰返し検索できる。
  検索はaccess controlではなく、安全なproject相対pathを返すdiscovery機能である。
- 確認範囲: リポジトリ固定AI-DLC 2.6.123と固定OKF v0.2 commitだけであり、最新upstreamとの一致は未確認。

## 依存関係、リスク、rollback

外部module `go.yaml.in/yaml/v3 v3.0.5`は、未知fieldを許容しつつ完全YAMLのmapping／sequence／scalar型を
安全に解析するために必要で、Go標準libraryに代替がない。ユーザーがversionを含めて明示承認済みである。
追加の外部module、tool、credential、permissionは導入しない。

主なリスクは、4,096件／64 KiB上限によるConcept除外、query token境界による検索漏れ、opt-in directoryを
誤って作った場合のlegacy cutover、通常file readerが検索後の本文を無制限に読み得ることである。明示error、
warning、exact filter、fresh scan、空Bundle非fallback、receiver指示で境界を観測可能にする。本文byte／行数と
Stage全体の文書数を製品CLIで制限しないことは承認済みの利用条件であり、access controlとは扱わない。

永続schemaや既存knowledgeを変更しないため、問題時は本Issueのscanner、CLI、cutover、receiver変更を
同じPR単位のrevertで戻し、全Spaceを従来rosterへ戻せる。利用者が置いた`knowledge/okf/`の内容は削除しない。
