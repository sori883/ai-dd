# 実案件で残ったhookの3課題を修正する計画

日付: 2026-09-11。状態: **①②の実施、③の調査を直接承認済み**。承認は[後続RAM](../ram/decisions/2026-09-11-hook-reliability-repair-approved.md)を参照。

AI-DLCのhookは、AIの操作前後に実行され、現在の工程で作業してよいか、人間の承認を待つ必要があるかを確認する仕組みです。実案件は最後まで完了しましたが、手動復旧や環境変更が必要でした。今回の目的は、その原因を調べ、製品で修正できる不具合を直すことです。

| 課題 | 利用者に起きること | 目指す結果 |
| --- | --- | --- |
| ① 終了済み操作が実行中として残る | 読取りや編集が終わっていても、次の操作を拒否される | 届いた正しい終了通知を保存競合で失わず、次の操作へ進める |
| ② 子から親への途中報告を拒否する | 調査担当・worker等が途中の発見や質問をメインAIへ返せない | 親宛と確認できる報告を許可し、共有stateや承認は変更しない |
| ③ linked worktreeでhookを読み込まない | hookファイルを配置しても保護が働くとは限らない | 読込みと実動作を確かめ、利用できる配置条件を明示する |

linked worktreeは、一つのGitリポジトリに追加する別の作業ディレクトリです。独立cloneはGitの管理情報も別に持つコピーです。③では独立cloneへの変更で試験を続けましたが、worktreeの問題を修正したわけではありません。

## 現在地と許可範囲

調査基準は `sori883/ai-dd` のmain、`cf5631b6a26f7f307c70f7e8df3547594a4c1da6` です。開始時にOpen Issue・PRは各0件。PR #164で実案件、#166で配布、#168で利用者ガイドがマージ済みです。

計画作成後、ユーザーは「1番と2番は実施してほしいです。3番はまあ調査してください」と回答しました。①②は原因確認・修正・検証・既存のIssue／PR運用まで、③は調査と対応案の記録までを直接承認された範囲とします。過去の担当管理やパイロットの承認を、今回の承認根拠へ流用しません。

最初にG0（原因の切り分け）を行います。既存の公開入力・保存形式・権限境界を保った①②の修正は、根拠を計画へ反映して追加承認を待たず進めます。親子対応の新しい永続記録、権限の拡大、Codexの版や配置の変更が必要なら、その具体的な選択を確認します。③の調査許可は製品や利用環境を変更する許可ではありません。

計画の作業場所は `/Users/const/sori883/ai-dd-hook-reliability-plan`、branchは `codex/hook-reliability-plan` です。元の `/Users/const/sori883/ai-dd` の未コミット資料と、完了した実案件のデータは保持します。

## 確認済みの事実と未確定事項

①の一時記録 `Session.Tool` は、会話内で実行中の操作IDを一つ持ちます。`session.go` の `withSession` は保存用ロックを一度取得し、競合したら直ちにエラーを返します。`hook.go` は操作後の通知 `PostToolUse` のIDが保持中IDと一致した場合だけ解除・保存します。**終了通知が到着しても、その処理がロック競合で失敗する経路はコードにあります**。実案件の原因がこれだったかは未確定です。

実案件のコマンド `exec-533e006c-a711-4526-a3de-437af25571e3` には、終了code 0の完了証拠と、実行中記録の残存証拠があります。しかし、終了通知の未到着と、到着後の保存失敗を区別できません。失敗した編集にも同じ原因を当てはめません。

②の `child_hook.go` は、子の操作について `Bash` と `apply_patch` 以外を拒否します。`send_message` もこの拒否に入ります。親側では同じ名前のツールを「子への追加依頼」として扱うため、親用処理へ通すだけの変更では不十分です。子のhookに届く `session_id` は親と共有され、それだけでは親宛かを判断できません。

③は固定Codex CLI **0.153.4 / macOS arm64** で、linked worktreeでは製品の5種類のhookが一覧に出ず、同じパス・資材を独立cloneへ移すと列挙・信頼確認できた実測があります。Goの配置処理、Codexの探索方法、設定条件のどれが原因かは未確定です。

現行の[公式hook仕様](https://learn.chatgpt.com/docs/hooks)には、Bashの非ゼロ終了や後続pollによる `PostToolUse` の説明があります。調査日の説明が固定0.153.4にも当てはまるとは断定しません。Context7の公式Codex資料でも現行schemaは取得できましたが、固定版の到着保証は確認できませんでした。[終了通知欠落の報告 #16246](https://github.com/openai/codex/issues/16246) と [worktree探索の報告 #23996](https://github.com/openai/codex/issues/23996) は手掛かりです。報告の存在やclose状態だけで、今回の原因・修正版を確定しません。

## G0：修正前に通知経路を確かめる

専用の試験環境で、入力と結果を対応付けます。比較条件はCodex CLI 0.153.4、macOS arm64、モデル `gpt-6-astra`、推論設定 `xhigh` とします。binaryのパス・hash、OS、モデル・推論設定、Go版、製品commit、配置・信頼状態を記録します。条件を揃えられなければ同条件の再現とは呼ばず、変更を確認します。

試験用wrapperは実製品hookへ入力を渡し、出力と終了codeを変更せず返します。受信・返却時点、session・turn・tool ID、保存前後、ロック競合のエラーを試験側へ記録します。自作の許可・拒否応答を返すmockだけの試験と区別します。観測でタイミングが変わる可能性があるため、最後にwrapperを外した通常配置でも確認します。

| ケース | 実施・観測 | 判定 |
| --- | --- | --- |
| 単発コマンド | 正常終了と非ゼロ終了。同一IDのPre・実終端・Post・保存結果 | 正しい終端通知でToolが空になるか |
| 長いコマンド | running後にpollで正常・異常終了する有限コマンド | 起動応答を終了と誤認せず、実終端に対応する通知があるか |
| 同時操作 | 親の読取り2要求と、試験で順序を制御した保存ロック競合 | 後続要求が拒否されても、先行操作の終了処理を失わないか |
| 失敗した編集 | 存在しないcontextへのpatch等、内容を壊さない失敗 | Postの有無・ID・エラー形式。tool失敗とhook失敗を区別できるか |
| 子の途中報告 | native起動した許可担当から親への通知と、親での実受信 | hook入力の宛先と親の対応が一意か |
| 配置の比較 | 通常Git、linked worktree、独立cloneへ同じ資材を配置 | config・hook一覧のsourcePath・実発火・cwdが各環境の意図した配置先に対応し、通常trustが有効か |

実機は各ケース5分、全体25分を上限とする案です。時間切れは成功や終了確認にしません。再実行は原因を説明して証拠の保存先を分けます。既存パイロットは再開せず、新しい一時プロジェクトを使います。

子の対応はspawn入力・応答のtask名、子の `agent_id`、実際の `send_message.target`、親の受信を照合します。rolloutの親子metadataを使う場合は調査証拠に限定し、非公開の保存形式を製品の権限判定へ組み込みません。自然言語の依頼文から宛先や作業rootを推測しません。

信頼確認はCodexの通常の操作を通します。trust台帳の直接編集、信頼確認の迂回、主checkoutへの無断配置は行いません。hook一覧への表示だけで合格にせず、承認待ちの試験用書込みを実際に拒否することも確認します。過去のパイロット限定のAI承認委任は流用しません。

実機開始前にfixtureの絶対パス・配置内容と既知の割当状態を提示し、通常のhook信頼確認と初回の `assignment init` に必要な人間確認を済ませます。これは既存の管理開始契約に基づく確認で、テスト一件ごとの承認は求めません。信頼対象の内容が変われば新しい対象を示します。準備・確認待ちは実機25分の計測外とし、必要な確認が揃ってから計測を開始します。

各課題を「製品内部で再現」「固定Codexから必要な通知・識別情報がない」「未再現・証拠不足」に分けます。Post重複はそれだけでは失敗にせず、同じ通知を再処理しても壊れないかを確認します。未再現の課題は完了にしません。

G0の完了条件は、原因を判断できる証拠を揃えることです。既知の残存・拒否を正しく再現して証拠を採取できれば、そのケースは「G0の観測完了・製品は未修正」とします。実機テストの結果も観測の成否と製品挙動の合否を分けます。製品修正後のfinalでは、残存解消や途中報告の成功を受入条件にします。

## G0後の修正案

G0の製品直接呼出しで保存ロック競合を再現し、固定Codexのsourceから終端通知とcanonical task階層を確認した。[確認結果と具体契約](../ram/decisions/2026-09-11-fixed-codex-hook-reliability-contract.md)に基づいて①②を進める。過去の全残存原因や実機での修正成功は、これだけでは確定しない。実機未完でも根拠が揃った製品内部の回帰と修正は進め、正常trustでの実利用検証はfinalに残す。

### ① 届いた終了通知を保存する

Post到着後の一時的ロック競合が原因なら、hookのsession保存経路に有限の取得再試行を入れます。全用途の `filestore.Lock` を一律変更せず、対象を限定します。現在のhook timeoutは10秒。待機予算は2秒を初期案とし、残りを処理・応答に残します。ロックを奪取・削除せず、競合以外のエラーは待たずに返します。

解除条件は「保持中IDと一致する終了通知」を維持します。重複通知、古いID、別会話の通知で新しい操作を解除しません。保存失敗・待機期限切れでは書込み前の記録を保持し、復旧に必要な理由を返します。ロック解放自体の失敗も診断対象にします。

同一親会話の一般操作は引き続き一つずつ扱います。複数Tool集合への変更は新しい保存契約になるため含めません。別worktreeでの既存の並列workerは維持します。

終端通知が届かなければ、この修正だけでは直りません。別の終端イベントが実測できた場合に限り、その意味・hook登録・互換性を別途具体化します。通知がないまま経過時間やStopで自動解除する案は採用しません。

明示復旧 `session bind ID --space SPACE --session SESSION --recover` は、メインAIが該当操作の終了を確認した後、同じSpace・Intentで行う既存契約を維持します。IDは現在のIntent ID、SPACEとSESSIONは現在のSpace名と会話IDです。「編集失敗時だけ」と読める一部案内を実際の終了確認条件に揃えます。Toolの解除とworker割当の解放は別です。

### ② 親への途中報告だけを通す

G0で親宛を安定して区別できた場合、子専用の分岐で `send_message` を許可する案です。既存の現在工程・担当適格性の検査を残し、宛先が欠落、不明、親以外なら拒否します。工程が変わって担当資格を失った子の操作を新たに許可する変更は含めません。

報告によって親のTool・Rule読込記録、Intent state・履歴、Knowledge、承認回答、Unit、割当記録の全bytesが変わらないことを検証します。子のspawn・追加依頼・interrupt・兄弟宛通知・共有管理操作の拒否と、親から子への既存の追加依頼検査を維持します。

公開hook情報だけでは親宛を判別できない場合、②の実装を止めます。メインAIによる宛先登録や新しい親子対応表を追加するには、新たな保存・権限契約を提示して承認を得ます。最終回答の回収を、途中報告の成功証拠へ置き換えません。

固定版の追加確認で、既存bound Dispatch.Canonicalの親pathから宛先を導出できました。同一session・Space・Intent・step・定義版の適格記録を一回のregistry読取りから評価し、全roleで親pathが一意、同roleが最低一件あることを条件にします。workerの予約owner/context・未解放も照合します。Canonicalは厳密に検査してからpath.Dirを使います。初回の適格pendingだけは最大2秒の再読取りとし、ロックを保持せず、待機後のbinding・工程・資格を検査し直します。特定の子本人や解放済みの子をagent_idで認証する保証は加えません。

coreの最初にG0判定を固定仕様へ補正します。Bash非ゼロの終端statusはfailedでもPost対象、失敗patchのPostなしは期待される保持・明示復旧経路として分けます。配置後の起動・bind等の準備操作と実際の試験操作を分けて採点し、準備操作がToolを記録しないことを誤って製品不具合にしません。

追加TDDは `TestHookReliabilityProbeEvidence`（failed終端・不正なstatus/exit対応・試験対象の選別）、`TestChildReportBoundary`（階層親、複数role、曖昧な親、不正Canonical、別context、releasedのみ、pendingからbound、待機中の工程変更・release/reset）にまとめます。test-only観測の補正と①②の製品変更を次の一つのcore work unitへ渡し、同じ単独writerが順序付きで処理します。

### ③ 読込み問題の修正可否を確定する

製品の出力設定に誤りがあれば、設定と配置検査を修正します。固定Codexの探索問題なら、Go側で直せると説明せず、該当版・配置での制約を記載します。Codex更新が候補なら、具体版、公式の修正根拠、同じケースの比較実測を揃えてから提案します。

独立cloneは回避策であり、linked worktreeの修正完了ではありません。worktreeの自動変換、主checkoutへのhook配置、対応環境の独立cloneへの限定は採用済みとせず、必要になった時点で影響と戻し方を確認します。

## ファイルと単独writerの所有範囲

実装承認後は親がIssueと承認範囲を管理します。G0と製品変更は未解決gateで区切ります。①②は必要条件が揃えば一つのIssue／PR・一つのwork unitで扱い、項目数だけを理由に分割しません。③の環境変更は別の判断です。

同じ作業ツリーのwriterは常に一人。Go担当が変更している間、親やreviewerはファイルを編集しません。計画担当・技術調査担当・独立reviewerは読取り専用です。

| 単独Go writerの作業 | 変更候補 |
| --- | --- |
| G0の証拠検査と実製品wrapper | 新規 `src/cmd/aidlc/hook_reliability_probe_test.go`、`hook_reliability_probe_integration_test.go`。後者はintegration tagと明示envで実機を分離 |
| ①②の処理と回帰 | `src/internal/minimal/session.go`、`hook.go`、`child_hook.go`、`child_hook_test.go`、新規 `hook_reliability_test.go` |
| 既存の復旧・親管理・案内の保護 | `src/internal/minimal/flow_test.go`、`agent_hook_test.go`、`src/internal/install/install_test.go` |
| 利用手順の該当箇所 | `src/harness/codex/minimal/SKILL.md`、`aidlc-cli/SKILL.md`、`agents/aidlc-*.toml`、`src/docs/user-guide.md`、`docs/distribution.md` |
| 新イベント等の必要性をG0後に確定 | `src/internal/install/install.go`、`relocate.go`と各test、`src/internal/minimal/session.go` のhook入力 |
| 判断・検証結果の記録 | 本計画、`docs/ram/decisions/`、`docs/ram/README.md`。その時点の単独writerが更新 |

③の製品変更ファイルは原因判明後に確定します。新しい診断CLIや常設の稼働台帳は、現時点では提案しません。

## 順序付きTDDと検証分担

TDDは、変更前に失敗するテストを確認してから最小の修正を行う方式です。既に通る保護条件は回帰確認にし、REDを捏造しません。新test名は実装予定名で、今は存在せず未実行です。

| 順 | 観測する挙動 | loop command |
| --- | --- | --- |
| G0-1 | wrapperの出力・exit保持。証拠欠落、ID不一致、保存失敗を成功にしない | `go test -count=1 ./src/cmd/aidlc -run '^TestHookReliabilityProbeProtocol$'` |
| G0-2 | 終端・親宛・配置を別々に判定。未観測・最終回答だけを合格にしない | `go test -count=1 ./src/cmd/aidlc -run '^TestHookReliabilityProbeEvidence$'` |
| ①-1 | 一時競合後の一致Post処理、期限切れ・非競合エラーで保持 | `go test -count=1 ./src/internal/minimal -run '^TestHookSessionContention$'` |
| ①-2 | 一致終端、重複、古いID、失敗toolの実測通知、保存失敗・再試行、Stop、ロック解放失敗 | `go test -count=1 ./src/internal/minimal -run '^TestHookTerminalPersistence$'` |
| ①-3 | 復旧の終了確認案内、同一binding限定、操作重複拒否 | `go test -count=1 ./src/internal/minimal -run '^TestFlowHookSelectionRulesAndRecovery$'` と `go test -count=1 ./src/internal/install -run '^TestInstallRecoveryGuidanceAndContextLimit$'` |
| ②-1 | 適格な子の親宛報告を許可、共有data不変 | `go test -count=1 ./src/internal/minimal -run '^TestChildReportToParent$'` |
| ②-2 | 別宛先・識別欠落・再委譲・共有書込み拒否、親経路維持 | `go test -count=1 ./src/internal/minimal -run '^(TestChildReportBoundary|TestChildHookNotifications|TestChildHookEligibility|TestChildHookCommandBoundary|TestAssignmentDispatch)$'` |

競合試験はchannel等で取得・解放の順序を制御し、偶然のsleep成功に依存させません。実測で存在しなかった終了通知を合成し、固定Codexで修正できた証拠にはしません。

- `loop`: 単独Go担当が順序付きTDDと限定回帰を実施。G0は `hook-reliability-preflight`、製品修正は `hook-reliability-core` のwork_unit_id案です。親は作業単位末尾に全差分とtargeted群を一度確認します。
- `review`: 独立reviewerが許可範囲、通知ID、解除条件、親宛識別、共有記録不変、実測とmockの区別を確認。findingの再現に必要なtargeted testだけを行います。
- `final`: 修正とblocking finding対応が完了して差分が安定した後、親が一度開始。対象ソースを変更せず全体検査と実機を集約します。変更後に古いfinal証拠を使い回しません。

finalの基本command:

```sh
go test -shuffle=on ./...
go test -race -shuffle=on ./...
go vet ./...
go mod tidy -diff
gofmt -l src
git diff --check
go test -tags=integration -count=1 ./src/internal/workspace ./src/internal/okf
go test -tags=integration -count=1 ./src/cmd/aidlc -run '^TestFlowJourney$'
go test -tags=integration -count=1 ./src/cmd/aidlc-dist -run '^TestDistributionJourney$'
```

`gofmt -l` は出力が空であることを検査します。既存CIの6種類のOS/CPU build、archive検査、3OSの配布実行検査も対象PRで成功を確認します。配布検査と本物のCodexのhook検査は別です。

G0で新設する実機入口の予定command:

```sh
AIDLC_HOOK_RELIABILITY_LIVE=1 go test -tags=integration -count=1 -timeout=30m -v ./src/cmd/aidlc -run '^TestHookReliabilityProbeLive$'
```

実行条件不足は明記し、skipや信頼未完を合格にしません。G0の実機は原因調査の証拠です。製品変更後のfinalでは新しいbinaryで再検証し、通常配置でも①の次操作成功、②の親での途中報告受信、禁止操作の拒否を確認します。未実行の版・配置を対応済みとしません。

## 保存・復旧・配布

①の最小案では `aidlc/.runtime/flow/sessions/<session>.txt` の6項目形式、Intent state・履歴、assignment registryを維持します。保存ロックも `aidlc/.runtime/locks/session-<session>/` のままです。全操作audit、新工程state、複数Tool台帳は追加しません。Go標準ライブラリと既存基盤を使い、外部moduleを追加しません。

G0観測記録は製品の永続dataではなく、専用環境の一時証拠ディレクトリへ保存します。RAMには採否、必要最小限の結果、証拠の場所とhashを記録します。生prompt・認証情報・モデルの内部推論はGitやRAMへ保存しません。

終了・保存が不明なら自動復旧せず、終了確認後の既存recoverを使います。割当解放は別の明示操作です。Unitの結果提出、子のStop、時間経過ではworker停止済みとしません。

更新前に既知の操作を終了させ、旧binaryと対応する配置資材を保持します。fresh配置と既存配置を別に検査します。hookイベント・matcher・commandが変わる場合はinstallerとrelocate検査を揃え、利用者設定を保った差分と通常trustの再確認を示します。自動updaterは追加しません。

rollbackは旧binaryと対応する製品資材へ戻します。Rule、Knowledge、state、履歴、runtimeを削除して復旧したことにはしません。保存形式維持でも動作中のbinary差替えは避けます。新形式が必要なら、この互換・rollback前提が変わるため再計画します。

## 本家との関係と完了条件

本家AI-DLCの参照は固定 **2.6.123** です。確認済み範囲はメインAIによる担当起動、Unitのworktree利用、CLIでの割当です。現製品の工程別起動制限・管理root単位の予約等は[承認済み差分](../ram/decisions/2026-09-10-managed-worker-assignment-approved.md)を維持します。

session保存の再試行はGo実装内部の修正です。子の途中通知とlinked worktreeのhook探索について、本家が同一の境界を保証することは未確認です。「本家との差分なし」とは記載しません。新しい意図的差分が必要なら、本家の根拠、採用挙動、理由、利用者・互換性への影響を提示して承認を得ます。

完了は課題別に判定します。①は確認できた終了経路で不要な残存がなく、不明時の保護が残ること。②は途中通知が親に実際に届き、共有変更・再委譲を許可しないこと。③は対象配置で通常hookが実発火すること。Codex側の未解消制約は「制約確認・回避策あり」と記録し、「修正済み」にしません。

実装時はIssueを作成し、G0のfixtureは `ユーザーリクエスト`、製品挙動修正は `機能開発` のどちらか一つを主要成果に従って付けます。承認済み修正が独立review・read-only final・対象PRのchecksを通ったら既存運用に従いmergeし、main反映とIssue closeを確認します。G0のmergeは製品修正完了を意味しません。

根拠: [実案件の残件](../ram/decisions/2026-09-11-pilot-delegated-hook-evidence.md)、[hook探索の実測](../ram/decisions/2026-09-11-pilot-hook-discovery.md)、[子hookの分離契約](../ram/decisions/2026-09-10-native-child-hook-separation.md)、[担当・作業場所管理の承認](../ram/decisions/2026-09-10-native-agent-assignment-implementation-approved.md)。
