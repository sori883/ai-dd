# 五CLIの独立review修復

2026-09-14。Issue #200、work_unit_id `five-cli-release-011-review-fixes`、verification_mode `loop`。
[既存承認](2026-09-13-five-cli-release-011-approved.md)と[計画](../../design/five-cli-release-011-plan.md)の範囲で、親が独立reviewの5findingを単独writerへまとめて委譲した。
開始HEAD `4359517222d68d0ec57d698ba23b6705488b15ab`。別writerは停止し、開始時のtreeはcleanだった。

| 順序 | 受入結果・所有対象 | targeted commandと証拠 |
| --- | --- | --- |
| 1 | 検証済み三runtime archiveのLICENSES以下を`aidlc/bin/VERSION/licenses/PRODUCT/`へ保存。既存衝突、保存失敗、同版移転時の欠落・改変を拒否。install/release.goと関連test、Native候補の配置照合 | `go test -count=1 ./src/internal/install -run '^TestReleaseLicenseRetention$'`: RED exit 1（許諾未保存・衝突通過）→GREEN exit 0。部分保存testのprefixを版directoryへ一意に訂正し、同subtestで未保存のRED exit 1も観測 |
| 2 | 同commitの正本から製品別の許諾集合・bytesとREADMEを独立照合。tar/zipのbinary0755、他file0644を検査。dist/candidate_license_test.goと公開候補検査へ接続 | `go test -count=1 ./src/cmd/aidlc-dist -run '^TestCandidateLicense$'`: RED exit 1（欠落・改変・余分な許諾を通過）→GREEN exit 0。`go test -count=1 ./src/cmd/aidlc-dist -run '^TestCandidateLicenseExecutableMode$'`: RED exit 1（tar/zipの非実行binaryを通過）→GREEN exit 0 |
| 3 | memory/flow/assignment/hook-reliability observerの転送commandを共通builderへ集約し、正しいsibling okf[.exe]を--okf-binaryで渡す | `go test -tags=integration -count=1 ./src/cmd/aidlc -run '^TestObserverBinaryArgs$'`: RED exit 1（転送flagなし）→GREEN exit 0 |
| 4 | Distribution draftのinstaller archive・checksum・manifest案内とAI-DD自称を修正 | 文書・設定だけの修正。人工REDなし |
| 5 | READMEと利用者ガイドで版directory内の役割別runtimeを案内し、旧memory入口をokfへ修正。documents helpの回帰test | `go test -count=1 ./src/internal/cli -run '^TestSplitRuntimeHelp$'`: RED exit 1（旧memory案内・runtime pathなし）→GREEN exit 0 |

許諾本文はinstaller自身の埋込版から補充しない。取得・checksum検証済みarchiveのbytesを保存し、同版移転でもそのbytesへ照合する。公開前候補検査は同commitのsourceと実Goの許諾へ独立に照合する。schemaや旧版互換の追加は行わない。元の13skillと日本語5許諾は変更しない。

Native候補testは、保存したすべてのruntime許諾を新規配置・移転後にbytes照合する。loopでは実候補E2Eを起動しない。observerの小testは送出するexec.Cmd.Argsと既存synthetic証拠を検査し、通常trustの実機成功の証拠とはしない。

## 末尾のtargeted群

下記を全件exit 0で再実行し、変更Goのgofmtとgit diff --checkを確認する。

- `go test -count=1 ./src/internal/install -run '^Test(ReleaseLicenseRetention|ReleaseAssetValidationCandidate|ReleaseAssetValidationOffline|InstallerCommandRelocation|InstallFailurePartial)$'`
- `go test -tags=integration -count=1 ./src/cmd/aidlc-dist -run '^Test(CandidateLicense|ReleaseCandidateMetadataValidation|ReleaseCandidateNativeSelection|ReleaseCandidateProjectDirectory)'`
- `go test -tags=integration -count=1 ./src/cmd/aidlc -run '^Test(ObserverBinaryArgs|MemoryMetadataCommandEvidence|BoundaryEvidenceSequence|HookReliabilityProbeProtocol|HookReliabilityProbeEvidence)$'`
- `go test -count=1 ./src/internal/cli -run '^Test(SplitRuntimeHelp|IntentDocuments|CodeKBHelp|RuleSkillSeparationHelp|OKFWorkLogHelp)'`

全package・race・vet・cross build・配布E2E・実Codexは未実施で親finalへ残す。commit・Issue・PR・公開操作も親担当。最終変更path・hashは一時証拠として親へ返す。

追加の移転時許諾欠落caseは `go test -count=1 ./src/internal/install -run '^TestReleaseLicenseRetention/missing$'` でALREADY_GREEN（exit 0）。既存改変検査の最小実装が欠落も拒否することを確認した。
