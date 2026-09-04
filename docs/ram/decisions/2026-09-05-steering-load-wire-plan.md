# 必須ルールbundleのdigestとload-steering JSONを組み立てる

- 日付: 2026-09-05
- 状態: Accepted（知識供給マイルストーンの包括承認内）
- Issue: [#99](https://github.com/sori883/ai-dd/issues/99)
- 実装許可: [全範囲の自律実装承認](2026-09-05-context-delivery-autonomous-authorization.md)
- 前提: [必須ルール本文の配信用chunk](2026-09-05-steering-chunks-plan.md)

## 背景・目的・利用者が得る結果

配置Markdownから読み直した必須ルール本文は、順序を保った20 KiB目標のchunkへ分割できる。
次に必要なのは、全本文が同じbundleであることを識別するdigestと、1個のchunkをAIへ渡す
`load-steering` JSONである。これにより、後続の継続token処理が、途中でルールが変わった場合に
古い続きを拒否するための安定した材料を得られる。

このPRは純粋なJSON組立までを完成させる。tokenの署名・保存、配置fileの再読込み、stateやrouteとの
freshness比較、CLI、Codex受領処理は変更しない。digestやJSONの部品だけを追加した時点で、配信全体の
完成とはしない。

## 根拠と採用する設計

基準は固定AI-DLC 2.6.123の次の確認済み範囲であり、最新upstreamとの一致は主張しない。

- `core/tools/aidlc-orchestrate.ts:3325-3411`: ruleのJSON wire表現とchunk。
- `core/tools/aidlc-orchestrate.ts:3666-3727`: ordered bundle digest、`load-steering`組立、28 KiB検査。
- `core/tools/aidlc-directive.ts:84-99`: `LoadSteeringDirective`のfieldと型。

既存`src/internal/steering`へ次を追加する。

```go
type LoadDirective struct {
    Stage         string
    Bundle        string
    Part          int
    Parts         int
    RulesContent  []RuleContent
    ContinueToken string
}

func BundleDigest(content []RuleContent) (string, error)
func MarshalLoad(input LoadDirective) ([]byte, error)
```

### BundleDigest

- 分割前の全`RuleContent`を入力順のまま`[{"path":...,"text":...}]`へencodeする。
- 重複path、空本文、nil/空sliceを勝手に除外しない。nilと空sliceはどちらも`[]`になる。
- JSON bytesに改行を加えずSHA-256を計算し、`sha256:`と小文字hexを連結して返す。
- pathと本文のquote、backslash、U+0000〜U+001FをJSON.stringify相当にescapeする。
  `<`、`>`、`&`、U+2028、U+2029、日本語、絵文字はUTF-8のまま保持し、Go標準JSONの
  HTML向けescapeへ置き換えない。文字列中のliteral `\\u003c`を`<`へ解釈し直さない。
- 不正UTF-8をreplacement characterへ変換せずerrorにする。入力を変更・保持しない。

### MarshalLoad

- field順を`kind`、`stage`、`bundle`、`part`、`parts`、`rules_content`、`continue_token`に固定し、
  `kind`は常に`"load-steering"`とする。余分な改行や任意fieldを追加しない。
- `rules_content`は受け取ったchunkの順序・重複・空本文を保持し、nilもJSONでは`[]`にする。
- `part >= 1`、`parts >= 1`、`part <= parts`を要求する。stage、bundle、tokenの空文字や
  digest書式について、上流schemaにない新しい制限は加えない。
- 全文字列の不正UTF-8をerrorにし、部分的なJSONは返さない。
- 完成JSONのUTF-8 byte数が28,672以下なら返し、28,673以上はnil結果とerrorにする。
  ぴったりは受け入れ、本文の切り直し・切捨て・後続への詰め直しは行わない。
- 毎回新しいbyte sliceを返す。入力sliceと別の呼出し結果を共有せず、入力を変更しない。

`ContinueToken`は後続が安全に生成したopaque文字列を受け取るだけで、このPRでは固定key、署名、
decode、保存、再利用防止を実装しない。digestも認証ではなく、内容同一性を比較する材料である。

新package、外部Go module、公開CLI、I/O、永続dataは追加しない。JSON.stringify互換に必要な短い
private encoderを`steering`内へ置き、既存`ChunkRules`の分割契約やreaderを変更しない。

## 所有権とTDD順序

Go writerは1人。`src/internal/steering/wire.go`、`wire_test.go`、`load.go`、`load_test.go`のみを
所有する。親はこの計画、索引、architecture、development、Issue・PRを担当し、Go writerと同時に
fileを編集しない。既存`chunks.go`、reader、knowledge、state、audit、CLI、`src/core/`は変更しない。
ユーザーの未追跡HTMLと`work/`には触れない。

1. `digest-order`: 順序、重複、空本文、nil/空sliceから決定論的digestを返す。
2. `digest-escaping`: 特殊文字、日本語、絵文字をJSON.stringify相当bytesとしてhashする。
3. `digest-invalid-utf8`: pathまたは本文の不正UTF-8を拒否する。
4. `load-wire`: 必須fieldを固定順で出し、rule順とnil `rules_content`の`[]`を保持する。
5. `load-validation`: part/parts不整合と全fieldの不正UTF-8をnil結果で拒否する。
6. `load-cap`: 完成JSON 28,672 bytesぴったりと+1を区別する。
7. `chunk-composition`: `ChunkRules`の各chunkを、同じ全体digest・part番号で個別JSON化できる。
8. `wire-ownership`: 入力や一方の返却bytesの変更が別の結果へ影響しない。

各sliceは1 testのRED依頼だけから始め、担当の最終応答後に親がtestと範囲を確認し、同じcommandを
再実行する。親のHEADと全対象file hash/ABSENTを含む受入後だけ別GREENを依頼する。既存実装で
満たされた補足testは`ALREADY_GREEN`と記録し、人工的な失敗を作らない。

最初の`digest-order` REDだけは`BundleDigest`のsignatureと必ず空結果を返すcompile-only scaffold、
最初の`load-wire` REDだけは`LoadDirective`と`MarshalLoad`のcompile-only scaffoldを許可する。
scaffoldでtestがcompileされ、期待値との不一致として失敗した場合だけ有効なREDとする。

## 実装記録

`digest-order`、`digest-escaping`、`digest-invalid-utf8`、`load-wire`、`load-validation`、`load-cap`は、
各REDを親が同じcommandで再現し、対象testとproduction/immutable fileのhashを固定してから別handoffの
GREENへ進んだ。各GREENも親が同じcommandで再実行し、先行behaviorのtargeted回帰も成功した。

`chunk-composition`と`wire-ownership`、公開`MarshalLoad`から特殊文字のexact JSONを確認する補足testは、
先行sliceで完成したproductionに対して最初から成功したため`ALREADY_GREEN`として扱った。人工的な失敗や
不要なproduction変更は作っていない。各sliceのcommand、failure理由、hashは[#99のTDD記録](https://github.com/sori883/ai-dd/issues/99)
へ残す。

## 検証・依存・リスク・戻し方

loopはGo 1.26.8で当該test名1件だけを実行する。独立review後、固定headへのread-only finalに
全package unit、race/shuffle、integration/race、通常・integration vet、`go mod tidy -diff`、
`gofmt -l src`、変更Go fileのgopls診断、baseからheadの`git diff --check`を集約する。

CLIとintegration tag付きsteering test binaryをdarwin/linux/windows × amd64/arm64の6構成へ
cross compileする。これは各OSでのnative実行証拠とはしない。対象headで起動したGitHub Quality 2構成と
Build 6構成を、push・PRの両runとも成功確認してからmergeし、main反映とIssue closeを確認する。

主なリスクはJSON escape、field順、array/commaの1 byte、28 KiB境界である。testではproductionの
encoderや容量helperを期待値生成に使わず、literal wireまたは独立した長さ計算で固定する。
外部moduleやtoolは追加しない。I/Oと永続形式の移行はなく、このAPIと関連文書だけをrevertできる。
新しい意図的な本家差分は採用しない。

## 後続への境界

次に、machine-local keyで署名した継続token、delivery-only cursor、毎回のrule/state/route再構築、
古い続きと再利用の拒否を扱う。その後、installed `.codex/`・`aidlc/spaces/`のsource、knowledge roster、
工程手順、入力資料、Codexの実読込へ接続する。既存Nextのread-only、製品の人間承認、未対応工程の
fail-closedは維持する。
