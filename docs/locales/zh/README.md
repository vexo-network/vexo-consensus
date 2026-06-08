# Vexo 文档

本目录是 Vexo 文档的中文入口。英文 (`en`) 文档是规范来源；中文目录保持相同结构，方便开发者和运维人员按主题查找。

## 建议阅读顺序

1. [共识协议概览](./consensus-protocol.md)
2. [共识规范](./specs/consensus-spec.md)
3. [交易格式](./specs/tx-format.md)
4. [验证者生命周期](./specs/validator-lifecycle.md)
5. [安全审计准备](./security/audit-readiness.md)

## 文档集合

| 类别 | 路径 | 说明 |
|---|---|---|
| 运维 | `operators/` | 节点初始化、添加验证者、配置文件管理 |
| 发布 | `release/` | 发布流水线、运行手册、兼容性和发布门禁 |
| SDK | `sdk/` | 应用模块、自定义 crypto/storage/transport、RPC 版本管理 |
| 安全 | `security/` | 威胁模型、假设和审计准备 |
| 规范 | `specs/` | 共识、网络、存储、交易和 finality proof |

命令、JSON 字段、RPC 方法和代码标识符保持英文原样。
