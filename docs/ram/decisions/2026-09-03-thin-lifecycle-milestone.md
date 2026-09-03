# 薄いライフサイクルを内部APIで完走するマイルストーン

- 日付: 2026-09-03
- 状態: Accepted
- 承認: ユーザーが、残り6〜7個程度の小さなPRで現在の薄いライフサイクルを最大範囲まで進めることを明示承認
- 包括承認枠: [AI-DLC Go実装ロードマップ](2026-09-03-aidlc-implementation-roadmap.md)と[ロードマップ単位の包括承認](2026-09-03-milestone-authorization-and-autonomous-merge.md)

## 背景と目的

Go版AI-DLCは、Intent作成、Stage Planと初期stateの保存、保存済みstateの読み取り、現在Stageのdirective解決、通常Stageの成果物存在確認まで実装済みである。一方、Stage完了条件、承認、state更新、次Stage、workflow完了はまだ接続されていない。

このマイルストーンでは、33 Stageの個別処理を作り込む前に、注入済みのStage graphと信頼済みhuman receiptを使う内部APIで、次のwalking skeletonを永続stateとaudit付きで完走させる。

```text
StartIntent
  → Next
  → 通常Stageの成果物・完了条件確認
  → approval gate
  → trusted HUMAN_TURN receipt
  → approve
  → state advance
  → 次Stage
  → workflow complete
```

## 固定する境界

- 比較対象はリポジトリに固定されたAI-DLC `2.6.123`の確認済み範囲とする。
- state更新はraw Markdownの対象箇所だけを置換し、未知section、未知field、comment、未変更bytesを保持する。
- 同一recordの遷移はrecord単位lockで直列化し、lock内でstateを再読込する。
- auditをstateより先にappendし、stateは同一directoryのtemporary fileをcloseしてからrenameする。
- audit失敗時はstateを変更しない。audit成功後にstate置換が失敗した場合、先行auditが残り得る本家の非対称durabilityを維持する。
- 保存済みStage suffixの`EXECUTE` / `SKIP`をrouting authorityとする既承認のfail-closed差分を維持する。
- 通常Stageだけをwalking skeletonの実行対象とする。per-unit、CodeKB、未実装のsummary／pipeline／review／sensor能力を要求するStageは通過させない。
- 外部Go moduleを追加しない。

## PR分割

1. Stage完了可否のread-only判定と必要なgraph metadata
2. byte-preserving state transition patcher
3. 既存stateのatomic update writer
4. 最小audit ledgerとrecord単位lock
5. approval gate遷移とtrusted human receipt
6. approveから次Stageまたはworkflow完了までのtransaction
7. 内部`Next` / `Report` facadeとlifecycle E2E

各PRは、自己完結した計画、`機能開発` Issue、単独writerのTDD、固定base/headの独立review、安定差分のfinal検証、GitHub checks成功、merge commit、Issue closeを個別に満たす。

## このマイルストーン後も確認が必要な境界

公開`aidlc next/report` CLIは、production `DataFS` / `ScopesFS`の供給元が未決である。binary埋込み、配置版data、flag/config指定では更新・改ざん・運用契約が異なるため、このマイルストーンでは選ばない。

trusted `HUMAN_TURN`のproduction取得元も未決である。CLI引数を人間の権限証明として扱わず、Codex hook、対話入力、nonce付きreceiptなどの選択は公開接続前に確認する。

現在のproduction graphはsummary、review、sensor等を要求する。dispatcherとreceiptが未実装の間はfail-closedとし、それらを省略する恒久仕様は採用しない。

