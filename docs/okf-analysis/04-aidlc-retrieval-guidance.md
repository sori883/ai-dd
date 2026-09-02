# AI-DLCでの検索とcontext制御

## この文書の位置付け

この文書は、OKF v0.2をAI-DLCへ適用するための設計整理であり、OKFの規範仕様ではない。
合意済み判断と未解決の推奨を分けて記載する。

## 本家AI-DLCのknowledge領域

ローカルで参照している本家AI-DLC v2.6.123には、少なくとも次の異なる領域がある。

| 領域 | 所有者と役割 |
| --- | --- |
| `.codex/knowledge/` | 配布物に含まれるAI-DLC方法論。共通知識とagent別知識 |
| `aidlc/spaces/<space>/knowledge/` | 利用teamが管理するspace固有のdomain知識 |
| `knowledge/documents/` | DocumentKBへ渡す利用team所有の原本 |
| `knowledge/documentkb/` | 原本から作られるtool所有の派生catalog |

`.codex/knowledge/`には`aidlc-shared/`とagent別directoryが配布される。
Space固有knowledgeにも、teamが作成した`aidlc-shared/`と`<agent>/`があれば本家実装が読み込む。
同じ名前でも、前者はframework知識、後者は利用project固有知識である。

現時点では、どの領域をOKF化するか、どのdirectoryを1つのBundle rootとみなすかは未決定である。
`knowledge/okf/`や`knowledge/bundles/`という特定の配置も、この参照基盤では採用しない。

## 決定的なmetadata検索

合意済みの初期方針は、LLMに全ファイルを読ませて選ばせるのではなく、
programがfront matterを走査して候補を決定することである。

次は後続計画で検討する未承認の設計候補である。検索対象field、検証失敗時の扱い、
順位と同点規則、本文選択方法は、この参照基盤では確定しない。

1. 明示されたBundle rootだけを列挙する。
2. Concept文書のfront matterを読み、Concept IDと標準metadataをメモリ上へ構築する。
3. 必須fieldと値を検証し、不正な文書をpathと理由付きで報告する。
4. IntentやStageから渡された条件を、`type`、`title`、`description`、
   `resource`、`tags`、lifecycle・trust signalへ照合する。
5. 同点時のpath順などを定義し、同じ入力から同じ順位を得る。
6. 上位候補のうちcontext上限に入るConcept本文だけを読み込む。

OKF文書にはAI-DLC固有metadataを追加しない。Stageやagentとの対応が必要なら、
検索条件を作る側の定義またはprofileで管理する。

## Context上限

初期実装では、モデル固有tokenizerに依存せず、次の2つを独立した設定値として持つ方針である。

- 注入するConcept本文の最大文書数
- 注入する本文のUTF-8合計byte数

正確なデフォルト値は未決定である。候補metadataの最大件数、本文の最大文書数、
合計byte数をfixtureで計測し、実際に必要な回答が欠落しない最小値を選ぶ。

選択したConcept ID、順位の理由、本文byte数、除外理由を診断可能にすることや、
上限超過時に本文を途中切断せずmetadataと超過理由を返すことは、後続計画の候補である。
出力契約としてはまだ承認されていない。tokenizer基準の制限も、使用modelと必要性が
明確になった後に追加検討する。

## メモリ上の検索index

初期段階ではCLI起動ごとにfront matterを走査し、検索用entryをprocess memoryへ作り、
終了時に破棄する。これはOKFの`index.md`とは別の内部data structureである。

永続cacheには、変更検出、無効化、atomic write、破損回復、複数processの競合という
追加契約が必要になる。小規模・中規模Bundleで走査が十分速い間は導入せず、
benchmarkで必要性が確認された後に設計する。

## Stage構成と依存検証

Stageの追加、除外、置換はIntent開始時だけ行う。Stage実行中に構成を変更する機能は作らない。
依存関係の検証もIntent開始前に行い、必要な入力成果物を供給するStageまたは既存成果物が
存在しない場合は、欠けている入力と修正候補を示して開始を止める。

ツールが利用者へ知らせずにStageを再追加することはしない。構成への人間の確認が必要な場合は、
実行中の置換承認ではなく、Intent開始時の構成確定として扱う。

## 本家との意図的な差分候補

本家v2.6.123は、active Spaceの利用者・team knowledgeをpath単位で読み込み、
最小depthでも対象となる各knowledge fileをcontextへ含める。

将来、OKF front matterによる検索後に必要な本文だけを注入する場合、これは
context量と文書選択が変わる意図的な挙動差になる。採用時には、既存knowledgeの互換性、
検索から漏れた文書の扱い、利用者が選択結果を確認する方法を計画で提示し、
改めて承認を得る。今回の参照基盤は実行時挙動を変更しない。

## 主要根拠

- [OKF v0.2 §1 Non-goals](upstream/SPEC-v0.2.md#non-goals)
- [OKF v0.2 §4.1 Frontmatter](upstream/SPEC-v0.2.md#41-frontmatter)
- [OKF v0.2 §8 Index files](upstream/SPEC-v0.2.md#8-index-files)
- [AI-DLC v2.6.123のknowledge配置説明](../配布_ai-dlc/AGENTS.md)
- `docs/実装_aidlc-workflows/core/tools/aidlc-orchestrate.ts`
- `docs/実装_aidlc-workflows/core/tools/aidlc-knowledge.ts`
