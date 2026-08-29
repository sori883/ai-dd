# Local配布E2E sandboxの運用

- 日付: 2026-08-30
- 状態: Accepted

## 背景

単一binaryという配布要件は、repository内のunit testやcross buildだけでは十分に確認できない。
実際にrepository外へ配置し、配布先から利用者に見える挙動を確認する場所が必要になった。

## 決定

1. `~/sori883/haihu-aidlc`をAI-DLCのlocal配布・動作確認用sandboxとして使用してよい。
2. この環境での絶対pathは`/Users/const/sori883/haihu-aidlc`である。
3. 各実行は`sandbox/e2e/<YYYY-MM-DD>-<scenario>/`という新規の子directoryへ隔離する。
4. 配布後にそのdirectoryから動作確認する一連の検証をE2E testとして扱う。
5. 実行結果は`docs/e2e-runs/`へ記録し、artifact自体はGit管理しない。
6. sandbox root、既存scenario、無関係なfileを上書きしない。削除やclean upは対象を明示し、別途ユーザー承認を得る。
7. local sandboxでのmacOS native実行と、CIでのLinux、macOS、Windows検証は相互補完とし、一方で他方を代替しない。

## 現在の適用範囲

現行Go版はhelpとversionのCLI基盤だけを公開している。そのため、2026-08-30時点では
単一binaryのCLI distribution smoke E2Eだけを実施する。資産展開、project root、space、
intent、state等を含む完全な配布E2Eは、対応機能を公開CLIへ接続した段階でscenarioを追加する。

## 影響

- 今後の配布機能は、unit testとCIに加えて実際の配布境界を通す受け入れ確認を持てる。
- 実行証跡と外部artifactが分離され、repository sizeを増やさずに再現条件を追跡できる。
- sandboxに残る過去artifactを暗黙に消さないため、比較や調査に利用できる。

## 根拠

- 2026-08-30のユーザー指示: `~/sori883/haihu-aidlc`配下へfolderを作成してAI-DLCを配布し、配布後の動作確認をE2E testとして記録してよい。
- [配布E2Eテスト](../../e2e-testing.md)
- [初回実行結果](../../e2e-runs/2026-08-30-cli-bootstrap.md)
