# OKF v0.2参照基盤と初期統合境界

- 日付: 2026-09-03
- 状態: Accepted
- 対応Issue: [#53](https://github.com/sori883/ai-dd/issues/53)

## 背景

AI-DLCのknowledge検索へOpen Knowledge Format（OKF）を採用する検討にあたり、
公式repositoryや本家AI-DLCの大規模な実装を毎回読み込むとcontextを過剰に消費する。
また、配布済みframework知識と利用projectのSpace固有知識を同じ保存領域として扱うと、
OKF Bundleの境界と既存ファイルの所有関係が曖昧になる。

## 決定

1. 参照対象をOKF v0.2へ固定する。
2. 公式仕様とlicenseをcommit SHA付きでローカルへ保存し、短い分析索引から必要な資料だけを読む
   `okf-reference` skillを作る。
3. metadata検索は将来、programによる決定的な処理として実装する。
4. OKF Conceptへ`aidlc:` namespaceや`phase`、`stage`、`agent`などの
   AI-DLC固有metadataを追加しない。必要な対応は検索条件を生成する側で管理する。
5. 初期の検索indexはCLI processのメモリ上に構築し、永続cacheは性能計測で必要性が確認された
   後に検討する。
6. context制御は最大文書数とUTF-8合計byte数の両方を使う。正確なデフォルト値はfixtureと
   実測を根拠に後続計画で決める。
7. Stageの追加、除外、置換はIntent開始時だけ行う。Stage実行中に構成を変更しない。
   依存関係の検証もIntent開始前に行う。

## Knowledge領域の区別

- `.codex/knowledge/`: 配布物に含まれるAI-DLC方法論のframework知識。
- `aidlc/spaces/<space>/knowledge/`: 利用teamが管理するSpace固有知識。
- Space内の`knowledge/documents/`: DocumentKBへ渡す利用team所有の原本。
- Space内の`knowledge/documentkb/`: toolが原本から作る派生catalog。

`.codex/knowledge/`とSpace固有knowledgeの両方に、共通用`aidlc-shared/`とagent別directoryが
存在し得る。同名でも所有者と役割が異なる。

## 今回確定しない事項

- どのknowledge領域をOKF化するか。
- 実行時に1つのOKF Bundleとみなすdirectory root。
- 文書数とbyte数の具体的なdefault。
- Goで完全なYAMLを解析するためのmodule。

これらは実行時挙動、互換性、依存関係へ影響するため、後続の実装計画で提示して承認を得る。
`knowledge/okf/`や`knowledge/bundles/`は現時点の決定ではない。

## 本家AI-DLCとの差分

この決定による実行時差分はない。今回作るのは参照資料と読み取り専用skillである。

将来、front matter検索により必要な本文だけをcontextへ注入する場合、本家v2.6.123の
path単位のknowledge読込とは意図的な挙動差になる。その計画では、既存knowledgeとの互換性と
利用者への影響を改めて提示する。

## 根拠

- [OKF v0.2参照インデックス](../../okf-analysis/README.md)
- [固定したOKF v0.2仕様](../../okf-analysis/upstream/SPEC-v0.2.md)
- [取得記録](../../okf-analysis/upstream/SOURCE.md)
- `docs/配布_ai-dlc/AGENTS.md`のKnowledge、Team Knowledge、DocumentKBの説明
- `docs/実装_aidlc-workflows/core/tools/aidlc-orchestrate.ts`のknowledge読込処理
- `docs/実装_aidlc-workflows/core/tools/aidlc-knowledge.ts`のDocumentKB所有境界
