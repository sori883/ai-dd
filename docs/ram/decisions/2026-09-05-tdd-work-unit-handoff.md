# Go TDDを作業単位の連続実装へ変更する

- 日付: 2026-09-05（Asia/Tokyo）
- 状態: Implemented（ユーザー直接承認、Issue #110）
- Issue: [#110](https://github.com/sori883/ai-dd/issues/110)
- 置換対象: [Go TDDの依頼をREDとGREENへ分離する](2026-09-04-tdd-phase-handoff-plan.md)

## 背景

従来は、1つの観測可能な項目ごとにREDだけを実施して親へ返し、親が同じtestを再実行してからGREENを
別依頼し、さらに親がGREENを再実行して次項目へ進んでいた。12項目では約24回の親子受け渡しとなり、
数秒のtargeted testに対して、依頼、コンテキスト再読込、待機、報告の時間が大きくなった。

この方式はIssue #93で起きた「最初のRED/GREEN後にproduction実装を先行し、残りのtestを後から追加した」
問題への回復策だった。しかし、testを先に実行した証拠を残すことと、各項目で親へ制御を戻すことは
別の条件である。細かな返却は後追いtestを防いだ一方、統合機能の所要時間を不必要に増やした。

## ユーザー決定と実装許可

ユーザーは、各項目でtest-firstを維持しながら、全項目を一人の実装担当が連続して作り、完了後に親へ
受け渡す方式への変更を明示的に依頼した。文章更新、review finding修正、再review、全体検証も、意味のある
作業単位より細かく分割しないことを求めた。この回答を本運用変更の直接承認とする。

本決定は、旧計画の1項目・1phase返却、親の中間RED／GREEN再実行、`tdd_phase`、`red_acceptance`を置換する。
[知識供給の包括承認](2026-09-05-context-delivery-autonomous-authorization.md)に記載した旧handoff cadenceも、
実装scopeの承認は維持したまま、本決定の作業単位方式へ置換する。

## 採用する作業単位

- 親は、承認済み計画、Issue、所有範囲、順序付きbehavior一覧、各targeted commandを1つのwork unitとして渡す。
- 1 Issue／PRの承認済み実装項目全体を既定のwork unitとする。項目数だけを理由に分割せず、異なるwriter、
  未解決gate、安全に扱えない作業量がある場合だけ、計画へ理由と境界を記録して分ける。
- 単独implementerは各behaviorで、test追加、runnableな意図したRED、最小GREEN、GREEN上のrefactorを順番に行う。
- compile failure、環境failure、skip、`no tests to run`はREDにしない。新APIは計画済みcompile-only scaffoldで
  runnable assertionへ到達させる。
- 後続behaviorを先回りして実装しない。先行実装によりtestが初回成功した場合は`ALREADY_GREEN`と記録し、
  人工REDを作らない。
- implementerは全behavior完了時、または仕様・所有権・環境等の本当のblocker発生時に一度だけ返す。
- work unit末尾で全targeted test、許可されたaffected package test、gofmt、差分checkをまとめて確認する。
- 親は返却後に全差分とtargeted test群を一度再実行する。中間RED状態を再現するためproductionを戻さない。
- 一回のreviewで得たblocking findingは一つのrepair work unitへまとめ、観測可能な修正は各回帰testを先行する。
  文書・設定だけの修正と`ALREADY_GREEN`に人工REDを要求せず、完了後に一度再reviewする。
- 固定差分の独立review、安定後1回のread-only final、現在headのGitHub checks成功、単独writerは維持する。

## 証拠とtrade-off

各behaviorについて、implementerはexact command、REDまたは`ALREADY_GREEN`の終了codeと理由、変更後GREENを
最終報告へ残す。親は実装末尾の完成差分と全targeted testを検証する。

親が各REDをproduction変更前の状態へ戻して再実行しないため、旧方式にあった独立した中間RED再現性は失う。
代わりに、実装担当の実行trace、最小実装の順序、完成後のtest、独立reviewでtest-firstと回帰防止を確認する。
ユーザーは、項目ごとの往復を削減するためこのtrade-offを選択した。実行していないREDを後から記録したり、
完成後のtestをtest-firstと偽ったりしてはならない。

## 変更範囲

- `AGENTS.md`
- `.codex/agents/go-tdd-implementer.toml`
- `.agents/skills/go-tdd/SKILL.md`
- `.agents/skills/implementation-planning/SKILL.md`
- `.agents/skills/go-code-review/SKILL.md`
- `docs/tdd-handoff.md`
- `docs/agent-workflow.md`
- 本記録と`docs/ram/README.md`

AI-DLC製品のGo code、公開API、永続data、本家準拠挙動、agent model／reasoning／sandbox、外部moduleは変更しない。

## 受け入れ条件と検証

- すべての運用入口がwork unit方式で一致し、旧phase方式を要求しない。
- 各sliceのtest-before-production、valid RED、ALREADY_GREEN、停止条件、証拠が明記される。
- 文書更新とreview findingをまとまりとして扱い、独立reviewとfinalは維持される。
- `skill-creator`のvalidator、TOML parse、Codex設定ロード、参照整合、`git diff --check`が成功する。
- 独立したread-only reviewで、後追いtest防止と不要な親子往復削減が両立していることを確認する。

運用ファイルだけの変更なので、製品Go codeの人工RED、全package test、race、vet、cross compile、配布E2Eは
行わない。問題時は本決定を削除・上書きせず、新しいRAMから置換し、通常の修正PRで運用ファイルを戻す。

## 実装・前向き評価記録

Issue #110で、`AGENTS.md`、custom agent設定、`go-tdd`／計画／review skill、handoff正本、agent workflowを
同じwork unit契約へ更新した。1 Issue／PRの全承認済み項目を既定の単位とし、slice数だけでは分割しない
条件も明記した。

製品外の一時Go moduleで、`Double`と日本語`JapaneseGreeting`の2 behaviorを一つのloop依頼へ渡した。
独立した評価agentは、各behaviorでtestをproduction変更より先に作り、どちらもcompile済みのassertion RED
（exit 1）を確認し、同じcommandをGREEN（exit 0）にしてから、最後に`WORK_UNIT_READY`を一度だけ返した。
親は完成差分と2 testをまとめて1回再実行し、対象testとpackage testの成功、gofmt、差分check、所有範囲を
確認した。

変更前から起動中だった`go_tdd_implementer`は、repository fileを読み直しても起動時の旧developer instructionが
優先され、`tdd_phase`欠落として無変更で停止した。これは設定が実行中agentへhot reloadされない境界であり、
merge後に新しく起動するagentから新方式が有効になる。現在の親は同じ作業単位を直接適用できる。

`skill-creator`の`quick_validate.py`は、最初に選んだPython環境にPyYAMLがなく起動できなかったため、既存cache内の
PyYAML利用可能なPythonで再実行し、変更した3 skillすべて`Skill is valid!`を確認した。TOML parseと
`codex features list`による設定ロードも成功した。
