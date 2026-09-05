# 配布E2Eテスト

## 目的

AI-DLCの実行物をrepository外へ配布し、配布先から起動して利用者に見える挙動を確認する。
unit testやrepository内でのbuild確認とは別に、配布境界を通過したことを検証する。

## 承認済みlocal sandbox

- 指定path: `~/sori883/haihu-aidlc`
- この環境で解決した絶対path: `/Users/const/sori883/haihu-aidlc`
- 実行ごとの配置先: `e2e/<YYYY-MM-DD>-<scenario>/`

sandbox rootそのものへ実行物を直置きせず、必ずscenarioごとの新しい子directoryを作る。
既存の子directoryや無関係なfileを上書きしない。過去のartifactを削除またはclean upする場合は、
対象を明示してユーザーの承認を得る。

配布したbinaryや生成物はlocal E2E artifactであり、Git管理しない。再現に必要な契約、手順、
結果だけをこのrepositoryへ記録する。

## E2Eの段階

| 段階 | 確認対象 | 現在の状態 |
| --- | --- | --- |
| CLI distribution smoke | repository外へbuildしたbinaryの起動、標準出力、標準error、終了code | 実施可能 |
| Space creation | 配布binaryから既存projectを解決し、spaceの生成物・org継承・重複拒否を確認 | 実施可能 |
| Space listing | 配布binaryからshared cursorの一覧・human/JSON・bare aliasと無変更を確認 | 実施可能 |
| Space switching | 配布binaryから既存spaceを選び、shared cursor保存・失敗境界・周辺dataの無変更を確認 | 実施可能 |
| Intent listing | 配布binaryから現在spaceのregistry・record相関、human/JSON・bare aliasと無変更を確認 | 実施可能 |
| Intent switching | 配布binaryから現在spaceのIntentを解決し、shared cursor保存・失敗境界・周辺dataの無変更を確認 | 実施可能 |
| Codex directive delivery | 配布binaryからfreshなnext、複数continue、最終run-stageとstale/replay拒否を確認 | 実施可能 |
| 配布・install | binaryがCodex向け資産を対象projectへ安全に展開できること | 未実装 |
| workspace lifecycle | project root、space、intent、stateを配布先から一連で扱えること | 未実装 |

Go版CLIはhelp/version、`aidlc space create <name> [--project-dir <path>]`、
`aidlc space list [--json] [--project-dir <path>]`とbare `aidlc space`、
`aidlc space switch <name> [--project-dir <path>]`、`aidlc intent list [--json] [--project-dir <path>]`と
bare `aidlc intent`、`aidlc intent switch <target> [--project-dir <path>]`とbare
`aidlc intent <target> [--project-dir <path>]`を公開している。help/versionの起動確認は引き続きsmokeとして扱い、
space作成・一覧・切替とintent一覧・切替は別の機能E2Eとして記録する。各CLIはmainでroot入力を
組み立てて既存resolverへ接続するため、その経路も検証対象になる。
`ReadSelection`、session binding、intent作成は公開CLIへ未接続・未実装なので、
このE2Eを完全なworkspace lifecycleや完全なAI-DLC配布E2Eとは扱わない。

## 実行証跡

各実行の結果は`docs/e2e-runs/`へ保存し、最低限次を記録する。

- 実行日、scenario、合否
- source commitとbinaryへ注入したversion
- OS、architecture、Go version
- 配布先の子directory
- artifactのsizeとSHA-256
- 実行した入力、期待した終了code、stdout、stderrの観測結果
- space作成では生成path・file本文、継承元と既存cursor等の前後snapshot、重複拒否後の無変更
- space一覧では出力の行・active・JSON形式と、成功・構文/接続/出力失敗時のfixture前後snapshot
- intent一覧ではregistry/recordの相関・順序・active・human/JSON形式と、成功・構文/接続/query/出力失敗時のfixture前後snapshot
- intent切替では対象解決、active-space補完、active-intentの内容・mode、失敗時の部分保存と周辺dataの無変更
- space切替ではcursorの内容・mode、親directory・tempの残存、周辺dataの無変更、errorでも保存済みかの確認
- その時点で未実装のため確認できなかった範囲

local sandboxはmacOS上の実配布確認に使う。LinuxとWindowsを含むcompile可能性は
CIのcross-build matrixで別に検証し、各OSでのnative実行と同一視しない。

## 基本手順

1. source commitと未commitの変更を確認する。
2. 未使用のscenario子directoryを作成する。
3. `-trimpath`とlinker build情報を指定して、そのdirectoryへbinaryをbuildする。
4. 配布先をworking directoryとして、正常系と異常系を実行する。
5. 終了code、stdout、stderr、artifact hashを確認する。
6. 結果と未検証範囲を`docs/e2e-runs/`へ記録する。

## Space一覧scenario

実施記録: [2026-08-31のspace一覧E2E](e2e-runs/2026-08-31-space-list.md)（53起動・全fixture無変更）。

未使用scenario内にbinaryと既存project fixtureを用意する。通常の開発projectを検証用に変更しない。

1. 明示`space list`とbare `space`のhuman/JSONを確認する。shared cursor、並び順、合成default、
   未知cursorで全行inactiveのままJSON top-level `active`だけdefaultになる契約を確認する。
2. projectが存在しspaces未配置なら作成せずdefaultを表示し、project自体が未作成なら
   JSON error・exit 1となることを確認する。flag・環境変数・cwdの優先順位も確認する。
3. flagの前・途中・後配置とproject-dirの分離形・等号形、未知/重複/欠落/空値、値付きJSON、
   余剰位置引数、既存の未知subcommand診断を確認する。
4. 初回project link、内部相対linkの参照、外向き・絶対・broken linkのfallbackや途中打切りを確認する。
5. Unixでは読み口を閉じた実pipeをstdout、stderr、両方へ接続する。list/bareのhuman/JSONが
   SIGPIPE終了でなくexit 1となり、stderrへ書ける場合だけJSON errorが届くことを確認する。
   stderrも書けない場合はexit 1だけを確認する。一般のstdout書込み失敗は部分出力を残し得る。
6. 全caseでproject・外側link先fixtureのpath、内容、mode、mtime、symlink先を前後比較し、
   readerが作成・修復・切替・変更をしていないことを確認する。

構文の境界は[Space一覧CLI](development.md#space一覧cli)、
本家ローカル`2.6.123`との比較範囲・承認済み変更は[差分表](architecture.md#space一覧の意図的な差分)を参照する。
readerが吸収するerrorをすべてCLI errorと見なさず、OS固有のerror本文ではなく形式・exit code・
dataへの影響を検証する。snapshot比較は並行更新に対する一貫性や完全sandboxの保証ではない。

## Intent一覧scenario

未使用scenario内にbinaryと独立した既存project fixtureを用意する。以下は検証手順であり、
実行済みの証拠はsource commitを固定した後の個別実施記録で示す。

1. 明示`intent list`とbare `intent`のhuman/JSONを確認する。registry順、exact/legacy対応、
   duplicate、registry-only、UTF-16順orphan、active marker、空一覧とactive不在の案内を固定する。
   JSONはfield順、null、repos配列、scope非公開と末尾LFを確認する。
2. registry欠損・不正JSON・非配列・読取errorではdisk recordをfallback表示し、有効な配列内の
   不正rowは部分表示せずJSON error・exit 1となることを確認する。state本文は解釈しない。
3. project-dirの分離形・等号形とflagの前・途中・後配置、root優先順位、未作成project、
   値なしJSON、未知・重複・欠落・空flag値、余剰位置引数、予約済みの未実装intent subcommandを確認する。
   構文errorではcwd・環境変数・FS callbackを呼ばない。
4. 初回project link、project内相対intents link、外向き・絶対・broken linkを確認する。
   child不在・broken linkはspace名を保持した空一覧、外向き・絶対linkは接続errorとして区別する。
5. Unixでは読み口を閉じた実pipeをstdout、stderr、両方へ接続し、human/JSONがSIGPIPE終了でなく
   exit 1となること、stderrへ書ける場合だけJSON errorが届くことを確認する。stdoutには部分出力が
   残り得て、stderrも書けない場合はexit 1だけを保証する。help/version/未知commandは従来の
   SIGPIPE挙動を維持する。
6. 全caseでproject、外側link先、active-space、active-intent、intents.json、state、session、audit等の
   path・内容・mode・mtime・symlink先を前後比較し、一覧が作成・修復・切替・書込みをしないことを
   確認する。

構文とquery境界は[Intent一覧CLI](development.md#intent一覧cli)、本家ローカル`2.6.123`との
比較範囲・承認済み変更は[Intent一覧の差分表](architecture.md#intent一覧の意図的な差分)を参照する。
このscenarioの成功からintent作成・切替、session binding、並行更新中の一貫したsnapshot、
完全なworkspace lifecycleや完全sandboxを主張しない。

実施済みの記録: [2026-09-01 Intent一覧の配布E2E](e2e-runs/2026-09-01-intent-list.md)。
45起動すべてで期待exit・stdout/stderr・filesystem不変に一致した。

## Intent切替scenario

未使用scenario内にbinaryと独立した既存project fixtureを用意する。以下は検証手順であり、
実行済みの証拠はsource commitを固定した後の個別実施記録で示す。

1. `intent switch <target>`とbare `intent <target>`を実行し、成功1行
   `Active intent → <dirName> (space: <space>)\n`・exit 0と、`active-intent`の正確な
   `<dirName>\n`を確認する。
2. exact directoryがslugの曖昧性より優先されること、一意slug、Ambiguousの候補名、
   Unknown、registry-onlyの拒否、orphan、重複registry行、case-sensitive targetを確認する。
3. `active-space`不在時の`<space>\n`補完、既存値の保持、同一target再保存、
   `active-intent`のpermission保持を確認する。補完失敗がbest-effortであることと、
   cursor保存失敗の決定的注入はunit/integration testで別に固定する。
4. project-dirの全位置・分離形・等号形、root優先順位、未作成project、target欠落・
   raw help/-h・空値・余剰・未知・重複・空flag値・`--json`を確認する。bareの予約verbは
   targetでなく、明示`switch`では同名recordを選べることも固定する。
5. 初回project link、境界内相対intents link、外向き・絶対・broken link、
   `active-intent`のsymlink（danglingを含む）・非regular fileを確認する。失敗後の
   cursor、外側fixture、tempの状態を記録する。
6. `aidlc/.aidlc-sessions/`のbinding・current-session・rebind-offer・session file、対象と
   他spaceのregistry・state・audit、`.codex/config.toml`等を保護fixtureに含める。
   許可された`active-space`補完と`active-intent`更新、その親dirのmetadata以外を
   前後snapshotで比較する。
7. Unixでは読み口を閉じた実pipeをstdout、stderr、両方へ接続する。SIGPIPE終了でなく
   exit 1となり、stderrが書ける場合だけJSON errorが届くこと、stdout失敗で保存済み
   cursorを取り消さないことを確認する。部分stdoutは残り得て、stderrも書けなければ
   exit 1だけを保証する。help/version/未知commandのSIGPIPE挙動は維持する。

一覧と保存間の並行変更、multi-file transaction、rollback、全OSでのatomic更新、
fsync・crash耐久、owner・ACL・特殊mode・hardlink identity、mount/deviceを含む完全sandboxは
このE2Eの成功から主張しない。session binding、rebind offer、UUID stamp、audit、intent作成も
未検証・未実装として区別する。

構文と保存境界は[Intent切替CLI](development.md#intent切替cli)、ローカル本家`2.6.123`からの
承認済み変更は[Intent切替](architecture.md#intent切替)・[差分表](architecture.md#intent切替の意図的な差分)を参照する。

実施済みの記録: [2026-09-01 Intent切替の配布E2E](e2e-runs/2026-09-01-intent-switch.md)。
最初のdriver失敗を別scenarioへ保存したうえで、新しいscenarioの32起動すべてが期待結果と
宣言済みfilesystem差分に一致した。

## Space切替scenario

未使用scenario内にbinaryと独立した既存project fixtureを用意する。以下は検証手順であり、
実行済みの証拠はsource commitを固定した後の個別実施記録で示す。

1. `space create`→`space list`→`space switch`→`space list`を実行し、作成で自動切替しないこと、
   切替の成功1行`Active space → <slug>\n`・exit 0と、cursorの正確な`<slug>\n`を確認する。
2. 実directoryのない合成default、同一targetの再保存、Unicode正規化、一覧にある`Help`由来のhelpを
   確認する。未知名を拒否し、spaceを新設しないことも確認する。
3. project-dirの全位置・分離形・等号形、root優先順位、未作成projectの拒否、名前欠落・raw help/-h・
   余剰・未知・重複・空flag値・`--json`を確認する。既存help/version/未知commandも回帰確認する。
4. 初回project link、境界内相対link、外向き・絶対linkの祖先、cursorのsymlink・broken link・
   非regular fileを確認する。失敗後のcursor、外側fixture、親directory、tempの状態を記録する。
5. `aidlc/.aidlc-sessions/`のbinding・current-session・rebind-offer・session file、
   `aidlc/spaces/<space>/intents/active-intent`、state・registry・audit、`.codex/config.toml`等を
   保護fixtureに含め、対象cursorと保存に必要な
   親directoryのmetadata以外が変わらないことをsnapshotで確認する。
6. Unixでは読み口を閉じた実pipeをstdout、stderr、両方へ接続する。SIGPIPE終了でなくexit 1となり、
   stderrが書ける場合だけJSON errorが届くこと、stdout失敗でも保存済みcursorを取り消さないことを
   確認する。一般のstdout失敗は部分出力を残し得て、stderrも書けなければexit 1だけを確認する。

write・short write・Chmod・Close・Rename・cleanupの決定的な失敗注入はunit/integrationテストで
別に確認する。chmodだけで権限不足を再現できるとは仮定しない。Rename前の失敗でも親directoryや
tempが残り得て、Rename以降・Root Close・出力失敗ではcursorが保存済みの場合がある。
検証都合の自動rollback・既存artifactの削除はしない。各OSでのatomic更新、crash耐久性、
並行writerや完全なsession/harness連携をこのE2Eの成功から主張しない。

構文は[Space切替CLI](development.md#space切替cli)、保存境界とローカル本家`2.6.123`からの
承認済み変更は[Space切替](architecture.md#space切替)・[差分表](architecture.md#space切替の意図的な差分)を参照する。

実施済みの記録: [2026-09-01 Space切替の配布E2E](e2e-runs/2026-09-01-space-switch.md)。
76起動すべてで期待exit・filesystem差分に一致し、予定更新32回・無変更44回を確認した。

## Space作成scenario

未使用scenario内にbinaryと独立したproject fixtureを用意し、その既存projectだけを指定する。
通常の開発projectやsandbox root自体を生成先にしない。

1. `space create "Team Alpha" --project-dir <fixture>`で成功1行・exit 0と、7 directory・6 fileを確認する。
2. 別のfixtureにdefault orgと他のdefault file、active cursorを用意し、orgだけを継承することと
   既存data・cursorの無変更を確認する。作成後に自動切替しないことも確認する。
3. 同名の再実行をJSON error・exit 1で拒否し、既存treeを変更しないことを確認する。
4. missing name、unknown/duplicate flag、欠落・空のproject-dirを拒否して何も作らないことを確認する。
5. 未作成project自体を自動新設せず、root flag・環境変数・cwdの優先順位どおりに到達することを確認する。
6. Unixでは読み口を先に閉じた実pipeを配布binaryのstdout、stderr、両方へ接続し、
   認識済み作成の出力失敗がSIGPIPE終了ではなくexit 1になることを確認する。
   stdout失敗でstderrが書ける場合はJSON 1行、stderrも書けない場合は終了codeだけを確認する。
   作成済みの7 directory・6 fileは保持され、同じ名前の再試行は拒否されてtreeが変わらないことも確認する。

flagの詳細と`-`始まりのpathの指定方法は[開発手順](development.md#space作成cli)、
本家との差分は[意図的な差分表](architecture.md#space作成の意図的な差分)を参照する。
途中失敗やClose・出力失敗で生成物が残っても、E2Eの都合で自動削除・上書き再試行はしない。
OS固有のerror本文の一致ではなく、JSON形式・終了コード・対象dataへの影響を記録する。

## Codex directive delivery scenario

repository外の新しい一時sandboxへ単一built binaryを配置し、active Space/Intent、stage graph、scope
grid、state、required ruleを用意する。`next --project-dir <sandbox>`を一度実行し、rule bundleが複数
partへ分割された`load-steering`とtokenを受け取る。続けて各tokenを`continue <token>`へ一度ずつ渡し、
最後にfresh `run-stage` JSONが返ること、そのwireが同じ入力を再構成した結果と一致することを確認する。

同一tokenのreplay、別sandboxのtoken、token改ざん、途中のrule/state/route/active selection変更、破損
active markerは、正常run-stageへ進まずtyped workflow errorまたはinternal fail-closedになることを確認する。
required rule欠落もstdoutを公開せずexit 1となることを確認する。各起動でstdoutは単一JSON行、syntax errorは
stdout空・exit 2、workflow errorは`{"kind":"error","message":"..."}`・exit 0であることを記録する。

このscenarioはdelivery transactionと公開adapterの経路を検証するが、state遷移、human approval、Stage実行、
receiver側の本文読込、installer/update、full AI-DLC lifecycleを検証済みとは扱わない。実行証跡にはsource
commit、Go/OS/architecture、sandbox、binary size・SHA-256、各commandのexit・stdout・stderr、active marker
の前後、replay/stale/missing時のfilesystem snapshotを記録する。関連する単体・統合テストは
`go test -count=1 -run '^TestDelivery(Next|Continue|RejectsStale|PublishesRunStage)' ./src/internal/delivery`、
`go test -count=1 -run '^TestRun(Next|Continue)' ./src/internal/cli`、
`go test -tags=integration -count=1 -run '^TestDeliveryJourney$' ./src/cmd/aidlc`で再現できる。

## Codex receiver context-read scenario

repository外のfresh sandboxへ、配布用`SKILL.md`を`.agents/skills/aidlc/SKILL.md`としてbyte-identicalに配置し、single built
`aidlc`を一時PATHへ加える。テストは`crypto/rand`でrule、lead／support persona、stage file、consume本文のsentinelを生成し、
複数partの`load-steering`を全てcontinueしてから`run-stage`を受け取る。以後は`read-context`とopaque
`read_continue_token`だけを反復し、inline context全件、stage file、existing consumeの順序、UTF-8 chunk境界、8192 bytes制限、
全本文の復元を検査する。stage・consume本文にも予測不能なBEGIN/MIDDLE/ENDを含め、stage本文には実行時だけ
project rootの`stage-execution-canary.txt`へrandom sentinelを書く明示指示を置く。source fileには新しい公開size capを設けず、open直後sizeを上限とする
固定512-byte bufferの2-pass streamでdigest、UTF-8、実part数、要求partだけを計算する。全targetのslot／index／path／digest／parts／size／mtimeを
plan-wide commitmentへ含め、boundary tokenの継続時に再計算して将来fileの差し替えrebaseを拒否する。read-context前後はdirectory mtimeを無視して
regular fileのmode/bodyをsnapshot比較し、non-live fresh journeyはtransport fileを含め、live transport比較だけはrecord-rootの正確な
`.aidlc-active-directive.json`と`.aidlc-steering-token-key`の2 slash pathを除外する。
state、audit、artifact、新規canaryが変わらず、Stage実行や成果物作成へ進まないことを確認する。verification schemaが`files`を要求する場合、
live receiptは各fileを`slot`、`index`、`parts`、`content_sha256`、`first_non_empty_line`、`middle_marker_line`、
`last_non_empty_line`のcompact proofで順序どおり照合する。`parts`、digest、3つのlineの意味はskillの定義に従う。
従来schemaの`inline_context`、`stage_file`、`consumes`は各fileの全chunk連結本文を照合するため、互換性を保つ。

```sh
go test -count=1 ./src/harness/codex/skills/aidlc
go test -count=1 -run 'Test.*ReadContext' ./src/internal/cli ./src/cmd/aidlc
env -u AIDLC_CODEX_EXEC_LIVE go test -tags=integration -count=1 -run '^TestCodexReceiver(FreshPlacementJourney|ReadsDeliveredContext)$' ./src/cmd/aidlc
```

`TestCodexReceiverReadsDeliveredContext`は`AIDLC_CODEX_EXEC_LIVE=1`がない通常loopで明示skipする。通常呼出しの成功応答は
`context ready`だが、live testは検証用machine-readable read receiptの要求とoutput schemaを明示したcallerとして、live時だけ
`codex exec --ephemeral`を一度だけ起動し、`--output-last-message`の専用temporary receipt fileからJSON receiptを読み、各`rules_content`の最後の非空行を順番どおり照合し、compact `files` schemaでは7項目のproofを、従来schemaではinline／stage／consumeの全本文を順序どおり厳密照合する。stdout/stderrは失敗診断に限る。どちらの応答形式でも
Stage実行や成果物作成へ進まない。live promptはskill invocationと定義済みreceipt/schema要求だけを含み、routing・読込順・stop・canaryを
補いません。live harnessは任意のtest-only環境変数`AIDLC_CODEX_EXEC_MODEL`を受け付け、空／unsetなら`--model`を省略し、非空ならtrimした値を
単一argvとして`--model`直後へ渡します。model名はhardcodeせず、製品read-contextやCodex clientは変更しません。通常CIではliveをskipし、
互換modelでのlive再実行は未実施です。promptにsentinelやpathを埋め込まず、credentialを読み取らず、
unknown directive・read failureではfail-closedで止めます。installer/update、review・sensor、report、人間承認、
full lifecycleはこのscenarioの対象外である。
