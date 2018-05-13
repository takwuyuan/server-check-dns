# server-check-dns

設定ファイルに記載されたドメインを応答するDNSサーバーです。
記載されたサーバーは複数指定し、それぞれにpingを打って稼働を確認できたサーバーのIPを応答します。


- ラウンドロビンは想定していません。
- 設定ファイルの servers に記載された順で ping を打ちます。

想定しているのは、冗長化しているが障害時はhostsファイルなどを手動で切り替える必要がある環境です。



# TODO
- [ ] ログファイルを指定できるようにする
- [ ] 生存確認方法を ping 以外の方法をできるようにする。
    - [ ] tcp
    - [ ] http
    - [ ] external script