# 単一CLIをGitHub Releasesへ渡す配布機構

## 背景・目的・実装許可

AI-DLCは、実行ファイル `aidlc` にskill・agent・hook・工程定義を同梱する。利用者はOS・CPUに合う版を取得し、`aidlc install codex --project-dir ROOT` でプロジェクトへ資材を配置する。CodexやClaude Code本体のインストールを代行する機能ではない。Claude用の同じCLI引数はIssue #185で未完了・保留中であり、本計画の基準mainではCodexだけが利用できる。

現在は開発用 `aidlc-dist` が6対象の圧縮ファイルと照合情報を生成するが、GitHub Releasesへの接続はない。また、各OSの既存配布テストは、その場で別の実行ファイルをbuildするため、公開候補そのものの実行を確認していない。利用者に渡す候補をそのまま検証し、確認済みのファイルをReleaseの下書きへ渡せるようにする。

ユーザーは2026-09-13に単一CLI・同梱資材・GitHub Releases配布へ同意し、配布処理を整備する説明に「はい、お願いします」と直接依頼した。[承認範囲](../ram/decisions/2026-09-13-github-release-pipeline-approved.md)に従い、実装・検証・Issue・PRを進める。旧33 Stageロードマップは許可の根拠に使わない。正式バージョン、Go製品のライセンス、初回公開物は未確定であり、今回の実装中に実tagやReleaseを作成・公開しない。

基準はPR #184をmergeしたmain `5970c99a057077b2edd62726fcd8c4f1245a2c7d`。専用作業場所は `/Users/const/sori883/ai-dd-release`、branchは `codex/github-release-distribution`。元checkoutとClaude対応の未commit変更は編集しない。

## 利用者と配布担当が得る結果

利用者が導入する製品CLIは一つのままで、実行ファイルの版と同梱資材の版が一致する。開発用の別インストーラーやGitからの追加取得は不要である。配布担当はGitHub ActionsのDistributionをmainから手動実行し、既存tagを指定して候補を検証できる。`create_draft` を明示して有効にしたときだけ、検証後にReleaseの下書きを作成する。一般公開は完成内容を確認した後の操作とする。

## 実装する契約

1. `.github/workflows/distribution.yml` のpush・PR検証を維持し、`workflow_dispatch` に必須文字列 `tag` と既定falseの真偽値 `create_draft` を追加する。別workflowへ同じbuild処理を複製しない。
2. 最初のjobで対象commitと版を確定し、後続の全jobへ渡す。push・PRは実行対象のcheckout SHAと `dev-<SHA先頭12文字>`。手動実行はmainからの起動に限定し、既存remote tagが指すcommitとmainへの到達可能性を検査する。tag名は既存packagerと同じ安全な版文字列（先頭英数字、英数字・dot・underscore・hyphen、128文字以内、連続dotなし）に限定する。新しいSemVer方針は決めない。
3. packageとnativeは同じ確定SHAをcheckoutする。既存のGo 1.26、CGO無効、trimpath、版・commit埋込み、6 OS/CPU対象、archive名、manifest schema 1を維持する。日本語補助CLIの既存build・梱包検証も維持する。
4. aidlcの6archive、manifest.json、SHA256SUMSの計8ファイルを、同じActions run内のartifactへ1日だけ保存する。これは検証間のファイル受渡しであり、正式版の公開ではないが、Actionsから取得可能な外部保存である。出力directory以外のsource・runtime・ログをuploadしない。artifact名はrun attemptを区別し、上書きしない。後続jobはpackageが返すartifact IDを明示して同じ内容を取得し、downloadのdigest不一致はerrorにする。
5. metadata検査には候補directoryと期待する版・commit・Go版を渡す。既存archive照合に加え、正確な6target/8file、期待値との一致を検査する。native検査は取得したarchiveから実行中OS/CPUに合うものを一意に選ぶ。候補を再buildせず、一時directoryへ展開したbinaryの版・commit、help、新規 `install codex`、同梱原稿と配置の一致、再installの拒否と既存file保全を確認する。
6. 新検査の入口は `TestReleaseCandidateMetadata` と `TestReleaseCandidateNative`。入力は `AIDLC_DIST_DIR`、`AIDLC_RELEASE_VERSION`、`AIDLC_RELEASE_COMMIT`、`AIDLC_RELEASE_GO_VERSION`。workflowで明示的に渡す。検査実行前に `go test -tags=integration -list` で名前の存在を確認し、古いtagで「no tests to run」となる成功を拒否する。候補指定時の不足入力は失敗とし、skipを成功証拠にしない。従来の環境入力なしのintegration実行を壊さないため、候補入力が全てない場合は入口でskipできる。
7. 下書きjobは手動実行かつ `create_draft=true`、packageと全native job成功の場合だけ起動する。このjobだけ `contents: write`、他jobは `contents: read`。最小限のGITHUB_TOKENを使い、個人tokenや追加secretは要求しない。
8. 下書きjobは候補を再照合し、remote tagのcommitがbuild対象から変わっていないこと、同じtagのReleaseが下書きを含め存在しないことを確認する。Release一覧取得の失敗を「存在しない」と扱わない。`gh release create --verify-tag --draft` に照合済みの8ファイルを渡す。tag作成、既存Releaseの自動再利用、`--clobber`、自動publishは行わない。Release説明は日本語とし、版・commit、対応環境、照合・導入手順と未公開の確認項目を記載する。

通常の既存Journeyは、手動で実行ファイルを切り替える場合の別検査として維持する。利用プロジェクトのcore、harness、配置内容、state、Knowledge、Rule、runtimeには変更を加えない。

## 変更ファイルと単独writer

[Issue #186](https://github.com/sori883/ai-dd/issues/186)、1 Issue／PR、`work_unit_id=github-release-distribution`。実装時はGo実装担当だけが次の全対象を編集し、親は同じ作業treeを同時編集しない。

| 対象 | 内容 |
| --- | --- |
| `.github/workflows/distribution.yml` | 対象SHA確定、手動入力、同じ候補の受渡し・検査、条件付き下書き作成 |
| `src/cmd/aidlc-dist/release_integration_test.go`（新規） | metadataとnative target選択の検査、実候補を実行するfixture |
| `src/cmd/aidlc-dist/distribution_integration_test.go` | 必要な場合のみ既存照合・展開helperを最小限再利用 |
| `README.md`、`src/docs/user-guide.md` | 取得・配置の流れ、未公開の現在地を正確に案内 |
| `docs/distribution.md`、必要なら `docs/development.md` | 配布担当の手動入力、失敗復旧、検証と公開の区別 |
| 本計画、承認RAM、`docs/ram/README.md` | 許可・作業単位の証拠・索引の整合 |

製品Go、新規開発CLI、Go module、既存archive保存形式は変更しない。新しい第三者ActionはGitHub公式upload/downloadの固定SHAだけで、既存checkout/setup-goは維持する。ライセンスの決定・追記は別の公開前判断とする。

## 順序付き検証とTDD

`verification_mode=loop`。新たなGo処理はtest用helper内に限定する。検証処理の誤りを検出する小fixtureを先に書き、必要なら型・署名・空返値だけのscaffoldからrunnableなREDを確認する。製品Go変更のない文書・YAMLへ人工的なREDを要求しない。

| 順序 | 検証する振る舞い | loopの正確なcommand |
| --- | --- | --- |
| 1 | 期待版・commit・Go版の欠落と不一致、対象集合の不一致を拒否するmetadata helper | `go test -tags=integration -count=1 ./src/cmd/aidlc-dist -run '^TestReleaseCandidateMetadataValidation$'` |
| 2 | native targetの一意選択、欠落・重複・不一致を拒否 | `go test -tags=integration -count=1 ./src/cmd/aidlc-dist -run '^TestReleaseCandidateNativeSelection$'` |
| 3 | 候補を照合・展開・起動するintegration入口を接続 | 前記2command、`go test -tags=integration -list '^TestReleaseCandidate(Metadata|Native)$' ./src/cmd/aidlc-dist` |
| 4 | workflowと手順を接続 | `git diff --check`、既存RubyのYAML読込みと各run blockの `bash -n`。追加toolの導入はしない |

ALREADY_GREENを人工REDへ変えない。実binaryのbuild・起動はloopで実行せずfinalへ集約する。末尾に2つのtargeted command、`go test -count=1 ./src/cmd/aidlc-dist`、変更Goのgofmtと差分確認を行う。親は作業単位末尾に対象test群と全差分を一度確認する。

独立担当の `verification_mode=review` では、対象SHA・artifact IDの一致、検査が実際に走る条件、入力のshell展開、PRから書込みjobに到達しないこと、tag変化・API失敗・既存Draft・部分失敗を重点確認する。対象の再現診断だけを行い、全体検証は代行しない。

blocking findingの修正とreviewが完了して差分が安定した後、親がread-onlyの `verification_mode=final` を一度実施する。

- `go test -shuffle=on ./...`、`go test -race -shuffle=on ./...`、`go vet ./...`、`go mod tidy -diff`、`gofmt -l src`、`git diff --check`。
- Go 1.26で6targetのaidlcを候補directoryへbuild・梱包。期待値とdirectoryを渡し、`TestDistributionArchives`、`TestReleaseCandidateMetadata`、local環境の `TestReleaseCandidateNative` を実行する。
- `.github/workflows/distribution.yml` のYAMLとshell構文を確認する。実tagを作らず、必要な対象選択・拒否分岐は隔離Git fixtureや代替gh応答で確認する。実GitHub書込みを成功扱いしない。
- workspace/OKFのintegration、製品FlowJourney/GitIndependentJourney、既存の両製品配布Journey、6target cross-buildと3OSでの実候補検査は対象PRの既存・拡張CIに委ね、該当する全checksの成功を確認する。古いheadの結果は使わない。

3OSで実行しても6CPU構成全ての実行確認にはならない。通常CodexのhookやClaudeの実機試験の成功とは区別する。Draft作成の実書込みは初回操作で確認する残件として明記する。

## 失敗・復旧・公開前確認

候補生成・照合・native実行が失敗したら下書きへ進まない。別の出力先と新しいrunで再検証する。tagが変わった場合はそのrunを停止する。既存Releaseを検出した場合や下書き作成・添付が途中失敗した場合は、残ったものを確認する。自動で下書きやtagを消さず、既存添付を上書きして成功にしない。機構の不具合はworkflow/testの修正またはrevertで戻せ、利用プロジェクトのデータを初期化する必要はない。

初回操作前に、版名・対象commit、Go製品ライセンスと表示方法、初回成果物の内容、実際に作成する下書きを具体化する。一般公開は内容確認後に行う。Claudeの検証再開はこの作業に含めない。

## 本家と外部仕様の根拠

固定本家2.6.123の[配布分析](../aidlc-analysis/02-build-config-dependencies.md)と既存GoのManifestによる配置を再利用する。Go単一binary化は既承認の差分であり、新しい工程や配置ルールの差分は採用しない。本家の最新公開workflow全体との一致は未確認である。

2026-09-13にContext7と一次資料で確認した根拠:

- [gh release create](https://cli.github.com/manual/gh_release_create): `--verify-tag` はtagの存在確認、`--draft` は下書き保存。存在確認だけでは対象commitの一致を保証しない。
- [GitHub CLI v2.94.0のcreate処理](https://github.com/cli/cli/blob/v2.94.0/pkg/cmd/release/create/create.go): Draft指定では既存Releaseの事前拒否を別途設ける必要があり、添付失敗で部分Draftが残り得る。
- [upload-artifact v7.0.1](https://github.com/actions/upload-artifact/tree/v7.0.1): 固定SHA `043fb46d1a93c77aae656e7c1c64a875d1fc6a0a`。IDによる受渡しと1日保持を使用する。
- [download-artifact v8.0.1](https://github.com/actions/download-artifact/tree/v8.0.1): 固定SHA `3e5f45b2cfb9172054b4087a40e8e0b5a5461e7c`。同じrunのID指定と `digest-mismatch: error` を使用する。

## 実装時の具体化

work unitの順序1・2は`release_integration_test.go`内のmetadata helperとnative選択helperを小fixtureでtest-first実装する。順序3は既存`verifyDistribution`と`distributionPayload`を再利用し、配置先の全Paths/bytesを同じSHAの`codex.Distribution`による資材と照合する。製品Goや既存Journeyは変更しない。

同tagの下書き作成jobをconcurrencyで直列化する。既存Releaseの確認は認証済み`gh api --paginate`で全ページのtagとIDを取得し、下書きも含めて比較する。API失敗を空一覧へ変えない。下書きの添付失敗後に残ったReleaseは次回の既存検査で拒否し、自動再利用しない。これは既存Releaseを上書きしない契約の具体化である。

両helperのnegative fixtureもpackage jobで実行する。integrationタグを使うため、通常のタグなしGo testだけに検証を委ねない。local loopは小fixtureと入口列挙・構文検査まで、実candidate build/runとremote拒否分岐の隔離fixtureは親のfinalに残す。


### Review修復: Gitを必要としない候補導入

`github-release-distribution-review-repair`は、実候補の導入先をGit repositoryから通常フォルダへ直す範囲内修復である。`TestReleaseCandidateProjectDirectory`を先に追加し、既存のGit初期化helperを呼ぶ状態で`.git`の不存在assertionがRED（exit 1）となった。その後、mkdirと実path解決だけのhelperへ変更してGREEN（exit 0）を確認した。正確なcommandは`go test -tags=integration -count=1 ./src/cmd/aidlc-dist -run '^TestReleaseCandidateProjectDirectory$'`。

実候補のversion/help/install/reinstallは空directoryをPATHに指定して絶対pathから起動し、配置後も`.git`がないことを確認する。通常OS環境変数を保持し、既存Journeyは変更しない。新しい小fixtureはpackage CIでも実行し、実候補の起動はfinalと3OS CIに委ねる。


### Shell guardの範囲内修復

`github-release-distribution-shell-guard-repair`では、親のfinalで見つかった拒否漏れを修復する。macOSのBash 3.2.57では、`set -e`下の単独`[[ ... ]]`の不一致後も処理が続き、隔離Gitとstub ghでworkflow本文を実行すると、移動済みtagが下書き作成stubまで到達した。修復前の`ruby /tmp/ai-dd-release-guard-fixtures.rb .github/workflows/distribution.yml`は`tag_moved`でexit 1となった。

package/native/recheckのHEAD一致、draftのtag名・版・remote SHA・8file数の必須guardに明示的な`exit 1`を追加した。同じ18ケースは修復後exit 0となり、`tag_moved`の作成呼出しは0回だった。条件内のif/while、正常系、権限、版仕様は変更しない。Ubuntu上の実Draft作成は未実測で、今回の証拠はmacOS Bash 3と隔離stubの拒否確認である。
