# Space作成の参照契約

- 調査日: 2026-08-31
- 状態: Current for local v2.6.123 snapshot
- 対象: ローカルAI-DLC `2.6.123`のspace作成CLIと呼び出されるhelper
- 関連: [初期実装の境界](../decisions/2026-08-29-initial-implementation-boundaries.md)

## 調査目的・範囲

読み取り中心のworkspace機能から、spaceを実際に作成できる最小のCLIへ進むために、
生成物、名前、既存data、選択、依存、失敗時の契約を確認した。
本家のCLIやhookは実行せず、原本・配布物・テストを読み取り専用で調査した。
競合、権限エラー、各OSの挙動を実行して確認した結果ではない。

`aidlc-version.ts`の製品版は分析索引と同じ`2.6.123`だった。
`aidlc.ts`、`aidlc-utility.ts`、`aidlc-lib.ts`、`aidlc-audit.ts`、`aidlc-version.ts`は、
core、canonical Codex dist、配置済みCodex配布物で各ファイルのSHA-256が一致した。
最新upstreamや配布物全体との一致を意味しない。

## CLI

公開構文は`aidlc space create <name> [--project-dir <path>]`。
旧`aidlc space-create <name>`も同じhandlerへ転送される。名前は位置引数で、`--name`ではない。
`--project-dir`はglobal flagとして途中にも置ける。作成固有のflagsはなく、
force、dry-run、作成結果のJSON出力は実装されていない。
余剰の位置引数や未使用flagを厳格に拒否するhandlerでもない。

成功時は終了コード0で、作成名、生成物の概要、`/aidlc space <name>`による切替案内をstdoutへ出す。
名前欠落はrouterからusageをstderrへ出して終了コード1。
handler内の拒否は`{"error":"..."}`をstderrへ出して終了コード1となり、
配布TSの通常実行時にはfilesystem例外も同じエラー経路へ到達する。

## 名前と重複

`slugify`は次の順で入力名を正規化する。

1. 小文字化する。
2. ASCII英数字以外の連続をhyphenに置き換える。
3. 先頭・末尾のhyphenを取り除く。
4. 48文字まで切り詰め、末尾のhyphenを再び取り除く。
5. 先頭が英字でなければ`intent-`を付加し、末尾のhyphenを取り除く。

prefix付加は切り詰め後なので、最終名が48文字を超える場合がある。
空の位置引数は拒否するが、空白・記号だけの非空入力は`intent`になる。
rawの`help`と`-h`は正規化前に拒否する。
正規化後の予約名は`help`、`list`、`switch`、`create`、`archive`、`rename`、`show`、`birth`。

`default`は禁止名ではなく、実directoryがなければ作成できる。
`existsSync(dest)`がtrueなら作成前に拒否する。既存directory、通常file、参照先が存在する
symlinkを更新・merge・修復しない。dangling linkを明示的に分類する処理はない。

## 生成物

基準は`<project>/aidlc/spaces/<正規化した名前>/`。
対象space自体を含め7directory、6fileを生成する。必要に応じて祖先directoryも作成される。

```text
<space>/
├── memory/
│   ├── org.md
│   ├── team.md
│   ├── project.md
│   ├── phases/
│   └── templates/
│       └── .gitkeep
├── intents/
├── codekb/
│   └── .gitkeep
└── knowledge/
    └── .gitkeep
```

- `org.md`: 常に`default/memory/org.md`をUTF-8文字列としてコピーする。
  `existsSync`がfalseなら`# Organization defaults\n`を生成する。
  存在確認がtrueになった後の読取・copyエラーにはfallbackしない。
- `team.md`: `# Team practices\n`。
- `project.md`: `# Project overrides\n`。
- 3つの`.gitkeep`: 空file。
- `phases/`と`intents/`: 空directoryで、`.gitkeep`は生成しない。

defaultのteam、project、phase/template本文、既存CodeKB/DocumentKB内容はコピーしない。
project自体の存在確認はなく、再帰mkdirで祖先を作れるが、`.gitignore`、harness設定、
default spaceを含むproject全体の初期化を行う機能ではない。

## 選択と他機能への依存

成功時の書込み先は論理上、新space配下と不足する祖先directoryだけである。
active-space、active-intent、registry、state、audit、session binding、harness includeは更新しない。
作成前に現在space・intentが選ばれている必要はなく、作成後に自動選択もしない。
CodeKB/knowledgeは空の置場だけを作り、解析・索引化・workflow開始は行わない。

ただし、次の間接依存が存在する。

- `knowledgeDir(projectDir, name)`は内部で`resolveWorkflowSelection(projectDir, {space})`を呼ぶ。
  space明示でもsession判定が先行し、session overrideとPID祖先sessionが不一致で、
  payload由来扱いでない場合はthrowする。この読取りでbinding自体は書き換えない。
- 最初の`knowledgeDir`呼出しより前にmemory、phases、templates、intents、codekbは作成されるため、
  上記のsession不一致でも部分treeが残る。
- `die`からの`emitError`は、現在workflowのstateがあれば`ERROR_LOGGED`監査追記をbest-effortで試みる。
  成功時に監査を書かなくても、失敗時まで完全無変更とはいえない。

## 失敗・並行性・参照範囲

観測したコードには、作成を囲むlock、staging、atomic publish、rollback、cleanupがない。
mkdirとwriteを逐次行うため、途中失敗では部分treeが残り、再実行が既存targetとして拒否され得る。

存在確認とmkdirが分離しているため、同名の並行呼出しが両方確認を通る可能性がある。
org.mdは無条件writeなので、単独成功や競合時のno-clobberは保証していない。
これはソースからの推論であり、競合実験の結果ではない。

親directory・seed元のsymlinkに対する封じ込めや、存在確認後の差替え対策はない。
Go版で参照範囲の制限や同名競合時の排他作成を採用する場合は、
意図的な設計変更として計画・承認の対象にする。

## 対応テスト

- `tests/integration/t175-space-create-memory-isolation.test.ts:48`: org継承、team/project分離、default保全。
- `tests/unit/t229-workspace-parser.test.ts:393`: 予約名の拒否。
- `tests/unit/t230-dispatcher-routes.test.ts:494`: 新旧CLIの出力・tree同等性、global flag。
- `tests/integration/t311-session-binding-writers.test.ts:216`: 作成でsession bindingを変えない。
- `tests/e2e/t-exec-codex-journey-workspace.serial.test.ts:303`: 初期fileの内容とCodeKB/knowledge配置。

今回確認したテストでは、org欠損/read失敗、default新規作成、blank名、同名競合、
途中失敗cleanup、symlink境界を直接固定するケースは見つからなかった。

## 主要根拠

以下は`docs/実装_aidlc-workflows/`からの相対pathと行番号。

- `core/tools/aidlc.ts:653,874`: space dispatcher、global flag処理。
- `core/tools/aidlc-utility.ts:6994`: handleSpaceCreate。
- `core/tools/aidlc-utility.ts:7011,7029,7055`: 作成先、seed、成功出力。
- `core/tools/aidlc-lib.ts:2173`: slugify。
- `core/tools/aidlc-lib.ts:941`: 予約名集合。
- `core/tools/aidlc-lib.ts:1581,3617`: knowledgeDirとsessionを含む選択解決。
- `core/tools/aidlc-lib.ts:22207`: emitError。

この調査は、作成だけの最小機能にregistry/state/audit/session本体を一括実装する必要がある、
という結論ではない。基本の生成契約と、今後実装する周辺機能の依存を分けるための記録である。

## 実装時の追加調査: Unicode小文字化（2026-08-31）

ECMAScriptの小文字化とGoの`strings.ToLower`は、U+0130（`İ`）の扱いが異なる。
本家の式は`i`とU+0307へ展開してからASCII以外を区切りへ変換するが、
Goの単純小文字化は`i`だけになる。このため、文字列途中では名前が変わる。

| 入力 | 本家の正規化結果 | Goの単純小文字化だけを使った結果 |
| --- | --- | --- |
| `İB` | `i-b` | `ib` |
| `AİB` | `ai-b` | `aib` |
| `İstanbul` | `i-stanbul` | `istanbul` |
| `İ` / `Aİ` | `i` / `ai` | 同じ（末尾の区切り除去で違いが隠れる） |
| `İ B` | `i-b` | 同じ |
| `K` / `AKB` | `k` / `akb` | 同じ |

Go版は元のU+0130を`i\u0307`へ展開してから小文字化・ASCII区切り変換を行う。
これは承認済みの名前互換性を満たすための実装で、意図的な仕様変更ではない。
追加moduleは使わず、上記ケースを回帰テストへ含めた。

根拠はローカル本家`core/tools/aidlc-lib.ts:2173`、Go1.26.4の
`src/strings/strings.go:763`・`src/unicode/tables.go:8635`、
[Unicode 15.0 SpecialCasing](https://www.unicode.org/Public/15.0.0/ucd/SpecialCasing.txt)。
本家の名前処理だけを取り出した純粋な式をNode24.15.0で確認した。
本家CLI・hookや、Unicode全体の網羅比較は実行していない。
