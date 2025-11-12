# Middleware for SealDice OneBot 文件跨机发送

### 鸣谢：Original Idea: [@Szzrain](https://github.com/szzrain)

### Inspired by:[@PaineNate](https://github.com/paiennate)


用于在 `sealdice-core` 与 OneBot 协议实现端位于不同机器时，实现文件发送功能的中间件，无需改动两端代码。

## 组件说明

### `middleware-a + middleware-b` 方案
- `middleware-a`：需位于 `sealdice-core` 机器（与 sealdice-core 程序同机），负责充当 WebSocket 代理。
  - 拦截 OneBot 动作 `upload_private_file` / `upload_group_file`。
  - 将本地文件上传至远端 `middleware-b` 并获取 URL 或本机绝对路径。
  - 对 `upload_*_file`：优先改写其 `file` 为 b 返回的本机绝对路径（onebot协议实现端可直接读取）；若不可用则降级为 `send_*_msg` + `[CQ:file,file=<url>,name=<name>]`。
  - 对 `send_*_msg` 中的 `[CQ:image]`/`[CQ:record]`：检测 `file` 为本地路径或 `base64://` 时，上传到 b 并改写为网络 URL（`file=<http(s)://...>`），实现跨机发送。
  - 其余事件与动作透明转发。

- `middleware-b`：需位于协议端机器（与协议实现端程序同机），负责提供文件上传与静态文件访问，需要 `middleware-a` 所在的机器可访问，否则可能无法正常工作。
- `middleware-b` 无需与协议端和 sealdice-core 程序对接，它只负责文件的持久化与读取。
  - 接收 `multipart/form-data` 上传，存储到本地目录。
  - 返回可公开访问的 URL，供协议实现端取用。

- `middleware-c`：用于连接已经配置好的`onebot v11`跨机器/跨容器/跨网段的协议端（Websocket服务端模式），适用于跨机部署和快速docker compose部署海豹，并链接已有的`onebot v11`协议端。该方案只需要启动`docker-compose`即可，无需双端部署。全程使用base64进行传输图片等内容。实现参考了`middleware-a`，原理是：`sealdice->middleware-c->onebot11(ws服务端)`，在传输过程中解析绝对目录的文件转换为base64。

简而言之，本项目本质上是将 **sealdice-core 程序的可读取目录** 内的图片/视频等外置资源转换到了 **协议实现端的可读取目录**，规避了跨机部署时协议实现端无法访问到海豹资源 URI 导致消息发送失败的问题

## 获取二进制文件
可以通过 [ action 构建 ](https://github.com/kenichiLyon/middleware-for-sealdice/actions) 获取，注意，**必须下载 middleware-a 和 middleware-b 并且部署才能正常工作**


## 构建

本项目使用 Golang 1.25.3 进行编写，建议以该版本进行代码编写与编译。

```powershell
cd middleware-b
go build

cd ../middleware-a
go build   # 若依赖下载失败，请设置 golang 的国内镜像站，这里我使用的是 goproxy.cn
```

## 配置与启动

### `a+b`方案
#### middleware-b（与协议端同机）

编辑 `middleware-b/config.json`：

- `listen_http`: 监听地址，例如 `":8082"`
- `storage_dir`: 文件存储目录，例如 `"uploads"`
- `public_base_url`: 该服务的外部可访问基地址，例如 `"http://< b 组件所在机器的 IP >:8082"`

启动：

```bash
.\middleware-b -config config.json
```

亦可以配置好 `config.json` 文件后双击启动

#### middleware-a（与 sealdice-core 程序同机）

编辑 `middleware-a/config.json`：

- `listen_http`: 监听地址，例如 `":8081"`
- `listen_ws_path`: WebSocket 路径，通常 `"/ws"`
- `upstream_ws_url`: Onebot V11 实现端 WS 地址，例如 `"ws://< Onebot 协议实现端所在机器的 ip >:6700"`
- `upstream_access_token`: 若实现端设置了 token，填入此处
- `upstream_use_query_token`: `true` 时以 `?access_token=...` 形式传递（符合 OneBot v11 正向 WS 规范），`false` 时使用请求头 `Authorization: Bearer ...`
- `server_access_token`: 可选，若设置，则 sealdice-core 连接到本代理时需带上 `Authorization: Bearer <token>`
- `upload_endpoint`: 指向 `middleware-b` 的上传接口，例如 `"http://< b 组件所在机器的 ip >:8082/upload"`

启动：

```bash
.\middleware-a -config config.json
```

windows用户 亦可以配置好 `config.json` 文件后双击启动

在 `sealdice-core` 的 账号设置界面中，选择 `Onebot V11 正向WS`

在WS连接地址处输入 

`ws://<a 组件所在机器的 IP >:8081/ws`

如出现 `已连接` 且的确未显示异常，则证明连接成功

### `c`方案
#### middleware-c

该方案可直接`docker-compose`启动`sealdice`，无需双端部署组件，并使用base64处理绝对路径的文件。也可以自行编译部署。

原理是：`sealdice->middleware-c->onebot11(ws服务端)` , `middleware-c`将对海豹核心的绝对路径的文件编码为base64，然后发送到OneBot11协议端，进而实现多机/跨网段/跨容器进行正常工作。

具体使用如下：

```bash
git clone https://github.com/kenichiLyon/middleware-for-sealdice
cd middleware-c
mkdir -pv docker-data/middleware-c
cp middleware-c/config.json.example docker-data/middleware-c/config.json
vim docker-data/middleware-c/config.json
```

修改`docker-data/middleware-c/config.json`

```json
{
  "listen_http": ":8081", #无需修改，用于海豹进行连接
  "listen_ws_path": "/ws", #无需修改，用于海豹进行连接
  "upstream_ws_url": "ws://127.0.0.1:6700", #上游onebotv11 ws服务器地址
  "upstream_access_token": "", #上游onebotv11 ws服务器验证秘钥
  "upstream_use_query_token": true, #上游onebotv11 是否使用秘钥
  "server_access_token": "",
  "upload_endpoint": ""
}
```

启动海豹：
```bash
docker compose up -d
```

打开本机3211端口（sealdice），在海豹配置添加客户端： ws://middleware-c:8081/ws，账号填写正确账号即可，验证秘钥可为空（与`server_access_token` 相同）

享受你的海豹吧。


## 行为与兼容性（OneBot v11）

- 代理会透明转发所有事件与动作，仅在检测到 `upload_private_file` / `upload_group_file` 时介入。
- 对于 `file` 字段：
  - `http(s)://...`：直接改写为 `[CQ:file,file=<url>]`，不经上传。
  - `base64://...`：解码后上传至 `middleware-b`，返回 URL 并改写为 `[CQ:file,file=<url>,name=<name>]`。
  - `file://` 或本地路径：读取本地文件上传至 `middleware-b`，返回 URL 并改写为 `[CQ:file,file=<url>,name=<name>]`。
- 上游鉴权支持 `Authorization: Bearer` 或 `?access_token=` 两种形式。
