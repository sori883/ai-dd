# workerの作業場所を通常フォルダとして明記する

日付: 2026-09-12。状態: ユーザーの直接修正依頼。Issue #175（ユーザーリクエスト）の具体的な作業計画とする。

## 背景・許可

ユーザーは `src/harness/codex/minimal/agents/aidlc-worker.toml` に別worktreeを要求する記述が
残っていると指摘し、「対応してください」と依頼した。対象は元checkout
`/Users/const/sori883/ai-dd` の古いbranch `codex/human-stage-approval`、HEAD `f0e97d4` のファイルだった。
descriptionを「承認された担当範囲をTDDで実装する」に変更した未commit編集もある。

main `f8ee60c` の現行原稿 `src/harness/codex/agents/aidlc-worker.toml` は、PR #172で登録root・
集合SHAを使う定義となり、PR #174で移動済みである。別worktreeは要求していない。
今回の指摘は、保全していた元checkoutに旧記述が残ったことによる。

[Git不要・順次作業の承認](https://github.com/sori883/ai-dd/blob/f8ee60c88eb1f812ccf94535d88a10f0213aa8fa/docs/ram/decisions/2026-09-11-git-independent-implementation-approved.md)と今回の直接依頼を根拠に、
通常フォルダ・同一rootで一人ずつの実装という既存方針を両方のworker定義へ明記する。
新しい本家差分やCLI機能、旧版の互換機能を追加するものではない。

## 変更範囲・所有権

親エージェントが単独writerとして次だけを変更する。

1. 現行 `src/harness/codex/agents/aidlc-worker.toml` に、通常フォルダを使用でき、同じrootでは
   一人のworkerずつ実装する説明を加える。登録root、予約、停止確認、TDD、報告の条件は保つ。
2. 指定された元checkoutの旧パスでは、開始条件の「別worktree、base」を「対象root」に直し、
   同じ説明を加える。ユーザー編集済みdescriptionと他の未commit資料は保全する。
3. このRAMと索引を現行branchへ記録し、元checkoutにも同じ記録を追加する。

元checkoutの旧Go実装自体にはGitと別rootの検査が残っている。今回の文言補正だけでその旧CLIが
通常フォルダの新方式を実行できるとは説明しない。新方式の実装はmainにある。
元checkout全体のbranch切替・コード同期・既存利用配置の更新はこの局所修正に含めない。

## 受入条件・検証・完了手順

両定義に別worktreeを必須とする指示がなく、通常フォルダ・同一rootでの順次実装が明記される。
元のdescriptionと対象外ファイルの内容が保持され、両TOMLの構文を読めることを確認する。
現行原稿は既存の `TestProductAgent*` を対象に、配布された文面・5担当・権限が一致することを確認する。
文言だけの修正なので人工REDや新しいGoテストは追加しない。

独立read-only review後に、安定した差分でTOML構文・対象test・diff checkをfinal確認する。
CI/Distributionは既存設定のまま実行し、対象検査全成功後に現行branchのPRをmainへmergeしIssueを閉じる。
元checkoutの補正は未commitのまま残し、古いbranchをmainへmergeしない。
失敗時は対象箇所だけを再検討し、ユーザーの既存編集を巻き戻さない。
Goコード、保存形式、hook、外部module・toolへの変更はない。
