# Space一覧CLIの参照契約

- 調査日: 2026-08-31
- 対象: ローカルAI-DLC 2.6.123
- 状態: Current for local snapshot
- 用途: [承認済みspace list計画](../decisions/2026-08-31-space-list-plan.md)

## 確認範囲

analysis indexからCLI契約へ絞り、core/toolsのaidlc.ts、aidlc-utility.ts、aidlc-lib.tsを静的確認した。
計画時のtechnical_researcherは、上記3ファイルについてcoreと配置版
`docs/配布_ai-dlc/.codex/tools/`のSHA-256一致を確認した。snapshotのCLI・hookは実行していない。
配布物全体のparityや最新upstreamの調査結果ではない。

## 一覧の契約

`core/tools/aidlc-utility.ts:6099`のprintSpaceListingはresolveWorkflowSelection(projectDir)のspaceを
listSpacesへoverrideとして渡す。human形式は `Spaces:\n`と選択行の `* `／未選択行の2空白。
JSONは `{active,spaces:[{name,active}]}`。active行がなければトップactiveだけdefaultにする。
一覧readerのdefault補完、UTF-16順、エラー吸収は既存の
[space読み取り調査](2026-08-31-space-reading-contracts.md)を参照する。

## 選択と副作用

- `aidlc-lib.ts:3613`のresolveWorkflowSelectionは、有効なsession bindingのspaceをshared activeSpaceより優先する。
  session IDはAIDLC_SESSION_OVERRIDEと親PID ancestryを照合し、不一致ではthrowする。
  AIDLC_SESSION_OVERRIDE_SOURCE=payloadには例外がある。
- `aidlc-lib.ts:3242`のreadSessionBindingは
  `aidlc/.aidlc-sessions/<id>.binding.json`を読み、JSON・名前・boundAt・必要なstate markerを検証する。
  不正・読取不能・staleなbindingは無効扱いで、削除・修復しない。
- sessionまたは有効bindingがなければ、`aidlc-lib.ts:3642`のselection.spaceはActiveSpaceと同値。
  shared cursorの追加検証・一覧所属確認・未知名からdefaultへの置換は行わない。
- resolverは一覧には不要なintent解決も行う。ancestry参照はLinuxの/procやmacOSのps等を使う。
  正常な一覧・選択経路はfilesystemへ書き込まない。
- utility直接/dev経路のcatch（aidlc-utility.ts:8342）はdie→emitError（aidlc-lib.ts:22207）を通り、
  stateを解決できれば監査追記を試みる。compiled dispatcherのcatch（aidlc.ts:1014）はJSON errorと1を返す。
  正常経路のread-only性と、すべての起動方式の例外経路は区別する。

Goの段階的実装では既存ActiveSpace/ListSpacesだけを接続し、session・intent・process ancestryに依存させない。
session binding使用中やsession衝突時には本家と結果が異なることを、未実装範囲として文書化する。

## public CLIとutility直接呼出の違い

public `aidlc.ts:653`はworkspace parser（aidlc-lib.ts:728以降）の結果から
`[space]`または`[space,--json]`へ引数を再構築する。

| publicへの入力 | 結果 |
| --- | --- |
| space / space list | 同じhuman一覧 |
| space --json / space list --json | JSON一覧 |
| space list extra --json | extra以降が捨てられhuman一覧 |
| space list --json=true / --json=false | human一覧 |
| space list --json false | falseを捨ててJSON一覧 |

global parser（aidlc.ts:874）は `--project-dir PATH`を前・途中・後から抜き出し、重複は最後優先。
欠落・空・`--`開始の値はerror。`--project-dir=PATH`はglobal形式として認識されず、
一覧の後なら無視され、先頭ならunknown commandになるなど位置依存である。
一方、utility直接呼出のparseArgs（aidlc-lib.ts:22276）は=形式や分離値を受ける。
こちらをpublic CLIの契約と混同しない。

Goでは承認済み計画に従い、入力を厳格に検証し、project-dirの=形式は既存createと揃えて受理する。

## Goの接続根拠・未知事項

Context7を優先し、Go標準ライブラリのOpenRoot/Root.FS/Closeを確認した。
返された資料はGo 1.25.3版だったため、ローカルGo 1.26.4のgo docでも照合した。
Rootは外向き・絶対symlinkを拒否し、Close後はRootの操作がerrorになる。
初回OpenRootの指定path自身はsymlinkを追従する。mount・device等まで隔離する完全sandboxではない。

参照: [os.Root](https://pkg.go.dev/os@go1.26.4#Root)、
[OpenRoot](https://pkg.go.dev/os@go1.26.4#OpenRoot)、
[Root.FS](https://pkg.go.dev/os@go1.26.4#Root.FS)、
[Close](https://pkg.go.dev/os@go1.26.4#Root.Close)。

SerenaとContext7は利用可能。gopls専用MCPは未公開のため既存CLI v0.23.0をfallbackに使う。
govulncheckは未導入で、新規tool追加の承認はない。脆弱性scanは実施済みと扱わない。
