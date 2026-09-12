# 工程別skillの第一候補を選び、日本語lintの実行方式を検討する

日付: 2026-09-12。
状態: **第一候補10件と補助2件の採用方針に同意。lintのGo移植は実現性の調査段階**。

## ユーザーの回答

[工程別skill比較](../research/2026-09-12-stage-skill-candidates.md)への回答として、次の指定を受けた。

> 第一候補全て良いと思います。
> `grilling`と`domain-modeling`も入れて良いです。

続いて、natural-japaneseのlintはローカルのPythonが必要か、Goで再現できるかを質問した。
AI-DLCのシングルバイナリへ取り込まなくてもよい、という許容も明示された。

この回答で、比較時点の「候補未選択」を以下の採用方針へ更新する。
自然言語の手引きの採用と、外部プログラム・辞書・ライブラリの導入は区別する。
Go移植の実装範囲や判定の互換性は、今回の質問だけでは確定していない。

## 採用するskillの範囲

| 観点 | 選択されたskill | 配布元 |
| --- | --- | --- |
| 目的の深掘り | grill-with-docs | mattpocock/skills |
| 公式資料の調査 | research | mattpocock/skills |
| 現状の構成分析 | architecture | owainlewis/blueprint |
| 要件・設計の文書化 | to-spec | mattpocock/skills |
| 実装計画 | planning | mblode/agent-skills |
| TDD | tdd | mattpocock/skills |
| 不具合調査 | systematic-debugging | obra/superpowers |
| 独立レビュー | code-review | mattpocock/skills |
| 統合・完了確認 | verification-before-completion | obra/superpowers |
| 自然な日本語 | natural-japanese | coji/natural-japanese |
| 深掘りの補助手順 | grilling | mattpocock/skills |
| 用語・判断の整理 | domain-modeling | mattpocock/skills |

比較表の第二候補や、追加参考として紹介したto-ticketsは、この回答で選択された12件には含めない。
補助資料の必要性は各原稿から確認し、採用する本体・版・参照ファイルを具体計画に列挙する。
Skill全体のセットを無条件で導入する合意ではない。

既存のステージ、Sensor、人間承認、メインAIによる担当起動、通常フォルダの順次作業、共有writer、
OKFへのCLI保存を維持する。原典のCONTEXT.mdや独自trackerを進捗・知識の別の正本として追加しない。
具体的な製品原稿、配布・更新、参照接続、検証の計画を整えてから実装する。

## Pythonが必要か

確認したnatural-japaneseはv1.5.0、commit `9a78a42964096da509b8f3e011f0085a5f080151`。
[`lint.py`](https://github.com/coji/natural-japanese/blob/9a78a42964096da509b8f3e011f0085a5f080151/skills/natural-japanese/scripts/lint.py)は
Python 3.10以降、SudachiPy、日本語辞書を実行時に必要とする。
スクリプトの宣言は `sudachipy>=0.6.8`、`sudachidict-core>=20240409` で、完全固定の依存版ではない。

ただし、事前にPythonを手動導入しておくことが絶対条件ではない。
原典が案内する `uv run` は、必要なPythonがなければ取得し、スクリプトの依存環境も用意できる。
これはPythonが不要になる方式ではなく、準備をuvへ任せる方式である。
自動取得が許可され、対応する実行環境とネットワークがあることが前提となる。
根拠: [uvのスクリプト実行](https://docs.astral.sh/uv/guides/scripts/)、
[Python取得の設定](https://docs.astral.sh/uv/concepts/python-versions/)。

## Goへ移植する場合の違い

以下は実装の読み取りに基づく技術評価であり、Go版の実測結果ではない。

| 検査の種類 | Goでの見通し |
| --- | --- |
| 定型句、翻訳調の文字列パターン、対比表現の反復 | 標準ライブラリの文字列処理や正規表現などで移植できる。PythonとGoの正規表現の差は個別に扱う。 |
| 文の長さのばらつき、段落ごとの文数、Markdownのコード等の除外 | 標準ライブラリで移植できる。文字数とUTF-8のバイト数、元の行番号、frontmatterの扱いを合わせる必要がある。 |
| 体言止め、助詞や活用を含む表現、語彙の繰り返し、固有名詞の判定 | 単語への分割、品詞、基本形を得る日本語解析器と辞書が必要。標準ライブラリにこれらの一式はない。 |
| 読み方を使った文のリズム | 原典はSudachiの読みを利用する。文字数だけに置換すると判定の意味が変わる。 |

原典の共有処理は [`textcore.py`](https://github.com/coji/natural-japanese/blob/9a78a42964096da509b8f3e011f0085a5f080151/skills/natural-japanese/scripts/textcore.py)。
標準出力対象のカテゴリと、実験的な構造・読解負荷の検査は区別されている。
Go版でどの範囲を扱うかは明示し、未実装の検査を実施済みや合格として表示しない。

日本語解析のGo製候補には [Kagome](https://github.com/ikawaha/kagome/blob/602ef6c55c03d51e6cf6b7e43e4634f0668fe096/README.md) がある。
pure Goで実装され、IPA/UniDicの辞書をバイナリへ含める構成を持つ。確認commitは
`602ef6c55c03d51e6cf6b7e43e4634f0668fe096`。利用時には外部Go moduleと辞書の採用が必要になる。
Sudachiと解析器・辞書が異なるため、同じ単語分割・品詞・読み・最終警告になるとは断定できない。
Kagome自身も[辞書ごとの分割・品詞の違い](https://github.com/ikawaha/kagome/wiki/About-the-dictionary)を示している。

本家と同じ検出を重視するなら、Python版を別ツールとして使う方法が最も直接的。
Python自体を不要にしたいなら、AI-DLCとは別のGo製文章チェックCLIを作る案が成り立つ。
その場合、表層検査だけの軽量版と、日本語解析まで備える版を区別して選ぶ。
Go版のための外部module、辞書、Python、uvはいずれも今回追加していない。

## 別機能と互換性の検証

[`semantic.py`](https://github.com/coji/natural-japanese/blob/9a78a42964096da509b8f3e011f0085a5f080151/skills/natural-japanese/scripts/semantic.py)は
通常lintとは別の実験的機能で、機械学習のライブラリとモデルを使う。
lintのGo移植という説明に、この機能の再現まで含めない。
原典の[手動確認](https://github.com/coji/natural-japanese/blob/9a78a42964096da509b8f3e011f0085a5f080151/skills/natural-japanese/references/manual-checklist.md)も利用可能だが、
自動lintを実行した証拠や同じ検出結果にはならない。

公開の固定fixtureには [`ai-smelly.md`](https://github.com/coji/natural-japanese/blob/9a78a42964096da509b8f3e011f0085a5f080151/skills/natural-japanese/scripts/fixtures/ai-smelly.md) と
[`natural.md`](https://github.com/coji/natural-japanese/blob/9a78a42964096da509b8f3e011f0085a5f080151/skills/natural-japanese/scripts/fixtures/natural.md) がある。
通常lintの期待件数はそれぞれ25件と0件と記載されている。今回は実行していない。
移植時には、これらだけで完全互換とせず、検査ごとの正常例・検出例・境界例を追加し、Python・辞書・
設定を固定した原典と行番号・カテゴリ・検出内容を比較する必要がある。
文章の検査結果はAIによる推敲の材料であり、文書の内容が正しいことを保証するSensorにはしない。

## 残る選択

1. Pythonの手動準備を省きたいのか、実行時のPython依存もなくしたいのか。
2. Go版なら、移植する検査の範囲と本家との一致をどこまで求めるか。
3. 日本語解析までGo化する場合の解析器・辞書・外部moduleの採用。

ユーザーは実行ファイルをAI-DLC本体へ統合しなくてもよいと指定しているため、別CLIを検討する許容はある。
この許容だけで、別CLIを実装済み、外部依存を承認済み、全検査のGo移植を実装承認済みとは扱わない。
