# 初回版0.1.0と開発者向け参考元・依存関係の整理

2026-09-13、ユーザーは初回バージョンを`0.1.0`と指定した。AI-DLC、OKF Agent Memoryなどの参考リポジトリと依存関係を、会話を知らない初心者にも分かる開発者向けMarkdown一枚へ整理し、ユーザーが確認した後にリリースへ進むよう依頼した。

この指定により、[配布機構の承認記録](2026-09-13-github-release-pipeline-approved.md)にあった「正式版名未確定」のうち初回版名を確定する。製品ライセンス、公開対象commit、初回の添付内容、実tag・Draft・Release作成はここでは確定・実行しない。文書PRのmergeと製品の公開を区別し、Claude対応のIssue #185は未完了・保留を維持する。

## 文書整備の計画と許可

[Issue #188](https://github.com/sori883/ai-dd/issues/188)。mainの`0889ff5a8023e53f3c781fa180f4afe0d5897c47`を基準に、親エージェントを唯一のwriterとして、[開発者向け本文](../../developer-references-and-dependencies.md)、本記録、RAM索引だけを変更する。今回の直接依頼が文書整備の許可であり、旧33 Stageの包括承認を使わない。

順序は出典・コード・固定版の調査、本文執筆、独立したread-only review、親のread-only final、PR/checks/mergeとする。調査担当は参考元の確認だけを行い、編集しない。reviewは初心者への説明、固定snapshotと最新上流、参考元と実依存、ライセンス表示と公開境界を照合する。finalではリンク、依存一覧、固定版、3ファイルの変更範囲、`git diff --check`を確認する。製品コードの変更や人工的なTDDは行わず、既存CIは対象PRで起動したものを確認する。

本文は一枚へ集約し、参考設計・同梱skill・Goライブラリ・開発/CIツールを区別する。Goの3つのCLIについて、6対象の実行用import経路を読み取り確認した。aidlcはYAML、別の日本語CLIはKagome/UniDic/辞書共通、aidlc-distは外部moduleなし。Goの要求グラフだけに現れるIPADICとx/textを実binaryの依存へ数えない。

OKF Agent Memoryは初期実装の比較基準`v0.1.2/d4c523…`と後発skill参考`a09e049…`を用途別に記載する。本家AI-DLCの旧140原稿の取得記録は履歴であり、現在の配布一覧として扱わない。製品ライセンス、YAML/GoやOKF skillの許諾表示、補助CLIの公開方法の残対応は本文へ明示し、このタスクで勝手に採用・修正しない。
