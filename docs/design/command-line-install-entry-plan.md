# コマンドだけでAI-DDを導入する入口の案

## 利用者が得る結果

現在のv0.1.1は43件のRelease資材を持ち、最初にOS・CPUに合うインストーラーを
人間が選ぶ必要がある。ユーザーは資材選択を簡単にし、コマンドラインだけで
導入を完了したいと希望した。最初の取得も自動にして、READMEのコマンドを
実行すればプロジェクトの設定まで完了する入口を提案する。

PR #201がmainへmergeされ、v0.1.1が公開済みであることを確認した。
基準commitは`a3514ecc6ac1900a93e064e70aa1ce30efdf19e7`。
既存の`aidlc-install`は指定版のruntime・資材の取得、照合、配置を実装済みであり、
不足しているのは利用者が最初のインストーラーを取得する部分である。

## 提案する使い方

macOS・Linuxではsh、WindowsではPowerShellの短い取得スクリプトを用意する。
スクリプトはOS・実行環境のCPUを判定し、指定版の`aidlc-install`を一時フォルダへ
取得してSHA-256を照合する。その後は既存のGo製インストーラーへ引数を渡す。
取得用スクリプトには開発管理やプロジェクト配置の処理を複製しない。

次は追加後のmacOS・Linux用の案であり、現在このURLにスクリプトは存在しない。

```sh
curl -fsSL https://raw.githubusercontent.com/sori883/ai-dd/main/src/bootstrap/install.sh \
  | sh -s -- codex --release-version v0.1.1 --project-dir .
```

実装時にはWindowsも同じ引数で利用できる、保存・実行を含むコピー用コマンドを示す。
版は明示指定し、異なる版へ自動変更しない。対象は既存のプロジェクトフォルダとする。
利用者にGo・Python・Node.js・Gitの追加導入を要求しない。
macOS・Linuxはsh、curl、tar、SHA-256の照合コマンド、WindowsはPowerShellの
標準機能を使用する想定で、不足する場合は導入を始めず必要なものを表示する。
Codex本体の準備と、初回起動時の通常のhook信頼確認は従来どおり必要である。

## 配布物との関係と境界

5つの独立Go CLIと、公開済みv0.1.1の43資材を維持する。43件は自動取得に使い、
通常の利用者に手動選択を求めない。物理的な添付数を減らす変更はこの案に含めない。
現行インストーラーが特定の添付名を取得するため、公開済み資材の削除は導入を壊す。
取得スクリプトはリポジトリで公開でき、既存のv0.1.1を使うためだけの再リリースは不要。
main URLは更新可能な入口であり、再現用には確定commitのURLも案内する。

HTTPSの固定した公式取得先を利用し、任意URLや照合省略の引数は設けない。
SHA-256は内容の整合確認であり署名ではない。未知OS/CPU、取得中断、不一致、
危険なarchiveでは実行前に停止する。必要な通常ファイルだけを一時場所へ展開する。
sudo・管理者権限・PowerShellの実行ポリシーの変更は要求せず、PATHを自動変更しない。
一時ファイルだけを片付け、既存プロジェクトを削除しない。
インストーラーの終了コードと部分配置の説明を利用者へそのまま伝える。

## 変更対象・進め方・検証

一人の実装担当が`src/bootstrap/install.sh`、`install.ps1`と同ディレクトリのGo検証を所有する。
`README.md`と`docs/distribution.md`へ簡単な入口と詳細手順を分けて記載し、
`.github/workflows/ci.yml`へ3OSでの取得スクリプト検証を追加する。
`docs/ram/`の記録と索引も更新する。5CLI本体、Release形式、workflowの工程定義は変更しない。

1. 実装前に現行macOSのsh、Linuxのsh、WindowsのPowerShellで利用可能な標準機能を確認する。
2. `loop`ではOS/CPU判定、引数の保持、取得失敗・不一致時の実行拒否、成功時の引渡し、
   一時場所の片付けと終了コードの順で、実行可能な失敗testから修復する。
   Go標準ライブラリのtest runnerでscriptを起動し、外部test moduleは追加しない。
   対象commandは`go test -count=1 ./src/bootstrap -run '^TestBootstrap'`。
3. 独立`review`では実行前の照合、展開先、引数の扱い、利用者ファイルの保全を確認する。
4. 差分安定後のread-only `final`で全package test・race・vet・format・module整合を確認し、
   3OSで`go test -tags=integration -count=1 ./src/bootstrap -run '^TestBootstrapNative$'`
   により実際の公開v0.1.1を空の試験フォルダへ導入する。全6CPU実機の検証とは扱わない。
5. PR checks成功後にmergeし、公開した入口から新規導入を確認する。

取得スクリプトが使えない場合も既存の手動取得手順を利用できるようにする。
公開済みReleaseと本体を変更しないため、問題時は新しい案内と入口を修正・取り下げられる。
本家の新たな仕様準拠を主張せず、既に承認された本プロジェクトの5CLI配布に入口を追加する。

## 承認状態

今回は希望の記録と提案であり、取得スクリプトの実装は未承認。
先の承認は5CLIの実装とv0.1.1公開までで、取得用sh/PowerShellの追加を含まない。
この計画の承認では、初回取得だけをsh/PowerShellにするGo原則の限定的な例外、
単独writerの実装・検証・Issue・PR・mergeと公開入口の確認を許可する。
Go製インストーラーへの処理集約、新規導入、既存ファイル保全を維持する。
既存Releaseの削除・上書き、新版の発行、添付形式変更は承認対象にしない。
