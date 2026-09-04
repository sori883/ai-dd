# Go TDDの段階別handoff

この文書は、親とGo実装担当の間で仕事を受け渡す契約の正本である。
1回の依頼は1つの観測可能な振る舞い（slice）の、REDかGREENの片方だけにする。
REDはtestの失敗確認、GREENはそのtestを通す最小実装を意味する。
Issue全体の完了を1回のloop依頼に含めない。

```text
親 → RED依頼 → 担当が最終応答で終了
親が差分確認・同じtestを再実行
親 → GREEN依頼 → 担当が最終応答で終了
親が差分確認・同じtestを再実行 → 次のslice
```

途中のcommentaryや連絡toolの到着をgateにしない。担当が終了するまで次の依頼を予約・送信しない。
連絡toolが使えなくても最終応答で返せる。親が確認できない場合は停止し、包括的な「続けて」で進めない。
これは依頼の発行と検証の運用であり、OSの書込権限による隔離ではない。

## 入力と検証mode

既存のverification_mode=loop/review/finalは検証範囲を表す。tdd_phaseはloop内の仕事を表す別の値である。
verification_mode未指定は既存どおりloop。loopのtdd_phase未指定・不明値・red/green同時指定はBLOCKED。
最初に、今回の親メッセージからtdd_phaseの明示値を取り出す。新規機能だからRED、test fileがあるからRED、
前回REDだったからGREEN、という補完は禁止する。値がなければfileの編集・test実行をせず最終応答で不足を返す。
finalは親が独立review後に明示するread-only検証であり、tdd_phaseやRED受入を要求しない。
Go実装担当はreviewを受け付けない。

loopの各依頼には、過去の会話から推測せず使える次の項目を渡す。

| 項目 | 内容 |
| --- | --- |
| plan / authorization / issue | 自己完結した計画、直接承認または包括承認の根拠、実在するIssue |
| workdir / starting_head | 作業rootの絶対path、依頼開始時のGit HEAD |
| verification_mode / tdd_phase | loopと、redまたはgreenの片方 |
| slice_id / behavior | そのIssue内で一意の識別子、期待する1つの観測可能な動作 |
| test_files / implementation_files | phase別の編集対象path。対象外の変更は許可しない |
| test_command | 作業rootで実行する1件を特定したtargeted command。Go toolchainも固定 |
| scaffold | REDでAPI骨組みが必要な場合だけ、許可file・型・signature・空の返値を明示 |
| red_acceptance | GREENだけに必須。親が再確認した下記のRED受入 |

型、関数名、配置先などをこの段階で決められない場合は、先に計画へ戻す。
無関係なuser変更は保全する。変更前後のstatusと差分を比較し、既存の汚れを自分の変更と混同しない。
scopeを満たすために別機能も必要と判明した場合は、複数機能を一括実装せず親へ返す。

## RED依頼

1. 入力、HEAD、許可対象を確認する。不足があれば編集前にBLOCKEDで返す。
2. 指定した1振る舞いのrunnable testをtest_files内に書く。
   同じ境界のtable caseはよいが、別の振る舞いや次sliceのtestをまとめて追加しない。
3. 本体は編集しない。唯一の例外は、親が事前に明示したscaffoldだけである。
   scaffoldはtestをコンパイル可能にする宣言・空の返値に限り、振る舞いを実装しない。
   骨組みの許可がなければ、compile errorをREDとせずBLOCKEDで必要な宣言を報告する。
4. testをgofmtし、test_commandを実行する。指定testが実際に走り、意図した期待値不一致で
   失敗したことを確認する。compile error、環境障害、no tests to run、skipはRED_READYにしない。
5. 意図した失敗ならRED_READY、初回成功ならALREADY_GREEN、その他はBLOCKEDで最終応答を返す。
   どの結果でもここで終了する。本体修正、GREEN移行、次sliceの開始を行わない。

最初から成功するtestを失敗させるために、既存本体を消す・戻す・期待値を歪めることは禁止する。
既存の正しい振る舞いの補足testなら、親はALREADY_GREENと記録して扱いを判断する。
これを新しい振る舞いの実装前REDと呼ばない。

## 親のRED受入

担当の最終応答を受け取ってから、親が次を確認する。

1. 変更がtest_filesと明示scaffold内だけであること。testが期待動作を検査しており、
   本体の先行実装や意図しない変更がないこと。範囲外変更は隠して戻さず、停止して扱いを決める。
2. 同じworkdir・toolchain・test_commandを再実行し、同じtestの意図した失敗を実測する。
   担当の「REDでした」という文章だけでは受け入れない。
3. HEAD、対象test・実装・scaffold・fixture・module定義の内容をsnapshotとして記録する。
   各fileはSHA-256、未作成fileはABSENTとする。実行後に内容が変わったら受入を作り直す。

この確認はloopのtargeted検証であり、全package/race/vet等を実行する理由にはしない。
親がGREEN依頼へ直接添えるred_acceptanceは、次の情報を含む。

```text
slice_id: 同じ識別子
workdir: 同じ作業root
head: 親の再実行時のGit HEAD
test_command: 再実行した正確なcommand
observed_failure: 指定test名と意図した期待値不一致
exit_code: 実測した非ゼロ終了code
files:
  各対象path: SHA-256またはABSENT
```

実装担当自身のRED_READY、repository内の未確認メモ、別sliceや過去turnの成功報告を、
親によるred_acceptanceの代わりにしない。親だけが確認済みのGREEN依頼を発行する。
受入証拠はhandoffと対応Issue/PRの実施記録に残し、一時的なhashや全出力をRAMへ大量保存しない。

## GREEN依頼

1. 通常の入力に加え、親が直接渡したred_acceptanceを確認する。欠落は編集前にBLOCKED。
2. workdir、slice_id、HEAD、test_command、すべての対象fileのhash/ABSENTを実物と照合する。
   red_acceptance.filesの全entryを対象にし、test・実装以外の文書や既存のuser変更も除外しない。
   hashは目視で一致と判断せず、親から渡された期待値を使うcheck command（例: shasum -a 256 -c）で
   全件を機械比較する。ABSENTは通常fileだけでなくsymlinkも存在しないことを確認する。
   全件の一致を示す終了code 0を得る前に編集しない。不一致を「無害な変更」として許容しない。
   変更対象を含む受入対象が足りない場合や、内容・root・HEADが違う場合はSTALEとしてBLOCKEDで返す。
   自分でhashを書き換えて受入を更新しない。
3. implementation_files内で、その1振る舞いだけを通す最小実装を行い、同じtest_commandで成功を確認する。
   受入済みtestを変更・削除・skipしたり、実行commandを緩めたりしない。
4. GREEN上で当該範囲の必要なrefactorとgofmtを行う。受入済みtestの編集が必要なら親へ返す。
   影響を受ける既存のtargeted testと、gofmt後の同じtest_commandを確認する。
5. GREEN_READYまたはBLOCKEDで最終応答を返して終了する。次のtest、次slice、全体finalへ進まない。

test側の誤りや別仕様が見つかった場合は、期待値を実装へ合わせず親へ返す。
親がtest変更を必要と判断したら、受入を破棄し、修正対象を限定したRED依頼から再確認する。
独立した振る舞い変更を必要とするrefactorは同じGREENへ混ぜない。

## 返却と親のGREEN確認

各phaseの最終応答には次を含める。途中メッセージはこれを代替しない。

- status: RED_READY / GREEN_READY / ALREADY_GREEN / BLOCKED
- slice_id、tdd_phase、変更path（自分の変更と既存変更を区別）
- 実行した正確なcommand、終了code、test名、意図した失敗または成功の要点
- GREENでは受入hashの全件機械比較に使ったcommandと終了code。不一致ならそのpath
- 対象fileのhash/ABSENTとHEAD、未解決事項。BLOCKEDなら不足gateをまとめて示す

親はGREEN_READYを受け取った後も、受入済みtestが同一で、変更が1件の実装範囲だけであることを確認し、
同じtest_commandを再実行する。成功と差分を確認するまでは次sliceを依頼しない。
検証できない場合は停止し、未確認を成功と解釈しない。

すべてのsliceを確認した後は、既存の固定差分独立review、read-only final、GitHub checks、mergeへ進む。
review findingの修正も、新しい振る舞いの回帰testについてRED/GREENを別依頼にする。
文書・設定だけの変更や、既に正しい振る舞いへの補足testに人工REDを要求する規約ではない。
