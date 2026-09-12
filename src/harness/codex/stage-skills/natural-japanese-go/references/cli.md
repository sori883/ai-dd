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
