# VShortLink - 高性能短链接服务

## 项目概述

VShortLink是一个高性能的短链接服务，支持不同长度短链码的生成、管理和访问。系统采用预生成短链码池的方式，提高短链创建的响应速度，并实现了短链码的回收与重用机制。

## 核心特性

- **多长度短链码支持**：支持4位、5位、6位等不同长度的短链码，满足不同场景需求
- **预生成短链码池**：通过批量预生成短链码并存储在Redis中，提高创建短链的响应速度
- **短链码回收机制**：支持短链码的回收与重用，高效利用有限的短链资源
- **高性能设计**：采用多级缓存策略，确保短链访问的高性能和低延迟
- **分布式支持**：基于Redis的分布式设计，支持水平扩展

## 技术架构

- **存储层**：
  - MySQL/PostgreSQL：存储短链基本信息和状态
  - Redis：存储短链码池和短链映射关系
  - 内存缓存：提供最快的查询响应

- **核心算法**：
  - 基于62进制的短链码生成算法
  - 批量生成与索引管理机制
  - 短链码回收与冷却期设计

## 快速开始

### 安装依赖

```bash
go mod tidy
```

### 运行示例

```bash
go run examples/generator_example.go
```

## 核心包说明

### cores包

`cores`包定义了短链接系统的核心常量和工具函数：

- `shortlink.go`：定义短链码的基本参数和转换函数
- `generator.go`：实现短链码的生成、获取和回收功能

### 使用示例

```go
// 创建短链码生成器
generator := cores.NewShortLinkGenerator(redisClient)

// 生成一批4位短链码
err = generator.GenerateShortCodes(ctx, cores.ShortLink4)

// 获取一个4位短链码
shortCode, err := generator.GetShortCode(ctx, cores.ShortLink4)

// 回收短链码
err = generator.RecycleShortCode(ctx, cores.ShortLink4, shortCode, 60) // 60秒冷却期
```

## 设计文档

详细的设计文档请参考 [设计文档](doc/design.md)。

## 许可证

本项目采用 Apache License 2.0 许可证。详情请参阅 [LICENSE](LICENSE) 文件。