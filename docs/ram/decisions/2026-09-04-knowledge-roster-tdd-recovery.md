# 知識一覧実装の手順不備と回復方針

- 日付: 2026-09-04
- 状態: Proposed（手順不備を記録して現実装を補強・採用する方針へのユーザー確認待ち）
- 対応Issue: [#93](https://github.com/sori883/ai-dd/issues/93)
- 関連計画: [工程・担当AIに応じて配置知識ファイルを選択する](2026-09-04-knowledge-roster-plan.md)

## 現状と背景

配置したMarkdownを毎回読み、工程・担当AIに応じて知識を選ぶ内部APIを実装した。
ただし、計画とgo-tdd skillが要求する「1つの振る舞いのテストを先に失敗させ、最小実装で成功させる」
手順を、大半の項目で満たしていなかった。最初のlead persona取得と、入力error時のzero Rosterへの
修正では実際のRED/GREENを観測したが、mode、DFS、preflight、plugin、Minimal、容量制限、
所有権の多くは実装後にテストを追加して初回から成功していた。

この履歴は後から作り直せない。完成コードを一時的に削除する検査を、実装前のREDと称さない。
対象はローカルbranch codex/knowledge-context-rosterにあり、PR作成・push・mergeはしていない。
利用者向け供給全体の完成でもない。

## 独立レビュー

base 6f33c82067298ba9732f4c3f828c2b79a9b15a46、
head e06421fa328f2a495d4b63a3e24d8cb6bb523dcaの固定差分を読み取り専用で確認した。

1. P1: Minimalのstage/owner表がない場合にも、無効pluginが所有する文書を省略する。
   本家2.6.123は表がなければ全保持する。表の有無をplugin判定より先に確認する必要がある。
2. P1: plugin metadataの警告を最後に生成し、全directory警告をfile preflightより先に集めている。
   本家はplugin警告を先頭、その後はpersonaとDFSの各読込順で警告する。
   警告の6 KiB上限に残る内容も変わるため、列挙時の逐次preflightへ修正する必要がある。
3. P1: 正常系unit testのFrameworkDirがPOSIX固定で、Windowsのnative絶対path検証に失敗する。
   成功系fixtureはnative絶対pathへ変更する必要がある。
4. P2（受入条件不足）: path/warning容量の上限一致・1 byte超過・正確な省略件数・特殊文字境界、
   SpaceKnowledgeの実Root借用、同一実Root上の本文編集を含むテストが不足している。

レビューで実行したknowledge packageの限定unit testとintegration testは成功したが、
上記の未検証条件を満たす証拠ではない。独立reviewはfinalへ進めないとの判断であり、
全体finalとGitHub CIは開始していない。

## 確認を求めた回復方針

手順不備を明示したまま現コードを保持し、契約違反を修正し、不足テスト・再レビュー・全体検証で
補強して採用する方針をユーザーへ確認した。返答を得るまでは採用・mergeを行わない。
既存の機能範囲や本家準拠の承認を取り消すものではなく、TDD手順の不備を免除済みとは扱わない。

承認を得た場合も、上記の不具合は修正前に回帰テストのREDを観測し、1件ずつ最小修正してGREENへ戻す。
既に正しく動く条件の追加テストは初回GREENと正直に記録し、架空のREDを作らない。
親が各sliceの証拠を確認し、単独writer、固定差分の独立review、read-only final、
対象headのGitHub checks成功という既存gateを維持する。

不足テストには独立したJSON wire文字列を期待値として用い、20 KiB等の後続機能へ範囲を広げない。
本家の警告順・Minimal対象範囲へ戻す修正は新しい意図的差分ではない。
外部module、原稿のOKF変換、binary埋込み、src/core直接参照は追加しない。

## 未解決事項

- 現コードを上記の補強を経て採用する方針へのユーザー回答。
- 上記4件の修正と受入テスト、再review、final、CI、merge。
