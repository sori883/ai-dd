# 工程・担当AIに応じて配置知識ファイルを選択する計画

- 日付: 2026-09-04
- 状態: Accepted（配置Markdownによる知識供給マイルストーン内）
- 基点: `6f33c82067298ba9732f4c3f828c2b79a9b15a46`（PR #92）
- Issue: [#93](https://github.com/sori883/ai-dd/issues/93)
- 単独writer: go_tdd_implementer。親はIssue、固定差分review、final、PR、mergeを管理する。

## 背景・目的・利用者が得る結果

Go版AI-DLCには、必須ルールを毎回ファイルから読み込む内部処理がある。一方、担当AIの役割文書
（persona）と参考知識を、工程や担当に応じて選ぶ部品がない。この変更では「読む文書の順序付き一覧」
（roster）と警告を返す内部APIを作る。利用先のMarkdownを実際に読めるか毎回確認し、任意の知識が
欠けても、読める他の文書を一覧へ残す。必須ルールの欠落時にエラーとする既存処理とは責務を分ける。

Minimalは工程で読むframework知識を必要な範囲へ絞る設定、plugin所有情報は各知識ファイルを
どの拡張機能が配置したかを示すmetadataである。利用プロジェクトの知識と開発者のRAMは混在させない。

## 実装許可

[配置Markdownによる知識供給マイルストーン](2026-09-04-file-based-knowledge-delivery.md)で
必須ルール本文、工程・担当AI別知識、Codexでの実読込の3段階が承認済み。
[利用先と固定順の直接承認](2026-09-04-installed-context-source-and-order.md)により、
利用先 `.codex/`・`aidlc/spaces/` から読むこと、知識のUTF-16固定順と本家との順序・上限への
影響も承認された。この内部一覧の構築は同マイルストーン第2段階に含まれ、追加承認は不要である。

本文の埋込み、cache、`src/core/` の直接参照やfallback、OKF変換はしない。既存Nextのread-only、
人間承認、未対応工程のfail-closed、Go標準ライブラリのみという境界を維持する。

## 確認済みの本家と現在コード

比較対象はリポジトリ固定AI-DLC 2.6.123であり、最新upstreamではない。
原典は `core/tools/aidlc-orchestrate.ts:2740-3137`、
`aidlc-lib.ts:273-305,464-475`。回帰根拠は
`tests/unit/t248-steering-content-delivery.test.ts:861-1084` と
`t314-minimal-scope-performance.test.ts`。
Goの `graph.Stage` にはSlug/Mode/LeadAgent/SupportAgentsが既にあり、変更しない。
Depthはcallerがstateまたはscope.Metadataから解決する。graph.Scopeはrouting表なので渡さない。

## 内部APIと所有権

```go
type Source struct {
    FS fs.FS
    DisplayPrefix string
}
type RosterInput struct {
    Stage graph.Stage
    Depth string
    Framework Source
    FrameworkDir string
    SpaceKnowledge *Source
    EnabledPlugins []string
}
type Roster struct {
    Paths []string
    Warnings []string
}
func BuildRoster(input RosterInput) (Roster, error)
```

- Framework.FSは配置 `.codex/`、SpaceKnowledge.FSはactive Spaceのknowledge directoryをrootとする。
  FSはcaller所有の `os.Root.FS()` 等を借用する。APIはrootを探索・開閉せず、cwd・envを読まない。
- DisplayPrefixは `.codex` と `aidlc/spaces/<space>/knowledge` 等の表示用slash形式path。
  実際のFS相対pathと分ける。FrameworkDirはplugin警告の絶対path表示専用で、I/Oに使わない。
- 使用するFSのnil/typed-nil、表示prefix、agent path成分、FrameworkDirはI/O前に検証する。
  prefixはUTF-8、NULなし、fs.ValidPath、非 "."、native localityを満たす。agentは同じ条件の単一成分。
  POSIXのliteral backslashを一律禁止せず、Windowsのnative localityとの差を保持する。
  FrameworkDirはUTF-8/NULなしのnative絶対directoryとし、明示rootのdot segmentを勝手に拒否しない。
- 有効agentがゼロならFSも配置用入力も使用せず、non-nil空Paths/Warningsを返す。
  SpaceKnowledge=nilはSpace知識の参照なしを表す。入力や返却sliceを共有して変更しない。
- EnabledPluginsは解決済みの名前一覧を注入する。nilは全有効、non-nil空は全無効。
  これは本家の未指定null／指定Setに対応し、完全一致のmembershipとして扱う。
  設定ファイル読込や名前の再正規化はこのAPIの責務にしない。
- 入力error時は空のRosterとerror。通常の任意文書の欠落・読取失敗・不正UTF-8は警告して省略する。

## 文書選択の契約

1. Mode=inlineはlead→supports、mobはleadのみ、それ以外はなし。agentは先勝ちで重複除去し、
   orchestratorを除外する。Execution（ALWAYS/CONDITIONAL）は選択条件にしない。
2. persona全員分→framework共通→framework担当AI別→Space共通→Space担当AI別の順に並べる。
   各directoryをUTF-16 code-unit順に整列し、深さ優先で列挙する。最後に表示pathを先勝ちで重複除去する。
3. 拡張子がexact ".md"の通常fileとsymlinkが対象。入れ子directory symlinkは再帰しない。
   毎回全文のUTF-8を確認する。欠落directoryは無警告、欠落personaは警告する。
   その他の読込失敗・不正UTF-8は本家形式の警告を返す。空文書・見出しだけの知識も保持し、
   Memoryのsubstantive filterを呼ばない。
4. preflight（読めるか確認）→Minimal/plugin絞込み→重複除去→容量制限の順。
   後で省略する候補も先に確認し、警告を失わない。
5. Minimalは本家のintent-capture、requirements-analysisにある固定tableだけを移植する。
   Depthの判定はECMAScriptのtrimと小文字化に相当する判定を行う。既知の同梱知識のみ絞り、
   persona、Space知識、未知ファイル、再帰directory内の同名ファイルは維持する。
   Standard等、tableのないstage/ownerは全保持する。
6. tools/data/plugin-files-*.jsonをUTF-16順で毎回読む。schema_version=数値1、plugin=string、
   knowledge=arrayを確認する。knowledge値はnonempty string、先頭"/"なし、"/"分割した
   ".."成分なしという本家検査を保ち、native join/toPosixで照合用pathへ変換する。
   metadata由来のpathではファイルを開かない。
7. plugin所有情報は複数ownerの和集合。途中の不正項目より前に追加済みの情報は保持して警告する。
   Minimal対象内ではplugin所有判定を既知文書tableより優先し、ownerが1つでも有効なら残す。
   JSON objectはexact key/last-winsで扱い、Go structのcase-insensitive field一致を混ぜない。
8. path一覧はJSON.stringify互換のUTF-8 bytesで8,192以下の先頭部分。最初の超過で打ち切り、
   後続の短いpathを詰め直さない。全候補の確認を終えてから省略件数を警告する。
9. 警告一覧は同じ計算法で6,144 bytes以下。本家どおり後続省略件数の要約を予約して先頭から詰める。
   GoのHTML-safe escapingやU+2028/U+2029のescapeを、そのまま本家のJSONサイズとして数えない。

## 設計と採用しない代替案

- FS注入と独立packageを選び、公開supplierやharness.json探索と分離する。
- 本文preflightは本家と同様に `fs.ReadFile` とUTF-8検査で行う。streaming独自処理は今回追加せず、
  1文書ごとに読み、本文をrosterへ保持しない。任意の新しい文書サイズ制限を追加しない。
- UTF-16比較は既存graph/workspaceと同じ標準libraryの比較式をpackage内で使用する。
  shared helperへの横断refactorやlocale library導入はしない。
- Minimal前にcapを適用しない。pruning後に本来残る文書やpreflight警告が失われるため。
- 小さなJSON文字列サイズ計算を独立してtestし、既存Go encoderのescape方針を変更しない。

## 所有ファイル

単独writerが次を担当する。親の合意RAMと索引には無断で手を加えない。

- 新規 `src/internal/knowledge/{roster,read,minimal,plugins,budget}.go`
- 対応する `*_test.go` と `read_integration_test.go`
- `.github/workflows/ci.yml` の既存filesystem integration stepへ新package追加のみ
- `docs/architecture.md`、`docs/development.md` の内部API説明
- 本計画への実施記録追記

親が本計画、合意RAM、`docs/ram/README.md` の索引を管理する。
`src/core/`、既存graph/scope/steering API、CLI/state/audit、無関係な
`docs/implementation-overview.html` は変更しない。

## 受け入れ条件とTDD順序

最小のコンパイル可能なAPI骨組みから始め、各項目で目的の期待値不一致によるREDを観測してから、
最小GREENとrefactorを行う。compile errorだけ、完成コードを外したnegative controlだけをRED証拠にしない。

1. lead persona取得→mode別agent選択・順序・重複・zero-agent時I/Oなし。
2. 5群の順序、再帰列挙、exact拡張子、ASCII本家順とUTF-16固定順。
3. 欠落・read error・不正UTF-8・空文書・fresh read・全件preflight。
4. plugin情報の正常／部分不正／複数owner、nilと明示空の選択差。
5. 本家Minimalの正確な2 roster、既知文書除外、再帰basename衝突・独自文書・Space知識の保持。
6. path/warning上限一致・1 byte超過、最初の超過で停止、件数要約、引用符・制御文字・
   <>&・U+2028/U+2029・日本語・補助平面文字。
7. 入力検証と使用FS検査がI/O前であること、入力/結果の所有権。
8. os.Root integration: 内向きfile symlink許容、外向きは警告して省略、directory symlink非再帰、
   同じRootで編集・追加・削除を次回に反映、borrowed Rootを閉じない。

## 検証

loopはsliceごとの最小test名のみを使う。
`GOTOOLCHAIN=go1.26.8 go test -count=1 -run '<当該test名>' ./src/internal/knowledge`
実FSだけは `-tags=integration` を付ける。gofmt適用はloop内かつreview前に終える。

固定base/headの独立review後、親が対象を変更しないfinalを1回だけ開始する。

```sh
GOTOOLCHAIN=go1.26.8 go test -count=1 -shuffle=on ./...
GOTOOLCHAIN=go1.26.8 go test -race -count=1 -shuffle=on ./...
GOTOOLCHAIN=go1.26.8 go test -tags=integration -race -count=1 -shuffle=on ./...
GOTOOLCHAIN=go1.26.8 go vet ./...
GOTOOLCHAIN=go1.26.8 go vet -tags=integration ./...
GOTOOLCHAIN=go1.26.8 go mod tidy -diff
gofmt -l src
git diff --check 6f33c82..HEAD
```

変更Go全fileのgopls check（integration fileはtag指定）、darwin/linux/windows × amd64/arm64の
CLIとknowledge integration test binaryのcross compile、原稿140件＋LICENSEのhashを確認する。
CIの既存Quality 2種とBuild 6種がpush/PRの両方で成功するまでmergeしない。
公開経路未変更のため公開CLI配布E2Eは対象外で、実FS接続をintegrationで確認する。
final後のtarget変更は証拠をstaleとし、loop→review→finalへ戻す。

## 意図的な本家差分・リスク・戻し方

本家2.6.123のlocaleCompareによる環境依存順を、承認済みUTF-16 code-unit順へ置換する。
理由は再現性と外部module回避。日本語、大文字小文字等では順序と8 KiB内に残る文書が変わり得る。
本文、path名、必須ルール順は変更しない。この差分以外に新しい意図的変更を採用しない。

preflight後の編集や複数文書の原子的snapshot、後続Codex読込の成功は保証しない。
任意FSや特殊fileの実行時間を制限するAPIではない。本文を一時的に1文書分保持する。
永続dataを変更しないため、問題があれば通常の修正PR/revert PRで戻せる。

本計画の範囲に新しい重大な未決事項はない。公開CLI/transportの永続key等は各後続計画で
確認し、今回の一覧構築をもって供給全体の完成とはしない。

## 実施記録（Issue #93）

- 2026-09-04、単独writerが`src/internal/knowledge`を追加した。`BuildRoster`は借用したframework
  `.codex` rootとactive Space knowledge rootだけを読み、inline/mobのagent選択、5群のUTF-16 DFS、
  UTF-8 preflight、Minimal/plugin選択、first-wins、Path 8 KiB・warning 6 KiB制限を実装した。
  本文は返却値やcacheへ保持せず、`FrameworkDir`はplugin warning表示だけに使用する。
- 最初のrunnable REDは骨組みがinline lead personaのPathを返さず失敗したものだった。
  `GOTOOLCHAIN=go1.26.8 go test -count=1 -run '^TestBuildRosterReturnsInlineLeadPersona$' ./src/internal/knowledge`
  で失敗し、最小実装後に同じコマンドが成功した。validation追加時には、invalid入力でnon-nil空sliceを
  返す実装がzero `Roster`契約に反するREDも観測し、error時zero結果へ修正した。
- mode/zero-agent、5群順序・UTF-16 DFS、preflight/fresh read、plugin所有権、Minimal、cap、ownershipの
  targeted testは、対応実装を含む時点で最初からGREENだったものもあり、完成コード除去やcompile errorを
  RED証拠として記録していない。実FSは`go test -tags=integration -count=1 -run '^TestBuildRosterIntegration' ./src/internal/knowledge`
  で内部・外向きsymlink、directory symlink非再帰、同一Rootの編集反映、借用Root生存を確認した。
- 残余リスクは、複数Markdown同時編集のatomic snapshot、任意FSの実行時間制限、後続Codex側の本文読込成功、
  公開CLI/transport接続をこの内部rosterが保証しないことである。これらは計画記載の境界内であり、今回の
  実装では変更していない。
