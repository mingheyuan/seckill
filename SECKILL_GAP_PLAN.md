# seckill 对比 newseckill 的差距与完善规划

## 1. 目标

基于当前 `seckill` 与 `newseckill` 的实现差异，形成一份以“补齐能力、可验证收尾”为核心的迭代规划。

目标不是重写，而是：

- 保留当前 `seckill` 已跑通能力
- 按优先级补齐 `newseckill` 已有而 `seckill` 仍缺失的能力
- 每阶段都可落地、可验收、可回归

---

## 2. 对比结论（当前状态）

### 2.1 已基本对齐（可继续沿用）

- 三层服务（proxy/layer/admin）基础可运行
- 活动开关与时间窗：`admin -> layer` 已生效
- `mysql-redis` 引擎切换入口已具备（含 fallback）
- 订单查询链路已打通（`/api/orders`）
- etcd 热更新链路已验证（写 etcd 后不重启即可影响秒杀）
- admin 更新活动自动同步 etcd 已验证
- 发布门禁脚本已存在：`scripts/test_release_gate.sh`

### 2.2 仍未补齐的核心差距（newseckill 有，seckill 仍缺）

#### P0（必须尽快补）

1. MySQL+Redis 一致性实现不完整（仍有 fallback 兜底路径，未形成强一致策略）
- newseckill 参考：`newseckill/backend/internal/layer/repository/mysql_redis.go`
- seckill 现状：`seckill/internal/layer/service/mysql_redis_store.go`
- 风险：高并发下可能出现“成功率异常低、状态不稳定、难定位一致性问题”

2. 自动化测试缺失（单测/集成测试）
- newseckill 参考：
  - `newseckill/backend/internal/layer/repository/repository_test.go`
  - `newseckill/backend/internal/layer/repository/mysql_redis_integration_test.go`
- seckill 现状：无 `*_test.go`
- 风险：每次修改后只能人工验证，回归风险高

3. 构建与测试标准化缺失（Makefile）
- newseckill 参考：`newseckill/backend/Makefile`
- seckill 现状：无 `Makefile`
- 风险：缺统一入口，CI 不易接入

#### P1（本轮建议完成）

4. 安全中间件缺失（RequestID / IP 控制 / 签名）
- newseckill 参考：
  - `newseckill/backend/internal/proxy/middleware/security.go`
  - `newseckill/backend/internal/proxy/middleware/security_test.go`
- seckill 现状：无 `internal/proxy/middleware`
- 风险：API 易被重放/刷接口，排障关联困难

5. Admin 发布器能力简化（缺同步统计与重试队列）
- newseckill 参考：`newseckill/backend/internal/admin/service/activity_publisher.go`
- seckill 现状：`seckill/internal/admin/service/etcd_pulisher.go`（建议修正命名为 publisher）
- 风险：发布失败不可观测，无法快速定位配置不同步

6. 健康检查与运维接口标准化不足
- newseckill 参考：统一 `/api/health`、`/internal/health`、`/admin/health`
- seckill 现状：使用 `/healthz`
- 风险：对接监控平台时需适配，运维一致性弱

#### P2（优化项）

7. 配置工程化程度不足（集中 config 包）
- newseckill 参考：`internal/*/config/config.go`
- seckill 现状：仍有部分硬编码路径/默认值

8. 负载测试工具化不足（k6）
- newseckill 参考：`newseckill/backend/bench/k6_seckill.js`
- seckill 现状：shell 并发脚本为主

---

## 3. 完善路线图（分阶段 + 验收）

## 阶段 A（P0）：一致性与测试基线

### A1. 强化 mysql-redis 一致性策略

改造点：

- 将 `mysql_redis_store` 从“fallback 可用”升级为“可配置强一致模式”
- 为关键路径增加失败原因统计（sold out / duplicate / busy）
- 保证库存、订单、重复购买标记在异常下可回滚或可追踪

建议文件：

- `seckill/internal/layer/service/mysql_redis_store.go`

验收命令：

```bash
cd /home/yuan/test_sum_seckill_concurrence/seckill/scripts
./test_release_gate.sh
```

通过标准：

- `stock_after >= 0`
- `success <= STOCK`
- 输出失败类型统计，且可解释

### A2. 建立单测与集成测试

改造点：

- 补 `memory_store`、`core`、`etcd_activity`、`security` 的单测
- 补 mysql-redis 并发一致性集成测试

建议文件：

- `seckill/internal/layer/service/*_test.go`
- `seckill/internal/proxy/middleware/*_test.go`
- `seckill/internal/layer/service/mysql_redis_integration_test.go`

验收命令：

```bash
cd /home/yuan/test_sum_seckill_concurrence/seckill
go test ./... -v
```

通过标准：

- `go test ./...` 全绿
- 集成测试可用环境下可重复通过

### A3. 增加 Makefile 统一入口

改造点：

- 标准化命令：`make test` / `make test-integration` / `make run` / `make gate`

建议文件：

- `seckill/Makefile`

验收命令：

```bash
cd /home/yuan/test_sum_seckill_concurrence/seckill
make test
make gate
```

通过标准：

- 命令稳定可复现
- 门禁失败时返回非 0 退出码

---

## 阶段 B（P1）：安全与可观测

### B1. 引入 proxy 安全中间件

改造点：

- Request ID 注入
- IP 黑白名单
- 可选签名校验（开关控制）

建议文件：

- `seckill/internal/proxy/middleware/security.go`
- `seckill/internal/proxy/controller/http.go`
- `seckill/cmd/proxy/main.go`

验收命令：

```bash
# 关闭签名校验可正常调用
curl --noproxy '*' -s http://127.0.0.1:8080/healthz

# 开启签名后，未签名请求应拒绝
```

通过标准：

- 关闭开关不影响现网
- 开启开关后规则正确生效

### B2. admin etcd 发布统计与重试

改造点：

- 增加发布队列长度、成功/失败计数、最后错误
- 增加重试策略

建议文件：

- `seckill/internal/admin/service/etcd_publisher.go`（建议统一命名）
- `seckill/internal/admin/controller/http.go`

验收命令：

```bash
curl --noproxy '*' -s http://127.0.0.1:8082/admin/activity/sync/stats
```

通过标准：

- 可观测发布状态
- etcd 短暂抖动时有重试且不阻塞主流程

### B3. 健康检查端点标准化

改造点：

- 保留 `/healthz` 兼容旧脚本
- 新增标准端点：
  - `/api/health`
  - `/internal/health`
  - `/admin/health`

验收命令：

```bash
curl --noproxy '*' -s http://127.0.0.1:8080/api/health
curl --noproxy '*' -s http://127.0.0.1:8081/internal/health
curl --noproxy '*' -s http://127.0.0.1:8082/admin/health
```

通过标准：

- 三端点均返回 200
- 旧 `/healthz` 不破坏现有脚本

---

## 阶段 C（P2）：工程化完善

### C1. 配置集中化

改造点：

- 新建 `internal/{proxy,layer,admin}/config` 包
- 将环境变量读取统一收敛

### C2. 压测工具化（k6）

改造点：

- 补 `bench/k6_seckill.js`
- 增加 `scripts/load_test.sh`

### C3. 文档与SOP

改造点：

- 增加“上线前检查清单”
- 增加“回滚操作手册”

---

## 4. 建议的最终交付件

- `seckill/SECKILL_GAP_PLAN.md`（本文件）
- `seckill/Makefile`
- `seckill/scripts/test_release_gate.sh`（强门禁版）
- `seckill/scripts/test_activity.sh`（活动热更新验收）
- `seckill/scripts/testredis_mysql.sh`（持久化验收）
- `seckill/internal/**/_test.go`（基础单测 + 集成测试）

---

## 5. 当前结论

当前 `seckill` 已跨过“能跑”阶段，进入“可稳定发布”阶段。
后续核心不是继续堆功能，而是优先补齐：

1. 一致性强校验
2. 自动化测试
3. 发布门禁标准化

完成上述 P0 后，再进入 P1/P2，才能把 `seckill` 真正补齐到接近 `newseckill` 的工程成熟度。
