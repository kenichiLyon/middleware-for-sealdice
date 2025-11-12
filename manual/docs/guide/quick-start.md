---
outline: deep
---

# 快速开始

<Steps :steps="[
  { title: '准备环境', description: '安装 Node.js 与 pnpm' },
  { title: '获取中间件', description: '克隆或下载项目代码' },
  { title: '配置连接', description: '对接 Sealdice-core 与 OneBot V11' },
  { title: '启动与验证', description: '运行并观察日志/事件' }
]" :activeIndex="0" />

## 环境要求
- Node.js 18+
- pnpm 8+

## 安装
```bash
pnpm install
```

## 启动（示例）
> 具体启动命令与参数请参考「配置说明」章节，以下为占位示例。

```bash
# 示例：启动中间件服务
pnpm run start
```

## 验证
- 观察日志是否出现与 OneBot V11 相关的连接/事件
- 在 Sealdice-core 中验证跨机消息是否连通

