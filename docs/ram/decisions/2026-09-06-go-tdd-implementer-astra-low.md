# go_tdd_implementerをGPT-6 Astra / lowで運用する

- 日付: 2026-09-06
- 状態: Accepted
- 関連Issue: #121
- 置換対象: [go_tdd_implementerをLuna / maxで運用する](2026-09-02-go-tdd-implementer-luna-max.md)

## 背景

`go_tdd_implementer`は、Go実装とテストを単独で担当するcustom agentである。従来は、
実装token単価を抑えながら最大のreasoning effortを使うため、`gpt-5.6-luna` / `max`に
固定していた。

利用者は、今後の実装品質を重視して、実装担当を`gpt-6-astra`へ変更し、reasoning effortは
`low`に抑える運用を選択した。OpenAI公式のGPT-6 Astraモデル資料で、`low`がサポート対象で
あることを確認した。

## 決定

- `go_tdd_implementer`のモデルを`gpt-6-astra`として明示する。
- `model_reasoning_effort`を`low`とする。
- 実行中のagentは中断または再設定せず、変更は新しく開始したCodexタスク、またはプロジェクト
  設定を再読込したタスクから適用する。
- `technical_researcher`は`gpt-5.6-terra` / `medium`を維持する。
- 他のcustom agentとグローバルのモデル設定は変更しない。

## 影響

- 新しく起動する`go_tdd_implementer`は、Astraの実装能力を`low`のreasoning effortで利用する。
- 既に起動済みの`go_tdd_implementer`は、起動時に確定したLuna / maxのまま作業を継続する。
- AI-DLC本体のCLI、API、保存形式、実行時挙動には影響しない。
- モデル単価と実際の利用量はLuna / maxと異なる。総消費量、待ち時間、品質は、タスク、
  コンテキスト、ツール利用に依存する。

## 検証

- agent TOMLを構文解析し、`gpt-6-astra` / `low`を確認する。
- `technical_researcher`が`gpt-5.6-terra` / `medium`のままであることを確認する。
- 差分がagent設定と本記録・索引に限定されていることを確認する。
- default branchへのmerge後に設定値とIssue closeを確認する。

## 根拠

- ユーザー直接承認: 2026-09-06
- [OpenAI: GPT-6 Astraモデル](https://developers.openai.com/api/docs/models/gpt-6-astra)
- [OpenAI: GPT-6 Astraモデルガイド](https://developers.openai.com/api/docs/guides/latest-model)
