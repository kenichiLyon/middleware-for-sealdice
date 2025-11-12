---
outline: deep
---

# 部署

## 构建镜像（示例）
> 若中间件以容器部署，可参考以下占位示例。

```dockerfile
FROM node:18-alpine
WORKDIR /app
COPY . .
RUN pnpm i --frozen-lockfile
CMD ["pnpm","run","start"]
```

## 运行时参数
- 通过环境变量或配置文件注入端点地址、认证信息
- 开启健康检查与重启策略

## 网络与安全
- 仅在内网或受控环境暴露服务端口
- 使用反向代理时注意超时与长连接设置

