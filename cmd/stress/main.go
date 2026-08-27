// stress：1000账号并发识别压测（模拟1000账号同时登录场景）
// 用法：stress.exe [并发账号数] （默认1000）
package main

import (
	"fmt"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	captchasdk "nazhi-captcha-sdk"
)

func main() {
	n := 1000
	if len(os.Args) > 1 {
		if v, err := strconv.Atoi(os.Args[1]); err == nil {
			n = v
		}
	}

	// 共享单例：模板表/连接池/预热全共享，内存1份
	solver := captchasdk.Default()
	solver.MaxConcurrent = 25 // 目标服务器对并发SESSION有节流，25路在途实测稳定

	fmt.Printf("并发账号: %d  在途闸门: %d  模板: %d\n", n, solver.MaxConcurrent, solver.TotalTemplates())

	var (
		okCount atomic.Int32
		totalNs atomic.Int64
		maxNs   atomic.Int64
	)
	start := time.Now()
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			t0 := time.Now()
			_, code := solver.Solve("/uiStudentLogin/validateCaptcha")
			d := time.Since(t0)
			totalNs.Add(int64(d))
			for {
				old := maxNs.Load()
				if int64(d) <= old || maxNs.CompareAndSwap(old, int64(d)) {
					break
				}
			}
			if code != "" {
				okCount.Add(1)
			}
		}()
	}
	wg.Wait()
	el := time.Since(start)

	fmt.Printf("完成: %d/%d 成功  墙钟: %v\n", okCount.Load(), n, el.Round(time.Millisecond))
	fmt.Printf("平均单号: %v  最慢单号: %v  吞吐: %.1f 号/秒\n",
		(time.Duration(totalNs.Load() / int64(n))).Round(time.Millisecond),
		time.Duration(maxNs.Load()).Round(time.Millisecond),
		float64(n)/el.Seconds())
}
