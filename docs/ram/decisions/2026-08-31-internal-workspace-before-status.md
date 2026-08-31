# 内部workspace機能を先行し、statusを後段で実装する

- 日付: 2026-08-31
- 状態: Accepted（実装順序の方針。次のspace実装の詳細計画は承認待ち）
- 関連記録: [初期実装の境界](2026-08-29-initial-implementation-boundaries.md)、[Project root解決の初期契約](2026-08-30-project-root-resolution.md)

## 背景

project root解決の実装後、次の公開機能として`status`への接続を検討した。
しかし、参照対象のAI-DLC v2.6.123の`status`は、space、intent、state、graphなどに
基づく進捗表示であり、rootだけの表示ではその契約を満たさない。

その代案として提案した`aidlc workspace root`も、本家には存在しない追加コマンドだった。
ユーザーから、表示用の追加コマンドを先に作るのではなく、必要な内部機能を整えてから
`status`を実装すればよい、との提案があり、この進め方で合意した。

## 決定

1. root解決は内部機能のまま維持する。
2. rootだけを表示する暫定的な`status`は実装しない。
3. root解決を公開するためだけの独自`aidlc workspace root`コマンドは追加しない。
4. rootの次はspace、続いてintentなどの内部機能を小さな単位で調査・計画・実装する。
5. `status`は、参照実装相当の表示に必要な依存が揃った段階で、出力と終了コードを
   明確にして実装する。必要な依存の範囲も、その計画時に確認する。
6. 各実装単位は詳細計画の明示承認とGitHub Issueを必要とする。
   今回の順序の合意を、space以降の具体的な仕様や実装の包括承認とは扱わない。

## 既存記録との関係

[Project root解決の初期契約](2026-08-30-project-root-resolution.md)の決定1〜7は維持する。
同記録の未解決事項「`status`からproject rootをどの形式で表示するか」は、本記録により
必須の検討事項ではなくなる。`status`にrootを表示すること自体を前提にしない。
過去の記録は経緯として残す。

## 影響・次の検討

- 次は共通space読み取りの最小範囲と、その互換性テストを計画する。
- spaceの作成・切替、intent選択、session bindingなどを一括で実装しない。
- 公開CLIに接続していない機能は単体テスト等で検証する。既存のCLI smoke testを、
  その内部機能を通したE2Eの証拠とは扱わない。
- この記録は実装順序の合意だけを保存する。spaceのAPI、異常系、filesystem境界は
  別の実装計画で確認する。

## 根拠

- ユーザーの提案: 他の必要な機能を先に作り、その後に`status`を実装する。
- 続くユーザー回答: 「はい、ではお願いします。」
- 参照実装v2.6.123の`docs/実装_aidlc-workflows/core/tools/aidlc.ts`:
  `workspace`のverbは`detect`と`codekb`で、`root`はない。
- 同`core/tools/aidlc-utility.ts`の`handleStatus`と、
  同`core/tools/aidlc-lib.ts`の`resolveWorkflowSelection`。

## 後続の承認

同日の次の確認で、space読み取り2機能の詳細計画が明示承認された。
この記録の「承認待ち」は順序合意時点の状態として残し、具体的なAPIと保証範囲は
[共通space読み取りの初期契約](2026-08-31-space-reading-contract.md)を参照する。
