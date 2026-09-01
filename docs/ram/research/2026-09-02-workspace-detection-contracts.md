# 読み取り専用ワークスペース分析の参照契約

- 日付: 2026-09-02
- 状態: Current for local AI-DLC 2.6.123 snapshot
- 関連: [実装計画](../decisions/2026-09-02-workspace-detection-plan.md)、
  [Intent作成coreの参照契約](2026-09-01-intent-create-contracts.md)

## 確認範囲

ローカルAI-DLC `2.6.123`のauthored implementation、canonical Codex dist、配置済みCodexで
`aidlc-utility.ts`のSHA-256が
`c0a27b957f121d45c47f104eed1f7648406fc4756679e2edb71679fc046d4458`と一致することを確認した。
主な根拠は次である。

- `docs/実装_aidlc-workflows/core/tools/aidlc-utility.ts:4818-5307`
- `docs/実装_aidlc-workflows/tests/unit/t20.test.ts:209-377`
- `docs/実装_aidlc-workflows/tests/unit/t203-nested-workspace-detection.test.ts:78-266`
- `docs/実装_aidlc-workflows/tests/unit/t211-detect-submodules.test.ts:111-261`
- `docs/実装_aidlc-workflows/tests/integration/t71-stage-workspace-detection-brownfield.test.ts:36-116`

SerenaとContext7はこのセッションにcallable toolとして公開されていなかったため、`rg`、`gopls`、
ローカル原典とGo標準ライブラリの文書で補完した。外部ライブラリやAPIを採用しないため、
Context7から補う依存仕様はない。

## 結果契約

本家の`detectWorkspace(projectDir)`は、書込みやGit commandを行わず次を返す。

- `projectType`: `Greenfield`または`Brownfield`
- `languages`: count順のcomma区切り、検出なしは`Unknown`
- `frameworks`: 固定検出順のcomma区切り、検出なしは`Unknown`
- `buildSystem`: 優先順位で選んだ1値、検出なしは`Unknown`
- `nestedRoot`: root fallbackでhitしたworkspace相対path。全OSで`/`区切り、複数は`, `区切り
- `submodules`: 宣言順の`name`、`path`、`url`、`initialized`

## Root signalと走査順

rootでsource file、framework、non-empty `package.json.dependencies`、対象manifest、または
`src`、`app`、`lib`、`pages`、`components`、`tests`のentryが存在するとBrownfieldになる。
対象manifestは`requirements.txt`、`pyproject.toml`、`setup.py`、`Cargo.toml`、`go.mod`、
`pom.xml`、`build.gradle`、`build.gradle.kts`、`composer.json`、`Gemfile`である。

rootがBrownfieldでない場合だけ、任意名のcontainerを最大3階層探索する。各階層はJavaScriptの
UTF-16 code-unit順でsortし、dot directory、harness・VCS・build directory、docs、examples、
samples、fixtures、templates、scripts、既知source directory、symlink、非directoryを除外する。
hitしたdirectoryではそれ以上descendせず、language、framework、build systemをmergeする。
古いreference文書にあるone-levelより、authored sourceとt203 testが固定する3階層を優先する。

rootの`.gitmodules`はnested fallback後に評価する。有効なsubmodule宣言が1件あれば、directoryが
未初期化でもBrownfieldになる。したがってsubmoduleだけがsignalのworkspaceでも、先にnested scanが
行われる。

## Language、framework、build system

言語対応はTypeScript、JavaScript、Python、Java、Kotlin、Go、Rust、Ruby、C#、C++、C、Swift、
PHPである。root直下fileと既知source directory内を最大depth 6で数え、symlink entryを追従しない。
拡張子はASCII case-insensitiveだが、最後の`.`がindex 0の`.py`のような隠しfileは数えない。

primaryは最大count、secondaryは`max(1, floor(primaryCount * 0.2))`以上を全て含む。同数時の
明示tie-breakはなく、stable sortにより最初の観測順が残る。file走査のdirectory entryはsortされないため、
BunとGo、OS、filesystemをまたいだ同数時の完全一致は保証されない。

frameworkの固定順はNext.js、Vite、Angular、Nuxt、Remix、Gatsby、Astro、Svelte、NestJS、React、
Django、Rails、Spring Bootである。Reactは`dependencies`と`peerDependencies`を後者優先でspreadした
`react`値のJavaScript truthiness、RailsとSpring Bootは各manifest内容で検出する。read・parse失敗は
framework signalとして吸収する。

build systemはpackage.jsonのpnpm、yarn、bun、npm、次にpyprojectのpoetry、uv、hatch、generic
python、続いてpip、setuptools、cargo、Go modules、Maven、Gradle、Composer、Bundlerの順である。
rootの値が`Unknown`の場合だけ、sorted nested traversalで最初に検出した値を採用する。
devDependenciesだけのpackage.jsonはGreenfieldだが、build systemはnpmになり得る。

## `.gitmodules`

parserはline-orientedで、commentと未知keyを許容し、`url`を任意とする。pathなし、`/`始まり、
Windows drive-absolute、`..` segmentを持つentryを破棄する。安全と判定したentryはduplicateを含めて
宣言順に保ち、trim後のpath文字列をnormalizeせず返す。missing・unreadable・全体がmalformedなら空、
一部をparseできればそのentryだけを返す。

`initialized`は`<path>/.git`というentryが存在するかだけを見ており、Git repositoryとしての妥当性、
gitlink、Git commandは確認しない。UNC、backslash root-relative、`.`、duplicate、symlink経由の外部参照は
本家parser単体では拒否しない。

## Go実装への示唆と未知事項

Go標準ライブラリだけで実装できる。言語の初回観測順を本家へ近づけるため、system走査は
`os.Root.Open`から`File.ReadDir(-1)`を使う。`os.ReadDir`、`fs.ReadDir`はfilename sortを加えるため
使わない。nested traversalだけUTF-16順を明示する。

scanner内部の個別I/O・JSON parse失敗は本家同様にそのsignalを無視する。package.jsonはtop-levelの
plain objectを確認し、weakly typedな`dependencies`・`peerDependencies`もJavaScriptのObject.keys、
object spread、truthiness相当で扱えば、malformed fieldに新しい差分を作らずに済む。

Go版は既存の`os.Root`安全境界を継承するため、root外またはabsolute symlink経由のconfig、source、
submodule `.git`を本家の通常filesystem APIのようには追従しない。これは新しいscanner固有の判断ではなく、
既存workspace APIで承認済みの安全境界である。mount、device、並行変更中のsnapshot一貫性は保証しない。
BunとGoのnative directory enumeration順が同じという保証もないため、同数languageのcross-runtime順は
残余互換性リスクとして扱う。
