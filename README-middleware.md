# 一个关于 sealdice-core 与 Onebot 协议端跨机部署的中间件尝试，暂时不可用

## 原始实现路径想法感谢： @SzzRain（<https://github.com/Szzrain）>

## 设想

### middleware-a: 充当 WebSocket 代理，拦截 OneBot 协议端的 `upload_private_file/upload_group_file` ，将本地文件上传至 `middleware-b` ，并把动作改写为 `send_*_msg 携带 [CQ:file] 的远端 URL`

### middleware-b: 充当文件存储服务，接收 `middleware-a` 上传的文件，返回可供协议端进行拉取的文件 URL
