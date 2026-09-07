# OKF Go実装のローカル参照先

- 日付: 2026-09-07
- 状態: Accepted（参照先についてのユーザー直接指示）

ユーザーは `docs/実装_okf-agent-memory/` を「OKF goの実装参考」として使うよう指示した。
今後、OKF Agent MemoryのGo実装を確認するときは、このローカル資料から必要な箇所を参照する。
これは製品コードへの組込み、外部tool導入、M0契約案の採用、M1実装開始の承認ではない。

`go.mod`にはmodule `github.com/okf-memory/okf-agent-memory`、Go `1.22.0`が記載され、require節はない。
ディレクトリ内に独立した `.git` はなく、ここでの `git rev-parse HEAD` は親のai-ddのcommitを返す。
その値をOKFの版として扱わない。ローカル資料の元commitは現時点で未確認であり、設計案の固定候補
`v0.1.2` / `d4c523ed5ce916fa207fe314851b98721421c891` と同一とは断定しない。
版に依存する判断では必要な対象fileを固定upstreamと照合する。

参照元のファイルは変更しない。判断は開発側RAMへ記録し、利用プロジェクトのbundleとは分ける。

関連: [M0契約案のRAM](2026-09-07-minimal-product-contract-proposal.md)、
[最小契約案](../../design/minimal-product-contract-m0.md)、
[ローカルGo module](../../実装_okf-agent-memory/go.mod)。
