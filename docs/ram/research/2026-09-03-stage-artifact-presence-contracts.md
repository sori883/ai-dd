# Stage completion artifact presence の参照契約

- 調査日: 2026-09-03（Asia/Tokyo）
- 状態: Current（このrepositoryに固定された本家AI-DLC `2.6.123`の確認範囲）
- 対象Issue: [#69](https://github.com/sori883/ai-dd/issues/69)
- 固定base: `61da013a8f3a62bbc9e8b5ec0eab2bb87630a77e`

## 背景と確認範囲

通常Stageを完了へ進める前に、Stage metadataの必須 `Produces` がIntent record内へ少なくとも1件
書き出されているかを、内容を読まずに確認する契約を調べた。本記録は、非per-unit・非CodeKBの
通常Stageに対するread-onlyな存在判定だけを対象とする。最新upstream全体との一致は主張せず、
repositoryに固定されたAI-DLC `2.6.123` snapshotの次の範囲だけを確認した。

| 根拠 | 固定snapshotで確認した範囲 |
| --- | --- |
| `docs/実装_aidlc-workflows/core/tools/aidlc-state.ts:2566-2638` | `producesDirsForStage` と `producesArtifactsExist` の通常Stage配置、空 `produces` の扱い、any-of存在判定 |
| `docs/実装_aidlc-workflows/core/tools/aidlc-state.ts:3302-3331` | `verifyStageArtifacts` が完了前に存在guardを呼ぶ境界 |
| `docs/実装_aidlc-workflows/core/tools/aidlc-artifact-vocabulary.ts:1-18` | artifact名から物理filenameを決める共有語彙と3つの例外 |
| `docs/実装_aidlc-workflows/tests/integration/t185-stage-artifact-guard.test.ts:604-700` | 欠損時の拒否、通常record path上の宣言artifactによる成功、state変更前のguard観測 |

## 通常Stageの契約

`graph.Stage.Produces` が空なら、確認対象がないため `HasRequiredOutput` は filesystem を参照せず
`(true, nil)` を返す。非空の場合は、Stageの `Phase`、`Slug`、各required artifact名を
`^[a-z][a-z0-9-]*$` のlowercase kebab shapeとして先に検証する。metadataが不正なら
`ErrInvalidMetadata` をwrapしたerrorを返し、regular fileが先に存在していても後続の不正名を
filesystemから隠さない。入力sliceとfilesystemは変更しない。

通常Stageの候補pathは、record filesystemをrootとして
`path.Join(stage.Phase, stage.Slug, filename)` で組み立てる。各候補は `fs.Stat` だけで調べ、
成功したinfoが regular file の場合に限り存在とみなす。missing、permissionなどの個別Stat error、
directoryやFIFOなどのnon-regular entryは未存在として次候補へ進み、全候補が該当しなければ
`(false, nil)` を返す。file内容は読まない。

`OptionalProduces` はこの完了guardのrequired判定に参加しない。複数の `Produces` は全件必須ではなく、
少なくとも1件のregular fileがあれば成功する。`recordFS == nil` は metadata検証後に
`ErrInvalidFilesystem` をwrapしたinput errorとする。typed-nil等への汎用reflect対策、内容検証、
書込み、state/audit/approval/lock/clock、CLI接続はこのAPIの責務に含めない。

## filename例外

既定の物理filenameは `<artifact>.md` である。固定v2.6.123のartifact vocabularyに合わせ、次だけを
例外として扱う。

| `Produces` 名 | 物理filename |
| --- | --- |
| `traceability` | `traceability.json` |
| `build-test-results` | `test-results.md` |
| `load-test-results` | `test-results.md` |

## 段階的な境界

本Issueは通常Stageの一つ以上のrequired outputを確認する内部APIだけを追加する。per-unit Stageの
unit列挙・`produces_kinds`、CodeKB Stageのrepoごとの配置、`workspace_requires`、全required instanceの
検査は後続の別責務へ残す。これにより、既存の `graph.Stage` schemaやIntent/stateの永続形式を変更せず、
呼出し側が対象Stageを決めた後にこの判定を組み込める。

## 未確認事項と残余risk

- 本家の特殊なper-unit・CodeKB・workspace source guardを、この通常Stage APIへ拡張する仕様は確認していない。
- `fs.FS`のcontainmentやlifecycleはcallerの責務であり、filesystemが並行更新中の一貫したsnapshotは保証しない。
- 本家snapshotの確認範囲外の将来artifact vocabulary、state遷移、CLI接続、監査証跡は未確認である。
