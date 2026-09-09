# Intent実行計画のintegration fixture修復

Issue153、work_unit_id=intent-execution-plan-integration-repair、verification_mode=loop。
開始HEAD: `639cbcd5b27f96f17e59337297fb8746ddac8d0a`。親finalで再現したfixture失敗を、既存承認範囲で一括修復した。

再現根拠: `/var/folders/9w/921pjkys39q28sk4xsc0hs000000gn/T/ai-dd-execution-plan-final-c113nhup/integration.log`。
親の `go test -tags=integration -count=1 ./...` が失敗した出力を確認した。今回はtest-only補正なので人工的なREDは作っていない。

- runBoundaryJourneyのNoMaterialsReason等configureを、正規initialization begin→独立review→試験回答capture/approval→finishの後へ移した。共有journey4件の同根修復。
- OperationsGitHandoff/UnitConflictsの直接エラー期待要求へ現在step_idを付けた。runtime欠落、既存割当、scope重複、依存未統合の元の拒否とstate不変のassertionを保持した。
- OperationsSaveRecoveryは成功retryのrevisionを操作前revision+1で比較する。失敗時state bytes不変のassertionを保持した。

HumanApprovalLiveは既にinitialization完了後にdiscovery configを設定していた。configure-helpとoperations共通create、Unit helper経由の要求も確認した。
他integration fixture内の固定revision比較/直接Unit requestを検索し、同類の残存を確認していない。製品コード・Sensor・保存境界は変更していない。

指定確認: `go test -tags=integration -count=1 ./src/cmd/aidlc -run '^TestExecutionPlanDistribution'` — exit 0（tool chunkbd25b5、0.432s）。
これはタグ付きfixtureのcompileとunit validatorのみ。journey/E2E/live本体は起動していないため、修正後のintegration完走を主張しない。
full/race/vet/buildも未実行。親が独立review後にfresh finalを行う。

変更fixtureのSHA256:

```text
da3fb3096a26ab0177630586d6fcefd7c9c1c923f97ecdb393c8b17f86448196  src/cmd/aidlc/flow_journey_integration_test.go
72054e1d3c49a894a701223e2a5b7fcebe9687340997aad542bbd84025c72fc2  src/cmd/aidlc/operations_integration_test.go
```

変更Goはgofmt適用済み。末尾にgofmt -lが空、git diff --checkがexit 0であることを確認する。終了HEADは本記録を含む修復commitとして親へ報告する。
