# アーキテクチャ

AI-DLCは、一つの目的をIntentとして管理する単一Go実行ファイルです。
初期化と目的整理を必須とし、その他の工程はユーザーが承認したIntentごとの実行計画で選びます。
利用者の操作は[利用者ガイド](../src/docs/user-guide.md)、配布・更新は[配布手順](distribution.md)を参照してください。

## 工程定義と実行状態

[stage-graph.json](../src/core/workflow/stage-graph.json)は6種類のstageと手順Markdownの対応を定義します。
必須の先頭は`initialization`、`discovery`です。`architecture-analysis`、`planning`、`tdd`、
`integration`の採否・順序・省略理由をIntentの計画として保存します。計画変更にも承認を要求します。
各[stage Markdown](../src/core/workflow/stages/discovery.md)のfrontmatterには許可担当、入力条件、
文書outputs、開始・終了Sensorを定義し、本文にはその段階の手順を記述します。
`intent procedure`は現在の実行回の定義と、metadata条件から解決した文書path・版を返します。

`src/internal/workflow`は配置済みの定義を読みます。`src/internal/flow`は実行計画、現在のstate、
Sensor、review、会話承認、Unitを管理します。正本は`aidlc/spaces/<space>/intents/<id>/state.json`です。
同じIntent directoryの`history/`へ状態変更を保存し、全操作の監査記録とは区別します。
同じstageを再実行する場合も新しいstep IDを使い、過去の成功をそのまま適用しません。

共有するstateはメインAI一人が更新します。期待する`revision`が一致しなければ保存せず、
同じdirectoryの一時fileから置換します。未知field、重複JSON key、不正identity、破損dataを診断します。
保存途中は同一要求で再試行し、state確定まで成功扱いしません。

## Sensor、レビュー、承認

各実行回では、開始Sensorとbeginの後に作業し、終了Sensor、独立review、成果承認を経てfinishします。
Sensorは文書の存在・metadata・対象Intent・受理版・hash、コード版や実測結果などの機械条件を検査します。
内容の妥当性、根拠、テストの意味は独立reviewで確認します。文書outputsとコード・テスト結果の証拠は別です。
選択しなかった段階の成果を一律には要求しません。

Sensorの対象hashには現在step、実行計画・定義、意味のある設定、成果物本文、検証対象一式のSHA、結果JSONと出力を含めます。
stateのrevisionやreview記録だけの更新では対象hashを変えません。対象fileが変われば古いpassは使えません。
計画承認と成果承認は別のrequestです。提示した対象に対する実際の後続会話の回答を記録します。
独立reviewと会話出典は運用上の確認で、同一OS権限に対する完全な著者・本人認証ではありません。

## メインAIと専門担当

メインAIがユーザーとの対話、共有stateと文書の保存、標準ツールによる担当の起動と結果回収を行います。
製品CLIはエージェントを起動しません。5種類の専門担当は要件整理、調査、ステージ計画、worker、reviewerです。
worker以外はread-onlyで本文案や報告を返し、共有Knowledgeの保存はメインAIが行います。
製品の行動規約は[aidlc Skill](../src/harness/codex/SKILL.md)、操作の選択は
[aidlc-cli Skill](../src/harness/codex/aidlc-cli/SKILL.md)、正確な引数・型はCLI helpにあります。

Unitは担当範囲、検証、依存、Bolt（作業のまとまり）を持ち、workerは通常ディレクトリへ割り当てます。同じrootでの順次作業を許可し、同一・親子rootの重複予約を拒否します。
依存統合前や担当範囲の重複を拒否し、結果は現在のrun/session/rootと実効検証集合のSHAに照合します。反映時は管理元の同じ集合の内容一致を確認します。
Unitなしのworkerも作業場所を登録します。同一管理rootの全Space・Intent・sessionをまたぐ予約競合を検査します。
別の管理rootまで横断して実workerの稼働を監視する機能ではありません。

担当管理の初回開始と復元不能な管理記録の再作成には人間確認を要求します。
結果提出、toolの完了、Stop通知だけで予約を解放せず、メインAIが既知の処理終了と成果回収を確認します。
中断したUnit runは`needs_confirmation`となり、実環境を確認してから再開します。

## hookとローカル情報

`src/internal/app`は公開操作とCodex hookを接続します。session選択、現在turnのRule全文読込hash、
実行中tool IDなどをローカルに保持します。PreToolUseは通常操作の前提・承認待ち・現在段階の担当を確認します。
同じIDのPostでtool slotを解放しますが、子エージェントの稼働終了を意味しません。
Postが届かない場合は処理終了を確認し、同じsession/Space/Intentへの明示`session bind --recover`を使います。
hookは通常のAI操作の飛ばし防止で、OS権限による全書込み経路の封鎖ではありません。

`aidlc/.runtime/`は内部`.gitignore`で無視する機械ローカルの会話・予約情報です。
`src/internal/filestore`はroot境界、symlink拒否、256 KiB上限、単一file置換、ローカルlockを提供します。
`workspace`はSpace/root選択と非上書き生成を担当します。

## Knowledgeと配置

選択Spaceの`knowledge/`全体をOKF文書として扱います。`codekb/`は現行のwhat/how、`adr/`は設計判断のwhy、
`design/<intent_id>/`は要件と実装計画、`rules/rule.md`は利用プロジェクトの共通ルールです。
差戻し理由は`log/<intent_id>-work-log.md`へ保存します。必要なADRを判断ごとに作り、typeは小文字`adr`、
新規文書の`intent_id`はその判断のIntentへ対応付けます。毎操作のKDR作成義務はありません。
一般Knowledgeのartifact区分とOKFの`type`は別で、内容に応じたtypeを保持します。
`okf`と`okfmemory`は固定OKF v0.2のmetadataを検査・保持・検索します。日時とfrontmatterはmemory CLIで生成します。
外部依存は承認済み`go.yaml.in/yaml/v3 v3.0.5`です。

配置原稿は`src/core`、`src/core/workflow`、`src/harness/codex`です。
installがfresh projectへ配置し、実行時は配置済みの定義・Rule・Skillを読みます。原稿へのfallbackはありません。
入口Skillは4 KiB以内、必須Rule本文は16 KiB以内とし、超過時に切り捨てません。

設計の経緯は[Intent実行計画](design/intent-execution-plan-implementation.md)と
[担当・作業場所管理](design/native-agent-assignment-plan.md)を参照してください。
[旧4段階契約](design/four-stage-workflow-contract.md)は後続決定で更新された履歴です。
除去した旧経路は[除去記録](design/four-stage-removed-product.md)に残し、旧利用dataを自動移行・削除しません。

検証集合はIntentの `verification_paths` と任意のUnit範囲で宣言し、Unit範囲はIntent範囲へ含めます。`scope` は編集担当の宣言です。集合の正規化したパス・種類・内容を二回読み、SHAの一致を確認します。Git呼出しは行いません。同じ相対配置と内容の別rootは同じSHAになります。

flow schema 6、assignment schema 2のみを新規保存します。旧commit fieldの互換読込みや移行は行いません。Knowledgeと結果は管理ディレクトリ内で別にhash化し、集合SHAとの循環を避けます。独立reviewerは別会話で同じrootを使え、別rootでは全体SHAを照合します。現在全体SHAの結果が終了条件であり、古いUnit成果だけでfinishできません。
