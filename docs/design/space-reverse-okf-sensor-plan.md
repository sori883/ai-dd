# Space入口・既存資材解析・OKF Sensorの実装具体案

追記: 担当構成は後続の[4担当の承認](../ram/decisions/2026-09-08-product-four-agents-approved.md)で置換した。
以下の「製品専用agentは増やさない」は検討履歴であり、現行要件ではない。Sensor案と未回答2点は後続へ保留。

状態: Draft。構成への同意と実装依頼は受領済み。合否が変わる鮮度基準・変更不要文書の扱いは回答待ち。
基準はmain 2ee7fbed5c0fd3f908179dfd50610a943053968e。新Issueやコード変更はまだ行わない。
許可の根拠はdocs/ram/decisions/2026-09-08-space-reverse-okf-sensor-implementation-request.md。

## 背景と利用結果

Space作成CLIは実装済みだが、製品Skillはdefault SpaceからのIntent開始を主に案内している。
既存資材からの現状解析・要件整理を行う固定手順と、今回の成果物であることを検査するSensorがない。
既存SensorはOKFの実在・形式・コードと成果物hashを検査するが、文書のIntent IDと更新日時を照合しない。

新しい入口ではSpaceを作成または選択し、同じSpaceでIntentを作る。discovery（目的整理＋深掘り）内で
対象資材を読取り解析し、OKFへ整理する。Go CLIが必要文書・今回Intentとの対応・鮮度を検査し、
独立AIが内容を実物と照合する。合格後だけplanningへ進める。
解析のみでIntentを完了する新経路は追加せず、承認済み4段階を維持する。

## 必須文書の最小案

- CurrentAnalysis: 既存資材があるときの現状解析。対象範囲、現行のWhat/How、根拠、推測/未確認事項。
- Requirements: 各Intentの要件整理。目的、対象利用者、範囲/制約、受入条件、未確定事項、参照資料。
- ADR: 設計判断の背景、選択肢、採用理由、影響。既存の必要/不要宣言を維持する。

例のConcept IDはdesign/INTENT_ID/current-analysisとdesign/INTENT_ID/requirements。
OKF typeは上記の自由文字列を使う案。要件は未実装の要求なのでdraftとし、現行機能の説明と混同しない。
既存共有Knowledgeを毎回別Intent用に書換えず、今回成果物から参照する。
本文はAIが作り、frontmatterはmemory create/updateの引数とCLIの自動日時で生成する。
テンプレートの読取り用CLIを追加する案。公開文法は回答後の最終契約で固定する。

## 解析と担当

調整役AIが対象を指定し、既存OKFを検索、資材を読取り、本文を整理してCLIで保存する。
大きい調査は必要に応じて読取り担当へ分担し、調整役が統合する。
workerは実装を分担する場合の役割、aidlc-reviewerは別root/sessionのread-onlyレビュー担当。
CLIは担当の起動や解析そのものを行うschedulerではない。製品専用agentは増やさない。

初版案は明示したrepository内のソース・設定・test・Markdown等のUTF-8 text。
対象/除外を計画へ明記し、未読・非対応形式は説明する。PDF/画像等の新parserや外部moduleを追加しない。
元資材を解析のために変更せず、確認できた事実と推測を分ける。
repository外の資材も必要な場合は、具体的な読取り範囲と参照版の契約を実装前に確認する。

## Sensorの契約案

検索で候補を発見し、採用した文書を現在stateへ登録する。検索順位だけで自動合格にしない。
実ファイルからSpace、Concept ID、役割、type、必須節、現在Intentのintent_idを検査する。
文書のsourcesと解析対象path/参照版を照合し、現在の入力bytesもレビュー対象hashへ含める。
日付やIDだけでは内容の妥当性を保証せず、独立reviewが現物と比較する。
mtimeではなくgenerated.atを使用する。日付だけ更新する操作は既存同様拒否する。

質問1: 各文書を担当する工程開始以降か、Intent開始以降か。前者を親の推奨案として提示済み。
工程基準ならstateに工程ごとの開始時刻をCLIが保存し、advanceで次の基準を設定、pause/resumeでは維持する。
前工程の成果物には前工程の基準を使い、後工程に進むだけで再生成させない。
reopenは対象工程からやり直す境界にする案。過去の基準は履歴でなく現在の工程基準として管理する。
不正日時・未来時刻・時計逆転を黙って補正せず、文書不足と区別して診断する。

質問2: 変更不要の文書は参照資料のみとして今回必須文書を作成更新するか、今回Intentでの再確認を記録して
必須成果物へ数えるか。前者を親の推奨案として提示済み。後者なら確認主体・日時・対象hashの具体契約が必要。
本文snapshot、全操作audit、独自receipt台帳は作らない。

現在stateに時刻がないため保存契約の追加が必要。既存Intentを現在日時で自動補完して合格扱いしない。
既存dataは削除しない。旧Intent移行不要の合意を守り、対応schemaと診断を最終計画へ固定する。

## 対象fileと検証案

単独Go writer、1 Issue/PR・1 work unitを基本とする。

1. flow/store.goとtest: 工程時刻、対象資材、成果物役割/参照の型と厳密保存。
   go test -count=1 ./src/internal/flow -run '^TestDiscoveryContract'
2. flow/transition.goとtest: 作成・advance・pause/resume・reopen時の基準。
   go test -count=1 ./src/internal/flow -run '^TestArtifactFreshness'
3. flow/sensor.go、okfmemoryとtest: 必須文書・節・type・ID・日時・対象版。
   go test -count=1 ./src/internal/flow -run '^TestDiscoverySensor'
4. flow/review.goとtest: 対象文書/入力資材の変更で旧reviewを拒否。
   go test -count=1 ./src/internal/flow -run '^TestDiscoveryReview'
5. cli、minimal/hook・command、coreテンプレート、SKILL/WORKFLOW: Space入口、検索/登録/保存手順とhelp。
   go test -count=1 ./src/internal/cli ./src/internal/minimal -run '^TestDiscoveryCommand'
6. cmd/aidlcの実CLI integration、docs/development.md・RAM・索引。
   入口、過去Intent文書拒否、今回成果物合格、入力変更で旧pass拒否、共有参照不変を確認。

Go標準ライブラリと既存依存だけを使い、外部moduleなし。各項目はrunnable test先行RED→GREEN。
新型/APIのcompile-only scaffoldと実CLI prefixは契約確定後に明記する。
独立review後のread-only finalへ全test・race・vet・format・integration・6構成buildを集約する。
限定liveはSpace入口→資材解析→OKF作成更新→discovery検査とレビューを対象とし、元資材hash不変を検査する案。
配布版更新・README全面整理・既存利用dataの移行や削除は別の残対応として保持する。

## 固定本家との関係

固定AI-DLC 2.6.123のreverse-engineeringはdeveloper→architectによる9成果物とreceipt、
requirements-analysisはproduct担当である。今回の案は承認された4段階の中で調整役/独立reviewerと
最小OKF文書を使うもので、本家33Stageを再導入しない。この構成差と新しいSensor条件は計画へ明示する。

回答後に未確定欄を採用契約へ置換し、ユーザーが読む具体計画とIssue本文を一致させてから実装へ渡す。
