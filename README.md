# Middleware for SealDice OneBot 文件跨机发送

## 鸣谢

### Original Idea: [@Szzrain](https://github.com/szzrain)

### Inspired by:[@PaineNate](https://github.com/paiennate)

用于在 `sealdice-core` 与 OneBot 协议实现端位于不同机器时，实现文件发送功能的中间件，无需改动两端代码，用户侧无感传输。

## 获取二进制文件

可以通过 [action 构建](https://github.com/kenichiLyon/middleware-for-sealdice/actions) 获取，注意，**必须下载 middleware-a 和 middleware-b 并且部署才能正常工作**

## 构建

本项目使用 Golang 1.25.3 进行编写，建议以该版本进行代码编写与编译。
本项目仓库为 monorepo，有需要可通过自行 clone 不同目录内的代码进行使用。

## 手册

本项目使用 monorepo 模式，手册位于 `manual` 目录下。

在线手册：https://kenichiLyon.github.io/middleware-for-sealdice/
