---
outline: deep
---

# 配置说明

> 本节描述跨机部署 OneBot V11 时的关键配置项与约定。具体键名与示例请依据项目实现补充。

## 连接配置
- 目标：连接到 OneBot V11 端点并与 Sealdice-core 协同
- 建议：使用环境变量或配置文件管理敏感信息

## 日志与监控
- 日志级别：`info` / `warn` / `error`
- 建议：在生产环境开启基础监控（CPU/内存/网络）

## 安全
- 不在日志中输出密钥或令牌
- 仅在受信网络中开放服务端口

## 示例（占位）
```json
{
  "onebotEndpoint": "ws://example:6700",
  "token": "<secret>",
  "sealdiceCore": {
    "host": "http://core-host",
    "port": 8080
  }
}
```

