# 工程skillを先に導入し、Kagome版の日本語lintを実装する承認

日付: 2026-09-12。状態: Accepted（ユーザーの直接実装依頼）。
対応は[Issue #179](https://github.com/sori883/ai-dd/issues/179)、分類はユーザーリクエスト。

[第一候補の選定とlint方式の比較](2026-09-12-stage-skills-selection-and-lint-options.md)の後、ユーザーは
「kagomeで実装をしてください。まずは他のskillsを導入して、netural-japanese-goを作ってください」と依頼した。
これを、採用済みの他11 skillを先に導入し、その後Kagomeでnatural-japanese-goを作る直接承認として記録する。
原典の綴りに合わせた名前を使い、AI-DLC本体とは別のCLIにする。Kagomeと必要な辞書・推移依存の追加は
この指定に含まれる。以前の「外部module未承認」「Go移植は調査段階」は、この範囲で置き換わる。

[具体計画](../../design/stage-skills-natural-japanese-go-plan.md)に、11 skillの配布名、参照版、対象file、
通常14カテゴリ、JSON/genre/baseline、別archive、単独writerの13項目TDD、独立review、read-only final、
固定Codexの限定実測と品質gate後のPR/mergeを記載した。

Go版はKagome v2.11.0とUniDic package v1.2.6を使う。日本語の品詞・原形・読みの取得に必要なため、
標準ライブラリに加えてこの依存を用いる。本体aidlcへ解析器をリンクせず、辞書は補助CLIへ含める。
Python/uvの実行や実行時の辞書取得は要求しない。原典の通常lint規則を移植するが、解析器と辞書の差による
判定の違いを完全互換と説明しない。実験的検査・reading-load・意味モデルは通常lintと別の機能として扱う。

追加skillは既存のステージ、許可担当、Sensor、人間承認、通常rootの単独writer、OKFへのCLI保存に従う。
既設を自動更新せず、旧Intent/stateの移行や本家AI-DLCの新しい工程方式を追加しない。
実装は専用branch `codex/stage-skills-natural-japanese-go`、作業場所は `/Users/const/sori883/ai-dd-naming`。
元checkoutの未commit資料と、この作業場所にある前回調査記録を保全する。

試験用trustを使う実Codex fixtureは通常trustの全経路確認とは区別して報告する。
自然言語のqualityは機械Sensorだけで保証しない。検出結果は推敲とレビューの材料にする。
