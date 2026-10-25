# SolidAPI 接入教程

访问入口：`/tutorial/`，常见问题直达：`/tutorial/#troubleshooting`。

## 内容与来源

本目录基于 https://help.supeai.cc/ 于 2026-10-26 抓取的教程页面适配，保留 Codex、Claude Code、CC Switch、Windows/macOS 配置步骤与常见问题。HTML、CSS、JavaScript、图标和六张截图均保存在本地，运行时不依赖来源站点。截图用于说明操作位置，实际界面以 SolidAPI 控制台为准。

页头品牌使用与官网一致、含英文名称的 `frontend/public/brand/solidapi-lockup-light.png`，favicon 使用 `frontend/public/brand/solidapi-mark.png`；主色与项目 Tailwind 配置一致：`#1677ff`、`#0958d9`、`#e6f4ff`。

## 配置与部署

- `site.config.js`：品牌、API 地址、密钥页、官网、完整文档、交流群和官方下载地址。
- 与主站同域部署时，默认使用当前域名，并读取 `/api/v1/settings/public` 的 `api_base_url`；Codex 使用 `/v1` 地址，Claude Code 使用根地址。
- 单独部署帮助域名时，复制整个目录与 `/brand/solidapi-mark.png`、`/brand/solidapi-lockup-light.png`，将 `site.config.js` 的 `apiBaseUrl`、`keysUrl`、`homeUrl`、`docsUrl` 改为主站完整地址，并设置 `usePublicSettings: false`。
- Vite 会将 `public/tutorial` 原样复制进前端构建目录，由现有后端静态服务发布，不需要额外服务或新依赖。
- 已有 `/docs` 文档继续保留；首页默认文档入口和控制台教程按钮进入本教程，首页仍优先使用管理员设置的 `doc_url`。

## 密钥处理

手动导入表单仅在点击导入时通过 `ccswitch://` 协议交给本机 CC Switch。页面不保存密钥、不把密钥放进网页 URL，也不向网络提交密钥。公共配置请求不携带登录凭据。复制的环境变量命令中只包含密钥占位文字。
