# OKF v0.2取得記録

## 固定対象

| 項目 | 値 |
| --- | --- |
| Repository | <https://github.com/GoogleCloudPlatform/open-knowledge-format> |
| Default branch | `main` |
| Commit | `ad30107c31c06aec8a7d5636e0d1058118604e6f` |
| Commit日時 | `2026-08-21T20:08:36Z` |
| 取得日 | 2026-09-03 |
| 仕様宣言 | `SPEC.md`本文の`Version 0.2` |
| License | Apache License 2.0 |

固定時点でGit tagは公開されていなかったため、`main`のcommit SHAを再現可能な境界として使う。
この固定コピーは最新upstreamを表すものではない。

## ファイル

| ローカルファイル | 固定元 | SHA-256 |
| --- | --- | --- |
| [SPEC-v0.2.md](SPEC-v0.2.md) | [SPEC.md](https://github.com/GoogleCloudPlatform/open-knowledge-format/blob/ad30107c31c06aec8a7d5636e0d1058118604e6f/SPEC.md) | `26aa5da029278939f914e578107242d9607d4f2dc5fe153272b82f9ed1030101` |
| [LICENSE.md](LICENSE.md) | [LICENSE.md](https://github.com/GoogleCloudPlatform/open-knowledge-format/blob/ad30107c31c06aec8a7d5636e0d1058118604e6f/LICENSE.md) | `8c6db340475136df3c1201d458fa5755698eace76e510471ecc9d857d6083dac` |

## 更新手順

1. 公式repositoryで対象仕様が宣言するversionとcommitを確認する。
2. 新しいcommitの仕様とlicenseを別差分として取得する。
3. 仕様上の変更を観点別分析へ反映し、旧版との差分を記録する。
4. local fileのSHA-256を再計算してこの表を更新する。
5. local snapshotのversionと最新upstreamを区別して報告する。

公式ファイルはApache License 2.0に従って無改変で保存する。
