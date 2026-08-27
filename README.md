# 纳智验证码破解 SDK（captcha-sdk）

Go SDK 一行接入，**内置预训练库，无需训练**，开箱即用。

## 接入方式

\`\`\`go
import captchasdk "captcha-sdk"

// 一行创建（内置189条预训练模板，learning=true开启自学习）
solver := captchasdk.New(true)

// 一站式破解：抓图→破解→提交验证→返回可用验证码
sid, code := solver.Solve("/uiStudentLogin/validateCaptcha")

// 进阶：自管理SESSION
sid, img, _ := solver.FetchCaptcha()
code := solver.SolveWithSession(sid, img)
ok, _ := solver.Verify(sid, "/uiLogin/validateCaptcha", code) // 各角色端点通用
\`\`\`

## 性能（v8.1，i9-12900HX 实测）

| 指标 | v8 | v8.1 | 变化 |
|---|---|---|---|
| wordSim 单次 | 1.55ns | 1.42ns | -8% |
| FullMatch 查表(4槽) | 390ns | 72ns | **-82%** |
| rank 候选计算 | 7.6µs 912B/28alloc | 2.9µs 0B/0alloc | **-62% / 零分配** |
| 端到端 FastOnly | 116-161ms | **101-105ms** | 全中 |
| 端到端 标准 | 2.3s | 526ms-3.7s | RTT主导 |
| 低配 GOMAXPROCS=2 | 161ms | **244ms** | 网络RTT主导 |
| 内置库体积 | 8.1KB | 8.1KB | 189模板 |

v8.1 优化点：wordSim 整数化、match 代表模板缓存、rank 栈数组零分配、
IdxToCode 预计算、请求体/响应缓冲池化、竞速击中硬终止在途请求。

## 架构

\`\`\`
sdk.go          对外API（New/Solve/SolveWithSession/Verify/FetchCaptcha）
embedded.go     go:embed 内置预训练库（data/builtin.bin）
popcount.go     POPCNT硬件指令相似度（整数汉明距离，零分配）
image.go        灰度+槽位分割+320bit位图
table.go        运行时模板表（match代表缓存/rank零分配/add自学习）
types.go        二进制序列化（40B/模板 vs JSON 124B，-67%）
cmd/gen/        table.json → builtin.bin 生成器
cmd/demo/       接入示例
\`\`\`

## 支持端点（全角色共用同一验证码池）

- 学生 /uiStudentLogin/validateCaptcha
- 教师 /uiLogin/validateCaptcha
- 管理员 /sysManagerLogin/validateCaptcha
- 区县/市级同理

## 重新生成内置库

\`\`\`bash
make gen   # 从 ../captcha-cracker-v6/data/table.json 生成
\`\`\`
