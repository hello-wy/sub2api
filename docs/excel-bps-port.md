# Excel / BPS 移植记录

## 来源与边界

- 移植日期：2026-09-26。
- 来源仓库：/Users/kuhne/code/sub2api-ranxi2001，production 分支，读取时 HEAD 为 594cdf0d6。
- 目标仓库：/Users/kuhne/code/sub2api，起点为 363a36f18（codex/tps-and-remove-openai-risk-control）。
- 移植分支：codex/port-excel-bps。
- 采用 git cherry-pick -x，保留 39 个相关提交的来源 SHA，并按目标仓库的接口解决冲突。未将来源 production 分支整体合并。

## 包含功能

- OpenAI OAuth 账号的 Excel / BPS 开关、按模型选择、单账号和批量设置。
- Responses/compact 转发、客户端工具转换与回放、子线程隔离、结构化输出校验和错误诊断。
- 连接测试、BPS 工具往返探测、请求与实际推理等级展示、缓存创建转普通输入计费。
- 429 用量快照与冷却、可选在适用的 403 后原子关闭协议并刷新调度缓存。
- 系统级 base64 图片中转、临时 HTTPS 访问、磁盘存储和资源准入限制；保留来源最新的 512 个并发槽位配置上限。

## 兼容调整

以下调整集中在本地提交 ed8708aa8，便于与来源补丁分开审阅。

- 保留目标仓库已有的 TPS、账号身份、调度和原生压缩逻辑；未恢复已删除的 OpenAI ticket/风控相关模块。
- 未引入来源中的 Copilot、Pelican、smartops、request capture 等无关功能。
- 单独从来源 b5ab03cff3e3e6f3069c4ba958f8b1720506702b 导入 BPS 依赖的 openai_sse_read_pump.go 及其生命周期测试。未迁入该提交对普通透传路径的其他修改；省略依赖源分支独有 streamReadIncomplete 字段的测试。
- 适配当前 OpenAIGatewayService 构造参数和 runtime-block 判断参数；将提示词默认值处理内联，避免引入 Pelican 测试服务依赖。
- 账号普通文本测试使用映射前的请求模型交给网关，保证映射仅执行一次；按 BPS 模型范围分流，原生压缩和图片专项测试继续使用专用流程。
- 补齐 EditAccountModal 测试的 flushPromises 导入和自动卸载；将批量设置、BPS 工具探测及推理等级用量测试加入根 Makefile 的前端关键回归清单。
- 更新中英文账号提示和使用说明，使 base64 图片、结构化输出与资源容量说明符合实现。

## 提交对应关系

下表按移植顺序列出本地提交与来源提交。来源 9129624f7 是以第一父提交为主线 cherry-pick 的合并提交，包含图片容量分支及其合并适配（df6e604a0、5caa0a1c5、e065786c1、2dfc5aa5a）。重复补丁 ff7b45665、e9381279b、dda4c4e55 未重复应用，其有效修改已由表中相关提交覆盖。

| 本地提交 | 来源提交 | 说明 |
| --- | --- | --- |
| b9780df13 | d133e7c479a6 | feat: add Excel Basispoints protocol for OpenAI OAuth accounts |
| 44086d61e | edf5bebbd153 | fix: isolate BPS tool replay by Codex thread and cover concurrent calls |
| 4f59984de | deba7d6c8f20 | Fix BPS Codex account identity headers |
| 1a219869c | 1b9076eb86f4 | fix: satisfy strict lint checks across Basispoints adapter |
| 0599b5adb | 1116abc22f6f | fix: update Basispoints direct tool recovery from upstream |
| c7a068bd1 | 2a04fa467d31 | fix: route account intelligence tests through Excel BPS |
| 4af7c44b7 | 63446dab4c68 | fix: pass Basispoints adapter CI lint |
| 2cf968610 | 2ed974344b50 | fix: preserve Excel BPS error responses and sanitized diagnostics |
| 9ea1399e5 | dbfa71a0a86b | test: cover Excel BPS account test request contract |
| 7e9583351 | 129cf7c0dc66 | Fix BPS tool output IDs and error reporting |
| 7bff6bc4a | 8131407c482f | fix: complete Basispoints errcheck cleanup |
| 218ded1e9 | 868273dbbbb8 | fix: complete account test and Basispoints CI lint cleanup |
| cc93eced0 | 87e6ee977cb3 | fix: correct Basispoints envelope lint cleanup |
| 7e64c84e8 | 8caeae77dd0f | Fix BPS lint checks |
| 02392bcf0 | 4de3585332fa | fix: satisfy account test lint checks |
| b593dee94 | 154ee6879c6b | fix: check Excel BPS account test reasoning type |
| 4247c0030 | 1c3dd267dfea | fix: support validated structured output in Excel BPS |
| aed618b1a | be285e99246a | feat: bill Excel BPS cache creation as regular input |
| 2930b1146 | 460b02d3b831 | 支持 Excel BPS 内联图片转临时 HTTPS 链接 |
| b2495aa6c | a2d3ea37d84a | 升级图片解码依赖以修复安全扫描问题 |
| bfd7802ae | eb069778a5ce | 将 Excel BPS 图片中转配置移至系统设置 |
| 3d8fbea50 | 9e4c33c7eee4 | 限制图片中转资源占用并改为磁盘暂存 |
| cc8e23c6a | b4634c9f3a3e | test(routes): account for BPS image admission middleware |
| c7e98ba47 | 75f117e52816 | test(server): include BPS image settings in API contract |
| 8d3c63336 | ad3070b7ce28 | feat: support model-scoped Excel BPS routing |
| a425c313a | 3fb62aeb50ff | fix(bps): recover malformed transports and bypass unsupported tools |
| ee4709b55 | 1fb21644f6fd | 修复 BPS 单工具封装解析并补充内容错误定位 |
| b1aa24e05 | 49bb7b049e1e | 同步 BPS 缓存创建转输入的下游用量 |
| 27fbc968a | 3355086e018b | feat: add bulk Excel BPS account settings |
| 100385cac | d815d4a23726 | fix: 标记 BPS 子代理工具参数为明文 |
| f51f49702 | beb86d6caf53 | fix: 在用量明细展示 BPS 推理等级映射 |
| db881d3f2 | 7da0b92a1898 | fix: size BPS image admission to actual request bodies |
| 9507e241d | 7e881f0fd271 | test: format image admission regression test |
| b301a83b3 | 6df263ef2b54 | feat: 支持 BPS 403 时自动关闭协议 |
| 267182d94 | 7a97283dd8db | test: 清理 BPS 并发测试的调度事件 |
| 23b576a1a | 1dfbc6f4d0a1 | feat: add Excel BPS account tool roundtrip probe |
| 50b27db4e | 5c1839b28210 | fix: 修复 BPS 代码工具的嵌套转义问题 |
| 94b82e773 | 19becb835df9 | fix: 补齐 BPS 用量快照和 429 冷却 |
| b55faac67 | 9129624f7d2d | Merge pull request #91 from zhoumooooo/feat/bps-image-relay-configurable-max-requests |

## 验证记录

- 前端：make test-frontend 通过 lint、TypeScript 检查及 29 个测试文件中的 528 项关键回归；pnpm run build 通过生产构建及额外的 3 项语言包完整性测试。
- 后端：BPS 适配器包完整测试通过；账号、网关、SSE 生命周期、图片中转、中间件、路由、设置和仓储相关定向测试通过。新补充的账号模型映射/协议选择/原生压缩兼容回归通过。
- 数据库：使用 CI=true 强制实际启动临时 PostgreSQL/Redis，BPS 403 自动关闭的并发与缓存集成测试通过；TestAccountRepoSuite/TestBulkUpdate_ExcelBPSModelScope 通过。
- 静态检查：golangci-lint run ./... 返回 0 issues，最后的 service 包兼容调整也再次通过 lint；所有变更 Go 文件通过 gofmt 检查，git diff --check 无错误。
- 构建：CGO_ENABLED=0 go build -trimpath -o /tmp/sub2api-excel-bps-server ./cmd/server 通过。
- 首轮后端全量 unit 测试有一项原有的 WS 排队计时断言失败（等待 16.45525 ms，断言至少 40 ms）。openai_ws_pool.go 和对应测试相对起点没有变更；该用例随后连续单独运行 20 次全部通过。最终 go test -tags=unit -p 1 ./... 全量复测通过。

测试使用 mock 上游和临时测试数据库，不使用真实账号发起 BPS 请求。未执行远程 push、GitHub Actions 或生产部署。

操作方法、支持范围与图片中转设置见 [excel-bps.md](excel-bps.md)。
