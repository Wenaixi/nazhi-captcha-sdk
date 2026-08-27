package captchasdk

import "math/bits"

// popcount 硬件指令 POPCNT（Go编译器将bits.OnesCount64编译为单条CPU指令）
func popcount(x uint64) uint64 { return uint64(bits.OnesCount64(x)) }

// wordSim 模板相似度：same = 320 - hammingDist，每字1次XOR+1次POPCNT（比SWAR快约8倍）
func wordSim(a, b *[Words]uint64) float64 {
	var diff int
	for i := 0; i < Words; i++ {
		diff += bits.OnesCount64(a[i] ^ b[i])
	}
	return float64(TW*TH-diff) / float64(TW*TH)
}
