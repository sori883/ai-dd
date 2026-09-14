# テスト削減M5完了

CLI一周・旧実験・診断と準備の34候補を整理し、[PR #213](https://github.com/sori883/ai-dd/pull/213)をmainへsquash mergeした。[Issue #212](https://github.com/sori883/ai-dd/issues/212)はclose済み。merge commitは `d8aa6cf3415e3cfe984dc2f530bd9da398cdddc2`。[結果](../../design/test-reduction-m5-result.md)に候補別の処置・生存保証・修復経緯を記録した。

実行ファイルの重複buildと不要なGit準備を減らし、共有するものを不変binaryへ限定した。Gitなしの直接作業とUnit作業を各一周残した。現在の実機調査は明示的なdiagnostic入口で維持し、旧実験と専用fixtureを削除した。OKF CLIへ保存・検索の検査を移し、自然日本語のstdin JSON検査は同じ配布候補で行う。製品Go・公開CLI・保存形式・権限の変更はない。

3担当の独立レビューで、C28のCLI終了コード、移転時の6path・生成hookの直接実行・binary pathの正規化、OKF更新競合の独立した拒否、CRLFのraw保存、不要なfixture分岐を修復した。限定再レビューで指摘解消を確認。初回CIはexit1という誤期待で失敗したが、既存CLIが返すexit2へテストを修正した。失敗を省略・skip・製品変更で通していない。

head `a21cd21e1410d50f3ebcfd0585d4ce36bff66a3b` の親によるread-only finalは全11群が成功。通常全package・race・vet・module・format・filesystem、Flow/GitIndependent/Assignment/Relocation/ConfigureHelpExamples、OKF/運用の保存復旧、小診断を確認した。手動記録未設定のReplayとOpaqueReceiptはskipであり、外部Codexの実機成功には数えていない。ログは `/Users/const/sori883/ai-dd-validation/test-reduction/m5-final-01/`。

修正版のPR [CI](https://github.com/sori883/ai-dd/actions/runs/34805390468)・[Distribution](https://github.com/sori883/ai-dd/actions/runs/34805390542)と作業branch側が成功した。6環境build、同候補の3OS Native/bootstrap、Windowsのpowershell.exe/pwsh.exeを確認した。Releaseやtagは公開していない。

[全6区切りの承認](2026-09-14-test-reduction-approved.md)に従い、最後のM6でCIの重複実行を整理する。
