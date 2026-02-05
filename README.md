# V2bX

> **This is an actively maintained personal fork after [wyx2685/V2bX](https://github.com/wyx2685/V2bX) stopped updates.**
>
> **这是 wyx2685/V2bX 停止更新后的个人维护活跃版本。**

A V2board node server based on multi core, modified from XrayR.
一个基于多种内核的V2board节点服务端，修改自XrayR，支持V2ay,Trojan,Shadowsocks协议。

## 特点

* 永久开源且免费。
* 支持Vmess/Vless, Trojan， Shadowsocks, Hysteria1/2多种协议。
* 支持Vless和XTLS等新特性。
* 支持单实例对接多节点，无需重复启动。
* 支持限制在线IP。
* 支持限制Tcp连接数。
* 支持节点端口级别、用户级别限速。
* 配置简单明了。
* 修改配置自动重启实例。
* 支持多种内核，易扩展。

## 功能介绍

| 功能        | v2ray | trojan | shadowsocks | hysteria1/2 |
|-----------|-------|--------|-------------|----------|
| 自动申请tls证书 | √     | √      | √           | √        |
| 自动续签tls证书 | √     | √      | √           | √        |
| 在线人数统计    | √     | √      | √           | √        |
| 审计规则      | √     | √      | √           | √         |
| 自定义DNS    | √     | √      | √           | √        |
| 在线IP数限制   | √     | √      | √           | √        |
| 连接数限制     | √     | √      | √           | √         |
| 跨节点IP数限制  |√      |√       |√            |√          |
| 按照用户限速    | √     | √      | √           | √         |
| 动态限速(未测试) | √     | √      | √           | √         |

## 软件安装

### 一键安装

```bash
bash <(curl -Ls https://raw.githubusercontent.com/yuwan027/V2bX/dev_new/install.sh)
```

### 手动安装

下载对应架构的二进制文件：
- [V2bX-linux-amd64](https://github.com/yuwan027/V2bX/releases/latest/download/V2bX-linux-amd64)
- [V2bX-linux-arm64](https://github.com/yuwan027/V2bX/releases/latest/download/V2bX-linux-arm64)

## 构建

```bash
# 编译 (需要 Go 1.25+)
./build.sh

# 或手动编译
GOEXPERIMENT=jsonv2 CGO_ENABLED=0 go build -v -trimpath \
  -tags "sing xray with_quic with_grpc with_utls with_wireguard with_acme with_gvisor" \
  -ldflags "-s -w -buildid=" \
  -o V2bX .
```

## 配置文件

参考 [V2bX 文档](https://v2bx.v-50.me/)

## 免责声明

* 此项目用于个人使用，不保证向后兼容性。
* 本人不对任何人使用本项目造成的任何后果承担责任。

## Thanks

* [wyx2685/V2bX](https://github.com/wyx2685/V2bX) - 原项目
* [Project X](https://github.com/XTLS/)
* [V2Fly](https://github.com/v2fly)
* [XrayR](https://github.com/XrayR/XrayR)
* [sing-box](https://github.com/SagerNet/sing-box)
