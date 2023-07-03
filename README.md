# 一个支持websocket级联的socks5/http复合代理.

支持在同一个端口同时提供 `websocket`、`socks5`、`http` 代理服务, 支持通过 `websocket` 级联代理.

```bash
                 +-------------------+          |         +-------------------+
  browser/app -> | socks5/http proxy | -> wss --|-- wss ->|-- sock/http proxy |--> target
                 +-------------------+          |         +-------------------+
                     local server              Wall           remote server
```

## 编译

安装 `golang`/`git` 环境后, 在项目目录执行以下命令编译

```bash
go build
```

即可完成编译，编译生成可执行程序, (注意, 上图中 `local server` 和 `remote server` 是同一个程序).

另外，`remote server` 可以配置多个以用于负载均衡，具体参考 `config.json.example` 文件.

## 说明

证书文件必须位于程序运行目录的 `.wsproxy/certs` 下, 统一通过 `ca.crt` 签名出 `server` 和 `client` 证书.
证书创建可以参考网上教程，如: [https://openvpn.net/community-resources/setting-up-your-own-certificate-authority-ca/](https://openvpn.net/community-resources/setting-up-your-own-certificate-authority-ca/)

`remote server` 端用到
`ca.crt`
`server.crt`
`server.key`

`local server` 端用到
`ca.crt`
`client.crt`
`client.key`

`config.json` 可参看 `config.json.example` 文件中的说明, 编写的 `config.json` 并放置于可执行程序同一目录.

## 意见和反馈

有任何问题可加tg账号: [https://t.me/jackarain](https://t.me/jackarain) 或tg群组: [https://t.me/joinchat/C3WytT4RMvJ4lqxiJiIVhg](https://t.me/joinchat/C3WytT4RMvJ4lqxiJiIVhg)
