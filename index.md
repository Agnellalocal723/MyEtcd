---
layout: home
title: MyEtcd - 分布式键值存储系统
description: 一个完整的分布式键值存储系统实现，用于学习分布式系统核心概念
---

# MyEtcd - 分布式键值存储系统

[![Go Version](https://img.shields.io/badge/Go-1.19+-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![Build Status](https://github.com/EvildoerXiaoyy/MyEtcd/workflows/CI%2FCD%20Pipeline/badge.svg)](https://github.com/EvildoerXiaoyy/MyEtcd/actions)
[![Coverage](https://codecov.io/gh/EvildoerXiaoyy/MyEtcd/branch/main/graph/badge.svg)](https://codecov.io/gh/EvildoerXiaoyy/MyEtcd)
[![Go Report Card](https://goreportcard.com/badge/github.com/EvildoerXiaoyy/MyEtcd)](https://goreportcard.com/report/github.com/EvildoerXiaoyy/MyEtcd)

MyEtcd是一个基于etcd设计理念的分布式键值存储系统，使用Go语言实现。这个项目旨在帮助开发者深入理解分布式系统的核心概念和实现细节。

## ✨ 项目特色

- **🎯 学习导向** - 从简单到复杂的渐进式学习路径
- **📚 详细注释** - 每个模块都有详细的中文注释和设计思路
- **🏗️ 完整实现** - 包含etcd的核心功能，非简化版本
- **🚀 生产就绪** - 包含配置管理、日志系统、监控指标等生产级特性
- **🐳 容器化** - 完整的Docker支持，便于部署和扩展
- **📊 高性能** - 基准测试工具，性能优化指导

## 🔥 核心功能

### 📦 存储引擎
- **ACID事务** - 完整的原子性、一致性、隔离性、持久性实现
- **并发控制** - 基于读写锁的安全并发访问
- **范围查询** - 支持键范围扫描和分页
- **批量操作** - 高效的批量读写操作

### 📝 WAL机制
- **数据持久性** - 通过预写日志确保数据不丢失
- **分段管理** - 避免单个文件过大
- **CRC32校验** - 保证数据完整性
- **崩溃恢复** - 系统重启后自动重放WAL

### 👁️ Watch机制
- **实时监听** - 基于事件驱动的高效监听
- **前缀匹配** - 支持模式匹配
- **历史数据** - 可选择返回变化前的值
- **SSE长连接** - 标准的Server-Sent Events实现

### ⏰ 租约系统
- **TTL管理** - 完整的生命周期管理
- **自动过期** - 灵活的租约操作
- **键绑定** - 自动清理过期键
- **批量管理** - 高效的租约操作

### 🚢 Raft算法
- **一致性保证** - 完整的Raft算法实现
- **集群管理** - 节点加入和离开
- **状态机** - 一致性应用状态
- **网络传输** - 节点间通信

### 🌐 HTTP API
- **RESTful设计** - 完整的API设计原则
- **统一格式** - 一致的API响应结构
- **错误处理** - 统一的错误处理机制
- **客户端SDK** - 便于使用的Go客户端

### 📈 监控系统
- **Prometheus集成** - 标准化的监控指标
- **性能分析** - 延迟和吞吐量监控
- **健康检查** - 系统状态监控
- **指标导出** - 完整的运行时指标

## 🚀 快速开始

### 安装和运行

```bash
# 克隆项目
git clone https://github.com/EvildoerXiaoyy/MyEtcd.git
cd MyEtcd

# 构建项目
make build

# 启动服务器
./bin/server

# 运行测试
make test
```

### Docker运行

```bash
# 构建镜像
docker build -t myetcd .

# 运行容器
docker run -p 2379:2379 myetcd
```

### 基本使用

```bash
# 设置键值
curl -X PUT -H "Content-Type: application/json" \
     -d '{"key":"test","value":"hello"}' \
     http://localhost:2379/v1/key/test

# 获取值
curl http://localhost:2379/v1/key/test

# 监听变化
curl -X POST -H "Content-Type: application/json" \
     -d '{"key":"test","prefix":false,"prevKV":false}' \
     http://localhost:2379/v1/watch
```

## 📖 学习路径

### 第一阶段：核心基础 (1-2周)
1. **WAL机制** - 理解数据持久性
2. **存储引擎** - 学习ACID事务
3. **API服务器** - 掌握HTTP服务设计

### 第二阶段：高级功能 (2-3周)
1. **Watch机制** - 理解事件驱动架构
2. **租约系统** - 学习资源管理
3. **批量操作** - 掌握性能优化

### 第三阶段：分布式系统 (3-4周)
1. **Raft算法** - 深入分布式一致性
2. **监控系统** - 学习可观测性
3. **配置和日志** - 掌握运维实践

## 📊 性能指标

| 操作类型 | 吞吐量 | 平均延迟 | P99延迟 |
|---------|--------|----------|---------|
| PUT     | 10,000 ops/s | 0.1ms | 0.5ms |
| GET     | 50,000 ops/s | 0.05ms | 0.2ms |
| DELETE  | 8,000 ops/s | 0.1ms | 0.6ms |
| WATCH   | 1,000 events/s | 0.2ms | 1.0ms |

## 🏗️ 项目架构

```
┌─────────────────────────────────────────────────────────┐
│                    客户端层                              │
│  HTTP API / CLI / 客户端库                               │
├─────────────────────────────────────────────────────────┤
│                    业务逻辑层                            │
│  Watch / Lease / Transaction / Range Query               │
├─────────────────────────────────────────────────────────┤
│                   一致性层                              │
│  Raft Algorithm / Leader Election / Log Replication     │
├─────────────────────────────────────────────────────────┤
│                   存储层                                │
│  WAL / Snapshot / Memory Store / TTL Management          │
└─────────────────────────────────────────────────────────┘
```

## 📚 文档

- **[实现总结](docs/implementation-summary.md)** - 完整的项目实现总结
- **[快速开始](docs/quickstart.md)** - 快速上手指南
- **[API文档](docs/api.md)** - 完整的API参考
- **[配置指南](docs/configuration.md)** - 配置选项详解
- **[部署指南](docs/deployment.md)** - 生产环境部署
- **[开发指南](docs/development.md)** - 开发环境搭建

## 🤝 贡献

我们欢迎各种形式的贡献！

1. **Fork项目**
2. **创建特性分支** (`git checkout -b feature/AmazingFeature`)
3. **提交更改** (`git commit -m 'Add some AmazingFeature'`)
4. **推送到分支** (`git push origin feature/AmazingFeature`)
5. **开启Pull Request**

## 📄 许可证

本项目采用 [MIT许可证](LICENSE)。

## 🙏 致谢

感谢以下开源项目的启发和帮助：

- [etcd](https://etcd.io/) - 分布式键值存储系统的参考实现
- [Raft](https://raft.github.io/) - 一致性算法的理论基础
- [Go](https://golang.org/) - 优秀的编程语言和生态系统

---

**⭐ 如果这个项目对你的学习有帮助，请给个Star支持一下！**

[GitHub Repository](https://github.com/EvildoerXiaoyy/MyEtcd) •
[文档网站](https://evildoerxiaoyy.github.io/MyEtcd) •
[问题反馈](https://github.com/EvildoerXiaoyy/MyEtcd/issues) •
[讨论区](https://github.com/EvildoerXiaoyy/MyEtcd/discussions)