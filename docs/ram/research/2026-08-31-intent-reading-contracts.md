# Intent候補列挙・現在intent解決の参照契約

- 日付: 2026-08-31
- 状態: Current for local v2.6.123 snapshot（静的調査。Go版の承認は下記の後続記録を参照）
- 対象: `listIntentDirs`、`activeIntent`と、直接関係するpath helper・テスト
- 関連: [内部機能を先行する実装順序](../decisions/2026-08-31-internal-workspace-before-status.md)、
  [space読み取りの初期契約](../decisions/2026-08-31-space-reading-contract.md)

## 調査の目的と範囲

space読み取りの次に、intent候補の列挙と現在intentの解決を小さく移植できるかを確認した。
`intents.json`との結合、state本文の解析、session binding、作成・切替、公開CLIは別の責務とする。

ローカル実装の製品versionは`2.6.123`で、分析索引と一致する。調査対象の共通
`aidlc-lib.ts`は、実装、canonical Codex配布、配置済みCodex配布、既存テストが参照する
Claude配布でSHAが一致した。配置物全体の設定差がないことを意味するものではない。

## 本家で確認した契約

| 対象・状況 | 動作 |
| --- | --- |
| space未指定 | `activeSpace`で決定する |
| spaceが空文字 | 明示値として採用し、`aidlc/spaces/intents`を参照する |
| spaceが空白 | trimせず、その名前をpathへ使う |
| explicit intentが未指定・空文字 | cursor、唯一の候補の順に解決する |
| explicit intentが非空 | 空白だけでもそのまま返す。trim・名前・存在の検証をしない |
| cursorが有効 | UTF-8で読みJS trimした値の結合先に`aidlc-state.md`が存在すれば返す |
| cursorが不在・空・read error・参照先不在 | 候補が1件なら選択し、0件・複数ならnull |
| 一覧のReadDir失敗 | 空一覧 |
| 個別markerの存在確認失敗 | その候補だけを除外し、後続も調べる |

`activeIntent`はexplicitの返却前にもspaceを決定する。そのためspace未指定の場合は
`active-space`を読むが、非空explicitがあれば`active-intent`と候補一覧は読まない。

候補判定は、intents直下の各entryについて`<entry>/aidlc-state.md`が存在するかだけで行う。
entryやmarkerが通常fileかdirectoryかを検証せず、本文もparseしない。markerがdirectoryでも、
存在する対象へのsymlinkでも候補になる。壊れたリンクは候補にならない。
registryがなくても候補を列挙し、最後にJavaScriptのUTF-16コード単位順で整列する。

個別の存在確認失敗で列挙を続ける点は、space readerの「子のStat失敗で打ち切る」契約と
異なる。共通化のために同じ動作へ変えてはならない。

## 名前とfilesystemの境界

本家のcursorは一覧への所属を照合しない。`../other`、`.`、`nested/name`等でも、
結合先markerが存在すればその値を返す。spaceも未検証のままpathへ結合する。
`..`の正規化やsymlinkにより、想定したintentsやspaceの外へ参照が届く可能性がある。

絶対path風の文字列や区切り文字も、この2関数は明示的に拒否しない。ただしNodeの`join`と
`resolve`は別のAPIであり、「後続が`/`で始まれば必ず基準pathを置き換える」とは解釈しない。
Windowsでは区切り文字の扱いも異なる。これらのOS別境界動作は今回実測していない。

上位のintent switchには一覧のexact directory名・一意slugへの照合があるが、
手編集されたcursorを保護する検証ではない。session bindingの`safeIntentRecordName`も、
この2関数には適用されない。

現行の作成処理は`YYMMDD-label`形式を使う。古いコメントの`<slug>-id8`を必須規則として
実装すると現行の記録を拒否するため、名前の安全性と命名形式は別に扱う。

## Go側の設計に関係する事実

Go標準APIはContext7を優先して参照し、versionがmasterに混在した箇所は手元の
Go `1.26.4`の`go doc`で確認した。

- `os.Root.FS()`は`fs.FS`を提供し、Root内に制限された読み取りに使用できる。
- Rootの操作はroot外へ参照するpathとsymlink、絶対symlinkを拒否する。
  mount境界、特殊file、device fileまで遮断する完全なsandboxではない。
- `os.DirFS`と、それに対する`fs.Sub`だけではsymlinkの越境を防げない。
- `fs.FS`というinterface型そのものには封じ込め保証がない。供給する実装の契約が必要。
- `fs.ValidPath`はslash区切りのpathを対象とし、backslashとcolonは名前の文字として許す。
  単一のdirectory名を要求するかどうかは、別途決める必要がある。

## 選択肢と未解決事項

1. project基準のFSとspace指定を受け、本家のpath結合まで今回再現する。
   小さな選択ルールの移植に加え、space名、空space、symlinkの扱いまで決める必要がある。
2. 選択済みの1 spaceのintents directoryを基準とするFSを受け、2つのreaderを先に作る。
   project rootからそのFSを用意する処理は後続へ分けられるが、まだ既存space readerや
   CLIから呼ばれる機能にはならない。

いずれも、未検証のpathを忠実に再現するか、安全性のための限定的な非互換を認めるかは
実装前の承認事項とする。名前検証だけをsandbox保証と表現しない。
本家の任意pathを安全な単一名へ制限する場合、拒否時のfallbackも明記する必要がある。

後続の[実装計画](../decisions/2026-08-31-intent-reading-plan.md)では、選択済みintents基準のFSと、
単一名に限定しない`fs.ValidPath`相当のcursor制約を提案した。調査時点では未承認だったが、
同日の詳細計画提示後にユーザーが承認した。採用した境界と理由は同計画の承認記録を参照する。

## テストの根拠と不足

既存の直接テストは、優先順位、0・1・複数候補、markerなしの除外、ASCII順序を扱う。
移植では次も固定する必要がある。

- 空・stale・read errorのcursor、空・空白のexplicit。
- markerがdirectory・symlink・broken link、存在確認失敗後の列挙継続。
- traversal、絶対path風の値、区切り文字、非BMP文字の順序、JS trimの差。
- 旧形式と現行形式の名前、registryやstate本文を読まないこと、filesystemを書き換えないこと。

今回は原典の静的調査であり、Bun・各OSの境界ケースの実行結果ではない。
不正UTF-8のdecodeの完全互換も保証を確定していない。

## 主要根拠

- `docs/実装_aidlc-workflows/core/tools/aidlc-lib.ts`:
  path helper（1569行付近）、`listIntentDirs`（1610行付近）、`activeIntent`（1633行付近）、
  `listIntents`（2448行付近）、`safeIntentRecordName`（3236行付近）、
  `resolveWorkflowSelection`（3613行付近）、`createIntent`（3905行付近）。
- `docs/実装_aidlc-workflows/core/tools/aidlc-utility.ts`: `handleIntent`（6160行付近）。
- `docs/実装_aidlc-workflows/tests/unit/t160-workspace-record-resolution.test.ts`（148行付近）。
- Go `1.26.4`: `go doc os.Root`、`go doc os.Root.FS`、`go doc os.DirFS`、
  `go doc io/fs.FS`、`go doc io/fs.ValidPath`。
