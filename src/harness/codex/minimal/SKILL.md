---
name: aidlc
description: 名前でIntentを開始・再開し、SpaceのOKF知識と同じKDRを更新する。
---
# Intentの作業

実行ファイルは @@BINARY@@。対象Spaceは利用者の指示から特定し、操作では --space を明示する。
hookが案内するsession IDと会話専用draft pathを使い、利用者にIDの手入力を要求しない。

新しい目的では `aidlc kdr template --space <space>` を読み、会話専用draftへ6節を具体的に記入する。
`aidlc intent create <name> --space <space> --file <draft> --actor <実際のactor>` で作成する。
名前はdraftのtitleと一致させる。既存の目的では `aidlc intent list --space <space>` で名前を確認する。
`aidlc intent switch <name> --space <space> --session <session>` でKDRと必須ルールを全文読む。
同名候補は利用者に確認して --id で特定する。作成だけでは選択されない。

一般操作は直列にする。長時間Bashはwrite_stdinで終了までpollし、実行中にKDR更新や別操作を始めない。
判断・結果がまとまったら `aidlc kdr show <id> --space <space>` を読み、hashと全文を取得する。
会話専用draftだけをapply_patchで更新し、同じID・未知metadataを保持する。
`aidlc kdr update <id> --space <space> --file <draft> --expect <hash> --session <session> --actor <実際のactor>` で記録する。
日時やmetadataだけの変更では記録を補完できない。質問待ちも理由と再開点を書く。

欠落・破損はshow --rawで現物を確認し、同じIDへrepairする。正常本文は保ち、復旧後に通常updateで経緯を書く。
競合や部分保存失敗では再読込みし、無条件の上書き・新ID作成・resetをしない。
未記録の別Intentへ切り替えず、同じKDRへ戻る。残留toolは実process停止を確認してからbind --recoverする。

独立レビューは固定コード版の別Git checkout・別root会話でread-only sandboxを使う。
レビュー先にはwriter hookを配置せず、同じKDR・必須ルール・差分を読ませ、結果をwriterがKDRへ記録する。
共有知識の採用は必須ルールの合意に従いmemory create/updateを使う。
