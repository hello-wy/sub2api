# Prism OAuth 浏览器适配器

这个 sidecar 通过受限的 Chromium 浏览器访问 `prism.openai.com`，供
Sub2API 的 OpenAI OAuth 账号使用。网关只连接 `127.0.0.1`，并通过独立的
适配器密钥和账号 ID 转发请求；OAuth token 不写入配置文件或项目目录。

## 安装

在运行主服务的机器上准备 Python 3.12、匹配版本的 Chromium 和 sandbox helper，
然后在本目录安装固定的二进制 wheel：

```sh
python3 -m venv /opt/sub2api/prism-adapter/venv
/opt/sub2api/prism-adapter/venv/bin/pip install --only-binary=:all: -r requirements.txt
```

适配器必须以非 root 用户启动，并设置以下受保护的环境变量（文件权限 `0600`）：

```dotenv
PRISM_ADAPTER_API_KEY=<至少 32 个字符的随机值>
GATEWAY_PRISM_BROWSER_API_KEY=<与上面相同>
PRISM_ADAPTER_CHROME=/opt/chromium/chrome
CHROME_DEVEL_SANDBOX=/opt/chromium/chrome-sandbox
PRISM_ADAPTER_STATE_DIR=/var/lib/sub2api-prism
```

在管理员「系统设置 → 功能开关 → Prism 浏览器桥」中开启全局开关，填写
`http://127.0.0.1:8319/v1` 和与 `PRISM_ADAPTER_API_KEY` 相同的密钥并保存。
随后在 OpenAI OAuth 账号中勾选 Prism 浏览器协议。两个开关均开启才会使用 Prism，
保存立即生效，无需重启主服务。密钥输入留空保留现有值，管理接口不返回密钥。

升级后全局开关默认关闭，需要在系统设置中重新开启。
`gateway.prism_browser.base_url/api_key`（对应 `GATEWAY_PRISM_BROWSER_BASE_URL/API_KEY`）
只提供未落库地址和密钥的初始值；已保存的系统设置优先。旧部署的
`GATEWAY_PRISM_BROWSER_ENABLED` 不再控制路由。
安装 `sub2api-prism-adapter.service` 后，先访问 `/health` 确认进程可用，
再从管理员账号测试入口验证真实 OAuth 会话。

适配器目前支持 `gpt-6.1-sol`、`gpt-5.6-sol`、`gpt-5.6-terra` 和 `gpt-6-luna`
的文本 Responses 请求（推理强度为 `medium`），成功响应的 `usage` 不可用且不会估算 token；
如果浏览器请求的终态不明确，pending 文件会保留并拒绝自动重放，需人工确认后处理。
