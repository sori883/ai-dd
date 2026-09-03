# ロードマップ単位の包括承認と自律マージを採用する

- 日付: 2026-09-03
- 状態: Accepted
- GitHub Issue: [#67](https://github.com/sori883/ai-dd/issues/67)
- 承認: ユーザーが、PRの粒度を維持しながら承認をロードマップ／マイルストーン単位へ広げ、
  本家準拠の実装を個別承認なしで進め、重要な疑問点だけを確認し、品質gate成功後にPRを
  自律マージする計画を2026-09-03に明示承認した。

## 背景

従来は、小さな実装ごとに詳細計画の承認、Issue、実装、独立review、final検証、PR作成、
人間によるmerge判断を順番に待っていた。品質を確認する機会は多い一方、本家AI-DLCの固定snapshotと
既存方針から一意に決められる変更でも、人間の返答待ちが繰り返されていた。

ユーザーが必要としているのは、IssueやPRを大きくすることではない。大きな方向性を一度承認した後、
その範囲内の小さなPRを、同じ品質gateのまま連続して完成させることである。

## 用語

- **直接承認**: 1つの詳細な実装計画をユーザーが明示承認すること。
- **包括承認枠**: 複数の小さなIssue・PRを含むroadmapまたはmilestoneについて、対象、目的、準拠根拠、
  重要な境界をユーザーがまとめて承認した実装範囲。
- **実装許可**: 計画が直接承認済みか、計画全体が包括承認枠に収まっている状態。
- **自律マージ**: 人間のPRごとのmerge指示を待たず、独立review、final検証、GitHub checksのgateを
  確認したエージェントがPRをmergeし、default branchとIssueの状態まで確認すること。

## 決定

### 承認の単位

1. 各Issueでは、従来どおり自己完結した詳細計画、受入条件、検証方法、変更範囲を作る。
2. 詳細計画には、直接承認または包括承認枠のどちらが実装を許可するかを記録する。
3. 包括承認枠内の計画は、個別のユーザー承認を待たず、Issue作成と実装へ進める。
4. 包括承認枠外のroadmapまたはmilestoneは、まとまり全体の計画をユーザーへ提示し、明示承認を得る。
5. IssueとPRは1つの観測可能な機能・挙動を扱う従来の粒度を維持する。包括承認を理由に複数の
   独立した変更を1つのPRへまとめない。

### 最初の包括承認枠

[AI-DLC Go実装ロードマップ](2026-09-03-aidlc-implementation-roadmap.md)の残作業を、最初の包括承認枠とする。
このroadmapは実装順序の粗い記録であり、各PRの詳細仕様は、本家の根拠、既存code、test、後続RAMを
確認して計画へ具体化する。

過去の決定と後続の決定が衝突する場合は、過去を削除せず後続決定を優先する。特に、roadmapにある
「Stage構成を実行中に変更しない」という記述は、
[in-flight recompose方針](2026-09-03-inflight-recompose-policy.md)により置換されている。包括承認は、
置換済みの古い方針を復活させない。同じ固定方針を記録している
[OKF v0.2参照基盤と初期統合境界](2026-09-03-okf-reference-boundaries.md)の決定7も、
in-flight recompose方針により置換されている。

本家準拠の根拠は、リポジトリに固定されたAI-DLC `2.6.123` snapshotの実際に確認した範囲とする。
最新upstreamとの一致を推測せず、対象versionの変更は互換性に関わる新しい判断として扱う。

### 人間へ確認する条件

包括承認枠内でも、次の場合は実装またはmergeを止め、選択肢、根拠、影響をまとめてユーザーへ確認する。

1. 本家の根拠が曖昧または矛盾し、結果が異なる複数の案を安全に一意選択できない。
2. 本家と異なる新しい仕様・挙動を意図的に採用する。
3. 計画が包括承認枠を越える。
4. 公開API、永続data、互換性、移行、安全性、権限、運用に重大な選択がある。
5. 外部Go module、有料service、認証情報、追加権限が必要である。
6. data削除などの不可逆または破壊的な操作が必要である。
7. testまたはreviewの問題について、複数の妥当な修正案から安全に一意選択できない。

Issue範囲内の通常の実装詳細、明らかなbug、review findingは、追加承認なしで同じ実装loopへ戻せる。
修正で計画の境界を越える場合は、この例外を使用しない。

### 品質gateと自律マージ

自律マージは、次をすべて満たすPRだけに使用する。

1. 実装許可の根拠、対応Issue、base/head、変更範囲が明確である。
2. 単独writerによる実装とtargeted検証が完了している。
3. 独立reviewにblocking findingがない。
4. 差分安定後のread-onlyなfinal検証が成功し、その後に対象fileが変わっていない。
5. PRで起動する対象workflowをrepository設定とpath条件から確認している。
6. 対象checkが出現し、現在headですべて成功している。未開始、pending、failure、cancel、古いheadの
   結果は成功扱いしない。
7. ユーザーがtask、milestone、またはPRをmanual mergeに指定していない。

変更pathによりcheckが1件も想定されない場合だけ、workflowのpath条件とlocal finalの証拠をPRへ記録して
進められる。check未表示をcheck不要と解釈しない。

GitHub native auto-mergeが有効でrequired gateを保持できる場合は利用できる。利用できない場合は、
対象checksの成功を待ってから、repositoryで継続利用している方式でmergeする。本決定時点では
repositoryのnative auto-mergeは無効、mainのbranch protectionとrulesetはなく、直近PRはmerge commitを
使用している。このため、当面はエージェントがchecks成功後にmerge commit方式でmergeする。

repository設定、branch protection、ruleset、required checkを変更または迂回する権限は、この決定に
含めない。merge後はPRの`mergedAt`とmerge commit、default branchへの反映、対応IssueのCloseを確認する。

## 維持するgate

- PRごとの小さな変更範囲とGitHub Issue
- Go変更のRed-Green-Refactorと単独writer
- 固定base/headに対する独立review
- blocking finding修正後の再review
- 差分安定後に1回行うread-onlyなfinal検証
- 自己完結した日本語のIssue・PR
- 新しい意図的な本家との差分と外部Go moduleへの明示承認

[検証頻度の決定](2026-09-02-validation-cadence.md)は変更しない。

## 置換する決定

- [GitHub Issue・PRを日本語の実装記録として運用する](2026-09-01-japanese-github-pr-workflow.md)の
  「自動マージを設定せず、mergeはユーザーの明示依頼に限定する」という項目を置換する。
- [AI-DLC Go実装ロードマップ](2026-09-03-aidlc-implementation-roadmap.md)の
  「細かな順序や対象fieldは各Issueの承認済み計画で確定する」という部分は、詳細計画で確定する点を
  維持しつつ、包括承認枠内では計画ごとのユーザー承認を要しない形へ置換する。
- [OKF v0.2参照基盤と初期統合境界](2026-09-03-okf-reference-boundaries.md)の決定7にある
  「Stage実行中に構成を変更しない」という部分は、
  [in-flight recompose方針](2026-09-03-inflight-recompose-policy.md)へ置換する。Intent開始前の初期routingと
  OKF参照境界に関するその他の決定は維持する。
- 過去のIssue固有計画に記録された「そのPRは自動マージしない」という判断は、その完了済みPRの
  履歴として保持する。将来のPRへ一般化しない。

## 影響と移行

この変更は開発運用だけに適用し、AI-DLC製品のAPI、保存形式、実行時挙動を変更しない。本家AI-DLCとの
意図的な製品差分はない。

Issue #67の導入PRまでは旧規則が有効なため、そのPRは自動マージせずユーザーが手動でmergeする。
導入PRがdefault branchへ反映された後、最初の包括承認枠と自律マージを使用する。

問題が生じた場合は、本決定と対応するrule、skill、agent設定の変更をrevertし、PRごとの直接承認と
manual mergeへ戻せる。既にmerge済みの製品変更を、この運用変更だけを理由にrevertしない。
