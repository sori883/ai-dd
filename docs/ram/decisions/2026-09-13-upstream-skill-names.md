# 外部由来skillに製品の接頭辞を付けない

2026-09-13。ユーザーは「ai-dlc等の独自のskillsじゃないものは、ai-dlcという接頭語を付けないで。
人のものだから」と修正を依頼し、ライセンスの許容とは別に作者への帰属を明確にすることを求めた。
この直接依頼を[命名・帰属修正計画](../../design/upstream-skill-names-plan.md)の実装根拠とする。
対応は[Issue #198](https://github.com/sori883/ai-dd/issues/198)、分類はユーザーリクエスト。

11工程skillの`aidlc-`を外し、原典の名前を使う。外部OKF手順を翻案した`aidlc-okf`も、
原典frontmatterにある`okf-agent-memory`へ変更する。13個の翻案skillは本文冒頭で原典・作者・翻案を示す。
元のLICENSEと固定commitを保持し、OKFに不足していた出典文書・原典LICENSEも配布する。
独自の進行用`aidlc`、操作用`aidlc-cli`、任意の`aidlc-github`、製品の担当名は維持する。
`natural-japanese-go`は既に接頭辞がなく、Go移植という変更を示す現名を維持する。

[工程skill導入](2026-09-12-stage-skills-natural-japanese-go-approved.md)と
[OKF skill導入](2026-09-12-aidlc-okf-skill-approved.md)の命名を置換する。
[共通化](2026-09-13-common-skills-codex-approved.md)の構造は維持するが、工程内のskill参照名を直すため、
該当本文と定義hashの不変条件だけを今回の修正に合わせる。旧記録・旧計画は履歴として保持する。
新規配置で検証し、既設fileや保存stateの更新・移行を実装しない。Go module・権限・承認の変更はない。


## 実装のloop証拠

work unit `upstream-skill-names`。開始HEADは`e7fc62493c08b56815c9954fbcc9e2a0b9723530`、開始treeはclean。
実装担当は単独writerで、commit・Issue・PR操作は親担当へ残した。

- slice1: `TestStageSkillsInstall`・`TestStageSkillsCollision`・`TestStageSkillsSymlink`・`TestUpstreamSkillNames`が新名未配置／衝突未検知でRED（終了1）。12folder/name・相互参照・13帰属とOKF LICENSE/sourceを整備し同じcommandでGREEN（終了0）。
- slice2: `TestOKFSkillReadApprovalPending`・`TestUpstreamSkillInitialization`は開始時に旧OKF入力を要求するためRED、`TestRelocateReferences`・`TestOKFSkillRelocate/known`は旧配置参照でRED（各終了1）。新OKF配置先へ接続し3commandがGREEN（終了0）。既存の欠落拒否だけの`TestOKFSkillInitialization`は最初からGREENだったため、正常開始の回帰testを追加した。旧名のfileを実際に作っても読取りを拒否する2ケースはALREADY_GREEN。
- slice3: `TestUpstreamSkillReferences`は工程・担当内の旧名、`TestCodexManifestParity`は69対71資材、helpの2testは旧OKF表示でRED（終了1）。名称参照と新71資材fixtureを揃えGREEN（終了0）。`TestProcedureBoundary`は名称変更前にALREADY_GREEN、変更後は旧工程hashとの差を検出したため、新しい承認済みfixtureへ追従してGREEN。旧69資材fixtureは履歴として保持し、本文hashの既存除外をinstaller／renderer両testから除去した。
- slice4: `TestStageSkillsEvidence/valid`・`TestMemoryMetadataCommandEvidence/valid`が新名の実読取りを証明できずRED（終了1）。証拠readerを新名へ揃え、未読・stdout不一致・失敗等の負例を維持してGREEN（終了0）。

slice3のtest参照置換で旧OKF名assertionも新名へ変える作業上の誤りがあり停止した。
親が計画表から旧名`aidlc-okf`へ一意に復元できることを確認して再開を指示し、正しいtestでREDを再実行した。
誤ったtestの失敗はRED証拠に含めない。

名称変更・帰属挿入・参照更新を正規化して開始HEADの既存core原稿65fileと比較し、それ以外のbytes差分がないことを確認した。
既存12skillのLICENSE bytesは不変。新OKF LICENSEのSHA-256は
`8de41d98bcdfca0d6a0399820f4d79e0738ff53a5fb04d605d1ecad7871cbafb`で取得済み原典と一致する。
親は12名称を各固定commitの原典frontmatterへ照合済み。作者は原典LICENSEの名義に対応する。
本家の新機能・権限は取り込まず、5担当名、工程入出力、Sensor、保存形式は維持した。

末尾に変更Go fileへgofmtを適用し、次のtargeted commandをすべて再実行して終了0を確認した。

| command | 終了code |
| --- | --- |
| `go test -count=1 ./src/internal/install -run '^Test(UpstreamSkill\|StageSkills\|NaturalJapaneseSkill)'` | 0 |
| `go test -count=1 ./src/internal/app -run '^Test(UpstreamSkill\|OKFSkillRead\|StageSkillsRead)'` | 0 |
| `go test -count=1 ./src/internal/flow -run '^Test(UpstreamSkill\|OKFSkillInitialization)'` | 0 |
| `go test -count=1 ./src/internal/install -run '^Test(Relocate\|OKFSkillRelocate)'` | 0 |
| `go test -count=1 ./src/internal/install -run '^Test(CodexManifest\|ProductAgentAssets\|UpstreamSkill)'` | 0 |
| `go test -count=1 ./src/internal/flow -run '^TestProcedureBoundary'` | 0 |
| `go test -count=1 ./src/internal/cli -run '^Test(RuleSkillSeparation\|CheckHelp)'` | 0 |
| `go test -tags=integration -count=1 ./src/cmd/aidlc -run '^Test(StageSkillsEvidence\|MemoryMetadataCommandEvidence)$'` | 0 |
| `go test -count=1 ./src/harness/codex -run '^TestDistribution$'` | 0 |
| `go test -count=1 ./src/internal/install -run '^Test(OKFSkillInstall\|InstallMemoryCommandGuidance\|RuleSkillSeparation\|Assignment\|CodeKB\|Flow)'` | 0 |

`git diff --check`は成功。全package・race・vet・配布E2E・実Codexはloopでは実行していない。
独立review、final検証、PR checksとmergeは親の後続gateであり、以前の実機読取りを新名の証拠にはしていない。
