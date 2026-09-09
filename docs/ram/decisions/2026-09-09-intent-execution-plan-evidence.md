# Intent実行計画・会話承認のloop実装証拠

対応Issue: [#153](https://github.com/sori883/ai-dd/issues/153)。work_unit_id=`intent-execution-plan`、verification_mode=`loop`。
[実装計画](../../design/intent-execution-plan-implementation.md)と[直接承認](2026-09-09-intent-execution-plan-request.md)に基づく単独writer実装。
開始HEADは `6571014f3a06932e75a527f566f9b732ac287999`、branchは `codex/intent-execution-plan`。
元checkoutと中断中Issue146の作業は変更していない。外部module/tool追加なし。

## 実装結果と確定事項

カタログschema2に6段階を定義し、state schema5の実行回ID・承認済み計画・変更案・進捗・確定履歴を実装した。
新規Intentは未承認の必須initialization/discoveryから開始する。初期化はダミー成果やGit checkoutを要求せず配置を検査する。
選択した計画の先頭未完了回をbeginし、Sensor・独立review・実際の会話成果承認が揃った時だけfinishする。
任意段階の省略・並べ替えを支持し、Entry、Gate、文書、実測結果、Unit操作をstep_idへ対応させる。
共有current入力と同回outputの更新は許容し、accepted入力のすり替えと過去回の結果転用は拒否する。

初回discoveryの2承認は提示済みDraftのrevision/content hashへ結び付ける。回答到着前に両requestが存在し、
対象が同じ時だけ1回答を別々のCLI承認に使える。同一Draftの採用時も成果を再照合する。
Draft置換・却下、対象変更は証拠を失効させる。A→B→A再送、後発request、別target/stepへの転用を拒否する。
回答sourceはstate保存前に消費せず、JSONエスケープ後とstate末尾改行を含む容量を事前検査する。

mandatoryの役割と順序を維持する。初期化完了後の初期化再実行は
`[s01 completed,s03 initialization pending(reopens:s01),s02 discovery pending]`。
未完了現在回も新IDへ置換し、`[s01 completed,s02 discovery active]` は
`[s01 completed,s03 discovery pending(reopens:s02)]`。旧回と理由・変更前後は履歴へ残る。
先頭2要素や生IDの無条件固定という初期内部解釈を、この承認済みreopen由来の検査へ補正した。
任意差替えを許可せず、完了実績を保持する。新回へUnit結果や実測結果を自動持越ししない。

reopen承認とOKF logは既存pending/前後hash境界を維持し、pendingを計画revision/hash/step/理由へ結び付ける。
時刻固定はdurable pending保存後。保存前の失敗では有効な空本文OKF土台だけ残る場合があり、未保存要求の再試行は新しい時刻を選べる。
履歴は内容hash付きimmutable記録を書いてからstateのheadを確定する。historyはheadが指す列だけを厳密に検査する。
段階MDは固有の手順を短く返し、memory/Unit等の共通案内はWORKFLOWとhelpへ集約した。

## 順序付き8項目のTDD実測

以下のREDはコンパイル済みの期待値失敗（exit 1）、GREENは同じ対象commandのexit 0。
tool chunkは実行会話の証拠識別子であり、ファイル名ではない。型/signatureだけの明示未実装scaffoldは計画の許可内で使用した。

|項目|正確なcommand|REDの観測|GREENの観測|
|---|---|---|---|
|Schema|`go test -count=1 ./src/internal/workflow ./src/internal/flow -run '^TestExecutionPlanSchema'`|6種/prefix/state契約のassertion失敗、5731dd exit 1|c9931a exit 0|
|Bootstrap|`go test -count=1 ./src/internal/flow -run '^TestExecutionPlanBootstrap'`|初期化開始の期待値失敗、005b3d exit 1|5fe237 exit 0|
|Draft|`go test -count=1 ./src/internal/flow -run '^TestExecutionPlanDraft'`|明示未実装APIの実行失敗、339088 exit 1|2134c1 exit 0|
|Evidence|`go test -count=1 ./src/internal/flow -run '^TestExecutionPlanEvidence'`|Entry/Gate/Unit・選択入力/結果の実行回、10fdcc等 exit 1|8a9fac等、最終9c3377 exit 0|
|Approval|`go test -count=1 ./src/internal/flow -run '^TestExecutionPlanApproval'`|request/capture、34e3c8 exit 1。初回同回答の版、5c93c2 exit 1|88b57f、0e61a2、03edcf exit 0|
|ReopenHistory|`go test -count=1 ./src/internal/flow -run '^TestExecutionPlanReopenHistory'`|未実装history/head、635fed exit 1。現在回新ID、a0b548 exit 1|27322d、adb048、a3b3f8 exit 0|
|CLI|`go test -count=1 ./src/internal/cli ./src/internal/minimal -run '^TestExecutionPlanCLI'`|文法/読取り/容量、e8b50b exit 1。待機touch、ac5a96 exit 1|17503c、ca5dad、9e83e0 exit 0|
|Distribution|`go test -count=1 ./src/internal/install ./src/cmd/aidlc -run '^TestExecutionPlanDistribution'`|カタログ/validator、5e5da3 exit 1|49e15f exit 0|

Evidence追加サイクルは選択入力3b84f1→cbd6e4、実行回文書9e7c9c→9b5e6e、Unit/終了700e57→4f6706、
旧結果2ab828→664ede、Sensor hash6721c4→03025b、初期化review bc4b97→14289b、省略Plan5a9036→746324、
未来文書1a6741→9c3377（いずれも上記Evidence command、RED exit 1→GREEN exit 0）。
Approvalの任意順integration→tdd完走は先行実装でALREADY_GREEN（e9df78 exit 0）。人工的な失敗は作っていない。
履歴enum/対象86870c→9a134b、reopen1940f7→b59907、durable pending bb054d→adb048も同項目commandでexit 1→0。
CLIの旧advance案内/重複JSON a4763d→1e0a93、procedure回ID fd5b77→9e83e0もexit 1→0。

## 追加境界回帰とfixture追従

- `go test -count=1 ./src/internal/flow -run '^TestExecutionPlan(SchemaEncodedLimit|EvidenceStartGateStep)$'`: start Gateの空stepと末尾改行容量の期待値RED 0f8927 exit 1、GREEN c910a7 exit 0。
- `go test -count=1 ./src/internal/flow -run '^TestExecutionPlanApprovalFinishDropsPreviousUnits$'`: 次回へ旧Unit持越しによるRED 38d102 exit 1、GREEN 8690cd exit 0。
- `go test -count=1 ./src/internal/flow -run '^TestExecutionPlanSchemaMandatoryIdentity$'`: 任意初期化IDを受理するRED 3f5412 exit 1、GREEN 9c99c2 exit 0。
- `go test -count=1 ./src/internal/flow -run '^TestExecutionPlanEvidenceArtifactSelectedOrder$'`: 未来の構成分析artifactを未知扱いするRED 0b7c74 exit 1、GREEN b90960 exit 0。
- `go test -count=1 ./src/internal/flow -run '^TestExecutionPlanEvidenceADRPreviousExecution$'`: 先行回ADRを無視するRED ace569 exit 1、GREEN 0365bb exit 0。
- `go test -count=1 ./src/internal/flow -run '^TestExecutionPlanReopenHistoryPendingBinding$'`: step/revision/hash/reason不一致受理のRED 0698ba exit 1、GREEN 6d0616 exit 0。
- `go test -count=1 ./src/internal/flow -run '^TestExecutionPlanApprovalAdoptionRechecksEvidence$'`: 変更成果を保持するRED 75d0fd exit 1。修正後は `go test -count=1 ./src/internal/flow -run '^TestExecutionPlanApproval(AdoptionRechecksEvidence|SameAnswer)$'` 5927a4 exit 0で同一回答の保持も再確認。

既存testは明示的な承認済みfixtureを供給して対象Sensor/Unit/状態境界を隔離する。新機能のpublic操作は8項目側で検証する。
旧4段階・直接state進捗編集・advance・stage名reopen・stepなし結果を新契約へ追従させ、Sensorやレビュー判定自体を弱めていない。
work-log保存失敗は計画提示を済ませた後の承認書込みへ注入し、history書込みを除く既存base/pending/log/final境界を検証した。
新しいlogのplan情報が増えたため「収まる」fixture余白を256から1024 bytesへ調整し、上限超過拒否は維持した。

初期の二重Space prefix、ADR非対応kind、配布Role不足、機械置換の構文不成立は有効REDに数えていない。
fixture補正は親の再開許可を受け、期待する正規path・実行回境界を変えず再測定した。
既存fixtureの追従失敗は完了済みのTDD実績と分けて扱う。

実機fixtureは既存のobserver/モデルrunner構造を参考に、新request・step・履歴契約へ更新した。
`TestHumanApprovalLive` / `AIDLC_HUMAN_APPROVAL_LIVE=1` を維持し、拒否canary、2requestの同一回答、実CLI exitと確定履歴を照合する。
loopでは実機・journey・E2E本体は実行していない。integrationタグ下のDistribution unitだけによるコンパイル確認は親が明示許可した。
全package/race/vet/cross-buildと実機/E2E、独立reviewは親の後続gateである。

## work unit末尾の固定差分確認

全項目exit 0。下記は末尾の実測出力であり、過去のRED記録とは別である。

### schema

`go test -count=1 ./src/internal/workflow ./src/internal/flow -run '^TestExecutionPlanSchema'` — exit 0。raw log: `/tmp/intent-execution-boundary-schema.log`。

```text
ok  	github.com/sori883/ai-dd/src/internal/workflow	0.407s
ok  	github.com/sori883/ai-dd/src/internal/flow	0.581s
```

### bootstrap

`go test -count=1 ./src/internal/flow -run '^TestExecutionPlanBootstrap'` — exit 0。raw log: `/tmp/intent-execution-boundary-bootstrap.log`。

```text
ok  	github.com/sori883/ai-dd/src/internal/flow	0.306s
```

### draft

`go test -count=1 ./src/internal/flow -run '^TestExecutionPlanDraft'` — exit 0。raw log: `/tmp/intent-execution-boundary-draft.log`。

```text
ok  	github.com/sori883/ai-dd/src/internal/flow	0.415s
```

### evidence

`go test -count=1 ./src/internal/flow -run '^TestExecutionPlanEvidence'` — exit 0。raw log: `/tmp/intent-execution-boundary-evidence.log`。

```text
ok  	github.com/sori883/ai-dd/src/internal/flow	1.849s
```

### approval

`go test -count=1 ./src/internal/flow -run '^TestExecutionPlanApproval'` — exit 0。raw log: `/tmp/intent-execution-boundary-approval.log`。

```text
ok  	github.com/sori883/ai-dd/src/internal/flow	4.922s
```

### history

`go test -count=1 ./src/internal/flow -run '^TestExecutionPlanReopenHistory'` — exit 0。raw log: `/tmp/intent-execution-boundary-history.log`。

```text
ok  	github.com/sori883/ai-dd/src/internal/flow	3.377s
```

### cli

`go test -count=1 ./src/internal/cli ./src/internal/minimal -run '^TestExecutionPlanCLI'` — exit 0。raw log: `/tmp/intent-execution-boundary-cli.log`。

```text
ok  	github.com/sori883/ai-dd/src/internal/cli	0.347s
ok  	github.com/sori883/ai-dd/src/internal/minimal	0.704s
```

### distribution

`go test -count=1 ./src/internal/install ./src/cmd/aidlc -run '^TestExecutionPlanDistribution'` — exit 0。raw log: `/tmp/intent-execution-boundary-distribution.log`。

```text
ok  	github.com/sori883/ai-dd/src/internal/install	0.358s
ok  	github.com/sori883/ai-dd/src/cmd/aidlc	0.490s
```

### fixture-compile

`go test -tags=integration -count=1 ./src/cmd/aidlc -run '^TestExecutionPlanDistribution'` — exit 0。raw log: `/tmp/intent-execution-boundary-fixture-compile.log`。

```text
ok  	github.com/sori883/ai-dd/src/cmd/aidlc	0.366s
```

### affected

`go test -count=1 ./src/internal/workflow ./src/internal/flow ./src/internal/cli ./src/internal/minimal ./src/internal/install ./src/internal/workspace ./src/cmd/aidlc` — exit 0。raw log: `/tmp/intent-execution-boundary-affected.log`。

```text
ok  	github.com/sori883/ai-dd/src/internal/workflow	0.238s
ok  	github.com/sori883/ai-dd/src/internal/flow	53.023s
ok  	github.com/sori883/ai-dd/src/internal/cli	0.471s
ok  	github.com/sori883/ai-dd/src/internal/minimal	3.859s
ok  	github.com/sori883/ai-dd/src/internal/install	1.742s
ok  	github.com/sori883/ai-dd/src/internal/workspace	1.132s
ok  	github.com/sori883/ai-dd/src/cmd/aidlc	2.484s
```

### diff

`git diff --check` — exit 0。raw log: `/tmp/intent-execution-boundary-diff.log`。

```text
(outputなし)
```

変更source/fixtureのSHA256 manifestは `/tmp/intent-execution-final-source.sha256`。manifest自体のSHA256は `98841085c564a699e36acbddf16e90c9ea6ad12bf58be6b2ad0a30104b68bc28`。

```text
2fc939f110aabe464fea9fd1f940ac014f4ab9d7413884672e3af0ce3444adc8  src/cmd/aidlc/boundary_fixture_integration_test.go
7cee097ba92075f722206f99f788d89da33b2e1f9430a5a01836fdedaefbcb21  src/cmd/aidlc/boundary_live_integration_test.go
4fcfd623389494db2e708f1b38bd5213239043353c247c33b5ea1ded51f731b7  src/cmd/aidlc/configure_help_integration_test.go
aad4bfe59f9d287d01ee683179eeebfd64fc746d5695386d36ae77a6276d97c5  src/cmd/aidlc/execution_plan_evidence_test.go
96d6fca888dc84fbfcd59f2118efacd64905b8de28958d5a8c4399437cdecd64  src/cmd/aidlc/flow_command_unix_test.go
12622b57877ce2265a7e2a7cb6b83ff65a5b082aefbc56fed2792d8b2b3d33b2  src/cmd/aidlc/flow_journey_integration_test.go
3d9782d3a5ac240f73414199e01466d3f9f881feec5934d78ade18dd9ca2573a  src/cmd/aidlc/human_approval_live_integration_test.go
0a10941d403a161a8fb051ca6f43cc7fbe633cb2afc18e1969ec2f860e50595d  src/cmd/aidlc/operations_integration_test.go
93963e450297e0efbebf780b102893a6a3ae0b3c42cd2150810f7f66b1e0ab16  src/cmd/aidlc/procedure_evidence_test.go
37bdb29d40b683f389a5c3597dad84599bceffbd6f029c85538492d117343af0  src/cmd/aidlc/procedure_live_integration_test.go
b25fece0e856a0ed823d4bc1e17ae475a4385b1c106185a834bf43bf14ff2a4a  src/cmd/aidlc/relocation_integration_test.go
63d9215492a6bcefca6b77271369c7fb37335e2db3855634a4041fc3b5b81f5b  src/core/minimal/knowledge/rules/rule.md
dbc9b36e9db7fa06b314a135e1b0c0e4b89b136d8e7e4bad99bd4fec67289825  src/core/workflow/stage-graph.json
7dd6cf2cd9051150809c5fc4840360e6bba0b4c0126b76f4f72063244a7677a7  src/core/workflow/stages/architecture-analysis.md
f0f0eb2b3606c069e6645d855122f39c0294f9abc6228f0084f319e56fb23148  src/core/workflow/stages/discovery.md
d8883896a1bc078ce919208ed0169992bb1d5ecfb96fcf1d057b777999750ff5  src/core/workflow/stages/initialization.md
ce40123ca62565caa94d82c26ec760c8171ef94178e2b08ee285154869e0b1a7  src/core/workflow/stages/integration.md
7e7fb9b113bb37e90645becf414e362bad90175e9ffe59cc393fa5068f80e6bd  src/core/workflow/stages/planning.md
d38fa11c7ed051328b8e9d0abf612d92a9945d0d2526993492326a2236fca665  src/core/workflow/stages/tdd.md
a6f67f75dad154a514b710a4255871710515b570431afc5fe5e4309d02ead858  src/harness/codex/minimal/SKILL.md
d8fba422c3d4ddc234444cd5ba5808bd78c62262386746c67933506309dda5b1  src/harness/codex/minimal/WORKFLOW.md
bffcd3f92d3edc6ea23b3596cd63c81aa1ae4f556214a3e2ce1dc4221a07ab36  src/internal/cli/execution_plan_test.go
32326c451d7b085c9033a8ca4ba8733ec8a0d4b6262ace5d2144fb36d78acfc9  src/internal/cli/help.go
73cca5c36487b85d02ed6b1a7a73a0fd94b71429c6207461bb513f469ec0c12a  src/internal/cli/minimal.go
a975afa04f5b6fcf4db604a7709492453aa94c5081d0d1afb7c8f6aa44b3b0cf  src/internal/flow/approval.go
29b0b14f3013429d0287bcf4204c23dbb9ba3647210780c754337f5903c2d962  src/internal/flow/boundary.go
25f7309db7ee2e7ded141801b6636bd9bf9f1a06e413e0355aaac5eb64e0aaca  src/internal/flow/boundary_documents.go
a325ecab63bc9bb36c4b8a70912e85ba656faa98a66475fca8f3efb87be7d8b0  src/internal/flow/boundary_end.go
087f1fd8b90f5d2af6176a2e222e5c1a10d4cad6496b762ecaf1f51f49169be2  src/internal/flow/boundary_results_test.go
f824024de1b3b8ef645fe6afc076449cfe226eb6a0cc198a8130d7a12961bc91  src/internal/flow/boundary_review_test.go
fe51f53f1ae35afe60f9982573b4e1c4c47130bb15e0f7bb5eb5cdfb07f673d5  src/internal/flow/boundary_sensor_test.go
0a3efe1c54833f98e74a5155d93c5e47ea90b4f9d8bdc1a1f4380d0308beb43f  src/internal/flow/boundary_snapshot_test.go
75e9a8623dfd62894fe472b2c83405bfb391f1b9ee799f858bf4c7a95e890005  src/internal/flow/boundary_store_test.go
1b9992fee4ec04b9a2ef45a31ac1256c04d06278b3df2a5d27dcec73527ab603  src/internal/flow/boundary_transition_test.go
5b3201e52854f793c4c09748637b7ebea550f6fcbce1d8e90f3fe9865948b7e3  src/internal/flow/codekb_test.go
c0ede7637caf534a1b05a3f23a0b13c544193ca0d66105b254b62c43721fd14c  src/internal/flow/documents.go
179f2553cfb6e64e990e17e3c68338356b9b72c8ccbcd5aa1e5565f7c992bdd8  src/internal/flow/documents_test.go
d3b1a1958b18d880e7b01ea7a3729260b9f3136b257374467c555d980d3a7ec6  src/internal/flow/execution_approval_test.go
bdf5937209ff626dc976ca882f069e5711e6282a3a53e02d8ff96fb912f17bee  src/internal/flow/execution_history_test.go
1e7d494da05cbd5b2981535be4a31f3717ac2891076e10dd941fd5c3c2031070  src/internal/flow/execution_plan.go
6d97e233c3d3a0d89edab85de59daadc73e338a2e2a608a3b71b6999f90ce3b8  src/internal/flow/execution_plan_test.go
3b06e4290b7d2358603a7a2d030ec7808f9442bca1380592bf9d286b8adcd816  src/internal/flow/execution_reopen.go
f757aa274392b007caebceb0ac2d9696cb5db3673cdd5ab10a703946b2c148d9  src/internal/flow/fixture_execution_test.go
7ed0051527528c99066bfaa762ad65e0bd7ad92e98c68459d3624bf43fba59af  src/internal/flow/graph.go
20180d0c94b8e32b80d6d6840ad3ae63f79166a32cc583b4adc60337846feced  src/internal/flow/graph_test.go
839d04011cc4742a2560a3e71501b0c35b3dd551c9559eb19371b49f300461b9  src/internal/flow/history.go
790476999bd94bd9c536bce70bdb5b156a2709d0a5cad6cdcd3a32a7377904c3  src/internal/flow/lowercase_adr_test.go
eb919050360610185529ba2dc3eb4916a49932cdbb1b48ad3644bbcb0f363ac0  src/internal/flow/procedure.go
63df9cd22bcc4e6dc3cdb4288dbe4585472c074cfdda50272cb66af70913a0c6  src/internal/flow/procedure_test.go
10d4a0a6dac006646e40950922fb4855e224b8919aeede01b367bb29d844cc59  src/internal/flow/reassign_test.go
6494d661e9b41fff5d9972b36fec63cccd41e2aeeca206c88a00cfadd2c61a4a  src/internal/flow/reopen_log.go
16cbf00573f7efdd9ff7ac86e5ec952d05735aa20fb35e5cae8bb6537461c36d  src/internal/flow/reopen_log_test.go
0b30b129a4c6ba9c9fd513af5cc24eb8261164eb7000b8b426ecd7ece265bd12  src/internal/flow/review.go
74ff09fa34e1de2b447ea46c66a2f86f2851054b2e04f40bf111321326f033e4  src/internal/flow/selected_documents_test.go
29156a1e379f9d66a65c366a644a2fa933efacc46681178d1f3d0082ab08e974  src/internal/flow/sensor.go
dbffd4cb786c209697c68ee33c13f206fb45c287d76a7d03b4a917b543f04050  src/internal/flow/sensor_test.go
2c702641027e3736ab638ab50accf1cf8fd3c59cddced1ebc375351dc1181290  src/internal/flow/store.go
3882e7808725fe9d404f0ef1239de91566a63545e0d9e256a764426500d4f0ab  src/internal/flow/store_test.go
6676e795e947f299a1d71c9a6a1251f599805f102a9c508f898132448d61bcce  src/internal/flow/transition.go
ff06a78d2ae4fa666dbe1ef98ea0d3a98e66012cc3c8b1384cd3196823b7039c  src/internal/flow/transition_test.go
221d062d5001a24876385ebd219f0726ba78240b122292a1cc9b1d1c5f69983a  src/internal/flow/unit.go
dd70e6cff24cf8d760b12ab7ea5f70116a4eebaa7af616cc8f9b350777595979  src/internal/flow/unit_test.go
271f92d5e78148f1a27786ecc6b348364bb3cc25d1a1404f465fc15b5c0c0144  src/internal/flow/work_log_okf_test.go
b5d4151eac424b1cf75ce255269a2cd6c6c901ea1f5085bd43d21c2bc0fb9213  src/internal/install/codekb_test.go
b018d2c37d00b95e0ec932baa859ca9e8dc2ddd0f7bca862cc982fe9cc2b2174  src/internal/install/documents_test.go
12d3eb69cb4633cd6ae469ca7b97c44b4b39be9e52d4f18f705ecb739a8060c5  src/internal/install/execution_plan_test.go
8e796a9b2e7b3ea6bb4093f318e376248b903b299379c48e534ad26fee618334  src/internal/install/flow_test.go
78cbf55b0ebae9561fb0b5c831b17597875bf0ad02d8db7689c86d136b7af5b8  src/internal/install/install_test.go
45b6c0603f06f88f7c591aa551cf591cc497efe64fced69e7dce521ce37ecacc  src/internal/minimal/boundary_test.go
c8968e6cc2765c9433422098774a8657e86f3ba3f8fcb11878f25559ff4ca79c  src/internal/minimal/codekb_test.go
b9313dc0db0fb3c8c7fbcc9ff813c88c6d93d20b9ba50146fae732cf16b54eaf  src/internal/minimal/documents_test.go
2978dbb85e5a1cc85bdb992a63c6685f6963b0e652dae52b8cb497c77e7aae74  src/internal/minimal/execution_plan_test.go
77d01c6c631ba14d1766a5c1f8f482738b35133f58689d92e8f5ac90ff248147  src/internal/minimal/fixture_execution_test.go
dc011db6dfbcf729943dfbaaf64904b39bd40710e0326aa62428a9fe8926920f  src/internal/minimal/flow.go
1ec592a537fdb51cd0f5001cd6fd959f533fb983b19430901c2d51beea99cc00  src/internal/minimal/flow_test.go
72c4a9205fa97d705247a78d089c910d05d854695908b5d462ee743b5e3dd7e4  src/internal/minimal/hook.go
1379fb5bab30fcdd007978da1a64a4bcfb3afbb2553628349addd43ec10e5fda  src/internal/minimal/memory_help_test.go
cda72bbd93c8a377938f1218c087cf144829284d4bbef8572f8f253bbb0f1a7e  src/internal/minimal/session.go
225267066fc72d2411da3734ca8b00bf683faf592e132fd74a37c07071feeb64  src/internal/minimal/session_test.go
0486064d5f63b65687a9e28f73298cdde2e111366de0f9f413d43d67982ac8bb  src/internal/minimal/work_log_test.go
559a9d800e7b2c80735167c176247ff56ea78f20dbaf3fb92fc20008a00c68ab  src/internal/workflow/definition.go
cdbf40f13b510b11c289287cf62d52715ad885760513b164544e2b753e43ec25  src/internal/workflow/definition_test.go
e442700115a13bf980cbff1d74cf39b23020f57a3fc1ab6f4a6770bc4e6ca228  src/internal/workflow/execution_plan_test.go
```

変更Goファイルはgofmt適用済み。commit前に同ファイル群の `gofmt -l` が空であることと `git diff --check` exit 0を再確認する。
終了HEADはこの記録を含む実装commitとして親へのWORK_UNIT_READYに報告する。独立review/finalは未実施。
