# 自動取得の入口で使うOS標準ツール

2026-09-14。対象はmacOS/LinuxのshとWindows PowerShell 5.1/7。
Go、Python、Node.js、Gitを利用者へ追加要求せず、最初のinstaller取得だけを実行する。
Context7でcurlとPowerShellの公式資料を検索した。PowerShellは7.7の説明が返ったため、
5.1固有の挙動はMicrosoftの5.1原文で補った。

- [curl公式manual](https://curl.se/docs/manpage.html)の`--disable`を最初に置いてcurlrcを無効化する。
  `--proto '=https'`、`--proto-redir '=https'`、`--location`、`--fail`を使い、
  HTTPSだけでredirectを追う。証明書検証を省略しない。取得先の初期URLは本repositoryの固定Release。
- [PowerShell 5.1のInvoke-WebRequest](https://raw.githubusercontent.com/MicrosoftDocs/PowerShell-Docs/main/reference/5.1/Microsoft.PowerShell.Utility/Invoke-WebRequest.md)
  は`-UseBasicParsing`でHTMLの完全解析を避け、HTTP失敗を例外として扱える。
  入口のscript本文取得例はこの方式にする。archive取得には既存の`curl.exe`を明示して使い、
  PowerShellの同名aliasと区別する。OSにcurl.exeがない場合は不足を表示し、追加installしない。
- [Get-FileHash](https://learn.microsoft.com/en-us/powershell/module/microsoft.powershell.utility/get-filehash?view=powershell-5.1)
  はSHA-256と`-LiteralPath`を利用できる。ファイル名をwildcardとして解釈しない。
- [標準.NET ZipArchive](https://learn.microsoft.com/en-us/dotnet/api/system.io.compression.ziparchive?view=netframework-4.8.1)
  でinstallerメンバーだけを読み、scriptが作った固定の一時ファイルへ保存する。
  archive全体の展開と検査は起動後のGoへ任せる。
- [ProcessArchitecture](https://learn.microsoft.com/en-us/dotnet/api/system.runtime.interopservices.runtimeinformation.processarchitecture?view=netframework-4.8.1)
  で実行環境のCPUを判定できる。利用できない環境では推測せず停止する。

ローカルmacOSでsh、curl 8.7.1、bsdtar、shasum、mktemp、Darwin/arm64を確認した。
PowerShellはローカルにないため追加せず、Windows CIで5.1と7の実行を確認する。
shellの判定は`uname -s/-m`による実行環境を基準とし、エミュレーション中に物理CPUへ
勝手に切り替えない。Windowsもプロセスのarchitectureを選び、起動したGoのtargetと一致させる。

archiveのSHA-256を先に確認した後、shはtarの標準出力から、WindowsはZIPのstreamから
installerだけを固定pathへ取り出す。リンクや重複installerを拒否する。
Goが一式を再検査するまで他のmemberをファイルシステムへ展開しない。
PowerShell実行policy、信頼設定、TLS検証、管理者権限の変更は行わない。
これらは採用案の根拠であり、3OSのscript実機成功をまだ示す記録ではない。
