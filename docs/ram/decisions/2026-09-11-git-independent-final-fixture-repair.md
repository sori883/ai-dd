# Git不要化のfinalで検出したfixtureとhelpを修復

日付: 2026-09-11。Issue #171。開始HEAD: `78468b467464bf7386c7e80b825538d91b74b52d`。
[承認済み計画](../../design/git-independent-workflow-plan.md)と[独立review修復](2026-09-11-git-independent-review-repair.md)の後続として、親のread-only final-1の全package testが検出した不整合をloopで修復した。Git必須条件や旧形式互換を復活させない。

assignmentの初期化testは現在schema 2を期待する。flowの不正schema testは実際のschema 6から旧schema 5へ書き換え、拒否を検査する。置換元が存在しない場合にはfixture自体を失敗させる。これらは古い期待値・無効だった入力生成の修正であり、productionのschema契約を変更しない。

check helpの回帰はHEAD・commitの要求を現在集合SHA、Unitの内容照合・反映、全体検証の説明へ置換した。confirm helpは登録runの現在root内容と64桁verification_sha256の確認を明記した。configure helpには既存Knowledgeをartifactsへ記載する `aidlc/spaces/default/knowledge/codekb/current.md` の具体例を復元した。Knowledge例の削除はGit不要化の要件ではなかった。

`TestCodeKBHelp` の既存失敗をtargetedで再現し、同じtestをGREENにした。confirmの旧期待を現行契約へ置換した `TestMemoryHelpUnitConfirmVerification` でも説明不足のREDを観測し、追記後にGREENを確認した。旧期待の置換だけのtestについて人工REDは作らない。

検証は `TestRegistry`、`TestDefinitionBindingMalformedState`、CLIの `TestCodeKBHelp|TestCheckHelp|TestMemoryHelpUnitConfirmVerification|TestConfigureHelp|TestGitIndependentInstall` のtargetedに限定し、すべて成功した。gofmtと差分検査も実施。全体final、race、vet、journeyは親の再review後に再開するgateであり、このloopでは実行していない。
