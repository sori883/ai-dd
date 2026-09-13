# 五CLI・指定版導入のloop証拠

2026-09-13。work_unit_id: `five-cli-release-011`、verification_mode: `loop`。
Issue #200、[承認](2026-09-13-five-cli-release-011-approved.md)、[計画](../../design/five-cli-release-011-plan.md)に基づく単独writer。
開始・終了HEADは`64833d7e94203be0284c18abbf4a35b23810cf77`。commit・GitHub操作は親担当。

## 順序付きTDD

下記C1〜C6は計画表のexact command。各sliceでtestを先に追加し、許可された空scaffoldを使って実行可能なassertionにした。compile errorや該当testなしはREDに数えていない。

| slice | REDまたは初回状態 | GREEN |
| --- | --- | --- |
| 1 | C1 exit 1。旧公開入口が通る、新OKF引数・help・版の空scaffoldが期待を満たさない | C1 exit 0。独立OKF入口とルート解決、旧公開入口拒否 |
| 2 | C2 exit 1。空OKFサービスが本文保存結果・保存失敗・競合契約を満たさない。既存metadataとIntent検索はALREADY_GREEN | C2 exit 0。共有処理を抽出し、CAS・日時・保存済み本文の失敗結果を維持 |
| 3 | C3 exit 1。設定済みOKF読取りが拒否され、子のOKF更新が役割判定を通過する | C3 exit 0。役割別絶対binaryの識別、親子の境界と既存Rule事前照合、一致IDのPost解放 |
| 4 | C4 exit 1。空の明示資材render／配置scaffoldが三役割pathと移転を満たさない | C4 exit 0。渡された同版FSを合成し、6参照の移転と既存資材保持 |
| 5 | C5 exit 1。五製品・data・許諾の梱包がない。data独自LICENSE追加も不足assertionのexit 1を確認 | C5 exit 0。43件、独立許諾、source schema 1、metadataとchecksum |
| 6 | C6 exit 1。空取得／installer scaffoldが指定版導入・拒否・予約・失敗結果を満たさない。同版移転は既存配置衝突からRED | C6 exit 0。通信とオフラインの同一検査、予約、既存file拒否、部分結果、同版移転 |
| 7 | 既存テストの入口期待を新役割へ移す。OKF helpの操作別案内とcodekb例、梱包器単独版表示、六target必須はrunnable REDを観測。文書とCIは人工REDなし | 関連通常testとobserver/metadataの小testはexit 0。実候補journeyはfinalで実行する |

C1: `go test -count=1 ./src/internal/cli ./src/internal/okfcli ./src/cmd/aidlc ./src/cmd/okf -run 'Test(FiveCLIContract|OKFCommand|OKFHelp|ProjectRoot)'`

C2: `go test -count=1 ./src/internal/okfcli ./src/internal/okfapp ./src/internal/okfmemory -run 'Test(OKFMemoryContract|OKFSaveFailure|OKFConcurrentUpdate)'`

C3: `go test -count=1 ./src/internal/app -run 'Test(HookSplitCLI|ChildHookSplitCLI|HookSplitCLIPost|HookRecoveryGuidance|AssignmentStage)'`

C4: `go test -count=1 ./src/harness/codex ./src/internal/install -run 'Test(SplitCLIDistribution|SplitCLIRelocation|SplitCLIInstallConflict)'`

C5: `go test -count=1 ./src/cmd/aidlc-dist -run 'Test(FiveProductArchive|FiveProductManifest|ReleaseLicenseInputs|VersionedAssets)'`

C6: `go test -count=1 ./src/cmd/aidlc-install ./src/internal/install -run 'Test(InstallerCommand|ReleaseDownload|ReleaseAssetValidation|InstallReservation|InstallFailure)'`

追加のRED→GREENは`go test -count=1 ./src/cmd/aidlc-dist -run TestFiveProductVersion`（exit 1: flag needs an argument → exit 0）、`go test -count=1 ./src/cmd/aidlc-dist -run '^TestFiveProductManifestRequiresSixTargets$'`（exit 1: incomplete release candidate accepted → exit 0）。OKF help移植の不足は`go test -count=1 ./src/internal/cli ./src/internal/okfcli`でexit 1からexit 0。誤った旧入口のtest期待を変更しただけの失敗は製品REDに数えない。

HTTPの正常・404・過大応答とunsafe archiveの追加caseは、既存実装に対してALREADY_GREEN。重複installer flagの回帰testは初回に実GitHubの404へ到達し、その後引数解析で拒否するよう修正した。この実行を制御されたHTTP fixtureの証拠と混同しない。

## 維持した境界と実装詳細

Ruleの説明誤記は[別RAM](2026-09-13-five-cli-hook-evidence-clarification.md)で訂正した。新たなPost出力認証や全操作auditは追加しない。OKFの共有Go関数はSensor等から直接使う。公開CLIの旧aliasはない。古いruntime journeyのsetupは内部install関数によるfixtureと明示し、ReleaseCandidateNativeは実installerを同じ43資材から起動する。両者の証拠は別である。

data archive自体に独自MITと13原典のLICENSE/sourceを含める。独自LICENSE正本はrootのみで、梱包入力へコピーして固定hashを照合する。Goの実GOROOTとGO_VERSIONを照合し、Distribution buildは確認済みGo 1.26.4に固定する。品質CIのstable matrixは維持する。Go vendorの許諾一致は1.26.4での親の調査に限り、他版へ一般化しない。日本語の既存5許諾bytesは保持する。

新しい`five-cli-assets-sha256.json`は71配置資材を固定する。旧二つのfixtureは履歴として保持し、SHA除外を増やさない。原稿の三役割参照が工程hashへ反映されるため、既存版のdata移行は提供せず新規配置に限定する。`aidlc/bin/`は既存の検証成果集合からaidlc全体が除外される境界内であり、除外処理は変更しない。

## 末尾検証と残範囲

C1〜C6をすべて再実行しexit 0。影響packageの通常test、変更Goのgofmt、`git diff --check`を行う。最終hashと変更path一覧は親へ渡す一時証拠で保存する。

observer/metadataの小test:
- `go test -tags=integration -count=1 ./src/cmd/aidlc -run '^Test(MemoryMetadataCommandEvidence|BoundaryEvidenceSequence)$'`
- `go test -tags=integration -count=1 ./src/cmd/aidlc-dist -run '^TestReleaseCandidate(MetadataValidation|NativeSelection|ProjectDirectory)$'`
- `go test -count=1 ./src/internal/flow -run '^TestProcedureBoundary(ComposedWorkflow|SharedOperationPropagation)$'`

全package、race、vet、cross build、実候補E2E、通常trustの実Codex、実依存・許諾照合、GitHub checksと公開はloopでは未実施。親の独立reviewとfinalへ引き継ぐ。Issue #200はPRでRefsを使い、公開確認後に親がcloseする。
