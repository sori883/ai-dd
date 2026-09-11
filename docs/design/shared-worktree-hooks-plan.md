# worktreeからAI-DLCのhookを使う実装計画

日付: 2026-09-11。基準main: `9360348cf0512d168b8604c85c24da322f186186`。
状態: ③への対応は直接依頼済み。下記の「管理元を一つに固定する利用条件」を確認してから実装する。

## 解決する問題と得られる結果

Git worktreeは、一つのリポジトリに追加する別の作業フォルダである。固定Codex CLI 0.153.4は、追加worktreeで会話を始めても、hookを元の作業フォルダ（主checkout）の `.codex` から読む。一方、現在のAI-DLCは、hookを置く場所と進捗を保存する場所を同じフォルダとしている。

そのため、追加worktreeにAI-DLCを配置しただけではhookが働かない場合がある。主checkoutへhookをコピーするだけでも、hookが確認する進捗とCLIが更新する進捗が別の場所になる可能性がある。

この計画では、hookの入口と進捗の保存先を明示的に結ぶ。メインAIを追加worktreeで動かしながら、ステージ、承認、担当の検査を同じ管理情報に対して実行できるようにする。CLIがエージェントを起動する仕組みには変更しない。

## 推奨する使い方と確認する選択

一組の主checkoutと追加worktreeに対し、AI-DLCの管理元を一つ選ぶ。以下では、主checkoutをH、管理元をC、workerの作業場所をWと呼ぶ。いずれも実在するフォルダであり、抽象的なファイル名ではない。

```text
/projects/example/                    H: Codexがhookを読む入口
  .codex/hooks.json                  → 管理元Cを明示して呼び出す
/projects/example-control/           C: メインAIを開始する追加worktree
  aidlc/spaces/                     → Knowledge・Intent・進捗
  aidlc/.runtime/                   → 会話と担当割当の管理情報
/projects/example-worker-a/          W1: workerの実装場所
/projects/example-worker-b/          W2: 別workerの実装場所
```

メインAIはCから会話を開始する。Cでは複数のSpace、Intent、会話を既存どおり管理できる。子の起動はメインAIのnative `spawn_agent`、workerの割当は既存の `assignment` CLIを使う。W1とW2での並列作業は維持する。CはHと同じ場所を選ぶこともできる。

新しい利用条件は、共有hookを有効にした組では、C以外から独立したメインAIを開始すると拒否してCでの開始を案内することである。別の場所の進捗をCへ自動的に取り込まない。既存の独立cloneは別の組として扱う。

もう一つの案は、同じリポジトリでC1、C2という複数の管理元を登録し、会話ごとに振り分ける方法である。この場合は登録・重複・解除・移転・復旧の新しい管理情報が必要になる。現在の「一つの管理元の中で全Intentを横断する」という契約からは、どちらを選ぶか一意に決まらない。

**確認する選択は、管理元を一つに固定する推奨案でよいか、同じリポジトリで複数の独立した管理元が必要か、の一点である。** 前者を選択した回答で、以下の具体的な導入・検証を含む実装を許可する。後者の場合は振分け契約を追加設計する。

## 実装許可と変更の境界

[③への直接依頼](../ram/decisions/2026-09-11-worktree-hook-implementation-request.md)が対応の許可である。③自体や、メインAIがnativeで担当を起動する既存方針への再承認は求めない。ただし、管理元を一つに固定し他の親会話を拒否する条件は、保存先と運用に関わる追加選択として確認する。旧33 Stageロードマップや①②の修正承認は根拠に流用しない。

Goの単一binary、標準ライブラリ、Sessionの6項目形式、assignment schema 1、既存の進捗stateを維持する。新しい振分け台帳、全操作audit、全書込み先の監視は追加しない。時間経過・結果提出・子のStopだけでworkerの割当を解放しない。hook対象外の経路やhook自体の故障を含む完全な遮断、子本人や全process終了の認証は保証しない。

作業場所は `/Users/const/sori883/ai-dd-worktree-hooks`、branchは `codex/worktree-hook-routing`。元の作業場所の未コミット資料と、完了したpilotの利用データを保持する。実装時は新しい `機能開発` Issueを作り、1 Issue/PRで進める。

## 導入・更新する操作

新しい管理元Cへは、従来どおり次を実行する。既設Cの製品更新は、[既存の配布手順](../distribution.md)の検証用配置と既知参照の移転を使う。共有入口の登録操作が既設資材を再インストールすることはない。

```sh
/opt/aidlc install codex --project-dir /projects/example-control
```

新しい明示操作は次の形にする。

```sh
/opt/aidlc install codex --shared-hooks \
  --project-dir /projects/example-control \
  --hooks-project-dir /projects/example
```

この操作はHの `.codex/hooks.json` に、現在の5種類の製品hookとCの対応を登録する。Cには新binaryに対応する既知の配布資材が必要で、古い・部分更新・独自編集を判別できない状態は登録前に診断する。Hのagent・skill・一般設定をCからコピーしない。HとCが異なる場合はC自身のhooks.jsonを変更せず、固定CodexではH側が読まれることをhelpへ明記する。H=Cの場合は、同じCとbinaryを指す既知の通常製品hookだけを共有modeへ変換し、利用者の無関係な設定を保持する。異なる管理先を指す既設の製品hookを暗黙に置き換えない。

H、Cは絶対path、実在、正規化後のGit top-levelを確認する。HがCと同じGit管理情報を持つ主checkoutであることをGitで調べ、remote URLや共有履歴だけで代用しない。bare repository、別のclone、主checkoutでない入口を拒否する。正当なsymlink別名は同じ実体として扱う。

登録前にHのJSONとinline hookの共存を確認する。利用者の無関係なhookは保持するが、製品hookの重複や識別できない編集、inlineとの重複を安全に判断できない設定は保存前に拒否する。解析できない設定を上書きして解決しない。複数の製品handlerを登録して管理先の選別を試みない。

同じH・C・binaryでの再実行は成功としてよい。別Cへの暗黙の差替えは拒否する。binaryだけを更新する場合は、同じ操作へ `--from-binary /opt/aidlc-old` を加え、旧参照の完全一致と、新旧いずれかの状態からの同一再試行を検査する。`--relocate` 等の通常配置操作との混用は拒否する。新flagと受け付ける値・エラー・具体例をhelpへ記載する。

H/C自体の移動、削除、別実体への変更は、別rootへ自動追従しない。特にassignmentは保存したRootとの一致を要求するため、共有hookだけ差し替えて担当情報を移行したことにしない。場所の変更が必要なら、既知処理の終了・成果回収・記録保全・既存の移転と担当復旧を先に行う。続いて旧共有製品entryを保管して手動で撤去し、新しいH/Cで新規登録する。独自hookや未知編集があれば全ファイルを上書きせず、製品部分の差分を確認する。この保守手順は管理情報の自動移行や、動作中の管理先を切り替えるAPIではない。

## hookとCLIの保存先を揃える

共有hookのcommandには、従来の `--project-dir C` と `--hooks-project-dir H` を埋め込む。非公開の `__minimal-hook` 入口では後者の指定が共有modeを表す。新しいhook分岐は、payloadの `cwd` とH/Cの関係を検査してから、既存の `minimal.Service` へ渡す。

`cwd` は会話の基準ディレクトリであり、workerが個別のBashで使う `workdir` ではない。これをworkerの本人や全編集場所の証明にしない。親がCから開始し、native子が同じ基準を継承することを固定実機で確認する。

共有modeのSessionStartはCを明示し、製品CLIに `--project-dir C` を付ける手順を伝える。認識できる製品CLIでは、root省略・相違を管理処理前に拒否する。現在のcommand解析の範囲を明記し、任意のshell構文や別プログラムを含む全書込みを監視できるとは説明しない。共有mode以外の通常hook/CLIの挙動は維持する。

cwd欠落、不正、C以外の会話、H/Cの対応不一致では、どの管理先のsession・進捗・assignmentも変更しない。PreToolUseには構造化されたdenyを返す。ほかのeventは既存の停止・診断応答に合わせ、記録の作成や自動recoverは行わない。親Cと子の境界が実機で成立しなければ実装完了にせず、原因と必要な追加契約を示す。

bootstrapは既存の4KiB上限を維持する。必要な案内をCLI Skillとhelpへ分け、長いbinary/C pathでも実際のSessionStartを検査する。上限を超える条件は導入前に診断し、実行開始後に突然切り捨てない。

## 単独writerと対象ファイル

`work_unit_id=shared-worktree-hooks`。Go実装担当1名が下表の全体を所有し、各項目のRED→GREEN→refactorを順に実行する。親・計画担当・調査担当・reviewerは同じ作業ツリーを同時編集しない。

| 対象 | 変更する役割 |
| --- | --- |
| `src/internal/install/shared_hooks.go`、対応test、`install.go`、`relocate.go` | 共通入口の登録、既知hook識別、更新・同一再試行、通常配置の維持 |
| `src/internal/workspace/hookroot.go`とtest | H/CのGit関係・実在・正規化を共有して検査する内部関数 |
| `src/internal/cli/minimal.go`、`help.go`、`cli.go`とtest | 明示操作とflag、混用拒否、値・使用例・復旧案内 |
| `src/cmd/aidlc/minimal.go`とtest | 管理処理前の共有入口検査、CLIへの接続 |
| `src/internal/minimal/session.go`、`hook.go`、`command.go`、`child_hook.go`とtest | cwd、共有mode、bootstrap、製品CLIの管理先検査、既存担当境界の保持 |
| `src/harness/codex/minimal/SKILL.md`、`aidlc-cli/SKILL.md` | 管理元を明示する操作とhelp参照、workerの作業場所との区別 |
| `src/cmd/aidlc/worktree_hook_integration_test.go`、対応するtest-only helper | 実Git構成、公開CLI、固定Codexの実発火・拒否・保存先の検証 |
| `src/docs/user-guide.md`、`docs/distribution.md`、`docs/development.md`、計画とRAM索引 | 導入、更新、通常trust、復旧、制限と証拠 |

既存の公開型や保存schemaを変更する必要が判明した場合は、内部関数の配置変更と区別して計画を再確認する。標準ライブラリで対応し、外部Go moduleは追加しない。

## 順序付きTDDと受け入れ条件

各commandのtest名は新設予定である。既存実装で失敗する、実行可能な回帰を先に作る。必要な型や未実装errorだけのscaffoldは許可するが、判定・保存の先行実装はしない。

| 順 | 受け入れる結果と拒否する対照 | targeted command |
| --- | --- | --- |
| 1 | 正しいH/C、C=H、symlink別名は対応を確認。別family・非top-level・欠落・移動を拒否 | `go test -count=1 ./src/internal/workspace ./src/internal/install -run '^TestSharedHookRoot'` |
| 2 | 新規登録・同一retry・利用者hook保持・H=Cの既知通常hook変換。別C・重複・未知編集・inline競合・保存失敗では誤更新なし | `go test -count=1 ./src/internal/install -run '^TestSharedHookInstall'` |
| 3 | 新flagとhelp例、混用・引数不足を検査し、通常install/relocateを維持 | `go test -count=1 ./src/internal/cli -run '^TestSharedHookCLI'` |
| 4 | 管理処理前にcwd/H/Cを検査。不正時はCと他root双方の保存bytesが不変。通常hookは従来どおり | `go test -count=1 ./src/internal/minimal ./src/cmd/aidlc -run '^TestSharedHookDispatch'` |
| 5 | Cの案内、CLI省略/相違拒否、4KiB予算、親子報告、担当予約・同一root競合・別root許可を保持 | `go test -count=1 ./src/internal/minimal ./src/internal/install -run '^TestSharedHookManagement'` |
| 6 | 同じH/Cのbinary参照更新、旧版との完全一致、途中失敗後の同一retry、rollback手順 | `go test -count=1 ./src/internal/install ./src/internal/cli -run '^TestSharedHookUpdate'` |
| 7 | 実機証拠の検査は未列挙・未発火・別root保存・拒否欠落を成功にせず、実際の親子途中報告と区別 | `go test -count=1 ./src/cmd/aidlc -run '^TestSharedHookEvidence'` |

`loop`は以上と影響する既存のtargeted回帰、変更Goへのgofmt、差分確認だけを実行する。親は作業単位末尾に全差分とtargeted群を一度確認する。独立した `review` は保存先の混同、既存hookの保全、通常modeの互換性、子の資格と保存失敗を点検し、必要なtargeted再現に限定する。

## 固定Codexで確認する条件

専用試験場所は `/Users/const/sori883/ai-dd-validation/worktree-hooks/` 配下に、新しく主checkoutH、追加worktreeC、W1、W2を作る。既存の利用プロジェクトや旧pilotへ製品hookを導入しない。固定Codex CLI 0.153.4、macOS arm64、gpt-6-astra/xhighを用い、binary・Go版・配置・hash・通常trustの状態を記録する。

この計画の実施には、空の専用環境の作成と、既存workerや割当がないことを確認したうえでの初回担当管理の初期化を含める。実案件の成果承認を代行する許可や、不明なworkerの割当を解放する許可には転用しない。通常のCodex信頼UIを通し、trust台帳の直接編集・迂回・Codex更新は行わない。

実装の順序は、承認後の前提確認、前提成立後のGo work unit、完成版の独立reviewとfinalである。最初の実機gateは新しい共有登録操作に依存させない。現行mainのbinaryと配布資材を空のCへ配置し、隔離Hには同じbinaryの固定 `--project-dir C` を持つ製品hookを試験用に配置する。このGit外の観測準備は完成した新modeの配布成功とは数えず、製品の管理判定を変更しない。

そのうえで、Cからのhook列挙元がHであること、通常trust後に本物の製品hookが発火することを確かめる。親子のsession/agent/cwdを観測し、Cを使う前提に反する場合はGo work unitへ進まない。複数handlerを重ねたり、自然言語からrootを推測したりして補わない。成立時は順序付きTDDの全項目を一つのwork unitとして渡す。

実装後は、Cで正常操作、承認待ちの禁止操作、担当違い、C以外からの親開始、CLIのroot省略・相違を対照確認する。子の途中報告、W1/W2の有限な並列作業、同じWへの二重割当拒否、終了確認後の明示解放を確認する。Cの一つのregistryへ集約し、H/W側に管理情報が誤作成されないことをbytes/hashで確認する。

観測wrapperは製品の入出力と終了codeを変更しない。最後にwrapperなしの通常配置でも許可・拒否・保存先を確認する。各nativeケースは5分以内の有限処理とし、timeoutは成功や停止済みの証拠にしない。内部推論や暗号文をGitへ複製せず、RAMには判断・証拠の場所・必要なhashだけを記録する。

## final、配布、復旧

全実装とblocking findingの修正後に、親が固定headのread-only `final` を一回開始する。変更後に古い結果を流用しない。

```sh
go test -shuffle=on ./...
go test -race -shuffle=on ./...
go vet ./...
go mod tidy -diff
gofmt -l src
git diff --check
go test -tags=integration -count=1 ./src/internal/workspace ./src/internal/okf
go test -tags=integration -count=1 ./src/cmd/aidlc -run '^(TestFlowJourney|TestSharedHookJourney)$'
go test -tags=integration -count=1 ./src/cmd/aidlc-dist -run '^TestDistributionJourney$'
```

実機の評価入口は新規test-only helperのhelp/envに定義し、必要な証拠の欠落・skipを合格にしない。PRで起動するGo2版、6種類のOS/CPU build・archive、3OSの配布検査も成功を確認する。独立review・final・現headのchecks後に通常運用でmergeし、main反映とIssue closeを確認する。

利用者は既知処理を終了させてからbinaryと対応資材を更新する。共有hookのcommandが変わるため、H側で通常の再信頼が必要になる。信頼前の完全遮断を保証しない。導入前のHのhookとCの資材を保管し、rollbackでは対応する組へ戻す。独自hookが後から変わっていれば全体の古いcopyで上書きせず、製品部分の差分を確認する。進捗やruntimeの削除を復旧の代用にしない。

## 本家との確認範囲

本家AI-DLCの参照はローカル固定2.6.123。確認済みなのはメインAIによる担当起動、Unitごとのworktree、Codex向けhook配布である。本家が今回の単一共有入口と親会話の開始場所制限を同じ形で持つことは未確認であり、「差分なし」とはしない。

今回提案する挙動は、固定Codexが読むHから一つのCへ接続し、C以外の親会話を拒否するもの。変更理由は、hook未読込と保存先の混同を防ぐためである。通常配置は維持するが、共有modeを選ぶ利用者には開始場所と明示管理先の指定が必要になる。この利用条件を計画の追加確認対象とする。最新upstreamや他ハーネス全体への一致は主張しない。

根拠: [固定Codex調査](../ram/decisions/2026-09-11-fixed-codex-hook-reliability-contract.md)、[前回の対応案](../ram/decisions/2026-09-11-worktree-hook-followup-options.md)、[既存の担当管理契約](native-agent-assignment-plan.md)、[固定本家の担当とworktree](../ram/decisions/2026-09-10-upstream-agent-dispatch-and-worktree-reference.md)。
