# 纳智验证码破解 SDK（captcha-sdk）

Go SDK 一行接入，**5364条模板出厂预训练，零学习零训练零配置，O(1)级破解**，开箱即用。

## 接入方式

\`\`\`go
import captchasdk "captcha-sdk"

// 推荐：进程级共享单例（5364条预训练库+FastOnly极速模式出厂默认开启）
solver := captchasdk.Default()

// 高并发（批量登录）：共享单例天然支持1000 goroutine，模板表/连接池全共享
solver.MaxConcurrent = 25 // 闸门可选；目标服务器对SESSION发放有节流，实测25路最优

// 一站式破解：抓图→破解→提交验证→返回可用验证码
sid, code := solver.Solve("/uiStudentLogin/validateCaptcha")

// 进阶：自管理SESSION
sid, img, _ := solver.FetchCaptcha()
code := solver.SolveWithSession(sid, img)
ok, _ := solver.Verify(sid, "/uiLogin/validateCaptcha", code) // 各角色端点通用
\`\`\`

## 性能（v10，i9-12900HX 实测）

| 指标 | 值 |
|---|---|
| 内置库 | **5364条预训练模板**（173KB二进制，全表O(1)扫描） |
| match命中率 | **100%（200/200，全表线性扫描）** |
| FastOnly 端到端 | **164-184ms（6/6全中，恒定）** |
| FullMatch 查表 | ~54µs / 0 alloc（5364条全表×POPCNT硬件指令） |
| **100账号并发** | **100%成功，3.5秒完成，282号/秒** |
| **1000账号并发** | **1000/1000成功，3.3秒完成，302号/秒** |
| 并发安全 | RWMutex：match读锁1000 goroutine无阻塞 |

O(1)实现：match从"每字符前2代表"改为全表线性扫描——5364条×5×POPCNT≈54µs纯CPU，
命中率从72%→100%（前2代表缓存漏掉了稀有字形，全表扫描100%利用训练库）。
FastOnly命中即返回，无rank/full网络风暴。1000账号=3.3秒。

## 架构

\`\`\`
sdk.go          对外API（New/Solve/SolveWithSession/Verify/FetchCaptcha）
embedded.go     go:embed 内置预训练库（data/builtin.bin）
popcount.go     POPCNT硬件指令相似度（整数汉明距离，零分配）
image.go        灰度+槽位分割+320bit位图
table.go        运行时模板表（RWMutex并发安全/前2代表match/rank零分配）
types.go        二进制序列化（40B/模板 vs JSON 124B，-67%）
cmd/gen/        table.json → builtin.bin 生成器
cmd/merge/      离线固化工具：learned.bin + 原库去重合并 → builtin.bin
cmd/count/      模板库条目统计工具
cmd/stress/     1000账号并发压测
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
