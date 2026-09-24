# Excel / BPS 协议（OpenAI OAuth）

在账号管理 → 编辑现有 OpenAI OAuth 账号 → 打开“Excel / BPS 协议”并保存。使用该账号已有的 ChatGPT access token/account ID，不需要 GitHub 登录、sidecar 或新建 API Key 上游账号。原有凭据刷新逻辑继续生效。默认关闭；切换后新开 Codex 会话。

本入口面向 HTTP `/v1/responses` 和 `/v1/responses/compact`。强制上游 HTTP/SSE，优先于账号的自动透传、WS mode 和 Codex ticket 注入。保持原始模型名或显式账号映射，不因模型权限不足偷偷切换模型。现有调度、分组授权和并发额度继续生效；开关不会重新启用已停用的账号。

## 工具调用与并发

- 客户端 tools 转成 developer 消息中的目录；只解析上游 `run_officejs.code` 内的 JSON，不在服务器执行 OfficeJS 或客户端命令。
- 支持 function、custom、namespace、Lite additional_tools，保留 custom 原始文本与 JSON 大整数。
- 文本增量实时转发；工具等完整 response.completed 到齐后统一验证，失败、不完整或未知工具不会被提前发送执行。
- 完整原生 item 按账号、API Key、线程作用域缓存。回放保留上游 ID、summary、references 等内容；多个完整工具和乱序结果按 call_id 配对。
- 子线程优先使用 thread-id / x-codex-turn-metadata，不因共用父会话 session_id 混用缓存；没有设置全账号串行锁。并行子代理仍受账号并发数、调度及上游能力约束。
- 提示词仍要求每个 transport 内只放一个工具对象，响应声明 parallel_tool_calls=false。多工具转换与子线程隔离已有离线回归，不等于真实 Codex 多代理工作流已完整验收。
- 缓存位于当前进程，有条数和内存上限。重启、跨实例、换账号或淘汰后的缺失原始调用会报错，不能恢复任意旧线程。

## 能力限制

`max`/`ultra` 映射 `xhigh`，`none`/`minimal` 映射 `low`，实际 effort 出现在响应/用量中。拒绝强制指定工具、托管工具、结构化输出与仅 previous_response_id 的增量历史。不要把 HTTP 200 当作模型能力证明。

上游仍只接收 HTTPS 图片 URL. 需要发送 base64 图片时, 按下节启用服务器临时图片中转. `file_id` 仍不受支持. 本地转换通过不等于真实上游视觉已验收, 账号权限或模型限制仍可能导致上游拒绝.

## Base64 图片中转

1. 为当前 Sub2API 实例配置公网可访问且证书有效的 HTTPS 域名. 将该域名的 `/api/bps-images/` 路径转发到接收 BPS 请求的同一实例.
2. 设置 `GATEWAY_EXCEL_BPS_IMAGE_BASE_URL=https://your-api.example.com`, 或在配置文件中设置 `gateway.excel_bps_image_base_url`. 只填写 HTTPS origin, 不附加 `/v1`, 查询参数或账号密码. 留空时禁用中转.
3. 使用 Docker Compose 时, 在 `.env` 中设置该变量, 并执行 `docker compose up -d --no-deps sub2api` 重新创建应用容器. 仓库内四种 Compose 配置均透传该变量.
4. 保持账号的 Excel / BPS 协议开关开启. 客户端继续发送 `input_image.image_url=data:image/png;base64,...`, 无需调用额外上传接口.
5. 允许上游免登录 GET/HEAD 临时图片路径, 禁止 CDN 缓存, 并在 Nginx/CDN 访问日志中屏蔽该路径的 token. 应用日志和 BPS 上游错误日志会自动脱敏 token.

- 自动转换用户消息中的图片和 `function_call_output` / `custom_tool_call_output` 中的截图. 原有 HTTPS URL, 相邻文本及工具参数保持原值.
- 仅接收 PNG, JPEG, GIF 和 WebP. 校验 base64, 图片格式, 声明 MIME 和图片尺寸. 每张最多 20 MiB, 每个请求最多 20 张且解码后合计不超过 32 MiB, 单张最多 64 Mi 像素. 网关已有请求体限制仍生效.
- 图片仅保存在当前进程内存, 总容量最多 128 MiB / 512 张. 容量用尽时明确返回 503, 不淘汰仍在有效期内的图片.
- 临时链接按账号, API Key 和线程隔离, 使用进程密钥生成不可猜测的 token. 相同作用域再次提交同图时复用链接并续期, 最后一次提交后 30 分钟自动删除; 读取链接不会续期.
- 链接持有者可在有效期内读取图片. 服务重启后链接失效; 客户端在下一次请求中重发原始 base64 图片即可重新生成. 多实例部署必须将图片下载请求固定到创建链接的实例.
- 服务端不会读取客户端文件路径, 不开放匿名上传接口, 不将原始图片写入数据库或对象存储.

验证范围: 自动化测试覆盖真实 HTTPS 测试服务器取图, BPS 请求转换, 普通/compact 与流式/非流式分支, 工具截图, 并发, 过期, 容量, 非法输入, 日志脱敏和环境变量透传. 真实 BPS 模型视觉结果需要部署后使用具有相应权限的账号验收.

原始协议来自 hloolx/codex2api：9d02d3f5 → c125e560 → 20ff3e86 → d39f7e36 → 4dea83ec（含中间依赖修复）。另外对照 JaxsonWang/cpa-plugin-oai-basispoints 05b2d97 的工具目录、信封和回放实现。出处见 `backend/internal/service/basispoints/NOTICE.md`。

验证区分：账号 300 的 gpt-5.6-sol 直连 BPS 糖果题返回 21；这只是文本上游验证。PR 中完整 Sub2API 转发、工具回放、多个子线程使用 mock 回归，尚未将该改动部署到生产。
