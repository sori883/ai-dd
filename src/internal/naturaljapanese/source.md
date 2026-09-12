# 規則の原典

coji/natural-japanese v1.5.0、commit 9a78a42964096da509b8f3e011f0085a5f080151 の scripts/lint.py と scripts/textcore.py をGoへ移植した。https://github.com/coji/natural-japanese/tree/9a78a42964096da509b8f3e011f0085a5f080151 。MIT許諾はLICENSE。testdataの2文書も同じ原典由来。

自動規則は固定lint実装を正本とし、旧手動指針のしきい値を採らない。通常14分類のみでexperimental/reading-load/semanticを含めない。Kagome/UniDicへの解析器変更と字句分割の差はsrc/docs/natural-japanese-go.mdに記載した。
