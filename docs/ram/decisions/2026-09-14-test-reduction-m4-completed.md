# テスト削減M4完了

CLI・Space・OKFの36候補を整理し、[PR #211](https://github.com/sori883/ai-dd/pull/211)をmainへsquash mergeした。[Issue #210](https://github.com/sori883/ai-dd/issues/210)はclose済み。merge commitは `271efb4b1e1030e7b25ef683ad6e49be1b859d35`。[結果](../../design/test-reduction-m4-result.md)に候補別の処置と生存保証を記録した。

共通処理へ入力検査を集約し、公開入口の短い転送確認を残した。ヘルプ全文の写しと用語の反復を削り、実path・必須H2・承認手順と実行できるJSON例を維持した。製品Goの変更は、呼出元のない旧org.md readerの削除のみ。現在のrule.md読取り、公開API、保存形式、権限は維持した。

独立レビューで空白付きpathの保持、未使用writerの削除、下位parserの拒否を直接必要とする入力代表の3点を修正し、同2担当が限定再レビューで解消を確認した。親の対象17群と修正後3対象名が成功。head `c1de52635202dc282228be22a5367ac24c113431` のread-only finalは全package・race・vet・module・format・diff・workspace/OKF filesystem・Flow/GitIndependent Journey・ConfigureHelpExamplesが全て成功した。ログは `/Users/const/sori883/ai-dd-validation/test-reduction/m4-final-01/`。

PR [CI](https://github.com/sori883/ai-dd/actions/runs/34802600268)・[Distribution](https://github.com/sori883/ai-dd/actions/runs/34802600277)と作業branch側が初回成功した。26検証check成功、公開用2jobは条件により非対象。6環境build、同候補の3OS Native/bootstrap、Windowsのpowershell.exe/pwsh.exeを確認した。Releaseやtagは公開していない。

[全6区切りの承認](2026-09-14-test-reduction-approved.md)に従い、M5のCLI一周・旧実験整理を別Issue/PRで続ける。
