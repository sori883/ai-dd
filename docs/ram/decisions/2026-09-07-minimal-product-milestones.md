# 現在のGo実装からOKF・Intent KDRへ移るマイルストーン案

- 日付: 2026-09-07
- 状態: Proposed。今回の依頼は現状整理とマイルストーン検討であり、実装許可ではない。
- 実装基準: GitHub `sori883/ai-dd` のdefault branch `main`、`fc8bd7ea633f0c1ad8cc605233572a6106b3b911`。
- 製品方針: [最小構成案](../../design/okf-kdr-minimal-workflow.md)、[Intent KDRとhookの承認済み境界](2026-09-07-intent-kdr-hook-boundary.md)。

## 後続の具体化

2026-09-07の引継ぎでmain・HEAD・PR #127・Issue #126・Open件数を再確認し、同じ現在地だった。
M0の6項目とM1の対象file・受入条件・検証を[最小契約案](../../design/minimal-product-contract-m0.md)へ具体化した。
下記の当初マイルストーン案は履歴として保持する。現在は「具体案作成済み・採用確認待ち」であり、M0完了やM1実装許可ではない。
判断と許可の境界は[後続RAM](2026-09-07-minimal-product-contract-proposal.md)を参照する。

## 目的と利用者が得る結果

現在のGo版は、固定AI-DLC 2.6.123の33 Stage、承認、state、auditを段階的に移植している。
ユーザーは製品を簡潔にし、工程の位置管理を省いて、早い試作と実装・テストの反復を行いたい。
新しい中心は、rules・設計・知識を保存するOKF Agent Memoryと、一つのIntentの目的・判断・結果を書くKDRである。
Intentは一つの目的を持った作業単位。Intentごとに一つのKDRファイルをCLIで作成・更新し、hookが通常のAI操作の記録漏れを防ぐ。

最初に大量の旧コードを削除するのではなく、一つのIntentを新方式で実際に完走できる縦の接続を作る。
その後、再開・共有・並行作業の運用を確認し、旧方式から切り替えて製品構成を縮小する。
マイルストーンはこの製品を開発する順序であり、利用者の作業に新しいStageを課すものではない。

## 2026-09-07時点の現状

GitHubのmainとローカルHEADは一致。直近PRのstate、merge commit、対象filesとchecksを取得し、現在のコード・test配置と照合した。
Open IssueとOpen PRは各0件。新しいOKF・KDR設計とRAMはローカル未commitであり、mainへ導入済みではない。

| 領域 | 確認できた状態 | 新方式への扱い案 |
| --- | --- | --- |
| CLI・workspace | Space作成・一覧・切替、Intent一覧・切替の公開CLI。Intent作成は内部APIがあるが公開helpに作成コマンドはない | root解決・識別・一覧等を再利用候補にする。新Intent開始をStage初期化から分ける |
| 保存 | state writer、record lock、path検査等がある | 安全なファイル保存の仕組みを選別する。既存state型やaudit依存を丸ごとKDRへ持ち込まない |
| Stage実行 | `intent-capture`の質問・確認・review・sensor・人間承認を接続。全33 Stageの実用化は未完 | 旧方式の拡張を新製品完成の前提にしない |
| receipt | PR #127でIdeation共通のsummary receipt内部基盤 | 新KDRの必須依存にしない。記録と承認証明を混同しない |
| OKF | PR #125のSpace内metadata検索 | 指定されたOKF Agent Memoryそのものの統合ではない。検索・保存先・rules供給を切替契約として扱う |
| Codex接続 | context配信・実読込とStage receiver。配置hookはUserPromptSubmitのみ | KDR確認用hookはこれから作る。現行配信transactionの流用は必要性で判断する |
| agent | 製品側は `aidlc-product-lead-agent.toml`。開発側にはplanner等の別定義がある | 製品の作業・reviewの役割を最小化する。開発側の必須review手順とは分離する |
| CI | Go test・race・vet・integrationと6 OS/arch構成のbuildを設定済み | 本repositoryのCIを継続利用する。利用先のCI整備を新しいStage実装待ちにしない |

実装履歴の主な根拠:

- [PR #123: intent-captureを質問から人間承認まで縦に接続する](https://github.com/sori883/ai-dd/pull/123)、merge `819279c1945c0e831057881db190d19eacb1abb9`。
- [PR #125: Space固有knowledgeをOKF metadataで段階的に検索する](https://github.com/sori883/ai-dd/pull/125)、merge `41e9e3da2d060f1063d41db5be3c47062bb99860`。
- [PR #127: feat: Ideation Stage共通のsummary receipt基盤を追加する](https://github.com/sori883/ai-dd/pull/127)、mergeは本記録の基準commit。Issue #126はclose済み。

取得した3 PRのcheckはいずれもCOMPLETED/SUCCESS。独立reviewの記録はPR本文の実施報告であり、GitHub review objectは各0件。
今回、Goの全検証や配布live E2Eを再実行したわけではない。過去の成功を新方式の動作証拠にはしない。

既知の注意点として `ApproveGate` は現在Stageのapproval audit・completed state保存後に後続能力を検証する。
固定graphで後続が非対応の場合の部分完了問題は未修正であり、新設計を考えただけでは直らない。
ユーザーは既存を無視してよいと回答したため、この旧経路の修正を新製品の先行作業にしない。問題は「解消済み」ではなく旧経路の既知制約として残す。

## 確定している境界

- rules・設計・knowledgeには指定のOKF Agent Memoryを使う方向性。
- 1 Intentにつき1 KDR。同じIntentの反復・再開では同じファイルへ記録する。
- hookの強制は通常のAI操作の記録漏れ防止。OS権限による全経路の書込み禁止は求めない。
- 新方式に工程遷移stateと全操作auditを要求しない。
- 外部Go moduleを追加しない。既存の承認済みYAML依存を新規追加とは扱わない。
- 設計資料はdocs、実装と製品配布用資料はsrc。開発側のRAMと利用先のOKF/KDRを混在させない。

## マイルストーン

| 段階 | 到達する状態 | 完了を判断する証拠 |
| --- | --- | --- |
| M0 現状を整理し、切替契約を決める | 続けるもの・置き換えるもの・既存互換性を要求せず、新規Intentの契約が分かり、最初の実装範囲を承認できる | 現状表、旧RAMとの対応、KDR/Intent/hook/OKFの最小契約、移行方針を含む自己完結した計画 |
| M1 一つのIntentを新方式で完走する | OKF読込、IntentとKDR作成、実装・test反復、review、結果記録、再開が最小構成でつながる | 固定したCodex環境で一連の動作を実証。KDRなしの対応変更が止まり、記録後に進める |
| M2 日常作業で壊れず使えるようにする | 中断、保存失敗、複数Intent、共有設計の変更を扱える | 再試行・競合・誤Intent・検証の古さを確かめるtestと、実作業での運用確認 |
| M3 最小構成を配布し、旧経路を整理する | 利用者が最小構成を導入・更新でき、不要なStage依存を保守しなくてよい | fresh導入・新方式の更新・rollback確認、配布物とhelpの整合、旧経路削除後のCI |

M0の現状調査は今回実施した。切替契約は未確定なのでM0全体を完了扱いにしない。
M1を最初の利用可能な成果とする。KDR CLIだけを作ってhookやOKF接続を先送りした状態を、M1完了にはしない。

### M0: 最初の実装前に固定すること

1. 既存Intent・stateの移行は不要とユーザー確認済み。新規Intentだけで完結する構成にする。旧方式の識別や保存形式を新方式へ引き継ぐ必要はない。
2. Intentの識別、KDRの配置・必要項目、対象Intentの明示方法。ファイルの存在だけで記録済みとするか等を曖昧にしない。
3. hookが確認するtool経路、記録更新を求める節目、読み取り・記録修復・質問待ち・中断・故障時の振る舞い。
4. OKFの固定版、取得・配布方法、共有bundleの正本、必須rulesの明示読込、知識変更の採用方法。
5. 新方式の配布対象と戻し方。旧公開コマンド互換・旧data移行・旧承認問題の修正は新方式の受入条件から外す。

推奨する初期対象は一つのCodex環境と新規Intent。OKFとhookの対応versionを固定して実証し、対象harnessを後から広げる。
既存を無視してよいとの回答により、移行と新旧二重運用は不要。根拠は[移行不要の合意](2026-09-07-minimal-product-no-legacy-migration.md)。

### M1: 薄い全体接続

依存順は、Intent/KDR保存 → OKF読込と作業手順 → hook接続 → 一周確認。
Issue/PRはレビュー可能な単位に分けるが、ユーザーが使える到達点は一つにする。

- KDR: template/read/create/update/check。開始・変更・結果・再開を同じIntentのファイルに記録する。
- OKF: 指定版CLIを使い、必須rulesと関連設計を読む。小さな共有知識の更新までつなぐ。
- agent: 作業担当は質問・調査・試作・実装を行い、レビュー担当は固定した対象版をread-onlyで確認する。
- hook: 対応操作の前後・終了時に最小契約を確認する。全toolのauditは保存しない。
- 完走例: 小さな機能を追加し、test失敗→実装→test成功→review修正→検証→結果記録→別sessionで再開する。

この段階ではStage graphやsummary receiptを経由せずに完走できることを確認する。
UI試作・調査でまだ実装しないIntentもKDRに記録でき、test未実施を成功と表示しない。

### M2: 再開・共有・並行作業

- 保存中断・書込み失敗でも直前のKDRを破損させず、競合更新を黙って上書きしない。
- 二つのIntentを並行してもKDRを取り違えない。reviewerが同じファイルを同時編集する構成にしない。
- crash後はKDRとGit/PR/CIを照合して再開し、既に行った外部操作を無条件に再実行しない。
- 共有設計の参照版を追え、変更案と採用済み設計を区別できる。
- コード変更後に古いtest・review結果を新しい版の成功として扱わない。
- 質問待ちやユーザー中断をhookが無限に妨げない。hook/OKF/CLIの不調時に復旧手順が分かる。

保存の基本的安全性はM1から必要。M2は不安全なM1を許す段階ではなく、複数作業と実運用の確認を拡張する段階である。

### M3: 切替と縮小

M1/M2の証拠が揃ってから、新規利用の入口・配布物を新方式へ統一する。旧方式との互換layerや二重運用機能は作らない。
不要なStage graph・state遷移・audit・receipt・旧receiverとtestは、依存を確認した範囲で段階的に削除する。
旧dataと意思決定の履歴を削除することとは区別する。
初回導入、新方式の更新、必須hookの有効性確認、新方式のrollbackを検証する。旧data移行のE2Eは要求しない。
README、help、製品agent、配布資料、CIを新方式へ揃えた時点で完了とする。

## 主な対象候補と検証の組み方

| 対象候補 | 用途・注意 |
| --- | --- |
| `src/internal/workspace/`、`pathnorm/`、`recordlock/`、`state/`の保存処理 | 再利用可否を精査。Intent作成とStage state初期化を分離できるか確認 |
| 新規 `src/internal/kdr/`、`src/internal/cli/`、`src/cmd/aidlc/` | KDRとIntent操作。pathと公開コマンド名はM0で確定 |
| `src/harness/codex/hooks.json`、`skills/`、`agents/` | 製品用の記録手順・hook・最小agent |
| `src/internal/okf/`、`knowledge/`、`delivery/`、`src/core/` | 指定OKFへの接続と既存供給経路の整理。全部流用する前提にしない |
| `src/internal/orchestrator/`、`audit/`、`graph/`、`stageplan/`等 | 旧経路の結合確認とM3の削減候補。今回削除はしない |
| `.github/workflows/ci.yml`、`docs/architecture.md`、`docs/development.md`、RAM | 新旧の検証範囲・配布手順・決定の更新 |

具体的なGo実装計画では、1 Issueを単独implementerの1 work unitとし、以下の順で必要なTDD項目を定める。
Intent/KDR対応と不正入力 → 安全保存と再試行 → 公開CLI → OKF失敗・読込 → hookの拒否/許可/復旧 → 一周のintegration。
各項目のtest名・exact commandは公開契約と対象fileが固まった時点で計画へ記載し、存在しないtestを今回の検証証拠にしない。
loopはtargeted test、固定base/headの独立review後に親がread-only finalを1回実施する。
finalは該当する全package test、race、vet、format確認、integration、cross buildと配布E2Eを集約する。
各PRはIssueへ紐づけ、GitHub checks成功後にmerge commit方式でmergeし、main反映とIssue closeを確認する。

## 許可・旧ロードマップとの関係

本案は、固定AI-DLC 2.6.123の工程stateとauditを前提とした旧ロードマップを、製品として意図的に置き換える提案である。
理由は作って試すまでの負担と日々の管理を減らすため。公開CLI、保存形式、導入・再開手順には互換性の影響がある。
本家最新upstreamへの準拠を主張しない。指定OKF Agent Memoryは前調査固定commit
`d4c523ed5ce916fa207fe314851b98721421c891`を検討基準とし、従来OKF仕様snapshotとは分ける。

採用する場合、旧「33 Stage実用化」、ARTIFACT_CREATED/UPDATED receipt、Stage共通sensor、market-research縦切りは新方式の前提作業から外す。
旧ロードマップや過去RAMは履歴を残し、置換対象と既存互換性を要求しない合意を新しい承認記録で明示する。
現時点では包括承認済みと扱わず、新Issue・PR・コード変更を開始しない。
M0の重要な契約を確定後、最初はM1の一まとまりを実装許可の対象にする案を推奨する。
M2/M3は到達目標を共有しつつ、実証で判明した新方式の配布・運用の判断を承認した後に実行する。

## 残る確認

既存Intent移行の確認は「既存は完全無視でいいです」との回答で解決した。
残るM0契約は、KDRの保存・Intent識別・hookの機械判定・OKF固定版と配布方法である。これらを一つの具体案へまとめ、M1の実装承認を求める。

## ローカルの主な根拠

- [旧ロードマップ](2026-09-03-aidlc-implementation-roadmap.md)、[能力対応表](../research/2026-09-07-stage-phase-production-capability-matrix.md)。対応表はPR #125基準なのでsummary共通化はPR #127で補完した。
- [公開help](../../../src/internal/cli/cli.go)、[CLI接続](../../../src/cmd/aidlc/main.go)、[Intent作成内部API](../../../src/internal/workspace/intent_create.go)。
- [承認の保存順](../../../src/internal/orchestrator/approve.go)、[後続能力guard](../../../src/internal/orchestrator/gate.go)。
- [現行hook](../../../src/harness/codex/hooks.json)、[CI](../../../.github/workflows/ci.yml)、[開発agent運用](../../agent-workflow.md)。
