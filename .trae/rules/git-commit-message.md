---
alwaysApply: true
scene: git_message
---

# Git 提交消息规范

生成 Git 提交消息时必须严格遵守以下规则。

## 输出规则

- 只输出提交消息本身，不要解释
- 默认只生成一行提交消息
- 不要输出多个候选项
- 不要使用英文 summary
- 不要在末尾添加句号
- 第一行不超过 72 个字符

## 基本格式

使用 Conventional Commits 简化格式：

```
<type>(<scope>): <summary>
```

示例：`feat(theme): 优化主题切换动画效果`

## type 规则

只能使用以下 type：

- **feat**：新增功能、增强已有功能
- **fix**：修复问题
- **style**：样式、视觉、格式调整，不影响功能逻辑
- **refactor**：代码重构，不改变功能表现
- **perf**：性能优化
- **docs**：文档修改
- **chore**：配置、依赖、构建、脚本等杂项

## scope 规则

scope 表示"所属功能模块"，不要表示实现细节。

优先使用以下 scope：

- **api**：Miniflux API 客户端
- **aggregator**：聚合逻辑
- **storage**：文件输出
- **config**：配置加载与校验
- **logger**：日志
- **model**：数据模型
- **main**：程序入口
- **docs**：文档
- **build**：构建
- **deps**：依赖

当改动涉及功能模块和实现细节时，优先选择功能模块。

## summary 规则

summary 使用中文，简洁描述本次提交的核心目的。

必须遵守：

- 使用动词开头
- 描述整体目的，不要只描述局部实现
- 优先概括用户可感知的变化
- 不要堆叠过多细节
- 不要使用"解决问题""修改代码""更新逻辑"等笼统表述

推荐动词：新增、优化、修复、调整、重构、移除、更新、统一

## 多行提交规则

只有当改动较多且一行无法清晰表达时才生成正文：

```
<type>(<scope>): <summary>

- 说明一个关键改动
- 说明另一个关键改动
```

## 禁止生成的提交消息

```
fix(api): 解决问题
feat(aggregator): 增加功能
chore(config): 更新代码
```

## 推荐生成的提交消息

```
feat(api): 支持自定义 HTTP User-Agent
fix(aggregator): 修复分类为空时空指针异常
refactor(storage): 重构 JSON 输出逻辑
perf(api): 优化并发请求调度策略
chore(deps): 升级 miniflux SDK 到 v2.3.1
```