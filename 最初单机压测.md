# seckill 压测报告

## 1. 测试目标

验证 `seckill` 在三层架构下的吞吐、延迟和错误率表现，确认本轮改造（Proxy 安全中间件、Redis Lua 原子扣减）后在高并发下仍稳定可用。

## 2. 测试环境与参数

- 测试时间: 2026-03-29
- 压测工具: k6
- 压测脚本: `seckill/bench/k6_seckill.js`
- 执行命令:

```bash
cd /home/yuan/test_sum_seckill_concurrence/seckill
PROXY_REQUIRE_SIGNATURE=false PROXY_REQ_PER_SEC=100000 bash ./scripts/start.sh restart
curl --noproxy '*' -s -X POST http://127.0.0.1:8082/admin/activity/init \
  -H 'Content-Type: application/json' \
  -d '{"activity_id":1001,"stock":500000}'
VUS=200 DURATION=30s BASE_URL=http://127.0.0.1:8080 ACTIVITY_ID=1001 bash ./scripts/load_test.sh
```

- 并发用户: 200 VU
- 持续时间: 30s
- 活动 ID: 1001
- 初始库存: 500000

## 3. 压测结果

### 3.1 核心指标

- 吞吐 (`http_reqs`): 105286
- 每秒请求数 (`RPS`): 3503.45 req/s
- 平均延迟: 5.99ms
- P90 延迟: 13.37ms
- P95 延迟: 17.19ms
- 最大延迟: 87.35ms
- HTTP 失败率: 0.00%
- Checks 通过率: 100%

### 3.2 阈值判定

- `http_req_duration p(95)<200ms`: 通过 (17.19ms)
- `http_req_failed rate<0.05`: 通过 (0.00%)

### 3.3 库存一致性观察

- 压测前库存: 500000
- 压测后库存: 394714
- 库存减少: 105286

库存减少量与请求总量一致，说明本轮压测中均完成了秒杀扣减，未出现超卖和异常失败风暴。

## 4. 结论

在 200 VU、30s 场景下，`seckill` 当前版本达到约 **3.50k req/s**，P95 约 **17.19ms**，失败率 **0%**，阈值全部通过。

本次结果可作为 `seckill` 当前阶段性能基线。后续建议在以下维度继续验证：

1. 开启签名校验后的性能对比
2. 开启 IP 黑名单/限流策略后的性能对比
3. MySQL/Redis 异常注入下的稳定性与恢复能力
