# 五つのCLIの配布と新規導入

`aidlc-install`はプロジェクトへ導入するCLI、`aidlc`は工程・担当・Sensor・hookを動かすCLI、`okf`は知識を検索・保存するCLI、`natural-japanese-go`は日本語を検査するCLIです。開発者用の`aidlc-dist`は五製品と共通資材を梱包します。通常の利用者にGoは不要です。

## 自動取得と候補の配置

ここで説明する7個の形式は、公開前のv0.1.2候補用です。公開済みv0.1.1の43個の添付物は変更しません。
v0.1.2のtagとRelease公開前には、[READMEの自動取得例](../README.md#2-プロジェクトに設定する)は実行できません。

初回scriptはOSと実行中のCPUを判定し、固定したGitHubの指定Releaseから`SHA256SUMS`と
`ai-dd_VERSION_OS_ARCH.tar.gz`（Windowsは`.zip`）1個を取得します。
macOS・Linuxはsh、curl、tar、mktempとshasumまたはsha256sum、WindowsはPowerShell 5.1/7、curl.exe、
Get-FileHashと標準.NETのZipArchiveを使います。必要ツールがなければ停止し、追加導入や管理者権限を要求しません。

SHA-256の一致を確認する前にarchiveを展開・実行しません。その後、正規fileのinstallerが直下に1個だけあることを
確認し、64MiB以内で取り出します。Go製installerは元の一式全体を検査します。
scriptは取得済みのdirectoryを`--release-dir`で渡し、同じarchiveを再取得しません。一時物は終了時に削除します。
公開PowerShellのScriptBlock呼出しは処理後に呼出元へ戻り、`$LASTEXITCODE`へinstallerの終了値を残します。
取得・検証失敗は1です。保存したscriptを`-File`で実行する場合は、同じ値をプロセスの終了値として返します。
実行環境のCPUを使うため、エミュレーション中に物理CPUへ勝手に切り替えません。

`aidlc-install`を既に持つ場合は、次の形で指定Releaseから直接導入できます。

```sh
/path/to/aidlc-install codex --release-version VERSION --project-dir /path/to/project
```

Go側もOS・CPUを自動選択し、checksum表と一式の2回だけ取得します。
全5binary・原稿・許諾を検査し、3runtimeだけを`aidlc/bin/VERSION/`へ配置します。
Windowsでは`.exe`が付き、PowerShellからは`& "C:\tools\aidlc-install.exe" codex ...`で起動します。
生成するskillとhookは各役割の絶対pathを参照するため、installer自身の移動は配置済み参照に影響しません。

オフライン入力は`--release-dir /path/to/candidate`で指定します。このdirectoryに必要なのは、
全6archiveのhashを含む6行の`SHA256SUMS`と、実行環境の1archiveだけです。たとえばLinux amd64なら
`ai-dd_VERSION_linux_amd64.tar.gz`です。公開前候補も同じ入力を使います。
手動で初回取得する場合はこの2ファイルを取得し、archiveのSHA-256を照合してから空の場所へ展開します。
取り出したinstallerには、この元の2ファイルがあるdirectoryを渡してください。

通信・offlineとも同じschema 2、version・target・全memberのhash/size/mode検証を通します。
指定版はinstaller自身のbuild版に固定せず、対応schemaであることを要求します。
旧43形式や未知schema、未知path、リンク、重複、親子衝突、不足は拒否します。
archiveの圧縮時・展開後は各512MiB、原稿は合計32MiB、manifestは4MiB、checksum表は4KiBが上限です。
別版の埋込資材へのfallbackや、offline指定時の追加通信はありません。

既存ファイルは上書きしません。予約中の同時導入、欠落、破損、版違い、既存配置との衝突では停止します。失敗時のJSONにある`Paths`は保存済み、`Pending`は残りの対象です。部分配置を成功とせず、出力と利用者ファイルを確認してください。利用者の`AGENTS.md`や既存設定を一律に削除して再試行しないでください。

Codexでプロジェクトを開き、新しい会話を開始します。通常のhook trustで、表示されたruntimeと対象プロジェクトを確認します。Gitの初期化は導入条件ではありません。旧`aidlc install`・`aidlc memory`のaliasや旧版dataの移行はありません。

## 切替と参照の補正

同じ版のプロジェクトを別directoryへ移した場合は、元のrootと元のruntime pathを明示します。

```sh
/path/to/aidlc-install codex --release-version VERSION \
  --project-dir /new/project --relocate \
  --from-project-dir /old/project \
  --from-binary /old/project/aidlc/bin/VERSION/aidlc
```

同版runtimeのbytesと既知のskill・hookを照合し、役割別参照を含む6ファイルを補正します。利用者のRule、Knowledge、state、runtime記録と独自hookを保持します。未知のskill編集や不足があれば書込み前に停止します。これは異版の互換性・移行を提供する操作ではありません。失敗時は`Paths`・`Pending`と保存した原文を確認し、知らない変更を自動削除しません。

## 開発者が候補を生成する

共通原稿は`src/core/`、Codex固有の接続差分は`src/harness/codex/`が正本です。対象commitから五製品を六つのOS・CPU向けにbuildし、`PRODUCT-OS-ARCH[.exe]`として入力directoryへ置きます。公開buildは確認済みのGo 1.26.4に固定します。品質CIのstable検査とは別です。

`--license-dir`には同じbuildの許諾入力を用意します。rootの`LICENSE`を`PRODUCT.txt`へコピーし、`go env GOROOT`の`LICENSE`・`PATENTS`を`Go-LICENSE.txt`・`Go-PATENTS.txt`へ、`go env GOVERSION`を`GO_VERSION`へ保存します。`src/distribution/licenses/`の三文書もコピーします。独自LICENSEの正本はrootだけです。

```sh
go run ./src/cmd/aidlc-dist --input-dir /tmp/release-input \
  --output-dir /tmp/new-candidate --version VERSION \
  --commit FULL_COMMIT_SHA --go-version go1.26.4 \
  --license-dir /tmp/release-licenses
```

各binaryのbuildには`buildinfo.Version`と`buildinfo.Commit`をlinkerから渡します。日本語CLIだけは既存の`main.version`へ版を渡します。具体的なbuildと入力作成は[Distribution workflow](../.github/workflows/distribution.yml)にあります。`aidlc-dist --version`だけなら梱包器自身の版を表示します。梱包器は公開やアップロードを行いません。

出力はmacOS・Linux・Windowsのamd64・arm64向け一式6個と`SHA256SUMS`の**7個**です。
各一式に5binary、schema 2の`manifest.json`、`core/`、`codex/`、`LICENSES/`、日本語CLIの`README.md`を含めます。
manifestは自身以外の全memberをpath順に記録し、5binaryは0755、その他は0644とします。
外側のchecksum表は6archiveだけを名前順に記録します。

`LICENSES/PRODUCT.txt`は原稿の独自MIT、`LICENSES/PRODUCT/`は各製品に対応する許諾文書です。
実GoのLICENSE・PATENTS、該当するYAML等の許諾、原稿を含む製品の13skillのLICENSEと出典、
日本語CLIの従来5文書をbytesを変えず保持します。導入先にもruntimeの許諾を
`aidlc/bin/VERSION/licenses/PRODUCT/`へ保存します。PRODUCTは製品名です。
許諾入力のGo版・本文が梱包器の実Goと一致しなければ出力しません。

## GitHub Actionsと公開

Distributionは確定commitから一度buildして梱包した7件をActions artifactへ1日保存し、その同じartifact IDを3OSのnative検査へ渡します。各OSでは初回scriptから同じ候補を導入します。取得だけを偽curlへ置き換え、実installerを起動し、WindowsはPowerShell 5.1と7を両方検査します。五つのCLIの版・help、実installer経由の新規導入、既存file保持、自然文検査、同版移転を実行します。再buildした候補を代用しません。全30binaryではGoのbuild情報にある製品path、Go版、OS・CPU、CGO無効、trimpathを確認します。trimpath時に保存されないlinker引数は検査に使いません。native実行では4CLIの製品名・版・commitを完全一致で確認し、日本語CLIは既存の製品名・版表示を確認します。全六CPU上での実行や、実Codexでのhook成功はこの検査だけでは証明しません。

手動実行の`tag`にはmainに含まれる既存tagを指定します。`create_draft`は既定falseで、trueならpackage・nativeが成功した後に7件を添付したRelease下書きを作ります。書込み権限はdraft jobだけです。remote tag変更、artifact不一致、API失敗、既存Releaseがあれば停止します。途中失敗で部分的な下書きが残った場合も自動削除・上書きしません。内容を確認してから公開します。

公開候補の最終確認では、実五製品の依存と許諾集合、三OS検査、通常trustの実Codex操作を区別して記録します。独立reviewとfinal検証が終わるまでは、候補を検証済みの一般公開版と扱いません。
