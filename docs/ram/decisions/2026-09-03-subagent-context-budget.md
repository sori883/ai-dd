# サブエージェントhandoffのコンテキスト予算

- 日付: 2026-09-03
- 状態: Accepted
- GitHub Issue: [#59](https://github.com/sori883/ai-dd/issues/59)

## 背景

長期間続くCodexタスクでは、要件、意思決定、調査、実装ログを含む会話履歴が大きくなる。会話全文を
サブエージェントごとに継承すると、各エージェントの入力へ同じ情報が複製される。OpenAIの
サブエージェント資料でも、各サブエージェントが個別にモデルとtoolsを使うため、同等の処理を単一の
エージェントで行う場合よりtoken消費が増えると説明されている。

このリポジトリには、承認済み計画、Issue、RAM、仕様、対象コードという永続的な参照先がある。
そのため、作業に必要な参照先と制約を短く渡し、サブエージェント自身が必要部分だけを読む方が、
判断根拠を維持しながら重複する入力を減らせる。

## 決定

- サブエージェント起動時は `fork_turns="none"` を既定とする。
- 親エージェントは、目的、期待する出力、承認範囲、Issue・RAM・対象ファイル、確定した制約、所有権、
  必要な検証modeを自己完結したhandoffとして渡す。
- 長いファイル本文、コマンド出力、会話履歴はhandoffへ複製せず、永続資料への具体的な参照を渡す。
- 最近の会話または会話全文を継承できるのは、会話そのものが不可分な入力で、要約や永続資料では意味を
  保てない場合だけとする。親エージェントは、例外を使う理由を起動依頼へ明記する。
- `technical_researcher` と `project_planner` は常時起動しない。親エージェントが外部仕様の不確実性や
  変更の複雑さを評価し、必要な役割だけを起動する。
- `technical_researcher` は既存設定どおり `gpt-5.6-terra` / `medium` を維持し、起動時に別の値で
  上書きしない。他のcustom agentのモデルと推論強度は変更しない。

## 影響

- 長いメインチャットをサブエージェントごとに複製する入力tokenを削減できる。
- サブエージェントはhandoffだけを根拠に推測せず、指定されたIssue、RAM、仕様、対象ファイルから
  必要な事実を確認する責任を持つ。
- 役割数や同時実行数を一律に減らすのではなく、親エージェントが必要性を判断する。実装担当を1つにする
  所有権、独立review、最終検証など、既存の品質gateは維持する。
- AI-DLC本体のCLI、API、保存形式、実行時挙動には影響しない。

## 検証

- `AGENTS.md` と `docs/agent-workflow.md` で既定値、handoff項目、例外条件が一致することを確認する。
- `.codex/agents/technical-researcher.toml` をTOML parserで読み、`gpt-5.6-terra` / `medium` を確認する。
- `go_tdd_implementer`、`independent_reviewer`、同時実行数の設定に差分がないことを確認する。

## 根拠

- ユーザー承認: 2026-09-03
- [OpenAI: サブエージェント](https://learn.chatgpt.com/ja-JP/docs/agent-configuration/subagents)
- [カスタムエージェント運用](../../agent-workflow.md)
- [`technical_researcher`設定](../../../.codex/agents/technical-researcher.toml)
