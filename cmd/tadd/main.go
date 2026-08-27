// tadd：AddTemplate单点验证
package main

import (
	"fmt"

	captchasdk "captcha-sdk"
)

func main() {
	solver := captchasdk.New()
	fmt.Println("before:", solver.TotalTemplates())
	sid, img, err := solver.FetchCaptcha()
	if err != nil {
		fmt.Println("fetch:", err)
		return
	}
	code := solver.SolveWithSession(sid, img)
	fmt.Println("code:", code)
	if code != "" {
		ok := solver.AddTemplate(img, code)
		fmt.Println("added:", ok, "after:", solver.TotalTemplates())
	}
}
