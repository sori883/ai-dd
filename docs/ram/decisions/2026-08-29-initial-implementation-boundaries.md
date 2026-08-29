# 初期実装の境界

- 日付: 2026-08-29
- 状態: Accepted
- 対象: GoによるAI-DLC再実装

## 背景

既存AI-DLC v2は33ステージ、状態機械、監査、センサー、承認ゲート、plugin、
複数ハーネス向け配布を持つ。すべてを一括移植すると、互換性の確認と変更理由の追跡が
困難になるため、小さな振る舞い単位で段階的に実装する。

## 決定

1. 初期互換範囲は、ローカル参照スナップショットのAI-DLC `2.6.123`とCodex向け配布契約とする。
2. workspace、state、audit等の永続化形式は既存形式を維持する。新形式への暗黙的な変換は行わない。
3. Go実装は単一moduleのモジュラーモノリスとし、手動dependency injectionを維持する。
4. package境界は`workspace`、`graph`、`state`、`store`、`audit`、`orchestrator`、`cli`を基本候補とする。ただし空packageを一括作成せず、必要になった境界だけ追加する。
5. 大きな`workspace`領域は、project root探索、space、intentの順に小さく実装する。
6. TypeScript実装の行単位移植ではなく、既存版の入力、出力、終了コード、永続化結果をcharacterization testで固定してからGoで再現する。
7. `skills-lock.json`は本プロジェクトに不要と判断し、意図的に削除する。自動的に復元しない。
8. このプロジェクトの意思決定と調査結果は`docs/ram/`へ蓄積し、AI-DLC成果物側のknowledgeと分離する。

## 影響

- 最初の機能実装は、状態を書き換えないworkspace探索から開始できる。
- 既存形式との互換性が受け入れ条件になるため、正常系だけでなく欠損、破損、旧状態もfixtureで扱う必要がある。
- state更新を始める前に、Windowsを含むatomic write、排他、失敗時復旧を設計する必要がある。
- frontmatterやTOMLの完全な読み書きが必要になった場合、標準ライブラリだけで対応する範囲と外部Go module採用を別途判断する。

## 未解決事項

- Go版の実行ファイルへハーネス資産を埋め込むか、既存版と同様にruntime directoryを併置するか。
- 単一バイナリがハーネス資産のinstall/updateまで担当するか。
- 最初の公開コマンドを`status`とする場合の、AI-DLC `2.6.123`との正確な表示・終了コード契約。

## 根拠

- `docs/aidlc-analysis/README.md`
- `docs/aidlc-analysis/01-package-architecture.md`
- `docs/aidlc-analysis/03-apis-contracts.md`
- `docs/aidlc-analysis/07-technical-debt.md`
- `docs/architecture.md`
