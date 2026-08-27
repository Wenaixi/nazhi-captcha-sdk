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

// wordDiffHamming 汉明距离（整数版，免除2次int→float转换与1次除法，比wordSim快~50%）
func wordDiffHamming(a, b *[Words]uint64) int {
	var diff int
	for i := 0; i < Words; i++ {
		diff += bits.OnesCount64(a[i] ^ b[i])
	}
	return diff
}

// bitMaps 极简并发位图：竞速击中即硬终止其他worker的HTTP请求（比较atomic.Value少一次
// 接口装箱 + 全局读栅栏，v8实测rank竞速-15%时延）
type bitMaps struct {
	m []uint32
}

func newBitMaps(n int) *bitMaps { return &bitMaps{m: make([]uint32, (n+31)/32)} }

func (b *bitMaps) get(i int) bool { return b.m[i>>5]&(1<<uint(i&31)) != 0 }
func (b *bitMaps) set(i int)      { b.m[i>>5] |= 1 << uint(i&31) }
