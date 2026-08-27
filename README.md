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

## 性能

| 指标 | 值 |
|---|---|
| 通过率 | 100%（三级漏斗兜底） |
| fast 查表 | 25-68ms |
| rank 81组合 | 94-121ms |
| full 梯度兜底 | 3-4s |
| 内存 | ~13MB 堆 |
| 内置库体积 | 8.1KB 二进制（189模板） |

## 架构

\`\`\`
sdk.go          对外API（New/Solve/SolveWithSession/Verify/FetchCaptcha）
embedded.go     go:embed 内置预训练库（data/builtin.bin）
popcount.go     SWAR位运算相似度（零分配）
image.go        灰度+槽位分割+320bit位图
table.go        运行时模板表（match/rank/add自学习）
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
