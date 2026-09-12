# D1 Go実装handoff

Issue #183。`work_unit_id=three-host-distribution-d1`、`verification_mode=loop`。承認は[実装依頼](../ram/decisions/2026-09-12-three-host-product-implementation-approved.md)、範囲は[計画](three-host-distribution-plan.md)。以下は利用者仕様を変えない内部API詳細であり、直接依頼の範囲で親が確定した。

## scaffold許可

新規 `src/harness/manifest.go` で `Mapping{Files fs.FS; Source, Destination string; Tree bool}`、`Asset{Path string; Data []byte}`、`Manifest{Mappings []Mapping; Generated []Asset}` を宣言する。`func (m Manifest) Render(binary string) ([]Asset, error)` の初期bodyは `return nil, nil` だけを許可する。このscaffoldで正常投影testを実行可能にし、空結果によるassertion REDを確認してから実装する。

RenderはFS読取りのみ。Tree=falseは単一file、trueはdirectoryを再帰展開する。Sourceの`.`はTreeに限り許可する。DestinationはFSに依存しないslash相対path。Treeの空Destinationはrootへの投影を表す場合だけ許可し、完成したAsset.Pathは空を許可しない。絶対path、backslash、空segment、`.`/`..` segment、drive風path、不正FS source、同一配置先の二重定義、file/directoryの衝突を拒否する。原稿の `@@BINARY@@` を既存と同じPOSIX shell引用binaryへ置換し、Generatedは完成済みbytesなので置換しない。返却はPath昇順。asset path/mode以外の製品契約は変更しない。

`src/harness/codex/manifest.go` には `func Distribution(root, binary string) ([]harness.Asset, error)` を追加できる。初期body `return nil, nil` のみをscaffoldとして許可する。既存core/codex/workflow FSをmappingへ渡し、既存と同一hook JSONをGeneratedへ含める。hook生成のprivate helperは必要な引数で分離してよい。既存install.Codexのroot/binary検証と保存は保持する。既存`codex.Files`原稿を移動しない。

## slice一覧

| slice_id | test所有file | 実装所有file | exact targeted command |
| --- | --- | --- | --- |
| d1-01-parity | `src/internal/install/manifest_parity_test.go`、`testdata/codex-assets-sha256.json` | 変更前の証拠採取のみ | `go test -count=1 ./src/internal/install -run '^TestCodexManifestParity$'` |
| d1-02-render | `src/harness/manifest_test.go` | `src/harness/manifest.go`、必要なら`render.go` | `go test -count=1 ./src/harness -run '^TestManifest'` |
| d1-03-codex | `src/harness/codex/manifest_test.go`、`src/internal/install/manifest_parity_test.go` | `src/harness/codex/manifest.go`、必要なら`emit.go`、`src/internal/install/install.go` | `go test -count=1 ./src/harness/codex ./src/internal/install` |
| d1-04-consumers | 既存testへの変更不要を既定、必要なregressionは所有packageに追加 | 対象変更はしないことを既定、必要なら`src/harness/codex/assets.go`のみ | `go test -count=1 ./src/internal/app -run 'Test(StageSkillsRead|OKFSkillRead|RuleSkillSeparationHook)'` および `go test -count=1 ./src/internal/install -run '^TestNaturalJapaneseSkill$'` と配布source差分なし確認 |

parityの基準はcandidate rendererで作らず、変更前installerから全fileのpathとSHA256を採取する。hookの一時project rootだけを固定markerへ置換し、固定binary文字列を使い、regular modeも比較する。SHA一覧はレビュー可能なtestdataとして保存する。既存動作を固定するtestはALREADY_GREENとして記録する。正常/不正原稿、複数mapping、Generatedとの衝突等の新APIは最初にrunnableなREDが必要。

新規testsを置くpackageの都合でfile分割は可能だが、scopeは本一覧に限定する。manifest helperの命名・内部ループなどは単独writerが既存慣例で決める。Goのfmt適用はloop内、最後に `go test -count=1 ./src/harness/... ./src/internal/install` を実行する。全体test/race/vet/E2Eは親のfinalへ残す。親の既存docs・調査・元checkoutを編集せず、他者変更をrevertしない。
