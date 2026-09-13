# 原典とGo版の対応

原典: https://github.com/coji/natural-japanese/tree/9a78a42964096da509b8f3e011f0085a5f080151 （v1.5.0、MIT）。[許諾全文](../LICENSE)。設計→執筆→検査→収束をAI-DLCの担当とOKF保存へ調整した。

通常lintの14カテゴリ、catalog、統計式、genreを固定scripts/lint.pyとtextcore.pyから移植する。Kagome v2.11.0、kagome-dict/uni v1.2.6を使用し、Python/Sudachi、outline/terms、semantic、実験検査の実行は引き継がない。品詞はUniDicの階層、原形はLemma、読みは活用した発音形Pronを使う。汎用Reading APIはUniDicで未定義。サ変名詞＋するは述語catalogの照合時だけ連結する。辞書の分割や表記はSudachiと異なるため全入力の同一結果を保証しない。

Markdownのfrontmatter、見出し、箇条書き、引用、表、フェンス、コメント、inline codeとリンクURLを除外する。原典どおりインデントcodeは除外しない。CRLF/CRはLFへ正規化し行番号を保つ。名詞終止・語彙検査の文字数gateは解析した文の原文部分の合計であり、除外された見出しやcodeは加えない。
