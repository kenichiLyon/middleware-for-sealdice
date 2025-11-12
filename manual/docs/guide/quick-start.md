---
outline: deep
---

# 快速开始

1. 准备环境：下载组件 或 Docker；准备 SealDice-core  
2. 选择方案：a+b 组件或 c 组件（本地/Docker）  
3. 配置连接：填写 upstream/server token 与端点  
4. 启动与验证：运行组件并在 SeaDice-core 端验证

## 环境要求

- 如使用 a+b 组件，本地运行需要满足 Go 1.25 的最低系统运行要求
- 如使用 c 组件，需要满足 Docker 24+-
- 可用的 SeaDice-core 实例

## 选择方案

- a+b 组件：`middleware-a` 负责 WS 转发与入站鉴权，`middleware-b` 提供文件上传与 URL 生成
- c 组件：单体实现，支持 Docker Compose，上传走 `base64://` 内联（无需独立上传端点）

## a+b 方案（Windows）

```powershell
cd f:\middleware-for-sealdice\middleware-b
./middleware-b -config config.json

cd f:\middleware-for-sealdice\middleware-a
./middleware-a -config config.json
```

在 SeaDice-core 客户端设置 OneBot V11 正向 WS 地址：

```text
ws://<middleware-a-IP>:8081/ws
```

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

## c 方案（Docker Compose）

### 拉取文件

```bash
git clone https://github.com/kenichiLyon/middleware-for-sealdice
cd middleware-c
mkdir -pv docker-data/middleware-c
cp middleware-c/config.json.example docker-data/middleware-c/config.json
vim docker-data/middleware-c/config.json
```

### 配置文件

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

### 启动

```bash
docker compose up -d
```

SeaDice-core 客户端地址：

```text
ws://middleware-c:8081/ws
```

## 验证

- 查看组件日志，确认与 OneBot V11 的连接与事件转发正常
- 在 SeaDice-core 中测试消息收发，确认跨机链路可用
