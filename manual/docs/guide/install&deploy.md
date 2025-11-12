---
outline: deep
---

# 安装部署

本项目目前存在两种方案，可根据实际情况选用。

## 选用方案

:::tabs keys:case

== a+b 方案

### a+b 方案优点

无需改动现有的 `sealdice-core` 程序，也无需改动 `OneBot V11` 协议端程序，下载好对应中间件后启动即可无感部署。

### a+b 方案缺点

不适用 docker 环境，需要下载 `middleware-a` 与 `middleware-b` 两个中间件程序；同时，需要 `OneBot V11` 协议端所在机器具备公网 IP 或已进行了内网穿透，另外，暂未对视频发送进行适配，未来存在适配计划。

== c 方案

### c 方案优点

适用 docker 环境，只需要一个中间件程序；无需改动 `sealdice-core` 与 `OneBot V11` 协议端程序。

### c 方案缺点

需要 `OneBot V11` 协议端所在机器具备公网 IP 或已进行了内网穿透；需要具备 docker 与 Linux 操作经验。非 docker 环境需先配置 docker 再使用。

:::

## 获取文件

:::tabs keys:case

== a+b 方案

> 从 [actions 构建](https://github.com/Sealdice/middleware-for-sealdice/actions) 下载对应平台的二进制文件压缩包，在适合的工作目录解压。

== c 方案

按如下操作即可获取 c 方案的文件：

```bash
git clone https://github.com/kenichiLyon/middleware-for-sealdice
cd middleware-c
mkdir -pv docker-data/middleware-c
cp middleware-c/config.json.example docker-data/middleware-c/config.json
vim docker-data/middleware-c/config.json
```

:::

## 配置对应配置文件

::: warning 注意：

这里所有的 `JSON` 文件均视作标准 JSON 格式，不支持注释，请不要直接复制粘贴示例，请按照 Standard JSON 格式自行修改。

:::

::: tabs keys:case

== a+b 方案

`middleware-b` 的 `config.json`：

``` json
{
  "listen_http": ":8081", # sealdice-core 与 middleware-a 进行连接的端口
  "listen_ws_path": "/ws", # 无需修改
  "upstream_ws_url": "ws://127.0.0.1:6700", # 在 Onebot V11 协议端配置的正向 WebSocket 地址
  "upstream_access_token": "<your-access-token>", # 在 Onebot V11 协议端配置的 access-token
  "upstream_use_query_token": true, # 是否使用 access-token 验证
  "server_access_token": "<your-access-token>", # 在 sealdice-core 配置的 access-token
  "upload_endpoint": "http://127.0.0.1:8082/upload"
}
```

`middleware-b` 的 `config.json`：

``` json
{
  "listen_http": ":8082", # middleware-b 与 middleware-a 连接的端口
  "storage_dir": "<your-storage-dir>", # 用于存储上传文件的目录
  "public_base_url": "http://127.0.0.1:8082" # 用于 middleware-b 与 middleware-a 进行连接的 URL
}
```

== c 方案

进入`docker-data/middleware-c`目录内，修改`config.json`文件，示例如下

```json
{
  "listen_http": ":8081", #无需修改，用于海豹进行连接
  "listen_ws_path": "/ws", #无需修改，用于海豹进行连接
  "upstream_ws_url": "ws://127.0.0.1:6700", # 在上游Onebot V11 配置的正向 ws 服务器地址
  "upstream_access_token": "<your-access-token>", # 在上游Onebot V11 正向 ws 服务器配置的 access-token
  "upstream_use_query_token": true, # 上游Onebot V11 是否使用 access-token 验证
  "server_access_token": "<your-access-token>", # 在 sealdice-core 配置的 access-token
  "upload_endpoint": ""
}
```

:::

## 部署中间件

::: tabs keys:case

== a+b 方案

=== Linux

- 如果`middleware-b`在 Linux 环境中，那么在 `middleware-b` 所在目录内，执行以下命令启动 `middleware-b`：

```bash
./middleware-b
```

- 如果`middleware-a`在 Linux 环境中，那么在 `middleware-a` 所在目录内，执行以下命令启动 `middleware-a`：

```bash
./middleware-a
```

启动失败，则按照`FAQ`先进行排查，有任何故障请及时前往 issue 区进行反馈。

=== Windows

- 如果`middleware-a`在 Windows 环境中，直接双击启动 `middleware-a.exe`即可

- 如果`middleware-b`在 Windows 环境中，直接双击启动 `middleware-b.exe`即可

启动失败，则按照`FAQ`先进行排查，有任何故障请及时前往 issue 区进行反馈。

== c 方案

启动海豹：

```bash
docker compose up -d
```

打开本机 3211 端口（sealdice），打开 `sealdice-core` 的 `WebUI`,在账号设置内选择 `Onebot V11 正向 WS`，填入 `middleware-c` 的监听地址 `ws://<middleware-c-host>:8081/ws`，账号填写正确账号即可，`access-token` 处可为空（与`server_access_token` 相同），如果出现已连接，那么，证明其已连接成功。

:::

程序启动后，先配置 `Onebot V11协议端`，配置成正向 WebSocket 服务器，监听 IP 建议为 `0.0.0.0`或 公网 IP，监听端口则按配置文件内地址定，默认为 `6700`,随后在配置文件内填写的 access-token，协议端提示连接成功即为协议端连上`middleware-a`

::: warning 注意：如监听 IP 设置为 0.0.0.0，请确保已经配置了 access-token 以保证安全

一般我们不推荐 Onebot V11 协议端监听 IP 设为 0.0.0.0 而不设置 `access-token`，这样的设置存在安全风险，可能造成无法挽回的后果，如有可能，请设置 `access-token` 以保障安全。

我们已告知了你，不建议设置为 0.0.0.0 而不配置`access-token`，所以，若你确实存在此需求，请自行承担风险。

:::

配置好协议端并连接成功之后，打开 `sealdice-core` 的 `WebUI`,在账号设置内选择 `Onebot V11 正向 WS`，填入 `middleware-a` 的监听地址（默认 `ws://127.0.0.1:8081/ws`）以及在配置文件内填写的 access-token，之后点击下一步进行连接，如出现 `已连接` 字样且的确未显示异常，有正常的信息通信，则证明连接成功。

::: warning 注意：

由于个人能力有限，暂时未对两方案进行日志与错误提示的适配，已有计划适配日志与错误提示，目前建议用户部署完成 (连接成功) 后，在对接了 `sealdice-core` 的账号内尝试测试图片发送、语音发送等功能是否正常使用，如出现异常，请先按照文档`faq`处进行排查，如无法解决，请在本项目仓库的 issue 区及时反馈。

:::
