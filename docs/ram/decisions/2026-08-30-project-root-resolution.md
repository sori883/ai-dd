# Project root解決の初期契約

- 日付: 2026-08-30
- 状態: Accepted
- GitHub Issue: [#11](https://github.com/sori883/ai-dd/issues/11)

## 背景

space、intent、state等のpathは、AI-DLCが対象にするproject rootを基準に決まる。
大きなworkspace領域を段階的に実装するため、最初の単位として副作用のないproject root解決だけを切り出す。

## 決定

1. `src/internal/workspace` packageがproject root解決を所有する。
2. 候補は、明示指定、`AIDLC_PROJECT_DIR`、互換用`CLAUDE_PROJECT_DIR`、現在directoryの順に選択する。
3. 相対候補は現在directoryを基準に解決し、結果をcleanした絶対pathとして扱う。
4. packageはprocess環境や現在directoryを直接読まず、process境界から受け取った値だけで決定する。
5. 解決処理はfilesystemを参照または変更しない。対象の存在確認やancestor探索は行わない。
6. Go標準ライブラリだけを使用する。
7. この段階では公開CLIへ接続しない。`status`の出力と終了コードを別途確定してから利用する。

## 影響

- spaceとintentの読み取り機能は、同じroot契約の上へ追加できる。
- 単体テストはprocess-globalな環境変数やworking directoryを変更せず、OS固有path規則を検証できる。
- 既存AI-DLCのruntime treeからproject rootを逆算する処理は、単一バイナリ構成には持ち込まない。

## 未解決事項

- `status`からproject rootをどの形式で表示するか。
- CLI process境界がCodexからproject directoryを受け取る具体的な方法。
- 将来ancestor探索を追加する必要があるか。

## 根拠

- 2026-08-30のユーザー承認: project root解決を最初の実装単位とする。
- `docs/実装_aidlc-workflows/core/tools/aidlc-lib.ts`の`resolveProjectDir`。
- `docs/ram/decisions/2026-08-29-initial-implementation-boundaries.md`。
