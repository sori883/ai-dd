# Bundle構造と適合条件

## 要点

OKFのKnowledge Bundleは、Markdownファイルを階層的に格納した1つのディレクトリである。
BundleはGit repository、archive、または大きなrepository内のsubdirectoryとして配布できる。

Bundle内の`index.md`と`log.md`以外の`.md`ファイルは、すべてConcept文書として扱われる。
したがって、Bundle rootをどこに置くかは単なる整理上の名前ではなく、適合性検査の範囲を決める契約になる。

## Concept

Conceptは次の2要素を持つUTF-8 Markdownファイルである。

1. ファイル先頭のYAML front matter
2. その後に続くMarkdown本文

Concept IDは、Bundle rootからの相対パスから`.md`を除いた値である。
例えば`tables/orders.md`のConcept IDは`tables/orders`になる。

front matterで常に必須なのは、空でない`type`だけである。
`title`、`description`、`resource`、`tags`などは任意であり、
利用目的に応じて追加する。

## 予約ファイル

次の名前はConcept文書には使えない。

| ファイル | 役割 |
| --- | --- |
| `index.md` | その階層で利用できるConceptを先に把握するための案内 |
| `log.md` | その階層に対する変更履歴 |

Bundle rootの`index.md`だけは、対象版を示す
`okf_version: "0.2"`をfront matterに持てる。それ以外の`index.md`はfront matterを持たない。

## v0.2適合条件

BundleがOKF v0.2へ適合するための必須条件は次の3点である。

1. 予約ファイル以外の全Markdownに解析可能なYAML front matterがある。
2. 各front matterに空でない`type`がある。
3. 存在する`index.md`と`log.md`が仕様の構造に従う。

一方、consumerは次を理由にBundle全体を拒否してはならない。

- 任意metadataがない。
- 未知の`type`や拡張fieldがある。
- リンク先が存在しない。
- `index.md`が存在しない。

producerが拡張fieldを追加することは認められているが、consumerは未知fieldを許容し、
round-tripする場合は保持することが推奨される。

## AI-DLCへ適用する際の意味

既存の`knowledge/`をそのままBundle rootとみなすと、その配下にあるすべてのMarkdownが
OKF適合性検査の対象になる。既存AI-DLCには複数のknowledge領域とDocumentKBがあるため、
実行時のBundle rootは各領域の所有者と既存ファイルを確認してから決定する。

`okf`という名前のdirectoryを設けることは仕様要件ではない。必要なのは、consumerが
検証・検索対象となるBundle rootを曖昧さなく特定できることである。

## 仕様根拠

- [§2 Terminology](upstream/SPEC-v0.2.md#2-terminology)
- [§3 Bundle structure](upstream/SPEC-v0.2.md#3-bundle-structure)
- [§4 Concept documents](upstream/SPEC-v0.2.md#4-concept-documents)
- [§11 Conformance](upstream/SPEC-v0.2.md#11-conformance)
- [§12 Versioning](upstream/SPEC-v0.2.md#12-versioning)
