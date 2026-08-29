# 既存AI-DLCの配布形式

- 調査日: 2026-08-29
- 対象: ローカルAI-DLC `2.6.123`スナップショット
- 状態: Current for local snapshot

## 結論

既存AI-DLCには、ハーネス別の生成済み配布treeと、release用にcompileしたCLIの二つがある。
compile済みCLIも全実行資産を単一ファイルへ内包する方式ではなく、隣接する`runtime/` treeを
必要とする。そのため、現在のGo版が目標とする厳密な単一バイナリ配布とは異なる。

## 1. 配布treeの生成

手書きの`core/`、`harness/<name>/`、`plugins/`を、Bunで動く`scripts/package.ts`が
`dist/<harness>/`へ投影する。

生成時には次を行う。

1. 共通coreをコピーし、ハーネス固有pathへ変換する。
2. 各`manifest.ts`のfile/directory mappingを適用する。
3. stage graph、scope grid、model rate等のJSONをcompileする。
4. 有効stageからstage runnerを生成する。
5. `emit.ts`でconfig、hook、agent定義等を追加する。

`bun scripts/package.ts --check`は、一時directoryへ再生成した結果とGit管理された`dist/`を
byte単位で比較する。`dist/`は生成物であり、直接編集しない。

## 2. Codexプロジェクトへの配置

Codex向けには、原則として次を対象プロジェクトへコピーまたは既存内容とmergeする。

- `.codex/`: tools、hooks、agent設定、config、rules
- `.agents/`: Codexが発見するskills
- `aidlc/`: workspace shellとdefault method memory
- `AGENTS.md`: onboardingと運用規約
- `.gitignore`: AI-DLC管理領域だけを既存規則へmerge

この配布treeに`package.json`や`node_modules`は含まれない。一方、同梱されたTypeScriptの
toolとhookは利用環境のBunで直接実行するため、通常のtree配布ではBunがruntime依存になる。

## 3. Release用CLI

release artifactは`scripts/package.ts --check`の成功後、`scripts/build-binaries.ts`が
生成済み`dist/claude/.claude/tools/aidlc.ts`を入口として`bun build --compile`する。

出力は`build/binaries/<target>/`配下に置かれ、次を含む。

- compile済み`aidlc`実行ファイル
- `runtime/<harness>/`に置かれた7ハーネス分の生成済み配布tree
- buildとsmoke gateの結果を記録する`build-results.json`

release matrixにはmacOS x64/arm64、Linux x64/arm64のglibc・musl等、Windows x64が含まれる。
ローカルsnapshotのmatrixにはWindows arm64は含まれない。

compile済みCLIはBunの実行runtimeを内包するが、stage、sensor、runner、hook、agent、skill等の
実行資産は隣接`runtime/`から解決する。したがって、配布単位は「実行ファイル1個」ではなく、
「実行ファイルとruntime directoryのbundle」である。

## 4. 正規生成物と配置物の差

正規の`dist/codex/`と、このリポジトリに置かれた`docs/配布_ai-dlc/`は各333ファイルで、
6ファイルに意図的な運用差がある。5つはagent model名、1つはCodexのprovider/model/AWS設定である。

Go版で既存配布との互換性を検証する場合は、正規生成物とローカル配置overlayを区別する。

## Go版への示唆

- workspaceの永続化形式と、engine自体の配布形式は別の設計問題として扱う。
- 厳密な単一バイナリを維持するなら、固定資産を`go:embed`等で内包し、必要なプロジェクト資産だけを明示的に展開する方式が候補になる。
- 既存方式へ忠実に合わせるなら、binaryとruntime treeをbundleする方式になる。
- どちらの場合も、利用者が編集するworkspaceと、framework所有の生成資産を混在させない。

## 根拠

- `docs/実装_aidlc-workflows/scripts/package.ts`
- `docs/実装_aidlc-workflows/scripts/build-binaries.ts`
- `docs/実装_aidlc-workflows/harness/codex/manifest.ts`
- `docs/実装_aidlc-workflows/harness/codex/emit.ts`
- `docs/実装_aidlc-workflows/README.md`
- `docs/aidlc-analysis/02-build-config-dependencies.md`
