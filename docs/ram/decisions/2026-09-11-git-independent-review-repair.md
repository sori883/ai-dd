# Git不要化の独立レビュー指摘を修復する

日付: 2026-09-11。Issue #171、作業単位 `git-independent-review-repair`。開始HEADは `f968f2bca40af4ffc5109fed317c36455cd070cd`。

[直接承認済みの計画](../../design/git-independent-workflow-plan.md)が求める結果・出力のhashと、現行利用手順の整合を修復する。新たな仕様選択や互換性は追加しない。[実装証拠](2026-09-11-git-independent-implementation-evidence.md)の後続確認である。

独立レビューで、現在stepのUnit結果が全体成功の対象scopeではないためTargetと受入証拠からも抜けていたことが判明した。現在stepの結果JSONと各runの出力を、scopeにかかわらずTarget・受入証拠へ含める。成功要件だけをscopeとUnit/runの対応で判定する。これによりUnit結果の内容変更は以前のレビュー・成果承認を失効させる。後続Unitの変更に伴う過去Unit SHAと現在全体SHAの違いは引き続き許可する。

過去stepの結果をTestResultsへ列挙しただけでは現在stepの証拠へ含めない。accepted入力として参照される過去成果のhash確認は既存の文書検査で維持する。

READMEと開発手順に残ったGit必須・別root reviewer・commit入力を、verification_paths、verification_sha256、同root可・別session、同一親子root排他へ修正した。既存test hostによるGit転送は製品の必須条件と区別する。旧記録は上書きせず、本記録を索引へ追加する。

`go test -count=1 ./src/internal/flow -run '^TestVerificationGatesUnitEvidence$'` で現在Unit結果JSON・非空出力の変更がTargetを変えずfinishを許すREDを観測し、修正後のGREENを確認した。過去stepの非入力結果の分離と、受入証拠に現在Unit結果を含むことも確認した。文書変更に人工REDは作らない。全体finalと実Codexの証拠は親の後続gateであり、このloopの成功とは区別する。

親の末尾確認で、結果parserのstage制限が旧実装から脱落していたことも修復した。結果JSONのstageは `tdd` または `integration` に限定する。initialization・discovery・planningの実在stepに結び付く結果でも拒否する回帰を先に追加し、RED→GREENを確認した。過去stepの境界fixtureは現在integration／過去TDDへ改め、planning結果を正常系にしない。現在Unit証拠変更による旧承認の拒否と、過去TDD結果を現在の受入証拠へ混ぜない期待は維持した。
