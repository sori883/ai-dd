# Intent一覧をCLIへ接続する実装計画

- 日付: 2026-09-01
- 状態: Accepted
- GitHub Issue: [#25](https://github.com/sori883/ai-dd/issues/25)
- base: 7bb4e5682b00ea8d9ecf4dddbe9367fd4e0a88d8
- 作業branch: codex/intent-list
- 関連: [参照契約](../research/2026-09-01-intent-list-contracts.md)、
  [意図的な差分の提示](2026-08-31-intentional-upstream-difference-reporting.md)

## 承認

Space作成・一覧・切替のマージ後、Intent一覧を次のread-only sliceとして
実装する案を提示した。registry相関、workspace root接続、human/JSON CLI、
Dependencies整理、TDD、独立レビュー、配布E2E、PRまでの具体的な計画に対し、
ユーザーの「はい、じゃあ。」を明示承認として記録する。

外部Go module、intent作成・切替、session binding、audit、status修復、
registry migration、未提示の意図的な仕様変更は承認されていない。
自動マージは行わない。

## 目的とAPI

active spaceのintent registryとdirectoryを読み、CLIから安全に一覧できるようにする。
内部の中心APIは次とする。

    type Intent struct {
        UUID    string
        Slug    string
        Status  string
        Scope   *string
        Repos   []string
        DirName *string
        Active  bool
    }

    type IntentListing struct {
        ProjectRoot string
        SpaceName   string
        Intents     []Intent
    }

    func ListIntents(intentsFS fs.FS, activeOverride *string) ([]Intent, error)
    func ReadIntents(input RootInput) (IntentListing, error)

ListIntentsはregistry順、その後に未対応directoryの既存UTF-16順で返す。
truthy dirNameはexact matchだけ、欠落・null・空文字はlegacy名へfallbackする。
registry-only、orphan、重複行を保持する。activeOverrideがnilなら既存ActiveIntentを使い、
non-nil emptyはcursorとlone fallbackを抑制する。

registry不在・読取error・JSON不正・top-level非arrayは空registryとしてdiskを表示する。
valid array内にrequired/optional fieldの型不正が1件でもあれば全queryをerrorにする。
requiredはuuid、slug、statusのstring。optionalはdirName、scopeのnull/stringと、
reposのnull/string array。未知fieldは無視し、値の意味までは検証しない。

ReadIntentsは既存のroot優先順位、絶対既存project、ActiveSpace、
localizeSpace、os.Root境界を再利用する。intents不在は空成功、それ以外のopen/close errorは
原因を保持し、error時はzero valueを返す。ReadSelectionの読取範囲は変えない。

## CLI契約

    aidlc intent
    aidlc intent list [--json] [--project-dir PATH]

bare intentと明示listだけを認識する。intent create/switch、intent target、
専用helpは未実装のままexit 2とし、switch shorthandを追加しない。

空一覧:

    No intents in space "<space>" yet. Start one by describing what to build: /aidlc "build the auth service"

非空一覧は Intents in space "<space>": のheaderを出し、active行をasterisk、
inactive行を2空白で始め、statusを角括弧で表示する。activeがなければ空行後に
(no active intent — switch with /aidlc intent <name>) を出す。

JSONは1行と改行で、field順はactive、space、intents。
row順はuuid、slug、status、repos、dirName、active。scopeは出さず、
reposは常にarray、active不在はnullとする。

既存のstrict parserを維持する。--jsonは値なし、project-dirは分離と等号を受理し、
重複・空・未知flag・余剰引数を拒否する。構文errorではcallback、cwd、FSへ触れない。
認識済みlistのquery/output errorはJSON stderr、exit 1。短いwriteをerrorにし、
workspace command用SIGPIPE hookを適用する。root helpを更新する。

CLIの拡張点として、長い位置引数を concreteな cli.Dependencies structへ置換する。
CreateSpace、ListSpaces、SwitchSpace、ListIntents、PrepareOutputをfieldにし、
interfaceやrouter層は追加しない。private helper名はcommand全体で使える名前にする。

## 承認済みの意図的な差分

比較対象はローカル本家2.6.123の静的調査である。

| 本家 | Go版 | 理由・影響 |
| --- | --- | --- |
| valid array内のrowをunchecked castし、不正dataの結果が一定しない | rowを厳格検証し、1件でも不正ならquery error | 壊れたregistryをorphanとして偽装せずfail closedする |
| public parserは余剰入力を許容し、project-dir等号形式を認識しない | 既存Go版のstrict検証と等号形式を維持 | 既存CLIの誤入力検出と一貫させる |
| 通常path処理と状況に応じたroot作成 | 絶対の既存projectとos.Root境界を維持 | 既存Go版の安全境界を維持する |
| JavaScript JSON serialization | Go標準JSON encoderと短いwrite検出 | HTML文字のescape差があり得るが、追加dependencyなく出力失敗を検出する |

session binding、audit、create/switch、state/status修復、migrationは段階的な未実装であり、
恒久的な差分として採用したものではない。表示文が未実装commandを案内していても、
今回の本家表示互換を優先する。

## TDD・対象・完了ゲート

実装writerはgo_tdd_implementerの1名とし、同時編集しない。

- workspace: registry exact/legacy/registry-only/orphan/duplicate/order/active/repos、
  malformed fallback、不正row error、root/space/open/close、real FS link境界。
- CLI: human/JSON/empty/no-active/escaping、short write、strict args、callback未実行、
  bare alias、help、space regression。
- main: lazy RootInput、閉pipe、実FS接続。
- docs: architecture、development、E2E手順と最終証跡。

各sliceをassertionのRED、最小GREEN、refactorで進める。compile failureや
追加時点でGREENのtestをRED証拠に数えない。最終gateはtargeted、shuffle、race、
integration race、vet、gofmt、tidy差分、diff check、gopls diagnostics、
darwin/linux/windowsのamd64/arm64 cross buildである。

承認済みsandbox /Users/const/sori883/haihu-aidlc/e2e/ に未使用scenarioを作り、
同一source binaryのhash、stdout、stderr、exit、filesystem snapshotを確認する。
既存scenarioは変更・削除しない。各OS向けcross buildは実OSでの起動確認とは扱わない。

Issue #25に紐づくPRを作成し、自動マージせず引き渡す。
IssueはPRマージ後に閉じる。承認記録時点では実装・レビュー・E2Eは未実施。
