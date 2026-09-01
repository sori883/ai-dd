# Intent切替を共有カーソルへ接続する実装計画

- 日付: 2026-09-01
- 状態: Accepted
- GitHub Issue: [#29](https://github.com/sori883/ai-dd/issues/29)
- base: `37dd654f080ceeade0d5516b0262bd66b3df13bb`
- 作業branch: `codex/intent-switch`
- 関連: [参照契約](../research/2026-09-01-intent-switch-contracts.md)、
  [Intent一覧計画](2026-09-01-intent-list-plan.md)、
  [意図的な差分の提示](2026-08-31-intentional-upstream-difference-reporting.md)

## 承認

Intent作成と切替の実装順序を再検討し、一覧の次は切替を先行する案を提示した。本家2.6.123の
CLI、対象解決、cursor保存、session副作用と現在のGo実装を調査した後、bare/explicit両形式、
shared cursorだけの最小slice、TDD、独立レビュー、配布E2E、PRまでの詳細計画を提示した。

本家との差分として、安全な一時file置換と保存失敗通知、`os.Root`境界とcursor型検査、
strict CLIの3点を提示した。「この計画、特に上記3点の意図的差分を含めて実装を開始してよいですか？」
に対するユーザーの「はい、じゃあ実お願いします。」を明示承認として記録する。
外部Go module、未提示の意図的変更、自動マージは承認されていない。

## 目的とCLI契約

現在のshared active spaceに存在するIntentを既存一覧から解決し、共有`active-intent` cursorへ
選択を保存できるようにする。次の両形式を同じ処理へ接続する。

    aidlc intent switch <target> [--project-dir <path>]
    aidlc intent <target> [--project-dir <path>]

bare `intent`と`intent list`は一覧のまま維持する。bare形式の
`list`、`switch`、`create`、`archive`、`rename`、`show`、`birth`はtarget扱いせず、
既存のverbまたは予約verb境界を保つ。これらの名前を持つ既存recordは明示switchで指定できる。
raw `help`と`-h`は切替せず、既存Goのstrict error契約を維持する。case違いは通常targetである。

`--project-dir PATH`と`--project-dir=PATH`、root優先順位、未知・重複・空flag、余剰位置引数の
strict検証を既存CLIから継承する。`--json`は一覧専用でswitchではerrorにする。
成功時はstdoutだけへ次を出しexit 0とする。

    Active intent → <実際のdirName> (space: <space>)

認識済みswitchのsyntax、query、保存、Close、出力errorはJSON stderr、exit 1とする。
構文errorではcallback、cwd、environment、filesystemを読まない。stdout失敗でcursorが保存済みでも
rollbackしない。

## 内部APIと対象解決

    type IntentSelection struct {
        SpaceName string
        DirName   string
    }

    func SwitchIntent(input RootInput, target string) (IntentSelection, error)

1. `ResolveRoot`で既存優先順位を適用し、絶対の既存projectを`os.OpenRoot`で開く。
2. shared `ActiveSpace`を読み、`localizeSpace`で単一componentとして検証する。
3. 対象spaceのintents Rootを開き、`ListIntents(intentsRoot.FS(), &emptyOverride)`を呼ぶ。
4. case-sensitiveな`dirName`完全一致を最優先する。
5. 見つからなければ`DirName != nil`のslug完全一致を集め、一意なら選択する。
6. 複数なら候補directory名を含むAmbiguous、0件ならUnknownとして保存前に失敗する。

registry-only行は切替不可とし、markerを持つorphan directoryはfull directory名または一意な
派生slugで切替可能とする。重複registry行も候補数へ含める。targetをtrim、slugify、case変換しない。
`ReadIntents`はRootを閉じるためmutation APIへ流用せず、既存reader契約も変更しない。

## Cursor保存と失敗境界

- target解決後、shared `aidlc/active-space`が不在なら`<space>\n`をbest-effortで補完する。
  同じRoot内でstaging fileのwriteとCloseを完了後、hard linkでno-replace配置し、既存値を上書きしない。
- 対象intents Root内の`active-intent`へ`<dirName>\n`を保存する。Space切替のcursor処理を
  cursor名とtemp prefixを指定できるprivate primitiveへbehavior-preservingに抽出して再利用する。
- 既存cursorがsymlinkまたは非regularなら拒否する。一時fileを`O_CREATE|O_EXCL`で作り、
  write error、short write、permission復元、Close、Rename、cleanup errorを検査する。
- 既存cursorのpermission 9bitを保存するが、owner、ACL、特殊mode、hardlink identityは保証しない。
- projectとintents Rootを逆順に閉じ、Close errorを元の原因と結合する。error時はzero valueを返すが、
  Rename後、Root Close後、stdout失敗ではcursorが更新済みの場合がある。

一覧と保存の間の同時変更、active-spaceとの競合、全OS atomic、fsync、crash/power-loss耐久、
multi-file transaction、rollbackは保証しない。Rootはmountやdeviceを遮断する完全sandboxではない。

## 承認済みの意図的な差分

比較対象はローカル本家2.6.123の静的調査であり、最新upstream全体との完全互換は主張しない。

| 本家 | Go版 | 理由・影響 |
| --- | --- | --- |
| `active-intent`を直接best-effort write | 一時fileから置換し、保存失敗をerror通知 | 失敗を成功表示しない。directory書込権限が必要で、error時にも置換済みの場合がある |
| 通常path操作 | 既存project、`os.Root`、cursor通常file制約 | root外linkや特殊cursor配置を拒否する。mount/deviceまで遮断する完全sandboxではない |
| 余剰引数・一部flagを無視し、helpを表示 | 既存Goのstrict構文、JSON error、project-dir等号形式 | 従来無視された入力やhelp要求がerrorになる |

valid array内の壊れたregistry rowをquery errorにする挙動はIntent一覧で承認済みの契約を継承する。
active-spaceのno-replace補完とtarget解決順は本家に合わせる。session関連の未実装は段階的実装であり、
意図的な恒久差分として扱わない。

## 対象ファイルと所有権

実装writerは`go_tdd_implementer`の1名に限定する。

- `src/internal/workspace/intent_switch.go`と対応unit/integration test。
- private cursor共通化用のworkspace source/testと、`space_switch.go`のwrapper接続。
- `src/internal/cli/intent_switch.go`とtest。
- `src/internal/cli/cli.go`、`src/cmd/aidlc/main.go`とmain/Unix pipe test。
- `docs/architecture.md`、`docs/development.md`、`docs/e2e-testing.md`。

親はRAM、GitHub、commit、固定base/headのレビューhandoff、E2E証跡、PRを担当し、writerとの
編集期間を重ねない。外部module、store/interface、command registry、広いparser refactorは追加しない。

## Assertion-first TDD

1. exact directory、一意slug、exact優先、曖昧、Unknown、registry-only、orphan、重複、case-sensitive。
2. root優先順位、active space、missing intents、不正space、open/close、壊れたregistryの無書込み。
3. active-spaceのno-replace補完、既存値保持、補完失敗の非fatal契約。
4. active-intentのLF、同一target再保存、mode、symlink/nonregular拒否、write/short-write/
   Chmod/Close/Rename/cleanup errorと残存状態。
5. bare/explicit、一覧との分類、予約verb、strict flag、lazy callback、正確なstdout/JSON stderr/exit。
6. mainのcwd/env接続、PrepareOutput、閉pipe、既存help/version/space/intent-list回帰。
7. integration tagで実FSのRoot/link境界とregistry/state/session/auditの無変更。

各behaviorでrunnable assertionのREDを観測し、最小GREEN、GREEN上のrefactorを行う。
compile failure、追加時点でGREENのguard、構造変更だけをRED件数に含めない。

## 検証と完了gate

    go test -count=1 ./src/internal/workspace ./src/internal/cli ./src/cmd/aidlc
    go test -tags=integration -count=1 ./src/internal/workspace
    go test -count=1 -shuffle=on ./...
    go test -count=1 -race -shuffle=on ./...
    go test -tags=integration -race -shuffle=on ./...
    go vet ./...
    go vet -tags=integration ./...
    go mod tidy -diff
    gofmt -l src
    git diff --check

darwin、linux、windowsのamd64/arm64をCGO無効でcross-buildする。cross-buildは各OSでのnative実行
証拠とは扱わない。承認済み`/Users/const/sori883/haihu-aidlc/e2e/`の未使用scenarioで、
bare/explicit、対象解決、active-space補完、cursor mode、strict syntax、link境界、閉pipeを確認する。
binary source/hash、stdout、stderr、exit、filesystem snapshotを記録し、既存scenarioを変更・削除しない。

Issue #29から単独TDD、固定base/headの独立review、必要な修正、最終検証、配布E2E、PRへ進む。
PRはIssueへ紐づけ、自動マージしない。Issueはマージと作業完了を確認した後にcloseする。
承認記録時点では実装、review、E2E、PRは未実施である。

## 実装・検証結果

2026-09-01、Issue #29の承認scopeで実装を完了した。製品差分を
`e4541fc2d65a0ad8eba79ded24debe27a90d085e`へ固定し、base
`37dd654f080ceeade0d5516b0262bd66b3df13bb`からの最終独立reviewはNo findingsだった。

初回head `e1f5e571bc7cfdf39577684cf29082f78ba27dbc`のreviewではP0/P1なし、
同一target再保存test不足のP2 1件と、targeted command・説明のP3 2件を発見した。
同じ実装担当が追加時点GREENの回帰guardと文書修正を行い、再reviewで3件の解消と
追加findingなしを確認した。

- observableなassertion RED→GREENはworkspace 5、CLI 6、main 2の合計13 slice。
  review guard、compile failure、構造整理はRED件数に含めない。
- raw RED transcriptの一部は永続化されず、commandと失敗要旨のturn要約だけが残る。
  final test sourceとfresh GREENは再現可能だが、過去の遷移を独立再現した証拠とは扱わない。
- 通常・integration・shuffle/race・integration race、vet、tidy差分、gofmt、gopls、
  diff checkを通過し、通常coverageは全体88.0%だった。
- darwin/linux/windowsのamd64/arm64でCLIとworkspace integration test binaryの
  合計12 artifactをcross compileした。native実行はmacOS/arm64だけである。
- clean sourceから配布binaryを作り32回起動した。期待exit・stdout/stderrに全件一致し、
  宣言したcursor更新14回以外のfixture差分はなかった。
- 最初のE2E driverは検査器側の2件の誤りでFAILしたためscenarioを保存し、
  新しいscenarioで検査器単独testを通した後に全caseを完走した。

具体的なartifact、command、hash、TDD、review、cross-build、最初のdriver失敗、
未検証範囲は[Intent切替の実装検証・配布E2E](../../e2e-runs/2026-09-01-intent-switch.md)へ記録した。
承認済みの意図的な差分3点は変更せず、新しい意図的差分も追加していない。
外部module/tool、CI、session binding、rebind、UUID stamp、audit、Intent作成は追加していない。
後続の証跡commitは文書だけで、検証binaryのsource commitとは区別する。
PRはIssue #29へ紐づけ、自動マージしない。Issueのcloseはマージ後とする。
