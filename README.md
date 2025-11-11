# Middleware for SealDice OneBot 文件跨机发送

用于在 `sealdice-core` 与 OneBot 协议实现端位于不同机器时，实现文件发送功能的中间件，无需改动两端代码。

## 组件说明

- `middleware-a`：需位于 `sealdice-core` 机器，充当 WebSocket 代理。
  - 拦截 OneBot 动作 `upload_private_file` / `upload_group_file`。
  - 将本地文件上传至远端 `middleware-b` 并获取 URL 或本机绝对路径。
  - 对 `upload_*_file`：优先改写其 `file` 为 b 返回的本机绝对路径（go-cqhttp 端可直接读取）；若不可用则降级为 `send_*_msg` + `[CQ:file,file=<url>,name=<name>]`。
  - 对 `send_*_msg` 中的 `[CQ:image]`/`[CQ:record]`：检测 `file` 为本地路径或 `base64://` 时，上传到 b 并改写为网络 URL（`file=<http(s)://...>`），实现跨机发送。
  - 其余事件与动作透明转发。

- `middleware-b`：需位于协议端机器（与 go-cqhttp 同机），提供文件上传与静态文件访问。
  - 接收 `multipart/form-data` 上传，存储到本地目录。
  - 返回可公开访问的 URL，供 go-cqhttp 取用。

## 构建

本项目使用 Golang 1.25.3 进行编写，建议以该版本进行代码编写与编译。

```powershell
cd middleware-b
go build

cd ../middleware-a
go build   # 若依赖下载失败，请设置 GOPROXY="https://goproxy.cn,direct"
```

## 配置与启动

### middleware-b（协议端侧）

编辑 `middleware-b/config.json`：

- `listen_http`: 监听地址，例如 `":8082"`
- `storage_dir`: 文件存储目录，例如 `"uploads"`
- `public_base_url`: 该服务的外部可访问基地址，例如 `"http://<b-host>:8082"`

启动：

```powershell
.\middleware-b.exe -config config.json
```

亦可以配置好 `config.json` 文件后双击启动

### middleware-a（sealdice-core 侧）

编辑 `middleware-a/config.json`：

- `listen_http`: 监听地址，例如 `":8081"`
- `listen_ws_path`: WebSocket 路径，通常 `"/ws"`
- `upstream_ws_url`: Onebot V11 实现端 WS 地址，例如 `"ws://<go-cqhttp-host>:6700/ws"`
- `upstream_access_token`: 若实现端设置了 token，填入此处
- `upstream_use_query_token`: `true` 时以 `?access_token=...` 形式传递（符合 OneBot v11 正向 WS 规范），`false` 时使用请求头 `Authorization: Bearer ...`
- `server_access_token`: 可选，若设置，则 sealdice-core 连接到本代理时需带上 `Authorization: Bearer <token>`
- `upload_endpoint`: 指向 `middleware-b` 的上传接口，例如 `"http://<b-host>:8082/upload"`

启动：

```powershell
.\middleware-a.exe -config config.json
```

亦可以配置好 `config.json` 文件后双击启动

在 `sealdice-core` 的 账号设置界面中，选择 `Onebot V11 正向WS`

在WS连接地址处输入 

`ws://<a-host>:8081/ws`

如出现 `已连接` 且的确未显示异常，则证明连接成功

## 行为与兼容性（OneBot v11）

- 代理会透明转发所有事件与动作，仅在检测到 `upload_private_file` / `upload_group_file` 时介入。
- 对于 `file` 字段：
  - `http(s)://...`：直接改写为 `[CQ:file,file=<url>]`，不经上传。
  - `base64://...`：解码后上传至 `middleware-b`，返回 URL 并改写为 `[CQ:file,file=<url>,name=<name>]`。
  - `file://` 或本地路径：读取本地文件上传至 `middleware-b`，返回 URL 并改写为 `[CQ:file,file=<url>,name=<name>]`。
- 上游鉴权支持 `Authorization: Bearer` 或 `?access_token=` 两种形式。

