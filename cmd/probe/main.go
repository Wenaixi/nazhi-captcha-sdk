// probe：miss样本每槽最近距离分布（定位系统性盲区）
package main

import (
	"flag"
	"fmt"

	captchasdk "captcha-sdk"
)

func main() {
	n := flag.Int("n", 200, "样本数")
	flag.Parse()
	solver := captchasdk.New()
	buckets := map[string]int{}
	total := 0
	for i := 0; i < *n; i++ {
		_, img, err := solver.FetchCaptcha()
		if err != nil {
			continue
		}
		dists, ok := solver.ProbeSlotDists(img)
		if !ok {
			buckets["splitFAIL"]++
			total++
			continue
		}
		max := 0
		for _, d := range dists {
			if d > max {
				max = d
			}
		}
		total++
		switch {
		case max <= 90:
			buckets["0-90(HIT)"]++
		case max <= 120:
			buckets["91-120"]++
		case max <= 160:
			buckets["121-160"]++
		default:
			buckets["160+"]++
		}
	}
	fmt.Printf("样本%d 距离分布: %v\n", total, buckets)
}
