# 0.1.1で導入・開発・知識管理のCLIを分ける計画

## 目的と現在地

AI-DDは、AIと対話しながら開発の目的、計画、実装、検証を進める製品である。
現在の`aidlc`は、日常の開発管理に加えて、プロジェクトへの導入と知識文書の操作も受け持つ。
利用者が導入用と日常用を区別できるように、0.1.1では次の五つの実行ファイルへ分ける。

| 実行ファイル | 使う場面 |
| --- | --- |
| `aidlc-install` | 利用者がプロジェクトへ導入する |
| `aidlc` | AIがIntent、工程、進捗、センサー、承認、担当、hookを操作する |
| `okf` | AIがKnowledge、ADR、Rule、作業記録を検索・作成・更新・検証する |
| `natural-japanese-go` | AIまたは利用者が日本語文章を検査する |
| `aidlc-dist` | 開発者が公開する圧縮ファイルと照合情報を作る |

Intentは一つの目的を持つ作業単位、hookはAIの操作の前後に確認処理を呼び出す接続である。
各CLIはGoの一つの実行ファイルにする。`okf`は本プロジェクトの既存Go実装から切り出す。
本体のセンサー等が利用するOKF処理は共通packageを再利用し、実装を複製しない。

基準はmainへmerge済みのPR #199、commit
`64833d7e94203be0284c18abbf4a35b23810cf77`。
作業場所は`/Users/const/sori883/ai-dd-release`、branchは`codex/release-0-1-1`。
元の`/Users/const/sori883/ai-dd`の未commit変更は本作業の対象ではない。

## 実装許可と採用した選択

ユーザーは五つの名前を指定し、その変更を0.1.1へ含めることを直接依頼した。
[許可の記録](../ram/decisions/2026-09-13-five-cli-release-011-scope.md)を根拠とする。
旧33 Stageロードマップから許可を流用しない。
対象はCodex向けの新規導入であり、共通手順の一箇所管理、既存の承認・担当管理、Git不要の運用を維持する。
旧配置の互換・移行、Claude・Copilotの再開、新しい外部Go moduleの追加は含めない。

推奨案と3点の確認を提示した後、ユーザーは「はい、実施してください」と回答した。
[直接承認](../ram/decisions/2026-09-13-five-cli-release-011-approved.md)により、次の推奨案を採用して
実装・検証・Issue・PR・merge・0.1.1公開まで進める。版選択、配置、独自ライセンスの回答待ちは解消した。

| 選択 | 採用案 | 採用しなかった案と影響 |
| --- | --- | --- |
| 版の選択 | インストーラーの引数で指定した版のCLIと資材を取得する | 自身と同じ版だけを導入すれば処理は簡単になるが、版ごとにインストーラーを取得する必要がある |
| 実行ファイルの配置 | プロジェクト内の`aidlc/bin/v0.1.1/` | ユーザー共通binでは容量を共有できるが、複数プロジェクトの版の扱いが変わる |
| 独自ライセンス | MIT、著作権者`2026 sori883` | 保留案は採用しない。外部由来の表示は置き換えない |

## 利用者から見える構成

導入例は次のとおり。`--version`はインストーラー自身の版を表示し、
導入対象を選ぶ引数`--release-version`と区別する。

```sh
aidlc-install codex --release-version v0.1.1 --project-dir /path/to/project
```

```text
project/
├── .codex/                     Codex固有の設定・担当・hook
├── .agents/skills/             共通原稿から配置した手順
├── aidlc/
│   ├── bin/v0.1.1/
│   │   ├── aidlc
│   │   ├── okf
│   │   └── natural-japanese-go
│   ├── workflow/               工程定義
│   └── spaces/                 Intentと知識
└── src/ または各アプリのフォルダ
```

Windowsでは実行ファイル名に`.exe`が付く。
`aidlc-install`は利用者が取得する入口で、`aidlc-dist`を利用先へ置く必要はない。
Codex本体をインストールする機能ではない。
導入後のhook・skillには利用する各CLIのパスを設定するため、日常の利用でPATH設定を必須にしない。

`aidlc install codex`は`aidlc-install codex`へ、`aidlc memory search`等は
`okf search`等へ移す。`okf`は`rules`、`search`、`show`、`check`、`create`、`update`を持ち、
Space・Intent指定、本文入力、日時の自動設定、更新時の内容照合を維持する。
各CLIのhelpに引数、入力できる値、例、版表示を揃える。

## 内部処理と配布の契約

### CLIとhook

公開CLIの入口を分離し、ルート探索やファイル入力の安全な読込みは共通処理を利用する。
OKFのコマンド解釈を`src/internal/okfcli/`、操作サービスを`src/internal/okfapp/`へ分け、
公開CLIとhookは同じコマンド契約を使う。`okf __hook`や旧`aidlc memory`を別の入口から受理しない。
`aidlc`の内部でOKFを読む必要がある場合に、`okf`プロセスの起動を必須にしない。
`okf`への切り出しでも、文書の排他保存、更新前の内容照合、保存後の索引更新失敗の報告を維持する。

hookは現在`aidlc`の実行ファイルを照合している。新構成では配置時に決めた`aidlc`と`okf`を区別し、
それぞれのCLIとして引数を解釈する。`okf`というファイル名だけで例外を許可しない。
承認待ち中の読取り、必要文書の修復、子担当の権限、同時workerの制約を維持する。
Rule本文とhashは既存のbindFlowで返し、RuleHash／RuleTurnを保存して次のPreで照合する。
Postは通常toolの一致IDでslotを解放し、読取り例外はslotを消費しない。
新たな`okf rules`のPost出力認証や証拠stateは追加しない。
installerやpackagerを日常操作の許可例外へ加えない。

共通原稿は`src/core/skills/`、`src/core/agents/`、`src/core/workflow/`に置き、
`src/harness/codex/`にはCodex固有の接続だけを置く。
原稿へ各CLI用の置換項目を追加し、未置換の項目が残る資材は配置前に拒否する。
外部由来skillの原典名・作者・出典・LICENSEを保持する。

### 指定版を取得する場合

取得先は`sori883/ai-dd`のGitHub Releasesに固定する。
指定版の3つの日常用CLIと、同じ版・source commitの共通原稿／Codex資材を取得する。
資材をデータとして梱包し、インストーラー側の内蔵資材と混ぜない。
資材形式には版を付け、インストーラーが対応しない形式を読み込まず更新を案内する。
将来のすべての版を古いインストーラーで扱えるとは保証しない。

初版の資材形式はschema 1とし、専用manifestにschema、製品版、source commit、
archiveの名前・SHA-256・サイズ、収録ファイルのpath・SHA-256・サイズを記録する。
共通原稿とCodex原稿だけを収録し、利用先のstate・runtime・任意の実行ファイルを入れない。
`codex.Distribution`を明示した共通FS・host FSから生成できるようにし、
埋込資材と取得資材で同じ生成処理を使う。未知のtemplate機能や収録pathも拒否する。

ファイル取得にはGo標準ライブラリを使う。取得の失敗、容量上限、不正な版やtarget、
照合値・サイズの不一致、資材の欠落・重複、危険な展開先、symlinkを配置前に検出する。
選択した版と違う版へ自動で切り替えない。照合値は配布ファイルの整合確認に使い、署名と同一視しない。
公開前の同一候補検証とオフライン導入には`--release-dir DIR`で取得済み候補を渡せるようにし、
通信取得時と同じ版・内容・資材の検査を通す。任意URLや照合省略の引数は追加しない。

ダウンロードと検査は一時場所で完了させ、全配置先を検査してから書き込む。
既存ファイルを上書きしない。途中失敗時は今回作成したものと残ったものを明示し、
未知のファイルを削除したり、部分配置を成功扱いしたりしない。
同じ導入先への同時実行を拒否し、次回に既存資材を黙って使い回さない。

自身と同版だけを導入する代案は採用せず、内蔵資材への自動fallbackも行わない。
配置済み資材の移転は既存`relocate`の検査を引き継ぎ、複数のCLIパスを更新する。
自動更新や既存ユーザーデータの移行は追加しない。

### 公開物とライセンス

5製品をmacOS・Linux・Windows、それぞれamd64・arm64向けにbuildする。
製品ごとの圧縮ファイルとmetadataを同じReleaseへ添付し、異なる製品の
`manifest.json`や`SHA256SUMS`が衝突しない名前にする。
指定版取得案ではOS非依存の資材archiveと、その版・内容の照合情報も添付する。
添付数と名前の正確な集合はfixtureへ固定し、余計なファイルの公開を拒否する。
指定版取得案では5製品それぞれの6archiveとmetadata 2件で40件、
資材archive・専用manifest・checksumの3件を加えた43件を計画する。
`aidlc`のmetadata名は既存を維持し、他製品は製品名を接頭辞として付ける。
checksumの行はファイル名で整列し、binary用manifest schema 1と専用資材manifestを区別する。

独自部分のMITライセンスはルート`LICENSE`を正本として置く。
既存skillの許諾表示、YAMLのLICENSE・NOTICE・Apache本文、Goと必要な依存の表示、
日本語チェックのKagome・辞書等の表示を製品別に梱包する。
実buildで使ったGo版と依存を確認し、過去の2製品だけの調査を5製品へ流用しない。
特にHTTP取得の導入で使われるGo内部vendorの表示を確認する。
梱包時の許諾入力欠落やGo版不一致は、公開候補の作成前に検出する。

## 変更対象と所有範囲

実装は一人のGo実装担当をwriterとし、親とreviewerは同時編集しない。
1 Issue／PRの`work_unit_id=five-cli-release-011`として次の範囲を渡す。
親はIssue、許可、レビュー、final、PR、merge、公開を管理する。
対応は[Issue #200](https://github.com/sori883/ai-dd/issues/200)（機能開発）。
取得側と梱包側で同じ形式を検査するために必要な共通packageは`src/internal/release/`へ置ける。
対象rootの共通探索を切り出す場合は`src/internal/projectroot/`へ置き、現行の曖昧root拒否を維持する。

| ファイル・package | 変更 |
| --- | --- |
| `src/cmd/aidlc-install/`（新規）、`src/internal/install/` | installer入口、版指定、取得、複数CLI配置、移転、失敗時の扱い |
| `src/cmd/okf/`、`src/internal/okfcli/`（新規） | OKF公開CLIとhelp、既存処理への接続 |
| `src/internal/okfapp/`（新規） | OKFの操作サービスを抽出し、CLIと本体から共用 |
| `src/cmd/aidlc/{main,command,project_root}.go`、`src/internal/cli/{command,help}.go` | installer・memory入口の移動、共通ルート探索の整理 |
| `src/internal/app/{command,hook,child_hook,session}.go`、`src/internal/okfmemory/` | OKF処理の共用、実行ファイルの照合、読み取り・修復例外 |
| `src/core/skills/`、`src/core/agents/`、`src/core/workflow/` | 分離後のコマンドへ共通手順を更新 |
| `src/harness/codex/{manifest,content}.go`と固有原稿 | 各CLIのパスを反映するCodex生成 |
| `src/cmd/aidlc-dist/{main,archive,manifest}.go`と関連test | 5製品、資材、許諾、名前が衝突しない公開候補 |
| `src/cmd/natural-japanese-go/main.go`と関連test | 必要な場合のみ版表示を揃える。文章の判定仕様は変更しない |
| 上記packageのtest、`src/cmd/aidlc/*integration_test.go`、配置fixture | 新しい入口とパスによる既存の受入検査 |
| `.github/workflows/{ci,distribution}.yml` | 5製品と資材のbuild、同じ候補の3OS検証、添付集合の照合 |
| `LICENSE`、配布許諾用資材 | 回答で確定した独自ライセンスと実依存の表示 |
| `README.md`、`src/docs/user-guide.md`、`docs/{distribution,architecture,developer-references-and-dependencies}.md` | 初心者向け導入例、開発者向け構成・公開手順・依存表 |
| 本計画、`docs/ram/` | 判断、TDD証拠、reviewとfinal、公開結果 |

## 順序付きTDDと受入条件

`verification_mode=loop`では、次の各項目の観測可能な振る舞いをtestで先に定義する。
testが実行できる状態で意図した失敗を確認し、最小実装、成功確認、整理を順に行う。
新しいtest名は計画上の予定名であり、現在存在・成功しているとは扱わない。
新APIについては、testを実行可能にする型・関数署名・空返値だけのscaffoldを許可する。
実装の事前追加は許可しない。既存コードの契約固定は初回GREENならそのまま記録する。
作業単位末尾は各sliceのtargeted群に加え、変更したpackageの通常test、変更Goのgofmt、
`git diff --check`を一度まとめて確認する。広域の検査はfinalへ残す。

| 順序 | 観測する振る舞い | 対象command |
| --- | --- | --- |
| 1 | 新CLIの引数・help・版表示、旧入口の撤去、ルートとSpace指定 | `go test -count=1 ./src/internal/cli ./src/internal/okfcli ./src/cmd/aidlc ./src/cmd/okf -run 'Test(FiveCLIContract|OKFCommand|OKFHelp|ProjectRoot)'` |
| 2 | OKFの日時・Intent検索・本文保存・更新競合・保存失敗の維持 | `go test -count=1 ./src/internal/okfcli ./src/internal/okfapp ./src/internal/okfmemory -run 'Test(OKFMemoryContract|OKFSaveFailure|OKFConcurrentUpdate)'` |
| 3 | 別バイナリのなりすまし拒否、親子の読取りと修復、承認待ち保護、既存Rule事前照合と一致IDのPost解放 | `go test -count=1 ./src/internal/app -run 'Test(HookSplitCLI|ChildHookSplitCLI|HookSplitCLIPost|HookRecoveryGuidance|AssignmentStage)'` |
| 4 | 全資材に正しい3CLIパス、未置換拒否、既存配置保全・移転 | `go test -count=1 ./src/harness/codex ./src/internal/install -run 'Test(SplitCLIDistribution|SplitCLIRelocation|SplitCLIInstallConflict)'` |
| 5 | 5製品・許諾・資材の正確な梱包、metadata名と照合、欠落と部分保存 | `go test -count=1 ./src/cmd/aidlc-dist -run 'Test(FiveProductArchive|FiveProductManifest|ReleaseLicenseInputs|VersionedAssets)'` |
| 6 | 指定版取得、異版・不正入力拒否、通信／保存失敗、同時導入拒否 | `go test -count=1 ./src/cmd/aidlc-install ./src/internal/install -run 'Test(InstallerCommand|ReleaseDownload|ReleaseAssetValidation|InstallReservation|InstallFailure)'` |
| 7 | 新しい入口へjourney・CI・利用案内を接続 | 関連する前記test、`git diff --check`。実候補をbuildする検証はfinalへ集約する |

取得の小testはGo標準の`httptest`で制御した応答を使い、公開Releaseへ依存しない。
各commandが想定testを実行したことを確認し、「該当testなし」を成功証拠にしない。
各項目のRED／GREENを作業単位末尾へまとめ、親は末尾で対象test群と差分を一度確認する。

独立担当の`verification_mode=review`では、許可範囲、CLIの分離、hookの例外、
通信・展開・保存の失敗、原稿の一箇所管理、許諾表示、同じ公開候補を試験することを確認する。
必要な再現testだけを実行し、全体検証を繰り返さない。

差分とblocking findingの修正が安定してから、親がread-onlyの`verification_mode=final`を開始する。

- `go test -shuffle=on ./...`、`go test -race -shuffle=on ./...`、`go vet ./...`。
- `go mod tidy -diff`、`gofmt -l src`、`git diff --check`。
- `go test -tags=integration -count=1 ./src/internal/workspace ./src/internal/okf`。
- `go test -tags=integration -count=1 ./src/cmd/aidlc -run '^Test(FlowJourney|GitIndependentJourney|IntentDocumentsJourney|AssignmentJourney)$'`。
- 5製品を6target向けにbuildし、同じ版・commit・Go版の候補を梱包・照合する。
- `TestReleaseCandidateMetadata`と`TestReleaseCandidateNative`を新しい候補集合へ接続する。
  環境変数で候補と期待版を渡し、実候補を再buildせずに起動する。
- macOS・Linux・WindowsのCIで、通常フォルダへの新規導入、help・版表示、Space作成、
  `okf`文書の作成・検索、既存配置の保全、失敗時の結果を確認する。
- 分離後に配置した資材で固定Codex 0.153.4の実機確認を行う。通常の信頼設定を使い、
  OKF読取り・書込み、承認待ち、禁止担当の起動拒否、workerの重複拒否を確認する。
  過去版の実機成功を今回の成功として扱わない。

最終確認中は対象ファイルを変更しない。変更が必要になった場合はloopへ戻り、
再び安定した差分でfinalを実施する。追加の有料環境や認証情報が必要なら、成功と見なさず具体的な不足を報告する。

## mergeと公開の順序

1. 採用案を反映した本計画と承認RAMを根拠に、日本語の機能開発Issueを作る。
2. 単独writerのTDD、独立review、read-only finalを完了する。
3. Issueに紐づくPRを作成し、対象headの必須checks成功を確認してmainへmergeする。
4. 公開対象のcommitを固定して`v0.1.1`を作成する。既存tag・Releaseがあれば上書きせず状態を確認する。
5. そのtagからbuildした同じ候補を3OSで検証し、照合済みの全添付をRelease下書きへ渡す。
6. 下書きの版・commit・添付集合・照合値・許諾表示・日本語導入説明を確認し、ユーザーの0.1.1公開依頼に基づき公開する。

取得側の本番通信は公開後にも確認する。公開前は同じ候補をfixture経由で取得する検証と区別する。
通信や保存に失敗した候補を配布しない。作成途中の下書きや添付を自動上書きせず、残った状態を確認する。
実装の修復は範囲内で行う。公開済みの版の内容差し替えや利用者データの削除をrollback手段にしない。

## 本家との差分と根拠の範囲

固定参照のAI-DLC 2.6.123は、共通資材と環境別資材を組み合わせる配布構成として確認済みである。
本計画でも共通原稿とCodexアダプターを分ける。
五つのGo CLIとGitHub Releasesからの版別導入は本プロジェクトの指定であり、
本家が同じ五つの実行ファイルを提供するという説明はしない。
変更理由は導入・開発管理・知識管理の役割を分けるためで、利用者は導入と日常のコマンドを使い分ける。
原典OKF CLIとの完全互換、本家の最新配布処理全体との一致は未確認である。

現行実装の根拠は`src/cmd/aidlc/command.go`、`src/internal/app/command.go`、
`src/internal/app/hook.go`、`src/internal/install/`、`src/harness/codex/`、
`src/cmd/aidlc-dist/`、`.github/workflows/distribution.yml`。
以前の[単一CLI配布計画](github-release-pipeline-plan.md)の候補検証と公開前照合を再利用し、
単一CLIという契約は本計画で置き換える。過去の計画と証拠は削除しない。

## 完了条件の補足

Issue #200は実装とv0.1.1公開を含む。PRは`Refs #200`で関連付け、merge時に自動closeしない。親がRelease公開と確認を終えた後にIssueをcloseする。GitHub操作は親担当である。

## 実装時の検証接続

原稿・設定の変更に伴う旧test期待は新入口へ移す。旧runtime journeyの配置setupは内部install関数のfixtureとし、実Release CLIの検査は同じ43資材を使うReleaseCandidateNativeへ集約する。後者は五CLIの版・help、指定版導入、知識作成・検索、自然文検査、再配置拒否と同版移転を行い、再buildしたbinaryで代用しない。loopではtagged observer/metadataの小testだけ実行する。exact commandとRED/GREENは[loop RAM](../ram/decisions/2026-09-13-five-cli-release-011-loop.md)を参照する。

公開buildをGo 1.26.4へ固定して実GOROOTの許諾入力を照合する。品質CIのstable matrixは保持する。梱包は全六target必須で、部分target集合を公開候補として成功させない。

## 独立reviewの修復単位

work_unit_id `five-cli-release-011-review-fixes`として、runtime許諾の保存と移転照合、同commit正本による公開候補の許諾集合・bytes・mode照合、observerのOKF絶対path転送、draft取得案内、日常操作の役割別helpを順に修復する。既存承認内の欠落修復であり、schema・旧互換・Rule出力認証は拡張しない。所有file・exact targeted command・RED/GREENは[修復RAM](../ram/decisions/2026-09-14-five-cli-release-011-review-fixes.md)に記録する。

## 実候補Nativeのroot探索修復

final-01で祖先の通常aidlc binaryによるENOTDIRを確認したため、親がfinalを終了し、共有projectroot.Resolveと回帰testの修復を単独writerへ委譲した。aidlc／workflowが通常fileなら候補外とし、本当の複数root拒否、permission/IOエラー、明示root挙動を維持する。実測された五CLIの依存説明も訂正し、許諾実装は縮小しない。所有・exact targeted command・RED/GREENは[修復RAM](../ram/decisions/2026-09-14-five-cli-native-root-fix.md)に記録する。
