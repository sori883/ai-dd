# Go TDDの作業単位handoff

この文書は、親とGo実装担当の間で仕事を受け渡す契約の正本である。1回の`loop`依頼は、承認済み計画に含まれる
まとまった作業単位（work unit）を渡す。実装担当は、その中の複数の観測可能な振る舞いを順番に
RED→最小GREEN→必要なrefactorへ進め、全項目の完了時または本当に停止が必要なときだけ親へ返す。

```text
親 → 承認済みwork unitを1回依頼
      └─ 担当: slice 1 RED→GREEN
         担当: slice 2 RED→GREEN
         担当: …
         担当: 全targeted testを再確認して1回返却
親 → 全差分とtest群を1回確認 → 独立review
```

項目ごとのテスト先行は維持するが、項目ごとの親返却、親による中間RED再実行、別GREEN依頼、hash受入は要求しない。
廃止した`tdd_phase`と`red_acceptance`を補完・要求しない。

既定では1 Issue／PRの承認済み実装項目全体を1つのwork unitにする。項目数が多いことだけを理由に分割しない。
異なるwriter所有権、未解決の承認・設計gate、1回では安全に扱えない作業量がある場合だけ、計画に理由と境界を
記録して複数work unitへ分ける。

## 入力と検証mode

`verification_mode=loop/review/final`は検証範囲を表す。Go実装担当は`loop`または親が明示したread-onlyの
`final`だけを受け付け、`review`は独立reviewerへ渡す。`loop`未指定は従来どおり`loop`として扱う。

loop依頼には、過去の会話を推測せず使える次の項目を渡す。

| 項目 | 内容 |
| --- | --- |
| plan / authorization / issue | 自己完結した計画、直接承認または包括承認の根拠、実在するIssue |
| workdir / starting_head | 作業rootの絶対path、依頼開始時のGit HEAD |
| verification_mode / work_unit_id | `loop`とIssue内で一意な作業単位名 |
| ownership | work unit全体で編集できるtest・production・文書・設定fileまたはpackage |
| behaviors | 実装順のslice一覧。各sliceに識別子、期待する観測可能な動作、test／実装file、exact targeted commandを含める |
| scaffold | 新APIをrunnable assertionへ到達させるため必要な場合だけ、許可する型・signature・空の返値 |
| boundary checks | work unit末尾にまとめて行うaffected package test、format、差分確認 |

型、公開API、保存形式、互換性などを計画時点で一意に決められない場合は、実装へ渡す前に承認gateへ戻す。
無関係なuser変更を保全し、同じ作業ツリーの実装writerは常に1つにする。

## work unit内のTDD

実装担当は開始時に全入力、HEAD、所有範囲、既存差分を確認し、各sliceを指定順に処理する。

1. そのsliceのproduction実装を変更する前に、期待動作を検査するrunnable testを追加・変更する。
2. testをgofmtし、指定commandを実行する。compileに成功し、指定testが実際に走り、意図した期待値不一致で
   失敗した場合だけREDとして記録する。
3. compile error、環境障害、`no tests to run`、skip、無関係なfailureはREDにしない。新APIの宣言不足なら、
   計画で明示されたcompile-only scaffoldだけを追加し、runnableな失敗まで再実行する。権限がなければ停止する。
4. testが初回から成功した場合は`ALREADY_GREEN`と記録する。既存実装を削除したり期待値を歪めてREDを作らない。
5. valid REDの後だけ、そのsliceを通す最小production実装を行う。後続sliceの振る舞いを先回りして実装しない。
6. 同じcommandをGREENにし、必要なrefactorをGREEN上で行い、gofmt後に同じtestと影響するtargeted testを再実行する。
7. 親へ返さず、次のsliceのtestへ進む。

先行sliceの最小実装で後続sliceが既に成立していた場合は、その後続testを先に追加して`ALREADY_GREEN`を実測する。
これは後からtestを作ったREDとは報告しない。計画外の実装先行が原因なら、scope超過として最終報告へ明記する。

## 停止条件

次の場合は後続sliceへ進まず、完了済みの有効な変更を勝手に戻さず、まとめて親へ返す。

- testの期待値が仕様と矛盾する、または複数案から一意に直せない。
- 所有file、計画、公開API、永続data、互換性、権限を広げる必要がある。
- 外部Go moduleまたは未承認toolが必要になる。
- compile、環境、既存failureにより意図したRED／GREENを安全に確認できない。
- 別writerの変更や開始snapshotとの衝突を検出した。

失敗を隠すためのtest緩和、skip、実装削除、範囲拡張を行わない。

## work unit末尾の返却

全slice完了後、許可された文書更新を一つのまとまりとして行う。全sliceのtargeted commandをもう一度実行し、
計画で許可されたaffected package test、変更Go fileのgofmt、`git diff --check`を確認してから1回だけ返す。
`loop`では全package、race、vet、全体lint、cross compile、配布E2Eを実行しない。

返却には次を含める。

- status: `WORK_UNIT_READY`または`BLOCKED`
- work unit ID、完了済み／未完了slice
- 各sliceのREDまたは`ALREADY_GREEN`のexact command、終了code、test名、観測した理由
- production変更があるsliceのGREEN commandと終了code
- 変更path、開始／終了HEAD、最終file hash、既存user変更との区別
- work unit末尾に再実行したtargeted／package command、gofmt、`git diff --check`
- 残余リスク、または後続を止めた具体的なgate

親は返却後に全差分、テストが期待動作を検査していること、範囲、各sliceの時系列証拠を確認し、work unitの
targeted command群を一度だけ再実行する。親は中間のRED状態を作り直すためにproductionを戻さず、実装担当が
実行していないREDを後からあったことにしない。問題がなければ独立reviewへ進む。

## review findingの修正

親は一回のreviewで得たblocking findingを、可能な限り一つのrepair work unitへまとめる。観測可能なGo動作を
変えるfindingは、各々の回帰testを先に追加して意図したREDを確認してから修正する。文書・設定だけの修正、
既に正しい動作への補足test、`ALREADY_GREEN`には人工REDを要求しない。

実装担当は全findingを順に修正し、末尾で対象test群をまとめて確認して一度返す。親の差分・test確認後、
独立reviewerへ一度再reviewを依頼する。新しいblocking findingが出た場合だけ、新しいrepair work unitを作る。

## review、final、merge

実装work unit確認後は、固定base/headの独立reviewを行う。blocking findingがなく差分が安定した後、親が
read-onlyの`final`を1回開始する。計画に応じて全package test、race、vet、format check、lint、cross compile、
配布E2Eをまとめる。final後に対象fileが変われば証拠はstaleになり、必要なrepair work unitと再review後に
finalを更新する。現在headのGitHub checks成功を確認してからmergeする。

文書・設定だけの変更に製品Goコードの人工REDは要求しない。対応するvalidator、parser、設定ロード、
参照整合、差分checkをwork unit末尾とfinalで適切に実行する。
