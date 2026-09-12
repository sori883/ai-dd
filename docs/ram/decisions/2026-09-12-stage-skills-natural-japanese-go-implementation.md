# 工程skillとKagome版の日本語チェックを実装

日付: 2026-09-12。対応: [Issue #179](https://github.com/sori883/ai-dd/issues/179)。
[直接承認](2026-09-12-stage-skills-natural-japanese-go-approved.md)と
[具体計画](../../design/stage-skills-natural-japanese-go-plan.md)に基づく実装記録。

## 実装した構成

先に11の工程skillを追加し、その後にnatural-japanese-goのskillと別CLIを実装した。
原稿はsrc/harness/codex/stage-skills、利用プロジェクトへの配布先は.agents/skills。
既存のaidlc、aidlc-cli、aidlc-okfと合わせて15のskillになる。工程本文と既存担当の指示から、必要なskillを読む。
stage-plannerの担当は工程の採否・順序の提案のままで、実装計画はメインAIがaidlc-planningで作る。

hookの読取り対象は、配布原稿に存在する既知Markdownへ広げた。最大3ファイルの単純なcatという境界、
Ruleの確認、開始状態、実行中tool、symlink等の拒否は維持する。Sensorの必須資材、工程の許可担当、
承認、state、同じrootの単独writerは変更していない。子は本文案と根拠を返し、共有保存はメインAIがaidlc memoryで行う。

natural-japanese-goはUTF-8ファイルまたは標準入力を読み、14種類の通常検査を日本語またはJSONで返す。
genreとbaselineに対応し、入力を書き換えない。前回結果の比較は同じschema・解析器・辞書を条件とする。
文章の正しさや著者を判定するものではなく、AIが推敲するための材料である。

依存はKagome v2.11.0、kagome-dict/uni v1.2.6、必要なkagome-dict v1.1.7。
当初の調査ではIPA辞書やx/textも候補としたが、実装のimport graphとtidyでは不要だった。
UniDicのデータ版はunidic-mecab-2.1.2。辞書を補助CLIへ埋め込み、Python/uvや実行時の辞書取得を要求しない。
本体aidlcへ解析packageをリンクしない構造とし、最終検証で依存一覧も確認する。

配布ツールはproductにaidlcまたはnatural-japanese-goを指定でき、既定はaidlc。
補助CLIのarchiveにはREADMEと5つのライセンス文書を同梱する。既存配置を自動更新せず、stagingで比較して保全する。
工程本文の変更はdefinition hashへ影響するため、新規配置・新規Intentで利用する。旧Intentのhashを上書きしない。

## 原典との対応で確認したこと

固定natural-japanese v1.5.0のlint.py/textcore.pyを正本として、catalog・しきい値・統計式を移植した。
調査要約の句数・抽象語数には数え違いがあり、原典の構文解析で定型句48、抽象語24、例示語11、述語10と確認した。
実装はこの原典値を使う。原典のインデントcodeは除外しない。Unicode空白とCRLF/CRの扱いもGoへ対応させた。

UniDicの品詞・Lemma・Pronを使うため、Sudachiと区切り方や表記が異なる。
サ変名詞とするの連結は述語catalogの照合で扱う。通常14カテゴリの実装と、全入力での完全一致は区別する。
公開fixtureはGo版でai-smellyが25件、naturalが0件だったが、Pythonを同環境で実行した比較ではない。
期待件数に合わせて判定式を変更せず、カテゴリ別の正常・検出・境界例で検証した。

## 検証の位置付け

一人の実装担当が13項目のRED/GREENを実行し、末尾の対象テストを確認した。親も指定15コマンドを一度実行し、
全て成功した。文字数の先行testにあった誤記は、production前に原文の6文字へ訂正した。
gotestsは導入せず、標準testingでテストを記述した。

独立したskill適用では、深掘りの質問と用語整理、および未実施の試験・レビュー待ちを含む完了報告を確認した。
特定の相談に対する限定確認で、全ての依頼で適切に判断できるという保証ではない。

独立reviewでは、baselineの必須excerpt欠落・空値の受理、通常ファイル以外の入力で待ち続ける問題、
入力失敗時に理由を表示しない問題が見つかった。各指摘の回帰testでREDを確認し、単独writerが修正した。
FILEとbaselineは開く前と開いた後に通常ファイルかを調べ、UnixのFIFOは制限時間付きの子processで検証する。
引数・読込み・UTF-8・baseline・出力の失敗は標準エラーへ理由を示し、既存の終了codeと標準出力の契約を維持する。
親は修正後にbaselineとCLIの対象test群を一度実行し、両方の成功を確認した。全体検証は再review後に行う。

日本語skillの独立適用では、実CLIで原文5件から推敲案1件への変化を確認した。
残った統計指摘を理由なく消そうとせず、検討中・試験予定の情報も維持した。これも一つの例による確認である。

独立コードreview、read-only final、固定Codex 0.153.4の実測、GitHub checksの結果は、
Issue #179に紐づくPRと検証記録へ保存する。実Codex fixtureは既存の試験用trust helperを使い、
通常利用者のhook trustや全OSの全経路を確認したとは扱わない。
今回、本家AI-DLC固定参照の工程・承認・担当制限に新しい意図的差分を追加していない。
