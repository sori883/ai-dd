# 検証頻度をloop・review・finalへ分離する

- 日付: 2026-09-02
- 状態: Accepted
- GitHub Issue: [#39](https://github.com/sori883/ai-dd/issues/39)

## 背景

Issue #37の実装では、独立reviewでfindingが見つかるたびに実装担当へ別handoffとして戻し、各handoffの
終了時に全package test、race、vet、静的解析、6構成cross compileを繰り返した。targeted test自体は
`go-tdd` skillに記載されていたが、個々の修正完了とプロジェクトの最終gateの境界が曖昧だった。

ユーザーは、修正中はtargeted testだけとし、全検証は最後に1回だけ実施する方針を2026-09-02に承認した。

## 決定

検証を次の3 modeへ分離する。

| Mode | 用途 | 検証範囲 | 所有者 |
| --- | --- | --- | --- |
| `loop` | 実装、refactor、review finding修正 | 最小のtargeted test | 実装担当 |
| `review` | review・再review | findingの再現・解消に必要なtargeted test・診断 | reviewer |
| `final` | blocking findingがなく差分が安定した後 | 承認済み計画の全検証を1回 | 親エージェント |

- handoffでmodeを明示し、未指定時は`loop`とする。
- modeとagentの役割が一致しない場合は暗黙に昇格せず停止する。reviewerは明示された`review`だけ、
  implementerは`loop`または親が明示した`final`だけを受理する。
- 個別sliceやfinding修正の完了から`final`を推測しない。
- 全package test、race、vet、全体lint、cross compile、配布E2E等は`final`へ集約する。
- Go codeへの`gofmt`適用はreview前の`loop`で行い、`final`では`gofmt -l`等のread-only checkだけを行う。
- `final`後に対象ファイルが変わった場合、以前の証拠はstaleとする。targeted確認へ戻り、差分が
  再び安定してから`final`を再実行する。
- 最終検証の具体的なコマンドは承認済み計画に従い、変更種別に該当しない検証を追加しない。

## 影響

review修正の反復時間と重複計算を減らしながら、PR前の最終headにはfreshな全検証証拠を残す。
中間handoffの返却内容はtargeted evidenceとなり、全体の成功を意味しない。親エージェントはreview完了と
差分安定を確認してから`final`を開始する責任を持つ。
