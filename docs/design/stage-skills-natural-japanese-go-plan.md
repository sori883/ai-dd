# 工程skillの導入とnatural-japanese-goの実装計画

日付: 2026-09-12。実装許可: 第一候補の採用に続く「kagomeで実装をしてください。まずは他のskillsを導入して、netural-japanese-goを作ってください」という直接依頼。
名称は参照元の綴りに合わせて `natural-japanese-go` とする。
対応Issue: [#179 工程別skillとKagome版の日本語文章チェックを導入する](https://github.com/sori883/ai-dd/issues/179)。

## 背景と利用者が得る結果

AI-DLCには工程・担当・承認・OKF文書保存の基盤がある。今回、ユーザーが選んだ外部skillの作業方法を、
この基盤に合わせて配布する。先に日本語lint以外の11 skillを導入し、その後にPythonを使わず日本語の文章を
検査できる別CLIと、そのCLIを使うskillを作る。CLIはコマンドとして実行するプログラムで、AIは検出結果を
読んで本文を推敲する。文書が正しいか、工程を進めてよいかをlintだけで決めるものではない。

開始時のmainは `c43a1ba1b532f6f635eb49bff3bd5ca4d7a497cb`。直前のPR #178でaidlc-okfが導入済み。
Open Issue・PRは各0件で、同じ日本語Go案件はない。作業場所は `/Users/const/sori883/ai-dd-naming`、
branchは `codex/stage-skills-natural-japanese-go`。元checkoutの未commit資材は変更しない。
この作業場所にある前回の比較・選定RAMは保持し、今回の判断を新しいRAMから記録する。

## 実施する内容と維持する境界

1. 下表の11 skillを、日本語の製品手順として導入する。配布先は `.agents/skills/<配布名>/`。
2. 工程本文、既存担当の指示、aidlc-cliから必要時の読込みへ接続する。全skillの一括読込みは要求しない。
3. 配置済みの既知Markdownを既存hookの読取り対象へ追加する。
4. `natural-japanese-go` を別の実行ファイルとして作り、通常の14検査、JSON出力、文章種別、前回結果比較を提供する。
5. Go版を使うskillと、独立した配布archive、利用手順を追加する。

| 配布名 | 元の作業方法と使う場所 |
| --- | --- |
| aidlc-grill-with-docs | discoveryの深掘り。下のgrilling/domain-modelingを実際に読んで利用する |
| aidlc-grilling | 事実を調査し、相互に依存しない質問をまとめ、回答が必要な後続質問を次へ送る |
| aidlc-domain-modeling | 用語・具体例・重要判断を整理し、OKF本文案へ反映する |
| aidlc-research | 公式文書・実装・仕様の調査。根拠と不確かさをresearcherが返す |
| aidlc-architecture | 共有current-analysis/architecture用の、実装済み構成の説明 |
| aidlc-to-spec | 深掘りで確定した要件をrequirementsへまとめる |
| aidlc-planning | planningでメインAIが実装方法・Unit・検証を具体化する |
| aidlc-tdd | tddでworkerが失敗テストから実装する |
| aidlc-systematic-debugging | 再現・比較・仮説検証を行い、修正はTDDへ接続する |
| aidlc-code-review | 既存reviewerが要求と規約を分けて独立確認する |
| aidlc-verification-before-completion | 完了主張に対応する実行結果と最新の証拠を確認する |

各原稿は、原典の有効な作業方法を保ち、Codexで存在しないSkill tool等への依存を除く。
質問案・本文案は子からメインAIへ返し、共有文書の保存はメインAIだけがaidlc memoryで行う。
原典のCONTEXT.md、docs/adr、Issue tracker、Git履歴、子の独自起動を別の正本・権限にしない。
stage-plannerは実行工程の採否・順序を提案する担当のままで、実装計画を作る担当へ変更しない。

initializationの必須3 skillとhooks、工程frontmatterの許可担当・Sensor・入出力、人間承認、state、同じrootの
単独writerを維持する。skill本文の読込みが新しい工程合格条件にはならない。
hookは最大3ファイルの単純なcatだけを既存どおり扱い、配布原稿由来の既知Markdown pathを共用の一覧から判定する。
任意path、glob、親ディレクトリ参照、symlink逃避、redirect、複合command、スクリプト実行は追加許可しない。
未選択・Rule未読/変更・実行中tool残存も従来どおり拒否する。

日本語CLIの実行は開始済みの工程で既存のhook検査に従う。承認待ちの実行例外は追加しない。
新skillは絶対binary/rootを埋め込まず、既存skillの相対参照とPATH上のnatural-japanese-goを使う。
現在のrelocate対象4ファイルは増やさない。

## 固定する原典と依存

| 対象 | 参照版 |
| --- | --- |
| mattpocock/skills | `3cca18b368ae95cdbdebbff572ccafa662551015` |
| owainlewis/blueprint | `2aeb882f06bc4b307015ea962b73aa60ac0c8ea7` |
| mblode/agent-skills | `f05d2de8cbd88f11a4e3c99f2880f32491c61393` |
| obra/superpowers | `b36e0829c6d0140e93cfef2ca599b1b07d4a7797` |
| coji/natural-japanese | v1.5.0、`9a78a42964096da509b8f3e011f0085a5f080151` |
| github.com/ikawaha/kagome/v2 | v2.11.0 |
| github.com/ikawaha/kagome-dict/uni | v1.2.6、UniDic一種類を補助CLIへ埋込み |

Kagomeと必要な辞書の追加はユーザーが指定した。Go標準ライブラリには日本語の単語分割・品詞・原形・読みを
得る解析器と辞書がないため利用する。Kagomeの必要な推移依存であるkagome-dict v1.1.7、golang.org/x/text
v0.32.0等もgo.mod/go.sumへ固定する。無関係なCLI frameworkや解析libraryは追加しない。
UniDicは品詞・原形・読みを一つの辞書で扱うため選び、実行時の辞書選択やダウンロードを要求しない。
辞書dataは確認版で約45.5MBあり、補助CLIの容量は増える。本体aidlcのimport graphへは含めない。

固定原典のSKILL.mdと必要な文書だけを参照し、未選択skillへのリンクを残さない。
配布する各skillへ原典URL・commit・変更点とMITの帰属/許諾を保持する。原典から参照する文書を残す場合は
同じskillのreferencesへ同梱し、依存する新しい工程やtoolを暗黙に増やさない。
natural-japaneseの文章指針はGo版向けに調整し、Pythonのoutline/termsを必須にせず、本文構造と用語はAIが確認する。
通常lintと矛盾する古い手動指針はそのまま転記せず、固定lint.pyの判定と人間の文章評価を区別する。

## natural-japanese-goの契約

```text
natural-japanese-go [--json] [--genre essay|tech|business] [--baseline PREVIOUS.json] FILE
natural-japanese-go --list-rules [--json]
natural-japanese-go --help
natural-japanese-go --version
```

FILEはUTF-8の通常ファイル、`-`なら標準入力。flagはFILEの前後で使える。
本文を読み、JSONまたは日本語の検出結果を標準出力へ返す。入力ファイルやOKF metadataを自動編集しない。
引数不正はexit 2、入力・出力・解析・baselineの読込み失敗はexit 1、正常な検査は指摘があってもexit 0。
短い文章で統計検査の最低量に届かない場合は無理に判定せず、検出なしを内容の正しさの保証と説明しない。
help/list-rulesに許可するgenre、severity、検査カテゴリ、検査対象外、終了コードを記述する。

JSONはschema_version、engine、dictionary、file、stats、findingsを持つ。engineはKagome、辞書の固定版を
明示する。findingはline、category、excerpt、severity、detail、必要時related_linesを持つ。
baseline指定時は同じGo版の解析方式・辞書・schemaのJSONだけを比較し、new/persisting/resolvedを返す。
不正・異なる解析方式のbaselineを通常成功として無視せず、理由を返す。
Python版の出力をそのまま比較できる互換CLIとは説明しない。

通常の検査対象は次の14カテゴリ。規則・しきい値は固定lint.pyを正本とする。

| 検査 | 基本条件 |
| --- | --- |
| forbidden_phrase | 固定版の定型句catalog。行と句ごとに最初の一致を報告 |
| translationese | 固定版の翻訳調の表層pattern |
| antithesis_repetition | 対比3件以上。文数比率でseverityを決める |
| low_sentence_variance | 5文以上、Unicode文字数のCVが0.25未満 |
| english_syntax_inanimate_subject | 無生物主語と特定動詞の表層pattern |
| nominal_ending | 5文以上・既定2000字以上で名詞終止が0件 |
| uniform_paragraph_structure | 4段落以上、文数のCVが0.15未満 |
| translationese_morph | 品詞を使う「こと＋が/は＋でき…」 |
| inanimate_subject_morph | 主語と辞書形の動詞catalogによる判定 |
| low_burstiness | 6文以上。読みから算出したモーラ近似長でBが-0.24未満 |
| repeated_sentence_lead | 文頭2形態素の反復が既定6回以上 |
| low_lexical_diversity_ttr | 4000字以上・内容語30以上、原形TTRが0.45未満 |
| low_lexical_diversity_mtld | 同じ対象語の前後方向平均MTLDが40未満 |
| low_specificity | 80字以上の段落・内容語15以上。固有名詞/数値/例示/抽象名詞の固定式で-0.15未満 |

genre未指定の既定値を維持する。essayは名詞終止の最低1500字・文頭反復5回、tech/businessは3000字・7回。
techの対比critical境界は4.5%。品詞はKagome/UniDicの値を明示的に対応付ける。
frontmatter、見出し、箇条書き、引用、表、フェンスコード、HTMLコメント、inline codeとURLを原典どおり
解析対象から除外し、元の行番号を保つ。原典の通常lintはインデントcodeをマスクしない点も対応表へ記す。

原典と異なる解析器・辞書を使う指定により、同じ判定式でも結果が一致しない場合がある。
全14カテゴリを実装したことと、原典の全入力での同一結果は区別する。公開fixtureの期待件数だけに合わせて
規則を調整せず、カテゴリ別の検出例・正常例・境界例とKagome固有の解析例で検証する。
実験的な検査、reading-load、機械学習を使うsemanticは通常lintとは別で、対応外のflagを明確に拒否する。

## ファイルと単独writerの所有範囲

| 対象 | 内容 |
| --- | --- |
| src/harness/codex/stage-skills/〔新規〕 | 11工程skillとnatural-japanese-go、参照文書・帰属 |
| src/harness/codex/assets.go | embedと配布Markdown一覧 |
| src/internal/install/install.go、stage_skills_test.go〔新規〕 | 新規配置、リンク、衝突/symlink/既存資材保全 |
| src/internal/app/hook.go、stage_skills_test.go〔新規〕 | 既知Markdown読取りと拒否境界 |
| src/harness/codex/aidlc-cli/SKILL.md、agents/aidlc-{requirements,researcher,worker,reviewer}.toml | 必要時のskill読込みと返却・保存の案内 |
| src/core/workflow/stages/{discovery,architecture-analysis,planning,tdd,integration}.md | 本文への適用位置の追加。frontmatterは維持 |
| src/cmd/natural-japanese-go/{main.go,main_test.go,integration_test.go}〔新規〕 | 別CLIの引数・入出力・実binary確認 |
| src/internal/naturaljapanese/〔新規〕 | analyzer.go、text.go、surface.go、structure.go、morph.go、rhythm.go、lexical.go、baseline.go、rules.goと対応test/fixture/帰属 |
| go.mod、go.sum | 許可された解析器と辞書の固定 |
| src/cmd/aidlc-dist/{main.go,archive.go,main_test.go,archive_test.go,distribution_integration_test.go}および関連test | 別製品の梱包と照合 |
| src/cmd/aidlc/stage_skills_live_integration_test.go〔新規〕 | 実読込み証拠の限定fixtureと判定test |
| .github/workflows/{ci,distribution}.yml | 新CLIのbuild、native実行、配布検査 |
| src/docs/natural-japanese-go.md〔新規〕、src/docs/user-guide.md、docs/distribution.md | 使用方法、手順接続、対応範囲と更新復旧 |
| この計画、docs/ram/、索引 | 親がwriter開始前と返却後に記録。writer稼働中は同時編集しない |

実装中に必要な隣接testの期待値・fixture修復も、既存契約を保つ範囲で同じwriterが行う。
実装担当以外は読み取り専用。原稿や実装へ追加のwriterは起動しない。

## 順序付きTDDと検証分担

1 Issue/PR、work_unit_id `stage-skills-natural-japanese-go`。以下を一人が順にtest-firstで実装する。
新しい型・関数の署名と空返値のみ、実行可能な失敗testへ到達するためのscaffoldとして許可する。
compile failureはREDに数えない。文書の単純な追加に人工REDは作らない。

| 順序 | 動作とtargeted command |
| --- | --- |
| 1 | 11 skill配置・必要参照・既存保全: `go test -count=1 ./src/internal/install -run '^TestStageSkills'` |
| 2 | 既知Markdown読取り・選択/Rule/tool/不正path拒否: `go test -count=1 ./src/internal/app -run '^TestStageSkillsRead'` |
| 3 | CLI help/version/list/input/error: `go test -count=1 ./src/cmd/natural-japanese-go -run '^TestCommand'` |
| 4 | 前処理と元行番号: `go test -count=1 ./src/internal/naturaljapanese -run '^TestText'` |
| 5 | 表層の4カテゴリ: `go test -count=1 ./src/internal/naturaljapanese -run '^TestSurface'` |
| 6 | 文長・段落の2カテゴリ: `go test -count=1 ./src/internal/naturaljapanese -run '^TestStructure'` |
| 7 | 実Kagome辞書で品詞/原形/読み: `go test -count=1 ./src/internal/naturaljapanese -run '^TestAnalyzer'` |
| 8 | 名詞終止と形態素の3カテゴリ: `go test -count=1 ./src/internal/naturaljapanese -run '^TestMorph'` |
| 9 | 読みのリズムと文頭反復: `go test -count=1 ./src/internal/naturaljapanese -run '^TestRhythm'` |
| 10 | 語彙2カテゴリと具体性: `go test -count=1 ./src/internal/naturaljapanese -run '^TestLexical'` |
| 11 | genre、統合JSON、baseline: `go test -count=1 ./src/internal/naturaljapanese -run '^Test(Genre|Report|Baseline)'` と `go test -count=1 ./src/cmd/natural-japanese-go -run '^TestCommand'` |
| 12 | Go版skill・公開fixture・読込み証拠: `go test -count=1 ./src/internal/install -run '^TestNaturalJapaneseSkill'`、`go test -count=1 ./src/internal/naturaljapanese -run '^TestFixtures'`、`go test -tags=integration -count=1 ./src/cmd/aidlc -run '^TestStageSkillsEvidence$'` |
| 13 | 2製品の別archive・既定aidlc維持・不正入力: `go test -count=1 ./src/cmd/aidlc-dist -run '^Test(Product|Archive)'` |

loopでは各項目の最小testと末尾の対象package確認だけを行う。全package/race/vet/cross build/配布E2Eは実行しない。
親は返却後に全差分とtargeted command群を一度確認する。独立reviewは仕様・境界・移植規則・テスト証拠を確認し、
findingの再現に必要な検証だけを行う。差分が安定した後、親がread-only finalを一度開始する。

finalは全package test/race/vet、tidy差分・module照合・gofmt確認・diff確認、現行workspace/OKF integration、
FlowJourney/GitIndependentJourneyを含む。両CLIをCGOなしで3OS×2architectureへbuildし、別archiveを照合する。
Go版をPython/uv/外部辞書を解決できない環境でも実行し、実辞書の検査・help・JSON・baseline・入力保全を確認する。
aidlcの `go list -deps` にKagome/辞書/naturaljapaneseが含まれないことを確認する。
skill validatorとTOML読込み、会話を知らない独立担当による限定的なskill適用も確認する。

固定Codex CLI 0.153.4で新skillの実読込みとGo CLI実行を限定fixtureで確認する。既存の明示的な試験用trustを
用いるhelper方式と、通常利用者が行うhook trustは区別する。試験用trustでの確認を通常trustの実測とは報告しない。
実機のread/execute・終了結果・Pre/Post対応を採取し、単なる配置やAIの成功申告だけを証拠にしない。
通常trustを含む全hook経路・全OSの実Codex再検証を今回の結果へ混ぜない。
現在headで起動するGitHub checksの成功後、既定運用でPRをmergeし、main反映とIssue closeを確認する。

## 配布・復旧・準拠

`aidlc-dist --product aidlc|natural-japanese-go` を追加し、既定はaidlc。製品ごとに別の新規出力directoryと
archiveを作る。manifest schema 1とaidlc側の既存出力を保つ。日本語CLI側のarchiveには利用文書とMIT/BSD等の
必要な帰属を同梱し、SHA256SUMSで照合できるようにする。自動アップロードや正式release公開は行わない。

fresh installは事前の全配置先検査を保ち、既存fileがあれば書込み前に拒否する。
既設利用先へ自動適用せず、隔離stagingで比較し、変更を保全して更新する手順を記述する。
工程本文はdefinition hashへ影響するため、新規配置と新規Intentで利用する。旧Intentのhashを上書きして移行しない。
復旧時は前のbinaryと対応する配布資材へ戻す。Rule/Knowledge/state/runtimeを初期化しない。

本家AI-DLC固定参照の状態遷移・承認・担当制限へ新しい意図的差分は加えない。外部skillの保存先・起動方法を
この製品の既に承認済みのOKF/担当方式へ合わせる。natural-japanese原典からは解析器/辞書と実行環境が変わり、
通常14カテゴリは維持するが結果の完全一致は約束しない。理由はユーザーがPython不要のKagome版を指定したため。
辞書と解析器の版、検査ごとの対応、確認した差を利用文書と帰属資料へ記録する。
