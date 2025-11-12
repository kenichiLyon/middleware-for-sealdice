---
outline: deep
---

# 常见问题

## Linux 环境下启动失败了怎么办？

> 首先自查是否按照[安装与部署](/guide/install&deploy)的步骤进行了操作，其次，自查是否给予了足够的权限（如执行权限），使用 `chmod +x <middleware工作目录>`为 middleware 组件添加执行权限。

## Windows 环境下启动失败了怎么办？

> 首先自查是否按照[安装与部署](/guide/install&deploy)的步骤进行了操作，其次，自查是否存在 WindowsDefender 等安全软件，若存在，请将 middleware 组件所在目录添加到安全软件的白名单中。

## 未来会增加其他协议的支持吗？

> 目前，我们仅支持 Onebot V11 协议，未来会根据实际需求，考虑是否增加其他协议的支持。
