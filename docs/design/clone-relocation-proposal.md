# 別cloneでAI作業を再開するための対応案

状態: Proposed。ユーザーは日常運用検証で判明した制約について「そう対応するようなことはできますか」と依頼した。
製品実装はまだ開始しない。新しい担当割当操作と配置済設定の書換え範囲を、この案で確認する。
基準はmain 82b711f。既存のIntent・Unit・Knowledge・ADRはGitで引き継ぎ、ローカルruntimeを複製しない。

## 提案する利用結果

別cloneでは、配置先設定を現在の場所へ更新し、停止確認済みの旧担当から新担当へUnitを引き継ぐ。
同じIntent IDとUnit ID、これまでの成果を保持する。旧PCのAI処理が動いているかはGitから分からないため、
移転操作が遠隔の処理を自動停止したとは扱わない。

### 1. 配置先の更新

公開CLI候補は `aidlc install codex --relocate --project-dir NEW_ROOT --from-project-dir OLD_ROOT --from-binary OLD_BINARY`。
新binaryは実行中のaidlc自身。旧pathを明示し、別の設定を推測で置換しない。
既存の製品hookと配置Skill内のbinary/root参照だけを更新する。Knowledge、ADR、rules/rule.md、
独自hook、Codexの認証・model設定には書き込まない。配置済製品資産の判別に失敗したら対象差分を示して停止する。
初回installは従来どおり。relocateはバージョン更新を兼ねず、現在版の既知の配置構造を対象とする。
適用前に全対象を検査し、現在bytesの競合検査と原子的file保存を用いる。
中断後は旧/新の確認済み参照を判別して同じ操作を再試行できる形にし、部分完了と残りを明示する。
移転先で別Intentを選択した際のhookが移転元stateを書き換えないことを検証する。

### 2. 進行中Unitの担当を割り当て直す

公開CLI候補は `aidlc unit reassign ID --space SPACE --expect REVISION --file REQUEST.json`。
入力はunit、新session、新worker root、引継ぐ現在commit、reason、previous_run_stopped=true。
実施主体のAIが旧処理の終了を確認した場合だけ停止確認を指定する。不明なら中断状態を保つ。
対象は既存のneeds_confirmation Unit。runningのままなら先に既存pause/resumeで明示的な確認待ちへ移す。
新rootのGit HEAD、baseからの履歴、依存統合、scope、他の割当との衝突を検査する。
ローカルruntimeへ新しいrun IDを発行し、Unitをrunningへ戻す。Intent/Unit ID・基準・計画・成果は保持する。
古いrun IDによるresultを拒否し、新担当は現在の成果を再読込・再テストしてからresultを提出する。
旧runtime自体はGit共有しない。レビューは移転先で別担当へ再割当し、新しい対象hashで受け直す。
遠隔PC間の自動排他や全操作auditを追加しない。新しい工程statusも追加しない。
書込み競合や保存失敗ではstateと割当の食い違いを成功と扱わず、再読込して回復できる保存順序をtestで確認する。

## 対象と検証

- src/internal/installとtest: 既知の配置参照だけ更新、独自編集保全、中断・競合・再試行。
- src/internal/flow/unit.goとtest: reassignの停止確認、ID/成果保持、Git前提、古いrun拒否、保存失敗。
- src/internal/cli、src/internal/minimal、src/cmd/aidlcとtest: 公開操作・help・hook認識・実CLI接続。
- src/harness/codex/minimal: 移転/再開手順。選択肢や引数はhelpへ集約。
- docs/development、設計、RAM索引: 新しい操作の説明と実測結果。

実装順序は配置更新→Unit再割当→CLI/help/hook→Git別cloneの一周と故障検証。
受入は元データ保全、移転先だけへの書込み、同一Intent/Unitでresult/integrateまで進むこと、
不明な旧処理・不正HEAD・scope超過・競合の拒否、失敗後の再試行、現在targetでの独立レビュー。
単独Go担当のtest-first実装と独立review後、全test/race/vet/integration/6構成buildをfinalへ集約する。
Go単一バイナリ、外部module追加なし。旧33 Stage dataの移行は行わない。

## AI-DLC準拠と確認範囲

固定AI-DLC 2.6.123の配布調査と同版aidlc-version.tsを確認した。
本家のharness配置先へ生成資産を配置する方針、Go単一バイナリへの内包という既承認差分を維持する。
このrelocate/reassignは現在のGo四段階製品の不足への提案であり、本家に同名同契約の操作があるとは主張しない。
本家の移転時の詳細動作は未比較。新しい意図的差分になる点があれば実装前に根拠と影響を提示する。

この案の確認対象は、既存資産を保持して参照先を更新し、旧処理の停止確認後に新担当へ明示再割当する方式。
新操作の実装許可を確認後、exact TDD commandと保存失敗時の契約を最終計画へ具体化する。
