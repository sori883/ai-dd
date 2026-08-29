# 技術負債のシグナル

## 評価方法

ここでいう「技術負債」は確定したbugだけではなく、変更コスト、再現性、障害検出力、保守性を
将来悪化させる可能性があるシグナルを含む。優先度は影響と発生可能性の組み合わせ、確度は
ローカルスナップショットから直接確認できる強さを表す。

P0相当の即時障害や、配布物全体の破損は確認していない。

## 優先度付き一覧

| 優先度 | シグナル | 確度 | 主な影響 |
| --- | --- | --- | --- |
| P1 | 正規Codex生成物と配置物の6ファイルdrift | 高 | 再生成でローカルprovider設定が消える、由来が不明になる |
| P1 | Claude Agent SDK依存グラフの既知high severity advisoryを許容 | 高 | 依存更新までリスクを受容し続ける |
| P1 | `aidlc-lib.ts` への責務とexportの集中 | 高 | 変更影響範囲、レビュー負荷、循環依存リスクが増える |
| P2 | semantic coverageの未被覆面と行/分岐coverage不在 | 高 | 内部branchや例外経路の回帰を見逃す可能性 |
| P2 | live-model/harness経路が常設CI外 | 高 | provider・CLI版・TTY差による回帰検知が遅れる |
| P2 | Knip設定が品質ゲートとして動いていない | 高 | 未使用export/fileや依存の蓄積 |
| P2 | formatter無効、Markdown lint範囲が限定的 | 高 | 大量のTS/Markdownでstyle driftが蓄積 |
| P3 | オンボーディング文書の重複段落 | 高 | 文書肥大化と将来の片側更新 |
| P3 | 配布runtimeのBun版が利用者環境任せ | 中 | CIと利用環境のversion差による互換性問題 |
| P3 | 生成物を含む巨大snapshot | 高 | AI参照時のcontext浪費、検索noise、誤ってdistを編集するリスク |

## P1: 配置物の生成drift

### 根拠

実装の `dist/codex/` と `docs/配布_ai-dlc/` は各333ファイル。内容差は次の6ファイル。

- `.codex/config.toml`
- `.codex/agents/aidlc-architecture-reviewer-agent.toml`
- `.codex/agents/aidlc-delivery-agent.toml`
- `.codex/agents/aidlc-operations-agent.toml`
- `.codex/agents/aidlc-pipeline-deploy-agent.toml`
- `.codex/agents/aidlc-product-lead-agent.toml`

config差分はBedrock/model/AWS固定値の無効化、agent差分はmodel IDのprovider prefix除去である。
意図的な運用調整と読めるが、`package.ts --check` が保証する正規生成物ではない。

### 推奨

- 6ファイルを手編集結果として持つのではなく、review可能なoverlayまたはpost-processとして定義する。
- 配置物更新時に「正規distとの許可差分がこの6ファイルだけ」を検査する。
- provider/modelの差は秘密情報ではない範囲で設定理由と所有者を記録する。

## P1: 既知high severity依存

### 根拠

`security-scanners.yml` が、Claude Agent SDK依存グラフに既知のhigh severity transitive advisoryが
あると明記する。Bun auditはartifactへ全件を残すが、CIを失敗させるのはcriticalだけ。

### 推奨

- advisory ID、影響経路、到達可能性、回避策、期限を追跡する。
- SDK更新ごとにlockを再監査し、high thresholdを例外付きblockingへ引き上げられるか確認する。
- 配布runtimeに含まれないtest-only経路なら、その事実を証拠付きでrisk acceptanceへ残す。

## P1: 中心モジュールの肥大化と結合

### 根拠

- `core/tools/aidlc-lib.ts`: 約23,333行、約580の `export` 宣言。
- `aidlc-orchestrate.ts`: 約8,574行。
- `aidlc-utility.ts`: 約8,346行。
- `aidlc-lib.ts` は約49の手書きTypeScript moduleからimportされる。
- 簡易静的解析では、lib、audit、graph、schema群の間に循環依存群が見える。

`aidlc-lib.ts` はatomic file IO、workspace、state、path、Git等の異なる関心を共有しており、
変更時に広い回帰確認が必要になる。

### 推奨

- 先にAPI利用グラフとcharacterization testを固定し、責務別moduleへ段階的に抽出する。
- 低levelのpath/atomic IO、domain schema、state service、Git adapterの依存方向を一方向にする。
- export追加を抑え、内部実装と安定contractを分ける。
- refactorは配布byte parityとsemantic coverage ratchetを保った小さいsliceで行う。

静的import解析は動的importとtype-only edgeを厳密に扱っていないため、循環の詳細はrefactor前に
TypeScript ASTまたは依存解析toolで再確認する。

## P2: カバレッジの空白

### 根拠

semantic coverageは強い独自指標だが、未被覆面が残る。

- function: 519中299、57.6%。
- audit event: 91中49、53.8%。
- stage: 33中11、33.3%。
- subcommand: 138中125、90.6%。
- scope/hook/render surfaceは100%。

また、行・分岐・変更行coverageの収集とthresholdは確認できない。

### 推奨

- まずstage、audit、functionの未被覆一覧をリスク順に分類する。
- semantic coverageを置換せず、Bunのline/function/branch reportを補助指標として追加する。
- threshold導入前にbaselineを計測し、変更行coverageから段階導入する。
- generatorが列挙しない内部branchにはtargeted unit/fuzz testを追加する。

## P2: live経路のCI空白

### 根拠

CIのintegration/e2eは `--no-llm` でClaude SDK、TUI、Kiro、Codex exec等のlive gateを
明示的に閉じる。これは決定性とcredential分離には有効だが、実モデル・実CLIの互換性を常時検査しない。

### 推奨

- 通常PR gateは現状の決定論的suiteを維持する。
- credential管理されたschedule/manual workflowで主要providerの最小canaryを動かす。
- harness最低版と最新検証版のmatrixを記録する。
- live failureを製品bug、provider drift、quota/infra failureへ分類する。

## P2: 休眠しているKnip設定

### 根拠

`knip.json` はあるが、`package.json` にKnip依存・scriptがなく、CIからも呼ばれない。

### 推奨

利用するならversionを固定し、まずreport-onlyでbaselineを整理してからgate化する。
利用しないなら設定を削除し、期待される品質保証を誤認させない。

## P2: style gateの限定

### 根拠

- Biome formatterは明示的に無効。
- organize importsも無効。
- markdownlintはrootの5文書だけで、`docs/`、`core/`、配布Markdownを直接検査しない。
- Zensical strict buildはサイト構造を検査するが、Markdown style lintとは異なる。

### 推奨

大規模一括整形を避け、変更ファイル限定formatterまたは段階的baselineを検討する。
Markdownは生成物を除いた手書き領域から対象を広げる。

## P3: 生成元にある文書重複

`core/templates/onboarding.md` で `Document knowledge (DocumentKB)` の長い項目が連続して重複し、
そのまま `dist/codex/AGENTS.md` と配置版 `AGENTS.md` に生成されている。機能障害ではないが、
オンボーディングcontextを余分に消費し、将来片方だけが更新される可能性がある。

生成物を直接直さず、共通templateの重複を除いて全ハーネスを再生成するのが正しい修正箇所である。

## P3: runtime version差

CIはBun `1.3.14` を固定するが、配布物は利用者PATH上の `bun` を使う。minimum/maximum rangeの
machine-readable enforcementはCodex onboardingからは確認できない。`aidlc --doctor` はCodex版を
検査するが、Bun互換範囲の保証は別途確認が必要。

## P3: 巨大snapshotとAI context

実装は3,364ファイル、うち `dist/` が2,201ファイルで、同じ論理ソースが7ハーネスへ複製される。
これは配布・diff reviewには必要だが、AIが無差別に読むと重複contextが大半になる。

本分析インデックスを先に読み、設計質問は `core/` と `harness/`、実行時質問は配置物、
parity質問だけ `dist/` と配置物の双方を読む運用が必要である。

## 誤検知しやすいもの

次は数だけで技術負債と判断しない。

- `TODO` の多くはplugin templateの利用者記入欄や、TODO検出機能を試すfixture。
- `test.skip` の多くはOS、TTY、credential、live model条件の表現。
- `dist/` の大量重複は生成・配布戦略そのもので、手書き重複ではない。
- patch versionの大きさは更新頻度を示すが、それだけで品質低下を意味しない。
- `.gitleaks-baseline.json` の存在は既知検出管理を示すが、各entryの妥当性を調べずに負債とは断定しない。

## 主要根拠

- `docs/実装_aidlc-workflows/core/tools/aidlc-lib.ts`
- `docs/実装_aidlc-workflows/core/tools/aidlc-orchestrate.ts`
- `docs/実装_aidlc-workflows/core/tools/aidlc-utility.ts`
- `docs/実装_aidlc-workflows/core/templates/onboarding.md`
- `docs/実装_aidlc-workflows/tests/.coverage-registry.json`
- `docs/実装_aidlc-workflows/.github/workflows/security-scanners.yml`
- `docs/実装_aidlc-workflows/knip.json`
- `docs/実装_aidlc-workflows/biome.json`
- `docs/実装_aidlc-workflows/.github/workflows/markdownlint.yml`
- `docs/実装_aidlc-workflows/dist/codex/`
- `docs/配布_ai-dlc/`
