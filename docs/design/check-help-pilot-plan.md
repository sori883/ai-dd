# 新Sensor付き実案件：check helpの必須文書案内

状態: Proposed。新Sensor付きで既存資材解析から統合検証まで1件進める依頼は受領済み。
具体的な改善対象として以下を提示し、ユーザーの選択を待っている。
基準はmainのPR #143、commit a8b9d5d71dd3cf47dc2c26b1ff8d73f9a27ac23b。

## 目的と利用者が得る結果

現在のintent check --helpは開始・終了の操作を説明するが、必須ファイル・文書型・本文見出しは
配布WORKFLOWを参照する必要がある。helpだけでも、各段階で何を準備すればよいか確認できるようにする。
この小さな改善を、実際の製品CLIと新しい開始・終了Sensorで4段階を通す実案件として使う。

## 変更範囲と受入条件

- src/internal/cli/help.go: intent check --helpへ4段階それぞれの開始・終了ファイル表、
  OKF typeと必須の第2階層見出し、共有解析/図の条件、ADR要否とテスト証拠の参照先を日本語で追加。
- src/internal/cli/check_help_test.go: 4段階、相対pathの基準、Intent IDの置換、型・見出し、
  初回discoveryとintegrationでの共有文書の違いを回帰検査。
- docs/designとdocs/ram: 計画、対象の承認、実装証拠、実案件の実施結果と索引を保持。

文書一覧は既存SensorとWORKFLOWの契約に一致させる。開始検査が作業開始の保存ではないこと、
終了Sensorだけでは進行せず独立レビューも必要なことを示す。既存helpの別名表記でも同じ説明を返す。
新コマンド、state形式、Sensor判定条件、外部Go moduleは追加しない。Go単一バイナリを維持する。
これは既存の公開契約の説明改善で、本家AI-DLCのSpace・配布へ新しい意図的差分を加えない。

## 実案件の進行

隔離したGit作業環境へ現在製品を配置し、専用Spaceと新規IntentをCLIで作る。
利用データはその作業環境のaidlc配下に保持し、本リポジトリ開発側のRAMや製品PRへ混ぜない。
親AIが共有stateとKnowledgeをCLIで保存し、調査・要件整理・実装・独立レビューを役割ごとに分ける。
小さな同一help変更なので実装は一人とし、Unitには分割しない。

1. discovery: 既存help、Sensorの文書検査、配布WORKFLOWをmaterial_sourcesへ指定し、start check→begin。
   調査担当は根拠付き解析を返し、共有knowledge/current-analysis.mdとknowledge/architecture.mdへ保存する。
   要件整理担当はその事実から要件案を返し、design/ID/requirements.mdへ今回intent_id付きで保存する。
   end checkと別root/sessionの独立reviewが合格したらadvance。
2. planning: start check→begin。design/ID/implementation-plan.mdへ対象ファイル、TDD順序、検証を記す。
   stateへ計画とtestsを設定する。アーキテクチャ/API/判定条件を変えないためADR不要理由を明示する。
   end check→独立review→advance。
3. tdd: start check→begin後に単独Go担当が回帰test→意図したRED→最小実装→GREENを実施。
   成果commitを固定して同commitで計画testを再実行し、実行結果JSONとログを作成してtest_resultsへ追加する。
   end check→独立コードreview→advance。
4. integration: start check→begin。同じ成果版で計画testとnative helpを確認し、
   knowledge/check-help.mdと共有解析/構成図を現行仕様へ更新する。
   TDD証拠を保持して別のintegration結果JSONを追加し、end check→独立review→advanceでcompletedを確認する。

reviewerのcheckoutは親と同じHEADおよびaidlc以外の対象bytesに揃える。
配置で新設した利用者用設定は隔離環境だけで管理し、追跡済みファイルをignoreで隠さない。
review割当後の変更は旧passを使わず再割当する。前段合格要件/計画を変更する場合はreopenする。
本文はAIが作り、frontmatterと日時はmemory create/updateに任せる。共有文書にIntent IDを一律付けない。
TDD証拠を後段用に書き換えず、未来の結果ファイルを事前登録しない。完了後RAMのcommitは成果commitと区別する。

## 実装と検証

対象選択後にIssueを作り、単独writerへwork_unit_id=check-help-pilot、verification_mode=loopで渡す。
順序は案内の欠落を検出する回帰test、help実装、既存help別名と文書条件の確認。
exact targeted commandは go test -count=1 ./src/internal/cli -run '^TestCheckHelp'。
同じcommandで各RED/GREENと末尾を確認し、親が差分全体と末尾testを一度確認する。

独立review後のfinalは全通常test、race/shuffle、vet、tidy-diff、format/diff、全integration、
darwin/linux/windows×amd64/arm64の6構成build、新しいnative binaryのhelpをread-onlyで検証する。
今回の実案件は本会話の親と独立agentが製品CLIを使って完走する確認であり、
新しいCodex会話の自動hook発火の実測とは区別する。前回の限定liveをhelp改善だけで繰り返さない。

## 許可と保全

実案件を進める依頼は受領済み。具体的なhelp改善の採用回答を得たら、この範囲のコード・Issue作成へ進む。
独立review・final・対象GitHub checks成功後はリポジトリ既定のmerge commitでマージしIssueを閉じる。
ユーザーの未commit AGENTS.mdとOKF実装参考フォルダ、既存の利用データは保全する。
配布更新・rollbackとREADME全体整理は別の保留項目として維持する。
