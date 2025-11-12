---
outline: deep
---

# 快速开始

<Steps :steps="[
  { title: '准备环境', description: '安装 Go 或 Docker；准备 SeaDice-core' },
  { title: '选择方案', description: 'a+b 组件或 c 组件（Docker/本地）' },
  { title: '配置连接', description: '填写 upstream/server token 与端点' },
  { title: '启动与验证', description: '运行组件并在 SeaDice-core 端验证' }
]" :activeIndex="0" />

## 环境要求
- Go 1.20+（建议 1.25+）
- Docker 24+（如使用 c 组件的 Compose 部署）
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

关键配置（示例键位）：
- `listen_http`: `:8081`（a）/`:8082`（b）
- `listen_ws_path`: `/ws`
- `upstream_ws_url`: `ws://127.0.0.1:6700`（上游 OneBot V11）
- `upstream_access_token`: `<token>`
- `upstream_use_query_token`: `true|false`
- `server_access_token`: `<token>`（SeaDice → 中间件入站鉴权）
- `upload_endpoint`: `http://127.0.0.1:8082/upload`（a 指向 b 的上传接口）

## c 方案（Docker Compose）
准备配置：复制示例到 Compose 数据目录并按需修改键位。

```powershell
Copy-Item f:\middleware-for-sealdice\middleware-c\middleware-c\config.json.example `
          f:\middleware-for-sealdice\middleware-c\docker-data\middleware-c\config.json
```

启动：

```powershell
cd f:\middleware-for-sealdice\middleware-c
docker compose up -d
```

SeaDice-core 客户端地址：

```text
ws://middleware-c:8081/ws
```

## c 方案（本地运行）
```powershell
cd f:\middleware-for-sealdice\middleware-c\middleware-c
go run . -config config.json
# 或
go build -o middleware-c.exe
./middleware-c.exe -config config.json
```

## 验证
- 查看组件日志，确认与 OneBot V11 的连接与事件转发正常
- 在 SeaDice-core 中测试消息收发，确认跨机链路可用
