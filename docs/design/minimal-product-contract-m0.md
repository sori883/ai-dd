# M0 最小契約案とM1実装計画

ステージ移行の最新要件: 各境界にSensorとレビューを設け、合格後にstateを進める。
[ステージ間の検査・レビュー方針](../ram/decisions/2026-09-08-stage-sensor-review-gates.md)を参照する。

記録の最新分担: Knowledgeは現行の「何を・どう」、ADRは設計の「なぜ」、stateは進捗を担う。
ADRの配置先は `aidlc/spaces/<space>/knowledge/ADR/`。[責任分担・配置の合意](../ram/decisions/2026-09-08-knowledge-what-how-adr-why.md)を優先する。

最新方針: [4段階のフローとADR・進捗state](../ram/decisions/2026-09-08-four-step-flow-adr-and-progress-state.md)を優先する。
ADRはアーキテクチャレベルの設計変更意図、進捗はIntent・Unitのstateで管理する。
目的整理・深掘りの内容はSpaceのOKF Knowledgeへ保存する。[Knowledge保存の合意](../ram/decisions/2026-09-08-discovery-content-in-knowledge.md)を参照する。
以下の作業記録必須化はM0/M1当時の契約であり、今後の要件としてそのまま適用しない。

2026-09-08 名称更新: 記録文書は **ADR** に統一し、1 Intentにつき1 ADRへ各工程の判断・結果を記録する。
[ADR名称とUnit実行stateの最新案](../ram/decisions/2026-09-08-adr-name-and-unit-runtime-state.md)を優先する。
以下はM0/M1契約の履歴として旧称と実装識別子を保持する。製品CLI等への改名反映は実装計画で扱う。

2026-09-08: 名前操作・標準metadata・intent_id検索の確認後、ユーザーが「はい、では進めてください」と直接承認。
実装許可・具体的な公開文法・保存復旧・対象file・検証は[確定M1計画](../ram/decisions/2026-09-08-m1-implementation-plan.md)を優先する。
以下に残る「提案」「未承認」「今回外部操作を行わない」は検討時の状態であり、現在のM1許可は後続計画で更新済み。

2026-09-08着手確認: 「不明点がなければ実装」の依頼を受領。Intent操作は名前による作成・選択とし、
内部では固定IDで同じKDRを特定する。IDと名前はfrontmatterに保存することで回答済み。
[名前操作・frontmatterの合意](../ram/decisions/2026-09-08-intent-name-frontmatter-accepted.md)を優先し、既回答の3点も再確認しない。

2026-09-08確定: OKF処理と初期資産をGoのaidlc単一バイナリに内包し、導入時に本家の配置先へ展開する。
Spaceの操作は本家準拠、生成内容は新OKF構成とする。組織Ruleはdefault Spaceから `rules/rule.md` を作成時にコピーし、以後は各Spaceで独立する。
[3点への回答とrule.md配置の合意](../ram/decisions/2026-09-08-single-binary-space-rule-accepted.md)を優先する。

2026-09-07。**Proposed（一部の配置・形式方針は下記のユーザー指示で確定）。M1の実装は未承認。**

後続指示により、Spaceの `aidlc/spaces/<space>/knowledge/` をOKF bundleとし、Knowledge・KDR・Ruleをすべて配下のOKF文書にする。旧案の `docs/kdr/` と `docs/knowledge/` の分離配置、KDR専用front matterは撤回する。詳細は[配置・形式の合意](../ram/decisions/2026-09-07-space-knowledge-okf-unification.md)。

## 背景・現在地・得られる結果

AI-DLCを、33 Stageの進行を管理する製品から、共有記憶とIntentごとの記録を支える製品へ絞る。
Intentは一つの目的を持つ作業単位、KDRはその目的・判断・結果・検証・残件を書くMarkdownである。
例えば「検索結果をタグで絞る」を一つのIntentとし、質問、試作、失敗するテスト、実装、修正、翌日の再開を同じKDRへ記録する。
KDR・rules・設計・knowledgeはSpaceごとのOKF Agent Memoryのbundle（関連する知識ファイルのまとまり）へ置く。

今回GitHubを再確認した結果、`sori883/ai-dd` のdefault branchは `main`、リモートmainとローカルHEADは
`fc8bd7ea633f0c1ad8cc605233572a6106b3b911`。Open Issue・PRは各0件。
[PR #127](https://github.com/sori883/ai-dd/pull/127)はMERGED、[Issue #126](https://github.com/sori883/ai-dd/issues/126)はCLOSED。
PRの対象files、commits、本文、checksも確認し、取得したchecksはCOMPLETED/SUCCESSだった。
独立レビューはPR本文の実施報告であり、GitHubのreview objectによる承認と同一視しない。
mainはそのmerge commit自身なので、その後のmain上の置換・revertはない。

公開CLIにはIntent作成がなく、現行hook設定はUserPromptSubmitだけである。
既存のworkspace、Go CLI、保存処理、CIは参考にできるが、新KDR・OKF Agent Memory接続の動作証拠はまだない。
既存の設計2件・RAM5件とRAM索引の未コミット差分を読み、保持した。この文書はそれらを上書きしない後続案である。
今回Goテストやlive E2Eは実行していない。

承認済みなのは、1 Intentにつき1 KDR、通常のAI操作での記録必須化、工程state・全操作auditを要求しないこと、既存Intent・stateの移行を不要とする境界である。
既存ファイルの削除許可ではない。旧ApproveGateの部分完了問題も未修正のままで、新方式の先行作業にしない。

## 1. Intentとファイルを一対一にする

Space作成もAI-DLC準拠とする。固定版 `2.6.123` の `aidlc space create <name> [--project-dir <path>]` を基準に、名前・予約名・重複・配置の契約を維持する。
作成と選択は別で、Space作成だけでactive Space、Intent、session bindingを変更したりKDRを作ったりしない。
生成内容は承認済みの新OKF構成へ変更し、Space作成時にbundleと初期Ruleを用意する。旧memory/intents/codekbを新Spaceのために生成する方式は採用しない。
組織Ruleは `aidlc/spaces/default/knowledge/rules/rule.md` から新Spaceの同じ相対pathへコピーする。旧org.mdは新規生成しない。作成後の自動同期はしない。
根拠: [Space作成準拠の合意](../ram/decisions/2026-09-07-minimal-space-creation-conformance.md)。

下記のSpace内配置と下位分類はユーザー採用済み。配布は固定AI-DLCの仕組みに準拠するとの追加指示を受けた。
[配置採用と配布方針のRAM](../ram/decisions/2026-09-07-minimal-layout-distribution-conformance.md)を参照する。

利用プロジェクトのGit worktreeのルートを基準に、次の配置を提案する。これはこの製品を開発する側の `docs/ram/` とは別である。

```text
aidlc/spaces/main/knowledge/          # このSpaceのOKF bundle root
├── index.md                         # 全体の案内
├── log.md                           # 知識文書の変更履歴
├── knowledge/
│   └── search-notes.md               # 一般知識の例
├── design/
│   └── search.md                     # 共有設計の例
├── kdr/
│   └── 9c2f0b1d7a684e55a19d0680f731db26.md
└── rules/
    ├── entry.md                      # 必須Ruleを明示する入口もOKF文書
    ├── rule.md                       # defaultからコピーする組織Rule（OKF）
    └── working-agreement.md          # 製品の作業手順Ruleの例
```

- IDはCLIが `crypto/rand` で生成する128 bitの小文字16進数32桁。タイトルを変えてもID・pathは変えない。
- Intent IDはKDRのfrontmatterの `intent_id` に保存し、ファイル名のIDと一致させる。OKF Concept IDは `kdr/<id>` とする。名前はOKF標準の `title` を使う。`intent_id` はユーザー指定を具体化した製品拡張であり、OKF標準項目ではない。SpaceとConcept IDの組が保存先を一意にする。別のIntent registryや工程stateは作らない。
- 同じ目的への修正・再開は同じID。独立した別の目的なら新規ID。意味が曖昧ならAIがユーザーへ確認し、勝手に分割・統合しない。
- 全workspace共通の「現在Intent」は作らない。利用者は名前で作成・選択し、AIとCLIが内部で固定IDへ解決してCodex sessionに結び付ける。同名候補は一覧とIDを返し、勝手に一件へ決めない。書込み処理は解決済みIDを使う。sessionは会話の識別子で、Intentそのものではなく、KDRのfrontmatterへ固定しない。
- M1はGit管理された一つのworktree・一つの新規Space・一つの書込みsessionで実証する。Spaceという区分と配置は使うが、旧Intent registry・工程stateを読み込んだり変換したりしない。
- 新方式のCLIには `--space <name>` を明示する案とし、sessionへSpaceとIntentの両方を結び付ける。別Spaceへ暗黙にフォールバックしない。
- `--project-dir` は新方式ではGit worktree rootを指す。省略時はcwdからGit rootを解決する。既存workspaceのroot解決APIにはGit探索がないため、同じ挙動と仮定して流用しない。

## 2. KDRテンプレートとCLI

以下の `aidlc` コマンドは**新規提案であり、現在は実行できない**。記載した名前をM1の公開契約候補とする。

```markdown
---
type: KDR
intent_id: 9c2f0b1d7a684e55a19d0680f731db26
title: 検索結果をタグで絞る
description: タグ絞込みの目的・判断・検証と残件を記録する。
tags: [kdr, search]
generated:
  by: "<実際のproducer/version>"
  at: "2026-09-08T03:00:00Z"
status: draft
---
# KDR: 検索結果をタグで絞る

## 目的と完成条件
利用者がタグを一つ選んで検索結果を絞れる。タグ未指定時の結果は変えない。
作業範囲の根拠: ユーザーがタグ一つの絞込みを依頼。複数選択は判断待ち。

## 参照する設計・ルール
bundle: aidlc/spaces/main/knowledge
rules/working-agreement: Git commit <参照したcommit>、本文SHA-256 <値>
design/search: Git commit <参照したcommit>、本文SHA-256 <値>

## 不明点と進め方
空の結果の表示は既存仕様を確認。まずタグ一つで試す。

## 判断と結果
タグは完全一致で比較する。部分一致は別タグが混ざるため採用しない。
試作で既存一覧と絞込みを共用できた。

## 検証・レビュー
対象コードcommit: <検証したcommit>
go test ./src/search -run TestFilter: 初回は期待3件に対し5件で失敗、実装後は成功。
レビュー: 未実施。CI: 未実施。成功とは扱わない。

## 残件と再開
次は独立レビュー。質問待ち: なし。完了を報告する前にCIを確認する。
```

テンプレートはOKF front matterと6見出しを持つ。`type: KDR` は型名の提案で、OKFの組込み型を主張しない。Ruleには `type: Rule`、一般知識には内容に応じた型を提案する。OKF必須のtypeに加え、製品テンプレートではtitle/descriptionを記載する。上例は作成後のKDRであり、作成前draftにIDの手入力は不要。CLIが作成時に `intent_id` とpathへ同じIDを割り当て、その後は変更しない。
`generated.by` は実際の作成・更新actor、`generated.at` は内容が最後に意味のある変更を受けた日時を記す。上例のactorと日時は説明用であり、実際の値へ置き換える。actorのCLI入力契約は実装計画で具体化する。descriptionは要約、tagsは分類・検索に使う。
`status` はOKF文書の成熟度であり工程の位置やIntent完了状態ではない。未知のmetadataを更新時に保持し、`verified`を未確認の成功証明として自動追加しない。sources・verified・stale_after・resourceも必要に応じて使い、保存時に失わない。全項目を一律必須にはしない。詳細は[標準metadataの補足](../ram/decisions/2026-09-08-kdr-okf-metadata-clarification.md)。
目的・完成条件は具体文必須。他の節は「未実施」「未確定: 理由」「該当なし: 理由」を許し、空欄・テンプレートの案内文のままは拒否する。
更新時には「判断と結果」「検証・レビュー」「残件と再開」の少なくとも一つに空白以外の変更を要求する。
CLIは文章の十分性、テストの真偽、人間の承認を判定しない。AIとレビューが意味を確認する。
日時だけ、コメントだけ、metadataだけの変更は今回の記録補完として扱わない。

| 操作案 | 役割 |
| --- | --- |
| `aidlc kdr template` | 配置されたテンプレート全文を返す |
| `aidlc kdr create --file <draft>` | AIが埋めたMarkdownを検査し、新ID・pathを返す。既存ファイルは上書きしない |
| `aidlc kdr list` | IDとタイトルを表示する。現在Intentは変更しない |
| `aidlc kdr show <id>` | 同じKDR全文と更新用SHA-256を返す |
| `aidlc kdr show <id> --raw` | 形式不正でも安全に読めるbytesとhashを返す。欠落は欠落と返す |
| `aidlc kdr update <id> --file <draft> --expect <sha256> --session <session-id>` | 読んだ版が現在も同じ時だけ保存し、対応sessionの未記録表示を解消する |
| `aidlc kdr repair <id> --file <draft> --expect <sha256-or-missing> --session <session-id>` | 不正・欠落KDRを指定した同じIDで復旧。形式を直しても未記録は解消しない |
| `aidlc kdr check <id>` | 形式・必要節を検査する。作業完了や最新検証の認定はしない |
| `aidlc session bind <id> --session <session-id>` | 対象Intentを明示し、必須rulesとKDRを読み出して開始・再開する |
| `aidlc session inspect --session <session-id>` | 対応ID・未記録の有無・実行中操作の有無を表示する |
| `aidlc memory rules` | 必須rulesの全本文と参照hashを返す。sessionの開始は別途bindする |
| `aidlc memory search <query>` / `aidlc memory show <concept-id>` | aidlc内のOKF処理で指定Spaceの検索候補/本文を返す |
| `aidlc memory search --intent-id <id>` | frontmatterのintent_idを完全一致検索する。検索語なしでも使え、検索語を併記した場合は両条件を満たす文書を返す |

各コマンドは `--project-dir <root>` を受理する。通常終了0、入力・契約違反2、I/O・外部CLI故障1とし、成功出力と診断をstdout/stderrへ分ける。
`show`のSHA-256は上書き競合を検出する値であり、承認receiptではない。
保存は標準ライブラリでpath制限、regular file・UTF-8・容量上限（KDR 256 KiB）、lock内の期待hash比較、同じディレクトリの一時ファイルから置換を行う。
不正path、symlink、ID不一致、競合は拒否する。置換後に同期処理等で失敗した場合は「保存結果不明」と返し、`show`で確認してから再試行する。
無条件上書きや、失敗後の自動的新規Intent作成はしない。
修復はIDを新規生成せず、復元先の明示IDと期待hash（欠落なら`missing`）をlock内で確認する。破損本文はraw読取で退避でき、修復後に今回の判断を通常updateで記録する。
repairは未bindのsessionでも明示IDで実行でき、bindingを作成・変更せず、未記録を解消しない。これにより新sessionでも壊れたKDRを直してからbindできる。
KDR保存とsession更新を一つのtransactionにはしない。KDR保存→sessionの未記録解消の順とし、後者の失敗は「KDR保存済み・未記録表示の更新失敗」と区別する。再読込して復旧経緯を追記する。
lockによる競合防止は正規CLI間の保証であり、外部editorや別processの任意書込みに対する完全な排他ではない。

利用者はIntentの名前で作成・選択を指示する。以下のID・session指定はAIとCLIが扱う下位操作であり、利用者へ手入力を要求しない。名前選択の入口はM1実装計画へ反映する。
開始例は、hookが示すsession IDをAIが使って `template` → AIが名前をtitleに含むdraftを記述 → `create` → 内部で `bind`。
draftの置場はworktree内 `aidlc/.runtime/drafts/<session-id>.md` とし、hookが絶対pathを案内する。runtime内の.gitignoreでGit管理対象外とする。
AIはそこだけを `apply_patch` で編集できる。`create`成功はIntent作成であり、まだそのターンの結果記録ではない。
質問への回答やテストがまとまったら `show` → draft更新 → `update --expect ...` を使う。
翌日は利用者が同じ名前を選び、AIとCLIが `list` のtitleから同じIDを特定して内部で `bind`。KDR、Git差分、PR・CIの現物を確認してから、再開時の判断を同じKDRへ追記する。
`resume`、`complete`、Stage遷移コマンドは追加せず、完了・質問待ち・中断理由は本文へ書く。

## 3. hookの記録漏れ判定

### 提案する最小の一時管理

ファイルの存在や更新日時だけでは、KDR更新後に再び実装した場合の記録漏れを判定できない。
そこで**sessionごとのSpaceと対応ID、現在turn ID、未記録の真偽、実行中tool IDを最大1件、必須rules読込済みの印と読込内容の合成hash**だけ、一時的に保持する案とする。
turnはCodexの応答単位であり、製品の工程ではない。配置はworktree固有の `aidlc/.runtime/sessions/<session-id>`。CLIとhookが同じroot・path検査を使う。
小さな固定形式のtextとして上書きし、Gitへ含めない。CLIとhookは同じsession lockを使う。
過去tool、コマンド内容、結果ログ、成果物snapshot、receipt、工程番号は保存しない。
消失・破損時は「記録済み」にせず、再bindと現物確認を要求する。KDRだけで目的と再開情報を読めることを保つ。
初回bindは必ず未記録にする。同じIDへの再bindは既存の未記録を保持する。
これは承認済み要件から自動的に確定する実装詳細ではなく、**採否を確認する設計選択**である。
UserPromptSubmitでhook入力のturn IDを設定し、rules読込済みの印を外す。bindはこのturnに対して本文読込が成功した時だけ印とhashを設定する。
PreToolUseは入力turnの一致と、entry list・必須本文の現在hashを照合する。変更やcompact後には印を外し、再bindを要求する。
hashは一時的な変更検出用で、知識や成果物の複製でも承認証明でもない。session IDはhookがAIへ案内し、認証秘密として扱わない。

### 対応経路と判定

M1の候補環境は現在インストール済みの `codex-cli 0.153.4`、macOS arm64。
バージョン表示は今回確認済みだが、新hookの動作は未実証である。現行公式文書の機能がこの版で動くかをM1の先頭で確認する。
2026-09-08に固定実行ファイル内のschemaも確認した。SessionStartにはturn_idがないためsessionの読込済み印だけを無効化し、turnはUserPromptSubmitから得る。実際の発火・拒否・非同期終端の対応は引き続きlive検証対象である。
不一致なら、黙ってversionを上げたり保証を弱めたりせず、対応版・影響を提示して再確認する。

| 節目 | 提案する処理 |
| --- | --- |
| SessionStart・再開・compact | 開始案内を出し、KDRと必須rulesの再読込を要求する。古い一時情報だけで書込みを再開しない |
| UserPromptSubmit | 既にbind済みならそのturnを未記録にする。読み取りだけの結果・質問も、当該Intentについて一回まとめて記録する |
| PreToolUse | `Bash`、`apply_patch`を対応対象とする。一般操作はbind済み・KDR形式有効・必須rules読込成功を要求。未記録にして実行中IDを1件置く |
| PostToolUse | 対応する実行中IDだけを外す。成功・失敗いずれも未記録のままにする。失敗したテストや途中変更も記録対象になる |
| KDR update成功 | 実行中の一般操作がなく、同じsession/Intentへの保存が完了したときだけ未記録を解消する。CLIの文字列・toolのexit codeだけでは解消しない |
| Stop | 未記録・KDR不正・実行中操作があれば、一回だけ補完を要求。補完後も不正なら警告して止まり、未記録を保持する |

未記録は「次の実装を禁止する」条件ではない。記録前でも、同じ目的のテスト→実装→テストを続けられる。
ただしKDRがない、不正、別ID、必須rulesを読めていない場合は、一般操作に入れない。
同時実行はM1で認めず、実行中slotがある間は次の一般操作とKDR更新を拒否する。
長時間コマンドは元のPostToolUseを受けるまでslotを保持する。`write_stdin`は元の操作の続きとして扱う。
process完了前の非同期session返却を終端と扱わない。先頭実証で、成功・失敗・非同期返却・poll完了を区別できることを確認する。
M1の手順はforeground完了待ちに限定する。toolが途中で非同期sessionを返す場合も、完了までslotを保持できる必要がある。
固定版のhook入力から終端を識別できなければM1の対応環境gateを満たさない。勝手にslotを解放せず、実装を止めて対応方法を確認する。
強制終了でslotが残った場合、停止したprocessと現物を人間またはAIが確認して再bindする。再bindを実行中processの自動取消しにはしない。
通常bindはslotが残っていれば拒否する。残留slotの復旧だけ `session bind ... --recover` を使い、process停止を確認したという明示操作として扱う。未記録を保持し、他Intentへの切替には使えない。
未記録のまま別Intentへbindすることも拒否する。同じIntentへの再bindで未記録を消さない。新sessionにはIDの自動選択を持ち込まない。

KDR用CLIとdraft修復には例外を設けるが、一般書込みとの混在は許可しない。
`apply_patch`の例外は、そのsessionのdraftファイル1件だけに触るpatch。KDR正本への直接patchはCLI更新へ案内する。
Bashの例外は、固定したaidlc実行ファイルに対する既知の単独コマンドと引数列だけ。
シェル結合、リダイレクト、置換、任意環境変数・別実行ファイルを伴うものを「aidlcが含まれる」という理由で免除しない。
draftを用いるため、記録コマンドにheredoc・パイプを要求しない。
KDR更新自身のPostToolUseで未記録を再設定しないよう、一般操作と記録操作を同じ分類で処理する。

| 未bind・KDR不正でも許す単独CLI | 追加条件 |
| --- | --- |
| `kdr template/list/show/check`、`session inspect`、`memory rules/search/show` | 読取りのみ。showの`--raw`を含む。KDRやrulesが不正ならその診断を返す |
| `kdr create` | 固定draftだけを入力とし、新規IDだけを作る。bindや記録完了を兼ねない |
| `kdr repair` | 固定draft、明示ID、期待hashまたはmissingが必要。未記録は解消しない |
| `session bind` | KDRを検査し必須本文を読み出す。実行中slotなし。別IDへの切替は元IDが記録済みの場合だけ |
| `session bind --recover` | 同じIDまたは未bindからの明示IDに限定。停止したprocessを確認後に残留slotを解消し、未記録で再開 |
| `kdr update` | 例外経路で呼べるが、成功には同じIDへのbind、期待hash、slotなしが必須 |

単独CLIの認識文法は、固定実行ファイルと空白区切りの引数、単一引用符で囲んだliteralだけとする。
裸のtokenは英数字と `_./:=@,+-` に限定し、変数展開・二重引用符・改行・制御演算子を拒否する。日本語や空白を含む値は単一引用符で渡す。
全例外のfile引数は、そのsessionのdraft絶対pathと一致させる。一般CLIの文字列を部分一致で免除しない。
draft編集と読取りのPostは未記録を消さず、一般操作のslotも解除しない。Pre/Postで同じ分類を使う。

記録漏れの例:

1. KDRなしでコードpatch → 拒否。draft作成とKDR作成は通る。
2. KDR作成・bind → RED → 実装 → GREEN → `Stop` → 未記録なので補完要求。
3. 結果を `update` → `Stop` → 通る。更新後に再patchすれば再び未記録になる。
4. 同じ本文の再保存、日時だけの変更、別Intentの更新、保存失敗 → 未記録を解消しない。
5. 方針を考えたがtoolを使わなかったターン → UserPromptSubmit時の未記録が残るので、結果・判断を記録する。

### 保証の限界

正常稼働する対応hookは記録の構造と更新機会を確認する。文章の真実性や十分性を証明しない。
shell内部、既存processへの入力、対応外tool、外部アプリ、人間による編集の全操作を捕捉するとは約束しない。
M1ではMCPによる書込み・入れ子の書込みagent・バックグラウンド書込みを対応作業手順に含めず、Bash/apply_patchへ寄せる。
全操作を記録する新auditやCodex transcript解析を代わりに要求しない。

公式文書はhook trust、対応tool、実行前拒否、Stopによる続行要求を説明し、完全な強制境界とは位置付けていない。
hookの無効化・未信頼・起動失敗では拒否処理自身が走らない可能性がある。
製品の診断と導入試験で拒否・復旧を確認し、故障中の一般作業は運用上停止する。
「すべてのhook故障でも強制停止できる」という受入条件は置かない。
根拠: [Codex hooks公式文書](https://learn.chatgpt.com/docs/hooks)（2026-09-07参照。0.153.4の実証とは区別）。

## 4. 読取り・修復・質問・中断・故障

| 状況 | AIとCLIの扱い |
| --- | --- |
| Intentを選ぶ前の読取り | `kdr template/list/show/check`、session診断、必須rules読込を許す。Bashの汎用コマンドを読み取りと推測して免除しない。必要なら質問・調査の目的でKDRを作ってbindする |
| bind後の読取り・調査 | 実装変更がなくても最後に判断・調査結果をまとめる。毎回のcatや検索結果の転記は不要 |
| KDR不正・欠落 | draftの作成・修復、show/checkを許す。既存IDのファイルが壊れているときは新Intentを増やさず、取得できたraw bytesと期待hashを使う修復経路を設ける |
| 質問待ち | 質問tool自体を記録不足で拒否しない。返答を待つ理由と、進められる範囲をKDRへ書く。回答後も同じKDRを更新する |
| 調査・試作だけで終了 | 得た結果と「実装・テスト未実施」を明示すれば正常終了できる。完成コードを必須にしない |
| ユーザー中断・crash | 中断を妨げない。保存できなかった内容を保存済みとしない。次回bindでKDR・差分・実process・PR/CIを照合する |
| 保存失敗・競合 | 以前のKDRを無条件に上書きせず、未記録を保持。現物をshowで再取得してから再編集・再試行 |
| hookまたはOKF故障 | 診断・読取り・記録修復・中断を残し、一般作業を止める。Stopは無限に再実行させず、警告付きで終了可能にする |

`Stop`再入には `stop_hook_active` を利用し、自動補完を一回に制限する。Stopの続行で新turnが生じる挙動も固定版で試験し、記録済みなのに永続的に未記録へ戻るループを防ぐ。
一つの補完turnではbind→必要な記録update→Stopまでを完結させ、KDR更新自身を新しい記録要求の原因にしない。
再入での停止は「KDR記録済み」の認定ではない。次回も未記録を表示する。
欠落した既存KDRの修復はGitからの復元を基本にし、CLIでの新規ID作成と混同しない。

検証の対象版は、まずコードをcommitしてそのIDをKDRへ記す。作業中は「未commit、暫定検証」と明示できるが、最終成功の根拠にしない。
レビューはbase/head、CIは対象headとURLを参照する。後でコードを変更したら必要な検証・レビューを更新する。
KDR自身の追記でcommitが増えても、記録に自己のcommit IDを書こうとしない。コード対象版と最終PR headを分け、PR headのchecksは別途成功を確認する。

## 5. OKF固定版・共有bundle・必須rules

ローカルの実装参考は、ユーザー指定の [`docs/実装_okf-agent-memory/`](../実装_okf-agent-memory/)。
独立したGit情報がないため、下記固定commitとの一致は未確認。版に関わる判断は対象fileを照合する。
参照先指定の合意は[RAM](../ram/decisions/2026-09-07-okf-local-implementation-reference.md)に記録した。

**固定候補はOKF Agent Memory `v0.1.2` / commit `d4c523ed5ce916fa207fe314851b98721421c891`。**
従来のOKF v0.2仕様snapshot `ad30107…` と別の固定対象であり、最新upstreamとは断定しない。
OKF処理はGoのaidlc内部に実装し、初期資産もバイナリに内包する。別のokf実行ファイルの取得・配置を利用者に要求しない。
参照実装の版を固定することと、外部CLIを必須依存にすることは分ける。先行案のrelease asset取得・hash表は今回の製品導入手順から外す。
初期資産は導入・Space作成時に展開し、通常の作業では配置済みOKF本文を読む。利用者が編集したRuleを毎回内包版で置き換えない。
Go内部への具体的な取り込み・実装方法は計画で確認し、外部Go moduleを追加しない。

Spaceは本家準拠の正規入口で作成し、その生成内容としてOKF bundleと初期Ruleを用意する。別のokf init操作は要求しない。
default Spaceの初期組織Ruleも `knowledge/rules/rule.md` とする。新Spaceにはその作成時の内容をコピーする。
初回配布でのdefault初期化、コピー元が欠落・破損した場合の失敗・復旧の詳細は、承認済みコピー方針と本家の根拠に沿ってM1計画で具体化する。
既存bundleや編集済みRuleを無条件に上書きしない。既存org.mdからの移行も行わない。
共有正本はSpaceごとの `aidlc/spaces/<space>/knowledge/`。Gitで共同管理し、各Intentが同じ正本を参照する。
複数repository間の自動同期・外部共有repository採用はM2の確認対象に残す。

利用者が呼ぶのはaidlcのCLIとする。以下は新規提案の文法であり、現在動作するコマンドとは扱わない。

```text
aidlc space create main
aidlc memory show rules/rule --space main
aidlc memory search '検索 タグ' --space main
aidlc memory search --space main --intent-id 9c2f0b1d7a684e55a19d0680f731db26
aidlc memory show design/search --space main
```

必須Ruleの入口もbundle内の `rules/entry.md` に置き、OKFの `type: Rule` を持つ文書にする案とする。本文に必須Ruleへの明示リンクを列挙する。hookの設定には入口Concept IDだけを持ち、Rule本文をbundle外に複製しない。
開始・各ユーザーturn・再開・compact後に、列挙順でaidlc内のOKF読取処理を呼び、全必須本文をAIへ返す。
検索上位で必須rulesを選ばない。欠落、失敗、過大出力は作業開始不可とする。
M1では必須本文合計16 KiBを上限候補とし、黙って切り捨てない。超過時はrules整理の確認へ戻す。
hook出力の省略を避けるため、hookは短い案内を返し、本文は明示CLI読込で渡す。
bind成功は「本文を読み出した」ことを確認するだけで、「AIが理解した」「そこに任意の権限がある」証明ではない。
必須ruleとentry listを同じturn中に変更したら再読込を要求する。

必須ruleの最小本文は、Intentの作成・同じKDRの継続、許可範囲を越える判断の確認、質問・調査・試作の選択、テスト先行の反復、独立レビュー、検証対象版と未実施の明記、共有知識の採用手順を定める。
一般knowledgeは命令の権限を持たず、必要な本文を `search` と `show` で読む。

共有知識の更新は、KDRに提案と理由を書き、差分レビューと利用先の承認に基づいてaidlc内のOKF作成・更新処理を使う。
M1の一周ではrule改変で自己合格させず、小さな新規knowledgeを一件追加する。
OKF既定の知識変更logとindex更新は維持する。これは知識変更の履歴であり全AI操作auditではない。
固定版create/updateは本文、index、logをまとめて原子的に保存せず、CASも提供しない。
そのためM1では単独writer・更新前のcleanなGit版・変更対象限定を前提にし、失敗時は3者の実物を確認する。自動再試行や無条件resetはしない。
KDRも同じbundleのOKF Conceptとして保存・検索・表示・検証できることを必須にする。KDR CLIはOKF形式に加えてKDRの本文契約と安全保存を扱う。形式対応と外部okfの書込み処理をそのまま使うことは分ける。
ユーザー指定によりintent_id検索もM1へ含める。指定Space内のfrontmatterだけを完全一致で比較し、該当なしは0件、空・形式不正の明示IDは入力エラーとする。結果にはConcept ID・intent_id・title・description・相対pathを含める。検索だけで会話の選択や未記録表示は変更しない。title変更後の検索、Space隔離、本文内のIDを拾わないこと、検索語との併用、metadata保持を受入検証へ加える。根拠: [ID検索の合意](../ram/decisions/2026-09-08-okf-intent-id-search.md)。
KDR更新もOKFのindex/log整合を検証対象に含める。本文保存後にindex/logだけ失敗した場合は部分成功を明示し、hookの未記録を解消しない。全操作auditは追加しない。具体的な安全保存・index/log修復手順は、この変更を踏まえたM1計画の再確認事項とする。
この節の変更は上記CLIの保存成功条件にも適用する。本文だけの保存成功で未記録を解消しない。OKF処理をGoバイナリ内へ統合する方針は承認済み。具体的な保存実装とM1全体の実装開始は別途計画の対象である。

根拠: 固定commitの [CLI](https://github.com/okf-memory/okf-agent-memory/blob/d4c523ed5ce916fa207fe314851b98721421c891/docs/CLI.md)、
[CLI実装](https://github.com/okf-memory/okf-agent-memory/blob/d4c523ed5ce916fa207fe314851b98721421c891/cmd/okf/main.go)、
[更新実装](https://github.com/okf-memory/okf-agent-memory/blob/d4c523ed5ce916fa207fe314851b98721421c891/pkg/okf/mutate.go)。
Context7に指定OKFと一致するlibraryがなく、一次資料へfallbackした。Codex hookはContext7と公式文書を照合した。

## 6. M1の実装計画と許可範囲

### AI-DLCに準拠する配布

配布の仕組みはリポジトリ固定AI-DLC `2.6.123` に準拠する。共通資産とハーネス別資産から配布treeを生成し、
Codexでは `.codex/`、`.agents/`、`aidlc/` と入口文書等へ配置する対応を計画する。
本家release CLIは隣接runtime treeを使うが、Go版はユーザー指定のシングルバイナリを継続する。
初期資産も内包し、導入時に本家の配置先へ展開する。別配布のruntime treeを必須にしない。
Goへの移植であり、Bun実装の再導入や全旧Stage資産の同梱を意味しない。

配布元のテンプレート・初期Ruleはsrc配下の製品資産、利用先のKnowledge・KDR・編集済みRuleはSpaceのOKF文書として区別する。
初期Ruleの配置先も `aidlc/spaces/<space>/knowledge/rules/` とし、更新で利用者の記録を無条件に上書きしない。
M1はfresh配置からCLI・hook・skill・OKF・KDRが接続することを確認し、M3は生成再現性、既存設定との統合、更新・rollbackまでを扱う。
外部okf専用配置の案は撤回する。配布・Space初期化・OKF操作はaidlc一つで成立することを検証する。
根拠: [固定版の配布調査](../ram/research/2026-08-29-existing-distribution-format.md)。

### 対象fileと単独writer

M1は「新規IntentをOKF読込からKDR記録、反復、独立レビュー、結果保存、別session再開まで完走させる」一つの実装単位を提案する。
製品の作業担当は通常のCodex本体と最小skillで足りる。custom agentはread-onlyレビュー役一つを追加する案とする。
レビュー役はM1では固定コード版の別Git checkoutと別root sessionで起動し、Codexのread-only sandboxを使う。
その専用配置にはwriter用hookセットを入れず、同じIntentのKDR・必須rulesと対象コードだけを読ませる。結果は作業担当へ返し、作業担当が同じKDRへ記録する。
reviewerにwriterのbind/updateを要求しない。共有の利用設定を無効化する操作ではなく、freshなレビュー用配置として検証する。
writerがレビューを依頼したturnは未記録を保持する。製品の役割管理stateや入れ子の書込みagentは作らない。
開発側のplanner、implementer、reviewer運用とは区別する。

| file / package候補 | 変更内容 |
| --- | --- |
| `src/internal/workspace/` のSpace作成処理・対応test | 固定本家の作成契約を維持し、OKF初期化との接続と失敗時の境界を計画段階で確認 |
| 新規 `src/internal/kdr/{document,store}.go` と対応test | OKF front matter保持、固定見出し、Space内ID、hash比較、安全保存、一覧・読取り・修復、index/log整合 |
| 新規 `src/internal/minimal/{session,hook,memory}.go` と対応test | session単位の一時管理、tool分類、hook応答、Go内部のOKF処理。旧Stage packageへ依存しない |
| `src/internal/cli/cli.go`、新規 `kdr.go`・`minimal.go` と対応test | 公開文法、help、終了コード、隠しhook入口 |
| `src/cmd/aidlc/main.go`、新規 `minimal.go`・`minimal_journey_integration_test.go` | 実I/Oとの接続、fresh利用先で一周 |
| 新規 `src/harness/codex/minimal/` 配下の `hooks.json`、`SKILL.md`、`agents/aidlc-reviewer.toml`、配置契約test | M1試験用の最小配置セット。利用先のdiscovery pathへ配置する |
| 新規 `src/core/minimal/kdr-template.md`、`knowledge/rules/working-agreement.md`、`knowledge/rules/entry.md` | テンプレートとOKFに配置する最小rules。製品本文はsrcへ置く |
| `.github/workflows/ci.yml`、`docs/development.md`、`docs/architecture.md`、RAM | 新方式の試験、固定依存の取得・復旧、保証範囲を記載 |

既存 `state` writerはstate形式とpath、`recordlock`はSpaceを含むidentityに結び付いているため丸ごと流用しない。
安全保存の考え方だけ参照し、新packageはGo標準ライブラリを優先する。新規外部Go moduleは0件。
旧CLIと配置セットを新規利用の標準入口から外す作業はM3。M1試験ではfresh利用先に最小セットだけを配置し、旧hookを重ねない。
これは新旧二重運用機能の提供ではなく、旧経路を削除せず新方式だけを試験する開発上の境界である。

### 受入条件と順序付き検証

最初に固定Codex版のhook対応を実証する。対応しない場合は、その環境を前提に残りを実装し続けない。
M1のfresh一周は正規のSpace作成CLIから始め、作成による暗黙の選択変更がないことと、OKF初期配置への接続を受入条件に含める。
その後、1 Issue／PR・1単独implementer・1 work unit `m1-minimal-intent-journey` に次の順序を渡す。
下記test名は追加予定の契約で、存在・成功を主張するものではない。

| 順 | 受入条件 / TDDで確かめる境界 | loopのtargeted command案 |
| --- | --- | --- |
| 1 | 1 Intent/1 OKF KDR、Space隔離・不正Concept path・空欄・no-op更新を拒否 | `go test ./src/internal/kdr -run '^TestDocument'` |
| 2 | 競合と保存失敗で黙って壊さない。破損KDRの修復可能性 | `go test ./src/internal/kdr -run '^TestStore'` |
| 3 | 公開CLIから作成・同一ID更新・再開。旧stateなしで成立 | `go test ./src/internal/cli -run '^TestKDR'` |
| 4 | Spaceの全Knowledge/KDR/Ruleをaidlc内のOKF処理で表示・検索・検証でき、必須Rule全文を読む。欠落・失敗・超過時に開始不可 | `go test ./src/internal/minimal -run '^TestMemory'` |
| 5 | 未記録、別ID、実行中競合、自己trigger、Stop再入、中断・故障を上記契約どおり処理 | `go test ./src/internal/minimal -run '^Test(Session|Hook)'` |
| 6 | 配置先からテンプレート/rulesを読む。単独writerとread-onlyレビュー役 | `go test ./src/harness/codex/minimal -run '^Test'` |
| 7 | RED→実装→GREEN→review修正→記録→別session再開。小さなOKF知識追加 | `go test -tags=integration ./src/cmd/aidlc -run '^TestMinimalJourney' -count=1` |

各変更behaviorは実装前にrunnableな意図したREDを確認する。過去成功やcompile failureをRED証拠にしない。
単独implementerが全項目を完了し、親が差分とtargeted testを一度確認する。独立reviewは `verification_mode=review` で読み取り専用。
blocking finding修正後に差分を固定し、親の `verification_mode=final` で次を一回集約する。

- `go test -count=1 ./...`、`go test -race -shuffle=on -count=1 ./...`、`go vet ./...`。
- `gofmt -l src`、`git diff --check`、`go mod tidy -diff`（変更を適用しない）。
- 既存CIのintegration対象と新規KDR/minimal filesystem試験、`TestMinimalJourney`。既存CIのgateは削らない。
- `CGO_ENABLED=0` のdarwin/linux/windows × amd64/arm64 buildを既存CIと同様に実行。build成功を各OSのhook動作保証にしない。
- aidlc単一バイナリを使うLinuxの非live一周と、macOS arm64/Codex 0.153.4のfresh sandbox live E2E。利用モデル名も結果に記録し、新たな有料service契約はしない。
- liveではKDRなし拒否、記録後許可、後続変更で再要求、質問待ち、中断と別session再開、必須rule欠落、CLI故障、長時間toolと更新の競合を確認する。

finalは対象fileを変更しない。修正が生じたら証拠をstaleとしてloopへ戻す。
GitHub Issueは `機能開発` を付け、計画・実装許可の根拠を本文に明記する。
Issue紐付きPR、独立review、対象PRのchecks成功後のmerge、main反映とIssue closeまでがM1実装許可を受けた場合の完了作業である。
今回これらの外部操作は行わない。

### 本家との差分・戻し方

固定AI-DLC 2.6.123の確認済み範囲では、Stage graph、承認・完了marker、auditと次Stage選択を使う。
提案は、それらを経由せずKDRとOKFで作業を進める意図的変更である。理由は試作・テスト・修正の反復と運用を簡素化するため。
公開CLI、保存形式、導入・再開手順は旧方式と互換にしない。ユーザーの既存無視の回答に従い、移行・二重運用・旧data削除は行わない。
本家最新upstreamとの全体比較や「その他に差分なし」は主張しない。

M1試験の戻し方は専用sandboxと専用okf配置の利用を止め、実装PRは必要なら通常のrevert手順で戻す。
KDRとbundleを自動削除せず、破損時は保存前Git版と差分を確認する。正式な一般配布・新方式更新/rollbackはM3で承認を得る。

## 採用前に確認する選択

一つの推奨案として、以下をまとめて提示する。採用されるまではこの文書を実装許可に使わない。

| 判断 | 推奨案 | 代案と影響 |
| --- | --- | --- |
| 対応と配置 | Space内 `knowledge/` にKnowledge/KDR/RuleをすべてOKF配置（ユーザー指定済み）。下位分類は採用済み・型名は提案 | タイトルpathは改名時に対応が変わる。外部共有repoは参照版・権限・更新運用が増える |
| 記録漏れ | session限定の未記録表示＋実行中1件、ターン終端でまとめる | 存在確認だけなら更新漏れを検出できない。各操作前後に本文更新すると反復の負担が増える |
| M1対応環境 | Codex 0.153.4/macOS arm64、Bash/apply_patch、一般操作は直列 | 対応tool/harness・並行writer拡大はM1の契約と検証量を増やす |
| OKF導入 | OKF処理・初期資産をGo内包、Space作成時にOKF初期化（採用済み） | bootstrapはAGENTS/skill等を変更する。自動最新版追従は検証済み契約が変わる |
| 許可 | まずM0契約案の採用可否を確認。M1開始は別に明示する | M1全体を許可する場合は、上記file・Go内包OKF・Issue/PR/mergeまでを一括対象にする |

**M0は具体案を作成した段階。ユーザーの契約採用がまだなので、M0全体を完了扱いにしない。**
M1承認があっても、固定hook非対応、新たな仕様差分、外部module、追加権限、重大な保存・運用選択が出たら停止して確認する。
M2/M3、旧33 Stageロードマップの継続実装、既存ファイル削除はM1許可に含めない。

参考記事からは、仕様・実装のレビューと小さな反復を参考にした。記事の全工程や二系統レビューを必須契約として輸入していない。
[Cojiの記事](https://zenn.dev/coji/articles/solo-software-factory-without-reading-code)（2026-09-07本文確認）。
