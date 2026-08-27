# 纳智验证码破解 SDK（nazhi-captcha-sdk）

Go SDK 一行接入，**出厂预训练、零学习零训练零配置、O(1)级破解**，开箱即用。
内置模板库为离线批量训练产物，覆盖目标站点全量字形空间，运行时不做任何学习。

## 特性

- 零依赖：仅标准库，`go get` 即用
- 零训练：`go:embed` 内置预训练模板库，无需任何训练步骤
- O(1) 破解：全表线性扫描 + POPCNT 硬件指令，纯 CPU 微秒级匹配
- 高并发：RWMutex 读锁共享 + 进程级单例，模板表/连接池/预热全共享
- 极速模式：查表命中直接返回，无 rank/full 网络兜底风暴
- 低内存：热路径零堆分配（位图池化、请求体/响应缓冲池化、栈数组）
- 灰度发布友好：`SetBase` 可切换目标站点，`LoadTable` 可热升级模板库

## 接入方式

```go
import captchasdk "nazhi-captcha-sdk"

// 推荐：进程级共享单例（预训练库 + FastOnly 极速模式出厂默认开启）
solver := captchasdk.Default()

// 一站式破解：抓图→查表→返回可用验证码
sid, code := solver.Solve("/uiStudentLogin/validateCaptcha")

// 高并发（批量登录）：共享单例天然支持 1000+ goroutine
solver.MaxConcurrent = 25 // 可选闸门：限制网络在途请求数，防目标服务器限流

// 进阶：自管理 SESSION
sid, img, _ := solver.FetchCaptcha()
code := solver.SolveWithSession(sid, img)
ok, _ := solver.Verify(sid, "/uiLogin/validateCaptcha", code) // 各角色端点通用

// 独立实例（每个实例独立连接池，一般用不到）
s := captchasdk.New()
```

## API

| 方法 | 说明 |
| --- | --- |
| `New() *Solver` | 创建独立实例（内置预训练库） |
| `Default() *Solver` | 进程级共享单例（推荐，高并发零额外开销） |
| `Solve(endpoint) (sid, code)` | 一站式：抓图→破解→返回验证码（FastOnly 时跳过验证） |
| `SolveWithSession(sid, img) code` | 对已有图片破解（不提交验证） |
| `FetchCaptcha() (sid, img, err)` | 获取新 SESSION + 验证码图 |
| `Verify(sid, endpoint, code) (ok, err)` | 提交验证码到指定端点 |
| `SetBase(url)` | 切换目标站点（测试/镜像环境） |
| `LoadTable(path)` | 外挂模板库（与内置合并，热升级） |
| `SaveTable(path)` | 落盘当前模板表（供离线固化为新内置库） |
| `TotalTemplates() int` | 当前模板总数 |
| `AddTemplate(img, code) bool` | 显式入库样本（仅离线训练工具使用） |

### Solver 字段

- `FastOnly bool`：极速模式，查表命中直接返回（省 1 次验证 RTT），默认由 `Default()` 开启
- `MaxConcurrent int`：网络在途请求闸门，0=不限；批量登录建议 10-30

## 性能（实测，具体数值因硬件/网络而异）

| 指标 | 典型值 |
| --- | --- |
| 单发端到端（FastOnly） | ~170ms（1 次 GET + 微秒级匹配） |
| 匹配耗时 | ~50µs / 0 alloc（全表线性扫描 × POPCNT） |
| match 命中率 | ~100%（全表扫描 100% 利用训练库） |
| 100 账号并发 | 100% 成功，~300 号/秒 |
| 1000 账号并发 | 100% 成功，~300 号/秒，数秒完成 |
| 内存 | 热路径零堆分配，常驻 <10MB |

复现方法：`cmd/stress` 压测（1000 账号并发）、`cmd/hitrate` 验命中率（默认 100 样本）。

## 原理

1. 抓图：GET `/kaptcha/kaptcha.jpg`，返回 SESSION cookie + 验证码图片
2. 预处理：灰度化 → 孤立点去噪（8 邻域全亮暗点清除）→ 列投影分割 4 槽
3. 位图化：每槽归一化采样为 320bit（5×uint64）
4. 匹配：与预训练库全表比较汉明距离（POPCNT 硬件指令），取每槽最近字符
5. 命中即返回验证码；未命中回退 rank（top3^4=81 组合并发试）→ full（梯度全量兜底）

## 架构

```
sdk.go          对外 API（Solver 生命周期/并发闸门/三级漏斗）
embedded.go     go:embed 内置预训练库（data/builtin.bin）
popcount.go     POPCNT 硬件指令相似度（整数汉明距离，零分配）
image.go        灰度+去噪+槽位分割+320bit 位图
table.go        模板表（RWMutex 并发安全/全表扫描 match/rank）
types.go        二进制序列化（紧凑 40B/模板）
cmd/gen/        v6 json 模板 → builtin.bin 生成器
cmd/train/      离线预训练（参数化：样本/并发/增量/空库重建）
cmd/merge/      训练产物固化 → data/builtin.bin
cmd/hitrate/    match 命中率验证（质量门禁）
cmd/count/      模板库条目统计
cmd/stress/     并发压测（默认 1000 账号）
cmd/demo/       接入示例
```

## 模板库训练与升级（唯一需要"训练"的场景）

SDK 运行零训练；只有升级内置库时才需要离线训练（一次性，产物固化后分发）：

```bash
# 1. 增量训练（在现有库基础上继续，从 data/builtin.bin 起步）
go run ./cmd/train -n 900 -c 10 -lib data/builtin.bin -out learned.bin

# 2. 固化：learned.bin → data/builtin.bin（重新编译 SDK 即内置新库）
go run ./cmd/merge

# 3. 质量门禁：命中率应 ~100%
go run ./cmd/hitrate -n 200

# 4.（可选）从零重建（新预处理管线时使用）
go run ./cmd/train -empty -n 900 -c 10 -out learned.bin
```

## 支持端点（全角色共用同一验证码池）

- 学生 `/uiStudentLogin/validateCaptcha`
- 教师 `/uiLogin/validateCaptcha`
- 管理员 `/sysManagerLogin/validateCaptcha`
- 区县/市级同理

## 构建

```bash
make build   # 编译全部工具
make test    # 单元测试
make bench   # 性能基准
make demo    # 运行接入示例
make clean   # 清理产物
```

## CI/CD

- **push / PR**：自动跑全量测试（gofmt 检查 + go vet + 竞态检测单测 + 全工具编译）
- **tag（v*）**：全量测试通过后构建发布产物（Windows/Linux/macOS × amd64/arm64 共 5 平台）
  并自动发布到 GitHub Release，产物内含全部工具二进制 + 内置模板库 + 文档
## 许可

MIT License，详见 LICENSE。
