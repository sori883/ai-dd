# codekb配置変更の実装証拠

Issue #151、work_unit_id: `knowledge-codekb`、verification_mode: `loop`。
開始HEAD: `c766ebb0b100e89b9a0496c3c3435c4e57ed84f2`。
[ユーザーの直接依頼](2026-09-09-knowledge-codekb-approved.md)と[実装計画](../../design/knowledge-codekb-plan.md)に基づく単独writerの実装。

## 実装結果

新規installとSpace作成は`aidlc/spaces/<space>/knowledge/codekb/index.md`を配置し、Bundle索引から参照する。
共有解析・構成図の既定path、統合時の機能Knowledgeの必須配置、開始前修復の既定path、4 Stage手順とhelpをcodekbへ揃えた。
現在のCLI例、開発手順、関連fixtureと配布snapshotも追従した。外側knowledgeとdesign/adr/rules/log、OKF typeは保持した。
配布sourceの内側folderをrenameし、利用者の既存配置への移動・削除・移行は行わない。

metadata selectorに一致する入力文書はBundle内の別pathでも選べる。codekbへ移した共有文書も既存のcurrent/accepted契約で扱う。
開始前修復も既存のmetadata selectorに一致すれば旧名を含む別Concept pathを使用でき、一般memory操作に旧名禁止を追加していない。
統合の必須機能Knowledge出力だけは新しいcodekb配置を要求する。

## TDD実測

### 1. 配布

`go test -count=1 ./src/internal/install -run '^TestCodeKBDistribution'`
はRED exit 1。default/teamの両Spaceでcodekb/index.md欠落・索引リンク欠落・旧内側folder残存を検出。
source folder renameと索引更新後、同commandはGREEN exit 0。

### 2. Sensorと修復hook

`go test -count=1 ./src/internal/flow ./src/internal/minimal -run '^TestCodeKB'`
の初回にはfixtureのBegin不足による`intent begin required`が含まれたため、これはREDに数えず停止して親へ報告した。
親の明示したfixture補正に従ってBeginを追加し、hookでは未開始状態と一般編集拒否を確認した。
また、旧名の入力もmetadata selectorに一致すれば修復可能という既存契約にtest期待を訂正した。

補正後、同commandはRED exit 1。`TestCodeKBIntegrationFeature/codekb`で
`declared feature Knowledge output required`を検出し、`knowledge`で旧配置の代用を検出した。
機能Knowledgeのprefix修正後、同commandはGREEN exit 0。
共有解析/図のcodekb配置と開始前の正規修復はALREADY_GREEN。既定pathも計画どおりcodekbへ揃えた。
codekbへのmemory create/search/showの補足testもALREADY_GREENであり、memory productionの変更は不要だった。

### 3. 手順・help・fixture

`go test -count=1 ./src/internal/cli ./src/internal/install -run '^TestCodeKB'`
はRED exit 1。help例、4 Stageのmemory案内、integrationの解析/図outputs、WORKFLOWのcodekb案内の欠落を検出。
更新後、同commandはGREEN exit 0。
関連fixtureは既定名だけを追従し、旧pathの拒否assertionと一般selector互換assertionは明示して保持した。
実CLI journey/live fixtureとworkspace配布snapshotは更新したが、結合実行は親finalへ集約している。

## 検証の境界

変更Goファイルへgofmtを適用した。上記3つのtargeted commandを末尾に再実行し、いずれもexit 0。
`go test -count=1 ./src/internal/flow ./src/internal/minimal ./src/internal/cli ./src/internal/install ./src/internal/workspace ./src/internal/okfmemory ./src/internal/workflow`は7 package全て成功、exit 0。
`git diff --check`はexit 0。sourceと現在の開発手順への限定検索で、旧既定名が残るのは回帰testの意図的なassertionだけと確認した。
全体test/race/vet/build/結合E2Eは実行していない。独立reviewとread-only final、GitHub checks、PRとmergeは親担当。
