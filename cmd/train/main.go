// train：超大参数离线预训练工具
// 用法：train.exe [-n 样本数] [-c 并发] [-lib 基础库路径] [-out 输出库] [-empty]
//
//	默认：train.exe -n 900 -c 10 -lib data/builtin.bin -out captchasdk-learned.bin
//
// 增量训练：lib传已有库，新样本在基础上继续累积（重复跑即在已有基础上继续训练）
// -empty：从空库开始完全干净重建（新pipeline专用）
package main

import (
	"flag"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	captchasdk "nazhi-captcha-sdk"
)

func main() {
	n := flag.Int("n", 900, "样本数")
	conc := flag.Int("c", 10, "并发数")
	lib := flag.String("lib", "data/builtin.bin", "基础库路径（-empty时忽略）")
	out := flag.String("out", "captchasdk-learned.bin", "输出库路径")
	empty := flag.Bool("empty", false, "从空库开始（完全干净重建）")
	flag.Parse()

	solver := captchasdk.New()
	solver.MaxConcurrent = *conc

	baseTotal := 0
	mode := "增量"
	if !*empty {
		if err := solver.LoadTable(*lib); err != nil {
			fmt.Println("加载基础库:", err)
		}
		baseTotal = solver.TotalTemplates()
	} else {
		mode = "空库重建"
	}
	fmt.Printf("基础库: %d 条  样本: %d  并发: %d  模式: %s\n", baseTotal, *n, *conc, mode)

	var okCount, addOk atomic.Int32
	start := time.Now()
	var wg sync.WaitGroup
	for i := 0; i < *n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sid, img, err := solver.FetchCaptcha()
			if err != nil {
				return
			}
			code := solver.SolveWithSession(sid, img)
			if code != "" {
				okCount.Add(1)
				if solver.AddTemplate(img, code) {
					addOk.Add(1)
				}
			}
		}()
	}
	wg.Wait()
	el := time.Since(start)

	fmt.Printf("训练: 破解%d/%d 入库%d  耗时%v  吞吐%.1f/s\n", okCount.Load(), *n, addOk.Load(), el.Round(time.Second), float64(*n)/el.Seconds())
	fmt.Printf("库规模: %d → %d 条\n", baseTotal, solver.TotalTemplates())
	if err := solver.SaveTable(*out); err == nil {
		fmt.Printf("已落盘 %s（用 merge.exe 固化为 builtin）\n", *out)
	}
}
