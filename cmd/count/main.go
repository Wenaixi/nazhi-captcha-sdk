// count：统计bin库条目（一次性工具）
package main

import (
	"fmt"
	"os"

	captchasdk "nazhi-captcha-sdk"
)

func main() {
	path := "data/builtin.bin"
	if len(os.Args) > 1 {
		path = os.Args[1]
	}
	b, err := os.ReadFile(path)
	if err != nil {
		fmt.Println(err)
		return
	}
	slots := captchasdk.UnmarshalTablePublic(b)
	if slots == nil {
		fmt.Println("invalid")
		return
	}
	total := 0
	perSlot := [4]int{}
	for s := 0; s < 4; s++ {
		for _, ts := range slots[s] {
			perSlot[s] += len(ts)
			total += len(ts)
		}
	}
	fmt.Printf("总条目=%d 每槽=%v 文件=%dB\n", total, perSlot, len(b))
}
