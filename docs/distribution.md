# 配布候補の生成と、既設配置の比較・更新・復旧

AI-DLCの利用者が実行するファイルは`aidlc`一つです。工程定義、Codex用Skill・agent・hookはbinaryに内包されています。`aidlc-dist`は開発者がbuild済みbinaryを圧縮するためのcommandで、利用先へ追加導入する必要はありません。

このリポジトリではlocal/CI内で配布候補を検査します。正式version、公開範囲、Go製品のライセンスは未確定です。ここに示す生成処理はtag、GitHub Release、artifact uploadを行いません。候補が生成できたことを一般公開済みとは扱わないでください。

## 開発者が候補を生成する

同じsource commitとGo toolchainから6targetをbuildします。未commit差分があれば、source commitだけではbuildした内容を示せません。公開候補に使う前にcheckoutが意図した版であることを確認してください。

次はrepository rootで実行するBashの例です。`DIST_WORK`には新しく作った一時directoryを指定します。output directory自体は梱包commandが作成するため、先に作らないでください。

```sh
DIST_WORK="$(mktemp -d)"
dist_input="$DIST_WORK/input"
dist_output="$DIST_WORK/candidate"
mkdir "$dist_input"
source_commit="$(git rev-parse HEAD)"
candidate_version="dev-${source_commit:0:12}"
toolchain="$(go env GOVERSION)"
for os in darwin linux windows; do
  for arch in amd64 arm64; do
    extension=""
    if [[ "$os" == windows ]]; then extension=".exe"; fi
    CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" go build -trimpath \
      -ldflags "-X github.com/sori883/ai-dd/src/internal/buildinfo.Version=$candidate_version -X github.com/sori883/ai-dd/src/internal/buildinfo.Commit=$source_commit" \
      -o "$dist_input/aidlc-$os-$arch$extension" ./src/cmd/aidlc
  done
done
go run ./src/cmd/aidlc-dist \
  --input-dir "$dist_input" --output-dir "$dist_output" \
  --version "$candidate_version" --commit "$source_commit" --go-version "$toolchain"
```

単一targetだけを扱う場合は`--targets linux/amd64`のように指定します。対象はdarwin/linux/windowsとamd64/arm64の組です。カンマ区切りの非空subsetを受け付け、重複や未知targetは拒否します。

入力名は`aidlc-OS-ARCH`、Windowsのみ末尾`.exe`です。空file、directory、symlink、欠落fileは使えません。versionは先頭英数字、英数字と`. _ -`のみ、128文字以内、`..`なしです。commitは小文字16進40桁、Go版は`go1.26.4`や`go1.27rc1`のような空白・pathを含まないgo1系の値を指定します。これらと全binaryの読取りを確認するまで出力directoryは作りません。

出力先は未存在である必要があります。既存の候補は上書きしません。保存途中の失敗では終了codeが非zeroになり、部分的なdirectoryが残ることがあります。その候補を完成扱いせず、原因を確認して別の新しい出力先へ再実行してください。終了codeは0が成功/help、2が不正引数、1がfilesystemや出力の失敗です。

## 圧縮fileと照合情報

| 対象 | archive名 | 中身 |
| --- | --- | --- |
| darwin / linux | `aidlc_VERSION_OS_ARCH.tar.gz` | 通常file `aidlc`、mode 0755 |
| windows | `aidlc_VERSION_windows_ARCH.zip` | 通常file `aidlc.exe` |

archiveには上記binary一つだけが入ります。file名、mode、tarの所有者情報・時刻、gzip header、ZIP時刻を固定します。ZIP時刻は1980-01-01 UTCです。build日時や入力fileのmtimeは持ち込みません。同じbinary bytesとmetadataを同じ梱包実装で処理すれば、target指定順にかかわらず同じ出力bytesになります。異なるGo/compiler/圧縮実装まで同一bytesになる保証ではありません。

`manifest.json`のschema_versionは1です。version、source_commit、go_version、およびtarget順のartifactsを持ちます。各artifactのbinaryはarchive内の名前、binary_sha256/binary_sizeは展開後の内容、archive/archive_sha256/archive_sizeは圧縮file名・内容・byte数です。
`SHA256SUMS`はarchiveとmanifestのSHA-256をfile名順に記録します。SHA256SUMS自体のhashは含めません。

これは入力と実内容の対応表です。署名でも出所の第三者認証でもありません。任意の入力binaryが指定sourceから作られたことを梱包commandだけで証明するものではありません。

全6target候補の照合は、開発checkoutから次で行えます。archive内の名前・mode・payload hash/size、圧縮fileとmanifestのhash/size、checksum一覧を確認します。foreign targetのbinaryは実行しません。

```sh
AIDLC_DIST_DIR="$dist_output" go test -tags=integration -count=1 -v \
  ./src/cmd/aidlc-dist -run '^TestDistributionArchives$'
```

`AIDLC_DIST_DIR`未指定ならこの検査はskipします。それを候補の照合成功に数えないでください。subsetのnative梱包・展開・実行は別の`TestDistributionJourney`が確認します。

利用するOS/CPUと一致するarchiveを選び、照合後に新しい空directoryへ展開します。Unixなら`tar -xzf ARCHIVE -C NEW_DIR`、Windows PowerShellなら`Expand-Archive -LiteralPath ARCHIVE -DestinationPath NEW_DIR`が使えます。実行前に対応targetと取得元を確認し、展開したbinaryの`version`と`--help`を確認してください。hash照合だけで取得元を信頼できるわけではありません。

## 既設配置を更新する前に

以下は利用者が確認して行う手順です。CLIが停止確認、backup、資材選択、切替、rollbackの全体を強制する自動updaterではありません。

1. 対象projectのメインAI、worker、既知の背景処理を停止・整理します。予約と保存途中の操作を確認してください。Stop通知や経過時間だけで予約を解放せず、状態が不明なら更新作業を止めます。
2. 旧binaryを実path・version・hashとともに保管します。配置した製品file、独自hooks/config、全Space、Git管理外の`aidlc/.runtime`も別の場所へbackupし、実際に読めることと元bytesが一致することを確認します。Git commitだけではignored runtimeのbackupになりません。
3. 新binaryは旧binaryと違うpathへ保存します。別の空Git projectを`git init`し、そこへ`NEW_BINARY install codex --project-dir STAGING_ROOT`を実行します。実利用先でfresh installを再実行して更新しようとしないでください。既存fileの上書きは拒否されます。
4. 旧配置・旧版の既知資材・staging候補を比較します。由来不明の編集を製品の古いfileだと決めつけず、そのfileの扱いを確認するまで止めます。

比較対象は次のように分けます。directory全体を無条件コピーしないでください。

| 対象 | 扱い |
| --- | --- |
| `.agents/skills/aidlc/SKILL.md`、`.agents/skills/aidlc-cli/SKILL.md` | 既知の製品Skillとして旧新を比較する |
| `.codex/agents/aidlc-*.toml`の製品5担当 | 既知の5fileを比較する。利用者が作った別agentは保持する |
| `aidlc/workflow/stage-graph.json`、`aidlc/workflow/stages/*.md`、`aidlc/templates/adr.md` | 対応する定義・templateを組で比較する |
| `.codex/hooks.json` | 製品handlerを比較し、独自handlerを残して手動mergeする |
| `aidlc/spaces/**` | Rule、Knowledge、ADR、Intent/state/historyを保持。stagingのseedで置き換えない |
| `aidlc/.runtime/**` | 会話・予約・保存途中の情報を保持。削除して空きや未実行に見せない |
| `.codex/config.toml`、利用者の`AGENTS.md`、独自agent/config | 利用者の設定を保持する |

## 切替と参照の補正

比較して扱いを決めた製品資材を、新binaryに対応する組で切り替えます。製品handlerはSessionStart/UserPromptSubmit/PreToolUse/PostToolUse/Stopの5イベントにあります。独自hookを消さず、製品のcommand・matcher・timeout等を候補と照合してください。

stagingで生成したSkillにはbinaryの絶対path、hookにはbinaryとstaging rootの絶対pathが入ります。実利用先へそのままコピーしてAIを再開してはいけません。新しい版の資材を選んだ後、既知形式の参照を補正する場合に限り、次を使えます。

```text
NEW_BINARY install codex --relocate --project-dir REAL_ROOT \
  --from-project-dir STAGING_ROOT --from-binary NEW_BINARY
```

すべて実際の絶対pathへ置き換えます。この例は新binaryのままroot参照をstagingから実利用先へ補正します。別pathからの配置移転では、元配置に埋め込まれた旧root/binaryをfromへ渡します。元pathが今も存在する必要はありません。

`--relocate`の対象はaidlc/aidlc-cli両Skillとhooks.jsonの3fileだけです。既知の現行/限定旧版のSkill bytes、既知の製品handler形状、元/新の参照が成立する場合にだけ補正し、独自hookのbytesを保持します。未知のSkill編集、製品command、matcher等があれば拒否します。エラーを回避するために利用者編集を無断で消さず、比較へ戻ってください。移転は版更新や定義移行ではありません。

部分失敗のPathsは更新済み、Pendingは未完了です。処理終了と原因を確認し、同じ引数で再検査できます。成功後も3fileの実pathと独自hookを確認します。通常のCodex hook trust確認、許可/拒否の対照を経てからAIを再開してください。trustや認証設定は自動変更しません。

定義hashが変わった場合、既存Intentは元の定義に結び付いています。旧版で進めている仕事を整理し、新定義では新Intentを使ってください。元Intentの定義hashだけを書き換えて移行したことにはしません。

## 失敗した場合の復旧

作業を止め、保管した旧binaryとそれに対応する製品Skill・agent・hooks・定義・templateを組で戻します。独自hookを含む元の製品file bytesと参照をbackupに照らして確認します。利用者の進捗、Knowledge、履歴、予約は新版で変更があった可能性があるため、古いseedや空のruntimeへ戻してはいけません。

新版で作ったIntentを旧版で再開できるとは保証しません。元の版と定義が必要な仕事はそれに対応する配置で扱い、判断できない場合は実行を再開せず確認します。

## 検証の範囲

Distribution CIはUbuntuで6targetをbuild・梱包・照合し、`ubuntu-latest`、`macos-latest`、`windows-latest`でnativeの梱包・展開・version/help・fresh install・既存file拒否・参照補正を検査します。実行したruntime.GOOS/GOARCHをlogへ出します。

```sh
go test -tags=integration -count=1 -v ./src/cmd/aidlc-dist -run '^TestDistributionJourney$'
```

このfixtureは同一sourceからversion付きの旧/新binaryを別pathへbuildし、隔離projectで独自hookと利用者dataの不変、手動で選んだ製品fileの切替、参照補正、元bytesの復元を確認します。未知版へのupgrade互換、自動updater、実利用環境での切替成功を実証するものではありません。

archive展開・配置file生成は実Codex hookの実行成功と別です。macOS/Codex CLI 0.153.4の通常trust実測はPR #164の記録を参照し、Windowsの実hook動作や全ハーネスの互換性は未確認として残します。今回その実機を再実行しません。
