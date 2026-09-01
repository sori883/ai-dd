# Intent作成の内部coreとworkspace lockの実装計画

- 日付: 2026-09-01
- 状態: Accepted
- GitHub Issue: [#31](https://github.com/sori883/ai-dd/issues/31)
- base: `d5fd5444a744d721b83d588120c9b9a83e853573`
- 作業branch: `codex/intent-create-core`
- 関連: [参照契約](../research/2026-09-01-intent-create-contracts.md)、
  [初期実装の境界](2026-08-29-initial-implementation-boundaries.md)、
  [意図的な差分の提示](2026-08-31-intentional-upstream-difference-reporting.md)

## 承認

Intent一覧・切替のマージ後、Memoryの読み取りより先にIntentを実施できる土台を作る方針を
ユーザーと確認した。本家2.6.123のauthored implementation、canonical Codex dist、配置済みCodex
distを調査し、lock-backed internal core、API、保存形式、TDD、独立review、PRまでの計画を提示した。

壊れたregistryのfail-closed、既存Space必須、cursor失敗通知、既存`os.Root`境界、最初のGo版では
stale lockを自動回収しないという意図的な差分を明示した。「この計画と、表の意図的な差分を含めて
実装を開始してよければ、明示的に承認をお願いします。」に対するユーザーの
「はい、じゃその実装で作ってもらっていいです。お願いします。」を明示承認として記録する。
外部Go module、未提示の意図的変更、自動マージは承認されていない。

## 目的と内部API

決定済みの既存Spaceへ、workspace lock下でIntent identity、record、registry、cursorを一貫して
作成する。内部APIは次とする。

    type IntentCreateInput struct {
        SpaceName string
        Label     string
        Scope     *string
        Repos     []string
    }

    type CreatedIntent struct {
        UUID      string
        Slug      string
        DirName   string
        RecordDir string
        SpaceName string
    }

    func CreateIntent(ctx context.Context, root RootInput, input IntentCreateInput) (CreatedIntent, error)

Space selection、scope、reposの意味検証はcallerが行い、coreはSpace名のsingle component検証と既存tree
確認だけを行う。labelは本家のslug ruleで正規化する。Reposは受取順を維持し、空ならregistry fieldを
省略する。private locked primitiveを分け、後続のfull handlerが同じcritical sectionをauditと
full stateまで延長できる構造にする。

## 作成・commit契約

1. `ResolveRoot`で絶対の既存projectを決め、context-aware WORKSPACE lockを取得する。
2. registryを検証し、invalidならmutation前に停止する。
3. UUIDv7、24文字slug、予約語、UTC date、directory collisionを解決する。
4. record directoryとheader-only state stubをexclusiveに作る。
5. 既存registry raw rowを保持し、新規rowをatomic appendする。ここをdurable create commitとする。
6. shared active-spaceを不在時だけ`ActiveSpace`のfallback値でno-replace補完し、対象Spaceの
   active-intentを安全に置換する。Intent作成先だけを理由にshared selectionを切り替えない。
7. Rootとlockを解放する。

registry commit前のerrorはzero resultを返す。commit後のcursor、Close、lock release errorは
`CreatedIntent`とerrorを同時に返し、callerが作成済みと判断できるようにする。retryを自動実行しない。
rollback、fsync、power-loss耐久、multi-file atomicityは保証せず、本家同様partial artifactが残り得る。

## Workspace lock

本家と同じcanonical identity、MD5先頭8文字、system temp lock directoryを用い、Go process同士と
通常の本家processとのmkdir排他を共有する。Windows pathはECMAScript default lowercaseの
U+0130展開とFinal Sigma文脈をGo標準Unicode tableへ補い、本家と同じidentityにする。取得時は
owner PID、epoch milliseconds、random tokenをowner stampへ保存し、自分のtokenと一致する
generationだけを解放する。100ms間隔、最大600 retriesを上限とし、より短いcaller deadlineまたは
cancelを優先する。取得失敗errorにはlock pathを含める。

最初のGo版はowner generation probeとreap gateを実装せず、既存lockを自動削除しない。dead ownerや
malformed stampもfail-closedとし、doctor相当を実装するまで診断後の手動復旧とする。Goが作るstampは
本家readerがlive/unknown ownerとして安全側に扱える形を維持する。

## 承認済みの意図的な差分

比較対象はローカル本家2.6.123である。

| 本家 | Go版 | 理由・影響 |
| --- | --- | --- |
| malformed・非配列registryを空listとして上書き | 既存rowをstrict検証し、異常時は無変更でerror | registry消失を防ぐ。壊れたworkspaceは作成前に修復が必要 |
| raw coreが存在しないSpace treeを作成可能 | 既存Spaceを必須とする | typoによる暗黙Space作成を防ぎ、Space lifecycleを既存APIへ限定 |
| cursor failureをbest-effortで吸収 | registry commit済み結果とerrorを返す | 失敗を通知しつつ、再試行による重複Intentを避けられる |
| 通常path操作 | `os.Root`、single component、通常file制約 | root外linkと特殊fileを拒否する。mount/deviceまで遮断する完全sandboxではない |
| owner generation付きstale-lock reaper | 自動回収しないfail-closed lock | 誤ったlock奪取を避ける一方、crash後は診断と手動復旧が必要 |

UUID・JSON・lockのGo内部表現や、public handlerを段階的に後続実装することは意図的な恒久差分として
扱わない。directory・stub・registry失敗時にrollbackしない点は本家と同じである。

## 対象ファイルと所有権

実装writerは`go_tdd_implementer`の1名に限定し、親と編集期間を重ねない。

- `src/internal/workspace/intent_create.go`とunit/integration test。
- `src/internal/workspace/workspace_lock.go`とtest。
- registry reader/writerを共有するworkspace source/test。
- slug helperの上限を共有するために必要な既存workspace source/testの小さなrefactor。
- `docs/architecture.md`、`docs/development.md`。

親はRAM、GitHub、commit、固定base/headのreview handoff、最終証跡、PRを担当する。外部module、
public CLI、store/interface、広いpackage分割は追加しない。公開CLIはfull state、audit、workspace scanと
一緒に別計画で接続し、内部coreだけを不完全な公開契約として表示しない。

## Assertion-first TDD

1. UUIDv7のformat、timestamp、variant、entropy failure、tight-loop uniqueness。
2. slug、U+0130、空・数字先頭、予約語、UTC date、base・suffix・`-999` exhaustion。
3. 既存SpaceとRoot境界、record directory、正確なstate stub、戻り値。
4. registry不存在・valid append、field順・indent・LF、scope/repos省略、未知field保持。
5. malformed・非配列・invalid row、symlink・特殊file、write/short-write/Close/Rename/cleanup failure。
6. active-space no-replace補完、active-intent安全置換、commit前後のresult/error境界。
7. lock identity、Windows Unicode lowercase固定vector、owner stamp、contended wait、
   context cancel・deadline、owner不一致release拒否、cleanup。
8. helper subprocessによる2 process同時作成と、distinct UUID・directory・2 registry rows。
9. 既存Space・Intent read/list/switchの回帰。

各behaviorでrunnable assertionのREDを観測し、最小GREEN、GREEN上のrefactorを行う。compile failure、
追加時点でGREENのguard、構造変更だけをRED件数に含めない。

## 検証と完了gate

    go test -count=1 ./src/internal/workspace
    go test -tags=integration -count=1 ./src/internal/workspace
    go test -count=1 -shuffle=on ./...
    go test -count=1 -race -shuffle=on ./...
    go test -tags=integration -race -shuffle=on ./...
    go vet ./...
    go vet -tags=integration ./...
    go mod tidy -diff
    gofmt -l src
    git diff --check

darwin、linux、windowsのamd64/arm64でpackage test binaryをcross compileする。cross-buildは各OSでの
native実行証拠とは扱わない。公開CLIを変更しないため配布binary E2Eとは呼ばず、実filesystemと
helper subprocessのintegration testを内部featureの実行証拠とする。

Issue #31から単独TDD、固定base/headの独立review、必要な修正、最終検証、日本語PRへ進む。
PRはIssueへ紐づけ、自動マージしない。Issueはmergeと作業完了を確認した後にcloseする。
承認記録時点では実装、review、PRは未実施である。
