# Stage Graphと段階別手順の実装証拠

- work_unit_id: stage-graph-procedures
- verification_mode: loop
- Issue: https://github.com/sori883/ai-dd/issues/144
- 承認: [直接実装依頼](2026-09-09-stage-graph-implementation-approved.md)
- 正本: [実装計画](../../design/stage-graph-implementation-plan.md)
- 開始HEAD: a8b9d5d71dd3cf47dc2c26b1ff8d73f9a27ac23b。writerはcommitしていない。

## 実装した契約

JSONを前進・祖先・差戻し候補の正本とし、旧switch/順位mapを除去した。
Goは既存4段階Sensorを保持し、手順frontmatterの文書参照も検査する。
文書outputsが空でも要件・計画・コード・test・ADR要否の既存必須検査は残す。
定義全体のpathとbytesをschema3 Intentへhashで結び付け、変更時は作業とstate変更を拒否する。
読取り専用procedureは現在手順全文と候補を返し、show/list診断は定義変更時も利用できる。
旧schemaのファイルは移行・削除しない。

reopenはpending・log・最終stateの順に保存する。要求revisionは最終保存まで維持し、
同一expect/from/to/reasonだけretryする。理由は引用文字列表現で改行や偽Markdown見出しを区別する。
既存logは前hashまたは決定的追記後hashとして照合し、変更された履歴を再生成しない。
新規logはpending前に空の通常fileを用意する。これは、元未存在と追記後の削除を区別して
削除時も復元診断するための通常実装詳細で、親が承認範囲内と確認した。
初期作成失敗はstate未変更、各保存失敗は成功を返さない。追加audit/state履歴は導入しない。

製品原稿はfresh配置へ同梱する。SKILLは現在手順とhelpへの短い入口、WORKFLOWは共通索引に分割した。
段階手順は必要な文書・操作・担当を記載し、共有state/文書のwriterと独立review時点を維持する。
定義の欠落を内包原稿で補わない。既設の自動更新は行わず、relocateの既知原稿照合を維持する。

## TDD証拠

各行の指定commandでtestを先に追加し、runnableな意図したRED exit 1を確認してから実装し、同commandのGREEN exit 0を確認した。
ログは `/tmp/stage-graph-N-red.log` と `/tmp/stage-graph-N-green.log`（Nは表の順）。

|順|exact command|観測RED|
|---|---|---|
|1|`go test -count=1 ./src/internal/workflow -run '^TestDefinition'`|空scaffoldが正常定義を読めず、不正・重複・危険pathを受理|
|2|`go test -count=1 ./src/internal/flow -run '^TestGraphTransition'`|graphで禁止した同段階reopenとgraph欠落を固定遷移が受理|
|3|`go test -count=1 ./src/internal/flow -run '^TestProcedureBoundary'`|宣言input/outputの欠落が検査へ接続されない|
|4|`go test -count=1 ./src/internal/flow -run '^(TestDefinitionBinding\|TestReopenLog)'`|schema2、drift時configure、log/最終保存故障での偽成功、履歴未記録|
|5|`go test -count=1 ./src/internal/cli ./src/internal/minimal -run '^TestProcedure'`|procedure文法・未開始/waiting読取りを拒否|
|6|`go test -count=1 ./src/internal/install -run '^TestWorkflowDefinition'`|fresh配置にworkflowが存在しない|
|7|`go test -count=1 ./src/cmd/aidlc -run '^TestProcedureEvidence'`|観測欠落・実行なし・canaryなしを空validatorが受理|

表のslice4の `\|` はMarkdown表記のescapeで、実コマンドのpatternは `^(TestDefinitionBinding|TestReopenLog)`。
空outputsでもtest gateを保つslice3回帰はALREADY_GREENであり、人工REDを作っていない。
追加回帰は次の通り。

- future accepted入力による不可能な前提: slice1の同command、`/tmp/stage-graph-1-prerequisite-{red,green}.log`、exit 1→0。
- malformed binding/pending拒否: slice4の同command、`/tmp/stage-graph-4-schema-{red,green}.log`、exit 1→0。旧schema拒否は既成立。
- fresh log削除拒否: slice4の同command、`/tmp/stage-graph-4-deletion-{red,green}.log`、exit 1→0。
- 開始前文書修復でdriftを免除しない: slice5の同command、`/tmp/stage-graph-5-drift-{red,green}.log`、exit 1→0。
- 既存logの改変/削除拒否は追加coverage時にALREADY_GREEN。

## 末尾確認とfixture追従

全7targetedを再実行しexit 0。ログ `/tmp/stage-graph-final-1.log` から `-7.log`。
affected通常testは次を実行しexit 0（`/tmp/stage-graph-affected-final.log`）。

```sh
go test -count=1 ./src/internal/workflow ./src/internal/flow ./src/internal/cli ./src/internal/minimal ./src/internal/install ./src/cmd/aidlc
```

初回affectedで旧schema2/help全文/旧手順配置期待と手作りSpace fixtureの定義未配置が検出された。
これらはINVALID_TEST_FIXTUREとして新規schema/配置の前提へ訂正し、元のCAS・故障・review/assertを保持した。
CLI故障fixtureのreview checkoutは配置assetsをfixture commitへ含め、同じコードbytes条件を維持した。
変更Goファイルをgofmtし、git diff --checkを確認した。

## 親finalへ渡す入口と未実行範囲

```sh
go test -tags=integration -count=1 -v ./src/cmd/aidlc -run '^TestProcedureJourney$'
AIDLC_PROCEDURE_LIVE=1 go test -tags=integration -count=1 -v -timeout 15m ./src/cmd/aidlc -run '^TestProcedureLive$'
```

journeyは実CLIで4段階・TDD→planning差戻し・再前進・各時点のprocedureを確認する。
限定liveのhostは既存実CLI fixtureでTDDまで準備する。fixture reviewを独立AIの実測と報告しない。
実モデルが現在手順・begin・通常canary・planningへreopen・新手順取得を行い、
raw hook Pre/Post・同session/Intent・実process exit・canary現物をvalidatorが照合する。
固定Codex 0.153.4、既存model/sandbox/auth/test observerを使用し権限を広げない。
loopではintegrationのcompile/実行、live、全package、race、vet、crossbuildを行っていない。
最終検証と実機の証拠は親PRへ記録する。

## 本家との差と影響

固定ローカルAI-DLC 2.6.123の確認済み段階frontmatter/生成graph/audit連動に対し、
本製品は承認済み4段階のJSON遷移と段階MDをruntime正本にする。読込み量と二重管理の削減が理由。
新fresh配置が必要で、既設の自動上書きや33段階/audit機構の復元は行わない。
最新upstreamや未確認の生成経路との一致は主張しない。実際のtoken削減効果は未測定。

## 独立レビュー3件の契約修復

`work_unit_id=stage-graph-review-repair`、`verification_mode=loop`。
開始HEADは `fa7e57f6b72094abe9678c3f7402d4771a43589c`。Issue144の直接承認範囲で修復した。

1. `TestReopenLogCapacity` はMaxBytes近傍の既存logに通常理由を追記して成功扱いとなり、次回読取り不能になる問題をREDで再現した。
   freshのescape後に容量超過する理由でも、拒否前に空logを作る問題を確認した。
   決定的な追記blockを完全にencodeして総bytesを検査し、超過はpending・state・logへの変更前に拒否する。
   既存state/log bytesの保持と容量内成功を同じ回帰で確認した。retryの書込み前にも容量を検査する。
2. `TestProcedureBoundaryAcceptedTDDOutput` は合法なTDD文書outputをintegrationからaccepted参照した場合、
   TDD終了passなのにAcceptedへ文書版が残らず、変更していない次段階の開始が失敗する問題をREDで再現した。
   同じcollectorが検査した明示outputsのFileVersionを返し、test proofと重複排除してTDD Acceptedへ保存する。
   別bytesの再読取りをしない。共有current入力は保存対象へ追加せず、明示output改変は次段階で拒否する。
   既存のpathに依存しないTDD test証拠の固定を維持した。
3. `TestDefinitionRejectsEmptyOrUnknownAgent` は `{}` と `{role: unsupported}` がmapの空値一致で通る問題をREDで再現した。
   roleの存在boolとagentの一致を両方要求する。agentだけの不完全な定義の拒否はALREADY_GREEN。

指定commandとログ（各RED exit 1→GREEN exit 0）:

- `go test -count=1 ./src/internal/flow -run '^TestReopenLog'`: `/tmp/graph-review-1-{red,green}.log`
- `go test -count=1 ./src/internal/flow -run '^(TestProcedureBoundary|TestBoundaryTransition|TestStartSensor)'`: `/tmp/graph-review-2-{red,green}.log`
- `go test -count=1 ./src/internal/workflow -run '^TestDefinition'`: `/tmp/graph-review-3-{red,green}.log`

末尾に同じ3commandを実行し、`/tmp/graph-review-final-{1,2,3}.log`へ保存した。
許可されたaffected通常testは `go test -count=1 ./src/internal/flow ./src/internal/workflow`、ログは `/tmp/graph-review-affected.log`。末尾3targetedとaffectedはすべてexit 0。
変更Goファイルはgofmtし、git diff --checkを確認。integration/live・race・vet・全packageは実行しない。
最終検証の証拠は親PRへ記録する。
