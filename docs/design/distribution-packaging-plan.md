# 配布物の生成と既設更新手順を仕上げる計画

## 背景、目的、実装許可

AI-DLCはGoの単一実行ファイルからCodex用のskill、agent、hook、工程定義を配置できる。
しかし現在のCIは6種類の実行ファイルをbuildするだけで、配布用の圧縮ファイル、照合用ハッシュ、
版の一覧を生成しない。既設更新も、比較・保全・復旧を利用者が追えるまとまった手順が不足する。
この計画では、同じ入力から配布物を生成し、展開後の実行と導入を検査し、既存データを保全する
更新手順を実証する。配る実行ファイルは引き続きaidlc一つとする。

ユーザーの「じゃまず一番から順次対応してもらえますか」は、実案件、配布・更新、利用者文書の
3項目を順に進める直接依頼である。実案件はPR #164でmainへマージ済み、Issue #163は完了した。
開始基準はmain `c5ec2e9f0243cf7382a90d4d65d45929dd5227ca`。
実装管理は [Issue #165](https://github.com/sori883/ai-dd/issues/165)。
この計画は2番目の配布・更新を、既存のGo単一binary・本家Codex配布・利用者データ保全の合意内で
具体化する。実装、Issue、review、final、PR/checks/mergeまで進める。

正式version、公開の可否、Go製品のライセンスは未確定である。実tag、GitHub Release、配布物の
uploadは行わず、localとCI内で候補を生成・検査する。public repositoryのCI artifact uploadも
非公開とは扱わない。新しい製品CLI、更新台帳、自動移行や自動rollbackは追加しない。
これらの重大な契約を後で追加する場合は、具体案への承認が必要となる。

## 本家との対応と確認範囲

参照した固定本家はAI-DLC 2.6.123。元checkoutの実装、canonical Codex dist、配置済みdistの
versionが一致し、共通aidlc-utility.tsの内容も一致していることを調査担当が確認した。

- Codex導入は配布treeのコピー、設定merge、Git確認、hook trust確認を行う手順である。
- 共通CLIのhandleUpgradeは未提供のエラーへ到達する。package.ts --checkは生成物の比較であり、
  利用者編集の識別や既設更新ではない。
- 更新・rollbackは実行中の処理を停止し、対応するengine・library・hooks・生成物を組で交換する。
- Cursor専用installerのreceiptによる編集検出はCodexの既存契約へ流用しない。

Go単一binaryへの内包は既承認の差分である。この計画で自動更新などの新しい意図的差分は加えない。
最新upstream全体、別ハーネスの更新互換性、WindowsでのCodex実hook動作まで確認したとはしない。
詳しい根拠と参照pathは今回のRAMへ保存する。

## 配布物の契約

新しい `src/cmd/aidlc-dist/` は開発者専用の梱包commandとする。利用者へ追加binaryを要求しない。
Go標準ライブラリだけを使い、既存Go moduleの追加・更新や外部tool導入を行わない。

入力は、同じcommitから通常のGo buildで作った実行ファイル、version、40桁のsource commit、
Go toolchain版、対象のOS/CPUである。CLIの開発用引数は `--input-dir`、`--output-dir`、
`--version`、`--commit`、`--go-version`、`--targets` とする。targets省略時は以下の6種類すべて、
指定時はその非空subsetをカンマ区切りで指定する。重複と未知のtargetは拒否する。
通常の検証版は `dev-<短縮commit>`、公開版は承認後の値を与える。

| target | 入力file | archive |
| --- | --- | --- |
| darwin/amd64、darwin/arm64 | aidlc-darwin-CPU | aidlc_VERSION_darwin_CPU.tar.gz |
| linux/amd64、linux/arm64 | aidlc-linux-CPU | aidlc_VERSION_linux_CPU.tar.gz |
| windows/amd64、windows/arm64 | aidlc-windows-CPU.exe | aidlc_VERSION_windows_CPU.zip |

archiveの中身は `aidlc` または `aidlc.exe` の通常file一つ。
Unix側のmodeは0755、Windows側は通常fileとして展開できる値を使う。archiveの時刻、所有者、
順序等を固定し、同じbinaryとmetadataから同じbytesを生成する。build時刻は入れない。
`manifest.json` にschema_version、version、source_commit、go_version、各targetのbinary名・
binary/archiveのSHA-256とbyte数を保存し、`SHA256SUMS` に各archiveとmanifestの照合値を並べる。
manifestは梱包時の入力と実内容の対応表であり、署名や出所の第三者認証ではない。

入力fileは空でない通常fileのみ。symlink、欠落、不正version/commit/toolchain、pathを含む値、
未知targetを出力作成前に拒否する。versionは安全な英数字と `. _ -` の範囲とし、先頭英数字、
長さ上限128、`..`を拒否する。commitは小文字16進40桁、toolchainは空白のないgo1系の版とする。
入力binaryとsourceの対応はbuild手順とCIの固定checkoutで管理し、渡された任意binaryの由来を
梱包commandだけで証明できるとは説明しない。

出力先は未存在のdirectoryを必須とし、既存出力を上書きしない。保存途中の失敗は非zeroで返し、
残った候補を成功扱いしない。確認後に別の新しい出力先で再実行できる。製品dataへ書き込まない。

## CIと導入確認

新しい `.github/workflows/distribution.yml` はpush/PRで実行し、権限はcontents: readを維持する。
既存CIの品質検査を複製せず、配布物の生成・展開を確認する。新しい外部Actionは追加しない。
checkoutとsetup-goは既存workflowの固定SHAを使う。

1. Ubuntuでdarwin/linux/windows × amd64/arm64をCGO_ENABLED=0、-trimpath、既存のVersion/Commit注入で
   buildし、6archive、manifest、SHA256SUMSを生成・検査する。
2. ubuntu-latest、macos-latest、windows-latestでnative binaryをbuildして梱包し、そのarchiveを
   展開したbinaryでversion/help、fresh install、既存file拒否、参照移転の限定確認を行う。
3. 実行したruntime.GOOS/GOARCHを出力する。6targetのcross-buildと3OSのnative実行を区別する。

runner labelはGitHub公式資料で確認した標準runnerを使う。公開repositoryで利用できる範囲に限り、
有料runner、認証情報、追加権限、Release公開は要求しない。
Windowsの実Codexがhook commandをどう実行するかは別の確認項目であり、file生成成功を実hook成功へ
読み替えない。今回hook内容は変更しないため、macOS/Codex 0.153.4での通常trust実測はPR #164を根拠とし、
同じ実案件を繰り返さない。新しい不整合が判明した場合は成功条件を緩めず報告する。

## 比較・更新・復旧の手順

`docs/distribution.md` に以下を初心者向けにまとめ、既存development.mdから参照する。

1. 既知のAI、worker、背景処理を停止し、予約と保存途中の状態を確認する。時間だけで予約を解放しない。
2. 旧binary、配置した製品file、独自hook/config、全Spaceとruntimeを保管する。backupを実物で確認する。
3. 新binaryを別pathへ保存し、別の空Git projectへfresh installする。実利用先へのinstallを再実行しない。
4. 製品fileを一覧で比較する。共有Rule、Knowledge、ADR、Intent/state/history、assignment/runtime、
   利用者config/AGENTSは新しいseedで置き換えない。比較元がなく編集の由来を特定できなければ、
   自動的に製品fileだと決めずそのfileの扱いを確認する。
5. 確認した製品資材を対応する組で切り替える。独自hooksは保持して製品handlerだけを手動mergeする。
   stagingに埋め込まれたroot/binaryの絶対pathを実利用先へ補正する。既存relocateを使える3fileの
   条件と、独自編集がある場合の停止条件を明記し、移転を版更新と混同しない。
6. 通常のCodex trust確認と許可/拒否の対照後に再開する。定義hashが変わる場合は新Intentを使う。
7. 失敗したら作業を停止し、対応する旧binaryと製品fileを組で戻す。進捗・Knowledge・予約を削除して
   未実行や空きへ見せかけない。新版で作ったIntentの旧版再開を保証しない。

隔離したtest用projectで、独自hookと利用者dataを置いた比較・製品file切替・参照補正・元file復元を実行し、
dataのbytesが不変であることを検査する。これは文書化された手順の検証であり、自動updaterの提供ではない。
実際の利用環境への適用はこの開発PRに含めない。

## 所有範囲、TDD、検証

専用作業場所は `/Users/const/sori883/ai-dd-distribution`、branchは`codex/distribution-packaging`。
元checkoutと完了したpilotの状態は変更しない。1 Issue、1 work unit、1 Go writerで進める。
親はIssue/PRと許可gateを管理し、実装中は対象fileを編集しない。

| 所有file | 内容 |
| --- | --- |
| src/cmd/aidlc-dist/main.go、archive.go、manifest.goと対応test | 開発用CLI、梱包、照合一覧 |
| src/cmd/aidlc-dist/distribution_integration_test.go | build tag integrationのarchive展開・導入・手動更新手順の実証 |
| .github/workflows/distribution.yml | 6target梱包と3OS native smoke |
| docs/distribution.md、docs/development.md | 配布、比較、切替、復旧手順と参照 |
| docs/design/distribution-packaging-plan.md、今回RAMと索引 | 親の事前計画、writerの実測記録追記 |

work_unit_idは`distribution-packaging`、verification_modeは`loop`。
新APIに必要な型・signature・空の返値だけのcompile scaffoldを許可し、動作はtestのrunnable RED後に作る。

| 順 | TDD項目 | exact targeted command |
| --- | --- | --- |
| 1 | tar.gz/zipのbinary名・bytes・mode、target対応 | go test -count=1 ./src/cmd/aidlc-dist -run '^TestArchiveLayout$' |
| 2 | 再現性、manifest/SHA256SUMSの実内容との一致 | go test -count=1 ./src/cmd/aidlc-dist -run '^TestArchiveReproducibility$' |
| 3 | 不正入力、symlink、欠落、既存出力、保存失敗を拒否・保全 | go test -count=1 ./src/cmd/aidlc-dist -run '^TestArchiveRejectsInvalidInput$' |
| 4 | 開発用CLIのhelp・引数・終了code・対象subset | go test -count=1 ./src/cmd/aidlc-dist -run '^TestDistCommand$' |

native E2Eは実装末尾にtestを追加しfinal/CIで実測する。人工的なREDは作らず、native未実測を成功に数えない。
loop末尾は上記4command、`go test -count=1 ./src/cmd/aidlc-dist`、変更Goのgofmt、git diff --check。
各sliceのtest先行・実exit・出力を保存し、証拠の.goコピーはmodule配下へ置かない。
親は末尾でdiffと4targetedを一度確認し、独立reviewへ渡す。

reviewはversion/target取り違え、archive内容・照合値、上書き拒否、metadataの説明、保全手順、
CIの権限・公開境界を確認する。review担当は対象fileを変更せず必要なtargeted確認だけを行う。

差分安定後のread-only finalは、全packageのshuffle testとrace、vet、tidy-diff、gofmt -l src、
git diff --check、既存workspace/okf integration、TestFlowJourney、TestAssignmentJourney、
新 `go test -tags=integration -count=1 ./src/cmd/aidlc-dist -run '^TestDistributionJourney$'`、
6targetの実build・全archive生成・照合をまとめて実行する。3OSは対象PRの新CI成功で確認する。
失敗をskipや対象縮小で成功に変えず、修正が必要ならloop/review後にfinalを取り直す。

## 完了条件と公開前の残件

6archiveの生成・照合、3OSのnative導入確認、隔離fixtureの更新/復旧と利用者data保全、
独立review、local final、対象PRの全checks、mainへのmergeをもって、この実装範囲を完了とする。
続く3番目でREADME全体と日常利用の案内を更新する。
正式公開の版、公開範囲、ライセンス、実hookのOSごとの確認範囲は具体的な成果物を提示した段階で確認し、
未公開の候補を一般配布済みと説明しない。
