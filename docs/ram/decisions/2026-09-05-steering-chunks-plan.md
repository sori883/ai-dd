# 必須ルール本文を配信用のまとまりへ分割する

- 日付: 2026-09-05
- 状態: Accepted（知識供給マイルストーンの包括承認内）
- Issue: [#97](https://github.com/sori883/ai-dd/issues/97)
- 実装許可: [全範囲の自律実装承認](2026-09-05-context-delivery-autonomous-authorization.md)

## 背景・目的・利用者が得る結果

配置Markdownから必須ルールを順序付きで読む内部処理はあるが、長い本文を複数回に分けてAIへ
渡すための分割処理はまだない。本文を途中で切り捨てたり日本語を壊したりせず、元の順序と内容を
復元できる小さなまとまり（chunk）を作る純粋な内部部品を追加する。

この部品はファイルを読まず、readerが取得・UTF-8検証した`RuleContent`（pathと本文）を受け取る。
配信全体の完成ではなく、後続の受領継続処理が使う本文分割の完成を受け入れ条件とする。

## 根拠と採用する設計

基準は固定AI-DLC 2.6.123の`core/tools/aidlc-orchestrate.ts:2023,3321-3411`。
`markdownSections`、`splitRuleText`、`steeringPieces`、`steeringChunks`の確認済み挙動を移植する。
最新upstreamとの一致は主張しない。

- 既存`src/internal/steering`へ`func ChunkRules(content []RuleContent) [][]RuleContent`を追加する。
- 入力順と同じpath内の本文片の順序を保持し、入力を変更しない。返却sliceはcallerが所有する。
- 空入力は0 chunk。通常のpathで空本文を含む入力はこの部品で除外せず、readerの既存の本文選別を変更しない。
  ただしpathだけで20 KiB目標を超え、かつ本文が空なら分割できるcode pointがないため、本家どおり
  その要素から本文片を作らない。この特殊境界を通常のreader入力へ広げて解釈しない。
- 改行を保持して行を読み、先頭1〜6個の`#`にECMAScript whitespaceが続く行を節の開始とする。
  小さい文書でも節ごとに同一pathの本文片を作る。7個の`#`、字下げ、whitespaceなしは見出し扱いしない。
- 各節が大きければUnicode code point境界で分割する。JSONの`[{"path":...,"text":...}]`として
  送る場合のUTF-8 byte数を使い、引用符・backslash・制御文字のescapeも数える。
  Go標準JSONのHTML/U+2028/U+2029 escapeによる過剰計上はしない。
- 本文目標は20,480 bytes。順番に本文片を詰め、候補が目標を超えると現在chunkを確定する。
  目標ぴったりは同じchunkへ入り、超過分を後続と並べ替えて詰め直さない。
- pathだけで大きくなる場合でも、最小の1 code pointを捨てたり無限loopしたりせず保持する。
  20 KiBは分割目標であり、絶対上限ではない。本家の28 KiB directive上限はtoken等を含む
  完成した送信命令の検査であり、後続のtransport組立側で検証する。

新package、interface、可変budget APIは不要。既存`RuleContent`をそのまま使う。
JSON byte計算の短いprivate helperは局所配置し、knowledge package依存や全体共通化を持ち込まない。
readerが受理しない不正UTF-8 byte列を受け付ける新契約は作らない。

## 所有権とTDD順序

Go writerは1人。`src/internal/steering/chunks.go`と`chunks_test.go`のみを所有する。
親はこの計画・承認RAM・索引・`docs/architecture.md`・`docs/development.md`を担当し、Go writerと
同時にファイルを編集しない。`src/core/`、既存reader、knowledge、state、audit、CLIは変更しない。
ユーザーの未追跡HTMLと`work/`には触れない。

1. `ordered-content`: 小さい入力と空入力から、順序・内容を保持したchunkを返す。
2. `markdown-sections`: 本家の見出し境界で本文片を作り、改行と非見出しを保持する。
3. `greedy-budget`: 複数本文片をJSON byte数で順番に詰め、20,480ぴったりと+1を区別する。
4. `unicode-split`: 大きな単一節を日本語・絵文字の途中で切らず、元本文へ完全復元できる。
5. `json-escaping`: path・本文のJSON escapeを含む正確な容量で境界を決める。
6. `oversized-path`: 1文字でも目標に入らない場合に文字を保持して前進する。
   本文が空でpathだけが目標を超える場合は、分割対象のcode pointがない本家の境界も固定する。
7. `ownership`: 返却sliceの変更と再呼出しが入力や以前の結果を変更しない。

各sliceは指定test1件のRED依頼だけから開始し、担当が最終応答で終了した後、親が差分を確認し
同じcommandを再実行する。親のHEAD・対象file hash付き受入を渡して初めて別のGREEN依頼を出す。
GREENでも親が再実行する。既に正しく動く補足testはALREADY_GREENとして記録し、人工的に失敗させない。
最初のREDだけ`ChunkRules`のsignatureと空返却のcompile-only scaffoldを明示許可する。

## 実装記録

`ordered-content`、`markdown-sections`、`greedy-budget`、`unicode-split`は、各REDを親が同じcommandで
再現してから別handoffのGREENへ進み、親の再実行でも成功した。見出しtestの初版には、単純な行matchである
本家挙動と異なりfenced code全体を特別扱いする誤った期待値があったため、productionへ進む前にtest dataだけを
訂正し、訂正後のREDを改めて受け入れた。履歴は[#97の記録](https://github.com/sori883/ai-dd/issues/97#issuecomment-5544315311)
へ残している。

`json-escaping`、`oversized-path`、path超過かつ空本文の補足境界、`ownership`は、先行sliceで完成した
productionに対して追加testがそのまま成功したため`ALREADY_GREEN`として扱った。`oversized-path`のtest初版は
比較helperの型推論でcompile errorとなり、有効なREDではなかった。productionへ進まずtestだけを修正してから
再実行した結果を記録している。人工的な失敗や不要なproduction変更は作っていない。各sliceのhash、親による
同一commandの再実行、補足境界は[#97のTDD記録](https://github.com/sori883/ai-dd/issues/97#issuecomment-5544878269)
を参照する。

## 検証・依存・リスク・戻し方

loopは`GOTOOLCHAIN=go1.26.8 go test -count=1 -run '^TestChunkRules<当該名>$' ./src/internal/steering`の
1 testのみ。独立review後、安定した差分に対する親のread-only finalへ次を集約する。

- 全package unit test、race/shuffle、integration/race、通常およびintegrationのvet。
- `go mod tidy -diff`、`gofmt -l src`、変更Go fileの既存gopls診断、`git diff --check`。
- CLIとsteering test binaryの6 target cross compile。これは各OS上の実行証拠とはしない。
- 対象headで起動するGitHub checksが全件成功した後に既存方式でmergeし、main反映とIssue closeを確認。

外部Go module/toolは追加しない。I/O・永続data・公開CLIの契約変更はない。
主なリスクはJSON escapeやECMAScript whitespaceのGo移植差で、実装helperに依存しない期待値で固定する。
必要ならこの独立部品と関連文書だけをrevertでき、利用プロジェクトの保存data移行は不要。
新しい意図的な本家差分は採用しない。

## 後続への境界

後続は本文bundleのdigestと継続token、毎回再構築して古い続きを拒否するtransport、配置sourceと
知識一覧の接続、Codexがルール→知識→工程手順→入力資料を実読込する経路を順に扱う。
既存Nextはread-only、製品の人間承認と未対応工程のfail-closedは維持する。
後続で本家や既存承認から一意に決まらない重大な保存・安全性の選択が見つかった場合は、
分割部品と混同せず、その選択の根拠と影響を明示する。
