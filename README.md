# AI-DLC

AIと目的を整理し、必要な工程を選び、検査・レビュー・承認を経て開発を進めるGo製CLIです。
一つの目的をIntentとして管理し、進捗はstate、現行仕様はKnowledge、設計判断の理由はADRへ保存します。

## 使い始める

利用する実行ファイルは`aidlc`一つです。Codex用のSkill・専門担当・hookと工程定義を内包しています。
Codexを使う通常のプロジェクトフォルダへ配置し、AIへ実現したい目的を伝えます。Gitは任意です。
Go 1.26以降が必要なのは、ソースからbuildする場合です。

1. [配布・導入・更新手順](docs/distribution.md)で実行ファイルと利用先を用意する。
2. [利用者ガイド](src/docs/user-guide.md)に沿ってCodexから初回の作業を依頼する。
3. AIが提示する実行計画と、各工程の成果を確認して承認する。

取得先は[GitHub Releases](https://github.com/sori883/ai-dd/releases)です。公開版が用意されたら、OS・CPUに合うarchiveを選びます。
現在は候補検証と手動指定時のRelease下書き作成を整備した段階で、正式版名・ライセンス・初回公開物は未確定です。
手元でbuildする場合は[開発手順](docs/development.md)を参照してください。
実際のCodex hook動作を確認した環境とOS別の配布検証は、[検証範囲](docs/distribution.md#検証の範囲)に記載しています。

## 開発の流れ

最初の「Space等の初期化 → 目的整理と深掘り」は必須です。
「現状の構成分析・実装計画・TDD・統合検証」は、目的整理で採否と順序を決め、ユーザーが承認します。
各実行回ではプログラムによるSensor、別担当のレビュー、会話による成果承認を確認して進みます。

メインAIが必要な専門担当を標準のサブエージェント機能で起動します。
同じフォルダで一人ずつ実装し、独立した別フォルダなら並列化できます。共有の進捗とKnowledgeはメインAIが保存します。
検証対象のソース・テスト・設定を `verification_paths` にまとめ、その内容のSHAとテスト結果で成果を確認します。
独立レビューは別会話で行い、同じフォルダを使えます。
CLIは担当の割当を管理し、エージェント自体は起動しません。

## 操作を調べる

```sh
aidlc --help
aidlc --version
aidlc intent procedure --help
aidlc memory search --help
```

型・引数・JSONの詳細は利用中のCLIのhelp、現在の工程の担当・入力・出力は`intent procedure`で確認できます。

Intent・Bolt単位でGitHub Issue／PRを使う場合は、[任意導入の追加skill](src/docs/optional-skills.md)を利用できます。本体の標準配置とは別に、必要なプロジェクトへコピーして使います。

## 開発者向け資料

- [開発・検証手順](docs/development.md)
- [現行アーキテクチャ](docs/architecture.md)
- [開発プロジェクトの判断記録](docs/ram/README.md)
- [固定版の本家AI-DLC分析](docs/aidlc-analysis/README.md)
