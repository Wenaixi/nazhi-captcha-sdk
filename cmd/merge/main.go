// merge：离线固化——captchasdk-learned.bin（含基础库+训练成果全表）→ data/builtin.bin
package main

import (
	"fmt"
	"os"

	captchasdk "captcha-sdk"
)

func main() {
	src, dst := "captchasdk-learned.bin", "data/builtin.bin"
	b, err := os.ReadFile(src)
	if err != nil {
		fmt.Println("read:", err)
		return
	}
	slots := captchasdk.UnmarshalTablePublic(b)
	if slots == nil {
		fmt.Println("invalid src")
		return
	}
	total := 0
	for s := 0; s < 4; s++ {
		for _, ts := range slots[s] {
			total += len(ts)
		}
	}
	if err := os.WriteFile(dst, b, 0644); err != nil {
		fmt.Println("write:", err)
		return
	}
	fmt.Printf("已固化: %d 条 → %s (%dB)\n", total, dst, len(b))
}
