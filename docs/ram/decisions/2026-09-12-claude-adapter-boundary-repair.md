# Claude接続の承認記録・故障時拒否・担当toolを補修する

2026-09-12。Issue #185の承認済み[D2計画](../../design/claude-code-connection-plan.md)を満たすための修復。C1〜C8と変更9packageのtestは成功したが、末尾確認で次の接続漏れが判明した。製品G0はまだ実施していない。

## 回答の登録成功

pending承認IDから消えたことだけでは登録成功を証明できない。計画を差し替えた場合も消える。また計画の修正依頼を登録するとDraftが消えるため、従来のState snapshotだけでは回答metadataが残らない。

既存HistoryRecordに任意の `approval_decision` を加え、plan/resultの区別と判断後のApprovalのコピーを最終State snapshotと一緒に確定する。Claude固有の値・hook形式は入れない。登録照会は現在stateの確定headから辿れるchainだけを読み、対象と回答証拠の全項目を照合する。孤児history、単なるpending消失、別sessionの回答は登録成功にしない。

読み取り専用の計画担当もこの案を確認した。adapter内の登録済みreceipt案は、state成功とreceipt保存の間で二重書込みの問題を増やすため採用しない。既存の承認判断履歴と、承認済みの回答保全要件の具体化であり、新工程stateや全操作auditの追加ではない。追加のユーザー確認は不要と判断した。

## hook失敗のnative接続

未知Claude toolを実binaryの `__hook` へ渡したところ、exit 1・stdout空だった。内部errorだけではnativeの操作拒否にならない。[Claude公式hook仕様](https://code.claude.com/docs/en/hooks)ではPreToolUseのexit 2またはdeny JSONが阻止を表す。Context7を優先して仕様を再確認した。

識別可能なPreエラーはClaude adapterがdeny JSONへ変換する。descriptor・読取り失敗などの入口障害でも `__hook` はexit 2とstderrを返し、通常CLIのexit 1は維持する。[Codex公式仕様](https://learn.chatgpt.com/docs/hooks)でも阻止可能なeventの失敗にexit 2を使える。SubagentStartやSessionStartで全動作を阻止できるという保証ではない。

## 必要toolの配置

Claude担当のfrontmatterにはBashがなく、OKF検索CLIやreviewerの限定検証を実行できなかった。researcherのWebSearch/WebFetchも不足していた。既存の担当責任を維持するため必要toolを配置し、既知の読取りをadapterから共通判定へ接続する。製品正本の子による更新は引き続き拒否する。追加MCP・権限を無条件に提供せず、Bash利用をOSの読取り専用保証と説明しない。

唯一のwriterが同じwork unitで順序付きTDDを実施する。core資材・外部module・既存利用者の配置は変更しない。修復後に対象確認、実製品の固定実機G0、独立review、read-only finalへ進む。

## loopの結果

補修後、全8項目の対象test、変更9package、integration tagの公開CLI protocolがexit 0だった。通常Submitが実session.Turnを変更しても質問回答を保持する対照、質問保存の競合・消失、登録済み履歴の孤児・別回答・破損chain拒否も確認した。厳密な子のRead経路で現在工程の資格確認を飛ばす漏れを回帰testから修正した。

ClaudeのWrite/Editから自分のsessionのDraftを作る接続も追加した。これがないと承認待ちにCLI入力JSONを用意できない。自分のDraft一件だけを許可し、別session・正本・他tool稼働の対照を確認した。

実装担当の返却はWORK_UNIT_READY。115変更fileのmanifest SHAは `551e195d2a11f8fec6a49c1c497d1ecc4cafa312a3d97e5f8acb0659583d5b2f`、HEADは `ec18517fe56ab61926430f71db6b5e53134cbe29`。sourceは未commit。実機G0・独立review・read-only final・PR checksは別gateであり、このloop結果だけで製品対応完了とはしない。
