# Redis Key 与类型

```text
键名(模板)                              类型
-------------------------------------  ----------------------
seckill:{activity_id}:stock:{shard_id} String
seckill:{activity_id}:stock:*          Pattern(Scan)
activity:config:{activity_id}          Hash
seckill:bought:{activity_id}:{user_id} String
seckill-stream                         Stream
activitylist                           Sorted Set
seckill:activity:{activity_id}:status  String
seckill:{activity_id}:shards:active    Set

group1                                 Consumer Group
{worker_id}(如0/1/2...)                Consumer
recovery-consumer                      Consumer
```

