# Gitなしで工程を進め、検証対象一式のSHAで成果を確認する計画

日付: 2026-09-11。基準HEAD: `9360348cf0512d168b8604c85c24da322f186186`。
状態: CLI・保存形式・Unit確認方法を含む具体計画全体を直接承認済み。Issue #171で実装する。承認根拠は[実装承認記録](../ram/decisions/2026-09-11-git-independent-implementation-approved.md)。

## 目的と利用者が得る結果

現在のAI-DLCは、工程を完了する検査や担当割当でGitを参照する。コードの版はコミット番号で記録し、workerとreviewerには管理元と別のGit作業フォルダを要求する。そのため、普通のフォルダにコードを置いた利用や、同じ場所での順次実装ができない。

この変更では、AI-DLCの利用にGitを必須としない。メインAIは共通の `project/` を開き、`project/aidlc/` で工程と知識を管理する。コードは `project/src/` でも、複数のアプリフォルダでもよい。利用者がGitで全体やアプリ別に履歴を管理することは任意とする。

コードが検証時から変わっていないかは、関連するファイル一式から計算したSHA-256で確認する。SHA-256は内容の同一性を比較する値である。ファイルごとのSHAを利用者が登録する作業は設けない。AIが適切な対象範囲を選び、CLIが列挙と計算を行う。

```text
project/
├── .codex/                 agent・hook
├── .agents/skills/         AI-DLCの進め方・CLI操作
├── aidlc/                  workflow・Space・Intent・Knowledge・ローカル管理
├── app1/                   アプリを分ける場合
├── app2/
├── shared/                 共通部品がある場合
└── src/                    コードを直接置く場合
```

同じ作業場所の実装担当は一人ずつとする。独立レビューは別エージェント・別会話で行い、同じフォルダを使えるようにする。担当の起動は引き続きメインAIの標準ツールで行う。

## 承認の根拠と変更する契約

[ユーザーの指定](../ram/decisions/2026-09-11-git-independent-content-sha-approved.md)は、Git不要化と、適切な範囲のファイルをまとめたSHA方式である。SHA方式や、全体Git・アプリ別Gitのどちらを使うかは再質問しない。旧33 Stageロードマップや、以前のshared-worktree-hook計画を実装許可の根拠にしない。

後続の[後方互換不要の指定](../ram/decisions/2026-09-11-git-independent-no-backward-compatibility.md)により、旧記録の互換読込み・移行・新旧二重運用・互換テストは今回の実装範囲から外す。既存ファイルの削除や利用環境の初期化を伴う指示ではない。

この計画で具体化する変更は、コミットを含む入力・保存形式の置換、同じ作業場所の割当、Unit成果の内容照合である。Go単一バイナリ、標準ライブラリ、OKF、ステージのstate、計画・成果承認、保存復旧の基盤を利用する。外部Go moduleは追加しない。

新しいGit代替の履歴管理、snapshot台帳、receipt台帳、全操作auditは作らない。ファイルの内容はその場で読み、集合SHAを既存の設定・結果・状態へ結び付ける。SHAは変更履歴や実行者の本人確認を証明するものではない。

## 検証対象とSHAの単位

Intentの設定に `verification_paths` を追加する。これは検証へ影響するコード・設定・テスト・共通部品のパス集合であり、担当者が変更してよい範囲を示す既存の `scope` とは区別する。

```json
{
  "scope": ["app1/src"],
  "verification_paths": [
    "app1/src",
    "app1/tests",
    "app1/package.json",
    "app1/package-lock.json",
    "shared"
  ]
}
```

この例では、実装の担当範囲は `app1/src` だが、テストや共通部品の変更も検知する。直接 `src/` に置くGoアプリなら、例として `src`、`go.mod`、`go.sum`、必要な設定ファイルをまとめる。複数アプリをまたぐIntentなら、関係するアプリを同じ集合へ追加する。

基本はIntent全体に一つのSHAとする。Unitを独立した範囲で検証する必要がある場合だけ、Unitにも `verification_paths` を設定する。省略時はIntentの範囲を利用する。Unitの実効検証範囲がIntent全体の対象に含まれることを、正規化したパスの包含関係で設定時に検査する。含まれない共通部品をUnitへ追加する場合は、Intent全体の範囲にも追加する。Unitがあるという理由だけでファイル別の管理情報を作らない。

対象はprojectからの相対パスで指定し、ディレクトリは配下の通常ファイルを再帰的に列挙する。生成物や依存物の展開先を含めないよう、AIがソース・設定・テストのパスを選ぶ。`node_modules` 等を名前だけで黙って除外したり、`.gitignore` を新たに解釈したりしない。明示した集合の範囲が不足していないかは実装計画と独立レビューで確認する。

コードを扱わない工程で新たにコード範囲を要求しない。TDDと統合検証には非空の対象指定を必須とし、その他の工程では対象が設定されている場合にその内容もレビュー対象へ含める。対象範囲の変更は設定変更として既存のbegin・検査・レビュー・承認の再確認条件を適用する。

## SHA計算の規則

1. 対象パスを正規化・重複整理して固定順へ並べる。指定順序だけでは値を変えない。
2. 算出方式の版、対象パス集合、列挙した相対パス・種類・内容を、長さを区切った曖昧さのない形式でSHA-256へ入力する。内容はバイナリを含むbytesとして読む。
3. 内容変更、追加、削除、名前変更を検知する。まだ存在しない指定パスは「不存在」として含め、CLIの結果にも表示する。新規実装前の計算や、ファイルを削除する作業を扱えるようにする。
4. 更新日時、Git情報、絶対パス、所有者は算出に使わない。同じ相対配置と内容なら、別の通常フォルダでも同じ値を得る。
5. コード集合から、共通project直下の管理ディレクトリ `aidlc/` と、任意の深さにある `.git` という名前のファイル・ディレクトリを除く。これらの内部を検証対象として直接指定することも拒否する。Knowledge・結果JSON・テスト出力は既存の文書・結果検査で別に比較する。結果にSHAを書いたためにコードSHAも変わる循環を避ける。
6. root自身の正当なsymlink別名は正規化する。対象集合内のsymlink、通常ファイル・ディレクトリ以外の項目、root外への参照は拒否する。
7. 一回の計算は最大10,000ファイル・内容合計256 MiBを初期上限とする。上限超過や読込失敗はエラーにし、一部だけを読んだSHAを成功として返さない。ファイル数・読込量を表示し、対象範囲の見直しを案内する。
8. 二回の列挙と内容計算が一致することを確認する。不一致なら再実行を求める。原子的なOS snapshotや、あらゆる書込みを封じる機能とは説明しない。

集合SHAはファイル内容の照合に使う。環境変数、外部サービス、インストール済みtoolchainの同一性まで示す値ではない。実行コマンド・結果と、環境に関する必要な情報は検証記録とレビューで確認する。

## CLIとテスト結果

終了Sensorに合格する前でも、読み取り専用の新しい入口からSHAを取得できるようにする。

```sh
aidlc intent hash INTENT_ID --space default --project-dir /path/to/project
aidlc intent hash INTENT_ID --space default --project-dir /path/to/project --unit unit-a --root /path/to/worker
```

通常は最初の操作だけを使う。Unitの別作業場所を指定する場合は、登録済みの担当rootと照合する。返値は算出方式の版、Intent ID、step ID、任意のUnit ID、実効対象パス、SHA、ファイル数、内容bytes数、不存在の指定パスとする。個別ファイルSHAの手動登録APIは設けない。新操作をhelp、CLI Skill、hookの読み取り許可一覧へ接続する。

メインAIは「SHA取得 → テスト実行 → SHA再取得」の順に確認し、前後が一致した結果を登録する。テストによって検証対象そのものが変わった場合は、変更内容を確認して再実行する。CLIはUnit証拠の提出・反映時にその時点のSHAを照合し、終了Sensorでは現在stepのIntent全体証拠を現在SHAと照合する。過去のUnit証拠は最新合格の代用にしないが、後続の内容変更だけで履歴の読取りを拒否しない。CLIからテストやagentを起動する新機能は追加しない。

既存の `test_results` は結果JSONのパス一覧として残す。JSONのコミット情報を次の形へ変更する。

```json
{
  "step_id": "s04",
  "stage": "tdd",
  "verification_scope": "intent",
  "verification_sha256": "64桁のSHA-256",
  "runs": [
    {
      "unit_id": "unit-a",
      "command": "go test ./...",
      "exit_code": 0,
      "output_path": "aidlc/evidence/INTENT_ID/s04/unit-a-tests.txt"
    }
  ]
}
```

直接実装では `unit_id` を省略する。独立Unitの証拠は `verification_scope: unit` とし、その結果のrunsを一つのUnitへ限定し、提出時の担当runとの対応も照合する。具体的にはunit用結果JSONに `unit_id` と `run_id` を追加し、各runに指定したUnitと一致させる。Intent全体の最終検証にはworkerのrun IDを要求しない。

同じコマンドを複数Unitが使っても `unit_id + command` で必要な成功結果を区別する。既存のstep、成功終了コード、非空出力、計画したコマンドの検査は残す。TDDと統合検証の終了時には、現在のIntent全体SHAに対して必要なテストが揃っていることを要求する。

テスト出力と結果JSONは `aidlc/` 配下の通常ファイルへ保存し、state・runtime・Knowledgeと混同しない。これらは従来の実行結果の保存であり、新しいsnapshot台帳ではない。登録内容が実際の実行を表しているか、RED/GREENの意味が妥当かは独立レビュアも確認する。

## Unit、割当、レビュー、承認

Unitの進捗と担当runは維持する。`claim` と `reassign` は現在step、依存Unitの進捗、担当、停止確認、割当競合を検査し、Gitの基準コミットと祖先関係を要求しない。

`result` は登録root・runと、CLIが再計算した集合SHAを照合して `result_sha256` を記録する。`integrate` は管理元で同じ相対パス集合を計算し、提出時の内容との一致を確認して `integrated` にする。この状態は「共通の成果へ反映した時点で内容を確認済み」を意味し、Git mergeを実行したことを意味しない。同じ場所での順次作業では、提出後の内容をその場で確認する。

提出後・反映前に対象が変わった場合、同じ担当runが有効であれば `reported` から最新結果を明示的に再提出できるようにする。再提出がないままSHAを自動差替えしない。別の作業場所から持ち込む場合は、比較対象の相対配置が対応していることを条件とし、自動的なパス変換は追加しない。

反映済みUnitに、現在SHAと過去の提出SHAの永久一致は要求しない。Unit Aの反映後にUnit Bが共通設定を変更した場合、Aの進捗は `integrated` のまま維持する。最新のIntent全体SHAで必要なテスト結果を登録し直し、全体をレビューする。古いUnit証拠だけでは終了Sensorを通さない。

workerの作業場所はGitルートでなく通常ディレクトリとして扱い、管理元と同じ場所を許可する。通常の順次作業は共通projectをroot、アプリ等をscopeとして割り当てる。正規化した同一rootと親子rootを競合扱いし、別session・Intent・Spaceからの二重割当も既存の管理元内lockで拒否する。互いに独立した別の作業場所の割当は残す。別々の管理元全体を横断する新しい中央台帳は作らない。

同じrootの独立レビューを許可し、レビュアと調整役は別会話であることを検査する。別rootのレビューでは、Intentの同じ検証範囲のSHAを照合する。レビュー中は実装を止める手順とし、受理・成果承認・finish時には現在のTargetを再計算する。

Targetには現在の全体SHA、step、実行計画と定義のhash、意味のある設定、対象文書、結果JSON・出力のhashを含める。Review・Approval自身や保存revisionをコードSHAへ含めない。コード・対象範囲・文書・結果が変わった場合は古い合格を使えず、state保存やmtimeだけで誤失効しないようにする。

Git差分による担当外ファイルの変更検査は撤去し、scopeによる割当と独立レビューで確認する。集合SHAだけで「どの担当が、範囲外のどのファイルを変更したか」を証明できるとは説明しない。担当範囲外の編集の網羅的制限は、元の要求でも対象外である。停止不明の割当を時間、結果提出、Stopだけで自動解放しない。

## 管理元の特定、保存形式、既存配置

CLIの管理元は、明示された `--project-dir` を最優先とする。省略時は現在位置から親方向へ既存の `aidlc/workflow/stage-graph.json` を探す。候補が複数ある場合は明示指定を要求する。新規installで候補がない場合は現在のフォルダを使い、その他の操作は未配置を案内する。Gitのルートや履歴を参照しない。hookは引き続き配置時の絶対 `--project-dir` を指定する。

flowはschema 6、assignmentはschema 2とする。これらは新形式の識別番号であり、旧形式との互換を意味しない。コミットfieldをSHAという別の意味で使い回さず、`code_revision`、`direct_commit`、Unitの各commitを新形式から外す。新形式そのものの必須項目・型・SHA・対応関係を検証する。

新方式は新規配置・新規Intentで開始する。旧記録の互換読込み、変換、旧版との二重運用、旧配置への切替・復元機能とその互換テストは実装しない。旧テスト・レビュー・承認を新方式へ引き継ぐ処理も作らない。検証用環境も新形式で作成する。

利用者の実プロジェクトや旧ファイルを自動削除・初期化する作業は行わない。新形式の保存失敗・途中保存・再試行の検査は維持する。開発中の候補を戻す場合は隔離した新規検証環境で候補binary・配置を切り替え、新旧の進行中stateを共有しない。一般の配布手順で必要な利用者設定の保全は維持する。

## 実装対象と単独writer

1 Issue / PR、1つのwork unit `git-independent-workflow` として、`go_tdd_implementer` 一人が以下の本体・テスト・製品手順を所有する。親はIssue・PR・承認と最終検証を管理し、実装中に同じ作業ツリーを編集しない。

- CLI: `src/cmd/aidlc/minimal.go`、新規 `project_root.go`、`src/internal/cli/minimal.go`、`help.go`。
- 集合SHA: 新規 `src/internal/flow/verification.go` とそのテスト。OSのファイル操作、`crypto/sha256`、既存のroot境界検査を利用する。
- state・Sensor・結果: `src/internal/flow/store.go`、`boundary.go`、`boundary_end.go`、`sensor.go`、`approval.go`、`src/internal/minimal/flow.go`。
- Unit・割当・レビュー: `src/internal/flow/unit.go`、`reassign.go`、`assignment.go`、`review.go`、`src/internal/assignment/reservation.go`、`store.go` と関連テスト。
- CLI接続とhook: `src/internal/minimal/child_hook.go`、`agent_hook.go`、`hook.go`、`session.go` の該当箇所。SHA取得は読み取りとして扱い、承認・担当制限を維持する。
- 配布: `src/harness/codex/minimal/aidlc-cli/SKILL.md`、必要な進行Skill、`agents/aidlc-worker.toml`、`agents/aidlc-reviewer.toml`、`src/core/workflow/stages/planning.md`、`tdd.md`、`integration.md`、`src/internal/install/` の既知配置判定・テスト。
- 説明と一周検証: `src/docs/user-guide.md`、`docs/distribution.md`、`docs/architecture.md`、新規 `src/cmd/aidlc/git_independent_journey_integration_test.go`、新形式へ更新する既存flow/assignment/配布journeyのfixture。

旧形式の対応だけのために古いassetsの文字列を増やさない。新方式の通常配置・移転で必要な既知assetsの照合と、無関係な利用者設定の保全は維持する。製品ビルド元のcommit情報や、本リポジトリのGitHub開発手順は利用時のGit依存と区別する。

## 順序付きTDDと検証

`loop`は次の順で、各項目の失敗する回帰テストを先に確認し、最小実装と整理を行う。実装担当は全項目を終えてから親へ返す。

1. Gitなしのroot解決: 明示指定、project直下、アプリ内からの探索、未配置、複数候補。対象 `./src/cmd/aidlc ./src/internal/cli`。
2. 集合SHA: 指定順序不変、内容変更、追加・削除・rename、不存在、二つのrootの同内容、対象範囲変更、symlink・上限・読込中変更の拒否。対象 `./src/internal/flow`。
3. 新設定・schema・hash CLI: 新形式の必須項目・型・対応、Intent/任意Unitの取得、help、承認待ち中の読み取り。対象 `./src/internal/flow ./src/internal/minimal ./src/internal/cli`。
4. 通常ディレクトリ割当: 同一root許可、同一・symlink別名・親子root競合、独立root許可、同時要求、新しい担当記録の検証、保存失敗と同一再試行。対象 `./src/internal/assignment ./src/internal/flow`。
5. 検証結果: step・SHA・Unit/run・command対応、別Unitで同じcommand、出力・終了コード、旧SHA拒否、結果保存によるSHA循環なし。対象 `./src/internal/flow`。
6. Unit: 提出・反映の内容一致、reportedでの再提出、依存順、再割当と停止確認、後続の変更で過去のintegratedを戻さない。対象 `./src/internal/flow`。
7. Sensor・レビュー・承認: 同じrootの独立レビュー、コード・範囲・文書・結果変更による失効、mtime/state保存だけでの誤失効なし、承認待ちと担当制限維持。対象 `./src/internal/flow ./src/internal/minimal`。
8. 配布・説明の整合: canonical Skill・agent・手順・help、新規配置、新方式の通常移転と無関係な設定の保全。対象 `./src/internal/install ./src/internal/cli`。
9. 一周fixture: Unit A反映→Bで共通依存変更→最新全体テスト→独立レビュー・承認・finish。同じrootの順次作業で完了し、Gitを呼ぶ入口がないことを確認する。全体journeyの実行はfinalへ集約する。

新設するtargeted testとコマンドは次に固定する。各prefix配下へ、その項目で必要なsubtest・追加testをまとめる。既存回帰の追加は実装中に判明した影響に応じて同じloopで行う。

```sh
go test -count=1 ./src/cmd/aidlc ./src/internal/cli -run '^TestProjectRootWithoutGit'
go test -count=1 ./src/internal/flow -run '^TestVerificationDigest'
go test -count=1 ./src/internal/flow ./src/internal/minimal ./src/internal/cli -run '^TestVerificationCLI'
go test -count=1 ./src/internal/assignment ./src/internal/flow -run '^TestDirectoryAssignment'
go test -count=1 ./src/internal/flow -run '^TestVerificationResults'
go test -count=1 ./src/internal/flow -run '^TestUnitWithoutGit'
go test -count=1 ./src/internal/flow ./src/internal/minimal -run '^TestVerificationGates'
go test -count=1 ./src/internal/install ./src/internal/cli -run '^TestGitIndependentInstall'
go test -count=1 ./src/cmd/aidlc -run '^TestGitIndependentFixture'
```

最後のコマンドは一周fixtureの入出力・判定helperだけを検査する。一周そのものはfinalで `go test -tags=integration -count=1 ./src/cmd/aidlc -run '^TestGitIndependentJourney$'` を実行する。

独立 `review` は承認済み計画との差、境界漏れ、旧合格の流用、誤ったSHA・Unit対応、保存失敗を確認する。必要な再現テストだけを実行し、全検証を繰り返さない。

差分が安定してから親がread-only `final` を一度開始する。全packageの `go test ./...`、`go test -race ./...`、`go vet ./...`、`gofmt -l`、差分検査、既存CI相当のbuild・配布検証を実行する。integrationタグの新規一周fixtureと既存flow/assignment/配布journeyを含める。変更後に古いfinal結果を使わない。

## 受入条件と固定Codex実機gate

- `.git` のない新規フォルダで、配置、Space/Intent作成、文書保存、Unitあり・なしの工程、Sensor、レビュー、承認、finishを完了できる。
- 製品子processへ渡すPATHにGitがなくても上記CLIが動く。テスト用Gitスタブで呼出しを検知する別検査も設ける。開発端末のGitを削除したり無効化したりしない。
- 一つのGit、アプリ別Git、Gitなしでも、同じ対象パスと内容のSHAは変わらない。任意のGit操作は利用者側で行える。
- ファイル単位のSHA登録なしで変更・追加・削除を検知し、古いSHAの結果・レビュー・承認で進めない。
- 同じ作業場所の重複workerを拒否し、停止不明や保存失敗で予約を失わない。別会話のreviewerが同じ場所を確認できる。

固定Codex CLI 0.153.4のsourceでは、Git markerがない場合はcwdをproject rootとし、通常trustを確認する構成になっている。native agentの公開入力仕様にもGit必須条件は見つかっていない。ただし非Git projectでの実機起動・hook発火は未実測である。[root探索](https://github.com/openai/codex/blob/rust-v0.153.4/codex-rs/config/src/loader/mod.rs#L1548-L1584)、[trust](https://github.com/openai/codex/blob/rust-v0.153.4/codex-rs/config/src/loader/mod.rs#L1314-L1417)、[agent仕様](https://github.com/openai/codex/blob/rust-v0.153.4/codex-rs/core/src/tools/handlers/multi_agents_spec.rs#L731-L758)。

実装前のG0では、隔離したGitなしprojectへ現行製品の明示root配置を行い、固定Codexをそのprojectから通常起動して、親子と既存hook入口の到達を確認する。現行製品のGit要求による拒否と、Codex側の読込不成立を区別する。通常trustを迂回せず、対象配置とhandlerが具体化した時点で必要な信頼確認を行う。前提不成立なら製品の保証を黙って弱めず原因を報告する。

最終候補では同じ非Git条件で、親子の担当制限、同じrootの割当、承認待ち拒否、結果提出、コード変更後の再検証を実測する。CLIだけの自動検証を、通常Codex hookまで確認した証拠として扱わない。

## 本家との差分と残る限界

固定した本家AI-DLC 2.6.123の確認範囲では、並列UnitにGit worktreeを用い、Teamの担当権はGit refで扱う。[確認した範囲と根拠](../ram/decisions/2026-09-10-upstream-agent-dispatch-and-worktree-reference.md)を参照する。本家全体や最新upstreamがGitを必須とする、と広げて断定しない。

今回はユーザー指定により、通常ディレクトリ、既存のローカル担当予約、集合SHAの内容照合を採用する。Gitの配置に左右されず、同じ場所で順次作業できることが理由である。影響は、Git履歴の包含証明・Git差分によるscope検査を外し、集合の内容一致と最新全体検証へ置き換えることである。Git上のチーム全体の担当権や、別の管理元同士の全体排他を新たに保証しない。

Git不要化、集合SHA方式、旧記録の後方互換不要と、新CLI・schema・Unit照合を含む本計画全体を直接承認済みである。Issue #171で、G0、単独writerのTDD、独立レビュー、final、PRのchecksを経て実装する。旧記録の互換性は再度の確認事項にしない。計画作成時点では製品コードとG0実機は未着手であり、その後の証拠はIssueとRAMに追記する。
