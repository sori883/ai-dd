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
| 配布・install | binaryがCodex向け資産を対象projectへ安全に展開できること | 未実装 |
| workspace lifecycle | project root、space、intent、stateを配布先から一連で扱えること | 未実装 |

Go版CLIはhelp/version、`aidlc space create <name> [--project-dir <path>]`、
`aidlc space list [--json] [--project-dir <path>]`とbare `aidlc space`を公開している。
help/versionの起動確認は引き続きsmokeとして扱い、space作成・一覧は別の機能E2Eとして記録する。
作成・一覧CLIはmainでroot入力を組み立てて既存resolverへ接続するため、その経路も検証対象になる。
`ReadSelection`、intent読み取りCLI、session binding、切替やintent作成は未接続・未実装なので、
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
