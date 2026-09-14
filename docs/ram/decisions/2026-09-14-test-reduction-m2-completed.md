# テスト削減M2完了

配布・配置資材・自然日本語の26候補を整理し、[Issue #206](https://github.com/sori883/ai-dd/issues/206)を閉じ、[PR #207](https://github.com/sori883/ai-dd/pull/207)をmainへsquash mergeした。merge commitは `91b54e1ae7555f9bc422a1ad185830b714668366`。削減と生存保証は[M2結果](../../design/test-reduction-m2-result.md)に記録した。

独立レビューで名詞終止の句読点保証の欠落と3つの不要な検査・準備を修正し、head `44b53ddcde51176a68ac45ffbae674e6f404668a` の限定再レビューでblocking findingなし。親の対象18コマンド、修正後3コマンド、差分安定後の全package・race・vet・module・format・diff・workspace/OKF filesystem・Flow/GitIndependent Journeyは全て成功した。ログは `/Users/const/sori883/ai-dd-validation/test-reduction/m2-final-01/`。

PR [CI](https://github.com/sori883/ai-dd/actions/runs/34797773007)・[Distribution](https://github.com/sori883/ai-dd/actions/runs/34797772924)が成功。6環境の配布build、3OSの実CLI・導入・移転・bootstrap、Windowsのpowershell.exe/pwsh.exeを確認した。作業branch側を含む26検証checkが成功し、公開用2jobだけ条件により非対象。Releaseやtagを公開していない。

## Windows試験の再発観測

M1の[移動失敗](../research/2026-09-14-windows-native-rename-observation.md)と同じ `os.Rename` のAccessDeniedが、M2作業branchの[Distribution attempt 1](https://github.com/sori883/ai-dd/actions/runs/34797763773/attempts/1)でも発生した。同じSHAの失敗jobを1回だけ再実行し、[attempt 2](https://github.com/sori883/ai-dd/actions/runs/34797763773/attempts/2)で成功した。同じ変更のPR側は初回成功したが、PRのmerge SHAとbranch headは別の候補になり得る。

原因は未特定であり、解決したとは扱わない。追加の読み取り検討でもcanonical化やCopyFS/RemoveAllが原因を修復する根拠は得られなかった。CopyFSはinode・時刻・ACL等をそのまま保つ操作でもなく、RemoveAllも同じ保持状態で失敗し得るため、推測で試験準備を変更していない。失敗ログは `/tmp/ai-dd-m2-distribution-failed.log`。sleep/rename retryや検査のskipは追加していない。

## 次の区切り

[全6区切りの承認](2026-09-14-test-reduction-approved.md)に従い、M3の工程・担当管理テストを別Issue/PRで続ける。M2の公開API・保存形式・権限・配布契約は維持した。
