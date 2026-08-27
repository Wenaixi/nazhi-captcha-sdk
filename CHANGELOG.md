# 更新记录

本项目更新记录遵循 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/) 格式，
版本号遵循 [语义化版本](https://semver.org/lang/zh-CN/)。

## [未发布]

## [0.2.1] - 2026-08-28

发布链接：[v0.2.1](https://github.com/Wenaixi/nazhi-captcha-sdk/releases/tag/v0.2.1)

### 修复

- README 接入示例的 import 路径同步为 `github.com/Wenaixi/nazhi-captcha-sdk`（v0.2.0 模块改名时遗漏，旧路径无法编译）

### 变更

- CI gofmt 检查改为失败式断言（此前仅列文件不拦截）；发布产物补充 CHANGELOG.md；移除零依赖项目无意义的 Go 模块缓存配置

## [0.2.0] - 2026-08-27

发布链接：[v0.2.0](https://github.com/Wenaixi/nazhi-captcha-sdk/releases/tag/v0.2.0)

### 新增

- `MatchImage` 包级纯本地查表识别 API（零网络/零连接池/零预热），供 nazhi-cli 内置验证码识别器接入

### 变更

- 模块名改为 `github.com/Wenaixi/nazhi-captcha-sdk`：满足 Go 模块规范（带点域名前缀），支持 nazhi-cli/nazhi-auto CI 远程拉取，import 路径同步更新

## [0.1.0] - 2026-08-27

发布链接：[v0.1.0](https://github.com/Wenaixi/nazhi-captcha-sdk/releases/tag/v0.1.0)

### 新增

- 初始化项目：Go SDK 一行接入验证码破解，内置预训练模板库，零训练零配置
- 三级破解漏斗：全表 O(1) 匹配 → rank 81 组合 → full 梯度兜底
- 图像预处理：灰度化 + 孤立点去噪 + 列投影分割 + 320bit 位图化
- POPCNT 硬件指令汉明距离匹配，热路径零堆分配
- 进程级共享单例 `Default()`：模板表/连接池/预热全共享，支持 1000+ goroutine
- 并发安全：RWMutex 读锁 + 网络路径闸门（match 零闸门）
- 离线训练工具链：`train`（参数化/增量/空库重建）+ `merge`（固化）+ `hitrate`（质量门禁）
- 诊断与压测工具：`stress`（1000 账号并发）、`count`（库统计）、`probe`（距离诊断）
- GitHub Actions CI：push 全量测试（gofmt/vet/race/编译），tag 构建 5 平台发布产物并自动发 Release
- MIT 开源许可

### 变更

- match 从"每字符前 2 代表"改为全表线性扫描：命中率 72% → 100%，O(1) 级破解
- 移除运行时自学习（零训练诉求），`AddTemplate` 仅离线训练工具使用
- 模板库扩容：189 → 5364 条（全去噪 pipeline 空库重建）
- 项目更名为 `nazhi-captcha-sdk`（模块名/路径/文档全量统一）
- 清理一次性诊断工具（diag/probe/tadd/loop），保留正式工具链

### 修复

- rankEnum 字符不在字符池时的负索引 panic（防御跳过）
- 并发闸门移出 match 路径：match 零等待，仅 rank/full 网络兜底限流

### 性能

- 单发端到端（FastOnly）：~170ms 恒定（1 次 GET + 微秒级匹配）
- match 命中率：~100%（200/200 实测）
- 100 账号并发：100% 成功，~300 号/秒
- 1000 账号并发：1000/1000 成功，数秒完成，~300 号/秒
- 匹配耗时：~50µs / 0 alloc（5364 条全表 × POPCNT）
