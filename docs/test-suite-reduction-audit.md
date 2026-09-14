# テストの削除・統合候補一覧

2026年9月14日。AI-DDの現行テストを、「削るとどんな現実的な不具合を見逃すか」で調べた監査結果です。

最初に取り除く価値が高いのは、**同じテストの再実行、廃止された経路しか検査しないケース、テスト用補助処理の自己検証**です。次に、同じ入力検査を各層で繰り返す表、資材の文章を固定する検査、不要なGit操作・ビルド・解凍を統合します。

## 対象と調べ方

- 対象commit: `adc4682ca265c9bda4434a94d9d99cff1f4b9394`。mainへマージ済みの [PR #203](https://github.com/sori883/ai-dd/pull/203) を確認して固定しました。
- 作業場所: `/Users/const/sori883/ai-dd-release`、branch `codex/test-suite-audit`。元の `/Users/const/sori883/ai-dd` にある未commit変更には触れていません。
- `src/` の **216個の `_test.go`、35,850行**を全件確認。integrationタグ、OS限定、実Codexを使う試験、補助関数だけのファイルも含みます。CIの2 workflowも確認しました。
- 隠しdirectoryも確認し、`.agents/skills/golang-cli/assets/examples/cli_test.go` は外部由来skillの説明用サンプルとして別に確認しました。製品suiteの216個には含めません。このサンプルの削除では通常テスト時間は減りません。
- テスト本文を読み、削減候補について対象の実装・呼出先・残すテストと照合しました。Go test関数を直接再呼出する箇所、前段の拒否に隠れている入力、productionを呼ばない補助処理も確認しています。
- 今回は**静的監査**です。テスト・実Codex・mutation実験は新たに実行していません。「検査へ届かない」はコードの到達順による判断です。削除後の成功を確認したという意味ではありません。
- テスト・製品コード・CI設定・Issue・PRは変更していません。この報告とRAMだけを追加しました。候補の削除やCI gate変更は、実施する範囲を決めた後の作業です。

件数・カバレッジの維持は判断基準にしていません。各候補には、位置、削る範囲、残す保証、失う保証、コスト、判断の確度を記載しました。重複する提案を含むため、候補番号を足して「削れるテスト数」とは数えません。

## 優先して整理するもの

| 優先 | 候補 | 理由 |
|---|---|---|
| 1 | [P01](#p01)、[A01](#a01)、[F01](#f01)、[C01](#c01)、[P17](#p17) | Testを別名で再実行するだけ。固有の保証を増やさない |
| 1 | [A02](#a02)、[F06](#f06)、[F09〜F16](#f09)、[C17](#c17)、[C28](#c28) | 名前が示す条件を検査しておらず、別の理由で先に失敗する |
| 1 | [P04〜P05](#p04)、[P22〜P24](#p22)、[C03〜C04](#c03)、[C18〜C19](#c18) | 標準処理・テスト補助関数・ログ出力だけの確認 |
| 2 | [F38](#f38)、[A36](#a36)、[C26〜C27](#c26)、[P07〜P08](#p07)、[P20](#p20) | 不要なGit・ビルド・圧縮・解凍。実行する保証を変えず準備を軽くできる |
| 2 | [P11〜P16](#p11)、[A18〜A20](#a18)、[F27](#f27) | 資材全文のhash・文章断片・存在確認が重複し、文章修正の負担を増やす |
| 3 | [A07〜A17](#a07)、[A21〜A30](#a21)、[F17〜F37](#f17) | 同じ検査の多層・組合せ反復。入口固有の転送・保存保証を残して統合する |
| 条件付き | [C13〜C25](#c13)、[C29〜C35](#c29)、[F41](#f41)、[W01〜W05](#w01) | 古い実験や低頻度試験、CI重複。現行の合否gate・復旧保証を整理してから減らす |

本文の「すぐ削除」は既存の残すテストで保証を維持できるもの、「統合」は固有のassertを移してから消すもの、「縮小」は同値ケースや準備だけを減らすものです。`alias` は別名で同じTestを呼ぶ入口、`fixture` は試験用のデータ・directory等の準備、`oracle` は結果の正しさを比較する独立した期待値を指します。CASは保存versionを照合して古い状態での上書きを防ぐ仕組みです。

## 実行コストと保守負担の根拠

過去の `ai-dd-validation/consolidated-release-202/final-04/` は、今回のcommitと同じtreeの検証記録です。以下はその観測値であり、削減後の見積もりではありません。packageは並列実行され、ビルド・マシン負荷も違うため合計して短縮時間にはできません。

| 記録 | 観測値 | 今回の判断との関係 |
|---|---:|---|
| `test.log` / internal/flow | 90.072秒 | 多数の実配置・Git・保存準備を繰り返すF群 |
| `test.log` / internal/app | 43.269秒 | 重複install/session/approval準備を含むA群 |
| `test.log` / bootstrap | 29.914秒 | test exeを使う繰り返し圧縮・process起動 |
| `test.log` / cmd/aidlc | 24.860秒 | 実験helper・process・buildを通常suiteに含む |
| `test.log` / internal/assignment | 21.426秒 | Git fixture・保存容量への反復到達等 |
| `test.log` / internal/install | 21.083秒 | 多数の資材配置・一式生成 |
| `candidate.log` / DistributionArchives、ReleaseCandidateMetadata | 25.56秒、19.89秒 | 同じ検証helperを別名で実行した実例（P01） |
| `push-distribution.log` / Windows BootstrapPowerShell | 31.34秒 | 2engine×2形式×8mode（P10）。実候補bootstrapは別に6.88秒 |

`final-03/ci-failure.log` には、`TestBootstrapOutputStreams` がcoverage子processの追加stderr `program not built with -cover` で失敗した記録があります。製品出力の混線ではありません。固定文字列・test helperへの過度な依存が、実際に修正負担になった例です。Windowsのshort/long path表記の違いでもテスト期待値の修復が必要でした。一方、PowerShellの呼出元復帰やStart失敗の診断は実際の製品修正を伴ったため、その検査は残します。

同じテストでも通常suiteで毎回走るものと、integration/live条件がなければskipされるものでは削減効果が違います。後者の整理は主に保守負担・誤った合格判定を減らすものです。

## 詳細の読み方

- **P**: 配布、インストーラー、共通資材、自然日本語。
- **A**: アプリ接続、CLI、Space、OKF。
- **F**: Intent・Unit進捗、担当管理、工程定義。
- **C**: CLIの一周試験、実験・観測helper。
- **W**: CIの重複実行。W06は残す項目です。

位置のリンクは監査commitに固定しています。後日のコード変更で行がずれても、この判断に使った内容を確認できます。

## 配布・インストーラー・共通資材・自然日本語：削除・統合候補

この節の P 番号は、配布を含む残り52 test fileの監査結果。位置は監査対象commitで固定した行番号である。「すぐ削除」は既存の生存テストで保証を維持できるもの、「統合後に削除」は固有のassertを移す必要があるもの、「縮小」は一部のケースや準備だけを対象にする。

<a id="p01"></a>

### P01 — 配布候補を同じ内容で再実行する4入口を削除

- 対象: [src/cmd/aidlc-dist/distribution_integration_test.go:17](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/cmd/aidlc-dist/distribution_integration_test.go#L17) `TestDistributionArchives`、`:120` `TestDistributionJourney`、`:122` `TestNaturalJapaneseDistributionArchives`、`:124` `TestNaturalJapaneseDistributionJourney`。
- 残す: `release_integration_test.go:218` `TestReleaseCandidateMetadata` と `:274` `TestReleaseCandidateNative`。ArchivesはMetadataと同じ引数で同じhelperを呼び、他3つはTestを直接呼ぶだけ。
- 失う保証: 別の名前・一時directoryで同じ処理をもう一度行うことだけ。通常の不具合への独自保証はない。実行順に依存する偶然の検出を常時重複の根拠にしない。
- コスト: 過去の同tree検証でArchives 25.56秒、Metadata 19.89秒。Nativeの再実行には一式の検査・導入・各CLI起動・移転が付く。現在のCI PackageもArchivesとMetadataを両方選択しているため、削除と同時に選択式を生存入口へ直す。Nativeを再呼出する別名は、広いintegration実行時の重複である。共有helperが残るためfile全体は削除しない。**すぐ削除、確度高。**

<a id="p02"></a>

### P02 — ライセンス同梱の重複を削除

[src/cmd/aidlc-dist/release_test.go:57](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/cmd/aidlc-dist/release_test.go#L57) `TestVersionedAssetsLicense` を削除。`bundle_test.go:12` `TestBundledRelease` が同じLICENSES/PRODUCTを含む6環境の一式を検査する。失う保証はなく、6archiveを別途作ってLinux分だけ確認する実行を省ける。必要なライセンス本文の正本照合はP07の生存検査にも残す。**すぐ削除、確度高。**

<a id="p03"></a>

### P03 — 存在しないflagの値検証を削除し、不正CLIの準備を簡略化

- [src/cmd/aidlc-dist/main_test.go:85](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/cmd/aidlc-dist/main_test.go#L85) `TestProductCLI` は廃止済み `--product` に `bad` を渡すが、値の検査へ届かずunknown flagで止まる。同file `TestDistCommand/unknown flag` へ集約して削除。
- `TestDistCommand` のmissing args / unknown flag / positional / empty・duplicate・unknown targetは、入力生成より前の構文拒否を検査する。各行の `releaseFixture` による30個の入力file・license tree生成をやめ、未存在の入出力pathと必要な引数だけを渡す。出力が作られないassertは残す。
- 失う保証は重複unknown flag拒否と、読まれないfixtureの存在だけ。公開引数、終了値、出力抑止の保証は残る。**削除・準備削減、確度高。**

<a id="p04"></a>

### P04 — 配布test自身のpath・選択helperの自己検証を削除

- `distribution_integration_test.go:51` `TestDistributionBinaryPath` と、唯一そのtestから呼ばれる `fixtureBinaryPath`。期待値も対象も `filepath.EvalSymlinks` で計算する。
- `release_integration_test.go:424` `TestReleaseCandidateProjectDirectory`。test用helperのMkdirAll/Abs/EvalSymlinksを検査する。Nativeでそのdirectoryを実際に使い、末尾でもGit非存在を検査している。
- `release_integration_test.go:190` `TestReleaseCandidateNativeSelection`。test用 `selectBundleNative` の名前計算とfixture欠損エラーの検査。実際のNativeが自身のOS/CPUのarchiveを選んで起動する。
- 製品のroot正規化や自動選択は製品tests・Nativeに残る。削るとtest helper単体の誤字の発見が実利用時になるが、stdlibの同じ式を二重に実行する価値は低い。**すぐ削除、確度高。**

<a id="p05"></a>

### P05 — bootstrapのtest helperだけの2テストを削除

[src/bootstrap/bootstrap_test.go:195](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/bootstrap/bootstrap_test.go#L195) `TestBootstrapOutputStreams`、`:219` `TestBootstrapProjectDirectory`。前者は偽test processと `bytes.Buffer / exec.Cmd.Run`、後者は `os.Stat / os.SameFile` を使うtest helperだけを検査する。

実製品のstdout分離と実配置は `TestBootstrapCandidateNative`、project引数の転送は `TestBootstrapPowerShell`、終了値0/17と呼出元復帰は `TestBootstrap` / PowerShellに残す。削除で失うのはhelperの局所保証だけ。前者には、製品不具合ではなくcoverage子processの追加stderrだけで失敗した実績もある。専用 `BOOTSTRAP_STREAM_HELPER` 分岐も未使用になるため整理可能。**すぐ削除、確度高。**

<a id="p06"></a>

### P06 — 配布の判定器を試す大量の文字列・struct変形を縮小

対象は [src/cmd/aidlc-dist/release_integration_test.go:26](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/cmd/aidlc-dist/release_integration_test.go#L26) `TestReleaseCandidateMetadataValidation`。

- `native version output` は5製品の手作り文字列を1文字・改行等ずつ変え、test用validatorの文字列完全一致を試している。ここは削除し、Nativeで実5バイナリの版・commit出力を検査する。
- `binary build identity` の手作り `debug.BuildInfo` のfield変形も、単なる比較条件の総当たりを削る。実30構成の製品名・target・toolchain・commit照合はMetadataに残す。
- candidate mutationのschema/version/target/modeは、製品 `TestBundleManifest` / `TestBundleArchiveComplete` の拒否と重複。異常配布を上位から1つ拒否する接続試験へ集約する。
- **source/licenseを正本と違う内容にして、manifestとchecksumまで再計算する改変**は別物。製品validatorと自己整合していても誤配布になる現実の不具合なので、独立した正本照合の代表を残す。

削減でtest用checkerの細部の取り違えを局所的には捕捉しなくなる。実バイナリ・実sourceによる照合と正本改変の負例を維持するなら、単純比較の全直積より価値が高い。**縮小、確度高。**

<a id="p07"></a>

### P07 — ライセンス用の重いfixtureと同値の改変ケースを統合

[src/cmd/aidlc-dist/candidate_license_test.go:20](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/cmd/aidlc-dist/candidate_license_test.go#L20) `TestCandidateLicense` は4つのpathごとにmissing/modified/extraを行い、毎回30入力・6archiveを作る。使用するのは1環境分で、extraは選んだpathを使わない重複もある。

fixtureは一度だけ生成し、対象archiveのcopyを改変する。missing/modified/extraは各代表path1つへ、製品別の必要ライセンス集合と本文はMetadata/Nativeの実正本照合へまとめる。同file `:179` `TestCandidateLicenseExecutableMode` はtest用mode checkerの検査を全6archive生成で行うため、P06の実mode検査と `TestBundleArchiveComplete/mode`、`TestBundleArchiveRejectsUnsafe` へ集約する。

失うのは同じcheckerのpath違い総当たり。**必要ライセンスの製品ごとの差と実行権限は削らない**。準備・解凍・圧縮のコスト大。**統合後に削除・縮小、確度高。**

<a id="p08"></a>

### P08 — 同じarchiveを製品ごと・testごとに何度も解凍しない

[src/cmd/aidlc-dist/release_integration_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/cmd/aidlc-dist/release_integration_test.go) の `validateCandidateBundle / verifyReleaseCandidate / TestReleaseCandidateNative` と、`candidate_license_test.go` の `validateCandidateLicenses`。

現状は一式のvalidator、5製品分のlicense検査、build identity検査で同じbundleを繰り返し解凍・走査する。Nativeも初めに全6targetのMetadata相当を実行してから、自OSのarchiveを再検査する。**同じ入力を一度解析し、entries/modeを各assertへ渡す**形へ統合する。6target静的検査はPackage側、Nativeは自OSの実行に必要な検査を担当する。公開直前の別境界で同じ配布候補を再照合することは維持する。

assertの内容を削らなければ失う保証はない。parse結果の共有は同じimmutable入力内に限定し、改変ケース間では共有しない。実行コストの削減余地大。**helper統合、確度高。**

<a id="p09"></a>

### P09 — 公開CLIから呼ばれない旧単品archiveのテストを退役

- [src/cmd/aidlc-dist/archive_test.go:39](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/cmd/aidlc-dist/archive_test.go#L39) `TestArchiveLayout`、`:196` `TestProductArchive`、`:277` `TestProductInvalid`。
- `manifest_test.go:17` `TestArchiveReproducibility` の旧schema 1・単品形式の検査。
- 現 `main.go` は常に `Product="all"` を設定し、`packageArchives` の旧単品branchへ公開CLIから到達しない。新一式は `TestBundledRelease` が配置・再現性を検査する。

**旧branchと一緒に退役する候補**。単品形式の内部呼出がないことを維持条件とし、現一式への変換を理由に旧形式を延命しない。ただし `TestArchiveRejectsInvalidInput:97` を丸ごと削ってはいけない。そこで使う `validateInputs` は新一式にも共通で、missing/リンク/不正入力の有効なケースを新経路へ移す。既存出力保全・途中書込失敗も新 `packageRelease` で確認してから旧ケースを削る。共用 `archiveFixture` は残るcallerへ移す。**旧コード整理を伴う削除候補、確度高。**

<a id="p10"></a>

### P10 — bootstrapのshell×呼出形式×異常の全直積を縮小

[src/bootstrap/powershell_test.go:16](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/bootstrap/powershell_test.go#L16) `TestBootstrapPowerShell` は2 engine × 2呼出形式 × 8 mode。**PowerShell 5.1/7両方は残す**。各engineの公開scriptblock呼出で8 modeを確認し、`-File` は正常・子exit・catch経路の代表へ絞る案。checksum/missing/version/duplicate/symlinkごとに両呼出形式を繰り返さない。

失うのは特定異常とFile形式だけの組合せ不具合。失敗を呼出元へ返す処理は共通なので全直積の価値は低い。一方、scriptblockの呼出元復帰とcurl Start失敗は実際の修正対象だったため、engine差を消さない。前回Windowsでは全体31.34秒。`bootstrap_test.go:82` も含め、各modeで同じ大きなtest exeを圧縮する準備は1回に集約できる。**ケース・fixture縮小、確度中高。**

<a id="p11"></a>

### P11 — 全資材の固定SHAを3層で維持しない

- [src/harness/codex/manifest_test.go:13](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/harness/codex/manifest_test.go#L13) `TestDistribution`。
- [src/internal/install/manifest_parity_test.go:25](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/install/manifest_parity_test.go#L25) `TestCodexManifestParity`。
- [src/internal/flow/content_test.go:17](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/flow/content_test.go#L17) `TestProcedureBoundaryComposedWorkflow`（F05）。

同じ `five-cli-assets-sha256.json` を使い、説明文の一文字変更でも全体の固定期待値を修正する構造。**全文の固定hashは廃止候補**。必要asset/role/path集合、リンクの解決、原典license、未展開token拒否を正本1か所へまとめ、配置層では「生成したbytesがregular fileとして正しく保存され、umaskと返却Pathsも一致する」を残す。apostrophe入りroot/binaryのquote検査も残す。

失うのは全文の任意の差を検出する保証。説明の意味の正しさは現hashでも判定できない。ただし生成結果をそのまま期待値にするだけでは生成側の欠落も一緒に通るため、**独立した必須資材・権限・licenseの期待値は残す**。移植当時のbyte同一性を、今後の全ての文章編集を止める契約として持ち続けない。**統合後に固定fixture削除、確度高。**

<a id="p12"></a>

### P12 — fresh installの存在・件数検査を1つへ集約

- [src/internal/install/install_test.go:13](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/install/install_test.go#L13) `TestInstallFresh`。
- `flow_test.go:10` `TestFlowInstallAssets`。
- `workflow_test.go:15` `TestWorkflowDefinitionFresh` の存在・bytes・再install拒否部分。
- `documents_test.go:12` `TestDocumentDistribution`、`codekb_test.go:13` `TestCodeKBDistribution` のscaffold存在表。
- [src/harness/codex/content_test.go:12](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/harness/codex/content_test.go#L12) `TestContentDistribution` / `split_test.go:11` の固定15/5/71件assert。

P11の配置検査と `workspace/TestCreateSpaceScaffold` へ集約し、workflow.Load成功、codekb/indexへのリンク、小文字adrとtype、defaultと追加Spaceの作成という固有のassertだけ移す。件数自体は維持目的にしない。固定件数を削っても必須集合・余分な未展開templateの禁止を残せば、利用者が必要なfileを得られない不具合は検出できる。追加install/Space作成のコスト中。**統合後に削除、確度高。**

<a id="p13"></a>

### P13 — 文書の「単語が含まれる」検査を整理

主な対象を以下に全て示す。削るのは説明文の断片・同じcommon本文の反復検査であり、role/sandbox、hook matcher、selector/outputの構造検証は残す。

| 対象 | 削る部分・統合先 |
|---|---|
| `install/install_test.go:70` TestInstallRecoveryGuidanceAndContextLimit | 日本語の復旧案内断片。実復旧はapp/flowのtests、入口リンクは1つのリンク検査へ |
| `install/install_test.go:111` TestInstallMemoryCommandGuidance | bootstrap・stage・CLI skill・OKF skill・実helpを連結した単語表。誤ったfileでも他fileに語があれば通る |
| `install/install_test.go:156` TestFlowInstallInactiveResumeGuidance | helpを足して同じ文法・文章を再確認する表。inactiveの実許可はA10へ |
| `install/install_test.go:182` TestMemoryHelpPlacedSkill | help参照の文章断片。実help commandの解決はA20に残す |
| `install/install_test.go:196` TestOKFWorkLogInstalledGuidance | 5fileの反復でstage分を同じcommon+helpに置換している。共通本文は1回でよい |
| `install/flow_test.go:46` TestFlowInstallJapaneseProcedure | 日本語断片のbag of words。文章の意味・工程順はこのassertでは保証しない |
| `install/assignment_test.go:49` TestAssignmentContractDocumentRules | ADR/outputs/毎操作の日誌等の文面断片。記録・承認の実挙動とリンクを残す |
| `install/codekb_test.go:43` TestCodeKBGuidance | 4stageへ同じcommon+knowledgeを連結して語を確認する部分。実出力pathの構造検査は残す |
| `install/execution_plan_test.go:11` TestExecutionPlanDistribution | step_idやplan/approvalの文字列。実workflow構造・公開CLI文法へ |
| `install/stage_planner_test.go:10` TestStagePlannerDistribution | PLAN/rationale等の文面。roleとreadonly sandboxはProductAgentAssetsへ |
| `install/product_agents_test.go:11` TestProductAgentAssets | 5担当全員へ同じ契約の単語表。5担当それぞれのrole/sandboxと正しくescaped TOMLになる点は残す |
| `install/git_independent_test.go:10` TestGitIndependentInstall | verification_shaという語の有無。実Git不要の動作はC02/F38に残す |
| `install/rule_skill_separation_test.go:12` TestRuleSkillSeparationAssets | command断片、旧WORKFLOW語、同じ再install保全。必要な資材境界はP11/P12へ |

表の `install/` は全て `src/internal/install/`。意味が逆でも単語さえ含まれれば通るので、これらを消して失うのは言い回しの固定。自然な編集のたびに複数testを直す維持負担が大きい。CLI例の実行・リンク切れ・役割権限・licenseという観測可能な契約へ集約する。独立した文書レビューは必要だが、全文hashへ置換して問題を再導入しない。**assert削除・統合、確度高。**

<a id="p14"></a>

### P14 — 4 KiB上限を各資材testで繰り返さない

P13各test、`TestFlowInstallAssets`、`TestFlowInstallJapaneseProcedure`、`TestWorkflowDefinitionFresh`、`TestDocumentDistribution`、`TestRuleSkillSeparationAssets` に散在する `len(bootstrap)>4096` を削り、[src/internal/install/bootstrap_test.go:12](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/install/bootstrap_test.go#L12) `TestInstallBootstrapBinaryPathBudget` に集約する。長いabsolute pathを展開した後の実SessionStart contextを検査する方が強い。

失う保証なし。上限の実不具合検出は残り、各成功fixtureの同じ長さ確認と文章改修箇所を減らせる。**部分削除、確度高。**

<a id="p15"></a>

### P15 — skillの存在確認を繰り返さない

[src/internal/install/stage_skills_test.go:13](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/install/stage_skills_test.go#L13) `TestStageSkillsInstall`、`:80` `TestNaturalJapaneseSkill` は、同file `TestUpstreamSkillNames:124`、`TestStageSkillsReferencesResolve:93` と重なる。

必須SKILL/README/SOURCE/LICENSE、natural-japanese固有のwriting.md、原典名・作者・出典・翻案表示を1つの資材表へまとめる。すべての参照先が実在する検査は残す。`TestUpstreamSkillReferences:163` の旧接頭辞混入検査は、directory名だけの確認と違って本文リンクも見るため、同じ表へ統合しても捨てない。失うのは同じfileが非空であることの重複のみ。**統合後に削除、確度高。**

<a id="p16"></a>

### P16 — install衝突のskill名総当たりを縮小

- `stage_skills_test.go:33` `TestStageSkillsCollision` の11pathは同じpreflight処理。代表として後ろの資材path1つを `manifest_parity_test.go:117` `TestCodexManifestPreflight` へ集約する。
- `stage_skills_test.go:61` `TestStageSkillsSymlink` と `install_test.go:56` `TestInstallRejectsSymlink` は、nested parent symlinkでroot外へ書かず拒否する代表を残す。
- `install_test.go:273` `TestOKFSkillInstall/existing`、`split_test.go:12` `TestSplitCLIInstallConflict` は同じOKF SKILL衝突。一般collision検査へ集約する。

衝突検査を資材別に実装しているわけではない。失うのは資材名ごとの同値反復で、preflight時点の全file無変更とnested symlink拒否は残す。異なる3binary参照の接続は `TestSplitCLIDistribution` に残す。**ケース縮小、確度高。**

<a id="p17"></a>

### P17 — relocation成功・再試行を何度も一周しない

[src/internal/install/relocate_test.go:27](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/install/relocate_test.go#L27) `TestRelocateReferences`、`:189` `TestOKFSkillRelocate/known`、`split_test.go:30` `TestSplitCLIRelocation`、`rule_skill_separation_test.go:53` `TestRuleSkillSeparationRelocate/success` を統合。

1つの往復に明示3binary、hook参照、ユーザーhook保全、同要求再試行を残す。`git_independent_test.go` 末尾の `TestRelocateReferences(t)` 呼出は単純な再実行なので削除する。`release_test.go:280` `TestInstallerCommandRelocation` は公開導入adapter経由という別の接続保証を持つため残す。失う保証なし。重複install・全file書換え・再読込のコスト中。**統合後に削除、確度高。**

<a id="p18"></a>

### P18 — relocationの同じ資材編集・欠損表を一本化

`relocate_test.go:136` `TestRelocateRejectsSymlinkAndEditedSkill`、`:189` `TestOKFSkillRelocate` のedited/missing、`:231` `TestRelocateComposedContractEditRejected`、`rule_skill_separation_test.go:53` のmissing/edited/legacy/symlink。

資材の既知bytes照合は同じ機構。代表のedited/missing/symlinkを残し、資材名ごとの重複を削る。特にlegacyは新SKILL欠損に無関係な旧WORKFLOW fileを足しただけなので、missingと同じ。

`TestRelocatePartialAndConcurrentRetry:102` の保存途中・競合時の保全・retry、`TestRelocateSameReferencesAndLock:168` は残す。partialの固定 `Paths/Pending` 件数・順番への依存は、公開済み集合／未保存集合とユーザーfile不変の検証へ寄せる。失うのは同一validatorの資材名別反復。各資材を処理対象へ含め忘れる不具合はP11の集合で守る。**統合後に削除、確度中高。**

<a id="p19"></a>

### P19 — release導入の重い成功fixture・異常表を縮小

- [src/internal/install/release_test.go:387](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/install/release_test.go#L387) `TestReleaseBundleDownloads` の「2回だけ取得」は、`TestReleaseAssetValidationCandidate:114` の正常installへassertを移す。
- Candidateのschema/余分なpathは下位 `TestBundleManifest / TestBundleArchiveComplete` が担当し、上位は正常、transport hash不一致、選択版不一致、異常時に配置しない代表を残す。
- `TestInstallFailure:40` の取得不能は `TestReleaseAssetValidationOffline:263` に失敗代表をまとめる候補。ただしofflineが選択targetだけで完結する正常境界は残す。
- `TestReleaseLicenseRetention:312` のfresh分は正常installへ統合できるが、衝突・部分保存・移転時のlicense保全は独自の実害があり、まとめて削らない。

失うのは同じvalidator/正常配置の反復だけ。DownloadのHTTPエラー・HTTPS以外へのredirect拒否、上限、offline入力は別実装なので残す。**統合後に削除・縮小、確度高。**

<a id="p20"></a>

### P20 — lockで止まる要求のために配布一式を生成しない

[src/internal/install/release_test.go:171](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/install/release_test.go#L171) `TestInstallReservation`。第1要求がlockを保持する間の第2要求は、取得へ進まず拒否することを調べる。この第2要求にも `candidateFiles(t)` で6archiveを生成している。

第2要求には「呼ばれたら失敗するFetch」とpathだけを渡す。lock拒否、取得なし、先行処理の成功はそのまま検査する。失う保証なし。並行性のtestを消すのでなく、競合を起こす前の不要な圧縮・file生成を省く。**準備削減、確度高。**

<a id="p21"></a>

### P21 — archive拒否の表を所属するvalidatorへまとめる

[src/internal/install/release_test.go:232](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/install/release_test.go#L232) `TestReleaseAssetValidationUnsafeArchives` はInstallReleaseを呼ばず、下位 `release.Unpack` を直接検査する。低層 [src/internal/release/bundle_archive_test.go:29](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/release/bundle_archive_test.go#L29) `TestBundleArchiveRejectsUnsafe` とデータの種類が重なる。

ただし **UnpackとunpackBundleは別実装で、UnpackはpackageReleaseのsource読込みにも使われる**。片方のtestだけ残して他方の保証まであるとは扱わない。低層packageへ同じ不正archiveデータを集約し、必要な両入口を確認する。独立assert数を減らせない箇所でもfixture・誤った所属を整理できる。tar/zipは別decoder、親子衝突の順序も別事故なので両方保持。**fixture統合、削除による時間削減は小。**

<a id="p22"></a>

### P22 — 出力をログへ出すだけの自然日本語テストを削除

- [src/internal/naturaljapanese/analyzer_test.go:33](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/naturaljapanese/analyzer_test.go#L33) `TestAnalyzerExamples`。5文のtokensをLogへ出すだけで品詞等の意味をassertしない。`TestAnalyzer` のmapping、`TestMorph` の意味検証を残す。
- `fixtures_test.go:8` `TestFixtures`。ai-smellyの件数が0でないこと、category/lineが存在することだけで、naturalの誤検知ゼロさえ要求しない。`TestSurface`、`TestReport`、CLIの `TestCommandAnalysis` が具体的な検出を確認する。

失うのはサンプルがpanic/errorにならないことと弱い件数条件。どの不自然な表現を誤検知・見逃すかの判定にはなっていない。意味ある既存例を残せばログ専用の重複解析は不要。testdataは他callerを確認して未使用分だけ整理。**すぐ削除、確度高。**

<a id="p23"></a>

### P23 — json.Marshalした直後のjson.Validを削除

[src/internal/naturaljapanese/rules_test.go:9](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/naturaljapanese/rules_test.go#L9) `TestReport` の末尾。`json.Marshal` 成功後に `json.Valid` を実行するのは標準ライブラリの保証の再確認。Reportの統計・Finding・版のassertは残す。実CLIのJSONは `TestCommandAnalysis` がparseして内容を調べる。失う製品保証なし。**assert削除、確度高。**

<a id="p24"></a>

### P24 — 自然日本語CLIの同じ異常処理を2つの表で維持しない

[src/cmd/natural-japanese-go/main_test.go:13](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/cmd/natural-japanese-go/main_test.go#L13) `TestCommand` のmissing/flag/genre/unreadable/invalid UTF-8は、`:137` `TestCommandDiagnostics` に同じ入力とより強いstdout/stderr/exit検査があるため削除。`:68` `TestCommandOutputError` もDiagnostics/text writeと同じなので削除する。

help/version/list-rules/stdinは残すか成功側へ統合。失う保証なし。エラー出力・stdin/ファイル入力・broken pipeの実境界を残したまま重複を除く。**すぐ削除、確度高。**

<a id="p25"></a>

### P25 — baselineのschema検証は下位、CLIは転送の代表にする

[src/cmd/natural-japanese-go/main_test.go:104](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/cmd/natural-japanese-go/main_test.go#L104) `TestCommandBaselineExcerptRequired` のmissing true/falseは、[src/internal/naturaljapanese/baseline_test.go:44](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/naturaljapanese/baseline_test.go#L44) `TestBaselineExcerptRequired` にある。CLIは不正excerpt代表1つでexit1・stderr・空stdoutを確認し、fieldの有無／空値は下位へ集約する。

失うのは同じparserの入力差の反復だけ。baseline fileを読めない場合とJSONが不正な場合は別処理なので残す。**ケース縮小、確度高。**

<a id="p26"></a>

### P26 — 自然日本語binaryをtestごとにbuildしない

[src/cmd/natural-japanese-go/integration_test.go:14](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/cmd/natural-japanese-go/integration_test.go#L14) `TestCommandBinary` は独自にCGO=0でbuildしてempty PATH・stdin JSONを検査する。配布 `TestReleaseCandidateNative` は同じ製品の実binaryをempty PATHで実行する。

stdin保証を候補Nativeへ移すか、C26の共有build済みbinaryを渡して1回実行する。fixtureのGo buildだけ削る案が最小。**配布環境変数未設定時のskipを無視して、唯一のbinary試験まで消さない**。失うのは同じsource/flagsを繰り返しlinkする確認。Kagome辞書を含むbuildの重複を省ける。**統合後に削除、確度高。**

<a id="p27"></a>

### P27 — 長文tokenizeを何度も行う閾値テストを軽くする

- [src/internal/naturaljapanese/lexical_test.go:29](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/naturaljapanese/lexical_test.go#L29) `TestLexical` の999/1,000文、masked 1,000行。4,000文字閾値の直前・到達は小さい手作りTokenizedSentenceで確認し、実tokenizer→解析は意味ある1例を残す。
- `morph_test.go:34` `TestMorphNominal` のshort/longは `TestMorphCharacterBoundary:51` の1,999/2,000文字と重複。名詞終端の成否が変わる最小token群＋必要Raw長で表現し、500文の同文反復を省く。
- Markdownを解析対象外にする検査は `TestText` と代表Checkの接続にまとめ、解析規則ごとに大量の見出しをtokenizeしない。

文字数境界と名詞判定の保証は残る。失うのは同じtokenizerを同じ文へ何百回も適用したときだけの組合せ。独自アルゴリズムの数値期待をproductの計算式から作る案ではない。**fixture縮小、確度中高。**

<a id="p28"></a>

### P28 — 同じ自然日本語入力・同値の境界側を重複実行しない

`rhythm_test.go:23` `TestRhythm` の5文/6文は、low_burstinessとrepeated_sentence_lead用に同じ入力を2回解析する。入力ごとに1回解析し2つassertを行う形に統合。結果と現実の不具合検出は変わらない。

`lexical_test.go:61` `TestLexicalTTRBoundary`、`rhythm_test.go:40` `TestRhythmBurstinessBoundary` は「境界直前・ちょうど・直後」のうち同じ不成立側を代表1つに絞る余地がある。ただし時間・維持コストは小さく、`<` と `<=` の違いを捕捉する2点は必ず残す。数値規則本体をstdlib保証として削らない。**重複解析は確度高、境界3点→2点は低優先。**

<a id="p29"></a>

### P29 — workflow出力の同じ衝突を2名前で試さない

[src/core/workflow/content_test.go:10](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/core/workflow/content_test.go#L10) `TestRenderRejectsDuplicateCompletedPaths`。stage Markdownとstage-graph JSONの2ケースは、どちらも `Render` の同じ出力先map衝突に入る。`.tmpl` 除去後に同名になる1例を残せば、この独自の衝突防止は検査できる。名前ごとの特別処理はない。失うのは2番目の拡張子での同値保証だけ。**1ケースへ縮小、確度高・効果小。**

<a id="p30"></a>

### P30 — harnessのpath不正例をsource/destination/generated全てで総当たりしない

[src/harness/manifest_test.go:55](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/harness/manifest_test.go#L55) `TestManifestRejectsInvalid` の共通path validator表。path種別の正本1表と、source/destination/generated各入口がvalidatorを呼ぶ代表へ分ける。

`source="."` 等の特別許可、親子衝突の前後、unknown token、missing source、生成との衝突は固有契約として残す。失うのは同じpath分類を同じvalidatorへ3回通すことだけ。各入口で検証を呼び忘れる不具合は代表で残す。**ケース縮小、確度中高。**

<a id="p31"></a>

### P31 — カタログ内部の順番だけを固定するassertを外す候補

[src/internal/naturaljapanese/surface_test.go:59](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/naturaljapanese/surface_test.go#L59) `TestSurfaceCatalogOrder` は、文中の登場順と無関係に配列の先頭が「重要なのは」であることを固定する。2つのphraseを検出することは集合として残し、内部catalogの並び替えまで失敗にしない案。

失うのは同一行・同一category内での特定の出力順。JSON出力順を互換契約として扱うなら、この1assertには意味があるため自動削除しない。規則・公開利用例を確認した範囲では、順番による誤検知の差はない。実行コストは小さく、P01〜P30より低優先の条件付き候補。

### この領域で残す境界

- `core/TestContentSharedChangeReachesEveryConsumer`、Codexのshared contract/TOML・role description伝播は、共通原稿の変更が実配布へ届く独自保証。全文hashとは異なる。
- `install/TestAssignmentContract` のtool matcher、`TestInstallHookCommands`、5担当のsandboxは、正規toolがhook対象から漏れる／readerがwrite可能になる不具合を防ぐ。
- `TestDocumentDistributionRuleSelector` は、配布したRuleが実際のselectorで見つかる接続保証。
- releaseのunsafe archive、transport hash、正本source/license、version/target、実行権限、同名出力保全、installerの取得制約は、製品が入力を信用する前の独自判断。
- `TestAnalyzer` のtoken情報変換、baselineの消滅/継続/newの識別、Unicode/Markdownの行位置、Mora/MTLD/構造判定の数値は自前の処理で、GoやKagomeが勝手に保証するものではない。
- 自然日本語CLIのFIFO拒否と閉じたpipe、PowerShell呼出元復帰は、単なるos/execの動作確認ではなく製品が正しく組み合わせる責任を検査する。


## アプリ接続・CLI・Space・OKF：削除・統合候補

### 明確な削除候補


<a id="a01"></a>

### A01 — 別Testを直接呼ぶaliasを5個削除

- [src/internal/app/rule_skill_separation_test.go:79](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/app/rule_skill_separation_test.go#L79) `TestRuleSkillSeparationHookApprovalPending` → 残存 `app/execution_plan_test.go:93` `TestExecutionPlanCLIPendingHook`。
- [src/internal/app/hook_split_cli_test.go:95](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/app/hook_split_cli_test.go#L95) `TestHookSplitCLIRepair` → 残存 `app/boundary_test.go:12`、`documents_test.go:65` / `:103`、`procedure_test.go:63`、`execution_plan_test.go:183`。5シナリオを再実行するだけ。
- [src/internal/workspace/rule_skill_separation_test.go:5](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/workspace/rule_skill_separation_test.go#L5) `TestRuleSkillSeparationRuleCopy` → 残存 `workspace/space_okf_test.go:10` / `:42`。
- [src/internal/okfcli/command_test.go:47](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/okfcli/command_test.go#L47) `TestOKFMemoryContract` → 残存同file `:10` `TestOKFCommand`。
- [src/internal/okfmemory/metadata_test.go:116](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/okfmemory/metadata_test.go#L116) `TestOKFMemoryContract` → 残存同file `:14` と `okfmemory/document_test.go:40`。

削る部分は各alias全体。失う保証なし。testの登録名だけのため同じケースを2回走らせている。コスト中〜大（appのinstall / session / flow fixture再実行）。先行、確度高。名前指定のCIがある場合、削除前に呼出側を実test名へ替える必要がある。

<a id="a02"></a>

### A02 — 既に廃止したCLIに対する古いflag validator試験

- [src/internal/cli/project_root_test.go:5](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/cli/project_root_test.go#L5) `TestProjectRootWithoutGitInstall` 全体。
- [src/internal/cli/relocation_test.go:8](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/cli/relocation_test.go#L8) `TestRelocationCLI` の先頭install要求と旧install異常値4ケース、`:49` `TestRelocationCLIEmptySourceFlags` 全体。
- [src/internal/cli/command_test.go:35](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/cli/command_test.go#L35) `TestCommandRejectsInvalid` の `actor_missing`、`space_missing`、`empty_id`、`duplicate`、`unknown`。
- [src/internal/cli/memory_help_test.go:11](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/cli/memory_help_test.go#L11) `TestMemoryHelp` の不正入力表にある `memory unknown --help`、`memory create name --help`、`memory create --help --body-file secret`。

現在の `ParseCommand` は `install` / `memory` / `kdr` をコマンド入口で拒否する。actor、ID、relocate等の旧validatorには到達しない。残すのは `cli/five_cli_test.go:10` `TestFiveCLIContract` の旧入口拒否（kdrも必要なら1行集約）と、relocationの現行unit reassign構文。`mixed_name_id` は現行intent構文なので残す。失う保証は入口拒否の重複だけ。コスト実行小、誤解を誘う保守負担大。先行、確度高。

<a id="a03"></a>

### A03 — 未使用の旧organization readerだけを検証

[src/internal/workspace/space_create_test.go:115](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/workspace/space_create_test.go#L115) `TestReadDefaultOrganizationFailures` 全体（6ケース、約80行）。`readDefaultOrganization` は `workspace/space_create.go:141` に残るがproduction呼出がなく、testのみが呼ぶ。現在の作成経路は `readDefaultRule` → OKF Ruleをコピーする。

残存: `workspace/space_okf_test.go:10` / `:42`、`space_create_integration_test.go:211` / `:547` / `:660`。現行導入で起こるbugの保証は失わない。旧関数の削除自体は実装許可後のdead-code整理事項。コスト実行小、不要な旧仕様保守が中。先行、確度高。

<a id="a04"></a>

### A04 — ADR scaffoldの単独存在確認

[src/internal/workspace/flow_test.go:9](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/workspace/flow_test.go#L9) `TestFlowSpaceADR` 全体。`CreateSpace`後の `knowledge/adr/index.md` 存在だけを調べる。

残存: `workspace/space_okf_test.go:10` の5path表、`space_create_integration_test.go:184` `TestCreateSpaceScaffold`。失う保証なし。追加Space作成1回を削減。先行、確度高。

<a id="a05"></a>

### A05 — 新規Spaceの弱い存在確認

[src/internal/workspace/space_create_integration_test.go:83](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/workspace/space_create_integration_test.go#L83) `TestCreateSpaceClaimsNewTarget` 全体。2行とも成功名とtargetがdirectoryであることだけを確認する。

残存: 同file `:184` のteam/default scaffold、`space_okf_test.go:10` の `Team Alpha` 正規化と実配置。必要ならscaffold側team入力を `Team Alpha` にする。失う保証なし。2回のscaffold生成を削減。先行、確度高。

<a id="a06"></a>

### A06 — real symlink試験の完全重複

[src/internal/workspace/space_switch_integration_test.go:57](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/workspace/space_switch_integration_test.go#L57) `TestSwitchSpaceRejectsCursorSymlink` 全体。

残存同file `:388` `TestSwitchSpaceCursorLinkBoundaries/inside relative` が同じ `old` を指すactive-space linkを拒否し、全tree不変まで検証する。失う保証なし。fixture/snapshot1回分。先行、確度高。

<a id="a07"></a>

### A07 — 同じOKF保存シナリオがappとokfappにコピー

[src/internal/app/memory_body_test.go:19](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/app/memory_body_test.go#L19) `TestMemoryBodyWrite` の生成時刻、CAS/no-op、拡張metadata、index/log検証を [src/internal/okfapp/command_test.go:21](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/okfapp/command_test.go#L21) `TestOKFMemoryContract` に一本化。`:85` `TestMemoryBodyWriteRejectsAndPreserves` の7ケースは `okfapp/command_test.go:87` `TestOKFSaveFailure` と同文であるため削除候補。

productionの `app/command.go:59` は `okfapp.Service.Execute` へのfieldコピー。appでは `session_test.go:97` の選択不変、`:209` のshow、`work_log_test.go:15` の検索を残し、create/updateのActor、Metadata、BodyFile、Expectを1つの短い転送試験に集約する。コピー全体を消す前にその転送保証を保持すること。失う保証はokfappで実行する同一validatorの重複。field渡し忘れは独立のbugなので消さない。7回のinstall / flow fixtureと成功CRUD一周を削減。先行、確度高。

<a id="a08"></a>

### A08 — CodeKB用の汎用create/show/search繰返し

[src/internal/app/codekb_test.go:46](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/app/codekb_test.go#L46) `TestCodeKBMemory` 全体を統合。CodeKB固有のbranchはなく `bodyRequest` と汎用memory操作を呼ぶだけ。

残存: `app/session_test.go:209` original bytes付きshow、`app/work_log_test.go:15` search結果のID/Space保証、A07の残すcreate転送。失うのは固定本文 `Current feature details.` と固定title `Memory` の別fixtureのみ。`:13` `TestCodeKBBeginRepair` は工程selectorと修復許可に関わるので残す。コスト中。統合、確度高。

## 同じ境界の表を複数層で繰り返している候補

<a id="a09"></a>

### A09 — approval pendingの同じfixtureを3回生成

[src/internal/app/verification_gates_test.go:9](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/app/verification_gates_test.go#L9) `TestVerificationGatesPendingHook` と `app/execution_plan_test.go:183` `TestOKFSkillReadApprovalPending` を、同file `:93` `TestExecutionPlanCLIPendingHook` へ統合。

3つとも同じBegin、2段階plan提案、bind、turn、bindを作り、通常write拒否・skill read・code read・plan・plan-approvalを調べる。差分はhash commandの許可とOKF skill単独readなので、その2行を残す表へ移す。失う保証なし。重いfixture2回と共通command rowsを削減。A01 alias削除と重複計上しない。先行、確度高。

<a id="a10"></a>

### A10 — skill family × 共通hook securityのcross product

対象: `app/rule_skill_separation_test.go:36` `TestRuleSkillSeparationHook`、`:81` `TestOKFSkillRead`、`app/stage_skills_test.go:10` `TestStageSkillsRead`、`app/flow_test.go:165` `TestFlowInactiveWorkflowReadAndResume`、`:229` `TestFlowWorkflowReadRequiresCurrentRulesAndRealFiles`。

`Hook`のselection/current Rule/in-flight gateと `workflowRead` (`app/hook.go:381`) のshell parser / file readは全family共通。`unread` / `inflight` / `missing` / `symlink` / `redirect` / `compound` / `arbitrary` / `retired`をfamilyごとに再試験している。

具体案: StageSkillsReadへ共通拒否表を1回集約し、RuleSkill/OKF側は登録pathの正常読込（single/all）と固有旧path拒否だけ残す。inactive試験はcompleted/waiting/pausedそれぞれのread許可とresume/reopen後の通常作業を残すが、7種類のmalicious commandを3status全部では回さない。status側に1つの通常mutation拒否、別の共通拒否表でshell安全性を保持する。各pathの登録漏れと各statusの分岐は別bugなので残す。機械的に全表を1本へ集めるだけでcase数を維持する案ではなく、同じgateの重複実行を減らす。コスト大。統合、確度高。

<a id="a11"></a>

### A11 — child split CLI表を既存child boundaryへ統合

`app/hook_split_cli_test.go:33` `TestChildHookSplitCLI` のrules readは `app/child_hook_test.go:163` `TestChildHookCommandBoundary/rules read` と同じ。create拒否とOKF未知`__hook`は既存表に移し、関数全体を削除可能。

残す表は各tool/operation routingと親state不変を検証する。createとupdateは異なるrequest routingなので統合後も両者の行を残す。独立保証なしにmutation行まで消さない。コストchildfixture1回。統合、確度高。

<a id="a12"></a>

### A12 — Root優先順位の表を4箇所で再実行

- `workspace/space_read_test.go:80` `TestReadSpacesRootPrecedence` 6ケース。
- `workspace/space_switch_test.go:224` `TestSwitchSpaceRootPrecedence` 5ケース。
- `workspace/space_create_integration_test.go:469` `TestCreateSpaceRootPriority` 5ケース。

優先順位とrelative解決の正本は `workspace/root_test.go:8` `TestResolveRoot`。各production入口は `ResolveRoot(input)` を同じ形で呼ぶ。上記3表は、各入口について「すべての候補をセットした明示rootが選ばれ、別rootを触らない」1ケースを残す。Read側の絶対candidate＋relative WorkingDirはroot_testへ移すか既存正本に統合する。Createを丸ごと消してroot単体だけ残すと、input渡し忘れや違うpathを開くbugを失うため不可。コスト大（Createの5×4tree）。統合、確度高。

<a id="a13"></a>

### A13 — Relative-root、open failureの過剰な同値ケース

- `workspace/space_read_test.go:12` 4ケース、`:145` のempty入力は重複。`:145` の「openしない」を残し、`:12`はrelative explicit等の正規化代表1ケースだけ残す。
- `workspace/space_create_test.go:14` と `space_switch_test.go:40` は各empty/relative cwd/relative explicitを1代表に縮小。3つは各入口の同じ `!filepath.IsAbs(ResolveRoot(...))` へ入る。
- `workspace/space_read_test.go:36`、`space_create_test.go:69`、`space_switch_test.go:176` のmissing/permission/arbitrary open errorは、各入口1つのwrapped sentinelに縮小。現実のmissing/fileは各integrationに残る。

残すのは入口ごとの呼出なし/zero result/error cause/選択root固定。消すのは異なるerror文字列だけで同じ無条件error返却を反復する部分。コスト中、統合、確度高。

<a id="a14"></a>

### A14 — List/Readのreal FS fallbackを下位単体表と重複

`workspace/space_integration_test.go:17` `TestSpaceReadersFilesystem` の8ケースと `space_read_integration_test.go:96` `TestReadSpacesFallbacks` の13ケース。

残存正本は `workspace/space_test.go:11` ActiveSpace、`:84` fallback、`:148` default、`:211` selection、`:343` UTF16 order。通常のmissing/empty/blank/BOM/NEL/unknown cursor/regular entryを層ごとに再現している。

具体案: `TestSpaceReadersFilesystem`は正常OS filesystem smoke1ケースへ、`TestReadSpacesFallbacks`はuninitialized＋aidlcがfileの2ケースへ縮小。他の純粋文字列/順序/selection例は正本だけで確認。ReadSpacesの `os.Root` 内外link (`:227`) とinitial project link (`:334`) は残す。DirFSはroot外linkを辿る一方Root.FSは閉じ込めるので、その2種のsymlink試験を同じものとして削除しない。コスト大。統合、確度高。

<a id="a15"></a>

### A15 — Switchの名前正規化14ケースごとに保護tree一式

`workspace/space_switch_integration_test.go:125` `TestSwitchSpaceNamesAndProtectedData`。

残す: teamの同じtarget書換えとprotected data不変1ケース、作成時予約語の1代表（例list）がswitchでは使える1ケース。case-sensitiveなraw help guardは`Help`で別に1ケースを保持。Unicode、prefix、truncationは `space_create_test.go:295` / `:326` / `:356` / `:395` のspaceSlug正本にあり、switchが同じ関数を使う。7予約語全部＋Unicodeなどのfixture/snapshotを省く。予約語が1つずつ別branchではないことはproduction `switchSpace` のraw guard3値以外はspaceSlugだけであることから確認済み。失うのは同じslug処理を各入口で反復する保証。コスト大。統合、確度高。

<a id="a16"></a>

### A16 — CLIのproject-dir位置・形式の組合せ

`cli/space_list_test.go:379` `TestRunSpaceListFlagPositions` はlist/bare×json位置×project位置×equalsで36ケース。`space_test.go:49` create8、`space_switch_test.go:121` switch8も同じ `workspaceArguments` を通る。

具体案: 共通parser表で先頭/中間/末尾、split/equals、jsonとprojectの前後を6〜8ケースで保持。各routeは既存 `TestRunSpaceCreate` / `TestRunSpaceSwitch` / `TestRunSpaceListJSON` / `TestRunSpaceBareAlias` にproject-dir forwardingを含め、各1回確認。listは分類後allowJSON=trueで再parseするため、その再parseでproject-dirを失わない行を必ず残す。位置全順列に独立分岐はない。コスト実行小、200行超の保守負担中。統合、確度高。

<a id="a17"></a>

### A17 — CLI shared parserのinvalid flags再検証

`cli/space_test.go:169` `TestRunSpaceCreateInvalidArguments`、`:231` `DashProjectDir`、`space_switch_test.go:165` `InvalidArguments`、`:221` `DashPath`、`space_list_test.go:490` `InvalidFlags`（13×bare/list）、`:545` `ProjectDirLiteral`。

unknown flagの位置3種、project-dir missing/empty/equals-empty/duplicate、dash値等は `workspaceArguments` 1本へ統合。各route固有のname数、raw help、list-only JSON、bare/list分類は現行public Runで各1〜2行残す。`--json=true/false/empty`は等号値をサポートしない同一unknown branchなので1代表へ縮小。別routeの処理自体を試さずparserだけにしない。失うのは同じparser errorをerror文脈のみ変えて再確認する部分。コスト中。統合、確度高。

## 内部の実装を固定している・保証が弱い候補

<a id="a18"></a>

### A18 — Help本文全コピーと同じ文字列の検査

`cli/cli_test.go:36` `wantHelp`、`:71` `TestRun_Help`、`:232` `TestRun_UnknownArguments`、`cli/space_test.go:290`、`space_switch_test.go:98`、`space_list_test.go:351`、`execution_plan_review_test.go:10` のroot help語句検査。

全文copyにより句読点変更でも大量testが壊れ、対照文の同じ誤りをcopyすると通る。推奨はroot helpの公開route一覧を1表で確認し、help aliasesは同一結果/exit0/依存callbackなしだけ確認。unknown errorではprefix、元args、help suffixがあることを確認し全helpを別途複製しない。3つのHelpIncludesSpaceはroute一覧正本へ統合。失う保証はヘルプの全字面固定であり、語句の意味をdomain試験が代替するとは言わない。CLI利用者に必要なsyntax/exit code/必須flagは残す。コスト実行小、保守負担大。統合、確度高。

<a id="a19"></a>

### A19 — Helpの用語リスト・別名に同じ段落を再検証

対象: `cli/check_help_test.go:15` / `:66` / `:93` / `:158` / `:173` / `:259`、`assignment_test.go:8`、`execution_plan_test.go:8`、`memory_help_test.go:56`、`help_codekb_test.go:10`、`help_work_log_test.go:9`、`git_independent_test.go:22`、`relocation_test.go:42`。

削る部分: 同じhelpを別入口で取得して同じ単語（例registry_epoch、実行）を繰返し調べる行、単なる説明文の日本語spellings、domainの状態名一覧をhelpへ丸写しする行。残す: Help dispatch表、必須flags、repair command、実際に使用するJSON例、workflowの安全な順序を利用者へ伝える最小契約。文章編集時の意味のreviewを無くさない。単にContainsを満たす矛盾文章を現行testも検出できない。各段落が唯一の仕様proofの場合は先に実例を残すため、機械的全文削除は不可。コスト実行小、保守大。統合、確度中。

<a id="a20"></a>

### A20 — Helpから抽出する実例は残し、二重のschema/件数検査を削る

- `cli/configure_help_test.go:10` のJSON型/初期statusを再解釈する部分 → [src/cmd/aidlc/configure_help_integration_test.go:16](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/cmd/aidlc/configure_help_integration_test.go#L16) `TestConfigureHelpExamples` が同じ2個をhelpから抽出し、configure→Sensorまで実使用する。低層表の同じschema期待を統合できる。ただしintegration実施時だけになるので、短いJSON decode smokeを残す選択は可。
- `cli/rule_skill_separation_test.go:10` `TestRuleSkillSeparationHelp` の `len(matches)<20` と末尾の広い語句辞書を削る。skillに実在する各help commandが解決する検査は残し、抽出0件を検知するnonemptyだけにする。

失う保証は「例が常に2個/commandが20以上」といった件数、言い回し固定。実際のcommand link切れは残存試験で守る。統合、確度高。

<a id="a21"></a>

### A21 — 同じwriteStdoutの失敗をhelp/version別名で繰り返す

`cli/cli_test.go:113` `TestRun_HelpWriteError` 3行、`:194` `TestRun_VersionWriteError` 2行、`cli/check_help_test.go:245` `TestCheckHelpOutputFailure` を代表1行へ縮小。成功routingは `TestRun_Help` / `TestRun_Version` に残る。

production `cli/cli.go:157` の同じwriteStdoutを通る。失うのはbroken-pipe分岐の再実行のみ。注意: Space create/switch/listのstdout短書込処理は別実装なので、そちらのfailureを同時削除しない。統合、確度高、実行コスト小。

<a id="a22"></a>

### A22 — stderr失敗cross productとfake prefix自己検証

- `cli/space_test.go:336` `TestRunSpaceCreateStderrFailure`、`space_switch_test.go:307` `TestRunSpaceSwitchOutputFailures` のstderr unavailable/both unavailable/short stderr、`space_list_test.go:738` `TestRunSpaceListStderrFailure`。共通 `writeCommandError` はencoder結果を意図して無視し常に1。errorWriter1ケースを代表とし、各routeのcallback failure JSONは残す。syntax/callback/stdout×stderrの全組合せを省く。
- `cli/space_list_test.go:695` `TestRunSpaceListPartialStdoutFailure` の `stdout.output == "Spa" / "{\"a"`、`space_switch_test.go:307` のpartial.output非空。これらはfake `partialListWriter` 自身が先頭3byteを書いた結果を確認している。productionが部分書込errorを伝えるexit/JSONは残し、fake内部bufferの固定prefixを削る。

失うのはfakeの実装とstdio failure組合せの重複。stdout rollbackを製品が行うAPIはなく、このfakeは製品にbuffer巻き戻し手段を渡していない。統合、確度高、実行小/保守中。

<a id="a23"></a>

### A23 — 出力準備の試験でWrite回数まで固定

`cli/space_test.go:356`、`space_switch_test.go:255`、`space_list_test.go:266` のOutputPreparation。

prepareがcallback/outputより先で、help/versionで呼ばれない点はSIGPIPE等の観測可能な境界なので残す。削るのはsuccessと同じproject flag位置行、list/bare×human/JSONで同じ順序を再確認する行、unknown commandで `stderr,stderr` の厳密回数。1回の連結Writeと2回のWriteの差は利用者保証ではない。残存各route1success、1failure-before-callback、1callback-failureとhelp/version bypass。統合、確度高、実行小/保守中。

<a id="a24"></a>

### A24 — Runの直後に同じParseCommandをもう一度呼ぶ

`cli/command_test.go:58` `TestHookCommandDispatch` の各行末のdirect ParseCommand確認。Run経由ですでにParseCommandされ、callbackに渡されたrequest、exitを確認している現行 `__hook` 行では重複。retired hookは入口分類が異なるためA02の拒否表へまとめる。残存Run側のdispatchとrequest field確認。失う保証なし。統合、確度高、実行小。

<a id="a25"></a>

### A25 — Unicode lowerの委譲wrapperを12回再試験

`workspace/workspace_lock_test.go:67` `TestNormalizeWorkspaceLockCanonicalMatchesECMAScriptDefaultLower` 全体 → `pathnorm/unicode_test.go:5` `TestECMAScriptDefaultLowerFixedWindowsVectors` に同じ12vectorsあり。

workspace側は同file `:98` / `:122` の固定lock filenameを残す。これがWindows正規化→sentinel→MD5→8hexという接続を保証する。失う保証なし。先行、確度高、実行小/領域誤配置の保守中。

<a id="a26"></a>

### A26 — Unicode overlayの実装copyと自分の定数をoracleにする検査

`workspace/workspace_lock_test.go:146`、`:165`、`:189` の55/107/88runeの表はpathnorm実装のrange一覧をコピーし、さらにfixture配列長を固定する。workspace productionには委譲wrapperしかない。`:254` のunicode.Version文字列のみのwhitelistも、実際の変換結果を試していない。

具体案: array長3assertは削除。範囲列挙の直接predicate試験をpathnormへ移し、独立に固定した入出力（Unicode15では変換しないrune、FinalSigmaの前後文脈）へ寄せる。`:228`の7つの実出力contextは価値が高いのでpathnormへ保持。巨大表を丸ごと削除すると表の途中rune typoを見逃す余地は残るため、まず範囲端/外・複数特殊文字のoracleを確保し、元のUnicode source根拠から導くfixtureが必要。現在の固定Bun/Unicode15契約を古いものとして捨てない。統合、確度中。

<a id="a27"></a>

### A27 — lock digest期待値がproduction式を複写

`workspace/workspace_lock_test.go:22` / `:49` は同じMD5/sentinel/先頭4byte式でwantを作る。`:98` / `:122` の固定hash/filenameが既により独立したoracle。

削るのはmd5式の再計算部分。canonical取得成功時にlexicalを無視すること、失敗時にlexicalへ戻すこと、指定temp-dirへ置くことは残す。`:98` / `:122` 内でもidentityからtest自身がMD5を計算してdigestを確認する中間段は、最終固定filename比較があるので省ける。失う保証なし（最終filenameとcase選択を保持）。統合、確度高、実行小/保守中。

<a id="a28"></a>

### A28 — artificial FS entriesと同値error cases

- `workspace/space_test.go:300` `TestListSpacesUnique/repeated entries are not repeated` は同じdirectory entryをfakeで2回返す。実OSの同一directoryで同名entry2個はなく、synthetic defaultとの重複だけが現実の経路。`existing default is not repeated`は残す。
- `space_test.go:84` ActiveSpaceFallback のpermission/arbitraryをpartial-data付きerror1行へ、`:388` ReadDirErrorのpermission/arbitraryをpartial-entries付きerror1行へ。コードはerror種類で分岐しない。
- `space_test.go:433` StopsOnStatErrorはmiddle1行で「先行を保持・失敗以後を止める」の両側を守りfirst/lastを省く。

失う保証: custom fs.FSが重複entryを返す非通常契約のみ（そのAPIが必要なら残す判断）。それ以外は同じ早期return分岐。統合、確度高、実行小/保守中。

<a id="a29"></a>

### A29 — cursorの内部呼出列と非regular mode総当たり

`workspace/space_switch_test.go:294` `TestReplaceSpaceCursorNonRegular` のdevice/socketは同じIsRegular falseの標準ライブラリ分類を繰返す。real symlink integrationとnamedpipeまたはdirectory代表を残す。

`:443` `TestReplaceSpaceCursorFailures` と `cursor_test.go:158` のFailuresは、エラー注入自体は価値がある。削るのは全列 `inspect/open/write/chmod/close/...` の完全一致への結合、同じ失敗行ごとのcursor名/固定temp名再assert。error cause、未完了ならpublishしない、own stagingのみcleanup、close、permission復元後にpublishという安全な部分順序は残す。`space_switch_integration_test.go:200` はreal fileの古いbytes保持なのでunit fault injectionと同じ保証として全削除しない。統合、確度中。

<a id="a30"></a>

### A30 — fake closeのあとos.Rootが閉じたことを再検証

`workspace/space_read_integration_test.go:39` `TestReadSpacesProjectClose`、`space_create_integration_test.go:18` `TestCreateSpaceProjectClose`、`space_switch_integration_test.go:286` `TestSwitchSpaceRootCloseFailures` の `captured.Stat(".")` / `roots` の閉鎖確認部分。

注入したclose関数がroot.Closeを呼び、productionがclose callbackを呼んだことはcall count / stepsで既に観測済み。その後Statが失敗するのは標準API保証の再確認。close callback呼出回数、error cause、zero result、保存済みbytes保持は残す。default closeを渡しただけでcall counterがない `TestSwitchSpaceSavesNormalizedName` の閉鎖検査はこの理由では消さない。低優先、確度高、実行小/保守小。

<a id="a31"></a>

### A31 — default buildinfoの2field subtests

`buildinfo/buildinfo_test.go:9` `TestCurrent_Defaults` はglobals `Version="dev"`, `Commit="unknown"` をstructへコピーしただけの結果を各subtestへ分ける。現実のversion stampingはnative release候補の [src/cmd/aidlc-dist/release_integration_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/cmd/aidlc-dist/release_integration_test.go) に別のproofがある。

最小案は2subtestとparallel boilerplateを1つのstruct比較へ縮小。全削除も候補だが、devビルドのdefault表示を誤変更したことへの唯一の直接proofは失う（release stamping試験はdefault文字列を保証しない）。その小さい開発時表示リスクを受け入れる場合のみ全削除。低優先、確度中、実行/保守小。

<a id="a32"></a>

### A32 — OKF Parse wrapperで下位validatorを4回再実行

`okfmemory/document_test.go:31` `TestParseRejectsInvalid` のno_frontmatter/empty_type/duplicate/utf8を1つの代表不正frontmatterに縮小。

`okfmemory.Parse` は全入力を `okf.ParseConcept` へ先に渡す。残存 `okf/frontmatter_test.go:44` `TestParseConceptInvalid` に同じ4種類がある。wrapper側のvalidator渡し忘れは1不正ケースで守り、unknown metadata/本文保持/serialized data再検証は `TestParseRoundTrip` とmetadata試験に残す。失うのは同じvalidatorへの反復入力だけ。統合、確度高、実行小。

<a id="a33"></a>

### A33 — OKF usage_window同一validatorの2箇所×8cases

`okf/frontmatter_test.go:99` `TestParseConceptUsageWindow` はtop levelとsource overrideで同じ8ケースを回す。productionは両箇所から同じusage window validatorを呼ぶ。

top levelの8ケースを正本として保持し、source overrideは正常1＋不正1へ縮小し、nested fieldの接続を守る。不正種類全部をnestedで繰り返す6ケースを削減。失う保証は同じvalidatorの繰返しのみ。統合、確度高、実行小。

<a id="a34"></a>

### A34 — scanの再実行によるtags所有権試験

`okf/scan_test.go:12` `TestScanBundleSelection` 後半のTags mutation→再ScanBundle→tags確認は、別走査で新規parseしたobject間の共有を検証している。scan実装にcacheはなく、再走査を跨ぐ所有権は現状ほぼ到達不能のbug。

削る部分は再Scan部分のみ。選別、warning、path、UTF16 orderは残す。`okf/search_test.go:46` `TestSearchLifecycleAndOwnership` の入力slice/timestamp/warningsとのalias検査は、その場の結果構築で起こりやすいbugなので残す。将来scanにcacheを導入した際には別途必要。低優先、確度中、再走査小。

<a id="a35"></a>

### A35 — session memory試験のCRUD共通部分

`app/session_test.go:97` のstale CAS再実行、`:146` `TestSessionMemoryBoundaries` のcreate→update→custom保持までをA07のokfapp契約へ集約。`:146`後半の他Space検索とbookkeeping失敗後に保存済みConcept+errorを返す部分は残す（できればokfappへ所有を移す）。`:209` original hashは残す。

同じunknown metadataの保持は `okfapp/command_test.go:21` と `okfmemory/metadata_test.go:14` が担当。app側はrequest変換と選択state不変に集中する。bookkeeping failureのpartial successは単なるvalidator試験ではなく固有の永続化契約で、削除しない。統合、確度高、コスト中。

<a id="a36"></a>

### A36 — 不要なGitと二重procedure生成

- `app/assignment_test.go:66` と `app/child_report_test.go:60` の各caseの `git init` を省く候補。現行 `assignment.workerRoot` はabsolute/evalSymlinks/directory/重複で判定し、Git不要。workerの実directoryは残す。Git不要環境での試験を隠してしまう初期化でもある。
- `app/session_test.go:23` の `deployProcedureFixture` は直前の `install.Codex` と同じworkflowを再生成。installerがdeploymentを作る経路を確認した上で省く。installを使わない `app/flow_test.go` のpartial fixture呼出は残す。
- `app/flow_test.go:104` / `:144` 付近の `err=nil; if err!=nil` は到達不能assertなので削除。

失う保証なし。Gitプロセスと重複file writeを減らす。先行、確度高（実装時targetedでfixture前提を確認）、コスト中〜大。

<a id="a37"></a>

### A37 — projectrootのexplicit branchをancestor種類ごとに繰返す

`projectroot/root_test.go:10` `TestResolveAncestorFiles` の2種類のancestor obstruction×explicit empty/explicit rootのうち、explicit root2行を同file `:42` `TestResolvePreservesErrors` のexplicit override1行へ統合できる。explicit branchはancestor探索を行わない。

ancestor `aidlc` がfileと `aidlc/workflow` がfileの自動探索2行は別depthでの失敗を調べるため残す。ambiguityと実FS errorは残す。低優先、確度高、コスト小。

## 消さない高価値境界と、候補を見送った箇所

- `app/flow_test.go:94` / `:119` のUnit進捗偽造・活動中assignment削除拒否はapp自身の `executeFlow` が実装する権限制約。domainにも似たstate validationがあるという理由で消さない。
- `app/boundary_test.go:52` はapp Serviceを呼ばずflow.Store.Unitを直接呼ぶため配置は悪いが、「Begin不足を他のUnit gateより先に拒否」の直接proofを削除すると失う。flowへ移動する候補で、今回確認したdomain testだけで同じ順序保証が代替済みとは言えない。
- `app/relocation_test.go:12` のService install adapterは現行公開CLIから呼べないが内部互換入口が残っている。入口そのものの削除を含む別判断なしに、testだけ先に消す候補にはしない。
- `app/child_report_test.go` のfresh registry再読込、parent宛拘束、unknown registry、pending release、bounded waitは実障害対策。mock clockは実際に状態やlockを変える時機を制御しており、mock自己検証ではない。
- `app/child_hook_test.go` の子が親sessionを変更しないこと、worker/readerの権限差、native actions別dispatchは保持。`agent_hook_test.go`、`assignment_test.go` の指示渡し先・owner/root/session/definition照合も保持。
- `app/hook_reliability_test.go` の終端ID照合、再送、別session、競合時timeoutとlock非奪取、bootstrap上限、復旧判断は保持。hookのhelp/読み取り/修復例外は通常許可と同一視しない。
- `app/documents_test.go` のJSON roundtrip空array、configureがdocument権限を横取りしない、未解決required selectorのrepairは固有の接続契約。
- `app/work_log_test.go:15` はworkflow差戻しが作った実文書がOKF検索へ入る縦の境界。typeを変えただけのCRUDではないので、他のSearch testを理由に全削除しない。
- `workspace/cursor_test.go` のno-replaceはreplaceと別のpublish方式。既存cursorを上書きしない、collision stagingを消さない、publish前close、失敗cleanupは保持。
- `workspace/workspace_lock_test.go` の取得/解放generation、stale lock非回収、caller cancellation、release失敗、panic時releaseは保持。`SwitchSpacePublicWaitsForContendedWorkspaceLock` は入口でのlock接続を保証し、下位WithLockだけでは代替しない。
- real filesystemのroot内/外/absolute/initial symlink、同target競合、部分保存後再試行、permission復元はOS/実装境界のため残す。単にos.Rootを使うという理由で全削除しない。
- `okf/frontmatter_test.go` のUTF8、厳密型、複数YAML文書、64KiB境界、optional metadata/trust、`scan_test.go`の4096件上限とwarning budgetは保持。標準YAML parserの機能確認ではなく、この製品がどの型/本文を許すかの契約。
- `okf/search_test.go` のAND/OR、score、deprecated/stale、timestamp順、UTF16 order、結果の非aliasは保持。
- `okfmemory/selector_test.go` のone/optional/manyはdistinct cardinality。symlink/broken文書の無視禁止、`selector_unix_test.go:12` FIFOで永久blockしない試験は保持。
- `okfmemory/metadata_test.go` の未知field保持、巨大JSON数値精度、nested duplicate key、generated-only no-op拒否、clear/omittedの差は保持。body bytes上限とfrontmatter拒否はokf.ParseConceptと役割が違う。
- `okfapp/command_test.go:138` の競合CAS成功が1件だけ、7種の書込前拒否でbookkeepingが増えないことは保持。`okfcli/metadata_test.go` はflags→fieldの転送とcreate/update必須flag差を保持する。値検証が重なる場合でも転送を消さない。


## Intent・Unit・担当管理・工程定義：削除・統合候補

### 根拠が明確な候補


<a id="f01"></a>

### F01 — 別Test関数を丸ごと呼ぶ入口2本を削除【先行・確度高】

- [src/internal/flow/boundary_sensor_test.go:311](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/flow/boundary_sensor_test.go#L311) `TestEndSensorUnitCommandPair` 全体。
- [src/internal/flow/unit_test.go:148](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/flow/unit_test.go#L148) `TestFlowUnitDependencyContent` 全体。
- 生存先はそれぞれ `verification_results_test.go:11` `TestVerificationResults`、`unit_without_git_test.go:53` `TestUnitWithoutGit`。Goの通常test discoveryで本体も実行されるため、この2入口は同一処理の再実行だけ。失う保証はない。前者は10行のtableをもう一巡し、後者は予約・提出・統合・解除・次担当までの一連のFS操作を丸ごと重複させる。

<a id="f02"></a>

### F02 — validと全く同じself cycle行を削除【先行・確度高】

- [src/internal/flow/verification_results_test.go:12](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/flow/verification_results_test.go#L12) `TestVerificationResults/self cycle` と57行の同名条件。
- switchに`self cycle`分岐がなく、入力・実行・結果検査は`valid`と一致する。`valid`の「resultを作成してもcode SHAが変わらない」検査を生存させる。自己参照対策の保証は失わない。fixtureとdigestの重複実行を1回削れる。

<a id="f03"></a>

### F03 — 同じcommandを持つ2 Unitの結果不足テストを一元化【先行・確度高】

- [src/internal/flow/boundary_results_test.go:32](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/flow/boundary_results_test.go#L32) `TestEndSensorSharedCommandRequiresEachUnitResult` 全体を削除。
- 生存先は `verification_results_test.go:11` `TestVerificationResults/valid` と`second unit missing`。両方が同じ`boundaryCollector.results`を呼び、同commandのa/bに対し一方だけでは不足、両方なら成功を確認する。失う保証はない。旧側だけが不要なGit・承認済みstage fixtureを準備している。

<a id="f04"></a>

### F04 — flow内のassignment schema定数テストを削除【先行・確度高】

- [src/internal/flow/verification_cli_test.go:64](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/flow/verification_cli_test.go#L64) `TestVerificationCLIAssignmentSchema` 全体。
- flowのCLIもHashも使わず、assignment.Init戻り値の`SchemaVersion==2`だけを確認する。生存先は [src/internal/assignment/store_test.go:11](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/assignment/store_test.go#L11) `TestRegistry/explicit initialization and aliases`。保証は失わず、別packageへの重複依存と初期化を削れる。

<a id="f05"></a>

### F05 — 同一goldenのworkflow部分だけを再検査するtestを削除【先行・確度高】

- [src/internal/flow/content_test.go:17](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/flow/content_test.go#L17) `TestProcedureBoundaryComposedWorkflow` 全体。
- 生存先は [src/internal/install/manifest_parity_test.go:25](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/install/manifest_parity_test.go#L25) `TestCodexManifestParity`。同じ`five-cli-assets-sha256.json`を使い、同じ7 workflow assetを含む全配置fileのpath/bytes/modeを検査する。[src/harness/codex/manifest_test.go:13](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/harness/codex/manifest_test.go#L13) `TestDistribution`も同じgoldenを検査するため、P11で両者を整理する場合は**少なくとも配置bytesの正本testを1つ残す**。
- flow版はcoreworkflow.Renderだけを呼び、flow独自の挙動を追加で保証しない。goldenが持つ保証は失わない。`TestProcedureBoundarySharedOperationPropagation`は単なる同一goldenではなく共通fragment変更の伝播を検査するため、この削除に含めない。

<a id="f06"></a>

### F06 — 対象宣言が実際には無視されるintegration feature testを削除【先行・確度高】

- [src/internal/flow/selected_documents_test.go:166](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/flow/selected_documents_test.go#L166) `TestSelectedDocumentsIntegrationFeature` 全体。
-186行の`DocumentOutputs`に`StepID`がない。`procedure.go:82`と235行、および`boundary_end.go:74`は現在StepIDの宣言だけを使用するので、成功してもこのfeatureの採用を検査していない。
- 生存先は `codekb_test.go:10` `TestCodeKBIntegrationFeature`。こちらは`boundaryDeclaration`がStepIDを設定し、正しいcodekb配置の成功と誤配置拒否を対にしている。旧test削除でfeature採用の保証は失わない。長いintegration fixtureと結果生成を1本削れる。

<a id="f07"></a>

### F07 — 移動先が元と同じになったCodeKB testを削除【先行・確度高】

- [src/internal/flow/codekb_test.go:51](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/flow/codekb_test.go#L51) `TestCodeKBSharedDocuments` 全体。
- `boundaryDoc`が既にcodekbへ配置するため、55行で作るtargetはsourceと一致し、rename分岐は実行されない。残るのは通常の共有文書でBegin/endが通ること。
- 生存先は `selected_documents_test.go:117` `TestSelectedDocumentsSharedPaths`（別名の共有文書を実際に移しmetadata選択する）と`boundary_sensor_test.go:269` `TestEndSensorIntegrationDocuments`。共有文書の許可・選択保証は失わず、旧配置切替の不要な準備を削れる。

<a id="f08"></a>

### F08 — 名前しか追加の意味を持たない初期化成功testを削除【先行・確度高】

- [src/internal/flow/rule_skill_separation_test.go:91](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/flow/rule_skill_separation_test.go#L91) `TestUpstreamSkillInitialization` 全体。
- `executionFixture → Create → Begin`成功だけ。生存先は `execution_plan_test.go:122` `TestExecutionPlanBootstrap`（同じfixture、Begin、再試行、既存file保全）と`rule_skill_separation_test.go:11`の`default`（実配置）。新skill名を個別に照合していないため、固有の名前保証も失わない。

<a id="f09"></a>

### F09 — 旧入力形式で先に拒否される「未来入力」testを削除【先行・確度高】

- [src/internal/workflow/definition_test.go:99](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/workflow/definition_test.go#L99) `TestDefinitionAcceptedInputMustPrecedeStage` 全体。
- 注入する入力は`path`方式で`match`がない。現`definition.go:247`はここで`input match required`を返す。そもそもworkflow読込には固定catalog順の`accepted_at`前後制約はなく、flowの選択済み実行順で先行stepを判断する。
- 生存先は同package `declaration_test.go:12`の`input path`、flowの`execution_plan_test.go:345` `TestExecutionPlanEvidenceSelectedInputs`、655行`TestExecutionPlanEvidenceArtifactSelectedOrder`。「未来acceptedをworkflow catalog順で拒否する保証」は現在獲得していない。古い仕様を誤って生存仕様と読む維持コストを除ける。

<a id="f10"></a>

### F10 — 承認hash不一致で隠れる省略規則4ケースを整理【先行・確度高】

- [src/internal/flow/execution_plan_test.go:62](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/flow/execution_plan_test.go#L62) `TestExecutionPlanSchemaChoices` の`missing choice / empty reason / duplicate omission / selected and omitted` table。
- 正常計画に`fixturePlanApproval`を付けた後、各caseでSteps/Omittedだけを変更する。`execution_plan.go:66`の承認Target/PlanHash照合で先に失敗するので、省略規則を消してもこの4caseはそのまま通る。
- 推奨は4つの偽の規則検査を削り、正常対照は`TestExecutionPlanApprovalSameAnswer`へ集約。省略の入力契約を残す場合は`TestExecutionPlanDraftRejects`へ有効なPlanRequestの単一変更として統合する（`choice missing`は既存）。承認hashの不正そのものは意図を明記した1caseで十分。現在の4caseが保証するのは同じ承認改変拒否だけで、各省略規則の保証は失いようがない。

<a id="f11"></a>

### F11 — StepID欠落で隠れる保存version表を削除・集約【先行・確度高】

- [src/internal/flow/boundary_store_test.go:46](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/flow/boundary_store_test.go#L46) `TestBoundaryStoreVersions` の47～62行。
- 6個全てのStageEntryにStepIDがなく、`boundary.go:137`のentry stage/step照合がFileVersions検査より先に拒否する。`../escape`、不正SHA、runtime path、重複fileの区別が実際には検査されない。
- 生存先は `execution_plan_test.go:271` `TestExecutionPlanEvidenceBindings/entry` と363行`TestExecutionPlanEvidenceAcceptedRuns`。StepID誤結合はそこで残る。末尾の未知acceptance拒否もAcceptedRunsの誤ラベル拒否へ集約できる。
- file versionのpath/hash制約は現実の意味があるが、この表は現在それを保証していない。必要なら正しいStage/StepIDの対照を使う少数caseに置き換える。6回のworkflow状態作成で同じ早期拒否を踏むコストを除く。

<a id="f12"></a>

### F12 — 現在のdraftを持たないpending hash表を削除・集約【先行・確度高】

- [src/internal/flow/work_log_okf_test.go:203](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/flow/work_log_okf_test.go#L203) `TestOKFWorkLogRecoveryPendingValidation` 全体。
- pending fixtureに現在のDraft、StepID、PlanRevision、PlanHashがない。`store.go:147`のdraft結合が常に不正なので、`log_hash / log_after_hash × missing / bad`はhash検査を取り去っても失敗する。
- 生存先は `execution_history_test.go:318` `TestExecutionPlanReopenHistoryPendingBinding` の正常pending対照と単一field変更方式。log hashの不正を残すならこのtableへ片方ずつ統合する。現在の4caseを削って失う独自保証はない。旧schemaのmap再構築・full state準備を削れる。

<a id="f13"></a>

### F13 — Unit graphの不正を別条件で隠すtestを縮小【先行・確度高】

- [src/internal/flow/sensor_test.go:92](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/flow/sensor_test.go#L92) `TestFlowSensorUnitGraph`。
- unknown dependency/cycle用UnitにはBolt/Scope/Testsが全てなく、`unitPlanProblems`はグラフ検査がなくても`incomplete Unit plan`を返す。duplicate caseはSave側のID検査でも拒否され、98行以降はSave失敗をそのまま成功扱いにしている。
- 推奨は現在の3fixtureを削り、`unitPlanProblems`へ完全な正常Unitからdependencyのみ変える小tableにまとめる。これは任意の細部テストを増やす提案ではなく、依存循環で作業が永久待ちになる実害を検査する最低限の代替。claim時の未統合依存拒否は `unit_test.go:30` と`unit_without_git_test.go:53`に残る。現テストのまま保持してもグラフbugを捕捉できない。

<a id="f14"></a>

### F14 — 同一リクエストになるdraft拒否2行を1行へ【先行・確度高】

- [src/internal/flow/execution_plan_test.go:233](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/flow/execution_plan_test.go#L233) `TestExecutionPlanDraftRejects/duplicate` と`stage swap`。
- 初期requestの3番目はStage=tdd。`duplicate`はそこへID=s02を代入し、`stage swap`も同じ`{ID:s02, Stage:tdd}`を代入する。どちらも既存discovery IDのstage変更拒否であり、duplicate ID検査に到達しない。
- `stage swap`を生存させ`duplicate`を削る。persistされた同ID拒否は同file `TestExecutionPlanSchemaState/duplicate id`が残る。保証を失わず実配置fixture1回を削れる。

<a id="f15"></a>

### F15 — tracked/untrackedのGit軸を削除【先行・確度高】

- [src/internal/flow/reassign_test.go:312](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/flow/reassign_test.go#L312) `TestFlowUnitReassignLiteralPaths` の316行`tracked=false/true`ループと332～341行のGit add/commit。
- 現`reassign.go`、`unit.go`にGit実行・Git index参照はない。tracked差は製品に観測されないので各名前を2回試す意味がない。文字列の特殊path契約を残すなら各1回で十分。Gitコマンドのquote保証を失うが、現製品にはその経路がない。

<a id="f16"></a>

### F16 — literal fileをhashしていない結果確認を削除・一本化【先行・確度高】

- [src/internal/flow/reassign_test.go:312](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/flow/reassign_test.go#L312) `TestFlowUnitReassignLiteralPaths` のfile作成→result部分、および356行`TestFlowUnitResultLiteralPaths`。
- 両方ともUnit.Scopeだけをliteral pathに変えるが、`unitFixture`はUnit.VerificationPathsを元の`a.txt`へ固定済み（`unit_test.go:20`）。`unitVerificationPaths`はその明示値を優先するため、literal fileのbytesを読む／hashする保証はない。
- 推奨は3名前×2 trackedの大きなfixtureと別Testの重複を削り、必要なら`TestFlowUnitResultLiteralPaths`だけを、実際に対象fileをVerificationPathsへ入れた1つの複合名ケースとして残す。scope文字列が受理されるだけの現保証を独立の実行往復で維持する理由は薄い。`TestVerificationDigest`のFS・path変化検査は残す。現状のままtestを削ってもliteral fileの提出照合bugを新たに見逃すことにはならない。

### 保証を生存testへ集約してから削る候補

<a id="f17"></a>

### F17 — 初期化Finishの53行のほぼコピーを統合【統合・確度高】

- [src/internal/flow/execution_approval_test.go:325](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/flow/execution_approval_test.go#L325) `TestExecutionPlanApprovalFinishDropsPreviousUnits`。
-76行`TestExecutionPlanApprovalBootstrapFinish`と同じ未開始拒否→Begin→review→承認なしFinish拒否→Capture→Decide→Finishを再実行する。追加の意味は旧Unitsを次stepへ持ち越さないことだけ。
- 生存するBootstrapFinishに旧Unitsの設定と最後の消去assertを移して、コピー側を削る。初期化から承認済み完了までの保証とUnits消去を1往復で維持できる。別の初期状態を保持する必要があるなら共通の往復helperで重複を除くが、新しい汎用ハーネスは不要。

<a id="f18"></a>

### F18 — 初期化の小さな成功test4本を既存public往復へ集約【統合・確度高】

- [src/internal/flow/execution_plan_test.go:306](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/flow/execution_plan_test.go#L306) `TestExecutionPlanEvidenceInitializationEnd`、386行`...InitialSensor`、530行`...InitialReview`、618行`...StartGateStep`。
- Endの成功とInitialReviewは `execution_approval_test.go:76` のBootstrapFinishで既に必要条件。Sensor/start/reviewのStepID assertだけを `TestExecutionPlanBootstrap`またはBootstrapFinishへ移して4本を削る。
- 失う保証は移したassert込みでなし。各testが実配置とstateを一から作成し、内部helper単体成功と同じpublic成功を両方維持する負担を減らす。

<a id="f19"></a>

### F19 — 初期化の欠損file tableを一箇所へ【統合・確度高】

- [src/internal/flow/execution_plan_test.go:166](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/flow/execution_plan_test.go#L166) `TestExecutionPlanBootstrapMissingConfiguration`。
- Rule欠損行は `rule_skill_separation_test.go:11`の`missing`と重複するので削る。`rule_skill_separation_test.go:63` `TestRuleSkillSeparationHookMissingCLI` と77行`TestOKFSkillInitialization`は削除する代わりに、それぞれの欠損pathを同じBootstrapMissingConfiguration tableへ移す。
- `.codex/hooks.json`、3つの必要SKILLの各欠損は異なる配置漏れなので残す。意味のあるcase数は減らさず、Ruleだけ1回減らす。入口・fixtureの重複をなくす。

<a id="f20"></a>

### F20 — 同一rootのsession独立性を既存gate testへ統合【統合・確度高】

- [src/internal/flow/verification_gates_test.go:87](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/flow/verification_gates_test.go#L87) `TestVerificationGatesIndependence` 全体。
- 生存先は同file `TestVerificationGates/none`。既存のsame-root・異なるsessionでの正常assignの直前へ、現testのsame-root・同じsessionの拒否assertを移して入口を削る。`review_test.go:10`の`TestFlowReviewIdentityTarget`は別rootで同じsessionを拒否するため、それだけを代替として数えない。
- 同一rootでもreviewerのsession分離が必要という実害のある保証をそのまま維持し、sensor/Git fixture1個を削れる。

<a id="f21"></a>

### F21 — schema定数・CAS・旧schema拒否の重複を集約【統合・確度高】

- [src/internal/flow/boundary_store_test.go:12](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/flow/boundary_store_test.go#L12) `TestBoundaryStoreSchema`。
-18～28行のschema/CASは `store_test.go:23` `TestFlowStoreCreateCAS`で検査済み。旧schemaの拒否は `execution_plan_test.go:19` `TestExecutionPlanSchemaState/old schema` と `reopen_log_test.go:147` `TestDefinitionBindingMalformedState/old schema`にもある。同じschema数値違いを3回検査する必要はない。
- 生存先 `store_test.go:104` `TestFlowStoreWireSchema`へ未対応schemaのRead拒否と「読込失敗がfileを書き換えない」を1case移し、BoundaryStoreSchemaと他2か所の旧schema行を削る。現schema数字の単純assertも `documents_test.go:23`、`reopen_log_test.go:20`、`verification_cli_test.go:18`では削り、作成/保存の契約testへ限定する。
- 旧形式互換は不要だが、未対応形式を誤読して書き換えない保証は1か所に残す。schema更新時に無関係な機能testを修正する負担を減らす。

<a id="f22"></a>

### F22 — 同じ不正JSONを重複させず正常stateの単一変更へ【統合・確度高】

- [src/internal/flow/store_test.go:45](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/flow/store_test.go#L45) `TestFlowStoreRejectsCorruptAndIsolates` の末尾にある重複schemaキーrawの拒否を削除。`TestFlowStoreWireSchema/duplicate key`が正常stateから同じ検査を行う。ID/path/Space分離部分は残す。
- [src/internal/assignment/store_test.go:50](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/assignment/store_test.go#L50) `TestRegistry`の`unknown version`は実際には現在version=2だけのidentity不足JSON、`unknown field`はversion=1・identity不足でもある。現3行をそのまま維持せず、正常Registryを使う`TestRegistryRejectsDamagedRecords`へ、必要な不正schema/未知fieldを単一変更としてまとめる。壊れたJSON代表は1行でよい。
- 現複数行ではversion/fieldのどちらで拒否されたか保証しない。認識できないdataを拒否する入口を1つ維持すれば、同じ下位JSON処理の大量再確認は不要。

<a id="f23"></a>

### F23 — 旧field名を列挙したserialization検査を削除【統合・確度高】

- [src/internal/flow/verification_cli_test.go:56](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/flow/verification_cli_test.go#L56) `TestVerificationCLI`のMarshal→旧Git field名5つをContainsで探す部分。
- Typed Stateに存在しないfieldを標準json.Marshalが勝手に出力しないことを再確認している。現在のwire schemaは `TestFlowStoreWireSchema`に残す。旧Git API互換を望んでいないため、過去field名のブラックリストを保守する意味はない。Hashのread-only・Unit/root/path境界はこの削除に含めない。

<a id="f24"></a>

### F24 — 特定の旧field名を使う未知field拒否を削除【統合・確度高】

- [src/internal/flow/documents_test.go:148](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/flow/documents_test.go#L148) `TestIntentDocumentsLegacyFieldsRejected` 全体。
- 生存先は `store_test.go:104` `TestFlowStoreWireSchema`のunknown field。両者は同じDisallowUnknownFields付きdecoderであり、Config/ADRは通常のstruct。特定の旧field2名のために下位の未知field再帰処理を再検査する必要はない。将来独自Unmarshalへ変える場合は、その新しい境界に必要なtestを作ればよい。
- [src/internal/workflow/declaration_test.go:20](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/workflow/declaration_test.go#L20) `TestDocumentDeclaration/old refs`も、`definition_test.go:51`のunknown yamlと同じKnownFieldsへの旧名入力。旧互換不要の前提では削除候補。`unknown match`等、独自metadata validationの入口は残す。

<a id="f25"></a>

### F25 — workflow catalog再構築と重複拒否を集約【統合・確度高】

- [src/internal/workflow/execution_plan_test.go:10](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/workflow/execution_plan_test.go#L10) `TestExecutionPlanSchemaCatalog`。
- `definitionFixture`で作った6stageを再度全て書き直してからvalid6を検査している。valid6は `definition_test.go:33`、`reversed`は51行の`invalid prefix`、`advance`という未知fieldは同table`unknown graph`と重複。
- uniqueなprefix件数検査だけをDefinitionRejectsInvalidへ統合し、現fileのtestを削る。`missing`と`optional mandatory`は同じlen!=2条件なので片方でよい（厳密長契約の両端を特に保証するなら両方を小tableに残す）。6stageの再作成と旧固定edge形式の重複を省ける。

<a id="f26"></a>

### F26 — unknown agent roleを1tableへ【統合・確度高】

- [src/internal/workflow/definition_test.go:110](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/workflow/definition_test.go#L110) `TestDefinitionRejectsEmptyOrUnknownAgent`の`{role: unsupported}`と、`stage_planner_test.go:10` `TestStagePlannerRole/unknown role`を統合。
- `{}`と`{agent: aidlc-worker}`も同じrole不足を異なる無効inputで確認している。planner valid / wrong agentを生存させ、未知role1つ、role無し1つへ絞る。各agentを独立に登録し忘れるbugはwrong agentが残る。unknown roleに7fileのfixtureを重複させる負担を省ける。

<a id="f27"></a>

### F27 — 手順の文章コピーassertを削る【統合・確度中高】

- [src/internal/flow/stage_planner_test.go:26](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/flow/stage_planner_test.go#L26) `TestStagePlannerProcedure` の日本語5語Containsループを削除。
- 文面を固定するgolden自体もP11の整理候補であり、これらの語が存在しても意味・実行順・承認が正しいことは保証できない。実際のplan/result承認は `execution_approval_test.go`にある。22行のrole/order結合まで削る場合は、`workflow/TestStagePlannerRole`とP11の構造検査へ実際のrole/order結合を移してから全testを削る。
- 失うのは特定言い回しが残っているという保証。ユーザー向け説明の自然な言い換えのたびにTestを直す維持コストに見合わない。

<a id="f28"></a>

### F28 — 結果JSONの全Sensor往復を小tableへ集約【統合・確度高】

- [src/internal/flow/boundary_sensor_test.go:172](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/flow/boundary_sensor_test.go#L172) `TestEndSensorResults`の186行table。
- exit!=0 / command違い / SHA不正は `verification_results_test.go:11`に同じcollector実装のcaseがある。missing exitと未知JSON fieldは同小tableへ移す。全Sensorを使う側は正常と不正代表1つで「結果拒否がGateへ伝わる」を残す。
- `execution_plan_test.go:480` `TestExecutionPlanEvidenceResultRun`もStepID正/旧/空を同tableへ集約できる（旧stepは既存`step`、空は必要なら追加）。result読み込みの内容契約を失わず、Graph/accepted文書/Gitの準備を反復しなくてよい。
- `boundary_results_test.go:49` の実code変更→integration SHA不一致→更新後成功は、単なる40桁文字列検査とは別の保証なので残す。

<a id="f29"></a>

### F29 — 履歴破損5種×入口3種を7caseに縮小【統合・確度高】

- [src/internal/flow/history_review_test.go:12](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/flow/history_review_test.go#L12) `TestExecutionPlanReviewHistoryMutation`。
- 5種の破損は共通`verifyHistoryHead/readHistoryRecord`が判断する。1つの入口で5種類を検査し、残り`configure / pause / source`のうち2入口は同一の代表破損1つずつにする。15→7 case。
- 各入口が共通検査を呼ぶ保証と、各破損形式を拒否する保証を両方残す。省いた8組合せだけを特別扱いする将来の分岐は検出しなくなるが、現実装にはそうした入口別decoder/破損別分岐がない。Save/Transition/Captureの書込みゼロ・file保全assertは残す。

<a id="f30"></a>

### F30 — 低層collector再読込の重複を集約【統合・確度中高】

- [src/internal/flow/boundary_snapshot_test.go:11](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/flow/boundary_snapshot_test.go#L11) `TestBoundaryTransitionCollectorSnapshot` の`remove=false`を削る。
- 生存先 `selected_documents_test.go:145` `TestSelectedDocumentsCollectorSnapshot`は同じfile cacheを使いつつselected/materialが同じbytes/hashを共有するところまで確認する。`remove=true`は必要ならこちらのtableへ統合。
- `boundary_snapshot_test.go:35` `TestBoundaryTransitionUsesOneSensorSnapshot`は、Finish保存時にfileが変化/削除されても承認したproofを採用する別の事故を防ぐので残す。低層cacheのmap形状ではなく実際のproofを確認する役割を優先する。

<a id="f31"></a>

### F31 — 再割当復旧tableの後半を3回繰り返さない【統合・確度高】

- [src/internal/flow/reassign_test.go:284](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/flow/reassign_test.go#L284) `TestFlowUnitReassignPendingBlocksStateUpdates` の「later pause/resume/reassignでrunが変わる」部分（284～299行）。
- この段落はconfigure/other_unit/pauseの3caseで同じことを繰り返し、`TestFlowUnitReassignStoppedAndIdentity:31`の後半にもある。後者を生存させ、tableでは「別操作を拒否→同じ要求で同じrunを復旧」までにする。
- 同じrun復旧後でも後日再割当できる保証は1回残る。3回のpause/resume、registry置換、state保存の重複を削れる。

<a id="f32"></a>

### F32 — 死んだscope分岐を削る【統合・確度高】

- [src/internal/flow/reassign_test.go:172](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/flow/reassign_test.go#L172) `TestFlowUnitReassignScopeAndDependency`。
- tableは`dependency`1値だけなのにelseでoutside.txtをGit commitする旧分岐が残っている。elseと1行table/ifを削り、dependency拒否だけのtestにする。
- 動的に失う保証はない。Scope衝突は120行`TestFlowUnitReassignMissingRuntimeScope`が別に残る。到達しない旧Git境界を保守対象と誤認しなくなる。

<a id="f33"></a>

### F33 — work-log保存失敗の重複test群を一本化【統合・確度高】

- [src/internal/flow/reopen_log_test.go:43](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/flow/reopen_log_test.go#L43) `TestReopenLogFailures`を、`work_log_okf_test.go:102` `TestOKFWorkLogRecovery`のexisting/pending・log・finalへ統合。
- 両者は同じfixture経由で実`DecidePlan → recordPlanReopen`を使い、同じ3保存点の失敗・旧revision・同要求再試行を検査する。前者にだけあるreasonの見出し偽装escape、configure/pause/CheckWork拒否、古いexpect拒否を該当caseへ移し、前者を削る。
- `execution_history_test.go:222` `TestExecutionPlanReopenHistoryLogRetry`は同じfinal失敗の部分重複。publicなCapture/DecidePlanで承認を明示したこの1ケースを生存させるなら、Recovery側のexisting/finalの同じbytes不変assertはここへ寄せられる。保存点ごとの復旧契約を丸ごと削ってよいという提案ではない。
- 最終的にbase新規作成、pending保存、log保存、final状態の各失敗点を一度ずつ確認する。現在は書込み回数に依存した注入規則も重複しており、保存処理を小さく変えるたびに複数testを直す。

<a id="f34"></a>

### F34 — pending log改変拒否の同じ2caseを一箇所へ【統合・確度中】

- [src/internal/flow/reopen_log_test.go:109](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/flow/reopen_log_test.go#L109) `TestReopenLogChangedHistoryRejected`。
- `work_log_okf_test.go:242` `TestOKFWorkLogRecoveryRejects/missing pending, changed pending`と同じ目的だが、前者はlog追記後のfinal失敗、後者はlog書込み前失敗という差がある。
- 推奨は共通tableに`before log / after log`の必要な失敗点を明示し、改変・削除を別々の巨大fixtureで四巡しない。例えばbefore log/改変とafter log/削除を残し、失敗点ごとの正しいhash復旧はF33で保証する。新規log削除を勝手に再作成しない `TestReopenLogFreshDeletionRequiresRestore:176`は独自境界として残す。
- この縮小は単なる同一入力削除よりriskがある（before/after片方だけの特別な破損処理を新設した場合）。そのため保証の移動後に削るB候補。

<a id="f35"></a>

### F35 — work-log metadata容量超過の重複を削る【統合・確度中高】

- [src/internal/flow/work_log_okf_test.go:242](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/flow/work_log_okf_test.go#L242) `TestOKFWorkLogRecoveryRejects/metadata overflow`。
- 同じ「既存logがMaxBytes-10、追記で上限超過、state/logが不変」は `reopen_log_test.go:203` `TestReopenLogCapacity/overflow`が検査する。後者のfits・fresh encoded reasonとともに容量契約をまとめ、前者のtable行を削る。
- 既存logの上限超過を見逃す保証損失はない。正規OKF生成とmetadata増加を2箇所で組み立てる維持コストを減らす。

<a id="f36"></a>

### F36 — JSON marshal/unmarshalでtyped値を覗き直すだけの検査を削る【統合・確度高】

- [src/internal/flow/work_log_okf_test.go:155](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/flow/work_log_okf_test.go#L155) `TestOKFWorkLogRecovery`のPendingReopenをMarshal→mapへUnmarshal→`log_after_hash`長64とする段落。
- `current`は既にStore.Readでvalidatedされており、`store.go:151`はLogAfterHashを検査する。wire名を直接保証する必要があるならwire testへ一度集約し、この復旧tableではPendingReopenのtyped値と復旧後の実bytesを確認すれば足りる。
- 同じブロック後半のdifferent request拒否は残す。JSON tag/型の同一変換を保存点ごとに反復しても復旧の新しい保証はない。

<a id="f37"></a>

### F37 — assignmentの既存成功・alias・init retryを統合【統合・確度高】

- [src/internal/assignment/reservation_test.go:36](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/assignment/reservation_test.go#L36) `TestReservation`の`alias`行、fresh reserve retry、別root成功は `directory_test.go:12` `TestDirectoryAssignment`と重複。DirectoryAssignment側に同じroot/子/親/alias・別root・idempotent retryを集め、Reservation側はrequest内容変更、Space/Intent/ownerをまたぐroot衝突、release後retry非復活・再利用を残す。
- [src/internal/assignment/recovery_test.go:117](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/assignment/recovery_test.go#L117) `TestAssignmentRecoveryInitRetry`は `store_test.go:11` `TestRegistry/explicit initialization and aliases`へ同一要求の再試行assertを移して削る。
- 各公開操作の事故は生存testに残る。重複する初期化と昔のGit fixture構築を減らす。

### fixture・ケース数・頻度の候補

<a id="f38"></a>

### F38 — Git不要の通常fixtureからGit作業を除去【先行・確度高】

- [src/internal/assignment/reservation_test.go:10](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/assignment/reservation_test.go#L10) `registryFixture`：rootのgit init/commit、a/bのworktreeを通常directoryに置換。現assignment productionはStat/EvalSymlinksとregistryだけを使う。4外部Git processを全callerで反復する必要がない。`TestDirectoryAssignment`が既に非Gitrootの契約を検査する。
- [src/internal/flow/boundary_sensor_test.go:14](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/flow/boundary_sensor_test.go#L14) `boundaryFixture`：17～18行のGit init/commitを外す。`sensorFixture:22`、`unitFixture:12`、`discoveryApprovalFixtureOrder:124`にもその前提が伝播している。`unitFixture:27`のworker、`review_test.go:76` `flowReviewRoot`、`execution_approval_test.go:168,298`のreview rootは、実際に照合するbytesを持つ通常directoryへ置換する。
- `unit_test.go:47,100`のcommit/mergeは、製品が必要とするworker→projectの**file bytesコピー**へ置換。非Gitの同一folder試験だけで代替すると、別root間の提出照合を失うため、別rootで不一致→一致になる検査自体は残す。
- `execution_plan_test.go:480,509,557,655`のGit init/commitは当該検査に一切使わない。discarded `rev-parse`は `procedure_test.go:78`、`codekb_test.go:17`、`boundary_snapshot_test.go:41`、`boundary_sensor_test.go:275`、`boundary_transition_test.go:121`、`selected_documents_test.go:171`、`execution_plan_test.go:484`。全て削除可能。
- `assignment_test.go:114,258`のreassign要求へGit HEADを入れる行も現reassign処理ではそのSHAを内容照合しないため削除。legacy-resultの40桁HEADは後述の「残す境界」を正常な新形式データで作る際に除く。
- **失う保証**はGit tool/index/worktree/merge自身が使えることだけで、製品契約ではない。別root・alias・file内容不一致・二重担当拒否は残す。削減時間は未計測だが、コードから多数の外部processとfile tree生成を省けると判断できる。

<a id="f39"></a>

### F39 — 同じwhitelist条件のstage3値を1値へ【低優先・確度中高】

- [src/internal/flow/verification_results_test.go:113](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/flow/verification_results_test.go#L113) `TestVerificationResultsStageValidity`。
- 現productionはTDD/integration以外の拒否を同一条件で行う。initialization/discovery/planningの3full fixtureを、非検証stage代表1つにする。正常TDDは `TestVerificationResults`、正常integrationは `TestEndSensorIntegrationRequiresCurrentSHA`が残る。
- 失うのは将来特定の非検証stageだけを例外許可してしまった場合の名指し検出。現実装にはcase別の処理がなく、3回のGit・result fixtureに見合う差分は薄い。

<a id="f40"></a>

### F40 — Ruleの任意metadata禁止caseを縮小【低優先・確度中】

- [src/internal/workflow/rule_skill_separation_test.go:8](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/workflow/rule_skill_separation_test.go#L8) `TestRuleSkillSeparationRuleReference` の`title / description / status / tags / intent_id / invalid metadata`。
- これらは「固定Ruleに個別metadata条件を付けない」という同じ条件をfieldごとにコピーする。`invalid metadata`（空title）は`title`と同じnil以外の値であり、まず削除できる。さらに縮小するなら任意metadata代表1つだけ残す。
- path/type/version/match/count/role/accepted_atは参照先や意味が変わるため、この削除へ含めない。field別のnil比較を1個落とすbugを検出しなくなる実riskはある。6caseは軽い直接呼出しなので、F01～F38より優先しない。

<a id="f41"></a>

### F41 — 容量限界まで実保存する2testを低頻度または少数境界fixtureへ【低優先・確度中高】

- [src/internal/assignment/recovery_test.go:69](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/assignment/recovery_test.go#L69) `TestAssignmentRecoveryCapacity`（最大1,000回PreSpawn＋全pending PostSpawn）、129行`TestAssignmentRecoveryEscapedReleaseCapacity`（最大100回reserve/release）。
- 単純削除は非推奨。予約時の容量見積り誤りにより後でRelease/Postが保存不能になる現実の事故を防ぐ。現在は毎回全registryをparse/marshal/saveして上限へ近づけるため、I/Oと計算を累積する。
- 推奨順は、通常suiteでは境界直前の正しいRegistry fixtureからReserve/Release/Postを少数回実行する形に縮小し、繰り返し本物のadmissionで満杯にする原型は明示stress/配布前の低頻度へ移す。上限fixtureをproduction計算式のコピーで作らないこと。公開操作での容量逼迫を検査する独自性は低頻度側に残す。
- 移動だけならPR通常実行で容量回帰の発見が遅れる。特にRelease不能は重大なので、そのriskを明示して選ぶ。実行時間は未計測で、頻度変更を自動決定したわけではない。

## 削除対象から外した具体的な境界

- **二重担当とprocess間競合**：assignment `TestReservationProcess`、directory競合、別Space/Intent/ownerのroot重複、flowのUnit scope/root重複。goroutineのtestだけではprocess間lockを代替できないので、子processを使うこと自体をモック自己検証として扱わない。
- **途中保存と復旧**：Reserveが成功してflow stateだけ失敗、Replace後の書込み失敗、pending時に別操作を拒否、同じrequestだけ同じrunへ戻す。F31/F33は重複部分の統合であり、事故の種類を削らない。
- **承認の実在と結合**：未Captureのanswer拒否、replay、後から作ったrequestへ昔のanswerを使えないこと、plan/resultを同じ回答で承認する両順序、採用時の結果再検査。型やJSONの保証ではなくユーザー承認のすり抜けを防ぐ。
- **実行単位の識別**：同stage再実行の別StepID、過去受理結果の付け替え拒否、future/current/previous文書の選別、完了済みprefixの保持、reopen後も履歴を破壊しないこと。
- **実FS安全性**：FIFOを開いて停止しないこと、親/leaf symlink、非regular file、削除済み新規logの復元要求、文書選択とproofが同一snapshotであること。標準os APIを使っていても、Lstatを先に行うなど呼出し順は製品の責任である。
- **集合SHAの意味**：bytes/rename/追加/削除/empty directory/対象範囲、mtimeを意味へ混ぜない、managed filesの除外、上限、途中変更、別rootとの内容一致。SHA256ライブラリの正しさを再確認するtestではなく、何をhashするかという独自仕様を検査している。
- **legacyと名の付く拒否の一部**：`assignment_test.go:131` `TestUnitAssignmentLegacyReassign`、202行`...LegacyResult`は無管理run/registry欠落拒否という現行境界を含む。旧data互換が不要というだけで丸ごと削除しない。名前・40桁Git fixtureを新形式の「管理された予約なし」の有効な他fieldへ整理し、managed eligibilityのtestにまとめるのは妥当。
- **validなmetadataに対する独自検査**：workflowのnumeric type/tagやflowのRule type/body、Document宣言と実metadataの照合は単に型保証を試しているとは言えない。YAMLの入力変換と独自schema制約の接続があるため、担当外okfmemory側の完全な同値testを確認せず削除とはしなかった。


## CLI一周・実験用helper：削除・統合候補

### 確度の高い候補


<a id="c01"></a>

### C01 — 完全に同じ一周journeyを3本削除

- 箇所: [src/cmd/aidlc/flow_journey_integration_test.go:84](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/cmd/aidlc/flow_journey_integration_test.go#L84) `TestFlowJourney`、`:85` `TestBoundaryJourney`、`:86` `TestProcedureJourney`、[src/cmd/aidlc/documents_journey_integration_test.go:9](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/cmd/aidlc/documents_journey_integration_test.go#L9) `TestIntentDocumentsJourney`。
- 範囲: 後者3 test関数を削除し、空になるdocuments fileも削除。すべて同一 `runBoundaryJourney(t)` 呼出しで引数差もない。
- 生存: `TestFlowJourney`。CIの既存選択名もこれを参照している。
- 失う保証/現実的risk: 同じ処理を新しいtemp directoryでもう3回行う反復だけ。固有のdocuments/procedure保証は失わない。偶然のflaky bug検出確率は下がるが、そのための常時4重実行は不要。
- コスト: 広い `-tags=integration` 実行では3一周と9 `go build` invocationを削減。現在CIの選択regexは既にこの3本を実行しない。
- 推奨: 削除。確度: 高。

<a id="c02"></a>

### C02 — Git不要journeyの直積を縮小

- 箇所: [src/cmd/aidlc/git_independent_journey_integration_test.go:15](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/cmd/aidlc/git_independent_journey_integration_test.go#L15) `TestGitIndependentJourney`。`no-git` / `detect-git` × Unit無/有の4一周。
- 範囲: `no-git`・直接実装を1本、検出用Git stub・Unit有を1本にする。C01生存の `TestFlowJourney` と no-git直接実装の一周も共通なので、最終的にはこの2一周を共通入口へ統合可能。
- 生存: 同testの2代表case。`runGitIndependentUnits` はUnit有caseに残す。
- 失う保証/現実的risk: 「GitがPATHに存在しない場合のUnit」と「呼出し検出stubの場合の直接実装」の組合せ限定回帰。Git呼出し有無とUnitの組合せ依存に具体的根拠がないため全直積は過剰。stubは無視されたGit失敗も検出でき、no-gitと役割が同一ではないため両環境を1つずつ残す。
- コスト: 全一周2回分。確度: 高（入口名の整理はCI選択を同時更新）。

<a id="c03"></a>

### C03 — observer引数の実装コピーtestを削除

- 箇所: [src/cmd/aidlc/observer_command_test.go:20](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/cmd/aidlc/observer_command_test.go#L20) `TestObserverBinaryArgs` 全体。
- 範囲: testだけ削除し `observerHookCommand` helperは必要なcallerが残る間維持。期待値もhelperも `filepath.Dir`/`Join` と `.exe` 条件を同じ式で作っている。
- 生存: `TestMainHookCommand` (`command_test.go:14`)、実際のhookを通すjourney/live、配布hook構成は [src/harness/codex/split_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/harness/codex/split_test.go)。
- 失う保証/現実的risk: test専用wrapperの文字列引数の局所確認。実際のwrapper引数破損はhook実行testで検出できる。同じ計算の誤りを両方へ写すbugは現testでも検出できない。
- コスト: 小、維持負担中心。推奨/確度: 削除/高。

<a id="c04"></a>

### C04 — filepath標準処理を再確認するtestを削除

- 箇所: [src/cmd/aidlc/hook_probe_live_test.go:399](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/cmd/aidlc/hook_probe_live_test.go#L399) `TestHookProbeCanonicalRoot`、`:424` `TestHookProbeTrustConfig`。
- 範囲: 前者全削除。helper `hookProbeCanonicalRoot` は `filepath.EvalSymlinks(root)` 1行で、期待値も `EvalSymlinks`。後者は単一JSON quote+定数連結の文字列比較なので削除候補。
- 生存: `TestHookProbeLive` の実canonical root/trust起動。製品のroot解決は `src/internal/projectroot/root_test.go:10,42`、配布native。
- 失う保証/現実的risk: Go標準のsymlink解決の再確認とtest専用設定文字列の局所的誤字。製品root/trust設定の保証ではない。
- コスト: 前者FS/symlink（Windows権限に依存）、後者ほぼゼロ。推奨/確度: 前者削除/高、後者削除/中高。

<a id="c05"></a>

### C05 — mainのCLI早期return直積を削減

- 箇所: [src/cmd/aidlc/main_test.go:106](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/cmd/aidlc/main_test.go#L106) `TestSpaceCreatorLazyCLIInputs`（10入力）、`:279` `TestSpaceListerLazyCLIInputs`（15入力）、`:428` `TestSpaceSwitcherLazyCLIInputs`（19入力）。
- 範囲: main adapter側は1つのroot help/versionで依存を呼ばないcaseと各workspace commandの1つの不正入力までに縮小。残るCLI文法の列挙は削除。
- 生存: [src/internal/cli/space_test.go:169](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/cli/space_test.go#L169) `TestRunSpaceCreateInvalidArguments`、`space_list_test.go:140,184,490` `TestRunSpaceListExtraArguments` / `DuplicateJSON` / `InvalidFlags`、`space_switch_test.go:165` `TestRunSpaceSwitchInvalidArguments`。すでに不正入力でcallbackが呼ばれないことも確認。root help/versionは `cli_test.go:71,152`。
- 失う保証/現実的risk: 44入力それぞれがmain helper内のgetwd/getenv/fsを呼ばない局所保証。parserがcallbackを呼ばない既存testで同じbugを検出できる。main自体は `cli.Run` に委譲している。
- コスト: 主に件数/改修負担。推奨/確度: ケース縮小/高。

<a id="c06"></a>

### C06 — RootInput testの呼出し順・回数を削る

- 箇所: `src/cmd/aidlc/main_test.go:28,187,349` `TestSpaceCreatorRootInput` / `TestSpaceListerRootInput` / `TestSpaceSwitcherRootInput`。
- 範囲: getwd/getenvの厳密回数、constructor時0回、envKeysの呼出し順のassertを削除。`ExplicitDir`/`WorkingDir`/環境値が正しい欄へ入り、生nameと結果が正しく渡ることは3 adapterで残す。
- 生存: 同3 testの入出力assert。副作用を早期実行しない保証はC05の代表case。
- 失う保証/現実的risk: 環境取得回数が増える、順序が変わる内部変更を検出しない。そこに利用者の意味のあるbugは確認できない。入力欄の取り違えは残るassertで検出。
- 推奨/確度: 内部assert削除/高。

<a id="c07"></a>

### C07 — adapterの純粋error転送testを統合

- 箇所: `main_test.go:85,243` WorkingDirectoryFailure、`:388` SwitcherFailures/cwd、`:172` CreatorCreationFailure、`:264` ListerReadFailure、`:388` SwitcherFailures/switch。
- 範囲: 6独立関数・subtestを共通tableへまとめ、単なる委譲errorの文字列/同一性を重複assertしない。特に既存workspace errorを返すだけの3caseは削除候補。
- 生存: cwd errorの1代表、3 RootInput wiring、`main_unix_test.go` の実workspace failure出力、および内部CLI error出力tests。
- 失う保証/現実的risk: adapterの一箇所だけerrorを握り潰す回帰。3 adapterは現在別関数なので完全削除の優先度はC05/06より低い。tableへ統合しても実行caseを全部残すだけなら時間削減はほぼない。
- 推奨/確度: 統合、転送だけの3case削除は中。

<a id="c08"></a>

### C08 — main packageに置かれた他packageのhelp検査を削除・統合

- 箇所: `flow_command_test.go:11` `TestFlowCommandPublicCutover`、`rule_skill_separation_test.go:10` `TestRuleSkillSeparationHelp`。
- 範囲: mainを呼ばず `cli.Run`/`cli.Help`/`okfcli.Help`だけを検査する両testをmainから削除。廃止route `next/report/intent legacy/__codex-user-prompt-submit` の全別名列挙は削除候補。公開helpの必要なrouteを残すなら内部CLIの1つの契約testへ統合。
- 生存: [src/internal/cli/five_cli_test.go:10](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/cli/five_cli_test.go#L10) `TestFiveCLIContract`、`command_test.go:58` `TestHookCommandDispatch`、`execution_plan_test.go:8` `TestExecutionPlanCLIGrammar`、`boundary_test.go:23` `TestBoundaryRootHelp`、`rule_skill_separation_test.go:10`。
- 失う保証/現実的risk: 古いコマンド名を偶然再導入した時の個別否定check、3特定helpに`--project-dir`が残ること。現行公開コマンドの動作検査ではない。廃止名全列挙を保持する必然性は低い。
- コスト: 小。推奨/確度: mainから削除・契約集約/高。

<a id="c09"></a>

### C09 — root resolver forwarding testを実装packageへ統合

- 箇所: `project_root_test.go:10` `TestProjectRootWithoutGit`。対象 `project_root.go` は `projectroot.Resolve`への1行委譲。
- 範囲: project/explicit/ambiguous は既存 `src/internal/projectroot/root_test.go:10,42` と統合。application（祖先探索）・missing・new installはそこで未網羅なので、必要な3caseだけ同packageへ移しmain fileを削除する。
- 生存: `TestResolveAncestorFiles` / `TestResolvePreservesErrors` と移した3case、実際のCLI root解決はjourney。
- 失う保証/現実的risk: forwarding wrapperのみの取り違え。現在引数をそのまま委譲しており、実CLIテストでも検出。3caseを移さず全部削除すると未配置fallbackの回帰検出が減るので「完全重複」とは扱わない。
- 推奨/確度: 統合/高。

<a id="c10"></a>

### C10 — Unix閉pipeの直積を縮小

- 箇所: `main_unix_test.go:22` SwitchClosedPipes、`:168` ListClosedPipes、`:241` CreateClosedPipes、`:313` RootCommandsKeepSIGPIPE。
- 範囲: Listの16caseを `list human/stdout`、`list JSON/stdout`、`bare human/stderr syntax`、`bare JSON/stderr root error` の4代表へ。Switchは `success/stdout/stderr syntax` を残し `both/stderr workspace failure` を削除候補。Createは `stdout/stderr missing name` を残し `both/stderr invalid flag` を削除。rootは `help/unknown` の2代表にし7別名を削除。
- 生存: 同testの代表case、文法・output preparationは内部CLI `space*_test.go`。`TestMainSpaceList` (:121) の実呼出しは維持。
- 失う保証/現実的risk: 異なるaliasだけsignal設定が壊れる、stdout失敗に続くstderr失敗だけで死ぬ組合せの検出。signal.Ignoreは共有 `PrepareOutput` であり、両pipeの直積を全commandへ繰り返す理由は弱い。**閉pipe自体はGoランタイムのSIGPIPEとmain wiringの実bugを検出するので全削除しない。**
- コスト: 少なくとも23 subprocess case程度削減。`mainProcess`はtest exe再利用で毎回buildではない。確度: 高、Bothを1本だけ残す判断は可。

<a id="c11"></a>

### C11 — 出力失敗後のscaffold完全コピー期待値を削除

- 箇所: `main_unix_test.go:419` `assertSpaceRetainedAfterOutputFailure`。
- 範囲: directory/file正確件数と全seed proseの期待値を削除。生成済みSpace/代表fileが残ること、同名retryが既存作成を上書きせず失敗しsnapshotが変わらないことだけ維持。
- 生存: [src/internal/workspace/space_create_integration_test.go:184](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/workspace/space_create_integration_test.go#L184) `TestCreateSpaceScaffold` が生成内容を検査。mainの残存データ保証は同helperの縮小版。
- 失う保証/現実的risk: 閉pipeの場合だけ特定seedが変化する組合せ。write stdoutは生成完了後なので現実性が低い。生成完了後にrollbackするbugは縮小版で検出。
- 推奨/確度: assert削除/高。

## 実験用判定器・fixtureを通常suiteから整理する候補

<a id="c12"></a>

### C12 — GitIndependentFixtureのJSON自己検証を削除

- 箇所: `git_independent_fixture_test.go:9` `TestGitIndependentFixture`、`:37` `TestGitIndependentFixtureReservation`。
- 範囲: 自作result生成→自作structへUnmarshal→同じ固定値をassertする部分、bad SHAのhelper内format判定、reservationのempty/invalid JSONを削除。PATH隔離確認と`found/missing`を残す場合も実journeyに近い1検査へ統合。
- 生存: `TestGitIndependentJourney` は生成結果を**実製品が受理すること**、正しいUnit reservationを使って実操作することを確認する。
- 失う保証/現実的risk: fixtureだけのformat/selector error検出。JSON marshal/unmarshal型保証の再確認は失って問題ない。PATH隔離のbugは「Git不要だった」という偽の結論を生むため削るならdetect stubの確認を生存testに残す。
- 推奨/確度: 大半削除、PATHは統合/高。

<a id="c13"></a>

### C13 — 合成flow evidenceと旧live hostを退役

- 箇所: `flow_evidence_test.go:248,261,410,420,451` の5test全体、`verifyFlowProof`・`validFlowProof`等専用型/helper。`flow_live_integration_test.go:445` `TestFlowJourneyLive` と `flowHostJob` (:277)、専用request/test helper mode。
- 根拠: verifierは `intent advance` 4回以上 (:51)、stage数4 (:69)、Git commit/別worktree型hostに固定。現行 [src/internal/flow/transition.go:13](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/flow/transition.go#L13) はadvanceを無条件で廃止エラーにする。現行workflowはinitializationを含む。合成fixtureの「pass」は製品の現行一周を証明しない。
- 範囲: 旧一周liveとその真偽表を削除。共有 `flowShellWords` / `flowRunModel` / transport読取り / hook helperは他の現行liveが使うので無条件にfileごと消さない。
- 生存: 現行 `TestFlowJourney` / `TestGitIndependentJourney`、`TestHumanApprovalLive`、`TestAssignmentJourney`。transportの真偽表それ自体がadvanceで無効になるわけではなく、旧専用hostの退役で不要になる分だけを対象にする。現在の実Codex担当動作はRAMの後続native試験も根拠だが、この監査では再実行していない。
- 失う保証/現実的risk: 旧独自hostがGit worktreeの3workerを回す実験の自動判定。現行製品の利用方式を守る保証としては使えない。旧hostの45分live timeoutと大きな専用保守を除ける。
- 推奨/確度: 退役/高。

<a id="c14"></a>

### C14 — 境界・procedure・実行承認 evidence真偽表を診断suiteへ縮小移動

- 箇所: `boundary_evidence_test.go:118` Incomplete、`:124` Sequence、`:172` Command、`procedure_evidence_test.go:112` Sequence、`execution_plan_evidence_test.go:27` DistributionEvidence。
- 範囲: 空配列拒否と各eventを1個ずつ落とす7/12重ケース、bool/string fieldを7種反転する表を、記録再生の正例1・実行transport欠落1・不一致identity1程度へ縮小。shell wrapper全組合せも観測済みの1例へ。helperを含め明示的diagnostic build tag/packageへ移す。
- 生存: 実製品 [src/internal/app/boundary_test.go:12](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/app/boundary_test.go#L12) `TestBoundaryHookRepairAndBegin`、`execution_plan_test.go:93` PendingHook、`src/internal/flow/execution_approval_test.go:14,52,76`。実機確認は `TestBoundaryLive` / `TestProcedureLive` / `TestHumanApprovalLive`。
- 失う保証/現実的risk: test-only証拠判定の欠落eventごとの厳密拒否。製品が未承認で動くbugは内部製品testsで検出。診断のfalse pass riskは残るため、validatorを活かすなら実記録再生を少数残す。現在は全部syntheticで、現実のwire変化は検出しない。
- コスト: 標準suiteの件数/コンパイルと保守。推奨/確度: 縮小して低頻度へ/高。

<a id="c15"></a>

### C15 — TestHookProbeVerifyの22変形を観測記録再生へ統合

- 箇所: `hook_probe_test.go:42` `TestHookProbeVerify`。`hookProbeFixture`が期待されるevent/call/fileを自作し、同fileの`hookProbeVerify`で判定する。
- 範囲: complete以外の1fieldずつの21変形を削除候補。特にwrong exit/status/sessionはGoやCodex動作でなく合成checkerのifを反転させている。観測済みasync start→poll→terminalの記録は `hook_probe_live_test.go:437` `TestHookProbeObservedTransport` に集約し、missing terminal/unknown wrapperの代表だけ残す。
- 生存: `TestHookProbeObservedTransport` と `TestHookProbeReplay` (:477)。live入口 (:139) は標準Codex hook互換調査として低頻度継続。
- 失う保証/現実的risk: 仮想dataから誤ってpassするchecker回帰。製品hookはこのprobeの自前denyを使わず、本体regressionではない。完全に全削除するならvalidatorを合否gateとして信頼しない扱いへ変える必要がある。
- 推奨/確度: 縮小・診断suite移動/高。

<a id="c16"></a>

### C16 — HookProbeHelperの自前protocolを通常suiteから移す

- 箇所: `hook_probe_live_test.go:87` `TestHookProbeHelperProtocol`（session/deny/stop_first/stop_second）。
- 範囲: ordinary suiteから外し、probe準備を変更した場合だけ実行。4 subprocessでtest helper自身のJSON/stdout/event保存を検査しており製品呼出しはない。standalone JSONとone-time Stopの2代表に縮小可。
- 生存: 実機 `TestHookProbeLive` / Replay。外部Codexがstdoutを解釈できることはsynthetic helper protocolだけでは証明されない。
- 失う保証/現実的risk: 診断wrapperがGo test PASSを混ぜる、記録保存を壊すbugの発見が診断実行時になる。probeだけの保守として十分。
- コスト:4 subprocess。確度: 高。

<a id="c17"></a>

### C17 — AgentHookProbeの常にinconclusiveになるcaseを削除

- 箇所: `agent_hook_probe_protocol_test.go:104` `TestAgentHookProbeEvidence`。
- 範囲: `parent_cwd_is_not_worker_root`、`prompt_root_is_not_worker_root`、`stop_with_remaining_process`、`stop_without_process_observation`、`unperformed_resume` を削除。`agent_hook_probe_test.go:131` `agentProbeEvaluate` はG0-2〜6を常にinconclusiveとする固定map。さらに先頭fixtureのCallsはSession未設定なのでG0-1 negative群も同一の前段拒否に潰れており、各mutation固有の条件を検証していない。
- 生存: 同test `measured_deny_and_allow_control`、より後段の `TestAgentHookProbeEvidenceObservedWire` (:599) の適正なcontrolから変更するcase。
- 失う保証/現実的risk: 「未実装checkerが固定文字列を返す」保証だけ。各gapの製品/外部動作保証は元からない。
- 推奨/確度: 削除/最高。

<a id="c18"></a>

### C18 — AgentHookProbeの12並列temp file検査・重複faultを削除

- 箇所: `agent_hook_probe_protocol_test.go:32` Protocolの`parallel_unique`、9 routing case、`save_failure`。`:190` Fixture/fault_modes、`:552` ProtocolObservedFault。
- 範囲: `parallel_unique`の12 subprocessは `os.CreateTemp`の一意性を再確認するため全削除。旧名spawn_agent側と観測名collaborationspawn_agent側の同じdeny/otherrole/post直積は観測名のallow/denyだけへ。fault_modesの3caseはObservedFaultのmissing/nonzero/save-failureと重複するため削除。ObservedFault自体も診断suiteへ。
- 生存: 観測名allow/deny protocol代表、ObservedFaultの必要なfault方式、実機G0記録。
- 失う保証/現実的risk: 診断記録名の衝突と未使用旧tool名経路。標準CreateTempに任せる部分の保証を自前で持たない。fault helperが1秒deadlineに到達する確認はcontext/execの標準動作で、モデル側のtimeout対応は検査していない。
- コスト: 少なくとも12 subprocess＋重複呼出し。ObservedFault/timeoutは常時1秒待ち、helper/race実行時の終了待ちもある。推奨/確度: 削除・診断移動/高。

<a id="c19"></a>

### C19 — 実験prompt・budget定数・Git worktreeの自明testを削除

- 箇所: `agent_hook_probe_protocol_test.go:190` Fixture/opt_in_and_platform、actual_worktrees、all_cases_have_requests、`:364` FixtureBudget、`:530` FixtureYield、`:795` FixtureObservedSchema。
- 範囲: 全削除候補。定数のtimeout値とprompt語句の包含、全9シナリオに同じphraseがあること、実際に `git worktree add` が `.git` にgitdirを書くことを再確認する。
- 生存: `agentProbePrepare` を変更した時の1つの生成物目視/診断smoke、実際のprobe起動。isolation/記録copyを残すならFixture/isolated_setupとFixtureCapture (:325)だけ診断suiteへ。
- 失う保証/現実的risk: 実験指示の特定単語が消えたこと、固定budget変更。指示の意味が壊れても文字列包含なら現在のtestはpassする。製品のagent設定は検査していない。
- コスト: scenarioごとの多数directory生成・symlink・Git subprocess、特にactual_worktreesは通常suiteをGit必須にしている。推奨/確度: 削除/高。

<a id="c20"></a>

### C20 — Agent evidence collectorの三重synthetic表を統合

- 箇所: `agent_hook_probe_protocol_test.go:373` EvidenceCollectedControl（11）、`:599` EvidenceObservedWire（13）、`:824` EvidenceObservedOptionalFields（9）、`:325` FixtureCapture。
- 範囲: 手作りobject/string/array wrapper全系列を生成・書出し・読戻しする33caseを、固定版の**実際に観測した**deny+allow記録1セット、そのprovenance不一致1、未完了capture1へまとめる。optional fieldのnull/empty/number/mismatch直積は1つの不正親identityに縮小。
- 生存: ObservedWireの正例、必要なwrong-parent/opaque-not-proven等。FixtureCaptureのraw byte preservationをそこへ統合。
- 失う保証/現実的risk: 診断collectorの不正record各形状を拒否する細分化保証。親子の別recordを誤採用するriskは実験評価に現実性があるためwrong-parent代表は残す。製品native dispatchの保証は [src/internal/app/agent_hook_test.go:50](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/app/agent_hook_test.go#L50) と `TestAssignmentJourney` が担当。
- コスト: file生成/JSON書換えが多い。推奨/確度: 縮小・低頻度移動/中高。

<a id="c21"></a>

### C21 — AgentHookProbeLiveを回帰suiteから実験入口へ明確化

- 箇所: `agent_hook_probe_live_test.go:245` `TestAgentHookProbeLive` と同fileの専用prepare/collector/process helpers。
- 範囲: liveprobe等の明示tag/packageへ移す。G0-1以外はinconclusive、最後のaggregate statusもassertせず記録だけ。現行test成功を製品回帰成功と誤読しない名称・入口へ。
- 生存: 必要時の固定Codex互換調査。current native product testは別に維持。
- 失う保証/現実的risk: 通常時は既にenv未設定でskipなので実行保証は減らない。通常suiteでhelper準備の不具合を検出する頻度だけ下がる。
- コスト: liveは9 case×通常2分/ライフサイクル5分上限、各caseでGit/worktree準備。通常はskipなので時間削減を大きく見積もらない。確度: 高。

<a id="c22"></a>

### C22 — Reliability observerの通常suite内buildと四重実行を縮小

- 箇所: `hook_reliability_probe_test.go:106` `TestHookReliabilityProbeProtocol`。
- 範囲: whole testをdiagnostic tagへ。現在は毎回本体を `go build` し、terminal/duplicate/lock_failure/invalid_wireごとに①製品直実行、②capture、③test helper経由、④shell fallback経由を繰り返す。残すならwire保全の成功1・非zero1とcapture failure fallback1だけ。
- 生存: [src/internal/app/hook_reliability_test.go:120](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/app/hook_reliability_test.go#L120) `TestHookTerminalPersistence` がmatching/duplicate/old ID/他session/Stopと保存失敗retryを製品に対して確認。`TestMainHookCommand` が入口を確認。
- 失う保証/現実的risk: 観測wrapperがraw/stdout/stderr/exitを変えない保証の細分化。これは重要な診断品質だが製品の全revisionでcompileし直す必要はない。製品bugを同じ製品の出力と比較する部分は製品期待値oracleにもなっていない。
- コスト: 通常suiteごとにbuild1回＋最大16製品実行、Go test helper終了をraceでも反復。推奨/確度: 縮小・低頻度へ/高。

<a id="c23"></a>

### C23 — Reliabilityの旧hookを自作して認識確認するtestを整理

- 箇所: `hook_reliability_probe_test.go:342` Evidence/prepare_preserves_registration、`:538` `reliabilityPrepare` 内の完全一致command。Protocol内のregistrationも同様。
- 根拠: helperが探す完全一致文字列は `aidlc __hook --project-dir ROOT` の旧形。test自体がこの旧形を登録している。現製品は [src/harness/codex/split.go:39](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/harness/codex/split.go#L39) で `--okf-binary OKF` を付加するので、現在の通常配置を使うprobe準備には一致しない。
- 範囲: 自作旧registrationを使う正例を削除。診断を残すなら実配布生成物から1つの登録を入力にしたwire保全smokeへ統合。旧shapeの互換性を製品要求にしない。
- 生存: actual candidate installを使う [src/cmd/aidlc-dist/release_integration_test.go:274](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/cmd/aidlc-dist/release_integration_test.go#L274) と [src/harness/codex/split_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/harness/codex/split_test.go)。observerを現在配置へ差し込める保証は現存testにはない。
- 失う保証/現実的risk: 旧4引数hookを観測wrapperに置換できることだけ。現行製品に対する有効保証は失わない。
- 推奨/確度: 旧fixture正例削除・現配布へ統合/最高。

<a id="c24"></a>

### C24 — Reliability evidenceの巨大boolean直積を縮小

- 箇所: `hook_reliability_probe_test.go:342` Evidence（child_request / child_receipt /18 terminal変形）、`hook_reliability_opaque_test.go:115` OpaqueEvidenceとhelpers、`hook_reliability_probe_integration_test.go:365` CollectedEvidence、`:460` OpaqueReceiptSynthetic。
- 範囲: child_requestの文章包含assert削除。opaqueの7identity field×wrong/empty×pre/post、snapshot5×pre/post、order8等の直積を、正例・wrong identity・欠落Post・final-only・opaque mismatchに縮小。`:460`は同じopaqueFixtureをfileへ書きTest関数へ渡すだけなので削除（test関数を直接再呼出し）。18 terminal変形もcollectedの実記録正例/不完全例へ統合。
- 生存: 診断の `TestHookReliabilityProbeCollectedEvidence` 正例/terminalなし、`TestHookReliabilityProbeOpaqueReceipt` 実記録再生。製品のsession解放/親宛制限は内部app HookTerminalPersistence/ChildHookNotifications。
- 失う保証/現実的risk: 自作検証器の拒否条件の一部。opaque ciphertextの到達は外部wire観測であり製品認証ではない。checkerを残してgateに使う限りwrong identity/no terminal代表を残す価値はある。
- コスト: CPUは小、独自protocol/保存形式6fieldへの結合と維持量が大。推奨/確度: 縮小・診断suiteへ/高。

<a id="c25"></a>

### C25 — Assignment process fixture自身の時間testを低頻度へ

- 箇所: `assignment_process_test.go:110` `TestAssignmentProcessRendezvous`（missing peer / pair+duplicate nonce）、`:73` subprocess helper。
- 範囲: ordinary suiteから外す。必要ならpair rendezvousとfinite cleanupを1つのdiagnostic smokeに統合し、30ms待ちが1秒内というwallclock assertを削除。
- 生存: `TestAssignmentLive`で本当に同時開始観測が必要な場合の有限process helper。製品のassignment exclusionは内部store testsと`TestAssignmentJourney`。
- 失う保証/現実的risk: 自作実験processのrendezvous/nonce保全。不具合は実験のinconclusiveになる。製品worker起動はこのhelperを使わない。
- コスト: このtest自体は同processのgoroutine・ポーリング（subprocessではない）。負荷下の1秒締切flakiness。liveから使うhelper入口は別process。推奨/確度: 移動・壁時計assert削除/高。

## 古いjourneyの不要範囲・現在の役割

<a id="c26"></a>

### C26 — 毎callerの3 CLI buildを共有・必要分だけにする

- 箇所: `flow_journey_integration_test.go:19` `buildAIDLCBinary` と各caller。`operations_integration_test.go:40` が各Operations testから呼ぶ。
- 範囲: 同じsource・同じflagsのaidlc/okfをtest実行単位で1回だけbuildし、全fixtureへ明示的に渡す。natural-japanese-goの常時buildを削除。必要なstage liveは既に `AIDLC_NATURAL_JAPANESE_BINARY` の別binaryを要求する (`stage_skills_live_integration_test.go:207`)。
- 生存: 製品binary自体を実行する全journey。binary生成保証はshared build1回、自然日本語は`TestCommandBinary`と配布native。
- 失う保証/現実的risk: 同じソースがtemp pathごとに再buildできること。ソースやflagsが異なるtestは共有しない。プロセスのworking dir/state/evidenceは引き続きfixtureごと分離するのでテスト干渉を持ち込まない。
- コスト: C01だけでも9build削減、CI現行2入口だけでも2つのJapanese buildを削減可。全Operations/旧liveを含む広いsuiteではさらに多い。Kagome辞書linkが重いことは構成上明らかだが今回個別時間未計測。
- 推奨/確度: helper統合/最高。

<a id="c27"></a>

### C27 — Git前提の準備を通常directoryへ簡略化

- 箇所: `flow_command_unix_test.go:20`（init/commit/rev-parse/worktree）、`operations_integration_test.go:33` operationsNew / :208 tdd / worktree helper、`configure_help_integration_test.go:16`、`assignment_integration_test.go:15`、現行liveのgit init。
- 範囲: root identity/review/Unitが対象のfixtureはGit操作を削り通常rootで作る。`flow_command_unix_test.go:59`付近のHEAD取得は結果すら使わないので削除。ConfigureHelpの `<CURRENT_HEAD>` 置換と最後のHEAD不変assertも現helpにはplaceholderがなく無効なので削除。
- 生存: それぞれのtestの実CLI操作・出力assert。Git不要性はC02の明示2case。
- 失う保証/現実的risk: Git worktreeやcloneの準備手順の確認。現製品はGit不要・通常rootを支持し、これらの準備自体は保証すべき機能ではない。別rootを必要とする境界caseまで全て同rootへ潰さない。
- コスト: 特に各Operations fixtureのGit実行と履歴管理。推奨/確度: fixture削減/高。

<a id="c28"></a>

### C28 — failed reviewの無効なadvance assertionを削除・現境界へ統合

- 箇所: `flow_journey_integration_test.go:148`〜155、`runGitIndependentBoundaryJourney`内。
- 範囲: fail review後の `intent advance` が非zeroならpassとするassertを削除。現在 `flow/transition.go:13` がreview状態に関係なくadvanceを拒否する。受入gateを一周で確認したいなら既存finishの負例1つに統合。
- 生存: 内部flowのreview/finish承認tests、同journeyのpass→finish。
- 失う保証/現実的risk: 廃止actionが失敗することだけ。failed reviewを理由に進行が止まる保証はこのassertには元からない。
- 推奨/確度: 削除（同時に誤った保証説明を修正）/最高。

<a id="c29"></a>

### C29 — OperationsのGit clone / merge演習を縮小

- 箇所: [src/cmd/aidlc/operations_integration_test.go:298](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/cmd/aidlc/operations_integration_test.go#L298) `TestOperationsGitHandoff`、`:539` `TestOperationsGitConflict`。
- 削る部分: cloneでGit tracked bytesがコピーされること、merge marker/ls-files-uの生成、Gitで手動resolve→commitできることを検査する演習。製品の移転は `TestReleaseCandidateNative` とC30のhook canaryへまとめる。
- **残す部分**: runtimeをGit共有しない配布資材の規則、移転先で管理された予約がないrunを拒否すること、不正JSONを既存fileの非改変で拒否すること。無管理run拒否は [src/internal/flow/assignment_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/flow/assignment_test.go)、不正保存dataは `store_test.go:45`、CLIのerror出力はFailureOutputへ照合・統合してから古い一周を削る。
- 独立照合による限定: `:342` の40桁HEADは成功系でなく、runtime欠落による失敗を期待する負例。assignment読込みが先に失敗するため、SHAの桁数だけでは無効なassertとは言えない。大文字 `ADR` はcase-sensitive FSでの早期失敗要因。これは現行schema/runのclone試験なので、旧形式の後方互換不要を削除理由にはしない。
- 失う保証: Git自身のclone/mergeの動作と、Git演習を含む複合一周。製品のruntime非共有・管理外操作拒否は維持する。長い工程準備・Git processを減らせる。**統合後の部分削除、確度高。**

<a id="c30"></a>

### C30 — RelocationCommandの後半TDD再一周と実装コピーassertを削除

- 箇所: `relocation_integration_test.go:59` `TestRelocationCommand`。`:117`〜125にquoted path `bytes.ReplaceAll` の期待値、`:145`〜203に旧registry→再TDD→2worktree→go test→commit/merge→integrate。
- 範囲: 後半を削除し、実際のrelocated hookの実行 (:131〜142)、移転先だけが変わること、利用者file保全、source snapshot不変 (:205)を残す。quoted置換の期待値は実配布nativeのasset/参照検査へ統合。
- 生存: `TestReleaseCandidateNative` の実installer relocation、C02のUnit有一周、`TestAssignmentJourney`。
- 失う保証/現実的risk: 移転直後にGit worktreeを2個作って一周する複合scenario。40桁commitを64桁VerificationSHA256に渡す (:176) 等、現仕様とずれている。移転先hookの実実行は配布nativeだけでは直接行わないため短いcanaryは残す価値がある。
- コスト: test2workerのGo build/test、Git commit/merge、stage再準備を削減。推奨/確度: 大幅縮小/高。

<a id="c31"></a>

### C31 — RelocationLiveと合成selectorはmemory liveへ統合・旧形を削除

- 箇所: `relocation_live_integration_test.go:56` `TestRelocationCommandSelectionEvidence`、`:73` `TestRelocationLive`。
- 根拠: `relocationSelected` (:30) は `memoryLiveArgs`を使うが同helper (`memory_live_integration_test.go:40`) はokf/catだけを許可しaidlc intent switchを通さない。正例もselectorに到達しない。:138のhook引数長4は現行`--okf-binary`付6引数に合わない。
- 範囲: 旧selector test/固有selectorを削除。移転後にFIRST/SECOND-BODYの同一memory exerciseをもう1回モデルへさせる箇所は `TestMemoryMetadataLive`へ統合。移転固有保証はC30のhook canaryと配布nativeへ。
- 生存: `TestMemoryMetadataLive` (:342)、`TestReleaseCandidateNative`、C30の短い移転hook。
- 失う保証/現実的risk: 移転先でモデルが既存Intentを選んでokf操作までできる複合機能。現testの旧形はこの保証を現在満たさない。liveモデルの依存は10分/固定version/出力wire。
- 推奨/確度: 旧fixture削除・統合/高。現在の全integration成功証拠には含まれていない。

<a id="c32"></a>

### C32 — MemoryMetadataCommandをOKFの所有suiteへ移しtype直積を削る

- 箇所: `memory_metadata_integration_test.go:15` `TestMemoryMetadataCommand`。
- 範囲: aidlcの古い`memory`表記をfixtureProductでokfに変換する迂回をやめ、cmd/okfの実行検査へ移す。Design/adr/Rule全3種でcreate→show→update→stale hashを反復する部分はDesign一連とRuleのIntent非自動付与だけへ縮小。ADR一般metadataは内部okfmemoryの責任。
- 生存: 同testをOKF側へ移した1一連、配布nativeのokf create/search、内部OKF metadata/CAS tests（個別削除可否は担当外auditとの照合が必要）。
- 失う保証/現実的risk: adrだけupdate metadataを失うbugのCLI全経路確認。このCLIはtype別の操作実装を持たないので3重一連は低価値。Ruleの特有挙動は残す。
- コスト: aidlc/natural不要build、Git不要準備、同じcreate/update一連2回。推奨/確度: 移動・縮小/高。

<a id="c33"></a>

### C33 — Memory/StageSkillsの合成evidence表を縮小

- 箇所: `memory_live_integration_test.go:199` `TestMemoryMetadataCommandEvidence`（13mode）、`stage_skills_live_integration_test.go:147` `TestStageSkillsEvidence`（7mode）。
- 範囲: generated transcript→同file checkerのfield反転表を、正例・実transport無し・拒否されたreadの3代表へ縮小。StageSkills checkerのKagome/辞書版・category文字列は自然CLI metadata testへ委ね、liveは実行成功とreportに実入力由来のdiagnosticがあることを中心にする。
- 生存: `TestMemoryMetadataLive` と `TestStageSkillsLive`。形態素解析の意味とversionは自然CLI suite/配布native。
- 失う保証/現実的risk: checker自身の細部誤判定。製品のskill読取り許可はinternal app、実機のfull read/executionは2liveの役割。これらは既にintegration tagなので通常suite時間には効かない。
- 推奨/確度: 縮小/高。

<a id="c34"></a>

### C34 — Operations補助assert・setupを削減し、固有のprocess境界は残す

- 箇所: `operations_integration_test.go:157` MultiIntent、`:353` ConcurrentCAS、`:402` UnitConflicts、`:466` SaveRecovery。
- 範囲: MultiIntentは同名2Intentを毎回初期化承認まで通す準備を削りcreateのみで曖昧名を用意。UnitConflictsはduplicate claim/dependency/overlapのdomain列挙を内部 [src/internal/flow/unit_test.go:30](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/flow/unit_test.go#L30) に寄せ、実CLIの1つの占有・解放をAssignmentJourneyへ統合。runtime fileの内部JSON field確認は公開assignment出力に必要なものだけ残す。SaveRecoveryのOKF後半はOKF suiteへ移し不要binaryをbuildしない。
- 生存: `TestOperationsConcurrentCAS` は別processの同revision更新で1勝1敗・state保全という実bugを検出するため維持。`TestOperationsSaveRecovery` の実FS pre-save失敗とpostcommit部分失敗→回復は維持価値あり。単なるstub自己検証ではない。
- 失う保証/現実的risk: UnitごとのCLI parsingとdomain拒否の全組合せ、OKFをaidlc suiteから起動すること。現行CLI分離後は後者に意味がない。chmod faultはroot権限/OS依存があり、必要なOSのintegrationへ置く。
- 推奨/確度: 部分縮小/中高。CAS・実保存失敗は残す。

<a id="c35"></a>

### C35 — 現行live入口の重複と低頻度扱い

- 箇所: `boundary_live_integration_test.go:19`、`procedure_live_integration_test.go:19`、`human_approval_live_integration_test.go:22`、`assignment_live_integration_test.go:96`、`memory_live_integration_test.go:342`、`stage_skills_live_integration_test.go:207`。
- 判断: Boundaryはbegin前禁止→repair→begin→実canary、Procedureは現在手順とreopen、HumanApprovalは**後続user turn**の明示承認、Memory/StageSkillsは実full-read/実行を検査するため単純重複ではない。live回数を減らすならBoundary→Procedure/HumanApprovalへ1シナリオ統合、Memory→StageSkillsへ1シナリオ統合が候補。ただし失敗診断とモデルcontext依存が増えるので確度は中。
- AssignmentLiveはallow-parallel/deny/missing-post/save-failureの4外部実験を記録するがmanifestが`automatic_gate: inconclusive`と明記。通常suiteとは別のmanual probeへ。古いGit/worktree・HEAD型requestも整理対象。
- 生存: 実機のdistinct boundary1個ずつ、実fixtureは通常trust/明示fixture trustの既存承認経路を保持。今回は何も起動していない。
- コスト: 既にenv+integration gateで低頻度。各10〜25分上限、固定Codex 0.153.4やwire/モデルに依存。移動だけで標準suiteの大幅高速化とは言わない。

<a id="c36"></a>

### C36 — 一周内の全mutation後procedure照合と加算TDDの反復を削減

- 箇所: `flow_journey_integration_test.go:102`〜114 のprocedure helperとcall後の自動再読込み、`:197`以降のTestAdd RED/GREEN、Unit後・integration段階の同じGo test。
- 範囲: procedureを全configure/review assign/review accept後に毎回再取得せず、初期・stage遷移・reopenの境界で1回ずつ確認。加算が0を返せば失敗しa+bなら成功するというTDD演習は代表一周だけへ。他の環境/Unit variantでは実行結果の保存・current SHA整合・各Unit結果の照合を残し、Goコンパイラを何度も起動する必要性を減らす。
- 生存: `TestFlowJourney` の代表TDD一周、procedureの境界確認、内部procedure/verification resultsの製品tests、C02のUnit順次成果照合。
- 失う保証/現実的risk: 単なるconfigure直後だけprocedure出力が壊れる複合条件、Git環境別に加算sampleが失敗すること。procedureがstateのstage/hashを読む責務は境界で検査できる。**Unitの成果変更後と最終stageの新しいSHA/結果登録は省略しない**。加算sampleのテスト実行と製品の検証結果受理を区別する。
- コスト: 1一周で複数の余分なCLI subprocess、全variantでGo test反復。推奨/確度: procedure反復削減/高、TDD sample集約/中（結果実行証拠を保つ設計が必要）。

## CIの削減候補

<a id="w01"></a>

### W01 — 6target cross buildの所有をDistributionへ集約

- 箇所: [.github/workflows/ci.yml:119](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/.github/workflows/ci.yml#L119)〜162 cross-build全部、重複先 [.github/workflows/distribution.yml:121](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/.github/workflows/distribution.yml#L121)〜132。
- 範囲: CI側6job/30buildを削除候補。Distributionは同じ5CLI×6targetをtrimpath/CGO=0でbuildし、さらにarchive metadataとnativeを検証している。
- 生存: Distribution/Package と Native 3OSを同じ対象commitのrequired成功条件として維持。
- 失う保証/現実的risk: CIのGo1.26.x buildと配布の固定1.26.4 buildの差。完全同条件ではない。最低版/新版compileをQualityに必要な範囲だけ残すなら6targetを2箇所で全buildする必要はない。
- コスト: 既存PRログで約158 runner秒/6job、同branch pushでも別途実行。確度: 高（required check名を変える作業は親の承認計画で扱う）。

<a id="w02"></a>

### W02 — Quality matrixの全てを2倍にしない

- 箇所: `ci.yml:18`〜20、29〜53。
- 範囲: gofmtとmodule checkは1toolchainに集約。通常testでGo互換性を2版確認する場合もrace/integration/journey/natural binaryを両版で全て繰り返す必要は低い。raceは主要toolchain1版、filesystem/journeyは配布toolchain1版に絞る候補。
- 生存: min/stableの通常tests、主要版race、1版のintegration+native配布。
- 失う保証/現実的risk: toolchain限定raceやOS API変更による特定組合せbug。ライブラリ互換性の通常testと配布nativeは残る。stableと1.26.xが同じresolved versionなら1jobへ重複排除できるが、今回その同一性は未確認のため断定しない。
- コスト: Quality各約4分、1/2全部削除という単純見積もりはしない。確度: 中高。

<a id="w03"></a>

### W03 — smokeの注入版buildを配布nativeへ統合

- 箇所: `ci.yml:83`〜98（aidlc-injected build/version）、54〜118の重複help/version。
- 範囲: 注入版versionはDistributionで5CLI実binaryのversionをチェックするのでCIで別文字列v0.1.0をlinkするbuildを削除。default `dev/unknown` は配布版と異なるため最低1回残す。unknown argv exit2は内部CLI/main processへ寄せるなら追加build不要。
- 生存: [src/cmd/aidlc-dist/release_integration_test.go:274](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/cmd/aidlc-dist/release_integration_test.go#L274) のnative5CLI version/help、内部 `cli_test.go:152,232`。
- 失う保証/現実的risk: 固定の仮version/短commitをlinkした時だけの挙動。実releaseのlinkを検査する方が直接的。default未注入の保証は失わない設計。
- コスト: Quality2版分の余分なbuild2回。確度: 高。

<a id="w04"></a>

### W04 — pushとPRの二重workflowを対象別に整理

- 箇所: `ci.yml:3`〜5、`distribution.yml:3`〜5。
- 範囲: PRで全gate、main pushで統合確認、必要な手動配布を維持し、作業branch pushとPRの二重全suiteを避ける候補。
- 生存: PR merge refのrequired checksとmain pushの確認。
- 失う保証/現実的risk: PR未作成branchの全自動検証。push HEADとPR merge commitは異なることがあるため「完全に同じcommitの無駄」とは断定しない。parent/task運用上PRで判定するならbranch push全配布の価値は低い。
- コスト: final-04にはpush/PR両方のCI+Distribution成功が別runとして存在。確度: 中、運用選択。

<a id="w05"></a>

### W05 — tagged integrationでuntagged package testを再実行しない

- 箇所: `ci.yml:43`〜46、39〜42との重複。
- 範囲: workspace/OKFで`-tags=integration` package全体を実行すると、同packageのuntagged testも通常+raceに続き再実行する。意味のあるfilesystem統合入口のみ選ぶか専用packageへ分離。
- 生存:通常/race test群、独立filesystem integration群。
- 失う保証/現実的risk: integration用fileが追加されたbuild状態でuntagged testをもう一度実行する組合せ。buildtagでproduction実装を切替えていないなら価値は小さい。実際の各packageタグ設計・重複数は他担当と照合が必要。
- 確度: 中高。`cmd/aidlc`は既にrun regexを限定しておりここには該当しない。

<a id="w06"></a>

### W06 — list入口確認とdraft前再検証は自動削除しない

- 箇所: `distribution.yml:98,191,246` のtest名存在check、241〜249候補再検証。
- 判断: go testの`-run`は該当testゼロでも成功し得るため、入口名存在checkは単なるcoverage維持ではない。draft直前のcandidate/remote tag再照合は公開対象取り違えを防ぐ別境界。削除推奨しない。
- 小改善: 同jobでlist時のcompileと実行時compileは通常Go cacheが効く。計測なしで重い再buildと断定しない。


## 確認した全ファイル

以下の216ファイルの本文を確認しました。候補のないfileやhelperだけのfileも含みます。各領域の「残す境界」は、削除を勧めない具体的な不具合検出です。production全行を別に監査したという意味ではありません。

### P領域（52ファイル）

- [src/bootstrap/bootstrap_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/bootstrap/bootstrap_test.go)
- [src/bootstrap/candidate_integration_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/bootstrap/candidate_integration_test.go)
- [src/bootstrap/fixture_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/bootstrap/fixture_test.go)
- [src/bootstrap/powershell_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/bootstrap/powershell_test.go)
- [src/cmd/aidlc-dist/archive_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/cmd/aidlc-dist/archive_test.go)
- [src/cmd/aidlc-dist/bundle_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/cmd/aidlc-dist/bundle_test.go)
- [src/cmd/aidlc-dist/candidate_license_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/cmd/aidlc-dist/candidate_license_test.go)
- [src/cmd/aidlc-dist/distribution_integration_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/cmd/aidlc-dist/distribution_integration_test.go)
- [src/cmd/aidlc-dist/main_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/cmd/aidlc-dist/main_test.go)
- [src/cmd/aidlc-dist/manifest_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/cmd/aidlc-dist/manifest_test.go)
- [src/cmd/aidlc-dist/release_integration_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/cmd/aidlc-dist/release_integration_test.go)
- [src/cmd/aidlc-dist/release_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/cmd/aidlc-dist/release_test.go)
- [src/cmd/aidlc-install/main_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/cmd/aidlc-install/main_test.go)
- [src/cmd/natural-japanese-go/integration_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/cmd/natural-japanese-go/integration_test.go)
- [src/cmd/natural-japanese-go/main_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/cmd/natural-japanese-go/main_test.go)
- [src/cmd/natural-japanese-go/main_unix_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/cmd/natural-japanese-go/main_unix_test.go)
- [src/cmd/okf/main_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/cmd/okf/main_test.go)
- [src/core/content_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/core/content_test.go)
- [src/core/workflow/content_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/core/workflow/content_test.go)
- [src/harness/codex/content_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/harness/codex/content_test.go)
- [src/harness/codex/manifest_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/harness/codex/manifest_test.go)
- [src/harness/codex/split_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/harness/codex/split_test.go)
- [src/harness/manifest_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/harness/manifest_test.go)
- [src/internal/install/assignment_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/install/assignment_test.go)
- [src/internal/install/bootstrap_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/install/bootstrap_test.go)
- [src/internal/install/codekb_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/install/codekb_test.go)
- [src/internal/install/documents_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/install/documents_test.go)
- [src/internal/install/execution_plan_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/install/execution_plan_test.go)
- [src/internal/install/flow_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/install/flow_test.go)
- [src/internal/install/git_independent_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/install/git_independent_test.go)
- [src/internal/install/install_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/install/install_test.go)
- [src/internal/install/manifest_parity_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/install/manifest_parity_test.go)
- [src/internal/install/product_agents_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/install/product_agents_test.go)
- [src/internal/install/release_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/install/release_test.go)
- [src/internal/install/relocate_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/install/relocate_test.go)
- [src/internal/install/rule_skill_separation_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/install/rule_skill_separation_test.go)
- [src/internal/install/split_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/install/split_test.go)
- [src/internal/install/stage_planner_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/install/stage_planner_test.go)
- [src/internal/install/stage_skills_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/install/stage_skills_test.go)
- [src/internal/install/workflow_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/install/workflow_test.go)
- [src/internal/naturaljapanese/analyzer_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/naturaljapanese/analyzer_test.go)
- [src/internal/naturaljapanese/baseline_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/naturaljapanese/baseline_test.go)
- [src/internal/naturaljapanese/fixtures_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/naturaljapanese/fixtures_test.go)
- [src/internal/naturaljapanese/lexical_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/naturaljapanese/lexical_test.go)
- [src/internal/naturaljapanese/morph_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/naturaljapanese/morph_test.go)
- [src/internal/naturaljapanese/rhythm_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/naturaljapanese/rhythm_test.go)
- [src/internal/naturaljapanese/rules_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/naturaljapanese/rules_test.go)
- [src/internal/naturaljapanese/structure_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/naturaljapanese/structure_test.go)
- [src/internal/naturaljapanese/surface_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/naturaljapanese/surface_test.go)
- [src/internal/naturaljapanese/text_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/naturaljapanese/text_test.go)
- [src/internal/release/bundle_archive_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/release/bundle_archive_test.go)
- [src/internal/release/bundle_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/release/bundle_test.go)

### A領域（75ファイル）

- [src/internal/app/agent_hook_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/app/agent_hook_test.go)
- [src/internal/app/assignment_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/app/assignment_test.go)
- [src/internal/app/boundary_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/app/boundary_test.go)
- [src/internal/app/child_hook_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/app/child_hook_test.go)
- [src/internal/app/child_report_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/app/child_report_test.go)
- [src/internal/app/codekb_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/app/codekb_test.go)
- [src/internal/app/documents_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/app/documents_test.go)
- [src/internal/app/execution_plan_review_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/app/execution_plan_review_test.go)
- [src/internal/app/execution_plan_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/app/execution_plan_test.go)
- [src/internal/app/fixture_execution_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/app/fixture_execution_test.go)
- [src/internal/app/flow_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/app/flow_test.go)
- [src/internal/app/hook_reliability_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/app/hook_reliability_test.go)
- [src/internal/app/hook_split_cli_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/app/hook_split_cli_test.go)
- [src/internal/app/memory_body_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/app/memory_body_test.go)
- [src/internal/app/memory_help_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/app/memory_help_test.go)
- [src/internal/app/procedure_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/app/procedure_test.go)
- [src/internal/app/relocation_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/app/relocation_test.go)
- [src/internal/app/rule_skill_separation_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/app/rule_skill_separation_test.go)
- [src/internal/app/session_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/app/session_test.go)
- [src/internal/app/stage_skills_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/app/stage_skills_test.go)
- [src/internal/app/verification_cli_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/app/verification_cli_test.go)
- [src/internal/app/verification_gates_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/app/verification_gates_test.go)
- [src/internal/app/work_log_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/app/work_log_test.go)
- [src/internal/buildinfo/buildinfo_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/buildinfo/buildinfo_test.go)
- [src/internal/cli/assignment_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/cli/assignment_test.go)
- [src/internal/cli/boundary_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/cli/boundary_test.go)
- [src/internal/cli/check_help_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/cli/check_help_test.go)
- [src/internal/cli/cli_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/cli/cli_test.go)
- [src/internal/cli/command_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/cli/command_test.go)
- [src/internal/cli/configure_help_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/cli/configure_help_test.go)
- [src/internal/cli/documents_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/cli/documents_test.go)
- [src/internal/cli/execution_plan_review_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/cli/execution_plan_review_test.go)
- [src/internal/cli/execution_plan_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/cli/execution_plan_test.go)
- [src/internal/cli/five_cli_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/cli/five_cli_test.go)
- [src/internal/cli/flow_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/cli/flow_test.go)
- [src/internal/cli/git_independent_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/cli/git_independent_test.go)
- [src/internal/cli/help_codekb_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/cli/help_codekb_test.go)
- [src/internal/cli/help_work_log_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/cli/help_work_log_test.go)
- [src/internal/cli/memory_help_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/cli/memory_help_test.go)
- [src/internal/cli/procedure_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/cli/procedure_test.go)
- [src/internal/cli/project_root_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/cli/project_root_test.go)
- [src/internal/cli/relocation_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/cli/relocation_test.go)
- [src/internal/cli/rule_skill_separation_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/cli/rule_skill_separation_test.go)
- [src/internal/cli/space_list_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/cli/space_list_test.go)
- [src/internal/cli/space_switch_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/cli/space_switch_test.go)
- [src/internal/cli/space_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/cli/space_test.go)
- [src/internal/cli/verification_cli_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/cli/verification_cli_test.go)
- [src/internal/okf/frontmatter_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/okf/frontmatter_test.go)
- [src/internal/okf/scan_integration_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/okf/scan_integration_test.go)
- [src/internal/okf/scan_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/okf/scan_test.go)
- [src/internal/okf/search_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/okf/search_test.go)
- [src/internal/okfapp/command_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/okfapp/command_test.go)
- [src/internal/okfcli/command_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/okfcli/command_test.go)
- [src/internal/okfcli/metadata_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/okfcli/metadata_test.go)
- [src/internal/okfmemory/document_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/okfmemory/document_test.go)
- [src/internal/okfmemory/flow_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/okfmemory/flow_test.go)
- [src/internal/okfmemory/metadata_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/okfmemory/metadata_test.go)
- [src/internal/okfmemory/selector_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/okfmemory/selector_test.go)
- [src/internal/okfmemory/selector_unix_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/okfmemory/selector_unix_test.go)
- [src/internal/pathnorm/unicode_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/pathnorm/unicode_test.go)
- [src/internal/projectroot/root_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/projectroot/root_test.go)
- [src/internal/workspace/cursor_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/workspace/cursor_test.go)
- [src/internal/workspace/flow_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/workspace/flow_test.go)
- [src/internal/workspace/root_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/workspace/root_test.go)
- [src/internal/workspace/rule_skill_separation_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/workspace/rule_skill_separation_test.go)
- [src/internal/workspace/space_create_integration_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/workspace/space_create_integration_test.go)
- [src/internal/workspace/space_create_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/workspace/space_create_test.go)
- [src/internal/workspace/space_integration_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/workspace/space_integration_test.go)
- [src/internal/workspace/space_okf_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/workspace/space_okf_test.go)
- [src/internal/workspace/space_read_integration_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/workspace/space_read_integration_test.go)
- [src/internal/workspace/space_read_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/workspace/space_read_test.go)
- [src/internal/workspace/space_switch_integration_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/workspace/space_switch_integration_test.go)
- [src/internal/workspace/space_switch_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/workspace/space_switch_test.go)
- [src/internal/workspace/space_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/workspace/space_test.go)
- [src/internal/workspace/workspace_lock_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/workspace/workspace_lock_test.go)

### F領域（48ファイル）

- [src/internal/assignment/directory_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/assignment/directory_test.go)
- [src/internal/assignment/dispatch_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/assignment/dispatch_test.go)
- [src/internal/assignment/recovery_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/assignment/recovery_test.go)
- [src/internal/assignment/reservation_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/assignment/reservation_test.go)
- [src/internal/assignment/store_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/assignment/store_test.go)
- [src/internal/flow/assignment_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/flow/assignment_test.go)
- [src/internal/flow/boundary_results_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/flow/boundary_results_test.go)
- [src/internal/flow/boundary_review_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/flow/boundary_review_test.go)
- [src/internal/flow/boundary_sensor_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/flow/boundary_sensor_test.go)
- [src/internal/flow/boundary_snapshot_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/flow/boundary_snapshot_test.go)
- [src/internal/flow/boundary_store_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/flow/boundary_store_test.go)
- [src/internal/flow/boundary_transition_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/flow/boundary_transition_test.go)
- [src/internal/flow/boundary_unix_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/flow/boundary_unix_test.go)
- [src/internal/flow/codekb_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/flow/codekb_test.go)
- [src/internal/flow/content_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/flow/content_test.go)
- [src/internal/flow/directory_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/flow/directory_test.go)
- [src/internal/flow/documents_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/flow/documents_test.go)
- [src/internal/flow/documents_unix_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/flow/documents_unix_test.go)
- [src/internal/flow/execution_approval_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/flow/execution_approval_test.go)
- [src/internal/flow/execution_history_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/flow/execution_history_test.go)
- [src/internal/flow/execution_plan_review_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/flow/execution_plan_review_test.go)
- [src/internal/flow/execution_plan_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/flow/execution_plan_test.go)
- [src/internal/flow/fixture_execution_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/flow/fixture_execution_test.go)
- [src/internal/flow/graph_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/flow/graph_test.go)
- [src/internal/flow/history_review_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/flow/history_review_test.go)
- [src/internal/flow/lowercase_adr_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/flow/lowercase_adr_test.go)
- [src/internal/flow/procedure_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/flow/procedure_test.go)
- [src/internal/flow/reassign_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/flow/reassign_test.go)
- [src/internal/flow/reopen_log_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/flow/reopen_log_test.go)
- [src/internal/flow/review_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/flow/review_test.go)
- [src/internal/flow/rule_skill_separation_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/flow/rule_skill_separation_test.go)
- [src/internal/flow/selected_documents_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/flow/selected_documents_test.go)
- [src/internal/flow/sensor_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/flow/sensor_test.go)
- [src/internal/flow/stage_planner_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/flow/stage_planner_test.go)
- [src/internal/flow/store_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/flow/store_test.go)
- [src/internal/flow/transition_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/flow/transition_test.go)
- [src/internal/flow/unit_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/flow/unit_test.go)
- [src/internal/flow/unit_without_git_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/flow/unit_without_git_test.go)
- [src/internal/flow/verification_cli_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/flow/verification_cli_test.go)
- [src/internal/flow/verification_gates_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/flow/verification_gates_test.go)
- [src/internal/flow/verification_results_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/flow/verification_results_test.go)
- [src/internal/flow/verification_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/flow/verification_test.go)
- [src/internal/flow/work_log_okf_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/flow/work_log_okf_test.go)
- [src/internal/workflow/declaration_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/workflow/declaration_test.go)
- [src/internal/workflow/definition_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/workflow/definition_test.go)
- [src/internal/workflow/execution_plan_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/workflow/execution_plan_test.go)
- [src/internal/workflow/rule_skill_separation_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/workflow/rule_skill_separation_test.go)
- [src/internal/workflow/stage_planner_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/internal/workflow/stage_planner_test.go)

### C領域（41ファイル）

- [src/cmd/aidlc/agent_hook_probe_live_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/cmd/aidlc/agent_hook_probe_live_test.go)
- [src/cmd/aidlc/agent_hook_probe_protocol_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/cmd/aidlc/agent_hook_probe_protocol_test.go)
- [src/cmd/aidlc/agent_hook_probe_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/cmd/aidlc/agent_hook_probe_test.go)
- [src/cmd/aidlc/assignment_integration_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/cmd/aidlc/assignment_integration_test.go)
- [src/cmd/aidlc/assignment_live_integration_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/cmd/aidlc/assignment_live_integration_test.go)
- [src/cmd/aidlc/assignment_process_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/cmd/aidlc/assignment_process_test.go)
- [src/cmd/aidlc/boundary_evidence_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/cmd/aidlc/boundary_evidence_test.go)
- [src/cmd/aidlc/boundary_fixture_integration_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/cmd/aidlc/boundary_fixture_integration_test.go)
- [src/cmd/aidlc/boundary_live_integration_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/cmd/aidlc/boundary_live_integration_test.go)
- [src/cmd/aidlc/command_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/cmd/aidlc/command_test.go)
- [src/cmd/aidlc/configure_help_integration_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/cmd/aidlc/configure_help_integration_test.go)
- [src/cmd/aidlc/documents_journey_integration_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/cmd/aidlc/documents_journey_integration_test.go)
- [src/cmd/aidlc/execution_plan_evidence_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/cmd/aidlc/execution_plan_evidence_test.go)
- [src/cmd/aidlc/flow_command_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/cmd/aidlc/flow_command_test.go)
- [src/cmd/aidlc/flow_command_unix_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/cmd/aidlc/flow_command_unix_test.go)
- [src/cmd/aidlc/flow_evidence_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/cmd/aidlc/flow_evidence_test.go)
- [src/cmd/aidlc/flow_journey_integration_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/cmd/aidlc/flow_journey_integration_test.go)
- [src/cmd/aidlc/flow_live_integration_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/cmd/aidlc/flow_live_integration_test.go)
- [src/cmd/aidlc/git_independent_fixture_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/cmd/aidlc/git_independent_fixture_test.go)
- [src/cmd/aidlc/git_independent_helpers_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/cmd/aidlc/git_independent_helpers_test.go)
- [src/cmd/aidlc/git_independent_journey_integration_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/cmd/aidlc/git_independent_journey_integration_test.go)
- [src/cmd/aidlc/hook_probe_live_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/cmd/aidlc/hook_probe_live_test.go)
- [src/cmd/aidlc/hook_probe_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/cmd/aidlc/hook_probe_test.go)
- [src/cmd/aidlc/hook_reliability_opaque_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/cmd/aidlc/hook_reliability_opaque_test.go)
- [src/cmd/aidlc/hook_reliability_probe_integration_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/cmd/aidlc/hook_reliability_probe_integration_test.go)
- [src/cmd/aidlc/hook_reliability_probe_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/cmd/aidlc/hook_reliability_probe_test.go)
- [src/cmd/aidlc/human_approval_live_integration_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/cmd/aidlc/human_approval_live_integration_test.go)
- [src/cmd/aidlc/main_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/cmd/aidlc/main_test.go)
- [src/cmd/aidlc/main_unix_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/cmd/aidlc/main_unix_test.go)
- [src/cmd/aidlc/memory_live_integration_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/cmd/aidlc/memory_live_integration_test.go)
- [src/cmd/aidlc/memory_metadata_integration_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/cmd/aidlc/memory_metadata_integration_test.go)
- [src/cmd/aidlc/observer_command_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/cmd/aidlc/observer_command_test.go)
- [src/cmd/aidlc/operations_integration_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/cmd/aidlc/operations_integration_test.go)
- [src/cmd/aidlc/procedure_evidence_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/cmd/aidlc/procedure_evidence_test.go)
- [src/cmd/aidlc/procedure_live_integration_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/cmd/aidlc/procedure_live_integration_test.go)
- [src/cmd/aidlc/project_root_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/cmd/aidlc/project_root_test.go)
- [src/cmd/aidlc/relocation_integration_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/cmd/aidlc/relocation_integration_test.go)
- [src/cmd/aidlc/relocation_live_integration_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/cmd/aidlc/relocation_live_integration_test.go)
- [src/cmd/aidlc/rule_skill_separation_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/cmd/aidlc/rule_skill_separation_test.go)
- [src/cmd/aidlc/split_fixture_integration_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/cmd/aidlc/split_fixture_integration_test.go)
- [src/cmd/aidlc/stage_skills_live_integration_test.go](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/src/cmd/aidlc/stage_skills_live_integration_test.go)

追加確認: [.github/workflows/ci.yml](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/.github/workflows/ci.yml)、[.github/workflows/distribution.yml](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/.github/workflows/distribution.yml)、[skillの説明用CLI test](https://github.com/sori883/ai-dd/blob/adc4682ca265c9bda4434a94d9d99cff1f4b9394/.agents/skills/golang-cli/assets/examples/cli_test.go)。

## 実施するときの順番

1. 同一Test再呼出、同一入力、test helper・到達不能assertなど、現行保証が増えないものから削る。名前で選択するCI/検証手順があれば生存Test名へ直す。
2. Gitや重複ビルド・圧縮・解凍を減らす。mutableなSpace・Intent・registryは共有せず、バイナリ等の不変な準備だけ共有する。
3. 多層の表を、判定本体の必要ケースと各入口の短い接続試験へ集約する。保存失敗・競合・承認・実FSの固有assertを移し終わってから元のtestを消す。
4. 古い実験host・診断checkerを整理する。現在も判断の根拠に使う観測記録については、成功・別identity・未完了の代表を残す。単にtagを付けて実行しなくなる変更と、重複を消す変更を区別する。
5. CIのbuild所有者・Go版・push/PRの役割を決める。必要な合否gateを生存jobへ移してから重複jobを削る。

削減後の確認は、残す各保証が実際に実行されたことを対象test名で確認する。カバレッジの数字を埋めるための代替テストは追加しない。今回は一覧作成までで、上記の削除・統合は未実施。
