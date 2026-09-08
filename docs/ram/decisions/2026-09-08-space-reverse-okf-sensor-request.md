# Space作成・既存資材の解析・Intentに対応するOKF Sensorの整理依頼

状態: 要求を受領し現状を確認。今回は整理・説明まで。以下の追加設計は提案であり実装計画の承認ではない。
基準HEAD/mainは2ee7fbed5c0fd3f908179dfd50610a943053968e（PR #139）。

## 保留する残対応

ユーザーは配布・更新の仕上げと利用者文書整理を残対応として保持し、今回の相談を先に進めるよう依頼した。
各OS向けリリース提供、配置資産の版更新・復旧/rollback、READMEと現行手順の統一が残る。
これらの実装・公開は今回開始しない。relocateは配置先の参照変更で、製品の版更新ではない。

## 新しい要求

- Space作成を利用の入口として使いたい。
- 既存フォルダや資材がある場合にリバースエンジニアリングしてOKF知識を整備したい。
- 解析結果や要件整理について、既定のOKF文書が存在するかをSensorで確認したい。
- 過去Intentの文書の存在だけで合格しないよう、検索・Intent ID・更新日時等を照合したい。
- まず既存ステージ、Sensor、担当AIの役割を整理して説明する。

## 現在のコードで確認した事実

- space createは公開CLIとCreateSpaceに接続済み。knowledge/ADR/design/rules等の初期資産を作成し、
  defaultのrules/rule.mdを継承する。既存Spaceを上書きせず、作成と選択は別操作。
- flowはdiscovery/planning/tdd/integration。調整役AIが作業と担当起動を管理し、Go CLIがstateと検査を担当。
  workerは別worktreeで実装・test、aidlc-reviewerは別root/sessionのread-onlyレビュー。
- intent checkは登録artifactの実在・OKF解析・type、ADR要否、未確定事項、計画、test成果物、Unit統合等を検査する。
  コードと成果物のhashで旧reviewを無効化するが、artifactのintent_id/generated.at一致は検査していない。
  種別別の現状解析・要件整理テンプレートを必須にする機能もない。
- memory search --intent-idは完全一致検索済み。検索結果には現在日時が含まれず、memory showのcontentで読める。
- memory create/updateはgenerated.atを自動設定。内容とmetadataが同一の日時だけ更新は拒否する。
  Intent/stageの開始時刻を用いるfreshness gateは未実装。
- 固定AI-DLC 2.6.123ではreverse-engineeringは既存project向けの条件付き工程でdeveloper→architectが担当、
  requirements-analysisはproduct担当。旧33Stageの工程/agent/receiptを現製品へ再導入する合意ではない。

## 整理案（未承認）

大枠の4段階を維持し、discovery内に対象指定・既存資材の読取り解析・OKF検索・要件整理を置く。
初期化は既存space createを再利用し、解析対象指定と解析手順を追加候補にする。
調整役が解析を実施し、大規模な場合だけ読取り調査を分担、統合してCLIでOKFを保存する。
解析の成果は現行の何/どうをKnowledgeへ記録し、設計変更を決めた理由だけADRへ記録する。
読み取った事実・根拠path/参照版・推測・未確認事項を分離し、元資材は解析によって改変しない。

Sensorの案は次の組合せ。検索候補だけで合格せず実ファイルを再読込みする。
1. 必須文書の役割、期待type/配置/必須節、対象SpaceとConcept IDを照合。
2. 今回の成果物はintent_idが現在Intentと一致すること。
3. 更新が必要な成果物はgenerated.atが今回の対象工程の開始以降であること。
4. 解析対象と参照版、成果物の現在hashがレビュー対象と一致すること。
5. 日時/IDだけの変更を内容の妥当性の証拠にせず、レビューで実物と照合すること。

日時はファイルmtimeではなくOKF metadataを利用する案。時計差や工程の再開/やり直しの境界は未確定。
現在stateには必要な開始時刻がなく、具体的な保存契約を別途決める必要がある。
共有の既存知識は参照可能なままにし、今回の成果物と単なる参照資料を分ける。
参照する全知識のintent_idや日時を毎回書換える案ではない。変更不要の再確認を合格させるか、
合格に必要な記録（verified等）をどう表すかは未決定。generated.atの空更新で代用しない。
全操作auditや独自receipt台帳は導入しない。これはSensor/レビューの設計案でありOKF一般仕様の必須条件ではない。

## 具体化時の未確定事項

既定文書の最小一覧と各節、既存資材の対象/除外と対応形式、参照だけと更新必須の区別、
同じ共有文書を複数Intentで扱う際の対応、鮮度の基準（Intent/工程/やり直し）、解析だけで完了するIntentの扱い。
現行flowはTDDと統合検証を経て完了するため、解析だけの完了を実装済みと説明しない。
複数案で結果が変わる点は具体例と影響を提示して確認し、コード/設定/Issue変更前に自己完結した計画へまとめる。

## 根拠

- src/cmd/aidlc/main.go、src/internal/workspace/space_create.go、workspace/flow_test.go
- src/internal/flow/sensor.go、transition.go、src/harness/codex/minimal/WORKFLOW.md
- src/internal/okfmemory/document.go、metadata.go、src/internal/minimal/command.go
- docs/ram/decisions/2026-09-08-four-stage-runtime-approved.md
- 固定参照 core/aidlc-common/stages/inception/reverse-engineering.md、requirements-analysis.md
