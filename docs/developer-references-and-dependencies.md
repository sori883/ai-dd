# AI-DLC 0.1.0 開発者向け：参考元と依存関係

この文書は、`sori883/ai-dd`を開発・配布する人が、何を参考に作った製品で、どの外部部品を使い、公開前に何を確認する必要があるかを把握するための一覧です。過去の会話を読む必要はありません。

初回バージョンはユーザー指定の **`0.1.0`** です。**この文書をユーザーが確認した後にリリースへ進みます。現時点ではtagもReleaseも作成していません。** バージョンの決定は、製品ライセンスの決定や公開済みであることを意味しません。

調査日：2026-09-13。製品コードの基準は[PR #187](https://github.com/sori883/ai-dd/pull/187)を取り込んだmain、commit `0889ff5a8023e53f3c781fa180f4afe0d5897c47`です。今回のライセンス追記は、文書追加の[PR #189](https://github.com/sori883/ai-dd/pull/189)まで反映した`742da9457b7d836a144ac3c77bb16d91e0677312`を照合しました。製品コードや依存バージョンは変更していません。

## 1. この製品と、外部の部品との関係

本リポジトリのAI-DLCは、AIと目的を整理し、必要な工程を計画し、検査・レビュー・人間の承認を経て開発を進めるためのGo製CLIです。CLIは、ターミナルからコマンドで操作するプログラムを指します。本家AI-DLCの考え方を参考にしていますが、本家の全機能をそのまま配布する製品ではありません。

利用者向けの本体は`aidlc`一つです。工程の定義、AIへの手順であるskill、担当AIの定義、操作前後に検査を挟むhook、初期文書を実行ファイルに同梱します。AI自体は別途用意したCodexで動きます。

| 作る実行ファイル | 役割 | 誰が使うか |
| --- | --- | --- |
| `aidlc` | 資材の配置、Space・Intent・知識・進捗・承認・検査の管理 | 製品の利用者と、その作業を行うAI |
| `natural-japanese-go` | 日本語の文章から推敲の候補を検出する補助CLI | 日本語チェックを使う人。`aidlc`とは別の実行ファイル |
| `aidlc-dist` | 作成済みの実行ファイルをOS別の圧縮ファイルへ梱包する開発用CLI | この製品の配布担当。利用先への導入は不要 |

「参考リポジトリ」は設計や手順を学んだ出典です。「依存ライブラリ」はプログラムが使う他の部品です。「同梱文書」は製品内に入る手順や許諾文です。この三つは、必要なインストールや更新方法が異なります。

```text
Codex（利用者が別途用意するAI実行環境）
  └─ 配置されたAI-DLCのskill・agent・hook
       └─ aidlc
            ├─ YAMLライブラリ：文書先頭の設定や工程定義を読む
            ├─ 同梱資材：初回配置に使う
            └─ project/aidlc/spaces/<space>/：利用プロジェクトの記録

必要な場合だけ別途使う：
  natural-japanese-go → Kagome + UniDic辞書
  aidlc-github skill → Git + GitHub CLI（gh）+ GitHub
```

Spaceは関連する作業と知識をまとめる場所、Intentは一つの目的を持つ作業単位です。利用プロジェクトの知識は`aidlc/spaces/<space>/knowledge/`に置きます。`codekb/`には現行の仕様・使い方、`design/`には要件・計画、`adr/`には設計判断の理由、`rules/`にはプロジェクト共通ルール、`log/`には作業ログを置きます。本リポジトリの開発判断を保存する`docs/ram/`とは別のものです。

現在のmainで配置できるAI環境はCodexです。Claude Code対応は[Issue #185](https://github.com/sori883/ai-dd/issues/185)で未完了・保留中、VS CodeのGitHub Copilot対応も現行の公開対象に含めていません。

## 2. 設計・知識管理の参考リポジトリ

固定commitは、Gitの特定時点の内容を識別する番号です。以下のリンクは分かる範囲でその番号に固定しています。固定した参考版と、各リポジトリの最新版は同じとは限りません。

| 参考元 | 確認した版 | この製品で参考にしたもの | 利用者が別途導入するか |
| --- | --- | --- | --- |
| [AWS Labs AI-DLC Workflows](https://github.com/awslabs/aidlc-workflows/tree/v2) | ローカルsnapshotの製品版`2.6.123`。上流commitは未確認 | 工程、担当AI、Sensor、承認、Space、共通coreからAI環境別資材を配置する考え方 | 不要。本家のTypeScript/Bun製runtimeを起動しない |
| [Google Open Knowledge Format](https://github.com/GoogleCloudPlatform/open-knowledge-format/tree/ad30107c31c06aec8a7d5636e0d1058118604e6f) | 仕様v0.2、`ad30107c31c06aec8a7d5636e0d1058118604e6f` | Markdownに本文とmetadataを持たせる知識形式、Bundle・Concept・来歴の考え方 | 不要。形式の仕様を参考にする |
| [OKF Agent Memory：初期実装の比較基準](https://github.com/okf-memory/okf-agent-memory/tree/d4c523ed5ce916fa207fe314851b98721421c891) | v0.1.2、`d4c523ed5ce916fa207fe314851b98721421c891` | 単一CLIで知識を作成・検索・更新する方式の検討 | 不要。外部`okf`実行ファイルやGo moduleを組み込んでいない |
| [OKF Agent Memory：後発skillの参考](https://github.com/okf-memory/okf-agent-memory/tree/a09e04918aa84d275b784374b5236d9eeac56c9e) | `a09e04918aa84d275b784374b5236d9eeac56c9e` | `aidlc-okf`の「検索して読み、必要な知識を残す」手順 | 不要。既存の`aidlc memory`操作に合わせている |

OKFは文書の**形式**、OKF Agent Memoryはその形式を使う**別プロジェクトの実装・手順**です。本製品の知識操作は[自前のGo実装](../src/internal/okfmemory/)です。Agent Memoryの二つのcommitは用途別の参照であり、後発skillのcommitへ製品全体を更新したという意味ではありません。

文書先頭の`---`で囲んだ設定をfrontmatterと呼びます。AIが本文を考え、CLIが`type`、`title`、`description`、`tags`、日時などを揃えて保存します。Intentとの関連には`intent_id`を使います。現在の検索はtitle・description・tagsの語句照合と`intent_id`の完全一致です。Agent Memory本家のMCP、本文全文検索、検索順位付けの実装まで導入したものではありません。

本家AI-DLCの参考snapshotには33工程がありますが、現行製品は初期化・目的整理と深掘りを必須とし、構成分析・実装計画・TDD・統合検証を目的に応じて選ぶ構成です。実行計画と変更には人間の承認を使います。旧本家原稿140件は現在の`src/core/`には残っておらず、[過去の取得記録](aidlc-content/SOURCE.md)とライセンスなどが履歴として残っています。その記録内の旧配置一覧を、現在の配布一覧として扱わないでください。

根拠は[AI-DLCの分析基準](aidlc-analysis/README.md)、[OKF仕様の取得記録](okf-analysis/upstream/SOURCE.md)、[初期実装の参照境界](ram/decisions/2026-09-08-m1-implementation-plan.md)、[OKF skillの採用記録](ram/decisions/2026-09-12-aidlc-okf-skill-approved.md)です。元の開発フォルダにある`docs/実装_okf-agent-memory/`も参考資料ですが、元commitは未確認で、通常のcloneやReleaseに含まれる資料ではありません。

## 3. 同梱するskillの参考元

skillはAIに渡す作業手順のMarkdownです。元プロジェクトの作業方法を日本語化し、この製品の担当、承認、知識保存の役割に合わせています。原典に出てくる追加ツールや独自の保存先を、そのまま製品の必須条件にしているわけではありません。

| 参考リポジトリと固定commit | 製品内のskill | 用途 |
| --- | --- | --- |
| [mattpocock/skills](https://github.com/mattpocock/skills/tree/3cca18b368ae95cdbdebbff572ccafa662551015) | `aidlc-grill-with-docs`、`aidlc-grilling`、`aidlc-domain-modeling` | 根拠を使う深掘り、質問の整理、用語と業務の構造化 |
| 同上 | `aidlc-research`、`aidlc-to-spec` | 調査、要件を仕様へまとめる |
| 同上 | `aidlc-tdd`、`aidlc-code-review` | テストを先に書く開発、変更のレビュー |
| [owainlewis/blueprint](https://github.com/owainlewis/blueprint/tree/2aeb882f06bc4b307015ea962b73aa60ac0c8ea7) | `aidlc-architecture` | 現在の構成や構成図を整理する |
| [mblode/agent-skills](https://github.com/mblode/agent-skills/tree/f05d2de8cbd88f11a4e3c99f2880f32491c61393) | `aidlc-planning` | 実装範囲・順序・検証を計画する |
| [obra/superpowers](https://github.com/obra/superpowers/tree/b36e0829c6d0140e93cfef2ca599b1b07d4a7797) | `aidlc-systematic-debugging`、`aidlc-verification-before-completion` | 原因を調べて修正する、完了前に証拠を確認する |
| [coji/natural-japanese](https://github.com/coji/natural-japanese/tree/9a78a42964096da509b8f3e011f0085a5f080151)（v1.5.0） | `natural-japanese-go` | 日本語文章の設計・執筆・推敲と、別CLIによる検査 |

この12個に、進行全体の`aidlc`、操作案内の`aidlc-cli`、知識操作の`aidlc-okf`を加えた**15個が標準配置のskill**です。原稿は[src/harness/codex/](../src/harness/codex/)にあり、利用先では`.agents/skills/`へ配置されます。表の12個はそれぞれ`references/source.md`と`LICENSE`を保持し、原典はいずれもMITです。[採用・翻案の計画](design/stage-skills-natural-japanese-go-plan.md)に固定元を記録しています。

`natural-japanese-go`のプログラムは、原典の通常14分類の検査をGoへ移植したものです。原典のPython/SudachiからKagome/UniDicへ解析器を変えているため、同じ文章でも結果が完全一致するとは限りません。skill自体は標準配置されますが、検査を実行する場合は別の`natural-japanese-go`実行ファイルが必要です。詳しい対応範囲は[日本語補助CLIの説明](../src/docs/natural-japanese-go.md)にあります。

### 任意で追加するGitHub skill

[aidlc-github](../.agents/skills/aidlc-github/SKILL.md)は、このリポジトリで作成した追加手順です。IntentやBoltという承認済みの作業のまとまりを、GitHub Issue・PRと対応付けます。外部原典・固定commitの記録はありません。

標準15skill、`aidlc`本体、通常のinstallerには含まれません。使うプロジェクトだけへフォルダをコピーし、Git・GitHub CLIの`gh`・GitHubの認証を用意します。導入方法は[任意skillの案内](../src/docs/optional-skills.md)にあります。製品本体の進捗管理をGitHubへ移す機能ではありません。

## 4. Goプログラムに入る依存ライブラリ

Goのライブラリ配布単位をmoduleと呼びます。[go.mod](../go.mod)は使うmoduleと版、[go.sum](../go.sum)は取得した内容の照合情報を記録します。ただし、module一覧に名前があるだけで、すべての実行ファイルにそのコードが入るとは限りません。

macOS・Linux・Windowsのamd64/arm64、合計6対象について、`go list -deps`で各CLIが実際に読み込むpackageから外部moduleを集計しました。どの対象でも以下の組合せは同じでした。これは依存経路の確認であり、6環境すべてで実行した試験ではありません。

| moduleと固定版 | 用途 | 入る実行ファイル | 原典のライセンス |
| --- | --- | --- | --- |
| [go.yaml.in/yaml/v3 v3.0.5](https://github.com/yaml/go-yaml/tree/v3.0.5) | YAML形式のfrontmatterや工程定義の読込み・書込み | `aidlc` | [ファイルによりMITとApache-2.0](https://github.com/yaml/go-yaml/blob/v3.0.5/LICENSE)。どちらか一方を自由選択するという記載ではない |
| [github.com/ikawaha/kagome/v2 v2.11.0](https://github.com/ikawaha/kagome/tree/v2.11.0) | 日本語を単語へ分け、品詞・原形・読みを得る形態素解析 | `natural-japanese-go` | [MIT](https://github.com/ikawaha/kagome/blob/v2.11.0/LICENSE) |
| [github.com/ikawaha/kagome-dict/uni v1.2.6](https://github.com/ikawaha/kagome-dict/tree/uni/v1.2.6/uni) | Kagomeから使うUniDic辞書を内蔵する | `natural-japanese-go` | moduleはMIT。辞書データは別途BSDのNOTICEを保持 |
| [github.com/ikawaha/kagome-dict v1.1.7](https://github.com/ikawaha/kagome-dict/tree/v1.1.7) | 解析器と辞書が使う共通処理。上記を介して使う間接依存 | `natural-japanese-go` | MIT |

つまり、`aidlc`本体の外部Go依存はYAMLライブラリ一つです。Kagomeと辞書は別の補助CLIに分けています。`aidlc-dist`の実行用packageには外部Go moduleへの依存がなく、Go標準ライブラリを使います。これらはインターネット上のAPIを毎回呼ぶ部品ではなく、build時に実行ファイルへ組み込む部品です。

UniDicの**module版`v1.2.6`と辞書データ版は別**です。内蔵データは`unidic-mecab-2.1.2`で、[UniDic ConsortiumのBSD通知](https://github.com/ikawaha/kagome-dict/blob/uni/v1.2.6/uni/NOTICE.txt)を保持します。解析器・辞書が内蔵されるため、補助CLIの利用時にPython、uv、別の辞書ファイルを取得する必要はありません。

`go list -m all`には、さらに`github.com/ikawaha/kagome-dict/ipa v1.2.6`と`golang.org/x/text v0.32.0`が現れます。前者はKagome、後者は辞書共通moduleの要求に含まれますが、今回確認した3つのCLIの実行用import経路にはありません。IPADICやx/textを、現在の実行ファイルに入る部品として数えないようにします。

Go標準ライブラリ・runtimeも実行ファイルを構成します。[Goのライセンス](https://go.dev/LICENSE)はBSD形式です。「外部moduleが少ない」ことと、「配布時に確認する許諾文が一つだけ」ということは同じではありません。

## 5. 利用環境・開発・配布で使うツール

| ツール・サービス | 必要な場面 | 版の扱い・現在の境界 |
| --- | --- | --- |
| [OpenAI Codex](https://github.com/openai/codex) | AI-DLCの手順を読む、担当AIを起動する、hookを呼ぶ | 別途導入するAI実行環境。本リポジトリの実hook検証基準はCodex CLI `0.153.4`。すべての版・OSでの動作保証を表さない |
| [Go](https://go.dev/) | ソースからのbuildとテスト | go.modは`1.26.0`。配布buildは`1.26.x`、品質CIは`1.26.x`と`stable`。調査時のローカルは`go1.26.4`。実際のbuild版は配布manifestに残す |
| [Git](https://git-scm.com/) | 本リポジトリの開発、変更管理、配布対象commit/tagの確定 | 製品の通常フォルダへの導入・進捗管理はGit不要。任意GitHub skillを使う場合は別途必要。CIのGit版は固定していない |
| [GitHub CLI（gh）](https://cli.github.com/) | Issue・PR操作、検証後のRelease下書き作成 | 製品本体への必須依存ではない。認証と操作権限が必要。workflowでCLI版は固定していない |
| [GitHub Actions](https://docs.github.com/en/actions) | 自動テスト、6対象のbuild、候補ファイルの受渡し | runnerは`ubuntu-latest`・`macos-latest`・`windows-latest`。OSイメージ自体は更新される |
| Bashなどのshell | 開発・CIのコマンド実行 | Distributionのrun処理はBash。Go版AI-DLCを利用するために本家のBun/TypeScript runtimeを導入する必要はない。AI実行環境自身の導入要件とは別 |

通常の利用者は、OS・CPUに合う`aidlc`を取得し、たとえば`aidlc install codex --project-dir /path/to/project`で配置します。Goが必要なのはソースから自分でbuildする場合です。参考リポジトリは調査用で、本製品の通常のbuildにも取得は不要です。[導入・配布手順](distribution.md)にコマンドの詳細があります。

### GitHub Actionsの固定部品

以下はCIでだけ動くGitHub公式Actionです。短い版名だけでなく、workflowの`uses`をcommit SHAに固定しています。Actionは利用者向けbinaryへ入りません。

| Action | 用途 | workflowで固定しているcommit |
| --- | --- | --- |
| [actions/checkout v6](https://github.com/actions/checkout/tree/d23441a48e516b6c34aea4fa41551a30e30af803) | 検証するソースを取得 | `d23441a48e516b6c34aea4fa41551a30e30af803` |
| [actions/setup-go v7](https://github.com/actions/setup-go/tree/b7ad1dad31e06c5925ef5d2fc7ad053ef454303e) | Go環境を準備 | `b7ad1dad31e06c5925ef5d2fc7ad053ef454303e` |
| [actions/upload-artifact v7.0.1](https://github.com/actions/upload-artifact/tree/043fb46d1a93c77aae656e7c1c64a875d1fc6a0a) | 検証済み候補を同じrunの後続処理へ渡す | `043fb46d1a93c77aae656e7c1c64a875d1fc6a0a` |
| [actions/download-artifact v8.0.1](https://github.com/actions/download-artifact/tree/3e5f45b2cfb9172054b4087a40e8e0b5a5461e7c) | 同じartifact IDの候補を取得 | `3e5f45b2cfb9172054b4087a40e8e0b5a5461e7c` |

定義の正本は[ci.yml](../.github/workflows/ci.yml)と[distribution.yml](../.github/workflows/distribution.yml)です。Actionsの候補は1日保持されます。手動の`create_draft`は既定falseで、明示した場合だけ成功後に下書きを作ります。自動で一般公開する処理はありません。

### このリポジトリを開発するAIのためのskill・MCP

リポジトリ直下の`.agents/skills/`には、製品開発用の手順もあります。[samber/cc-skills-golang](https://github.com/samber/cc-skills-golang)由来の`golang-*`群はMITで、Goの実装・調査・レビューを助けます。個別skillのmetadata版はありますが、取り込み元repo全体の固定commitは記録されていません。これらを利用者向け15skillと混同しないでください。

`aidlc-reference`・`okf-reference`は参考資料を読むため、`github-pr-workflow`は本リポジトリのIssue・PR運用のための手順です。利用先へ任意導入する`aidlc-github`とは目的が異なります。開発用のcustom agentは`.codex/`、製品として配置するagentの原稿は`src/harness/codex/agents/`にあります。

開発時にはSerenaでコードを参照し、Context7で外部仕様を調べます。これらのMCP接続は開発AIの環境にある補助機能で、`aidlc`の外部Go依存や利用者への必須インストールには含まれません。Go系skillに例示された追加ライブラリも、それだけでは本製品の採用済み依存ではありません。

## 6. ライセンスと公開前に残る整備

**参考元には利用・改変・再配布を認めるライセンスがありますが、本製品独自のコード・文書のライセンスはまだ決めていません。** また、許諾文をソース内に保存していることと、実行ファイルの受取人へ必要な表示を届けられることは別です。後者には公開前の整備が残っています。

ここでいう参考元には、設計の参照だけでなく、文章の翻案、日本語化、コードのGo移植も含めています。これらを一律に「引用だから表示不要」とは扱いません。以下は固定版の原文と現在の取り込み方を照合した一覧で、法令上の引用要件や製品全体の権利関係を審査したものではありません。

### ライセンス名と、配布するときの基本条件

LICENSEは利用・再配布の条件を記した許諾文です。NOTICEは原作者などの表示を伝える文書で、LICENSEとは役割が異なります。SPDX識別子はライセンスを短い名前で区別するための表記です。次の表は条件の要約なので、実際の配布には必要な原文を保持します。

| 名前・SPDX識別子 | 認められていることと、残す表示 |
| --- | --- |
| MIT No Attribution／`MIT-0` | 利用・改変・再配布・商用利用などを許可。通常のMITにある著作権・許諾文の保持条件を省いたもの。本家AI-DLCの保存版がこれに当たる。出典の記録は本プロジェクトの来歴管理として続ける。[MIT-0原文](https://spdx.org/licenses/MIT-0.html) |
| MIT／`MIT` | 利用・改変・再配布・商用利用などを許可。コピーや相当部分の再配布には、元の著作権表示と許諾文を含める。全文を保持すれば免責条項も一緒に伝えられる。[MIT原文](https://opensource.org/license/mit) |
| Apache License 2.0／`Apache-2.0` | 利用・改変・再配布を許可。受取人へライセンスの写しを渡し、変更したファイルには変更した旨を表示する。ソース配布では関係する原表示を保持し、原典にNOTICEがあれば該当する表示を読み取れる形で引き継ぐ。特許の許諾・終了条件もあり、商標の使用を一律に許すものではない。[原文第3・4・6条](https://www.apache.org/licenses/LICENSE-2.0) |
| BSD 3-Clause／`BSD-3-Clause` | ソース・binaryの改変、再配布を許可。ソースでは著作権・条件・免責を保持し、binaryでは同梱する文書などへ再掲する。原作者・団体名を無断で製品の推薦に使わない。今回該当するのはGoとUniDicデータ。[Go原文](https://go.dev/LICENSE)、[UniDic原文](https://raw.githubusercontent.com/ikawaha/kagome-dict/uni/v1.2.6/uni/NOTICE.txt) |

これらの原文には、利用しただけで本製品全体のソース公開を要求する条項はありません。ただし、独自部分のライセンスを選んでも、取り込んだ他者の許諾条件や著作権表示を置き換えられるわけではありません。原典URLをこの一覧へ載せることと、必要な許諾文を受取人へ渡すことも区別します。

### 設計・skillの原典ごとの確認結果

著作権欄は原典の名義・年の要約です。配布時にはこの要約で代用せず、原文を保持します。固定commitの完全な値と各skillの対応は第2・3節にあります。

| 原典・確認版 | ライセンスと原典の著作権表示 | このリポジトリでの取り込み方・表示の状態 |
| --- | --- | --- |
| AWS Labs AI-DLC Workflows `2.6.123` | [保存版LICENSE](aidlc-content/LICENSE)：MIT-0。Amazon.com, Inc. or its affiliates | 工程・配布などの設計参照。旧原稿の取得記録とLICENSEは`docs/aidlc-content/`に残る。元commitは未確認で、現行上流の版まで確認したとは扱わない |
| Open Knowledge Format v0.2、`ad30107…` | [固定版LICENSE.md](https://github.com/GoogleCloudPlatform/open-knowledge-format/blob/ad30107c31c06aec8a7d5636e0d1058118604e6f/LICENSE.md)：Apache-2.0 | 仕様と許諾文を[upstream資料](okf-analysis/upstream/)に保存。固定版のリポジトリにはNOTICEファイルがないことを確認。仕様の保存・参照と本製品独自実装を区別する |
| OKF Agent Memory v0.1.2、`d4c523…`／後発skill参考`a09e049…` | [初期参照版LICENSE](https://raw.githubusercontent.com/okf-memory/okf-agent-memory/d4c523ed5ce916fa207fe314851b98721421c891/LICENSE)・[skill参考版LICENSE](https://raw.githubusercontent.com/okf-memory/okf-agent-memory/a09e04918aa84d275b784374b5236d9eeac56c9e/LICENSE)：MIT。2026 sknr and the OKF Memory Contributors | 初期設計の比較と`aidlc-okf`の手順の翻案。元CLIは同梱しない。RAMに出典はあるが、製品の`aidlc-okf/`には上流LICENSE・出典文書がなく、配布表示の整備が残る |
| mattpocock/skills `3cca18…` | [固定版LICENSE](https://raw.githubusercontent.com/mattpocock/skills/3cca18b368ae95cdbdebbff572ccafa662551015/LICENSE)：MIT。2026 Matt Pocock | 7skillを翻案。各skillの`LICENSE`に原典表示、`references/source.md`に出典と翻案内容を保持 |
| owainlewis/blueprint `2aeb882…` | [固定版LICENSE](https://raw.githubusercontent.com/owainlewis/blueprint/2aeb882f06bc4b307015ea962b73aa60ac0c8ea7/LICENSE)：MIT。2026 Owain Lewis | `aidlc-architecture`を翻案。原典LICENSEと出典を保持 |
| mblode/agent-skills `f05d2de…` | [固定版LICENSE.md](https://raw.githubusercontent.com/mblode/agent-skills/f05d2de8cbd88f11a4e3c99f2880f32491c61393/LICENSE.md)：MIT。2026 Matthew Blode | `aidlc-planning`を翻案。原典LICENSEと出典を保持 |
| obra/superpowers `b36e082…` | [固定版LICENSE](https://raw.githubusercontent.com/obra/superpowers/b36e0829c6d0140e93cfef2ca599b1b07d4a7797/LICENSE)：MIT。2025 Jesse Vincent | デバッグ・完了前検証の2skillを翻案。原典LICENSEと出典を保持 |
| coji/natural-japanese v1.5.0、`9a78a42…` | [固定版LICENSE](https://raw.githubusercontent.com/coji/natural-japanese/9a78a42964096da509b8f3e011f0085a5f080151/LICENSE)：MIT。2026 coji | skillの翻案と検査のGo移植。skill側と[移植コード側](../src/internal/naturaljapanese/source.md)に出典・LICENSEを保持。補助CLI用の許諾文にも原典表示を収録 |
| samber/cc-skills-golang（開発用） | [調査時のmainのLICENSE](https://github.com/samber/cc-skills-golang/blob/main/LICENSE)：MIT。2026 Samuel Berthe | `.agents/skills/golang-*`で使用し、製品には同梱しない。取り込み時のrepo commitは未記録。現行上流のLICENSEを、取得当時の原文を検証できた証拠にはしない |

MITの原典表示・出典を各フォルダに保持しているのは、第3節の**12skill**です。標準15skillすべてについて整備済みという意味ではありません。独自原稿の`aidlc`・`aidlc-cli`、任意の`aidlc-github`は製品側のライセンスを別に定める必要があります。

### 実行ファイルへ入るコード・辞書の確認結果

| 部品・確認版 | ライセンスと原典の表示 | 配布で扱う原文・注意点 |
| --- | --- | --- |
| go-yaml v3.0.5 | [LICENSE](https://github.com/yaml/go-yaml/blob/v3.0.5/LICENSE)：ファイル別にMITとApache-2.0。MIT側は2006–2010／2006–2011 Kirill Simonov、Apache側は2011–2019 Canonical Ltd | **[NOTICE](https://raw.githubusercontent.com/yaml/go-yaml/v3.0.5/NOTICE)も存在**し、2011–2016 Canonical Ltdの表示がある。moduleのLICENSE、Apache-2.0全文、NOTICEを配布物で引き継ぐ整備が残る |
| Kagome v2.11.0 | [LICENSE](https://github.com/ikawaha/kagome/blob/v2.11.0/LICENSE)：MIT。2020 ikawaha | 日本語補助CLIの[licenses/kagome.txt](../src/harness/codex/stage-skills/natural-japanese-go/licenses/kagome.txt)に原文を保持 |
| kagome-dict v1.1.7／uni module v1.2.6 | [共通moduleのLICENSE](https://github.com/ikawaha/kagome-dict/blob/v1.1.7/LICENSE)・[uniのLICENSE](https://github.com/ikawaha/kagome-dict/blob/uni/v1.2.6/uni/LICENSE)：ともにMIT。前者は2021 ikawaha、後者は2020 ikawaha | 補助CLIの`licenses/kagome-dict.txt`と`licenses/uni.txt`に原文を保持。辞書データの条件は次行 |
| UniDicデータ `unidic-mecab-2.1.2` | [NOTICE.txt](https://raw.githubusercontent.com/ikawaha/kagome-dict/uni/v1.2.6/uni/NOTICE.txt)：BSD-3-Clause。2011–2013 The UniDic Consortium | 補助CLIの[licenses/UniDic-NOTICE.txt](../src/harness/codex/stage-skills/natural-japanese-go/licenses/UniDic-NOTICE.txt)に著作権・条件・免責を保持 |
| Goのruntime・標準ライブラリ、調査時`go1.26.4` | [Go LICENSE](https://github.com/golang/go/blob/go1.26.4/LICENSE)：BSD-3-Clause。2009 The Go Authors。別に[PATENTS](https://raw.githubusercontent.com/golang/go/go1.26.4/PATENTS)もある | 3つのCLIに関係する。実際に配布するbuild版の許諾文と、含まれるコードの追加表示を確認する。独立したGoの許諾文は現archiveに未同梱 |

YAMLのMIT対象は`apic.go`、`emitterc.go`、`parserc.go`、`readerc.go`、`scannerc.go`、`writerc.go`、`yamlh.go`、`yamlprivateh.go`で、LICENSEは残りのファイルをApache-2.0としています。**MITかApacheを自由に選べるという意味ではありません。** また、OKF仕様にはNOTICEがなく、YAMLにはあるため、Apache-2.0という名前だけで表示内容を同じにしないようにします。

GoのPATENTSは、Googleが対象となるGo実装の特許利用を追加で許可する文書です。対象や終了条件があり、あらゆる改変に及ぶ無条件の保証ではありません。公開準備ではLICENSEと一緒に確認・参照できるようにする案ですが、BSDの条文がPATENTSの同梱を直接要求しているという説明はしません。Go全ファイルの追加表示を網羅した監査も、この一覧では未実施です。

### 受取人へ表示を届けるための残対応

ソースをcloneした人は`docs/`や`src/`の許諾文を読めます。一方、圧縮ファイルだけを取得した人に届く内容は梱包処理で決まります。現行の`aidlc` archive内の独立ファイルはbinaryだけで、12skillのLICENSEはbinaryに埋め込まれ、installerによる配置後に読める構成です。原典の記録があるだけで、配布全体の表示が完了したとは扱いません。

| 残対応 | 現在の状態と、公開準備で行うこと |
| --- | --- |
| 本製品独自のライセンス | rootの製品用LICENSEはない。ユーザーと対象範囲・ライセンスを決める。他者の原典表示は維持する |
| `aidlc-okf`の表示 | 上流MIT原文・著作権・固定commit・翻案内容を配布資材へ揃え、installerの配置先からも読めるようにする案。今回は文書への整理まで |
| `aidlc` archiveの許諾文 | YAMLのLICENSE・NOTICE・Apache-2.0全文、GoのLICENSE、同梱する翻案skillの原典表示を受取人が読める構成にする。archive内に`LICENSES/`などを置く方法を公開準備で具体化する |
| 日本語補助CLIの許諾文 | 専用archiveにはREADMEと`LICENSES/`を追加する実装があり、coji、Kagome、辞書共通、uni、UniDic通知の5文書が入る。Goを含む表示と実際の公開方法の確認が残る |
| 参照版不明の資料・開発用skill | 本家AI-DLC snapshotと開発用Go skillの未記録commitを明示して扱う。再取得や同梱範囲の拡張時は、取り込む版の出典・許諾を記録する |
| 公開候補そのものの照合 | 選んだcommit・Go版から作ったarchiveと、配置後の資材に、必要な許諾文・著作権・NOTICEが入ることを確認する。製品LICENSEや参照URL一つでまとめて代用しない |

第5節のCodex、Git、gh、Actions、Bash、MCPは別途使う開発・実行ツールで、上表のライブラリのように本製品へ同梱していません。この節ではそれらのツール自体の全依存・利用規約の監査までは行っていません。将来ツール本体やそのコードをコピーして配布する場合は、その配布内容に応じた確認が必要です。

現在のRelease機構が添付するのは、**aidlcの6種類の圧縮ファイルと`manifest.json`・`SHA256SUMS`の計8ファイル**です。`natural-japanese-go`の専用archiveは生成・検証できますが、このRelease添付には入っていません。梱包内容の根拠は[archive.go](../src/cmd/aidlc-dist/archive.go)です。上の整備案は公開前の残対応であり、この文書の追記で製品ライセンスを採用したり、許諾文の同梱処理を変更したりはしていません。

## 7. 文書確認から0.1.0公開まで

1. **ユーザーがこの文書を確認する。** 参考元・採用範囲・必要な部品・残対応に認識違いがないかを確認します。
2. 製品ライセンス、許諾文の配布方法、補助CLIの公開範囲を決め、必要な整備を行います。この文書の作成ではそれらを決定・実装していません。
3. 公開するソースのcommitと`0.1.0`のtagを確定し、その内容から候補を作って検証します。現時点ではtagを作成していません。通常の開発buildは引き続き`dev`表記で、ここで版を決めただけでは書き換わりません。
4. 検証済みの候補・許諾文・導入手順を確認してReleaseの下書きを作り、内容確認後に一般公開します。文書のPRをmergeする操作と、製品を公開する操作は別です。

PR #187では、macOS・Linux・Windowsで同じ配布候補集合を使う導入検査が成功しています。ただし、これを6種類のCPU構成すべてでの実行確認や、すべてのOSでの実Codex hook検証へ読み替えません。Claude Codeの検証保留も継続します。

更新時は、変更した参考元のcommit、`go.mod`・`go.sum`、各CLIの`go list -deps`、skillの`references/source.md`とLICENSE、workflowの固定SHA、実archiveの内容を照合してこの一枚を更新します。`go list -m all`の一覧だけから配布内容を判断しないようにしてください。
