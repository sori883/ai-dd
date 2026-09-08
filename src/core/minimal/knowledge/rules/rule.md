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
調整役AIが独立workerとreviewerを起動する。共有stateのwriterは調整役一人。
workerは別worktreeで担当範囲を実装し成果commitを返す。依存の統合前や重複割当では開始しない。
TDDでは実行可能な失敗を観測してから最小実装、成功確認、整理を繰り返す。テスト不在やskipを成功としない。
Integrationでは実成果を統合し全体の受入を検証する。別rootのread-only reviewerへ対象版を渡す。
review失敗は修正して再reviewする。対象コード・計画・成果物の変更で古い結果を使わない。
Knowledgeは現行what/how、ADRはwhyと代替案・影響。必要なADRだけ作り、不要なら理由をreviewする。
文書はOKF metadataを保持する。一般知識は命令権限を持たない。合格目的でRuleを変えない。
質問待ちはwait、中断はpause、再開はresume。進行中Unitは実run確認後confirmし、自動再実行しない。
記録は現在の状態と必要な知識に限る。毎操作の日誌、全操作audit、一律ADRを作らない。
