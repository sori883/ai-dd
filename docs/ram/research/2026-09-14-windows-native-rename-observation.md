# Windowsの配布試験で発生したfixture移動失敗

2026年9月14日。テスト削減M1のPR #205は、対象headのCI/Distributionと作業branch pushの全検証成功後にマージした。M1完了記録の「push側を含む26検証check」はこの作業branch側を指す。

その後のmain `a782bfbcb9abfa0352349911523e4baebb34a6ec` の[Distribution run 34795432742 / attempt 1](https://github.com/sori883/ai-dd/actions/runs/34795432742/attempts/1)では、Windowsの `TestReleaseCandidateNative` が試験用projectをmovedへ変更する `os.Rename` で `Access is denied` になった。`aidlc-install --relocate` を呼ぶ前の試験準備である。mainのCI run 34795432731は成功した。

読み取り調査では、子processはCombinedOutputで同期実行され、Go 1.26.4のWaitが出力とpipe・process handleを処理する。test自身のChdir、待機しないStart、明白な未Closeは見つからなかった。ReadFile/WriteFileも戻る前にCloseする。該当するtest/helper 3fileは、成功したPR head `a17f6ebe379b72dac41338489ec4a8c2684f5434` とmainで同じだった。

同じSHA・同じ配布候補の失敗jobを1回だけ再実行し、[attempt 2](https://github.com/sori883/ai-dd/actions/runs/34795432742/attempts/2)でWindows Native/bootstrap・PowerShellを含め成功した。公開用jobは条件により非対象。元の失敗を取り消したり、原因を解決したと扱ったりしない。

短いpath表記と長い表記、外部processによるlock等の可能性は、根拠が不足しており原因と断定しない。製品コードやtestに推測によるsleep/rename retryを追加していない。再発時はWindows error番号、移動先の有無、canonical parent、保持process/handleを追加診断する。M2の同headに対する独立review・新しいfinal/CIは別途すべて必要である。

失敗ログ: `/tmp/ai-dd-m1-main-distribution-failed.log`。この記録は不安定さの観測と確認範囲であり、新しい製品仕様の採用ではない。
