# 成果をPRとして提出する

## 提出できる状態

「完了時にPR」は、選んだ作業単位の成果が揃った時点を指す。PR作成とAI-DLCのfinish、GitHubでのmergeは別の操作である。

1. Issueの対象・受入条件と実際の変更が一致する。Intent全体の未完了工程を、Bolt一つの結果で完了としない。
2. Boltの場合、対象step_idとboltのうち、そのrepositoryのIssueに含めた全Unitが統合されている。他repositoryのUnitはこの集合に含めないが、依存や全体検証の条件は別途満たす。`reported`は提出、`integrated`は内容の統合であり、レビューや成果承認を意味しない。
3. 利用先が要求するSensor、テスト、独立レビュー、成果承認を確認する。実行結果と対象の内容SHAを参照し、古い結果や別Unitの成功を流用しない。
4. 対象Git作業場所の書込みが落ち着き、base...headの差分と未commit変更を確認できる。目的外の変更を一緒にcommitしない。必要な成果を漏らさず、秘密情報やruntimeの一時記録を公開しない。
5. PRに含めるcommitと、レビュー・最終検証した成果を対応付けられる。新しい変更が入ったら必要な確認をやり直す。

必要な条件が欠ける場合は、残る作業と証拠を返す。途中共有を利用者が求めた場合はDraftとして提出できるが、未実施項目と承認待ちを明記する。
まだ起動していないPR向けGitHub checksを、PR作成の前提や成功済みの証拠にしない。作成後に取得する。

依存する別Boltや別repositoryの成果が未反映の場合は、その依存を明記する。順次mergeを待つか、合意済みの依存branchをbaseにするかを確認し、全てdefault branchへ入ったように報告しない。

## 本文

利用先のPR templateを優先し、初めて読む人が目的・変更結果・確認範囲を理解できる文章にする。
同じIntent markerと作業単位の項目表を引き継ぎ、実装結果、検証コマンドと結果、独立レビュー、必要な承認、残件を記載する。

```markdown
<!-- aidlc-intent: 0123456789abcdef0123456789abcdef -->
## 変更の目的と結果
画面側だけにあった請求額の計算をAPIから利用できるようにしました。

## 対応する作業
repository: example/billing / Space: billing
intent_id: 0123456789abcdef0123456789abcdef
単位: Bolt / step_id: s04 / bolt: invoice-api / Unit: calculate、endpoint
関連Issue: https://github.com/example/billing/issues/42

## 検証とレビュー
実行したコマンド・対象・結果と、独立レビューの結果を実データで記載する。
現在headのGitHub checksは作成後に確認し、未開始・実行中を成功と書かない。

## 残件と影響
このBoltに含まれる成果と、Intent全体で後続に残る工程を区別して記載する。
```

Issueの全文、会話の履歴、試した順番の羅列を転載しない。変更理由や確認できた限界を短く説明する。

## Issueを閉じる条件

default branch向けPRが、そのIssueの全受入条件を満たし、merge時のcloseが承認された運用に含まれる場合だけ`Closes #42`を付ける。
別repositoryのIssueなら、完全なURLを使う前にそのIssueを閉じる権限と全範囲の完了を確認する。
一部のBoltのPRへ、全IntentのIssueを閉じるkeywordを付けない。関連付けだけなら通常のIssue URLを記載する。

非default branch向けPRではclosing keywordは自動closeに使えない。通常リンクで関係を残し、合意した取り込み先と完了境界に達してからIssueの状態を確認する。
PRの作成成功だけではIssueをcloseしない。進捗のstateも手動で完了へ書き換えない。

## mergeを依頼された場合だけ

利用者の明示依頼または承認済み運用がmergeを含む場合に限り、現在head、base、対応Issue、review、必要なchecksと保護規則を確認する。
作成済み・Open・Draft・merge待ちとMergedを区別し、未開始・pending・失敗・cancelのcheckを成功としない。
merge方式も利用先の規則に従い、`--admin`や保護設定変更で通さない。

```sh
gh pr view PR_NUMBER --repo OWNER/REPO --json state,isDraft,baseRefName,headRefOid,reviewDecision,statusCheckRollup,mergedAt,mergeCommit,url
gh pr merge --help
gh issue view ISSUE_NUMBER --repo OWNER/REPO --json state,closedAt,url
```

実行時は確認したheadを`--match-head-commit`で固定できる。merge queueやauto-mergeへ登録された状態をmerge完了と報告しない。
応答が不明なら再実行する前にPRの状態を取得する。merge後はmerge commitの取り込み先への反映とIssueの状態を確認する。
全受入条件とclose許可が揃い、自動closeだけがされていない場合に限り明示closeする。branch削除はその許可もある場合だけ行う。
