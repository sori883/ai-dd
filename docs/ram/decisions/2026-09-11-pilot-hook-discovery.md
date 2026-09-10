# 実案件環境のhook読込と独立Gitコピーへの補正

日付: 2026-09-11。Issue #163、承認済みの専用環境と通常hook有効化の範囲内。

固定Codex 0.153.4の通常起動で、専用worktreeの `/hooks` に製品5イベントが表示されなかった。
modelを起動しない `config/read` では専用project layerが有効かつtrustedである一方、
`hooks/list` はuser設定3件のみ、warning/errorなしだった。正しいmap形式のtrust指定と
`features.hooks=true` でも同じだった。stateは初期化の成果承認待ちを保持し、承認を代替していない。

比較用の通常Git repositoryでは、同じhooks.jsonの5イベントがuntrustedとして列挙された。
比較は列挙APIのみで、hook実行や製品完走の証拠ではない。
公開Codex [Issue #23996](https://github.com/openai/codex/issues/23996) は、linked worktreeで
hook専用の探索先がprimary checkoutへ向く問題を報告している。元checkoutにはhooks.jsonがなく、
今回の現象と整合する。公開報告のversionを今回の固定版ソースの直接証明とは扱わない。
通常の信頼手順は [公式hook仕様](https://learn.chatgpt.com/docs/hooks) に従う。

## 採用する復旧手順

専用worktreeを削除せず退避し、同じ絶対path・同じHEAD・全file内容の独立Gitコピーへ切り替える。
元checkoutへhookを置く案は他worktreeへ影響するため採用しない。trust bypassや手動trust台帳作成も行わない。
これは隔離環境を用意する承認済み作業の実施詳細で、製品仕様・人間承認・既存data互換性の変更ではない。
具体的な一致確認と復旧方法は [実案件計画](../../design/latest-workflow-pilot-plan.md) に記載した。

## 補足

通常Codexでは既存開発用agent3件が `invalid transport` で読込拒否された。製品のaidlc-*定義とは別であり、
この実案件で無断修正しない。Linear MCPも再認証要求で使用できなかったが、本案件の必須機能ではない。
外部toolの導入、認証変更、Codex更新は行っていない。

## 実施結果

旧worktreeを `ai-dd-latest-pilot-linked-backup` へ退避し、同じpathの独立cloneへ切り替えた。
HEAD `aeef3c9` と `.git` を除く1,339fileの内容・mode・symlinkが一致した。
同じCodexの通常UIで新規5hookが表示され、事前確認した専用pathとcommandを照合後にtrustした。
元checkoutのhooksやユーザー資料は変更していない。これによりworktree固有の探索問題の確度が高まった。

実session `01a08be5-f6fa-7490-9aca-4729726cd117` がSessionStart contextを受け、
Intent bindとRule読込みに成功。state revision4と同じ承認request/targetを保持した。
bind前のpwd拒否も実測した。approvalとfinishはまだ行っていない。

一方、読取りcommandの並列実行時にsession lock競合が発生した。読取りcatの実tool ID
`exec-533e006c-a711-4526-a3de-437af25571e3` は実transcriptでcompleted/exit0だが、
Session.Toolに残った。Postの不達と処理失敗は今回のログだけでは区別できない。
bootstrapのCodex自体もexit0を確認した。根拠付きの同session復旧と以後の直列呼出しで進め、
並列時の追跡問題を解決済みとは扱わない。help改善とは別の製品改善候補として残す。
生ログ・1339file照合・UI信頼操作・終端の抜粋は専用rootの `aidlc/evidence/latest-pilot/18-*`〜`21-*` に保存した。
