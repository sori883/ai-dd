# Space作成をCLIから使えるようにする実装計画

- 日付: 2026-08-31
- 状態: Accepted（2026-08-31に詳細計画と意図的な変更を明示承認）
- GitHub Issue: [#19](https://github.com/sori883/ai-dd/issues/19)
- 前提: [PR #18](https://github.com/sori883/ai-dd/pull/18)はマージ済み、Issue #17はClosed。
  merge commitは`01a6d523037c303e8c0b6f59029a6208a551a832`。
- 関連: [参照契約の調査](../research/2026-08-31-space-creation-contracts.md)、
  [初期境界](2026-08-29-initial-implementation-boundaries.md)、
  [実装順序](2026-08-31-internal-workspace-before-status.md)、
  [意図的な差分の提示](2026-08-31-intentional-upstream-difference-reporting.md)

## 背景・承認の境界（計画提示時点の記録）

root、space、intentの読み取り接続後、state本文読み取りを次候補として提示したところ、
ユーザーからspaceをいつ作れるようにするか確認があった。
space作成を先に扱い、「本家が何を生成するか調査し、space作成だけの計画を提示して、
承認後に実装する」順序を提案し、ユーザーは「はい。お願いします」と了承した。

この回答は調査・計画と実装順序への了承として記録する。
以下の新しいCLI、書込み、エラー、安全性の契約への包括承認とは扱わない。
本タスクでは親がRAMの調査・計画・索引だけを追加し、コード、設定、Issue、PRは変更しない。
前タスクから残る`AGENTS.md`とRAMの方針変更は保持する。

## 実装承認（2026-08-31）

以下の詳細計画と意図的な変更を提示し、Issue作成、TDD実装、独立レビュー、
配布E2E、PR作成まで進めてよいか確認した。
ユーザーは「はい、じゃあ実装お願いしていいですか。」と明示承認した。

この回答を、以下のscope、CLI・書込み・安全性の契約と比較表の採用案への承認として記録する。
次にspace作成を計画する順序だけを了承した以前の回答とは区別する。
外部Go module/toolの追加、対象外機能、scope外の仕様変更は承認されていない。

Issue #19を作成し、マージ済みPR #18を含む`origin/main`から
`codex/space-create`を作成した。前タスクの未pushのAGENTS/RAM変更は保持し、同じPRに含める。
実装writerは1名とし、親のRAM・E2E記録編集とは期間を重ねない。
PRはIssueへ紐づけ、自動マージせず、マージ後にIssueを閉じる。

## 目的・受入条件

Goの単一binaryから、次のコマンドで新しいspaceの雛形を作成できるようにする。
内部helperだけで完了とはしない。

```text
aidlc space create <name> [--project-dir <path>]
```

1. 本家ローカル`2.6.123`と同じ名前正規化・初期directory/file・org継承を持つ。
2. 既存spaceを上書き・merge・修復せず、作成後の現在spaceも切り替えない。
3. 現在space、intent、state、stage graphを作成の前提にしない。
4. CLIの入力、stdout/stderr、終了コード、無効入力時の無書込みをテストで固定する。
5. workspaceの参照境界、同名作成の競合、失敗時の残存物を明確にする。
6. 配布したbinaryから作成・重複拒否・不正入力を確認するE2Eを持つ。

## Scopeと対象外

追加するのはspace作成CLIと、そのためのroot入力接続・名前処理・filesystem書込みだけとする。
Go標準ライブラリと手動DIを維持し、外部Go module/toolは追加しない。

space一覧CLI、切替、削除、rename、intent作成、state/status、registry、audit、session、
CodeKB/DocumentKBの解析、harness資産展開、project全体の初期化は対象外。
legacyの`space-create`alias、force、dry-run、作成結果の`--json`も追加しない。
空のstore/state等のpackageを先行作成しない。

本家のknowledgeDirに結合したsession検証と、失敗時のbest-effort監査追記は、
周辺機能の段階的な未実装範囲として扱う。今回の処理はそれらを呼ばず、
現在workflowを読んだり、監査やbindingを書き換えたりしない。

## CLI契約

- `<name>`はちょうど1つの位置引数。複数単語の名前は引用して渡す。
- `--project-dir <path>`と`--project-dir=<path>`を受け付ける。
  commandの前・途中・名前の後に配置できるようにする。
- 余剰位置引数、未知flag、重複したproject-dir、欠落・空のproject-dir値は拒否する。
  `--name`や`--force`等を黙って無視しない。
- これらの構文エラーでは作成callbackを呼ばない。stdin読取り・対話promptは追加しない。
- rootの既存help/versionは挙動を維持し、helpへ新しい作成構文を追記する。
  作成位置の`help`や`-h`は名前として拒否し、spaceを生成しない。

| 状況 | 出力・終了コード |
| --- | --- |
| 作成成功 | stdoutに`Space created: <正規化名>\n`、stderr空、0 |
| 認識済み`space create`の失敗 | stderrに1行のJSON `{"error":"..."}`、1。名前欠落も同じ |
| rootの既存help/version | 現行の表示・終了コードを維持。作成callbackは呼ばない |
| rootの未知command | 現行のstderrと終了コード2を維持 |
| 成功メッセージの出力失敗 | 1。作成済みspaceは取り消さない |

JSONは`encoding/json`で生成し、手動の文字列結合でescapeを実装しない。
OS固有のエラー本文の完全な一致は約束せず、操作・対象・原因がわかる文脈を付ける。
stderrも書き込めない場合は終了コードで失敗を知らせる。

## API・責務

候補APIは`workspace.CreateSpace(input RootInput, rawName string) (string, error)`。
成功時は正規化した名前を返し、error時は空文字と原因を返す。
errorでも部分生成物が存在し得ることをAPI文書へ明記する。

CLIへは`func(rawName, explicitDir string) (string, error)`のcallbackを渡す。
`cli.Run`は構文・表示・終了コードを所有し、正常な入力でcallbackを1度呼ぶ。
Rootや書込み用FSはCLIへ持ち出さない。

`main`がcallbackの中でだけcwdと環境変数を読み、次を`RootInput`へ渡す。

- `ExplicitDir`: `--project-dir`の値。
- `AIDLCProjectDir`: `AIDLC_PROJECT_DIR`。
- `ClaudeProjectDir`: `CLAUDE_PROJECT_DIR`。
- `WorkingDir`: `os.Getwd()`の結果。

既存`ResolveRoot`の優先順位と正規化は変えず、filesystemへ到達する時点では絶対pathを要求する。
help/versionとCLI構文エラーではcwd取得・環境参照・filesystem操作を始めない。
`ReadSelection`やactive-space/intent readerを作成の前処理に使わない。

## 名前と生成内容

独自のASCII入力拒否ではなく、参照契約の`slugify`と予約名判定を再現する。
小文字化、ASCII英数字以外の連続のhyphen化、端のhyphen除去、48文字切詰め、末尾hyphen除去、
必要な`intent-`prefix付加の順序を維持する。
prefixは切詰め後なので、最終名を一律48文字に再制限しない。

空入力とrawの`help`・`-h`を拒否し、正規化後の
`help/list/switch/create/archive/rename/show/birth`も拒否する。
非空の空白・記号だけの入力は本家どおり`intent`に正規化する。
`default`自体は禁止せず、実directoryが既にあれば通常の重複として扱う。
Go側の検証・path変換はfilesystem操作の前に行う。

生成先は`<project>/aidlc/spaces/<正規化名>/`。
詳細なtreeと本文は[参照契約](../research/2026-08-31-space-creation-contracts.md)を正とする。

- `memory/org.md`: defaultの同名fileを継承。不在なら`# Organization defaults\n`。
- `memory/team.md`: `# Team practices\n`。
- `memory/project.md`: `# Project overrides\n`。
- `memory/phases/`と`intents/`: 空directory。
- `memory/templates/.gitkeep`、`codekb/.gitkeep`、`knowledge/.gitkeep`: 空file。

team/projectやtemplateの本文、既存CodeKB/DocumentKB内容はコピーしない。
orgの読取りに失敗した場合にその失敗を新規stubで隠さず、確認できた不在だけをfallback対象にする。
既存defaultの内容、permission、mtime等を変更しない。

## 書込み境界・既存data・失敗

1. 指定されたproject自体は既存directoryを`os.OpenRoot`で開く。
   初期project pathがsymlinkの場合は既存Root方針どおり追従する。
2. 以後のアクセスをそのRoot配下に限定する。必要な`aidlc/spaces`等の祖先は内部で作れるが、
   project外や絶対symlink経由の作成・org継承は拒否する。
   境界内の相対symlinkは一律禁止にしない。
3. targetに対する単独の`Mkdir`成功で作成権を取得する。
   既存directory・file・symlink（danglingを含む）は再利用せず、`fs.ErrExist`を識別できるerrorにする。
4. 取得後に必要な子directoryを作り、fileは`O_CREATE|O_EXCL`を使って新規作成する。
   既存fileを`O_TRUNC`で上書きしない。
5. file・Rootは取得したものを必ずCloseし、Close失敗を無視しない。
   操作失敗とClose失敗が重なっても`errors.Join`等で原因を保持する。

本CLI同士の同名作成では、1つだけがtargetの作成権を得て書込みへ進む。
これは成功を保証するlockではなく、権利を得た側もI/O失敗することがある。
異なる名前の作成は各targetを使い、共有cursor/registry等を変更しない。

途中失敗・Close失敗・出力失敗では、部分的または完成済みのspaceが残り得る。
自動repair、上書き再開、rollback、再帰削除はしない。再実行は既存targetとして拒否する。
エラーには対象pathと失敗した操作の文脈を含める。

directoryの敵対的な差替え・mount・特殊file/deviceまで防ぐ完全sandboxや、
全OSでのall-or-nothing transaction、crash durabilityは保証しない。
stagingをRenameすれば全OSでatomic/no-clobberになる、という前提は採用しない。

## 承認対象となる意図的な変更

| 本家の挙動 | 採用案 | 理由・利用者への影響 |
| --- | --- | --- |
| project自体の不足も再帰mkdirで作れる | projectは既存directory必須 | 指定誤りでprojectを新設しない。必要なら利用者が先にdirectoryを用意する |
| 通常のpath操作でリンク先を参照 | project Rootの境界を適用 | 境界外への書込み・copyを防ぐ。そのような配置はerrorになる |
| 事前存在確認後に再帰mkdirし、排他はない | targetの単独Mkdirとfileの排他作成 | 同名競合・dangling targetでも既存dataを再利用せず、片方を拒否する |
| orgのexistsSyncがfalseならstubにする | 確認できた不在だけstub、その他のアクセス障害はerror | orgの継承失敗を黙って置き換えない。読取り可能なseedまたは不在が必要 |
| 余剰引数や未使用flagを厳格に検証しない | 不明・余剰・重複・空flag値を事前拒否 | typoによる意図しない作成を防ぐ |
| 成功時3行と切替案内、名前欠落時はusage | 成功1行、認識済み作成エラーはJSONへ統一 | 未実装の切替を案内せず、失敗を機械判読可能にする。作成失敗の終了コード1は維持 |

言語・型・内部構造の違いは追加の差分承認事項として列挙しない。
audit/sessionやlegacy alias等は、意図的な恒久的非互換ではなく今回の対象外として分ける。

## 対象ファイル・所有権

承認後の実装writerは`go_tdd_implementer`の1名とする。

- 更新`src/internal/cli/cli.go`、`cli_test.go`: dispatch、help、callback接続と既存非回帰。
- 新規`src/internal/cli/space.go`、`space_test.go`: 作成構文、出力、終了コード。
- 更新`src/cmd/aidlc/main.go`、新規`main_test.go`: lazyなroot入力組立と作成callback。
- 新規`src/internal/workspace/space_create.go`、`space_create_test.go`、
  `space_create_integration_test.go`: 名前、作成、filesystem契約。
- 更新`docs/architecture.md`、`docs/development.md`、`docs/e2e-testing.md`。
- E2E実施後に`docs/e2e-runs/<実行日>-space-create.md`を追加。

既存root/space/intent readerや`ReadSelection`のAPI・挙動、go.mod、CI設定は変更しない。
必要な私有helperと既存test helperは再利用できるが、別機能のrefactorは含めない。
RAM・索引・GitHub操作は親担当で、実装writerと編集期間を重ねない。

## TDDと検証予定

1. 名前の正規化、予約名、48文字前後、数字開始、空白・記号、Unicode小文字化を固定する。
2. 空のprojectで正確なtree/bodyを作り、orgの継承・fallback、default新規作成を固定する。
3. 既存directory/file/linkの保護、同名競合、seed読取失敗、途中write/Close失敗を固定する。
4. flagの前後・途中配置、`=形式`、不正構文でcallback未実行、出力・終了コードを固定する。
5. mainのroot入力・優先順位、cwd取得失敗、help/versionの無副作用を固定する。
6. 実Rootのsymlink境界と、既存data/cursor等の無変更をsnapshotで確認する。

各新動作で、変更前の実装で失敗するassertionを観測してから最小実装を行う。
compile errorや既にGREENのguardをRED実績には数えない。
既存の`bytes.Buffer`、`errorWriter`、関数注入、snapshot helperを必要な範囲で使う。
異常系はchmodだけに依存せず、非公開の関数注入で再現する。
mutable globalや大きな独自FS/store interfaceは追加しない。

```sh
go test -count=1 ./src/internal/cli ./src/cmd/aidlc ./src/internal/workspace
go test -tags=integration -count=1 ./src/internal/workspace
go test -count=1 -shuffle=on ./...
go test -count=1 -race -shuffle=on ./...
go test -count=1 -tags=integration -race -shuffle=on ./...
go vet ./...
go vet -tags=integration ./...
go mod tidy -diff
gofmt -l src
git diff --check
```

`CGO_ENABLED=0`でmacOS/Linux/Windows × amd64/arm64の6構成をcross buildする。
実行可能なhostのnative testと他OSのcompile確認を区別する。
Windowsのsymlink作成権限等でskipする場合は該当caseと理由を記録する。

配布E2Eは承認済み`/Users/const/sori883/haihu-aidlc/e2e/`配下の未使用scenarioへ隔離し、
binaryと検証用projectを配置する。新規作成、org継承、重複拒否、不正入力と生成物を確認する。
既存scenarioやsandbox rootを上書き・削除しない。これはspace作成E2Eであり、
切替・intent・workflow実行までを検証したとは扱わない。

## 調査・計画時点の根拠・ゲート

調査・計画は読み取り専用のtechnical_researcherとproject_plannerで分担した。
親は本家の作成本体、名前処理、RAM、Go APIを確認して統合した。
調査・計画時点では、新機能のRED/GREEN、実機動作、cross build、配布E2Eは未実施だった。

Go APIはContext7の標準ライブラリ資料（Go1.25.3）を優先参照し、ローカルGo1.26.4の
`go doc`で`os.Root`、`Root.MkdirAll`、`Root.OpenFile`、`O_EXCL`、`Rename`を照合した。
Rootでsymlinkはroot内に限定されるが完全sandboxではなく、非UnixのRenameはatomic保証がない。
参照: [os.Root](https://pkg.go.dev/os@go1.26.4#Root)、
[os.OpenFile](https://pkg.go.dev/os@go1.26.4#OpenFile)、
[os.Rename](https://pkg.go.dev/os@go1.26.4#Rename)。

詳細承認、RAMへの承認追記、新しいIssueと作業branchの準備は上記の実装承認時に完了した。
前タスクの未pushのAGENTS/RAM変更を失わず、新しい作業とともにPRへ反映する。
実装承認時点の残るゲートは単独TDD実装、独立レビュー、親の最終検証、配布E2Eと
Issueへ紐づくPR作成だった。後続結果は下記へ追記し、自動マージはしない。

撤回する場合はコードと対応文書の変更を戻す。利用者が作ったspaceやE2E artifactの削除とは分離し、
それらを自動で消さない。上記scope、CLI出口、安全性の意図的変更は承認済みである。

## 実装時の契約具体化と検証（2026-08-31）

分離形式の`--project-dir`に続く値が`-`で始まる場合は、欠落値としてcallback前に拒否する。
例えば`--project-dir --force`で未知flagをpathとして消費しない。
実際に`-`で始まるdirectoryは`--project-dir=-dir`または`--project-dir ./-dir`で指定できる。
これは承認済みのstrict flags契約を親が具体化したもので、新たなユーザー承認の記録ではない。
テストと利用手順にも同じ条件を反映した。

名前のU+0130互換性について追加調査し、[参照契約](../research/2026-08-31-space-creation-contracts.md)へ記録した。
実装担当は21 sliceでassertionのREDを観測し、最小実装でGREENにした。
最初からGREENだった非回帰guardや、テスト期待の訂正はRED実績に数えていない。

実装完了時点で、全packageの通常・shuffle・raceテスト、integration付きrace/shuffleテスト、
通常/integrationのvet、`go mod tidy -diff`、gofmt、差分の空白検証が成功した。
変更Goファイルの通常/integration gopls CLI診断にも出力はなかった。
専用gopls MCPは公開されていないため既存CLIを使用し、外部toolは導入していない。
govulncheckは未導入・未実施であり、脆弱性検査済みとは扱わない。

親は固定したGoソースについて、`CGO_ENABLED=0`のmacOS/Linux/Windows × amd64/arm64で
CLIとintegration付きworkspace test binaryをそれぞれcross compileし、6構成とも成功した。
nativeテストはmacOS arm64であり、他OSの実行確認とは区別する。
この時点で残るゲートは配布E2E、独立レビュー、Issueへ紐づくPR作成である。
