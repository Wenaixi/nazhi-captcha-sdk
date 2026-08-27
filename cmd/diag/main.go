// diag：并发阶段耗时诊断——找出1.4号/s瓶颈在哪一段
package main

import (
	"fmt"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	captchasdk "captcha-sdk"
)

func main() {
	n := 50
	if len(os.Args) > 1 {
		if v, err := strconv.Atoi(os.Args[1]); err == nil {
			n = v
		}
	}
	solver := captchasdk.Default()
	solver.MaxConcurrent = 0 // 不限，裸测服务器节流

	var (
		fetchNs  atomic.Int64 // 抓图RTT累计
		solveNs  atomic.Int64 // 识别CPU累计
		verifyNs atomic.Int64 // 提交验证累计
		okCount  atomic.Int32
	)
	start := time.Now()
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			t0 := time.Now()
			sid, img, err := solver.FetchCaptcha()
			fetchNs.Add(int64(time.Since(t0)))
			if err != nil {
				return
			}
			t1 := time.Now()
			code := solver.SolveWithSession(sid, img)
			solveNs.Add(int64(time.Since(t1)))
			if code == "" {
				return
			}
			t2 := time.Now()
			ok, _ := solver.Verify(sid, "/uiStudentLogin/validateCaptcha", code)
			verifyNs.Add(int64(time.Since(t2)))
			if ok {
				okCount.Add(1)
			}
		}()
	}
	wg.Wait()
	el := time.Since(start)
	fmt.Printf("n=%d 成功=%d 墙钟=%v\n", n, okCount.Load(), el.Round(time.Millisecond))
	fmt.Printf("平均 抓图=%v 识别=%v 验证=%v  吞吐=%.1f号/s\n",
		time.Duration(fetchNs.Load()/int64(n)).Round(time.Millisecond),
		time.Duration(solveNs.Load()/int64(n)).Round(time.Millisecond),
		time.Duration(verifyNs.Load()/int64(n)).Round(time.Millisecond),
		float64(n)/el.Seconds())
}
