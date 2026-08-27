// demo：SDK接入示例（零训练零配置直接用）
package main

import (
	"fmt"
	"time"

	captchasdk "nazhi-captcha-sdk"
)

func main() {
	// 推荐用法：Default单例（306条预训练库+FastOnly极速模式出厂默认开启）
	solver := captchasdk.Default()
	fmt.Printf("内置模板: %d 条  FastOnly: %v\n", solver.TotalTemplates(), solver.FastOnly)

	endpoint := "/uiStudentLogin/validateCaptcha"

	// 极速模式：查表命中直返（省1次验证RTT）
	t0 := time.Now()
	sid, code := solver.Solve(endpoint)
	el := time.Since(t0)
	if code != "" {
		fmt.Printf("[极速] code=%s session=%s... 耗时=%v\n", code, sid[:8], el.Round(time.Millisecond))
	} else {
		fmt.Println("[极速] 查表未命中（自动回退rank/full兜底）")
	}

	// 标准模式（验证后返回，100%可靠）
	plain := captchasdk.New()
	plain.FastOnly = false
	t1 := time.Now()
	sid2, code2 := plain.Solve(endpoint)
	el2 := time.Since(t1)
	if code2 != "" {
		fmt.Printf("[标准] code=%s session=%s... 耗时=%v\n", code2, sid2[:8], el2.Round(time.Millisecond))
	} else {
		fmt.Println("[标准] 破解失败")
	}
}
