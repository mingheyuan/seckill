cd /home/yuan/test_sum_seckill_concurrence/seckill
LAYER_STORAGE_ENGINE=mysql-redis \
LAYER_MYSQL_DSN='root:123456@tcp(127.0.0.1:3306)/seckill?parseTime=true&loc=Local&charset=utf8mb4' \
LAYER_REDIS_ADDR='127.0.0.1:6379' \
LAYER_ETCD_ENABLED=true \
LAYER_ETCD_ENDPOINTS='127.0.0.1:2379' \
LAYER_ETCD_ACTIVITY_KEY='/seckill/activity/config' \
ADMIN_ETCD_ENABLED=true \
ADMIN_ETCD_ENDPOINTS='127.0.0.1:2379' \
ADMIN_ETCD_ACTIVITY_KEY='/seckill/activity/config' \
bash ./scripts/start.sh restart