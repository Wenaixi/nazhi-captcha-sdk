// loop：自动训练闭环——训练→固化→测命中率→不足继续，直到>=目标命中率
// 用法：loop.exe [-target 99] [-maxround 5] [-n 900] [-c 10]
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

func main() {
	target := flag.Float64("target", 99, "目标命中率%%")
	maxRound := flag.Int("maxround", 5, "最大轮数")
	n := flag.Int("n", 900, "每轮样本数")
	conc := flag.Int("c", 10, "并发数")
	flag.Parse()

	workdir := "."
	for round := 1; round <= *maxRound; round++ {
		fmt.Printf("\n===== 第%d轮训练 =====\n", round)
		// 训练：当前builtin为基础，增量累积
		run(workdir, "./train.exe", "-n", strconv.Itoa(*n), "-c", strconv.Itoa(*conc),
			"-lib", "data/builtin.bin", "-out", "captchasdk-learned.bin")
		// 固化
		run(workdir, "./merge.exe")
		// 测命中率
		rate := runHitrate(workdir, 100)
		fmt.Printf("本轮命中率: %.1f%%\n", rate)
		if rate >= *target {
			fmt.Printf("达标！命中率 %.1f%% >= %.0f%%\n", rate, *target)
			return
		}
	}
	fmt.Println("达到最大轮数，命中率未达标——继续增大 -maxround 或样本数")
}

func run(dir, name string, args ...string) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Println("run error:", err)
		os.Exit(1)
	}
}

func runHitrate(dir string, n int) float64 {
	out, err := exec.Command("./hitrate.exe", strconv.Itoa(n)).Output()
	if err != nil {
		fmt.Println("hitrate err:", err)
		return 0
	}
	line := string(out)
	// 解析 "样本100: match命中68 (68%)"
	fields := strings.Fields(line)
	for _, f := range fields {
		if strings.HasSuffix(f, "%)") {
			v := strings.TrimSuffix(strings.TrimPrefix(f, "("), "%)")
			if fv, err := strconv.ParseFloat(v, 64); err == nil {
				return fv
			}
		}
	}
	_ = runtime.GOOS
	return 0
}
