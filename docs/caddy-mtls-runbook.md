# Caddy mTLS 部署运行手册

> 最后更新：2026-08-02
>
> 当前阶段：准备执行第 6 步——离线生成客户端 CA
>
> 云端续接：其他终端拉取本仓库后，阅读本文并从第一个未完成的检查项继续。

## 1. 目标与约束

目标是在腾讯云轻量应用服务器上，用 Caddy 为现有 Go API 增加高安全、轻量级的设备鉴权：

- Caddy 作为唯一公网入口，负责 HTTPS 和 mTLS。
- 服务端证书由 Caddy 通过 Let's Encrypt 自动签发和续期。
- 客户端证书由自建离线 CA 签发，每台 Windows/macOS 设备一个独立证书。
- 客户端证书有效期暂定 730 天；CA 有效期暂定 3650 天。
- Caddy 同时验证客户端 CA 和允许的叶证书白名单，以便单独停用丢失设备。
- 尽量不修改前端、后端业务代码，不增加业务数据库表。
- 客户端不安装常驻代理或 VPN，只在系统证书库安装一个 P12 客户端证书。
- Go 服务继续监听 31168，由 Caddy 在本机反向代理；最终关闭 31168 的公网访问。

当前选择会让 `www.ledongtennis.cn` 整个主机名都要求客户端证书，包括 `/health` 和 `/swagger`。如果以后要让前端页面公开访问而只保护 API，应新增 `api.ledongtennis.cn`，并只在 API 子域启用 mTLS。

## 2. 当前环境

| 项目 | 当前值 |
|---|---|
| 云服务 | 腾讯云轻量应用服务器 |
| 操作系统 | Ubuntu |
| 公网 IP | `124.221.103.110` |
| 域名 | `www.ledongtennis.cn` |
| Go 服务 | `127.0.0.1:31168`（Caddy 上游） |
| 健康检查 | `GET /health` |
| Swagger UI | `/swagger/index.html` |
| Caddy | `v2.11.4`，systemd 服务 |
| 公网入口 | TCP 80、TCP 443；UDP 443 可选 |
| 当前 mTLS | 尚未启用 |

## 3. 已确认的线上状态

- [x] `www.ledongtennis.cn` 的 A 记录指向 `124.221.103.110`。
- [x] 腾讯云 80/443 入站链路可用。
- [x] Caddy 已安装并由 systemd 管理。
- [x] Caddy 已反向代理到 `127.0.0.1:31168`。
- [x] `http://www.ledongtennis.cn/` 返回 308，并跳转到 HTTPS。
- [x] `https://www.ledongtennis.cn/health` 返回 HTTP 200 和 `{"status":"ok"}`。
- [x] `https://www.ledongtennis.cn/swagger/index.html` 可通过 GET 打开。
- [x] Let's Encrypt 服务端证书签发成功。
- [x] 服务端证书域名 SAN 为 `www.ledongtennis.cn`，证书链校验通过，使用 TLS 1.3。
- [x] 当前服务端证书有效期为 2026-08-02 至 2026-10-31，后续由 Caddy 自动续期。
- [ ] 生成离线客户端 CA。
- [ ] 为第一台设备签发独立客户端证书。
- [ ] 在 Caddy 中启用 mTLS 和叶证书白名单。
- [ ] 验证无证书拒绝、有证书放行。
- [ ] 关闭 31168 公网入口。

当前不用客户端证书仍可访问 `/health`，这是预期状态，证明目前只完成了普通 HTTPS，尚未启用 mTLS。

## 4. 当前 Caddy 基础配置

服务器文件：`/etc/caddy/Caddyfile`

```caddyfile
{
	email <真实联系邮箱>
}

www.ledongtennis.cn {
	reverse_proxy 127.0.0.1:31168
}
```

注意：全局配置块 `{ ... }` 必须位于文件最前面。配置修改后必须先验证，再重新加载：

```bash
sudo caddy fmt --overwrite /etc/caddy/Caddyfile
sudo caddy validate --config /etc/caddy/Caddyfile
sudo systemctl reload caddy
```

只有看到 `Valid configuration` 才能执行 `reload`。不要在配置无效时执行 `restart`。

## 5. 绝不能提交或上传的内容

以下内容禁止进入 Git、GitHub、聊天附件、普通网盘或腾讯云服务器：

- `client-ca.key`：客户端根 CA 私钥。
- `client-*.key`：每台客户端的私钥。
- `client-*.p12`：包含客户端私钥的安装包。
- P12 密码、CA 私钥密码。
- `/etc/caddy/.env` 的真实内容。
- 腾讯云 SecretId、SecretKey、数据库密码。

必须在代码仓库之外创建证书目录。例如：

```text
Windows: C:\secure\ledong-mtls-ca
Linux/WSL: ~/secure/ledong-mtls-ca
macOS: ~/secure/ledong-mtls-ca
```

本仓库只同步这份不含秘密的运行手册。

## 6. 当前执行步骤：生成离线客户端 CA

### 6.1 执行位置

在一台受控管理电脑上执行，不要在腾讯云服务器上执行。推荐使用：

- Windows：WSL Ubuntu 或安装了 OpenSSL 3 的 Git Bash。
- macOS：Homebrew 安装的 OpenSSL 3。
- Linux：发行版提供的 OpenSSL 3。

先检查版本：

```bash
openssl version
```

建议使用 OpenSSL 3.x。以下命令均在 Bash、WSL 或 Linux/macOS 终端运行。

### 6.2 创建仓库外安全目录

```bash
mkdir -p ~/secure/ledong-mtls-ca
cd ~/secure/ledong-mtls-ca
umask 077
```

确认当前目录不是 `ledong-db` Git 仓库的子目录：

```bash
pwd
git rev-parse --is-inside-work-tree 2>/dev/null || true
```

预期第二条命令不应输出 `true`。

### 6.3 生成加密的 P-256 CA 私钥

```bash
openssl genpkey \
  -algorithm EC \
  -pkeyopt ec_paramgen_curve:P-256 \
  -aes-256-cbc \
  -out client-ca.key
```

命令会要求输入 CA 私钥密码。要求：

- 使用独立强密码，不复用服务器、GitHub或腾讯云密码。
- 密码离线保存到可靠密码管理器。
- 不要把密码写进本文或 Shell 历史。

检查私钥，命令会再次询问密码：

```bash
openssl pkey -in client-ca.key -check -noout
```

成功时应看到类似：

```text
Key is valid
```

### 6.4 生成有效期 10 年的客户端根 CA 证书

```bash
openssl req -x509 -new -sha256 \
  -key client-ca.key \
  -days 3650 \
  -out client-ca.crt \
  -subj "/CN=Ledong Tennis mTLS Client CA" \
  -addext "basicConstraints=critical,CA:TRUE,pathlen:0" \
  -addext "keyUsage=critical,keyCertSign,cRLSign"
```

这里的 CA 只用于签发客户端证书。Caddy 的公网服务端证书继续由 Let's Encrypt 管理。

### 6.5 验证 CA 证书

```bash
openssl x509 \
  -in client-ca.crt \
  -noout \
  -subject \
  -issuer \
  -serial \
  -dates
```

检查约束和密钥用途：

```bash
openssl x509 \
  -in client-ca.crt \
  -noout \
  -text | grep -A4 -E "Basic Constraints|Key Usage"
```

需要确认：

```text
CA:TRUE
Certificate Sign
CRL Sign
```

验证自签名链：

```bash
openssl verify -CAfile client-ca.crt client-ca.crt
```

预期输出：

```text
client-ca.crt: OK
```

### 6.6 完成检查项

- [ ] OpenSSL 版本已确认。
- [ ] CA 在 Git 仓库之外生成。
- [ ] `client-ca.key` 使用 AES-256 加密。
- [ ] CA 私钥密码已存入可靠的密码管理器。
- [ ] `client-ca.crt` 显示 `CA:TRUE`。
- [ ] `openssl verify` 返回 `client-ca.crt: OK`。
- [ ] CA 私钥和密码至少有两份加密离线备份。
- [ ] 未把 `client-ca.key` 上传到服务器、GitHub或聊天。

第 6 步完成后，本地安全目录中应至少存在：

```text
client-ca.key   # 最高敏感，永久离线保存
client-ca.crt   # 公钥证书，后续允许上传 Caddy
```

## 7. 后续步骤概览

### 第 7 步：签发第一台客户端证书

- 为设备定义唯一名称，例如 `client-win-001`。
- 生成独立 P-256 私钥和 CSR。
- 使用 `extendedKeyUsage=clientAuth`。
- 使用离线 CA 签发 730 天客户端证书。
- 验证证书链、有效期、序列号和客户端用途。

### 第 8 步：导出客户端 P12

- 将设备私钥和设备证书打包为带密码的 P12。
- P12 与密码通过不同安全渠道交付。
- Windows 导入当前用户“个人”证书库，禁止标记为可导出。
- macOS 导入“登录”钥匙串。

### 第 9 步：上传非秘密证书

只上传：

```text
client-ca.crt
client-win-001.crt
```

服务器目标：

```text
/etc/caddy/pki/client-ca.crt
/etc/caddy/pki/allowed/client-win-001.pem
```

### 第 10 步：配置后端兼容请求头

- 通过 `/etc/caddy/.env` 设置 `BACKEND_SECURE`。
- 当前兼容值需与后端现有校验值一致。
- Caddy 删除外部 `secure` 头，再注入内部值。
- `.env` 权限设为 `root:root 0600`，不得提交 Git。

### 第 11 步：启用 Caddy mTLS

目标配置核心：

```caddyfile
tls {
	client_auth {
		mode require_and_verify
		trust_pool file /etc/caddy/pki/client-ca.crt

		verifier leaf {
			folder /etc/caddy/pki/allowed
		}
	}
}
```

叶证书白名单用于单独停用某台设备。移出 `allowed` 目录并重新加载 Caddy 后，该设备即使证书未到期也无法访问。

### 第 12 步：验证并加载配置

- `caddy fmt`
- `caddy validate`
- `systemctl reload caddy`
- 检查 `systemctl status caddy` 和 `journalctl -u caddy`

### 第 13 步：验收 mTLS

- 不提供客户端证书：TLS 握手必须失败。
- 提供白名单客户端证书：`/health` 必须返回 200。
- 非白名单客户端证书：必须失败。
- Swagger 页面必须只能由授权设备打开。

### 第 14 步：关闭后端公网端口

- 在腾讯云轻量服务器防火墙中删除或禁用 TCP 31168 放行规则。
- 如 UFW 已启用，拒绝公网访问 31168。
- 服务器本机 `curl http://127.0.0.1:31168/health` 仍应正常。
- 公网 `http://124.221.103.110:31168/health` 应连接失败。

## 8. 已知问题与后续改进

1. Swagger 生成信息当前把 API Host 写为 `localhost:31168`。Swagger UI 可以打开，但远程点击 “Try it out” 可能请求客户电脑自己的 localhost。后续应删除固定 Host 或改为 `www.ledongtennis.cn` 并使用 HTTPS。
2. 当前后端把 `secure` 与腾讯云 SecretId 比较。Caddy可以在不改代码的情况下兼容，但长期应改为独立业务随机密钥，避免复用云身份凭据。
3. 普通 P12 属于设备凭证，但可以被有足够本机权限的人复制。更严格的不可导出绑定可在后续引入 Windows TPM 生成密钥。
4. 如果 `www.ledongtennis.cn` 未来承载公开前端，应把 mTLS API 迁移到独立子域 `api.ledongtennis.cn`。

## 9. 故障恢复

修改 Caddy 前先备份：

```bash
sudo cp /etc/caddy/Caddyfile /etc/caddy/Caddyfile.before-mtls
```

如果新配置验证失败，不执行 reload。如果 reload 后业务异常，恢复上一个有效配置：

```bash
sudo cp /etc/caddy/Caddyfile.before-mtls /etc/caddy/Caddyfile
sudo caddy validate --config /etc/caddy/Caddyfile
sudo systemctl reload caddy
```

配置失败的 reload 通常不会中断当前正在运行的有效配置。

## 10. 其他终端续接方式

在另一台电脑登录有权访问仓库的 GitHub 账号，然后：

```bash
git clone git@github.com:niyaou/ledong-db.git
cd ledong-db
git switch codex/structured-service-logging
git pull
```

如果已经克隆：

```bash
git fetch origin
git switch codex/structured-service-logging
git pull
```

在新 Codex 任务中使用以下提示：

```text
请完整阅读 docs/caddy-mtls-runbook.md，核对“当前环境”和“已确认的线上状态”，然后从第一个未完成的检查项继续。不要要求或上传任何 CA 私钥、客户端私钥、P12、腾讯云凭据或数据库密码。
```

每完成一个步骤，应更新本文中的状态和检查项，再提交并推送到远端，使其他终端获得最新进度。

## 11. 官方参考

- [Caddy TLS 和客户端证书认证](https://caddyserver.com/docs/caddyfile/directives/tls)
- [Caddy Automatic HTTPS](https://caddyserver.com/docs/automatic-https)
- [Caddy systemd 运行方式](https://caddyserver.com/docs/running)
- [腾讯云轻量应用服务器防火墙](https://cloud.tencent.com/document/product/1207/44577)
- [OpenSSL X.509 扩展配置](https://docs.openssl.org/4.0/man5/x509v3_config/)
