# Excel BPS 接入与开发验证

更新日期：2026-09-26。

## 转发入口

启用账号的 Excel BPS 后，以下入口统一复用同一个 BPS 转发实现：

| 入口 | 入站协议 | BPS 返回适配 |
| --- | --- | --- |
| /v1/responses | Responses | Responses JSON / SSE |
| /v1/chat/completions | Chat Completions；兼容 Responses 形状 | Chat Completions JSON / SSE |
| /v1/messages | Anthropic Messages | Messages JSON / SSE |
| 分组智能测试 | 进程内 /v1/chat/completions | 与正常分组网关请求一致 |

Messages 仍遵守分组“允许 /v1/messages 调度”开关。审计时两套环境的 OpenAI 分组都关闭此开关，因此当前分组的真实 Messages 请求应返回 403；本次保持该访问策略，兼容转换由离线回归覆盖。

实际上游为 https://bps.openai.com/basispoints/api/responses；响应头 X-Codex2API-Upstream: basispoints 和用量日志 upstream_endpoint=/basispoints/api/responses 可用于确认。不能仅依据账号 BPS 开关、客户端使用的 URL 或模型名称判断实际协议。

BPS 适用范围在账号/分组模型映射之后判定，模型只映射一次。未选中的模型仍使用原有协议；显式空列表不启用任何模型。请求包含必须由原生 Codex 执行的能力时继续遵循 basispoints.NativeFallbackReason，并返回 X-Codex2API-Basispoints-Bypass 原因。

## 一致性约定

- 三种入口使用相同的 BPS Prepare、工具目录封装、工具回放、图片中转、403/429 处理及用量计量链路。
- 兼容接口只转换返回协议；原始上游用量用于本地计费，“缓存创建计入输入”仅改变下游展示。
- 推理等级遵循客户端请求和分组策略，BPS 按原有规则规范化（例如 max -> xhigh）。不会把所有请求强制改为 high；对照评测须使用相同题目、模型、等级和账号。
- 工具调用仅在 BPS 桥接层验证完成后返回；后续工具结果按同一线程/会话作用域回放。
- 缺少终态、失败或 incomplete 返回错误，不能伪装为成功的 stop/message_stop；不自动重放失败的 BPS 请求。
- 本次没有改写参考仓库的 BPS 核心；与 /Users/kuhne/code/sub2api-ranxi2001 对照的 basispoints 目录 37 个文件保持一致。新增的是本项目兼容入口的接线与回归覆盖。

## 本地开发

默认开发后端为 http://127.0.0.1:8083，前端为 http://localhost:3000。

启动方式：

~~~bash
./backend/scripts/run-local.sh
cd frontend
pnpm dev
~~~

启动脚本只为开发后端设置 SERVER_HOST=127.0.0.1、SERVER_PORT=8083；生产配置不受影响。已有环境变量优先。如果覆盖开发后端端口，请同步设置前端 VITE_DEV_PROXY_TARGET。

避免使用含糊的 localhost:8080：在审计时，本机 127.0.0.1:8080 为另一个 Docker API 代理，而 [::1]:8080 为当前开发后端。

dev/build/preview 脚本明确使用 --config vite.config.ts。本机留有旧的、未跟踪的 vite.config.js，Vite 默认发现机制会优先加载它并覆盖修改后的 TypeScript 配置；显式配置路径可以避免此问题再次出现。

## 图片中转地址

图片中转 Base URL 是 **BPS 远端读取图片使用的地址**，不是浏览器访问本地后端的地址。

- 开发环境可保留中转关闭、Base URL 留空；文本、工具请求以及上游可访问的 HTTPS 图片链接仍可使用 BPS。
- localhost、127.0.0.1 和私网地址不能用于远端访问本机图片。需要调试本地上传图片时，先建立通向当前开发后端的公网 HTTPS 隧道，再把该 HTTPS origin 填入中转配置。
- 生产环境填写实际反向代理到本实例的公网 HTTPS origin；/api/bps-images/ 路由必须转发到存储该临时图片的实例。不要直接借用另一实例的域名。
- 中转关闭时，base64 图片明确报错，不会悄悄切到另一协议或把请求重复发送。

## 回归命令

~~~bash
cd backend
go test ./internal/service/basispoints -count=1
go test ./internal/service -run 'TestExcelBPSCompat|TestExcelBPSGroupPelican|TestExcelBPSForwardContract' -count=1
go test -tags=unit ./internal/service ./internal/handler ./internal/server/middleware ./internal/server/routes ./internal/repository ./internal/pkg/apicompat -count=1
~~~

回归覆盖入口与流式/非流式组合、推理等级、模型映射与模型范围、缓存用量、错误终态、终态文本补齐、工具多轮续接、取消、结构化输出、图片中转及分组测试。

## 本次验证结果

- BPS 核心测试通过；service、handler、middleware、routes、repository、apicompat 六个相关包的完整 unit 测试通过。最终边界修复后，定向回归再次通过。
- 成功回包的 error:null 不再被当成错误响应，非流式 Chat Completions/Messages 都会执行正确的格式转换。
- 前端类型检查、国际化键检查及生产构建均通过；本地后端已切换为包含当前工作区改动的新二进制，监听 127.0.0.1:8083。前端已重启并验证实际请求到达新后端。
- 使用当前分组 2、gpt-6-astra、low 等级做真实上游验证：Responses 非流式、Chat Completions 非流式与流式均返回 HTTP 200、BPS_OK 和 basispoints 路由标识，单次约 3–4 秒。
- Chat Completions 流式输出包含正确的 stop 与 [DONE]；非流式返回 choices/message 和 Chat 用量字段，未泄漏 Responses 结构。
- 对应真实验证的用量记录 1150–1152 均落到账号 11，upstream_endpoint 均为 /basispoints/api/responses；确认协议标识与持久化计量一致。
- Messages 真实请求按两仓库一致的分组权限返回预期 403。图片中转按开发环境要求保持关闭、地址留空。

这些验证确认协议、入口和运行实例一致，不等同于相同复杂题目的生成质量 A/B。原分组计划的 medium 设置未被擅自改为参考历史请求常用的 high/xhigh；比较输出质量时应统一等级。
