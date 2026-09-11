# Gitなし一周fixtureの公開出力処理を修復

日付: 2026-09-11。Issue #171、開始HEAD `6552e4cd229fa3394c67efde2ec31a62682402f2`。[承認済み計画](../../design/git-independent-workflow-plan.md)内の受入fixture修復であり、製品の公開APIは変更しない。

親のfinal-2ではFlowJourney、AssignmentJourney、Gitなしdirect経路が成功し、新GitIndependentJourneyのUnit経路が失敗した。原因は `assignment list` が返す既存のReservation配列を、fixtureがRegistry objectへdecodeしていたことだった。配列からUnitに対応する予約を選択する小helperを使用し、欠落は明示errorとする。実行したテストに合わせ、記録commandへ `-count=1` を含めた。全体結果を登録するときもUnitの結果一覧を保持して追記し、現在stepのUnit証拠をTargetに含む通常経路を通す。

`TestGitIndependentFixtureReservation` で実出力と同じ配列のdecode失敗をREDとして確認し、配列処理へ修正後にGREENを確認した。最初の小fixtureでIDキーを `id` と記した誤りも公開形式 `assignment_id` へ訂正した。成功・欠落・空配列・不正JSONを確認する。tagged compileは `go test -tags=integration -count=1 ./src/cmd/aidlc -run '^TestGitIndependentFixture'` で行い、一周そのものはloopで実行しない。fresh finalの一周再実行は親の再review後のgateである。
