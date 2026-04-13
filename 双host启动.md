一、先起 Redis Cluster（两台都执行 up，再在 131 执行 create）

在 192.168.32.131 执行：
cd /home/yuan/test_sum_seckill_concurrence/seckill
ROLE=A SELF_IP=192.168.32.131 PEER_IP=192.168.32.132 scripts/start_redis_cluster_2host.sh up
在 192.168.32.132 执行：
cd /home/yuan/test_sum_seckill_concurrence/seckill
ROLE=B SELF_IP=192.168.32.132 PEER_IP=192.168.32.131 scripts/start_redis_cluster_2host.sh up
回到 192.168.32.131 执行创建集群：
cd /home/yuan/test_sum_seckill_concurrence/seckill
ROLE=A SELF_IP=192.168.32.131 PEER_IP=192.168.32.132 scripts/start_redis_cluster_2host.sh create
任一台查看状态：
cd /home/yuan/test_sum_seckill_concurrence/seckill
ROLE=A SELF_IP=192.168.32.131 PEER_IP=192.168.32.132 scripts/start_redis_cluster_2host.sh status
二、配置并启动服务（两台都执行）

假设 Nacos 跑在 192.168.32.131:8848。

在 192.168.32.131:
cd /home/yuan/test_sum_seckill_concurrence/seckill
SELF_IP=192.168.32.131 NACOS_IP=192.168.32.131 scripts/start_services_dual_layer_2host.sh
在 192.168.32.132:
cd /home/yuan/test_sum_seckill_concurrence/seckill
SELF_IP=192.168.32.132 NACOS_IP=192.168.32.131 scripts/start_services_dual_layer_2host.sh
注意事项（很关键）

两台机器防火墙需放行 Redis 端口 17001-17006 以及集群总线端口 27001-27006。
两台都要能访问 Nacos 8848。
两台机器时间尽量同步(NTP),避免签名/时序类问题。