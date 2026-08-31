# Intent一覧CLIの参照契約

- 日付: 2026-09-01
- 状態: Current for local AI-DLC 2.6.123 snapshot
- 関連: [既存Intent読み取り調査](2026-08-31-intent-reading-contracts.md)、
  [承認済み計画](../decisions/2026-09-01-intent-list-plan.md)

## 確認範囲

technical_researcherがanalysis indexを入口に、ローカルのauthored core、
canonical Codex dist、配置済みCodex distを静的に確認した。対象の
aidlc-lib.ts、aidlc-utility.ts、aidlc.tsは3形式でSHA-256が一致し、
versionは2.6.123だった。command、hook、installer、testは実行していない。
最新upstream全体との一致は主張しない。

## registryとdirectoryの相関

- registryはspace配下のintents/intents.jsonを読む。行順を維持し、重複行も残す。
- truthyなdirNameは同名directoryだけに対応し、不一致時にlegacy fallbackしない。
- dirNameが欠落、null、空文字なら、slugとUUID末尾のlowercase hexを組み合わせた
  legacy directory名を候補にする。UUIDはdashを除き、hex suffixは1文字以上。
- 対応directoryがないregistry行も一覧へ残す。未使用directoryはorphan行として追加する。
- directory側の追加順は既存のUTF-16比較順。orphanのstatusはunknown、UUIDは空文字。
- orphan slugは先頭の6桁とdashを優先して外し、それがなければ末尾の
  lowercase hex suffixを外す。
- active-intent cursorはdirectory名で照合する。同一directoryへ対応する重複registry行は
  どちらもactiveになり得る。
- reposは表示用レコードに含まれる。scopeは内部で読み得るがpublic JSONへは出さない。

registryが不在、読取不能、JSON不正、top-level非arrayの場合、本家は空registryとして
disk discoveryを続ける。一方、array内要素は型検証せずcastしており、不正行の挙動は
data依存で安全な契約になっていない。

主要根拠:

- docs/実装_aidlc-workflows/core/tools/aidlc-lib.ts:2331、:2385、:2442
- docs/実装_aidlc-workflows/tests/integration/t165-intent-create-p4.test.ts:930
- docs/実装_aidlc-workflows/tests/integration/t229-intent-dirname-persistence.test.ts:135
- docs/実装_aidlc-workflows/tests/integration/t230-intent-dirname-migration.test.ts:285

## active解決と表示

public一覧はworkflow/session selectionを解決し、解決済みspaceとintent overrideを
一覧処理へ渡す。session bindingが共有cursorより優先する場合があり、
明示nullはlone-intent fallbackを抑制する。

人間向け表示はspace名のheader、active行のasterisk、statusの角括弧を使う。
activeがなければ空行の後に切替案内を出す。空一覧は新規intentの作り方を案内する。
JSONはactive、space、intentsを持ち、各行はuuid、slug、status、repos、
dirName、activeを持つ。active不在はnullである。

public parserは余剰引数等を広く許容し、project-dirは分離形式だけをglobalに抽出し、
重複時は最後を使う。utilityが認識したruntime errorはJSON stderrとexit 1になる。

主要根拠:

- docs/実装_aidlc-workflows/core/tools/aidlc-utility.ts:6055
- docs/実装_aidlc-workflows/tests/integration/t297-intent-listing.test.ts:458

## Go版への示唆

最初のsliceではsession bindingを実装せず、active overrideをpointerで受け取れる内部APIにして、
nilだけ既存ActiveIntentへfallbackさせる。CLIは現在の共有cursorを使う。
本家の不正array行は契約化せず、Go版は構造を厳格検証して明示errorにする。
その他の採用差分、CLI境界、非対象は承認済み計画に記録する。

外部libraryやAPIは今回使わないためContext7照会は不要だった。
