mysql -uroot -p123456 -e "CREATE DATABASE IF NOT EXISTS seckill DEFAULT CHARSET utf8mb4;"

cd /home/yuan/test_sum_seckill_concurrence/seckill
LAYER_STORAGE_ENGINE=mysql-redis \
LAYER_MYSQL_DSN='root:123456@tcp(127.0.0.1:3306)/seckill?parseTime=true&loc=Local&charset=utf8mb4' \
LAYER_REDIS_ADDR='127.0.0.1:6379' \
bash ./scripts/start.sh restart