# 五つのCLIの配布と新規導入

`aidlc-install`はプロジェクトへ導入するCLI、`aidlc`は工程・担当・Sensor・hookを動かすCLI、`okf`は知識を検索・保存するCLI、`natural-japanese-go`は日本語を検査するCLIです。開発者用の`aidlc-dist`は五製品と共通資材を梱包します。通常の利用者にGoは不要です。

## 新形式の候補を配置する前に

[GitHub Releases](https://github.com/sori883/ai-dd/releases)で、使用する版のOS・CPUに合う`aidlc-install_VERSION_OS_ARCH.tar.gz`（Windowsは`.zip`）、`aidlc-install-manifest.json`、`aidlc-install-SHA256SUMS`を取得します。checksumにあるarchiveとmanifestのSHA-256を照合してから空の場所へ展開し、`--version`と`--help`を確認してください。macOS・Linux・Windowsのamd64・arm64に対応します。

```sh
/path/to/aidlc-install codex --release-version v0.1.1 --project-dir /path/to/project
```

PowerShellでは`& "C:\tools\aidlc-install.exe" codex --release-version v0.1.1 --project-dir "C:\projects\app"`です。版名は取得したReleaseのtagに合わせます。

installerは`sori883/ai-dd`の指定Releaseから、そのOS・CPUの三つのruntimeと同版の共通資材を取得します。manifestとchecksum、version、commit、archiveとbinaryのhashを検査してから、`aidlc/bin/v0.1.1/`へ`aidlc`・`okf`・`natural-japanese-go`を配置します。Windowsでは`.exe`が付きます。生成するskillとhookは各役割の絶対pathを参照します。installer自身を移動しても、配置済みruntimeの参照は変わりません。

取得済みの公開前候補やオフライン資材を使うときは`--release-dir /path/to/candidate`を加えます。この入力も通信時と同じ検査を通します。installerに埋め込んだ別版の資材へ切り替えるfallbackはありません。未知のschema、機能、pathは拒否するため、その場合は対象版に対応するinstallerを用意します。

既存ファイルは上書きしません。予約中の同時導入、欠落、破損、版違い、既存配置との衝突では停止します。失敗時のJSONにある`Paths`は保存済み、`Pending`は残りの対象です。部分配置を成功とせず、出力と利用者ファイルを確認してください。利用者の`AGENTS.md`や既存設定を一律に削除して再試行しないでください。

Codexでプロジェクトを開き、新しい会話を開始します。通常のhook trustで、表示されたruntimeと対象プロジェクトを確認します。Gitの初期化は導入条件ではありません。旧`aidlc install`・`aidlc memory`のaliasや旧版dataの移行はありません。

## 切替と参照の補正

同じ版のプロジェクトを別directoryへ移した場合は、元のrootと元のruntime pathを明示します。

```sh
/path/to/aidlc-install codex --release-version v0.1.1 \
  --project-dir /new/project --relocate \
  --from-project-dir /old/project \
  --from-binary /old/project/aidlc/bin/v0.1.1/aidlc
```

同版runtimeのbytesと既知のskill・hookを照合し、役割別参照を含む6ファイルを補正します。利用者のRule、Knowledge、state、runtime記録と独自hookを保持します。未知のskill編集や不足があれば書込み前に停止します。これは異版の互換性・移行を提供する操作ではありません。失敗時は`Paths`・`Pending`と保存した原文を確認し、知らない変更を自動削除しません。

## 開発者が候補を生成する

共通原稿は`src/core/`、Codex固有の接続差分は`src/harness/codex/`が正本です。対象commitから五製品を六つのOS・CPU向けにbuildし、`PRODUCT-OS-ARCH[.exe]`として入力directoryへ置きます。公開buildは確認済みのGo 1.26.4に固定します。品質CIのstable検査とは別です。

`--license-dir`には同じbuildの許諾入力を用意します。rootの`LICENSE`を`PRODUCT.txt`へコピーし、`go env GOROOT`の`LICENSE`・`PATENTS`を`Go-LICENSE.txt`・`Go-PATENTS.txt`へ、`go env GOVERSION`を`GO_VERSION`へ保存します。`src/distribution/licenses/`の三文書もコピーします。独自LICENSEの正本はrootだけです。

```sh
go run ./src/cmd/aidlc-dist --input-dir /tmp/release-input \
  --output-dir /tmp/new-candidate --version v0.1.1 \
  --commit FULL_COMMIT_SHA --go-version go1.26.4 \
  --license-dir /tmp/release-licenses
```

各binaryのbuildには`buildinfo.Version`と`buildinfo.Commit`をlinkerから渡します。日本語CLIだけは既存の`main.version`へ版を渡します。具体的なbuildと入力作成は[Distribution workflow](../.github/workflows/distribution.yml)にあります。`aidlc-dist --version`だけなら梱包器自身の版を表示します。梱包器は公開やアップロードを行いません。

五製品それぞれ6archive・manifest・checksumの8件、計40件に、`aidlc-assets_VERSION.tar.gz`・`aidlc-assets-manifest.json`・`aidlc-assets-SHA256SUMS`の3件を加え、**43件**です。`aidlc`のmetadataは`manifest.json`と`SHA256SUMS`、他製品は製品名を接頭辞にします。schema 1は受取るpathと機能を固定し、新しい資材集合には対応するschemaとinstallerが必要です。

各binary archiveには独自MIT、実GoのLICENSE・PATENTS、該当するYAML等の許諾を含めます。原稿を含む製品には13skillのLICENSEと出典も含め、日本語CLIには従来の5文書をbytesを変えず同梱します。data archive自身にも独自MITと原典表示を含めます。導入時には各runtime archiveの許諾文を`aidlc/bin/VERSION/licenses/PRODUCT/`へ保持します。VERSIONは導入したRelease名、PRODUCTはruntimeの製品名です。許諾入力のGo版・本文が梱包器の実Goと一致しなければ出力しません。

## GitHub Actionsと公開

Distributionは確定commitから一度buildした43件をActions artifactへ1日保存し、その同じartifact IDを3OSのnative検査へ渡します。各OSでは五つのCLIの版・help、実installer経由の新規導入、既存file保持、自然文検査、同版移転を実行します。再buildした候補を代用しません。全六CPU上での実行や、実Codexでのhook成功はこの検査だけでは証明しません。

手動実行の`tag`にはmainに含まれる既存tagを指定します。`create_draft`は既定falseで、trueならpackage・nativeが成功した後に43件を添付したRelease下書きを作ります。書込み権限はdraft jobだけです。remote tag変更、artifact不一致、API失敗、既存Releaseがあれば停止します。途中失敗で部分的な下書きが残った場合も自動削除・上書きしません。内容を確認してから公開します。

公開候補の最終確認では、実五製品の依存と許諾集合、三OS検査、通常trustの実Codex操作を区別して記録します。独立reviewとfinal検証が終わるまでは、候補を検証済みの一般公開版と扱いません。
