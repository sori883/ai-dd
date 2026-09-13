# 7配布物・初回取得のloop実装記録

Issue #202、work_unit_id `consolidated-release-bootstrap`、verification_mode `loop`。
開始HEADは`a3514ecc6ac1900a93e064e70aa1ce30efdf19e7`、作業branchは`codex/consolidated-release-assets`。
[直接承認](2026-09-14-consolidated-release-assets-approved.md)と
[自己完結計画](../../design/consolidated-release-bootstrap-plan.md)の8項目を単独writerで実施した。
既存の親作成計画・RAMを保持し、commit、GitHub操作、公開操作はしていない。

## 採用した実装詳細

schema 2の一式6個と6行のSHA256SUMSを作る。原稿の独自MITは`LICENSES/PRODUCT.txt`、
各製品の従来許諾は`LICENSES/<product>/`、日本語CLIの説明は直下`README.md`へ入れる。
原典13skillと日本語5許諾の正本文字列は変更していない。導入する3runtimeの許諾は従来どおり
`aidlc/bin/VERSION/licenses/<product>/`へ保持する。取得した版のmanifestが全memberのbytesを束縛し、
installer自身の別版の許諾本文へ置換しない。公開候補検査は同commitの正本と別途照合する。

Go installerの取得はchecksum表とnative一式の2回。offlineはこの2ファイルだけで成立し、
他の5archiveは不要。同版移転・衝突・部分保存は既存処理を再利用した。
旧schemaの内部helperを使う単体testは残るが、公開installerの旧43形式fallbackはない。
原稿、配置71資材の固定fixture、段階・Sensor・hook・OKFの意味は変更していない。

shはcurlのstdoutを上限+1で切って、curl終了codeを別途確認する。古いcurlのContent-Length依存だけに
上限を任せない。PowerShellもProcessのstdoutをbyte streamで読み、上限超過時に停止する。
HTTPS以外へのredirectを拒否する。Goの取得側もHTTPS redirectと回数上限を維持する。
shのtar指定は同名directoryの子も選び得るため、名前一覧が直下`aidlc-install`の1行であることを
検査した後、正規fileであることを検査する。tar末尾の非zero data、installerの64MiB超過もGo側で拒否する。

PowerShell scriptはASCIIで記述し、5.1のBOMなし読込みと7で診断文字列の解釈を変えない。
公開入口は取得完了後のscriptblock、CIは同じASCII scriptを実行する。
Unixの`pwd -P`とmacOSの`/var`→`/private/var`正規化は契約どおり。初回valid testの未正規化path期待値は
誤りだったため、一度停止し、親の明示handoffに基づき`filepath.EvalSymlinks`へ訂正した。
この失敗は製品のREDに数えない。空白・日本語を含むpath、失敗時の拒否は維持する。

## TDD証拠

各commandはrepository rootから実行。REDはcompile失敗やskipではなく実行可能なassertion失敗。

| 項目 | exact targeted command | 観測 |
| --- | --- | --- |
| 1 | `go test -count=1 ./src/internal/release -run '^TestBundle(Manifest|Sums)$'` | 宣言だけのscaffoldでは不正manifest/checksum受理と空結果でexit 1、検証実装後exit 0 |
| 2 | `go test -count=1 ./src/internal/release -run '^TestBundleArchive'` | 不完全集合・不正hash・duplicate/link/親子衝突/mode受理でexit 1→0。完全集合・原稿32MiB超過も確認。追加のinstaller64MiB超過とtar末尾dataはそれぞれ受理のRED exit 1→拒否GREEN exit 0 |
| 3 | `go test -count=1 ./src/cmd/aidlc-dist -run '^Test(BundledRelease|ReleaseLicenseInputs|CandidateLicense)'` | 公開数43、期待7のRED exit 1→0。6対象・再現bytes・許諾正本/mode照合。旧archive名を読むtest fixtureを新形式へ切替 |
| 4 | `go test -count=1 ./src/internal/install -run '^Test(Release|Install)'` | 完全な新bundle fixtureの導入が旧manifestを求めて失敗、exit 1→0。2回取得・offline nativeのみ・3runtime・license保存・競合・部分失敗・既存保持 |
| 5 | `go test -count=1 ./src/internal/install -run '^Test(InstallerCommandRelocation|ReleaseLicenseRetention)$'` | 新取得から既存配置処理へ接続後、同版移転・許諾改変/不足拒否は初回exit 0、ALREADY_GREEN |
| 6 | `go test -count=1 ./src/cmd/aidlc-install ./src/cmd/aidlc-dist -run '^Test(InstallerCommand|DistCommand)$'` | helpのbundle入力・SHA256SUMS説明不足でexit 1→0。引数とexit codeの既存検査も通過 |
| 7 | `go test -count=1 ./src/bootstrap -run '^TestBootstrap'` | 実行可能な空scriptの引数欠落・不正入力成功でexit 1→実装後exit 0（Unix）。nested installerの誤受理もexit 1→0。exit code17伝達・存在しないproject・未対応環境はALREADY_GREEN。PowerShell testはmacOSではskip、成功の証拠に含めない |
| 8 | `go test -tags=integration -count=1 ./src/cmd/aidlc-dist -run '^TestReleaseCandidate(MetadataValidation|NativeSelection|ProjectDirectory)$'` | commit/version/toolchain/target/schema/mode/license/source改変の受理でexit 1→0、新native選択の空結果でexit 1→0、Go build情報不一致の受理でexit 1→0。小fixtureだけで実binary起動なし |

追加の取得境界検査は`go test -count=1 ./src/internal/install -run '^TestReleaseDownloadRedirectMustRemainHTTPS$'`。
HTTP redirect受理のRED exit 1→拒否GREEN exit 0。項目4の末尾commandにも含まれる。
補助のnested検査実行は`go test -count=1 ./src/bootstrap -run '^TestBootstrap$'`、RED exit 1→GREEN exit 0。

## work unit末尾とfinal境界

末尾に上記8commandと追加取得境界を再実行し、全9commandのexit 0を確認した。
影響package通常testは
`go test -count=1 ./src/cmd/aidlc-dist ./src/internal/release ./src/internal/install ./src/cmd/aidlc-install ./src/bootstrap`。
影響5packageの通常testもexit 0。変更21 Go fileへgofmtを適用し、`git diff --check`はexit 0。

`go test -tags=integration -list '^TestBootstrapCandidateNative$' ./src/bootstrap`はcompile/listだけ実行し、
入口を確認した。実候補をbuild/起動する検証の代わりではない。
final用の入口は`TestReleaseCandidateMetadata`、`TestReleaseCandidateNative`、
`TestBootstrapCandidateNative`。後者は初回取得だけをtestの偽curlへ置換し、同じ候補installerで
実配置し、3runtime・原稿・許諾のbytesと版を確認する。製品に任意URLや検証skip/test環境変数は追加しない。
CIは同じ7候補を3OSへ渡し、WindowsではPowerShell 5.1/7の両方を実行する。

全package、race、vet、cross build、配布E2E、実候補起動、Windows実行、公開HTTPはloopでは未実行。
独立reviewと親のread-only final、GitHub checksは別gate。v0.1.2は公開前候補としてREADMEに明示し、
公開承認が確定する前にtagやReleaseを操作しない。
