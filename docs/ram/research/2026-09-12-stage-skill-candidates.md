# AI-DLCの工程で使うskill候補の比較

調査日: 2026-09-12。状態: **調査・提案。候補の採用、導入、製品配布は未承認**。

ユーザーは、GitHub・X・ブログなどから各工程に役立つskillを集め、観点ごとに複数候補を示すよう依頼した。
指定された候補は `mattpocock/skills` の `grill-with-docs` と `coji/natural-japanese`。
本記録は調査結果であり、実装計画や新しいロードマップではない。製品、設定、Issue、PRは変更していない。

## 判断の前提

skillは「担当AIが作業するときに読む手引き」。ステージの実行順、担当の権限、プログラムが行うSensor検査、
人間の承認とは役割が異なる。良い質問や文章を書くskillを追加しても、それだけで検査や承認が強制されるわけではない。

現行製品は、初期化と目的整理・深掘りを必須とし、現状の構成分析、実装計画、TDD、統合検証の採否と順序を
Intentごとに決める。進行は `aidlc`、CLI操作は `aidlc-cli`、知識の検索・保存は `aidlc-okf` が案内する。
追加候補は、この仕組みの中で質問・調査・計画・実装・文章の質を高めるために比較した。

本リポジトリを開発する `.agents/skills/technical-research`、`implementation-planning`、`go-tdd`、
`go-code-review` は既にある。ただし、利用プロジェクトへ配布する製品skillとは別である。
AI-DLC本体がGo製であることから、利用プロジェクトもGo専用であるとは扱わない。

## 1. 目的と未確定事項を深掘りする

| 候補 | 得意なこと | この製品へ合わせる点 |
| --- | --- | --- |
| **[grill-with-docs](https://github.com/mattpocock/skills/blob/3cca18b368ae95cdbdebbff572ccafa662551015/skills/engineering/grill-with-docs/SKILL.md)** | 質問で曖昧な判断を解消し、用語と重要な設計判断を残す。指定候補の第一候補。 | 本体は `grilling` と `domain-modeling` を呼ぶだけで、両方が必要。固有のSkill tool呼出しをCodexの読込み手順へ合わせる。保存先と共有writerも変更が必要。 |
| [brainstorming](https://github.com/obra/superpowers/blob/b36e0829c6d0140e93cfef2ca599b1b07d4a7797/skills/brainstorming/SKILL.md) | 目的・制約・成功条件を質問し、複数案の長短を比較する。一問ずつ進めたい場合の別候補。 | 独自の作業分類、保存先、Git commit、次skillへの移行をそのまま持ち込まず、既存の目的整理と承認へ対応付ける。 |
| [grill-me](https://github.com/mattpocock/skills/blob/3cca18b368ae95cdbdebbff572ccafa662551015/skills/productivity/grill-me/SKILL.md) | 文書作成を伴わない質問用の入口。小さな相談に向く。 | `grilling` への依存がある。確定事項を残す責任は既存の要件整理・OKF保存側に置く。 |

現行の [grilling本体](https://github.com/mattpocock/skills/blob/3cca18b368ae95cdbdebbff572ccafa662551015/skills/productivity/grilling/SKILL.md)は、
互いに依存しない質問を一回にまとめ、回答が必要な後続質問は次の回へ送る方式。
一部の紹介文にある「常に一問ずつ」とは一致しない。質問量を少なくする方針を採用するなら、組込み時に明示する。
コードから確認できる事実とユーザーが決める選択を分ける点は、この製品にも有効と判断した。

[domain-modeling](https://github.com/mattpocock/skills/blob/3cca18b368ae95cdbdebbff572ccafa662551015/skills/engineering/domain-modeling/SKILL.md)は
用語を `CONTEXT.md`、判断を `docs/adr/` へ直接書く。AI-DLCでは、用語や確定要件をOKF文書、設計変更の理由を
`knowledge/adr/` へ保存する。原典のADR作成条件をそのまま採用して、既存のADR要否契約を変更してはいけない。

## 2. 公式資料や外部技術を調査する

| 候補 | 得意なこと | この製品へ合わせる点 |
| --- | --- | --- |
| **[research](https://github.com/mattpocock/skills/blob/3cca18b368ae95cdbdebbff572ccafa662551015/skills/engineering/research/SKILL.md)** | 公式文書・実装・仕様などの一次資料から、根拠付きの短い調査結果をまとめる。小さく組み込みやすい。 | 原典は背景agentを起動して直接Markdownを保存する。起動はメインAI、調査担当は根拠と本文案の返却、共有保存はメインAIへ分ける。 |
| [ECC deep-research](https://github.com/affaan-m/ECC/blob/c4904e3f6381df934fc00bffb0afa7a1f8dae0e3/skills/deep-research/SKILL.md) | 複数の調査質問、資料の突合、出典付き報告書を扱う。広い比較調査向け。 | FirecrawlまたはExaのMCPが前提。新しい接続、認証や料金条件を要し得るため、そのままの導入は別判断。小さな調査に一律の資料数目標を課さない。 |

開発用の既存 `technical-research` も、版・根拠・未確認事項を区別する基礎として利用できる。
製品へ取り込むなら、開発用RAMへ書く案内と利用プロジェクトのOKF保存を分離する。

## 3. 既存のコードと構成を説明する

| 候補 | 得意なこと | この製品へ合わせる点 |
| --- | --- | --- |
| **[Blueprint architecture](https://github.com/owainlewis/blueprint/blob/2aeb882f06bc4b307015ea962b73aa60ac0c8ea7/skills/architecture/SKILL.md)** | 実装済みの構成、依存方向、データの流れ、責任と権限をコードから検証して説明する。現状解析の第一候補。 | 原典のルート `ARCHITECTURE.md` を、共有の `codekb/current-analysis.md` と `codekb/architecture.md` に合わせる。調査担当は草稿を返す。 |
| [improve-codebase-architecture](https://github.com/mattpocock/skills/blob/3cca18b368ae95cdbdebbff572ccafa662551015/skills/engineering/improve-codebase-architecture/SKILL.md) | 理解しにくい責任分割やテストしづらい境界を探し、改善候補を図で比較する。改善目的の分析に向く。 | 現状を説明するだけのskillではない。`codebase-design` 依存、既定のGit履歴調査、HTML/CDN出力を含む。実装済みの事実と提案を分け、Gitなしの対象にも対応させる必要がある。 |

「現状がどうなっているか」は前者、「どこを変えると良くなるか」は後者が適する。
目的整理ステージと構成分析ステージを混同しない。

## 4. 要件・設計文書を読み手に伝える

| 候補 | 得意なこと | この製品へ合わせる点 |
| --- | --- | --- |
| **[to-spec](https://github.com/mattpocock/skills/blob/3cca18b368ae95cdbdebbff572ccafa662551015/skills/engineering/to-spec/SKILL.md)** | 深掘り済みの会話を、問題・解決案・利用例・設計判断・テスト方針へ整理する。再度の全面的な聞き取りを避けられる。 | 原典はIssue trackerへ公開し、専用ラベルを付ける。製品ではまず `design/<intent_id>/requirements.md` などへOKFとして保存する。原典のファイルパスを省く指示を、具体的な対象ファイルが必要な実装計画へ適用しない。 |
| [doc-coauthoring](https://github.com/anthropics/skills/blob/34040c9c568585f6929bedeaad110ad08f079624/skills/doc-coauthoring/SKILL.md) | 背景収集、節ごとの推敲、会話を知らない読者による理解確認を行う。初心者向けの説明や大きめの仕様書に向く。 | 原典は多めの質問・案出し・読者役agentを含む。既に確定した事項を聞き直さず、読者役はメインAIが既存権限内で依頼する。 |

推奨は、確定内容の整理に `to-spec`、説明の分かりやすさの確認に `doc-coauthoring` の読者確認手順を使うこと。
全文書へ二つの対話手順を重ねる必要はない。

## 5. 実装計画とUnitへの分割

| 候補 | 得意なこと | この製品へ合わせる点 |
| --- | --- | --- |
| **[mblode planning](https://github.com/mblode/agent-skills/blob/f05d2de8cbd88f11a4e3c99f2880f32491c61393/skills/planning/SKILL.md)** | 既存コードを根拠に、成果・対象・重要判断・検証・復旧を自己完結した計画へまとめる。計画の評価もできる。 | 保存先をOKFの実装計画へ合わせ、既存の実装許可と計画承認を正本にする。現在の原典は、点数を全項目5/5にするまで反復する契約ではない。 |
| [writing-plans](https://github.com/obra/superpowers/blob/b36e0829c6d0140e93cfef2ca599b1b07d4a7797/skills/writing-plans/SKILL.md) | 対象ファイル、担当間の入出力、テストと期待結果を細かな手順へ落とす。別担当への引継ぎに向く。 | 原典の頻繁なcommit、実行skillへの引継ぎ、独自の計画保存先を調整する。全実装コードを計画へ先書きする規則はTDD方針と合わせて取捨選択する。 |
| [to-tickets](https://github.com/mattpocock/skills/blob/3cca18b368ae95cdbdebbff572ccafa662551015/skills/engineering/to-tickets/SKILL.md) | 一つずつ確認できる機能単位へ作業を分け、先行作業への依存関係を明示する。Unit分割の参考に適する。 | 原典はtrackerまたは `.scratch/` にチケットを作る。AI-DLCのUnitと既存stateへ合わせ、新しい進捗台帳を作らない。分割できることと、同じフォルダへ同時に書けることを区別する。 |

ここで扱うのは「どう実装するか」の計画。どのステージを実行するかは、既存の `aidlc-stage-planner` が案を返し、
ユーザーが承認する別の責任である。

## 6. テストを先に書いて実装する

| 候補 | 得意なこと | この製品へ合わせる点 |
| --- | --- | --- |
| **[Matt Pocock tdd](https://github.com/mattpocock/skills/blob/321658273cb1d20b76026717d027d505790106d4/skills/engineering/tdd/SKILL.md)** | 公開された振る舞いを入口に、一つの失敗テストから最小実装へ進む。実装の写しになったテストを避ける。 | テスト対象の境界を承認済み計画で決め、毎項目の追加承認へ変えない。利用プロジェクトの言語に合わせる。 |
| [test-driven-development](https://github.com/obra/superpowers/blob/b9e75dddec7a384f42ce08532ec17bb1ef5d9459/skills/test-driven-development/SKILL.md) | テストが意図した理由で失敗することを確認し、その後に通す規律が明確。 | 原典にある先書き実装の削除を機械的に適用しない。既存の変更や他者の成果を保全し、対象に応じた回帰テストへ落とす。 |

AI-DLC本体のGo開発には、既存の `go-tdd` を引き続き使う。上記の導入だけで製品workerに適用済みとは扱わない。

## 7. 不具合の原因を調べる

| 候補 | 得意なこと | この製品へ合わせる点 |
| --- | --- | --- |
| **[systematic-debugging](https://github.com/obra/superpowers/blob/c74782ead66b8ded584d9b9cf64dcba95457f320/skills/systematic-debugging/SKILL.md)** | 再現、正常例との比較、仮説の検証、修正の順に進む。思いつきの修正を繰り返さないための第一候補。 | 調査範囲を現在Intentへ限定し、修正はTDDと既存の実装許可へ戻す。 |
| [diagnosing-bugs](https://github.com/mattpocock/skills/blob/321658273cb1d20b76026717d027d505790106d4/skills/engineering/diagnosing-bugs/SKILL.md) | 速く繰り返せる再現方法を確立し、複数仮説の優先順位を付けて原因を絞る。 | 調査結果だけを完成とせず、回帰テストと修正結果へ接続する。独自の工程管理は追加しない。 |

Go固有の不具合では開発用 `golang-troubleshooting` も候補となる。外部debugger等の導入を自動的に許可する意味ではない。

## 8. 実装を独立した視点でレビューする

| 候補 | 得意なこと | この製品へ合わせる点 |
| --- | --- | --- |
| **[Matt Pocock code-review](https://github.com/mattpocock/skills/blob/5c89081d4bbeb3d039a42093653f90bb698d780e/skills/engineering/code-review/SKILL.md)** | 開発規約への適合と、要求仕様への適合を別の観点で確認する。 | 原典自身の担当起動をそのまま使わず、メインAIが既存reviewerへ必要な観点を渡す。レビューの比較範囲を固定する。 |
| [requesting-code-review](https://github.com/obra/superpowers/blob/cfb6281371ef2d2b7937b22eb475a11a9644ff87/skills/requesting-code-review/SKILL.md) | 変更内容・要求・比較範囲を揃えてレビューを依頼する。依頼漏れの防止に向く。 | レビュー実施そのものより依頼手順が中心。Git SHA前提を、製品が保存する検証範囲・集合SHA・証拠へ合わせる。 |

本体のGo開発は既存 `go-code-review` が承認範囲・正しさ・テスト証拠を確認する。製品reviewerへ配布する案と区別する。

## 9. 統合・完了を証拠で確認する

| 候補 | 得意なこと | この製品へ合わせる点 |
| --- | --- | --- |
| **[verification-before-completion](https://github.com/obra/superpowers/blob/3be5aad3dd2400ef23b15680969f4bcd3b6d7b8b/skills/verification-before-completion/SKILL.md)** | 「成功した」と報告する前に、その主張に対応するコマンド、終了結果、最新の証拠を確認する。 | 既存Sensorの代わりにはしない。開発側では `loop` の限定テスト、独立 `review`、差分安定後のread-only `final` という検証分担を維持する。 |
| [webapp-testing](https://github.com/anthropics/skills/blob/ef740771ac901e03fbca3ce4e1c453a96010f30a/skills/webapp-testing/SKILL.md) | ブラウザで操作し、画面や動作を確認する。利用プロジェクトがWebアプリの場合の候補。 | Python・Playwright・ブラウザ等が前提。CLIだけの対象に一律導入せず、Go単一バイナリの配布要件へ混ぜない。 |

この二つは用途が異なる。前者は完了判断の共通規律、後者はWeb向けの具体的な検証方法として比較する。

## 10. 自然で読みやすい日本語にする

| 候補 | 得意なこと | この製品へ合わせる点 |
| --- | --- | --- |
| **[natural-japanese](https://github.com/coji/natural-japanese/tree/9a78a42964096da509b8f3e011f0085a5f080151/skills/natural-japanese)** | 業務文書の翻訳調、冗長さ、文のリズムなどを整える。指定候補で、全工程の本文推敲に適する。 | 文体の指針と自動検査は分けて選べる。自動検査にはPython/SudachiPy等が必要。独自agent起動や保存先はAI-DLCへ合わせる。 |
| [japanese-tech-writing](https://gist.github.com/k16shikano/fd287c3133457c4fd8f5601d34aa817d) | 技術文書の段落・論証・用語を明確にし、読者の負担を減らす。文章による規範が中心で追加実行環境を要しない。 | 日本語KnowledgeとADRの本文に向く。Gist配布なので採用時には履歴を固定する。 |

`natural-japanese` の確認版はv1.5.0。`lint.py` 等は `uv run`、Python 3.10以降、SudachiPyと辞書を使う。
意味の検査用 `semantic.py` は別途大きなモデル取得もあるため、Markdown指針の利用と同じ導入規模として説明しない。
まず [手動確認の指針](https://github.com/coji/natural-japanese/blob/9a78a42964096da509b8f3e011f0085a5f080151/skills/natural-japanese/references/manual-checklist.md)
を使う案なら、製品の実行依存を増やさずに検討できる。ただし原典の全機能を導入したことにはならない。

自動検査は読みやすさの疑いを見つける補助であり、事実が正しいことや要件を満たすことの証明にはならない。
OKFのfrontmatter、コード、パス、CLI引数、数値の意味を推敲で変更せず、本文案の保存は `aidlc-okf` に従う。

比較した追加候補のうち、[cognitive-rhythm-writing](https://gist.github.com/k16shikano/eb2929f13ed19c97188393d297be8432)は
読み物向けで、原典もADRや手順書への不使用を指定するため、今回の第一候補にはしなかった。
[writing-clearly-and-concisely](https://github.com/jjmartres/opencode/blob/28dc323162d2a38a5ac27944cc204697373936ea/opencode/skills/writing-clearly-and-concisely/SKILL.md)
は英語文書向けの代案であり、日本語専用の二候補と同じ効果があるとは扱わない。

## このAI-DLCへ組み込む場合の提案

初回候補は、深掘りの `grill-with-docs`、現状解析の `architecture`、要件文書の `to-spec`、
原因調査の `systematic-debugging`、日本語の `natural-japanese` と `japanese-tech-writing`。
不足する作業方法を補う点を重視した推薦であり、性能比較による順位ではない。

| 適用する場面 | 主に使う担当 | 保存と既存契約への接続 |
| --- | --- | --- |
| 初期化 | メインAI、既存の計画担当・reviewer | `aidlc` と `aidlc-cli` の既存手順を使う。外部setup skillを追加する必要性は今回確認していない。 |
| 目的整理・深掘り | メインAIと `aidlc-requirements`、必要時に `aidlc-researcher` | 専属担当から質問案・根拠を返し、ユーザーとの対話はメインAI。確定要件は `design/<intent_id>/requirements.md`。 |
| 実行する工程の選択 | `aidlc-stage-planner` とメインAI | 採否・順序の案をユーザーへ提示。既存の計画と承認を用いる。外部skillが独自にステージを進めない。 |
| 現状の構成分析 | `aidlc-researcher` | 共有の `codekb/current-analysis.md` と `codekb/architecture.md` の本文案を返す。 |
| 実装計画 | メインAI、必要な計画担当・reviewer | `design/<intent_id>/implementation-plan.md` と既存Unitへの案をまとめる。 |
| TDD・修正 | `aidlc-worker`、独立 `aidlc-reviewer` | 既存の担当範囲と同じrootの順次作業を維持する。 |
| 統合検証 | メインAIと `aidlc-reviewer` | 既存の検証証拠、Sensor、成果レビュー、人間承認を維持する。 |
| 日本語の推敲 | 本文を作る各担当、保存はメインAI | Knowledgeの「何を・どうやって」とADRの「なぜ」を分け、CLIでmetadataを生成・更新する。 |

上表の保存先は `aidlc/spaces/<space>/knowledge/` からの相対位置。
作業の進捗はstate、差戻し理由等はCLI管理の `log/<intent_id>-work-log.md` に残す。
外部skillの `CONTEXT.md`、tracker、独自の計画台帳を並列の正本にはしない。
利用プロジェクトの `rules/rule.md` にAI-DLC固有の操作手順を移す案でもない。

採用時には、選んだskillと参照版、必要な補助ファイル、保存先の変更、許可するtoolと担当、配布先、
ユーザーの既存設定を保つ更新方法、実機での読込み確認を一つの計画にする必要がある。
今回それらの具体実装、依存導入、skillの実行比較、Codex実機試験は行っていない。

## 調査の根拠と限界

- ローカル参照は `ai-dd-naming` のHEAD `66cc21a6c3ba5eae9badcb0f3e027df34ed6cde3`。
  調査開始時の `origin/main` は `c43a1ba1b532f6f635eb49bff3bd5ca4d7a497cb` で、両者のファイル内容は同じ。
  元checkout `/Users/const/sori883/ai-dd` の未commit資材は変更していない。
- [Stage Graph](../../../src/core/workflow/stage-graph.json)、[段階別手順](../../../src/core/workflow/stages/)、
  [OKF skill](../../../src/harness/codex/aidlc-okf/SKILL.md)、
  [役割分離の合意](../decisions/2026-09-10-project-rule-and-cli-skill-approved.md)、
  [Git不要化の承認](../decisions/2026-09-11-git-independent-implementation-approved.md)、
  [ステージ計画担当の役割](../decisions/2026-09-09-stage-planner-agent-request.md)を比較の基準にした。
- 外部skillの原稿、補助資料、GitHubの版を確認した。固定URLは確認したスナップショットを示す。
  `japanese-tech-writing` の確認時Gist履歴は `8f2d57610a73`。導入時は改めて固定する。
  検索やContext7の説明が原稿と異なる場合は、固定した原稿の挙動を優先して比較した。
- 候補発見には [skills.sh](https://skills.sh/)、GitHub、ブログ、X検索を使った。
  作者の [grillingの説明](https://www.aihero.dev/skills-grilling)、
  [grill-with-docsの説明](https://www.aihero.dev/skills-grill-with-docs)、
  [natural-japaneseの説明記事](https://zenn.dev/coji/articles/natural-japanese-ai-smell-lint)も参照した。
  Xは検索結果に転載・ミラーが多く、確認対象の原投稿を取得できなかったため、仕様や効果の根拠には使っていない。
- 実際に使った際の品質、速さ、token消費を同じ課題で計測していない。原稿の構造と現行契約からの適合評価である。
  紹介記事の評判やstarsだけでは推薦を決めていない。
- 再配布するならskillと補助ファイルのライセンス・帰属を参照版ごとに確認する。
  AnthropicのREADMEは多くのskillをApache 2.0と説明する一方、一部文書skillは別条件と説明している。
  リポジトリ全体を単一ライセンスとみなさず、採用対象の適用範囲を配布計画で確定する。

本記録は既存の決定を置き換えず、候補比較と未決定事項を保存する。
