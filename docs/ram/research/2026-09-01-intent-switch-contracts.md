# Intent切替CLIの参照契約

- 日付: 2026-09-01
- 状態: Current for local AI-DLC 2.6.123 snapshot
- 関連: [Intent一覧の参照契約](2026-09-01-intent-list-contracts.md)、
  [承認済み計画](../decisions/2026-09-01-intent-switch-plan.md)

## 確認範囲

analysis indexと`core/tools/aidlc-version.ts`でローカルsnapshotのversion 2.6.123を確認した。
technical_researcherがauthored core、canonical Codex dist、配置済みCodex distを静的に比較し、
`aidlc-lib.ts`、`aidlc-utility.ts`、`aidlc.ts`、`aidlc-version.ts`が3形式で一致することを
確認した。主要3 toolのSHA-256は次のとおりである。

| file | SHA-256 |
| --- | --- |
| `aidlc-lib.ts` | `3b0ff3dda8abfbead8ff7e4a61b2ce334d1d3e78977995d1bd249668fc99db09` |
| `aidlc-utility.ts` | `c0a27b957f121d45c47f104eed1f7648406fc4756679e2edb71679fc046d4458` |
| `aidlc.ts` | `ad1b612d2fd4fb0f4817c04616a3f19e7d8b82fd3bcba793eec27d0da9c80187` |

command、hook、installer、testは実行せず、最新upstream全体との一致も主張しない。

## CLI grammar

public CLIは次の両形式を受理する。

    aidlc intent <name>
    aidlc intent switch <name>

`intent list`は一覧、`intent help`と`intent -h`はhelpである。bareの
`list`、`switch`、`create`は現在verbとして解釈され、`archive`、`rename`、`show`、`birth`も
将来または退役済みverbとして予約される。これらの名前を持つ既存recordは明示switchで到達できるが、
`help`と`-h`はutility側のbackstopでもhelpとなる。

public parserはglobal `--project-dir PATH`を分離形式で抽出し、重複時は最後を使う。
`--project-dir=PATH`はglobal parserでは認識しない。utilityへ転送するときに最初のtargetだけを残すため、
余剰位置引数や一部の後続flagは無視される。switch成功のJSON形式はない。

主要根拠:

- `docs/実装_aidlc-workflows/core/tools/aidlc-lib.ts:647-669`、`:732-818`
- `docs/実装_aidlc-workflows/core/tools/aidlc.ts:653-661`、`:874-922`
- `docs/実装_aidlc-workflows/core/tools/aidlc-utility.ts:6127-6158`

## 対象解決

本家は`resolveWorkflowSelection`でspaceとactive表示overrideを解決し、次の順でtargetを選ぶ。

1. `dirName`のcase-sensitiveな完全一致。
2. `dirName != null`の行に対するslugのcase-sensitiveな完全一致。
3. slug一致が1行なら選択、複数ならAmbiguous、0行ならUnknown。

targetはtrim、slugify、大文字小文字変換を行わない。exact directory一致は同名slugの曖昧性より
優先される。registry-only行は`dirName == null`なので選択できない。`aidlc-state.md` markerを持つ
orphan directoryは一覧に追加され、full directory名または一意な派生slugで選択できる。
重複registry行もslugのmatch数へ含まれる。

成功時はstdoutへ次を改行付きで出す。

    Active intent → <dirName> (space: <space>)

AmbiguousとUnknownはJSON stderr、exit 1である。Unknownは新しいworkflowの作成を促さず、
既存Intentの一覧だけを案内する。

主要根拠:

- `docs/実装_aidlc-workflows/core/tools/aidlc-utility.ts:6160-6205`
- `docs/実装_aidlc-workflows/core/tools/aidlc-lib.ts:1610-1625`、`:2350-2355`、`:2448-2488`

## 保存順と副作用

本家の成功経路は次の順である。

1. shared `active-space`が不在なら、完成済みstaging fileからhard linkを使ってno-replace補完する。
2. 対象spaceの`active-intent`へ`<dirName>\n`を直接上書きする。
3. 解決できたsession bindingを更新する。
4. rebind offerを削除する。
5. registry UUIDがあればsession UUID stampを更新する。
6. 成功表示を出す。

cursorとsession関連のmkdir、write、unlink失敗はbest-effortとして吸収される。lock、atomic replacement、
rollbackはなく、保存できなくても成功表示になり得る。registry、state、stageは成功時に変更しない。
Unknown等のerror経路では、解決可能なstateがあれば`ERROR_LOGGED`監査をbest-effort追記し得る。

session bindingはshared cursorよりspace選択を優先し得る。ただしactive-space補完値はsession選択spaceではなく、
shared `activeSpace(projectDir)`の値である。Intent切替だけでshared active-spaceを別spaceへ動かさない。

主要根拠:

- `docs/実装_aidlc-workflows/core/tools/aidlc-lib.ts:2491-2527`
- `docs/実装_aidlc-workflows/core/tools/aidlc-lib.ts:3219-3293`、`:3611-3652`、`:3683-3707`
- `docs/実装_aidlc-workflows/core/tools/aidlc-utility.ts:6183-6205`

## Go版への示唆

現行Go版はPR #26までにroot解決、shared active-space、registryとdirectoryの相関、Intent一覧を
実装済みである。`ReadIntents`はRootを閉じてmaterialized resultを返すため、mutation前のreadとして
再利用せず、同じproject/intents Rootを保持する新API内で`ListIntents`を再利用する。

既存Space切替は`os.Root`内の一時file、短いwrite、permission、Close、Rename、cleanupを検査する。
Intent切替もこの保存primitiveをprivateに共通化し、active-intentへ適用する。active-space不在時の
no-replace補完だけは本家の補助cursor契約に合わせ、失敗をswitchの成否へ昇格しない。

最初のsliceはshared active-spaceだけを使う。session binding、rebind offer、UUID stamp、auditは
後続の段階的実装であり、恒久的な本家差分として採用するものではない。外部libraryやAPIを
使わないためContext7照会は不要だった。親セッションではSerena 1.6.1を有効化し、現行Go symbolを
確認した。technical_researcherの実行環境にはSerena toolが公開されなかったため、その担当内では
`rg`と静的source確認で補完した。

## Test根拠と未固定箇所

- parserとexplicit switch: `tests/unit/t229-workspace-parser.test.ts`
- exact directory、help、Unknown: `tests/integration/t165-intent-create-p4.test.ts`
- slug switchとsession再stamp: `tests/integration/t173-session-switch-restamp.test.ts`
- ancestry session選択: `tests/integration/t311-session-binding-writers.test.ts`
- public router転送: `tests/unit/t230-dispatcher-routes.test.ts`

本家testで直接固定されていないAmbiguousの完全な出力、registry-only/orphan、active-space初回補完、
cursor保存失敗、同一target再指定、余剰入力、stdout失敗は、Go版のTDDで承認済み契約を固定する。
