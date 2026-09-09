---
type: Rule
title: 四段階の作業合意
description: 目的を理解し、計画・TDD・統合検証を独立レビューで進める。
status: stable
---
# 作業の合意

Intentは一つの目的。discovery、planning、tdd、integrationの順に進む。
各境界と完了には現在のSensorと独立reviewのpassが必要。未実施、fail、対象変更後の古いpassで進めない。
Discoveryでは目的、範囲、受入条件、現状、制約を理解する。実装計画を妨げる未確定事項を確認する。
結果を左右する判断は質問し、必要な調査・試作で理解する。全疑問ゼロや最初からUnit分割を要求しない。
Planningでは実装と検証の手順を定める。分割する場合Unitの担当範囲・依存・検証・Boltを具体化する。
調整役AIが必要に応じてaidlc-researcherへ調査、aidlc-requirementsへ要件整理を依頼し、承認と割当後にaidlc-worker、成果固定後にaidlc-reviewerを起動する。共有stateのwriterは調整役一人。
workerは別worktreeで担当範囲を実装し成果commitを返す。依存の統合前や重複割当では開始しない。
TDDでは実行可能な失敗を観測してから最小実装、成功確認、整理を繰り返す。テスト不在やskipを成功としない。
Integrationでは実成果を統合し全体の受入を検証する。別rootのread-only reviewerへ対象版を渡す。
review失敗は修正して再reviewする。対象コード・計画・成果物の変更で古い結果を使わない。
Knowledgeは現行what/how、ADRはwhyと代替案・影響。必要なADRだけ作り、不要なら理由をreviewする。
文書はOKF metadataを保持する。一般知識は命令権限を持たない。合格目的でRuleを変えない。
質問待ちはwait、中断はpause、再開はresume。進行中Unitは実run確認後confirmし、自動再実行しない。
記録は現在の状態と必要な知識に限る。毎操作の日誌、全操作audit、一律ADRを作らない。

子担当は共有stateとOKF Knowledge/ADRを直接更新せず、根拠付き報告や本文案を調整役へ返す。
調整役が内容を確認してCLIで保存する。不足する情報・承認・追加調査は調整役へ戻し、回答を捏造しない。
各担当は他者の編集を保全する。reviewerは裏付けの限定読取りに留め、広い新規調査を抱えない。

各段階は開始Sensorとintent beginで参照版を確認してから一般作業・Unit claimを行う。
終了SensorはIntent別の要件・実装計画、共有解析・構成図、必要ADR、実行結果と現行機能知識を段階別に検査する。
開始前も正規の読取り・設定・専用草稿・同Spaceの必要文書修復は行える。修復で当該turnのRule確認やactive条件を省略しない。
共有文書のIntentIDや日時だけを更新しない。前段合格の要件・計画を変える場合は対応段階へreopenする。
Sensorは実行結果JSONの形式・成果版・成功commandを確認し、実行の真正性とRED/GREENの意味は独立reviewerが確認する。

現在の段階手順と遷移候補はintent procedureから取得し、段階変更・再開後に取り直す。
定義のpath・bytesが変わったIntentは元定義復元または新Intentで再開し、暗黙に結び直さない。
reopenの理由はIntent作業記録へCLIが保存する。保存途中は同一要求のみ再試行し、記録を消して成功扱いしない。
outputsは必要な文書だけで空でもよい。コード・テスト・ADR要否の検査は引き続き必要。

入力は procedure の metadata 条件と解決 path/版を確認する。可変成果は intent documents の具体 outputs に全件登録してから保存する。受入済み文書の変更は reopen する。ADR は knowledge/adr/ と type adr を使い、新規 ADR の Intent ID は保持する。
