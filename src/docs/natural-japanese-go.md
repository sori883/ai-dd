# natural-japanese-go

同梱の実行ファイルをPATHへ置くと、Python/uvや辞書の追加取得なしで日本語を検査できます。入力はUTF-8通常ファイルです。Unix系では実行権限を保ち、Windowsでは.exeを使います。

```
natural-japanese-go --help
natural-japanese-go --list-rules --json
natural-japanese-go --json --genre tech text.md
natural-japanese-go --json --baseline previous.json text.md
```

FILEが `-` なら標準入力です。flagはFILEの前後に置けます。JSONを前回結果として別ファイルへ保存して比較できます。入力を自動編集しません。exit 0は指摘ありを含む正常検査、1は入出力・解析・baseline失敗、2は引数不正です。severityはinfo/warn/criticalです。

schema_version=1、engine=Kagome v2.11.0、dictionary=UniDic v1.2.6のJSONでfile/stats/findingsを返します。baselineは同じGo版方式・辞書・schemaだけを受け付け、new/persisting/resolvedを返します。不正baselineは成功扱いしません。Sudachi版との全入力同一結果を保証しません。

通常14カテゴリを備えます。表層の定型句/翻訳調/対比/無生物主語、文長/段落CV、名詞終止、形態素の翻訳調/無生物主語、モーラburstiness/文頭反復、原形TTR/MTLD、具体性を検査します。短文では統計最低量に届かず判定しない場合があります。指摘は推敲の材料であり、著者や内容の正しさを判定しません。実験・reading-load・semanticは対応外です。

元規則: coji/natural-japanese v1.5.0 commit 9a78a42964096da509b8f3e011f0085a5f080151。解析器・辞書と実行環境をGoへ変更しています。UniDicのデータ版はunidic-mecab-2.1.2です。帰属と許諾はLICENSES/を参照してください。配布directoryのmanifest.jsonとSHA256SUMSでarchiveを照合します。

更新は別directoryで新版を確認してからPATHの実行ファイルを置き換えます。問題があれば以前のbinaryと対応資材へ戻します。AI-DLCの既設skill更新は別のstagingで比較し、利用者の変更を保全します。Ruleやstateを初期化しません。

## 規則と移植の対応


原典: https://github.com/coji/natural-japanese/tree/9a78a42964096da509b8f3e011f0085a5f080151 （v1.5.0、MIT）。[許諾全文](../harness/codex/stage-skills/natural-japanese-go/LICENSE)。設計→執筆→検査→収束をAI-DLCの担当とOKF保存へ調整した。

通常lintの14カテゴリ、catalog、統計式、genreを固定scripts/lint.pyとtextcore.pyから移植する。Kagome v2.11.0、kagome-dict/uni v1.2.6を使用し、Python/Sudachi、outline/terms、semantic、実験検査の実行は引き継がない。品詞はUniDicの階層、原形はLemma、読みは活用した発音形Pronを使う。汎用Reading APIはUniDicで未定義。サ変名詞＋するは述語catalogの照合時だけ連結する。辞書の分割や表記はSudachiと異なるため全入力の同一結果を保証しない。

Markdownのfrontmatter、見出し、箇条書き、引用、表、フェンス、コメント、inline codeとリンクURLを除外する。原典どおりインデントcodeは除外しない。CRLF/CRはLFへ正規化し行番号を保つ。名詞終止・語彙検査の文字数gateは解析した文の原文部分の合計であり、除外された見出しやcodeは加えない。

| 分類 | 判定・適用境界 |
| --- | --- |
| forbidden_phrase | 固定48句。行と句ごとに最初の一致。弱信号5句はinfo |
| translationese | 固定10表層pattern。全一致をinfo |
| antithesis_repetition | 3件以上。文数比率2%未満info、3%以上critical、その間warn。techはcritical 4.5%以上 |
| low_sentence_variance | 5文以上、Unicode文字数の母標準偏差/平均が0.25未満 |
| english_syntax_inanimate_subject | 固定2表層pattern。無生物主語の疑いをinfo |
| nominal_ending | 5文以上・解析文の原文合計2000字以上、名詞終止0件。essay1500字、tech/business3000字 |
| uniform_paragraph_structure | 4段落以上、段落あたり文数CVが0.15未満 |
| translationese_morph | 表層こと＋助詞が/は＋動詞のでき始まり。原典実装に合わせ出来表記は対象にしない |
| inanimate_subject_morph | 指示語/形式名詞＋が/はの後に固定述語catalogの原形。同じ二形態素主語を二重計上しない |
| low_burstiness | 6文以上、Pronまたはsurfaceのモーラ近似長で(σ−μ)/(σ+μ)が−0.24未満。小書きカナは同じ形態素の前字へ併合 |
| repeated_sentence_lead | 記号を除いた文頭2形態素が6回以上。essay5回、tech/business7回。info |
| low_lexical_diversity_ttr | 解析文の原文合計4000字以上、内容語30以上、原形TTRが0.45未満 |
| low_lexical_diversity_mtld | 同じ対象語でTTR 0.72以下ごとにfactor、端数factorを含む前後平均MTLDが40未満 |
| low_specificity | 80字以上の段落・内容語15以上。(固有名詞数＋数値hit数)/内容語数＋例示加点0.1−1.5×抽象名詞率が−0.15未満 |

2026-09-12のローカルtargeted fixture確認では、固定原典のai-smelly.mdで25件、natural.mdで0件を観測しました。これはGo実装の観測値であり、Python版を同環境で実行した比較ではありません。testは期待件数への一致を要求せず、catalogの検出と分類・行番号を確認しています。モーラburstinessはそれぞれ約−0.365485、−0.169135でした。UniDicの短単位分割・Lemma表記・PronはSudachiと異なります。文脈の自然さと検出件数を同一視しません。

baselineは行番号を同一性キーに含めません。集計5カテゴリはカテゴリ名、それ以外は空白を除いたexcerpt先頭20文字との組で多重集合比較します。同じ指摘が複数ある場合も1対1で対応付けます。誤った型や異なるengine/dictionary/schemaのbaselineはエラーです。
