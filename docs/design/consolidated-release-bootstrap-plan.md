# 7個の配布物とコマンドによる自動導入

## 背景・目的・承認

AI-DDは、導入を担当する`aidlc-install`、作業を管理する`aidlc`、Knowledgeを扱う`okf`、
日本語を検査する`natural-japanese-go`、開発者が配布物を作る`aidlc-dist`の5つの実行ファイルを持つ。
公開済みv0.1.1では、これらのOS・CPU別archiveと確認用のファイルが合計43個あり、
利用者が最初に取得するファイルを選びにくい。

5つの実行ファイルは独立したまま、配布時だけ環境別の一式にまとめる。
添付はmacOS・Linux・Windowsの各amd64・arm64用の6個と、SHA-256確認表1個の合計7個にする。
利用者はコマンドを実行すると、自分の環境の一式が自動で選ばれ、指定プロジェクトへ配置される。
プロジェクトに残る日常用実行ファイルは`aidlc`、`okf`、`natural-japanese-go`の3つである。

ユーザーの「はい。お願いします。インストーラーがダウンロードするアセットを自動選択するの？」を、
直前の7個構成と自動導入の提案への直接承認とする。
[承認記録](../ram/decisions/2026-09-14-consolidated-release-assets-approved.md)に従い、実装、
独立レビュー、最終検証、PR checks、mainへのmergeまで進める。新版v0.1.2の公開可否は別途質問中で、
返答前にtag作成・Release公開は行わない。旧33 Stageの包括承認は根拠にしない。

基準はPR #201のmain `a3514ecc6ac1900a93e064e70aa1ce30efdf19e7`、
作業先は`/Users/const/sori883/ai-dd-release`、branchは`codex/consolidated-release-assets`。
対応Issueは[#202](https://github.com/sori883/ai-dd/issues/202)。
公開済みv0.1.1のtag・添付物は保持する。旧43形式を新installerで読む互換機能は作らない。
以前の初回取得だけを追加する案は検討履歴として残す。

## 利用者から見える動作

1. READMEのOSに合うコマンドを、導入先の既存ディレクトリで実行する。
2. 初回取得scriptがOS・実行中CPUを判定し、指定版の`SHA256SUMS`と1つの一式archiveを取得する。
3. SHA-256の一致を確認してから、一時ディレクトリへ`aidlc-install`だけを取り出して実行する。
4. Go製installerが元のarchive全体、版、環境、各ファイルを検証し、既存のCodex配置処理へ渡す。
5. `aidlc/bin/VERSION/`に3つのruntimeと必要な許諾文書が配置される。Codex設定・共通資材も既存方式で配置する。

初回scriptは取得済みarchiveを`--release-dir`で渡し、同じ一式を再取得しない。
既に`aidlc-install`を持つ人も、従来どおり`codex --release-version VERSION --project-dir ROOT`を使える。
その場合はGo自身がOS・CPUに合う一式を選ぶ。配布される5つのCLIを1つへ結合する変更ではない。

READMEは取得完了後に実行する短いコマンド列を示す。Unixでは`curl`でscriptを一時ファイルへ保存してから
`sh`で実行し、Windowsでは`Invoke-WebRequest -UseBasicParsing`で取得完了後にscriptblockを実行する。
scriptのURLは配布版のtagで固定する。版未公開時の例を、既に実行可能な公開手順とは説明しない。
初回script自体はリポジトリから取得し、Release添付を増やさない。

## 一式archiveと検証契約

名前は`ai-dd_VERSION_OS_ARCH.tar.gz`、Windowsのみ`.zip`とする。
各一式の直下に5binary、`manifest.json`、`core/`、`codex/`、`LICENSES/`と日本語検査CLIの説明を入れる。
既存の原典のライセンス本文・帰属表示・製品ごとの対応は維持する。
共通の手順の原稿は`src/core/`、Codex固有差分は`src/harness/codex/`から取得する。

内部manifestはschema 2で、次の情報を持つ。

```json
{
  "schema_version": 2,
  "version": "VERSION",
  "source_commit": "40桁の小文字16進数",
  "go_version": "go1.26.4",
  "target": "linux/amd64",
  "files": [
    {"path": "aidlc", "size": 123, "sha256": "64桁の小文字16進数", "mode": 493}
  ]
}
```

`files`はmanifest自身を除く全memberをpath順に記録する。5binaryは0755、その他は0644とする。
manifest自身を含むarchive全体のdigestは外側の`SHA256SUMS`が持ち、循環する自己hashを作らない。
JSONの重複・未知key・後続data、不正な版・commit・target・hash、旧schema、未知schemaを拒否する。
必要binary・原稿・許諾文書の不足、未知member、manifestとのサイズ・hash・mode不一致を拒否する。
archiveの絶対path、親参照、重複path、symlink、hardlink、特殊file、親子衝突も拒否する。

`SHA256SUMS`は当該版の6archiveだけを名前順で列挙した`HASH  NAME\n`の6行に固定する。
不足・余分・重複・不正hashを拒否する。offlineの`--release-dir`には全6archiveは不要で、
この6行の確認表と実行環境の1archiveだけを必要とする。offline指定時はnetworkへfallbackしない。

圧縮archiveと展開後合計は既存の512MiB制限を維持し、原稿は合計32MiB以内とする。
manifestは4MiB、checksum表は4KiBを上限とする。取得中にも上限を適用する。
installerは指定版と対応schemaを検証するが、自分自身のbuild版に取得版を固定しない。
これは既存の`--release-version`で版を選ぶ契約を維持するためである。
Release候補検査では別途、指定commit・Go版・各binaryのbuild情報との一致を検査する。

## 初回取得script

macOS/Linuxは既存のsh、curl、tar、mktemp、shasumまたはsha256sumを使う。
WindowsはPowerShell 5.1/7、OSにある`curl.exe`、Get-FileHash、標準.NETのZipArchiveを使う。
必要なコマンドがなければ理由を表示して終了し、toolを勝手に導入しない。
Unixの`uname -m`、WindowsのProcessArchitectureで実行環境を選ぶ。
Rosetta等では実行中のCPU種別を使い、物理CPUを推測しない。

取得先は固定GitHub repositoryのHTTPS Release URLとする。curlは先頭`--disable`で利用者の
curl設定を読み込まず、HTTPSとHTTPS redirectだけを許可する。TLS検証、timeout、サイズ制限を保つ。
版は必須引数で、`latest`の解決を加えない。導入先は既存directoryを絶対path化し、
空白・日本語を含む引数も分割・評価せずにGoへ渡す。

SHA-256を照合するまでは実行・展開しない。照合後も全体を展開せず、
直下の正規file `aidlc-install[.exe]`が1個だけあることを確認して、固定した一時pathへ内容を取り出す。
取り出すinstallerは64MiB以内とする。Go側がその後に元のarchive全体を再検証する。
終了codeを伝え、一時物だけを片付ける。sudo、PATH恒久変更、Codexのtrust承認、
PowerShellのExecutionPolicy変更、TLS検証無効化は行わない。

## 配置・失敗・再実行

既存の`.aidlc-install.lock`、全配置先の事前検査、排他的な新規作成、`Paths`/`Pending`の失敗表示を再利用する。
検証に失敗した一式は配置しない。保存途中の失敗では作成済みpathと未完了pathを表示し、利用者の既存dataを削除しない。
同じ場所への新規再配置は従来どおり拒否する。同版の移転は、3runtime・許諾文書・管理対象資材の照合をしてから
既存のRelocateFromを使い、利用者のhook、rules、stateを保持する。
新しい進捗stateや永続runtime形式は導入しない。

本家AI-DLCについては、固定参照と既存の承認済み配布方針で確認した、共通資材から環境別に配置する範囲を維持する。
今回のGo binaryの梱包・初回取得は本リポジトリの既存配布方式の改善であり、最新upstreamと同じ方式だとは主張しない。
工程、Sensor、agent起動、hook、skill本文、OKF契約を変更しない。

## 所有ファイルと作業単位

`work_unit_id=consolidated-release-bootstrap`、実装担当1名、`verification_mode=loop`とする。
以下はその担当の所有範囲で、親は担当稼働中に同じtreeを編集しない。

- `src/internal/release/`：format/schema、archive、検証、対応test。
- `src/internal/install/release.go`と関連release/移転test：取得・一式照合・既存配置への接続。
- `src/cmd/aidlc-install/main.go`とtest：引数・help。
- `src/cmd/aidlc-dist/`：7個の生成、licenses、candidate integration test。
- `src/bootstrap/install.sh`、`install.ps1`とGo test/helper：初回取得と失敗検査。
- `.github/workflows/distribution.yml`：同一候補7個の転送・3OS検証・下書き作成。
- `README.md`、`docs/distribution.md`、関連する開発者参照説明：実際の手順と個数の更新。
- 本計画、今回のRAM・索引：実装の事実と検証結果の追記。

既存の未commit文書を保持する。新規の外部Go moduleや開発toolを導入しない。
公開は親だけが許可を確認して行い、担当はIssue/PR/tag/Releaseを操作しない。

## 順序付きTDDと受入条件

各項目はtestを先に追加し、実行可能な意図したREDを見てから最小実装とrefactorへ進む。
新APIを必要とする場合だけcompile用の宣言を先に置ける。compile失敗をREDとは数えない。
既に通る既存契約はALREADY_GREENとして記録する。test名は次の入口にまとめる。

| 順序 | 動作・受入条件 | loopのtargeted command |
| --- | --- | --- |
| 1 | schema 2と6行checksum、版・環境の一致、不正・旧形式の拒否 | `go test -count=1 ./src/internal/release -run '^TestBundle(Manifest\|Sums)$'` |
| 2 | 5binaryのmode、全member照合、安全なpath、link・衝突・上限の拒否 | `go test -count=1 ./src/internal/release -run '^TestBundleArchive'` |
| 3 | 全6環境で合計7個だけを生成、再現可能なbytes、原典license保持、不正入力時の出力防止 | `go test -count=1 ./src/cmd/aidlc-dist -run '^Test(BundledRelease\|ReleaseLicenseInputs\|CandidateLicense)'` |
| 4 | online取得2回、offline1環境だけ、3runtime配置、既存保持、競合・保存失敗 | `go test -count=1 ./src/internal/install -run '^Test(Release\|Install)'` |
| 5 | 同版移転、runtime・license改変拒否 | `go test -count=1 ./src/internal/install -run '^Test(InstallerCommandRelocation\|ReleaseLicenseRetention)$'` |
| 6 | help・引数・exit codeが新形式と合う | `go test -count=1 ./src/cmd/aidlc-install ./src/cmd/aidlc-dist -run '^Test(InstallerCommand\|DistCommand)$'` |
| 7 | bootstrapの対象選択、checksum不一致、取得失敗、不正版・path、重複installer、引数保持、cleanup、再取得しない | `go test -count=1 ./src/bootstrap -run '^TestBootstrap'` |
| 8 | 候補検査自身が誤commit・版・環境・mode・licenseを検出する | `go test -tags=integration -count=1 ./src/cmd/aidlc-dist -run '^TestReleaseCandidate(MetadataValidation\|NativeSelection\|ProjectDirectory)$'` |

bootstrapのtestは一時PATH上の偽curlを用い、固定URLをtest用dataへ対応させる。
production scriptに任意URL、検証skip、試験専用の環境変数を追加しない。
補助実行ファイルはGo標準ライブラリで作り、shell/PowerShellと実際のscriptを実行する。
macOS上にPowerShellを追加せず、Windows CIで5.1と7をそれぞれ検証する。
取得を模擬したtestを、公開HTTPが成功した証拠とは説明しない。

loopでは対象testのみ実行する。全package、race、vet、cross compile、配布E2Eは最終検証へ集約する。
Goのformatはreview前に適用する。

## 独立レビュー・最終検証・公開

独立reviewerへ固定差分を`verification_mode=review`で渡す。担当範囲、取得・展開前の検証順、
既存dataの保持、5binaryの独立性、license原文、CIの同一候補性を確認する。
必要なfinding再現だけを実行し、修正は単独writerへ戻す。

差分が安定しblocking findingがなくなってから、親がread-onlyな`final`を1回開始する。

- `go test -shuffle=on ./...`、`go test -race ./...`、`go vet ./...`、gofmtの未適用確認、
  `go mod tidy -diff`、`git diff --check`。
- 既存のworkspace/OKF integration、FlowJourney/GitIndependentJourneyと関連する配布integration。
- Go 1.26.4、CGO無効、同じcommit・版で5binary×6環境を作り、候補7個を1回生成する。
- 生成した候補をmetadata検査とnative検査に渡す。既存のhelp/version、空PATHでのGo導入、
  期待する配置bytes、OKF、日本語検査、再配置拒否、移転、利用者資材保持を新形式でも確認する。
- CIはこの同じ候補を3OSへ転送する。bootstrapは取得だけを模擬し、実際のscript、展開、
  候補installer、配置を実行する。WindowsではPowerShell 5.1と7を両方確認する。
  macOS/Linux/Windowsのnative検査は全6CPUの実機実行成功とは説明しない。

親はPR checks全成功後に既存方式でmergeする。最終検証後の対象変更は証拠をstaleとして再検証する。
新版公開が許可された場合だけ、merge済みcommitへ新tagを作り、同一候補を検証する既存workflowから
7個の下書きを作る。個数・digest・版・commitを確認して公開し、公開URLから実際のbootstrapを1回確認する。
新しいCodex hook動作を変更していないため、以前のnative Codex証拠を今回の再実行結果とは呼ばない。

取り消しは新しい変更をrevertし、既存v0.1.1の手順を使う。公開済みtagや添付を上書きしない。
SHA-256は破損・取り違えの検知であり、配布元そのものの侵害を防ぐ署名とは説明しない。
実装上の未解決の重大な選択は現時点でない。新版公開の可否だけが保留事項である。


## 実装時の具体化と証拠

[loop実装記録](../ram/decisions/2026-09-14-consolidated-release-bootstrap-loop.md)に各RED/GREEN、
正規pathのtest期待値訂正、末尾commandと未実行finalを記録した。
許諾の配置は原稿用`LICENSES/PRODUCT.txt`と製品別`LICENSES/<product>/`、日本語説明は直下`README.md`。
既存bytesと製品対応を保持するためのpath具体化で、schema項目や機能を増やす変更ではない。

取得上限はcurlのstdoutを上限+1で制限し、curl自身の終了状態も確認する。
GoのredirectもHTTPSだけを許可する。tarの同名directory配下がinstallerとして選ばれないこと、
installer64MiBとtar末尾dataの拒否を追加の小testで固定した。

実候補bootstrap入口は`go test -tags=integration -count=1 -v ./src/bootstrap -run '^TestBootstrapCandidateNative$'`。
`AIDLC_DIST_DIR`と`AIDLC_RELEASE_VERSION`で同じ候補を渡す。これは親final/CI用でloopでは起動していない。
Windowsの通常testはPowerShell 5.1/7を両方必要とし、macOS上のskipを成功と数えない。

## Review修復01の順序と受入

Issue #202の承認済み範囲で、`consolidated-release-bootstrap-repair-01`を単独writer、loopで実施する。
R1はPowerShell公開ScriptBlockが呼出元へ戻り、LASTEXITCODEへ0/17/取得失敗を保持すること。
既存-Fileもプロセス終了値を保持する。5.1/7の両経路にtestを先に追加し、実候補bootstrapにも公開経路を接続する。
R2はarchiveの重複・link・path・mode拒否を、正常対照付きunpackBundle直接testで観測する。
既存実装が通るtestはALREADY_GREENとし、人工REDは作らない。
末尾は`go test -count=1 ./src/bootstrap -run '^TestBootstrap'`、
`go test -count=1 ./src/internal/release -run '^TestBundleArchive'`、影響2package通常test、gofmt、diff-check。
実候補bootstrapはintegrationの-listまで。PowerShellはローカル未導入のため動的RED/GREENを主張せず、
旧HEADと公式実装の根拠を残し、Windows CIの5.1/7実行を必須gateへ残す。

## Final修復02の検証分担

Issue #202、work_unit `consolidated-release-bootstrap-repair-02`をloopで実施する。
Go 1.26.4は-trimpath時に-ldflagsをBuildInfoへ保存しない。V1では実測に合うfixtureを先に追加し、
正しいbinaryの誤拒否REDから、Path・GoVersion・GOOS/GOARCH・CGO_ENABLED=0・-trimpath=trueの厳密照合へ直す。
V2ではnative版表示の完全一致helperとtestを先に追加し、4CLIのproduct/version/commit、
日本語CLIのproduct/versionと改行だけを受理する。誤値や追加dataを拒否する。
両項目のtargetedは`go test -tags=integration -count=1 ./src/cmd/aidlc-dist -run '^TestReleaseCandidate(MetadataValidation|NativeSelection|ProjectDirectory)$'`。
末尾はこれと影響package通常test、gofmt、diff-check。実候補・crossbuild・全体gateは親finalへ残す。

## Windows CI修復03

Issue #202 / PR #203、work_unit `consolidated-release-bootstrap-repair-03`、単独writer・loop。
R3aは複数curl.exeのPATH先頭選択と、最初のexeのStart失敗で元診断・code1を保持するtestを先に追加する。
Get-CommandのTotalCount 1とApplicationInfo.Pathを使い、開始成功前のHasExited/Killを防ぐ。
既存0/17/取得失敗、ScriptBlock後続到達と-File契約を維持する。
R3bは候補testのstdout/stderrを分離し、成功codeを確認した上でstdoutだけをJSON解析する。
末尾はbootstrap targeted・通常test、候補integrationの-list、gofmt/diff-check。
Windows動的GREENと実候補E2Eは新HEADのCI/finalへ残し、旧final-02のWindows失敗を成功扱いしない。
