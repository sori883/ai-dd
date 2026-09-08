# KnowledgeのfrontmatterをCLIで生成し、ヘルプを操作の正本にする

状態: Accepted。ユーザーは本文とmetadataを分ける前案に「はい、お願いします」と回答し、
型やstatusの選択肢をhelpへ掲載し、Skillが迷った際にhelpを参照する実装を直接依頼した。
旧ロードマップの包括承認は流用しない。本計画はその依頼の具体化である。

## 背景と利用結果

現在のmemory create/updateはAIがfrontmatter付きMarkdownを作るため、項目や書式が草稿に依存する。
新方式ではAIは本文を書き、CLIが引数からfrontmatter（文書先頭のmetadata）を生成する。
Knowledge・ADR・Ruleで共通利用し、現在のOKF v0.2文書形式・Space配置・intent_id検索を維持する。
ユーザーとAIはヘルプから有効な値・必須項目・更新方法を確認できる。

## 公開操作

```sh
aidlc memory create knowledge/authentication --space default \
  --type Design --title '認証の仕様' --description '現行の認証方式と利用手順' \
  --tag authentication --status stable --actor process:codex --body-file body.md
aidlc memory update knowledge/authentication --space default \
  --body-file body.md --actor process:codex --expect <現在のhash>
aidlc memory create --help
aidlc memory update --help
```

| 引数 | 入力と意味 |
| --- | --- |
| type/title/description | 作成時必須の非空文字列。更新時は指定した項目だけ変更。typeは自由な非空文字列で、Design/ADR/Rule等は例示であり列挙型ではない |
| actor | 作成・更新時必須。generated.byへ保存。OKFのproducer/version、human:id、process:idの書式を説明する |
| body-file | 本文だけのUTF-8ファイル。既存のpath・容量境界を維持。先頭のfrontmatterを拒否し、本文途中の水平線は許可 |
| tag | 繰返し指定で文字列配列。作成時省略は空。更新時省略は保持、指定すれば配列全体を置換 |
| clear-tags | 更新時のtags消去。tagと同時指定しない |
| status | draft/stable/deprecated。作成時省略は既存OKFと同じstable、更新時省略は保持。工程完了や検証合格とは別 |
| intent-id | 32桁の小文字16進数。省略時、作成は未設定、更新は保持。共有Ruleへ一律付与しない |
| resource/stale-after | 既存OKFの文字列/明示UTC offset付きISO 8601日時。指定時だけ設定 |
| sources-json | OKF sources配列をJSON引数で指定。resource必須など既存検査を適用 |
| verified-json | OKF verifiedのobjectまたは配列をJSON引数で指定。実施済みの確認者・日時を明示し、自動で検証済みにしない |
| metadata-json | 未知拡張項目のobjectをJSON引数で指定。標準項目・generated・intent_idの指定を拒否し、専用引数との二重入力を防ぐ |
| expect | 更新時必須の現在hash。競合時に保存せず再読込を案内 |

CLIはgenerated.byをactor、generated.atを内容または明示metadataの変更を保存する現在UTC時刻とする。
generatedはユーザー入力で上書きしない。出典のlast_modified、verified.at、stale_afterは意味の異なる日時であり、
現在時刻で補完しない。既存metadataと未知キーは更新で保持する。本文とmetadataが全く同じなら保存を拒否し、
generatedの時刻更新だけを変更に数えない。metadataのみの明示変更は有効な更新として扱う。
JSONは不正型・重複key・末尾dataを拒否する。最終的なYAML出力は既存serializer/parserで検査する。

memory create/updateの旧--fileによるfrontmatter付き草稿経路は--body-fileへ置換する。
旧flagには新しいhelpへの案内を返す。Intent/UnitのJSON入力用--fileは維持する。
既存文書を移行・削除しない。新旧の二重書込み経路は設けない。

## helpとSkill

root、公開command group、公開actionのhelpを厳密に識別する。
--helpとhelp形式で辿れ、未知commandのhelpやhelpに書込み引数を混ぜた形を正常な読取りと扱わない。
memory create/updateのhelpは上表、必須/任意、省略時、例を日本語で自己完結させる。
Intentのstage/status、reviewのstatus、Unitのstatusなど既存の選択肢も該当actionのhelpで説明する。
helpはSpace、Intent、Rule、本文ファイルを要求せず、stdoutへ表示してexit 0。実行callbackは呼ばない。
hookでは同一CLIの正規helpだけをread-only例外とし、未選択・Rule未読・inactive・稼働slotありでも参照できる。
helpでsession/Rule/tool slotを書き換えず、redirect・compound・別binaryを例外として許さない。

配置SKILL/WORKFLOWは短い新操作例と「引数・選択肢に迷ったらhelpを参照」を記す。
enum一覧をSkillとhelpで二重管理しない。4 KiBの入口上限、Rule全文確認、作業gateを維持する。
入力parserエラーはhelpへの案内を返すが、既存の実行時CAS/stale等の診断を変更しない。

## 対象と実装方法

単独Go実装担当が以下を一つのIssue/PR・work unit `knowledge-cli-frontmatter` で変更する。
Go単一バイナリ、標準ライブラリと既存承認済みYAML moduleのみを使う。外部module・toolを追加しない。

- src/internal/cli/{cli,minimal}.go、必要なhelp.goと対応test: 引数presence、繰返しtag、help、旧flag案内。
- src/internal/okfmemory/metadata.goとtest: metadata生成/patch。内部MetadataInput型とBuildMetadata関数を導入可能。
- src/internal/minimal/{command,hook}.goと対応test: 本文保存、CAS、正常help例外。既存session_testの旧草稿fixtureも更新。
- src/harness/codex/minimal/{SKILL,WORKFLOW}.md、必要時core/minimal Rule: CLI入力とhelp参照。
- src/internal/installの配布test、src/cmd/aidlcの実CLI/help/配布integration・限定live fixture。
- docs/development.md、関連公開契約の現行操作説明、docs/ram記録と索引。

bundle lock、原子的文書保存、index/log更新、文書保存後の補助処理失敗の報告を維持する。
本文が変わっていない更新の扱いと旧--file例は、この新しい承認で置き換える。
state遷移・Sensor契約・Unit scheduler・配布更新方式を変更しない。
本家AI-DLC 2.6.123のSpace/配布の既承認境界に新しい意図的差分を追加しない。

## 順序付きTDDと受入条件

新test名は以下prefixで追加する。新しい内部型/関数が必要なら、MetadataInputとBuildMetadataの
宣言・空の返値だけのcompile用scaffoldを許可する。振る舞いをRED前に実装しない。

1. C1 cli: 引数の必須・省略・型・重複・tag・旧file拒否。`go test -count=1 ./src/internal/cli -run '^TestMemoryMetadataCLI'`
2. C2 okfmemory: YAML安全生成、日時自動、標準/未知metadata保持、JSON型/重複/予約key、no-op。`go test -count=1 ./src/internal/okfmemory -run '^TestMetadataInput'`
3. C3 minimal: 本文のみで作成/更新、metadata-only変更、日時とactor、CAS/保存失敗、前文拒否、索引の整合。`go test -count=1 ./src/internal/minimal -run '^(TestMemoryBodyWrite|TestSessionMemory)'`
4. C4 cli/minimal: help出力・選択肢・未選択/Rule未読/稼働/inactiveでの参照、無変更・複合shell拒否。`go test -count=1 ./src/internal/minimal ./src/internal/cli -run '^TestMemoryHelp'`
5. C5 install/cmd: 配布Skillのhelp参照、Knowledge/ADR/Ruleの実CLI作成更新検索、旧fixture整合。`go test -count=1 ./src/internal/install -run '^(TestInstallMemory|TestMemoryHelp)'` と `go test -tags=integration -count=1 ./src/cmd/aidlc -run '^TestMemoryMetadataCommand'`。live本体はloopで起動しない。

末尾に上記全targetedと変更packageの既存関連prefix、gofmt、git diff --checkを確認する。
親が差分とtargetedを一度確認し、独立reviewを経てread-only finalへ進む。
finalは全package test、race/shuffle、vet、tidy-diff、format/diff、integration、6構成buildとnative help/exit確認。
配置Skillを使う固定Codexの限定liveでは未選択時help、本文からのKnowledge作成/更新、OKF metadataを確認する。
同じfixture内で製品CLIの実操作とファイルを照合し、AIの完了宣言だけを成功証拠にしない。
Unit並列・全4段階の長時間liveは対象動作を変えないため、関連した不具合がなければ再実行しない。

## 許可と完了

ユーザーの前案への明示承認と今回の追加依頼を直接の許可根拠とする。
Issueは機能開発で作成。独立review・final・対象PRのCI成功後、通常のmerge commitでマージしIssueを閉じる。
ユーザーのAGENTS.md差分、未追跡のOKF参考資料、他worktreeは保全する。
差分はGitで戻せる。利用先の既存Knowledgeを自動上書きするrollback処理は追加しない。
