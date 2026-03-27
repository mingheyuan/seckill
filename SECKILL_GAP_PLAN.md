# seckill 补齐 newseckill 能力分阶段实施文档

## 1. 目标

本文件只关注一件事：把 newseckill 已具备、但 seckill 尚未完整具备的能力，按可落地、可验收、可回归的方式分阶段补齐。

阶段规则与之前一致：

- 每阶段都有高亮代码（可直接照着实现）
- 每阶段都有验收命令和通过标准
- 每阶段都有“前一步错误检验”（防止改完新功能把旧功能改坏）

## 2. 差异清单（newseckill 有，seckill 缺或不完整）

### 2.1 接入安全策略

- 集成限流（每 IP 每秒限流）
- IP 黑白名单
- 请求 ID 注入（便于链路追踪）
- 可选签名校验（时间戳 + HMAC）

### 2.2 库存一致性与并发可靠性

- Redis Lua 脚本原子扣减库存 + 原子累计用户购买次数
- MySQL 持久化 + Redis 热数据双存储协作
- 并发一致性集成测试（库存不超卖、同用户不重复）

### 2.3 配置热更新与运维可观测

- layer 通过 etcd watch 热更新活动配置
- admin 发布活动到 etcd 带队列、重试、统计
- 统一测试入口（Makefile + 门禁脚本）

## 3. 代码高亮对照

> 本节是“可照写核心片段”。左侧是 newseckill 已验证实现，右侧是 seckill 当前状态。

### 3.1 安全中间件（Request ID + IP 黑白名单）

newseckill 关键代码：

文件：newseckill/backend/internal/proxy/middleware/security.go

```go
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader("X-Request-ID")
		if id == "" {
			id = randomID()
		}
		c.Set("request_id", id)
		c.Header("X-Request-ID", id)
		c.Next()
	}
}

func IPAccessControl(whitelist, blacklist []string) gin.HandlerFunc {
	white := toSet(whitelist)
	black := toSet(blacklist)
	return func(c *gin.Context) {
		ip := c.ClientIP()
		if _, blocked := black[ip]; blocked {
			c.JSON(http.StatusForbidden, gin.H{"message": "ip is blocked"})
			c.Abort()
			return
		}
		if len(white) > 0 {
			if _, ok := white[ip]; !ok {
				c.JSON(http.StatusForbidden, gin.H{"message": "ip is not in whitelist"})
				c.Abort()
				return
			}
		}
		c.Next()
	}
}
```

seckill 当前状态：

- 目录里没有独立中间件目录：seckill/internal/proxy/middleware
- 入口只做了基础路由注册：seckill/cmd/proxy/main.go

### 3.2 限流中间件（每 IP 每秒）

newseckill 关键代码：

文件：newseckill/backend/internal/proxy/middleware/ratelimit.go

```go
func (r *RateLimiter) Handler() gin.HandlerFunc {
	return func(c *gin.Context) {
		now := time.Now().Unix()
		ip := c.ClientIP()

		r.mu.Lock()
		b, ok := r.m[ip]
		if !ok {
			b = &bucket{timestamp: now, count: 1}
			r.m[ip] = b
			r.mu.Unlock()
			c.Next()
			return
		}

		if b.timestamp != now {
			b.timestamp = now
			b.count = 1
			r.mu.Unlock()
			c.Next()
			return
		}

		b.count++
		allow := b.count <= r.limit
		r.mu.Unlock()

		if !allow {
			c.JSON(429, gin.H{"message": "请求过于频繁，请稍后重试"})
			c.Abort()
			return
		}
		c.Next()
	}
}
```

seckill 当前状态：

- 没有 proxy 层全局中间件限流，只有 layer/core 内部按 userID 的轻量频控

### 3.3 签名校验（可开关）

newseckill 关键代码：

文件：newseckill/backend/internal/proxy/controller/handler.go

```go
if h.cfg.RequireSignature {
	if !h.verifySignature(c, req) {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "签名校验失败"})
		return
	}
}

func (h *Handler) verifySignature(c *gin.Context, req model.SeckillRequest) bool {
	timestampText := c.GetHeader("X-Timestamp")
	signature := c.GetHeader("X-Signature")
	if timestampText == "" || signature == "" {
		return false
	}
	...
	payload := fmt.Sprintf("%d:%d:%d", req.UserID, req.ProductID, ts)
	mac := hmac.New(sha256.New, []byte(h.cfg.SignSecret))
	mac.Write([]byte(payload))
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signature))
}
```

seckill 当前状态：

- seckill/internal/proxy/controller/http.go 没有签名校验流程
- seckill/cmd/proxy/main.go 没有签名开关配置

### 3.4 Redis Lua 原子扣减库存 + 防重复

newseckill 关键代码：

文件：newseckill/backend/internal/layer/repository/mysql_redis.go

```go
const acquireScript = `
local stock = redis.call('GET', KEYS[1])
if (not stock) then
  return 0
end
stock = tonumber(stock)
if stock <= 0 then
  return 0
end
local uid = tostring(ARGV[1])
local limit = tonumber(ARGV[2])
local c = tonumber(redis.call('HGET', KEYS[2], uid) or '0')
if c >= limit then
  return 1
end
redis.call('DECR', KEYS[1])
redis.call('HINCRBY', KEYS[2], uid, 1)
return 2
`
```

seckill 当前状态：

- seckill/internal/layer/service/mysql_redis_store.go 仍是 SetNX + Decr 两步，非 Lua 单脚本
- SaveOrder 失败时 fallback 到 memory，存在双写语义复杂度

### 3.5 etcd 发布器（重试 + 统计）

newseckill 关键代码：

文件：newseckill/backend/internal/admin/service/activity_publisher.go

```go
type PublishStats struct {
	Enabled        bool      `json:"enabled"`
	QueueLen       int       `json:"queueLen"`
	QueueCap       int       `json:"queueCap"`
	QueuedTotal    uint64    `json:"queuedTotal"`
	PublishedTotal uint64    `json:"publishedTotal"`
	FailedTotal    uint64    `json:"failedTotal"`
	DroppedTotal   uint64    `json:"droppedTotal"`
	LastError      string    `json:"lastError,omitempty"`
	LastErrorAt    time.Time `json:"lastErrorAt,omitempty"`
	LastSuccessAt  time.Time `json:"lastSuccessAt,omitempty"`
}

func (p *EtcdActivityPublisher) PublishActivity(ctx context.Context, cfg model.ActivityConfig) error {
	select {
	case p.queue <- cfg:
		p.queuedTotal.Add(1)
		return nil
	default:
		p.droppedTotal.Add(1)
		p.setLastError(ErrPublishQueueFull.Error())
		return ErrPublishQueueFull
	}
}
```

seckill 当前状态：

- seckill/internal/admin/service/etcd_pulisher.go 仅同步 put，无队列/重试/统计

### 3.6 并发一致性集成测试

newseckill 关键代码：

文件：newseckill/backend/internal/layer/repository/mysql_redis_integration_test.go

```go
func TestMySQLRedisConcurrentStockConsistency(t *testing.T) {
	...
	for i := 0; i < 200; i++ {
		uid := int64(100000 + i)
		wg.Add(1)
		go func(userID int64) {
			defer wg.Done()
			p, err := store.TryAcquire(productID, userID)
			if err == nil {
				success.Add(1)
				store.SaveOrder(NewOrder(p.ID, userID, p.Price))
			}
		}(uid)
	}
	...
	if success.Load() != stock {
		t.Fatalf("success should equal stock, want=%d got=%d", stock, success.Load())
	}
}
```

seckill 当前状态：

- 缺少 *_test.go 的并发一致性自动测试

## 4. 分阶段实施计划（每阶段含前置回归）

## 阶段 1：补齐 proxy 安全骨架

### 实施目标

补齐 Request ID、IP 黑白名单、全局限流，并挂到 proxy 入口。

### 需改文件

- 新增：seckill/internal/proxy/middleware/security.go
- 新增：seckill/internal/proxy/middleware/ratelimit.go
- 修改：seckill/cmd/proxy/main.go

### 最小落地步骤

1. 创建 RequestID 与 IPAccessControl 中间件（可直接复用 3.1 逻辑）
2. 创建 NewRateLimiter(limit).Handler()（可直接复用 3.2 逻辑）
3. 在 proxy 启动处按顺序挂载：RequestID -> IPAccessControl -> RateLimiter

### 验收命令

```bash
cd /home/yuan/test_sum_seckill_concurrence/seckill
bash ./scripts/start.sh restart

curl --noproxy '*' -si http://127.0.0.1:8080/healthz | head
curl --noproxy '*' -si -H 'X-Forwarded-For: 1.2.3.4' http://127.0.0.1:8080/api/orders?user_id=u1 | head
```

### 通过标准

- 响应头带 X-Request-ID
- 黑名单 IP 返回 403
- 正常请求不受影响

### 前一步错误检验（基线回归）

```bash
bash ./scripts/test_user_buy_twice.sh
```

- 预期：同用户重复购买第二次失败
- 若失败，先回看是否误改 proxy 请求参数名（user_id/activity_id）

## 阶段 2：补齐签名校验（可开关）

### 实施目标

把签名校验作为可配置能力，默认关闭，灰度开启。

### 需改文件

- 修改：seckill/internal/proxy/controller/http.go
- 修改：seckill/cmd/proxy/main.go
- 建议新增：seckill/internal/proxy/config/config.go

### 最小落地步骤

1. 新增环境变量：
- PROXY_REQUIRE_SIGNATURE（默认 false）
- PROXY_SIGN_SECRET
- PROXY_SIGN_MAX_SKEW_SEC
2. 在 Seckill handler 中，解析请求后按开关执行 verifySignature
3. 签名算法按 newseckill 对齐：payload=userID:activityID:timestamp，HMAC-SHA256

### 验收命令

```bash
# 1) 开关关闭：应可正常下单
curl --noproxy '*' -s -X POST http://127.0.0.1:8080/api/seckill \
  -H 'Content-Type: application/json' \
  -d '{"user_id":"sig_u1","activity_id":1001}'

# 2) 开关开启且未带签名：应返回未授权
PROXY_REQUIRE_SIGNATURE=true bash ./scripts/start.sh restart
curl --noproxy '*' -si -X POST http://127.0.0.1:8080/api/seckill \
  -H 'Content-Type: application/json' \
  -d '{"user_id":"sig_u2","activity_id":1001}' | head
```

### 通过标准

- 开关关闭时行为兼容旧版本
- 开关开启时无签名请求被拒绝

### 前一步错误检验（阶段 1 回归）

- 403 与 429 行为仍符合预期
- X-Request-ID 不丢失

## 阶段 3：库存扣减升级为 Lua 原子脚本

### 实施目标

把 seckill 的 TryReserve 从多命令流程升级为 Lua 原子流程，减少并发竞态与补偿复杂度。

### 需改文件

- 修改：seckill/internal/layer/service/mysql_redis_store.go

### 最小落地步骤

1. 引入 Lua 脚本（参考 3.4）
2. Eval 返回码约定：
- 0: 售罄
- 1: 重复购买
- 2: 成功
3. 保留 rollback 逻辑用于“队列提交失败”场景

### 验收命令

```bash
cd /home/yuan/test_sum_seckill_concurrence/seckill/scripts
bash ./test_release_gate.sh
```

### 通过标准

- success <= STOCK
- stock_after >= 0
- 无明显异常日志风暴（redis eval error 连续刷屏）

### 前一步错误检验（阶段 2 回归）

- 签名开关开/关行为保持不变
- 限流和黑白名单逻辑不受影响

## 阶段 4：补齐并发一致性自动化测试

### 实施目标

把“肉眼看日志”升级为“go test 可重复证明一致性”。

### 需改文件

- 新增：seckill/internal/layer/service/mysql_redis_integration_test.go
- 新增：seckill/internal/proxy/middleware/security_test.go

### 最小落地步骤

1. 参考 newseckill 的并发测试结构，至少覆盖：
- 并发抢购成功数等于库存
- 同用户高并发仅成功 1 次
2. 为 security 中间件补单测：
- 黑名单拦截
- RequestID 注入

### 验收命令

```bash
cd /home/yuan/test_sum_seckill_concurrence/seckill
go test ./... -v
```

### 通过标准

- 所有新增测试稳定通过
- 测试失败日志可直接定位到断言

### 前一步错误检验（阶段 3 回归）

- release gate 仍通过
- 活动关闭时继续返回 activity not open

## 阶段 5：补齐 admin etcd 发布器可靠性

### 实施目标

把 admin 的 etcd 发布从“同步单次 put”升级为“异步队列 + 重试 + 统计”。

### 需改文件

- 修改：seckill/internal/admin/service/etcd_pulisher.go（建议重命名为 etcd_publisher.go）
- 修改：seckill/internal/admin/controller/http.go（新增统计接口）

### 最小落地步骤

1. 增加 PublishStats 结构体
2. 增加发布队列和后台 worker
3. 增加重试次数、重试间隔配置
4. 暴露 /admin/activity/sync/stats

### 验收命令

```bash
curl --noproxy '*' -s http://127.0.0.1:8082/admin/activity/sync/stats
```

### 通过标准

- 能看到 queueLen/PublishedTotal/FailedTotal/LastError 等指标
- etcd 短暂异常后恢复，发布可继续成功

### 前一步错误检验（阶段 4 回归）

```bash
go test ./... -v
bash ./scripts/test_activity.sh
```

- 预期：活动变更仍能实时生效，且测试仍全绿

## 阶段 6：工程化收口（Makefile + 统一门禁）

### 实施目标

把阶段成果固化为统一执行入口，避免“会的人才能跑”。

### 需改文件

- 新增：seckill/Makefile
- 修改：seckill/scripts/test_release_gate.sh

### 最小落地步骤

1. 参考 newseckill/backend/Makefile 增加目标：
- make test
- make test-integration
- make gate
2. 强化 gate 输出：
- success/fail 分类统计
- 失败类型分桶（sold out/duplicate/system busy/activity not open）

### 验收命令

```bash
cd /home/yuan/test_sum_seckill_concurrence/seckill
make test
make gate
```

### 通过标准

- 两条命令都可在干净环境重复执行
- gate 失败时返回非 0，成功时返回 0

### 前一步错误检验（阶段 5 回归）

- /admin/activity/sync/stats 可访问
- etcd 热更新链路不回退

## 5. 阶段完成定义（DoD）

每阶段都必须满足：

1. 代码：功能实现并可运行
2. 验证：验收命令可复现通过
3. 回归：前一步错误检验通过
4. 文档：本文件对应阶段状态更新为“已完成”

建议增加一列阶段状态：

- [ ] 阶段 1
- [ ] 阶段 2
- [ ] 阶段 3
- [ ] 阶段 4
- [ ] 阶段 5
- [ ] 阶段 6

---

如果你要我继续，我可以直接按这个文档从“阶段 1”开始落代码，并且每完成一步都给你：改动文件、关键 diff、验收命令输出和回归结果。