# PRマージ

実装許可のあるPRを、品質gateを維持したまま自律的にmergeし、default branchへの反映とIssue完了を確認する。

## 自律マージの許可

次の両方を満たす場合だけ、自律マージを使用する。

1. PRの計画が、ユーザーの直接承認または承認済みの包括承認枠で許可されている。
2. ユーザーが、そのtask、milestone、またはPRをmanual mergeに指定していない。

許可の根拠が曖昧な場合はmergeせず、ユーザーへ確認する。このrepositoryでは
`docs/ram/decisions/2026-09-03-milestone-authorization-and-autonomous-merge.md`が包括承認枠内の
自律マージを許可する。承認済みフロー外の単発のPR作成依頼だけからmerge許可を推測しない。

## 品質gate

merge前に、現在のheadについて次をすべて確認する。

- PRがDraftでなく、base、head、対応Issue、変更範囲が意図どおりである。
- 独立reviewにblocking findingがない。
- 差分安定後のread-onlyなfinal検証が成功し、その後に対象fileが変わっていない。
- PRで起動する対象workflowをrepositoryの設定とpath条件から確認している。
- 対象workflowのcheckが出現し、すべて成功している。未開始、pending、failure、cancel、古いheadの
  結果は成功として扱わない。

変更pathによりGitHub checkが1件も想定されない場合だけ、対象workflowがない根拠とlocal finalの証拠を
PRへ記録して進められる。「まだcheckが表示されていない」ことを「check不要」と解釈しない。

checkやreviewが失敗した場合は保護を迂回せず、許可範囲内の修正を実装loopへ戻す。修正でheadが変われば
reviewとfinalのfreshnessを再確認する。複数の安全な修正案を一意に選べない場合はユーザーへ確認する。

## merge方式

- GitHub native auto-mergeが有効で、required checksなどのgateを保てる場合は、repositoryで継続利用して
  いるmerge方式を指定してauto-mergeを設定できる。
- native auto-mergeを利用できない場合は、対象checksの成功を待ってから、同じmerge方式でmergeする。
- repository設定、branch protection、ruleset、required checkを変更または迂回しない。変更が必要なら
  別の許可を得る。

再実行時は、既にmergeされていないかを先に確認し、同じ操作を重ねない。

## merge後

1. PRのstate、`mergedAt`、merge commitを読み直す。
2. merge commitがdefault branchの履歴に含まれることをremoteから確認する。
3. 対応Issueがcloseされ、変更が完了していることを確認する。自動closeされなかった場合だけcloseする。
4. branch削除がrepository運用に含まれる場合は、削除結果を確認する。
5. PR番号、URL、merge commit、checks、Issue状態を日本語で報告する。

merge結果が不明な場合は再実行せず、PR状態を照会してから判断する。
