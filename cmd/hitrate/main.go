// hitrate：纯match命中率统计（可调并发）
package main

import (
	"flag"
	"fmt"
	"sync"
	"sync/atomic"

	captchasdk "nazhi-captcha-sdk"
)

func main() {
	n := flag.Int("n", 100, "样本数")
	c := flag.Int("c", 1, "并发数")
	flag.Parse()

	solver := captchasdk.New()
	solver.MaxConcurrent = *c
	var hit, splitFail atomic.Int32
	var wg sync.WaitGroup
	sem := make(chan struct{}, *c)
	for i := 0; i < *n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			_, img, err := solver.FetchCaptcha()
			if err != nil {
				return
			}
			words, ok := captchasdk.ImageWordsPublic(img)
			if !ok {
				splitFail.Add(1)
				return
			}
			if _, ok := solver.TblPublic().MatchPublic(&words); ok {
				hit.Add(1)
			}
		}()
	}
	wg.Wait()
	fmt.Printf("hit:%d/%d (%.0f%%) splitFail:%d conc:%d", hit.Load(), *n, float64(hit.Load())/float64(*n)*100, splitFail.Load(), *c)
}
