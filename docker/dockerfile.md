docker compose -f docker/docker-compose.yml up -d

# 查看容器状态
docker compose -f docker/docker-compose.yml ps

# 查看日志
docker compose -f docker/docker-compose.yml logs -f proxy
# 如需：
# docker compose -f docker/docker-compose.yml logs -f namesrv
# docker compose -f docker/docker-compose.yml logs -f broker

# 查看服务状态
docker-compose ps

# 查看应用日志
docker-compose logs -f dt-server

# 重启应用
docker-compose restart dt-server

# 停止所有服务
docker-compose stop

# 停止并删除容器（保留数据）
docker-compose down

# 停止并删除容器和数据（慎用）
docker-compose down -v

# 连接 MySQL
docker exec -it dt-mysql mysql -uroot -proot dt_game

# 连接 Redis
docker exec -it dt-redis redis-cli