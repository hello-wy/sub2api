# 简化经营统计 / Operating estimates

管理后台「经营分析」用于管理口径估算，不是可核对的财务利润表。
余额按实际扣减额度确认收入；订阅按冻结成交金额 × 整期消费比例确认收入并封顶。
成本由手工成本与赠送面值扣减组成。赠送面值不是现金支出，不要再手工重复登记。
线下收款仅作凭据，不增加利润或用户余额。缺少订阅依据时利润显示“待确认”。
期间报表按所选日期统计；余额和订阅价值是当前快照，累计估算利润截至当前。

Operating estimates use consumed wallet credits and the consumed share of a subscription's
frozen value as revenue. Costs combine manual expenses with grants at face value, not cash
outflow. Receipts do not add revenue or top up users. Missing subscription evidence leaves
profit unknown. Period totals follow the date filter; positions and cumulative profit are current.

## 切换及期初

迁移 255 保留所有 business_* 历史表和视图，停止旧业务捕获触发器；应用不再启动旧投影任务。
原始历史迁移不可修改。新统计默认未启用，不倒推历史，不重置余额、订单、订阅或用量。
管理员核实三个历史总数（系统已确认收入、未包含的私下收入、总成本）后启用。
期初总数不要包含预收款或尚未消费的套餐收入；期初包含的历史赠送不会再次计入成本。
启用后不能覆盖期初；使用收入/成本调整和有原因的冲销更正。重复凭据不会重复入账。
订阅缺失的成交金额或整期额度需要依据；已冻结字段不能修改。
报表时区与实际配额重置时区独立，无法可靠推导整期额度时保留待补充。

## 归档及回退

发布前对整个数据库做完整备份。仅回退应用可能使旧引擎重新写入归档，因此回退必须配套恢复数据库。
部署后可用标准 libpq 环境变量和 PGPASSFILE 执行：

```sh
python3 scripts/export-operating-legacy.py /secure/backups/legacy-20261010
```

脚本要求新目录，在同一只读快照中输出 legacy.dump（结构及数据）与 manifest.json
（切换时间、表清单、行数和 SHA-256）。归档含内部凭据，应限制访问。
归档不是全库备份：恢复到已具备对应历史迁移结构的隔离数据库；不要直接覆盖生产库。
本期不提供历史查询页面，也不自动将旧账折算成新期初。

## 验证

使用专用 PostgreSQL 18 测试实例，测试会创建并删除 operating_test_* 数据库：

```sh
cd backend
OPERATING_STATISTICS_TEST_DSN='postgres://postgres@localhost:55439/postgres?sslmode=disable' go test ./internal/repository -run TestOperating -count=1 -v
```

CI 集成任务设置 OPERATING_REQUIRE_POSTGRES=1，缺少测试连接时必须失败。需验证空库、真实旧账升级、重复迁移、归档恢复、
期初与结算并发、重复请求、过期订阅版本、赠送、冲销和失败事务。
旧 API 成本池、导入和对账接口已退役；新接口保留 /admin/business 下的
 overview/settings/initialize/records/records/:id/reverse/terms/terms/:id。

## 本次验证记录（2026-10-10）

- 后端服务完整编译通过；经营统计、HTTP 错误分类、退款及计费相关回归通过。
- PostgreSQL 18 独立测试库：6 项集成测试通过，包含实际旧迁移升级、归档导出/恢复、
  期初→结算→成本→冲销闭环、赠送、订阅封顶、并发初始化、结算失败回滚、过期权益期和夏令时。
- 前端全量类型检查、正式构建、语言键完整性（3 项）、新经营面板测试（10 项）及相关 ESLint 通过。
- 没有执行生产迁移或生产页面验收；数据库及组件测试不等同于真实浏览器端到端验收。
- 所有修改保持未提交；原有依赖锁文件和 .iq-preview 预览内容未纳入本次实现修改。

## 管理首页人民币消费折算（2026-11）

首页第一张卡的「消费折合 ¥」与第三张卡「今日收款」采用不同口径：前者估算当天
消费对应的人民币价值，后者是当天订单现金实收（包含未消费预收款），不应比较大小。
费用明细中的「成本」「标准」仍为美元计价。

首页统计不依赖经营分析是否初始化，也不会启用经营统计或修改余额及扣费：

- 余额：当天扣费额度 × 用户历史已履约人民币充值的净实付/净到账额度加权均价。
  到账额度含充值赠送；退款按订单比例扣除。独立赠送、管理员调账及线下充值没有逐笔资金追踪，
  有充值历史的混合余额统一按加权价估算，不能视为精确的现金收入。
- 套餐：关联订单净实付 × 当天消耗/整期额度，并结合本期历史消耗将每期累计折算封顶至净实付。
  采用订单实际会员折扣；余额购买套餐先按购买时的充值均价换算，不增加现金收款。
  只通过订阅备注中的精确 `payment order <id>` 行关联同用户同分组订单；同组重购开始新周期。
- 整期额度采用当前分组配置：生命周期模式取有效额度限制的最小值；滚动模式必须有明确的整期额度。
  没有订单、非人民币订单、没有整期额度或没有充值换算依据的部分不猜汇率，返回 `unpriced_credits`，
  页面显示「部分」并在提示里列出未折算额度。零元成交与未知依据分别处理。

这是只读估算，不是历史冻结的财务账：分组额度变更、退款和充值均价变化会影响结果；
历史用量清理会影响套餐累计封顶判断。统计按站点时区自然日划分；查询有 5 秒超时与
30 秒缓存，失败显示「—」，不会回退成美元数字或阻断整个首页。
