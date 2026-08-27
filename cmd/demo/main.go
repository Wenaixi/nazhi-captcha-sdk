// demo：SDK接入示例（含三种模式实测）
package main

import (
	"fmt"
	"time"

	captchasdk "captcha-sdk"
)

func main() {
	// 模式1：标准模式（验证后返回，100%可靠）
	solver := captchasdk.New(true)
	fmt.Printf("内置模板: %d 条\n", solver.TotalTemplates())

	endpoint := "/uiStudentLogin/validateCaptcha"

	t0 := time.Now()
	sid, code := solver.Solve(endpoint)
	el := time.Since(t0)
	if code != "" {
		fmt.Printf("[标准] code=%s session=%s... 耗时=%v\n", code, sid[:8], el.Round(time.Millisecond))
	} else {
		fmt.Println("[标准] 破解失败")
	}

	// 模式2：FastOnly（查表直返，25ms，省1次RTT）
	fast := captchasdk.New(true)
	fast.FastOnly = true
	t1 := time.Now()
	sid2, code2 := fast.Solve(endpoint)
	el2 := time.Since(t1)
	if code2 != "" {
		fmt.Printf("[FastOnly] code=%s session=%s... 耗时=%v\n", code2, sid2[:8], el2.Round(time.Millisecond))
	} else {
		fmt.Println("[FastOnly] 查表未命中(回退标准模式内部处理)")
	}

	// 自学习持久化
	if err := solver.SaveTable(""); err == nil {
		fmt.Printf("[自学习] 已落盘 captchasdk-learned.bin 新增=%d 条\n", solver.LearnedCount())
	}
}
