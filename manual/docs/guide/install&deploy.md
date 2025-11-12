---
outline: deep
---

# 安装部署

本项目目前存在两种方案，可根据实际情况选用。

## 选用方案

::: tabs key:case

== a+b 方案

### 优点

无需改动现有的 `sealdice-core`程序，也无需改动 `OnebotV11` 协议端程序，下载好对应中间件后启动即可无感部署

### 缺点

不适用 docker 环境，需要下载`middleware-a`与`middleware-b`两个中间件程序，同时，需要`OnebotV11协议端程序`的机器存在公网 IP / 进行了内网穿透

== C 方案

### 优点 (C 方案)

适用 docker 环境，只需要下载一个中间件程序，同时，也无需改动 `sealdice-core`程序，也无需改动 `OnebotV11` 协议端程序

### 缺点（C 方案）

需要 `OnebotV11协议端程序`的机器存在公网 IP / 进行了内网穿透，同时，需要有 docker 操作经验与 linux 操作经验。非 docker 环境需要先配置 docker 环境再使用。

:::

## 获取文件

::: tabs key:case

== a+b 方案

> 从 [actions 构建](https://github.com/Sealdice/middleware-for-sealdice/actions) 下载对应平台的二进制文件。

== c 方案

c 方案将 dockerfile 文件放在了源码中，直接 clone 源码仓库后按照后续进行操作即可。

:::

## 配置对应配置文件

::: tabs key:case

== a+b 方案

`a+b` 方案需要

`middleware-a`的`config.json`
