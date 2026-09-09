# 開始・終了Sensorの実装証拠

Issue #142、work_unit_id=start-end-sensors、verification_mode=loop。
[承認記録](2026-09-09-start-end-sensors-approved.md)と[正本計画](../../design/start-end-sensor-plan.md)の範囲で実装した。
最終の独立review・全体検証・実機証拠は親がPRへ記録する。この記録はloopの実測と保証境界を示す。

## 実装した契約

新規Intentをschema2とし、旧schemaを保持して明示エラーにした。開始checkはGit初期化や終了コードdigestに依存せず、
Rule、存在する共有文書、前段合格文書を検査する。beginはCASとlockで参照版を保存し、同段階の再試行は版を差し替えない。
終了checkは固定path/type/IntentID/必須節、共有解析とMermaid節、明示UTF-8資材、strict実行結果を検査する。
終了Sensor合格後だけreviewを割り当て、実報告の対象を照合し、advanceで直近acceptedを保存する。
pause/resumeはentryを保ち、reopenは対象段階以降を無効化する。material宣言変更はdiscoveryの再begin、後段はreopenを要求する。

一般hookとUnit claimは開始を要求する。同turn Rule・active・同Spaceを保った正規memory文書修復だけを開始gateから除外し、
一般shell、別Space、任意文書を修復扱いにしない。SKILLと4agent原稿、Go moduleは変更していない。
共有文書・現在段階の成果・資料を毎操作の不変入力とせず、requirements/planとintegration開始で使うTDD証拠だけを前段合格と照合する。
TDD資料directoryを成果証拠として誤固定しないよう、review digestのsourcesとaccepted outputsを分離した。

実行結果の計画成功条件は、直接実装ではconfig.tests、Unit分割時はUnit.tests。
Unit commandはそのUnit成果commitと対応させる。追加commandは現HEADとstrict形式・非空outputを検査して受理できるが、
必要な計画commandの成功の代用にはしない。独自shell engineやログ認証は追加せず、真正性・十分性は独立reviewが確認する。

## TDDの実測

各commandは `go test -count=1`。REDはcompile/skip/0件ではなく以下のassertionで観測した。
ログは今回実行環境の `/tmp/start-end-*` に保存した。

| 順序 | 対象とrun pattern | REDの理由 | GREEN/補足 |
| --- | --- | --- | --- |
| 1 | ./src/internal/flow `^TestBoundaryStore` | schema1のまま、不正entry/acceptedを受理 | schema2、旧bytes不変、CAS、path/hash/重複/段階検査。01-red/01-green |
| 2 | ./src/internal/flow `^TestStartSensor` | 空gate、開始入力未検査 | 初回不存在、合格版、資材版。02-red/02-green、02-material-red/green |
| 3 | ./src/internal/flow `^TestEndSensor` | 正式requirements/実行結果でも旧汎用Knowledge/test条件で失敗 | 文書/資材/strict JSON。03-red/03-green |
| 4 | ./src/internal/flow `^TestBoundaryTransition` | 未開始作業を許可、begin空返値、保存失敗を伝播しない | begin/CAS/idempotency/保存失敗/reopen/前段変更拒否。04-red/04-green |
| 5 | ./src/internal/flow `^TestBoundaryReview` | 未開始・終了不合格でもassign可 | 現在の終了合格と実報告targetを要求。05-red/05-green |
| 6 | ./src/internal/cli ./src/internal/minimal `^TestBoundary` | boundary/begin未認識、未開始一般操作許可 | 文法/help/修復/Rule読込/開始後操作。06-red/06-green、06-unit-red、06-root-red |
| 7 | ./src/cmd/aidlc `^TestBoundaryEvidence` | 空観測だけでlive成功扱い | 欠落Pre/Post、自己申告、禁止canary存在を拒否。07-evidence-red/green |

追加の有効REDは、資料directoryをintegration開始でfileとして読んだ失敗（04-material-red）、
FIFOを開いて検査が停止した失敗（03-fifo-red、02-fifo-red）、Unit commandと別Unit commitの混同（03-unit-red）、
Unit時の直接用tests余計な要求（03-unit-direct-red）、追加commandの不要な拒否（03-extra-red）。各修正後GREEN。
統合文書・他IntentIDを持つ共有文書・図欠落の回帰、壊れたsymlinkを不存在と混同しない回帰はALREADY_GREEN。
Git未初期化のfresh installでbeginする境界はTestBoundaryHookRepairAndBeginで確認した。

既存fixtureの旧schema期待、未開始状態での一般作業、終了不合格でのreview割当は新契約の前提へ更新した。
これらの旧前提による失敗はINVALID_TEST_FIXTUREとして扱い、製品REDとして数えない。
元のCAS、隔離、復旧、review対象、Unit依存/統合、閉pipe出力のassertは維持した。

## 末尾と実機の境界

7targetedの最終ログは `/tmp/start-end-boundary-01.log` から `07.log`。
affected通常testは `go test -count=1 ./src/internal/flow ./src/internal/cli ./src/internal/minimal ./src/internal/install ./src/cmd/aidlc`、
ログは `/tmp/start-end-affected-final.log`。7targetedとaffected全5packageはexit 0。gofmtとgit diff --checkも成功した。

実CLI4段階入口は次。既存TestFlowJourneyも同じ新契約journeyを保持している。

```text
go test -tags=integration -count=1 -v ./src/cmd/aidlc -run '^TestBoundaryJourney$'
AIDLC_BOUNDARY_LIVE=1 go test -tags=integration -count=1 -v -timeout 15m ./src/cmd/aidlc -run '^TestBoundaryLive$'
```

integrationとliveはloopでは実行していない。限定liveは固定Codex0.153.4、gpt-6-astra/medium、元の認証と通常sandboxを維持する。
既存TestFlowLiveHelper observerで製品hookを中継し、判断を変更せずraw/現在stateを記録する。
未開始の禁止canary拒否、必要文書の正規CLI修復、beginの保存、開始後canaryの許可を同session/Intent・Pre/Post・実CLI exitと現物で照合する。
モデル自己申告やtimeoutは成功としない。限定liveの検証範囲を4段階全体の実機完走と同一視しない。

## 親境界での実行証拠契約修復

work_unit_id=start-end-sensors-evidence-repair、verification_mode=loop。同じIssue142の受入違反を修復した。

1. 同じcommandを計画した2 Unitを、一方のResultCommitだけで満たすバグを再現した。
   TestEndSensorSharedCommandRequiresEachUnitResultは意図したRED後、command+ResultCommitの組ごとの成功照合でGREEN。
2. integrationで一部Unit統合時点のcommitを受理するバグを再現した。
   TestEndSensorIntegrationRequiresFinalHEADはRED後、全計画commandを現在HEADの成功へ統一してGREEN。
   全Unit統合済みの確認は既存終了SensorのUnit状態・祖先検査と組み合わせる。追加commandも現在HEADに対応させる。
3. 別段階のJSONをaccepted outputsへ混ぜるバグを再現した。
   TestEndSensorOtherStageDoesNotEnterAcceptedOutputsはRED後、別collectorで形式・必須field・出力を検査し、
   現在段階のfile hashへ混ぜない実装でGREEN。未来の未存在JSONを事前宣言せず作成後に追記する手順へ更新した。

各項目は `go test -count=1 ./src/internal/flow -run '^TestEndSensor'` でRED exit 1→GREEN exit 0。
ログは `/tmp/sensor-repair-{1,2,3}-{red,green}.log`。
末尾も同command、help変更は `go test -count=1 ./src/internal/cli -run '^TestBoundary'` で確認し、双方exit 0。
ログは `/tmp/sensor-repair-final-flow.log` と `/tmp/sensor-repair-final-cli.log`。gofmtとgit diff --checkも成功。
integration/liveはこのrepairでも実行しない。最終証拠は親PRへ記録する。
