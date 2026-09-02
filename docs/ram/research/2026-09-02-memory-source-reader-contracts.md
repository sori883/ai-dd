# 4層Memory source readerの参照契約

- 調査日: 2026-09-02
- 比較対象: ローカルAI-DLC v2.6.123（`docs/実装_aidlc-workflows`）
- 対象Issue: [#47](https://github.com/sori883/ai-dd/issues/47)
- 確認範囲: 実装版の`core/tools/aidlc-graph.ts`と`core/tools/aidlc-steering.ts`。最新upstream、全workflow、全配布物の完全なparityは確認していない。

## 本家の根拠

`aidlc-graph.ts:271-324`はMemoryを`aidlc/spaces/<space>/memory/`へ配置し、
`org.md`、`team.md`、`project.md`と`phases/`を同じrootから解決する。`500-529`には
4層のscope名、`phases/<phase>.md`の配置、`^[a-z][a-z0-9-]*$`のphase file regexがある。
`604-655`の`loadRules`はトップレベル3 fileと`phases/`を対象にし、その他のfileを無視する。

`aidlc-steering.ts:85-107`の`readRuleBundle`は候補を呼出しごとに読み、fatal UTF-8 decodeや
read errorがある場合は部分contentを使用せずerrorにする。一方、内容のfrontmatter解析や
substantive判定は本readerの責務ではない。stageごとの後続resolverが担当する。

## 採用APIと取得契約

```go
type Layer string

type Source struct {
    Layer Layer
    Path string
    Content string
}

func ReadSources(memoryFS fs.FS, phase string) ([]Source, error)
```

実装packageは`src/internal/memory`。`LayerOrg`、`LayerTeam`、`LayerProject`、`LayerPhase`
を公開し、`Source.Path`はMemory root相対のslash pathとする。

| 項目 | 契約 |
| --- | --- |
| 候補順 | `org.md` → `team.md` → `project.md` → `phases/<phase>.md`の固定順 |
| phase | 非空、ASCIIの`^[a-z][a-z0-9-]*$`。既知phase enumには限定しない |
| 欠損 | `errors.Is(err, fs.ErrNotExist)`ならそのlayerだけskip。全欠損はnon-nilの空slice |
| その他のread error | path contextとcauseを保持してfail-closed。部分sliceは返さない |
| 不正UTF-8 | `fs.ErrInvalid`をcauseとするerror、path context、結果nil。途中までのsourceも破棄 |
| 内容 | UTF-8検証後のbyte列をstring化するだけ。CRLF、BOM、空内容、frontmatter、末尾改行を保持 |
| unknown file | walkせず固定候補だけをdirect read |
| cache | global cacheなし。呼出しごとにfresh readし、結果sliceはcaller-owned |
| nil FS | panicせず`fs.ErrInvalid`を識別できるerror |

merge、override、frontmatter parse、空/templateのsubstantive判定、known phase validation、
filesystemのroot解決、Root open/Close、CLI、writerはこのlayerへ持ち込まない。

## filesystem境界

実filesystemを使うcallerは、既存projectの`memory/` directoryを`os.OpenRoot`で開き、
その`Root.FS()`を渡す。readerはRootをcloseしない。通常fileとroot内相対symlinkは読み、
root外・絶対symlinkは外部bytesを返さずerrorとする。`fs.FS`一般や`os.DirFS`自体には
このsandbox保証がないため、境界はcallerの供給方法で成立する。

これは本家Node実装が通常filesystem経由でroot外symlinkを追跡し得る点からの意図的差分である。

| 本家の挙動 | 採用した挙動 | 理由 | 利用者・互換性への影響 |
| --- | --- | --- | --- |
| 通常Node filesystemでMemory pathを解決し、root外symlinkを追跡し得る | `os.Root.FS()`を受け取りroot外・絶対symlinkを拒否 | project外の任意file読取を防ぐ | 外向きsymlink構成はerrorとなり外部内容を返さない。通常fileとroot内symlinkは影響なし |

この差分はIssue #47の承認済み計画に含まれ、比較対象v2.6.123の上記範囲を根拠とする。
stage固有の第5層は本家v2.6.123でも予約・未実装のため、API・実装には含めない。

## Go runtimeの注意

Go 1.26.0〜1.26.4のGO-2026-4970は、末尾slashを伴うroot外leaf symlinkに関する注意である。
本readerの固定候補pathには末尾slashがないが、CIと最終検証は修正版Go 1.26.5以降を前提とする。
この実装ではOS固有separatorをpathへ連結せず、`phases/<phase>.md`のslash表現を維持する。

## 未解決事項

- Memory各fileを並行更新したときの一貫したsnapshotは保証しない。
- `os.Root`もmount、device、特殊filesystem全般を遮断する完全sandboxではない。
- Node/Bunの全OS・全path解釈、最新upstream、配布物全体との完全互換は未確認である。
