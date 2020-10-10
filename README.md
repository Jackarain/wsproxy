## 一个支持websocket级联的socks5/http复合代理.

支持在同一个端口同时提供websocket、socks5、http代理服务, 支持通过websocket级联代理.

```
                 +-------------------+          |         +-------------------+
  browser/app -> | socks5/http proxy | -> wss --|-- wss ->|-- sock/http proxy |--> target
                 +-------------------+          |         +-------------------+
                     local server              Wall           remote server
```

## 编译

安装golang/git环境后, 在项目目录执行以下命令编译
```
go build
```

即可完成编译，编译生成可执行程序.

## 说明

证书文件必须位于程序运行目录的 .wsproxy/certs 下, 统一通过ca.crt签名出server和client证书.

remote server端用到
ca.crt
server.crt
server.key

local server端用到
ca.crt
client.crt
client.key

config.json 可参看 config.json.example 文件中的说明, 编写的config.json并放置于可执行程序同一目录.


##### 相关讨论群 https://t.me/joinchat/C3WytT4RMvJ4lqxiJiIVhg
