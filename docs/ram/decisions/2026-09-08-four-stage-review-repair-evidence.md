# Issue #130 独立レビューの一括修正

work_unit_id `four-stage-review-repair`、verification_mode `loop`。
開始HEAD `62d8355a77c912f8971bbe25e367021c80d44d51`。
[修正計画](../../design/four-stage-review-repair.md)に従い単独writerで修正した。
元の承認済み四段階契約を満たす修正であり、外部module・権限・製品schedulerを追加していない。

| 指摘 | 有効な回帰RED | 修正とGREEN |
| --- | --- | --- |
| configure進捗偽装 | TestFlowConfigureCannotForgeProgressで新integrated/running、既存進捗変更、reported除去が受理された。 | 新Unitはpending/空結果。既存進捗は保持し、未回収Unitを保全。configure prefix成功。 |
| 後続base不一致 | TestFlowUnitRejectsPreIntegrationBaseで依存統合前baseが受理された。 | Git祖先関係を確認。Unit prefix成功。 |
| reviewer checkout | TestFlowReviewRejectsWrongCheckoutで空directoryが受理された。 | assign/acceptで実Git root・HEAD・コードbytesを照合。review prefix成功。変更後checkoutの拒否coverageはALREADY_GREEN。 |
| artifactの自己循環とstage | TestFlowSensorArtifactStageAndAuthorityでstate/runtime・未知kind/stageが受理され、将来成果物が存在必須だった。追加で将来pathの../もRED。 | path/kind/stageを検査し、現在までの成果物だけ内容検査。Sensor prefix成功。 |
| Unit ID | TestFlowSensorUnitIDsAreComponentsで../、slash、backslash、dotを受理。 | 英数字先頭、英数字/underscore/hyphen、80文字以内。Sensor prefix成功。 |
| live偽証拠 | TestFlowCommandRejectsInventedReviewEvidenceでCLIなし・echo・exit偽装・別report/root/target・checkout hash欠落を受理。TransportReviewEvidenceは空実装で実transportを取得できなかった。 | 実command_execution exit/outputとPre時点のrequestをsession/正規化command/順序で結合し、実reportと受理stateを照合。Command prefix成功。 |
| 日本語手順 | TestFlowInstallJapaneseProcedureで全文への入口と配置先が欠落。 | 日本語SKILLとWORKFLOW.mdを配置。4KiB capと読込経路を検査しinstall prefix成功。翻訳自体の人工REDは作っていない。 |

review用nonlive fixtureは実worktreeを準備し、必要な非sharedコードbytesを一致させる。
live fixtureは準備用.gitignoreをcommitし、制御用draftをignored/runtimeへ置くよう明示した。
shared Knowledge/ADRは調整rootから読む運用を維持する。

liveの受理証拠は、モデル自身の `item.completed` / `command_execution` の終了codeと出力を使う。
旧M1の保存済み0.153.4 JSONLでこのwire shapeを確認した。別host CLI実行をモデルの実行と扱わない。
未知のcommand/transport形で必要な証拠を結合できない場合は判定成功にしない。
reviewerのcommit/コードbytes hashと実reportを保存し、CLI受理のsession/root/target/status/summaryと照合する。

最終targetedログは `/tmp/ai-dd-flow-repair-{configure,unit,review,sensor,command,install}.log`。
全command exit 0。configure commandはminimal側の実testが実行され、flow側には同prefix testがない。
影響package flow/minimal/cli/installのpackage testもexit 0 (`/tmp/ai-dd-flow-repair-affected.log`)。
integration tag付きTestFlowCommandはcompile/targeted成功。fresh一周・live・全project/race/vet/crossbuildは未実行。

親finalの入口は変更なし。

```sh
go test -tags=integration -count=1 ./src/cmd/aidlc -run '^TestFlowJourney$'
AIDLC_FLOW_LIVE=1 go test -tags=integration -v -count=1 -timeout=35m ./src/cmd/aidlc -run '^TestFlowJourneyLive$'
```

固定sandbox下のGit準備・実bytesのcommit・統合は承認済みtest host支援であり、モデル自身のGit成功とは区別する。
AGENTSのユーザー差分、参照資料、過去RAMを保全した。
