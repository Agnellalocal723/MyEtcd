# 贡献指南

感谢您对MyEtcd项目的关注！我们欢迎各种形式的贡献，包括但不限于代码、文档、测试、问题反馈和建议。

## 🤝 如何贡献

### 报告问题

如果您发现了bug或有功能建议，请：

1. 检查[现有Issues](https://github.com/EvildoerXiaoyy/MyEtcd/issues)确认问题未被报告
2. 如果是新问题，请创建一个Issue并使用相应的模板
3. 提供详细的问题描述、重现步骤和环境信息

### 提交代码

1. **Fork项目**
   ```bash
   # 在GitHub上fork项目到您的账户
   # 然后克隆您的fork
   git clone https://github.com/YOUR_USERNAME/MyEtcd.git
   cd MyEtcd
   ```

2. **创建分支**
   ```bash
   # 创建一个新的特性分支
   git checkout -b feature/your-feature-name
   
   # 或者修复bug
   git checkout -b fix/bug-description
   ```

3. **进行开发**
   - 遵循项目的代码规范
   - 添加必要的测试
   - 更新相关文档
   - 确保所有测试通过

4. **提交更改**
   ```bash
   # 添加更改
   git add .
   
   # 提交（使用清晰的提交信息）
   git commit -m "feat: add new feature description"
   ```

5. **推送并创建PR**
   ```bash
   # 推送到您的fork
   git push origin feature/your-feature-name
   
   # 在GitHub上创建Pull Request
   ```

## 📝 开发规范

### 代码风格

- **Go代码规范**: 遵循[Go官方代码规范](https://golang.org/doc/effective_go.html)
- **格式化**: 使用`gofmt`格式化代码
- **注释**: 为公共函数和复杂逻辑添加详细的中文注释
- **命名**: 使用清晰、描述性的变量和函数名

### 提交信息规范

使用[Conventional Commits](https://www.conventionalcommits.org/)规范：

```
<type>[optional scope]: <description>

[optional body]

[optional footer(s)]
```

类型说明：
- `feat`: 新功能
- `fix`: 修复bug
- `docs`: 文档更新
- `style`: 代码格式化（不影响功能）
- `refactor`: 重构代码
- `test`: 添加或修改测试
- `chore`: 构建过程或辅助工具的变动

示例：
```
feat(watch): add prefix filtering support for watch mechanism

- Add prefix filtering in watcher.go
- Update API endpoint to support prefix parameter
- Add unit tests for new functionality

Closes #123
```

### 测试要求

- **单元测试**: 新功能必须包含单元测试
- **集成测试**: 重要功能需要集成测试
- **测试覆盖率**: 保持测试覆盖率在80%以上
- **性能测试**: 性能敏感的代码需要基准测试

运行测试：
```bash
# 运行所有测试
make test

# 运行特定测试
go test -v ./tests/...

# 运行基准测试
make benchmark

# 查看测试覆盖率
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

## 🏗️ 项目结构

```
MyEtcd/
├── cmd/                 # 应用程序入口
├── internal/            # 内部包
│   ├── api/            # HTTP API
│   ├── storage/        # 存储引擎
│   ├── wal/            # 预写日志
│   ├── watch/          # Watch机制
│   ├── lease/          # 租约系统
│   ├── raft/           # Raft算法
│   ├── metrics/        # 监控指标
│   ├── config/         # 配置管理
│   ├── logger/         # 日志系统
│   └── benchmark/      # 基准测试
├── tests/              # 测试文件
├── examples/           # 示例代码
├── docs/               # 项目文档
├── config/             # 配置文件
└── .github/            # GitHub配置
```

## 🔧 开发环境设置

### 前置要求

- Go 1.19+
- Docker (可选)
- Git

### 设置步骤

1. **克隆项目**
   ```bash
   git clone https://github.com/EvildoerXiaoyy/MyEtcd.git
   cd MyEtcd
   ```

2. **安装依赖**
   ```bash
   go mod download
   ```

3. **运行测试**
   ```bash
   make test
   ```

4. **构建项目**
   ```bash
   make build
   ```

### 开发工具

推荐安装以下工具以提高开发效率：

```bash
# 代码格式化工具
go install golang.org/x/tools/cmd/goimports@latest

# 静态分析工具
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# 复杂度分析工具
go install github.com/fzipp/gocyclo/cmd/gocyclo@latest

# 文档工具
go install github.com/swaggo/swag/cmd/swag@latest
```

## 📚 文档贡献

### 文档类型

- **API文档**: 接口说明和使用示例
- **开发文档**: 架构设计和实现原理
- **用户文档**: 安装、配置和使用指南
- **示例代码**: 各种使用场景的示例

### 文档规范

- 使用Markdown格式
- 提供清晰的标题层次
- 包含代码示例和命令行示例
- 添加适当的图表和截图

## 🐛 问题分类

### Bug优先级

- **P0 - 严重**: 系统崩溃、数据丢失、安全漏洞
- **P1 - 高**: 主要功能无法使用、性能严重下降
- **P2 - 中**: 功能异常但有替代方案
- **P3 - 低**: 用户体验问题、文档错误

### 标签说明

- `bug`: Bug报告
- `enhancement`: 功能增强
- `good first issue`: 适合新手的问题
- `help wanted`: 需要帮助的问题
- `documentation`: 文档相关
- `performance`: 性能相关
- `security`: 安全相关
- `testing`: 测试相关

## 🚀 发布流程

### 版本管理

项目使用[语义化版本](https://semver.org/)：

- `MAJOR.MINOR.PATCH`
- `MAJOR`: 不兼容的API变更
- `MINOR`: 向后兼容的功能新增
- `PATCH`: 向后兼容的问题修正

### 发布检查清单

- [ ] 所有测试通过
- [ ] 文档已更新
- [ ] CHANGELOG已更新
- [ ] 版本号已更新
- [ ] Docker镜像已构建
- [ ] 性能测试通过
- [ ] 安全扫描通过

## 💡 贡献建议

### 新手贡献

1. 从`good first issue`标签的问题开始
2. 修复文档中的拼写错误或格式问题
3. 改进错误消息
4. 添加更多的测试用例

### 高级贡献

1. 实现新功能
2. 性能优化
3. 架构改进
4. 安全增强

### 学习资源

- [Go官方文档](https://golang.org/doc/)
- [etcd源码](https://github.com/etcd-io/etcd)
- [Raft论文](https://raft.github.io/raft.pdf)
- [分布式系统设计](https://book.douban.com/subject/26380823/)

## 📞 联系方式

- **GitHub Issues**: [提交问题](https://github.com/EvildoerXiaoyy/MyEtcd/issues)
- **GitHub Discussions**: [参与讨论](https://github.com/EvildoerXiaoyy/MyEtcd/discussions)
- **Pull Requests**: [贡献代码](https://github.com/EvildoerXiaoyy/MyEtcd/pulls)

## 🙏 致谢

感谢所有为MyEtcd项目做出贡献的开发者！

---

**再次感谢您的贡献！每一个贡献都让这个项目变得更好。🎉**