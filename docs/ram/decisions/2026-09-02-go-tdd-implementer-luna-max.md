# go_tdd_implementerをLuna / maxで運用する

- 日付: 2026-09-02
- 状態: Accepted
- 関連Issue: #41

## 背景

`go_tdd_implementer`はモデルを明示せず、グローバル設定のGPT-5.6 Solを継承し、
reasoning effortを`high`としていた。TDD実装担当はコード、テスト、コマンド出力を
読みながら反復するため、custom agentの中でも利用量が大きくなりやすい。

OpenAIのCodexモデルガイドでは、GPT-5.6 Lunaは高速・高ボリュームのfocused coding taskに
適したモデルとされている。ユーザーは、実装担当のモデル単価を抑えながら最大のreasoning
effortを使う運用を選択した。

## 決定

- `go_tdd_implementer`のモデルを`gpt-5.6-luna`として明示する。
- `model_reasoning_effort`を`max`とする。
- 他のcustom agentとグローバルのモデル設定は変更しない。

## 影響

- `go_tdd_implementer`はグローバルモデルを継承せず、Luna / maxで起動する。
- Lunaの低いクレジットレートを利用できる一方、`max`は`high`より長い推論時間とtokenを
  使う可能性がある。実際の消費量と品質は、タスク、コンテキスト、ツール利用に依存する。
- AI-DLC本体のCLI、API、保存形式、実行時挙動には影響しない。

## 検証

- agent TOMLを構文解析する。
- Codexからリポジトリ設定を読み込めることを確認する。
- 差分がagent設定と本記録・索引に限定されていることを確認する。

## 根拠

- ユーザー承認: 2026-09-02
- OpenAI Codex Models: https://learn.chatgpt.com/docs/models
- OpenAI Codex Pricing: https://learn.chatgpt.com/docs/pricing
