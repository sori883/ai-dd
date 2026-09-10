# 現行製品の入口と利用者向け説明を揃える

- 状態: 「実案件 → 配布・更新 → 利用者文書」を順に進める直接依頼の3番目。
- 実装許可: [順次対応の依頼](2026-09-10-sequential-completion-request.md)。旧33 Stageの包括承認は使用しない。
- 計画: [利用者ガイド整備](../../design/user-guide-plan.md)。

実案件はPR #164、配布整備は[PR #166](https://github.com/sori883/ai-dd/pull/166)でmainへ反映した。
今回の基準は`eac541fa439da6533c2d4173eaec6500654ded4c`。実装管理は[Issue #167](https://github.com/sori883/ai-dd/issues/167)。
原checkoutの未commit資料、完了したpilotの進捗とKnowledge、runtimeを保全し、専用worktreeで文書を編集する。

## 更新する説明

READMEをCLI基盤だけの案内から、現行の導入・初回依頼の入口へ直す。
新規の`src/docs/user-guide.md`に、目的、Space/Intent、6種類の工程、5担当、Sensorとレビュー、
計画承認と成果承認、Knowledge/ADR/進捗の保存先、helpと再開をまとめる。
初期化と目的整理だけが必須で、その後の採否・順序はIntentの計画で承認する。
実装計画の本文作成とステージ計画担当の役割を混同せず、CLIがagentを起動するとも説明しない。

architectureの旧4段階固定順と大文字ADRを補正する。e2e-testingの旧手順は履歴として保持し、
冒頭から現行の検証案内へ誘導する。RAM索引の旧「現在」の要約を更新し、個々の過去決定本文は書き換えない。

これは実装済みの契約を説明する変更で、新しい製品挙動、保存形式、配布設定、依存関係は加えない。
本家2.6.123の確認済み範囲と既承認のGo/OKF方針を引き継ぎ、最新upstream全体との一致は主張しない。

## 試験と通常運用の区別

実案件では、承認待ちを実際に提示した後、外側AIが委任された試験用回答を通常UserPromptSubmitへ送った。
承認前の書込み拒否、許可外担当の起動前拒否、回答後の再開をmacOS/Codex CLI 0.153.4で観測した。
同じIntentは6段階を完了した。これを通常利用の自動承認や、すべての段階に対する人間の個別承認とは説明しない。

配布整備は6targetの梱包・照合と3OSのnative導入検査を含む。PR #166の24checksが成功し、
Issue #165のcloseとmerge commitがmainに含まれることを確認した。
ローカルfinalも固定head `14c4125c18d4a84095a10de425bd99121fc7e4a6`で17commandが成功し、
前後のsource bytesとGit状態は不変だった。初回のmacOS path別名によるfixture失敗を保全し、
小回帰のRED→GREEN・独立再review後にfinal全体を取り直した。証拠はmodule外の
`/Users/const/sori883/ai-dd-validation/distribution-165/`に保管した。配置file生成と実Codex hook動作は別の証拠である。
正式version、公開範囲、Go製品ライセンスは引き続き未確定とし、今回の文書変更で決定しない。
Windowsの実hook、既知のlinked-worktree探索やtool通知の課題も修正済みとはしない。

## 検証方針

文書のみのため人工REDや新Go testを作らず、変更したリンク、保存先、コマンドを現物へ照合する。
独立reviewで初心者の導線と現在の挙動への一致を確認し、read-only finalでリンク・掲載helpを検証する。
配布整備の検証済みbinaryを使用し、前後で対象文書のbytesとGit状態が不変であることを確認する。
最終結果、対象head、GitHub checksとmain反映は対応PRへ記録する。

独立reviewで、初回配置がhelp案内だけでは実行手順として不足すると指摘された。
実行ファイルと対象projectの絶対pathを指定する配置commandを追記した。
finalでは掲載helpに加え、配布検証済みbinaryから空のGit projectへ実配置する。
