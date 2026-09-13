# 実候補Nativeで見つかった祖先root探索の修復

2026-09-14。Issue #200、work_unit_id `five-cli-release-011-native-fix`、verification_mode `loop`。
開始HEAD `e12a5c82479e7a3eff00ad7f1a42e70b79304e20`、clean tree。親がfinal-01を終了して修復を委譲した。
[承認計画](../../design/five-cli-release-011-plan.md)の正常な導入を妨げるbugの修復であり、探索契約や互換範囲を拡張しない。

final-01のReleaseCandidateNativeでは、base/aidlcが展開した通常binary、base/projectが導入先だった。projectでokf rulesを起動すると、有効rootを見つけた後に祖先のaidlc/workflow/stage-graph.jsonを直接statしてENOTDIRとなった。fixture配置を回避する変更ではなく、共有projectroot.Resolveを修復する。

## 順序とTDD証拠

1. 所有は`src/internal/projectroot/root.go`と`root_test.go`。通常fileのaidlc／workflowを持つ祖先があっても有効projectへ解決する回帰testを追加。`go test -count=1 ./src/internal/projectroot -run '^TestResolve'`はexit 1で両ケースのENOTDIRを観測した。
2. 各path要素をstatし、非directoryを候補外にする最小修正。同じcommandがexit 0。不存在以外のstat errorは返し、symlink loopのfilesystem errorを隠さない。複数の本当のinstalled rootsは拒否し、明示rootは従来どおり直接canonical化する。これらの既存保護testは初回からGREEN。
3. 親のfinal-01/dependencies.json（同HEAD、Go 1.26.4、五製品×六target）に基づき開発者向け依存文書を訂正。YAMLはaidlcとokf、installerはGo vendor、梱包器は外部moduleなし。実依存と多めに同梱している許諾文を区別し、license/schema実装は変更しない。通常利用者の取得対象もaidlc-installへ訂正。

## 末尾検証

`go test -count=1 ./src/internal/projectroot ./src/cmd/aidlc ./src/cmd/okf -run '^Test(ResolveAncestorFiles|ResolvePreservesErrors|ProjectRootWithoutGit|FiveCLIContract)$'`をexit 0で確認する。変更Goへgofmtを適用し、git diff --checkを確認する。全package、race、vet、cross build、実候補E2Eと実Codexは親の新しいfinalへ残す。修復前のfinal成功部分を今回の差分の成功とは扱わない。commit/GitHub操作は親担当。
