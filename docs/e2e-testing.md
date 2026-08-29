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
| 配布・install | binaryがCodex向け資産を対象projectへ安全に展開できること | 未実装 |
| workspace lifecycle | project root、space、intent、stateを配布先から一連で扱えること | 未実装 |

現時点のGo版CLIが公開しているのはhelpとversionだけである。このため、現在のE2Eを
「完全なAI-DLC配布E2E」とは呼ばず、「CLI distribution smoke E2E」として扱う。
project root resolverも公開CLIへ未接続なので、root解決はこのE2Eの対象外である。

## 実行証跡

各実行の結果は`docs/e2e-runs/`へ保存し、最低限次を記録する。

- 実行日、scenario、合否
- source commitとbinaryへ注入したversion
- OS、architecture、Go version
- 配布先の子directory
- artifactのsizeとSHA-256
- 実行した入力、期待した終了code、stdout、stderrの観測結果
- その時点で未実装のため確認できなかった範囲

local sandboxはmacOS上の実配布確認に使う。LinuxとWindowsを含むplatform互換性は、
引き続きCIのcross-build matrixで別に検証する。

## 基本手順

1. source commitと未commitの変更を確認する。
2. 未使用のscenario子directoryを作成する。
3. `-trimpath`とlinker build情報を指定して、そのdirectoryへbinaryをbuildする。
4. 配布先をworking directoryとして、正常系と異常系を実行する。
5. 終了code、stdout、stderr、artifact hashを確認する。
6. 結果と未検証範囲を`docs/e2e-runs/`へ記録する。
