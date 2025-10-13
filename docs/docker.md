# MyEtcd Docker 部署指南

本文档介绍如何使用Docker部署MyEtcd，包括单节点和集群模式。

## 目录

- [前提条件](#前提条件)
- [快速开始](#快速开始)
- [单节点部署](#单节点部署)
- [集群部署](#集群部署)
- [监控部署](#监控部署)
- [配置说明](#配置说明)
- [故障排除](#故障排除)

## 前提条件

- Docker 20.10+
- Docker Compose 2.0+
- 至少2GB可用内存
- 至少10GB可用磁盘空间

## 快速开始

### 1. 克隆项目

```bash
git clone https://github.com/your-org/myetcd.git
cd myetcd
```

### 2. 启动单节点实例

```bash
docker-compose up -d myetcd-single
```

### 3. 验证服务

```bash
curl http://localhost:2379/health
```

## 单节点部署

### 启动单节点实例

```bash
# 构建并启动
docker-compose up -d myetcd-single

# 查看日志
docker-compose logs -f myetcd-single

# 停止服务
docker-compose stop myetcd-single
```

### 端口映射

| 容器端口 | 主机端口 | 说明 |
|---------|---------|------|
| 2379    | 2379    | API端口 |
| 2380    | 2380    | 指标端口 |
| 8080    | 8080    | Raft端口 |

### 数据持久化

单节点实例使用以下Docker卷：

- `myetcd-single-data`: 存储数据
- `myetcd-single-logs`: 存储日志

## 集群部署

### 启动三节点集群

```bash
# 启动集群
docker-compose --profile cluster up -d

# 查看集群状态
docker-compose ps

# 查看节点日志
docker-compose logs -f myetcd-node1
```

### 端口映射

| 节点 | API端口 | 指标端口 | Raft端口 |
|------|---------|----------|----------|
| node1 | 12379   | 12380    | 18080    |
| node2 | 22379   | 22380    | 28080    |
| node3 | 32379   | 32380    | 38080    |

### 负载均衡

集群模式包含Nginx负载均衡器，通过以下端口访问：

- `http://localhost:80`: API访问入口
- `http://localhost:80/metrics`: 指标访问入口

### 集群配置

集群配置文件位于 `config/cluster/config.json`：

```json
{
  "node_id": "node1",
  "cluster_nodes": ["node1", "node2", "node3"],
  "node_addresses": {
    "node1": "myetcd-node1:8080",
    "node2": "myetcd-node2:8080",
    "node3": "myetcd-node3:8080"
  }
}
```

## 监控部署

### 启动监控服务

```bash
# 启动Prometheus和Grafana
docker-compose --profile monitoring up -d

# 访问Prometheus
open http://localhost:9090

# 访问Grafana
open http://localhost:3000
```

### Grafana配置

- 用户名: `admin`
- 密码: `admin`

### 监控指标

MyEtcd提供以下监控指标：

- HTTP请求计数和延迟
- 存储操作计数和延迟
- Raft状态和日志指标
- 系统资源使用情况

## 配置说明

### 环境变量

| 变量名 | 默认值 | 说明 |
|--------|--------|------|
| MYETCD_DATA_DIR | /app/data | 数据目录 |
| MYETCD_WAL_DIR | /app/data/wal | WAL目录 |
| MYETCD_LOG_DIR | /app/logs | 日志目录 |
| MYETCD_CONFIG_FILE | /app/config/config.json | 配置文件路径 |
| MYETCD_LOG_LEVEL | INFO | 日志级别 |

### 自定义配置

1. 修改 `config/config.json` 文件
2. 重新构建镜像：

```bash
docker-compose build
```

3. 重启服务：

```bash
docker-compose up -d
```

## 故障排除

### 常见问题

#### 1. 容器启动失败

```bash
# 查看详细日志
docker-compose logs myetcd-single

# 检查配置文件
docker exec myetcd-single cat /app/config/config.json
```

#### 2. 集群节点无法通信

```bash
# 检查网络连接
docker exec myetcd-node1 ping myetcd-node2

# 检查端口
docker exec myetcd-node1 telnet myetcd-node2 8080
```

#### 3. 数据丢失

```bash
# 检查卷状态
docker volume ls
docker volume inspect myetcd-single-data

# 备份数据
docker run --rm -v myetcd-single-data:/data -v $(pwd):/backup alpine tar czf /backup/myetcd-backup.tar.gz -C /data .
```

### 性能优化

#### 1. 调整资源限制

在 `docker-compose.yml` 中添加资源限制：

```yaml
services:
  myetcd-single:
    deploy:
      resources:
        limits:
          cpus: '1.0'
          memory: 1G
        reservations:
          cpus: '0.5'
          memory: 512M
```

#### 2. 优化存储性能

使用SSD存储并调整挂载选项：

```yaml
volumes:
  myetcd-single-data:
    driver: local
    driver_opts:
      type: none
      o: bind
      device: /path/to/ssd/myetcd-data
```

### 安全配置

#### 1. 使用TLS

生成TLS证书并更新配置：

```bash
# 生成证书
openssl genrsa -out server.key 2048
openssl req -new -x509 -key server.key -out server.crt -days 365

# 更新配置
cp server.crt server.key /path/to/certs/
```

#### 2. 网络隔离

创建自定义网络：

```bash
docker network create --driver bridge myetcd-net
```

在 `docker-compose.yml` 中使用：

```yaml
networks:
  myetcd-net:
    external: true
```

## 开发环境

### 开发模式启动

```bash
# 启动开发环境
docker-compose -f docker-compose.yml -f docker-compose.dev.yml up -d
```

### 调试容器

```bash
# 进入容器调试
docker exec -it myetcd-single sh

# 查看进程
docker exec myetcd-single ps aux

# 查看网络
docker exec myetcd-single netstat -tlnp
```

## 生产环境

### 生产环境配置

1. 使用外部数据库存储
2. 配置日志轮转
3. 设置备份策略
4. 配置监控告警

### 备份和恢复

```bash
# 备份数据
docker exec myetcd-single tar czf /tmp/backup.tar.gz -C /app/data .

# 恢复数据
docker cp backup.tar.gz myetcd-single:/tmp/
docker exec myetcd-single tar xzf /tmp/backup.tar.gz -C /app/data
```

## 更多信息

- [MyEtcd文档](../README.md)
- [API参考](api.md)
- [配置指南](configuration.md)
- [监控指南](monitoring.md)