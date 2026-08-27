package captchasdk

import (
	"testing"
)

// 基准：单次wordSim
func BenchmarkWordSim(b *testing.B) {
	var a, c [Words]uint64
	for i := 0; i < Words; i++ {
		a[i] = 0xDEADBEEFCAFEBABE
		c[i] = 0x1234567890ABCDEF
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		wordSim(&a, &c)
	}
}

// 基准：单槽match（全部字符×全部变体）
func BenchmarkSlotMatch(b *testing.B) {
	slots := unmarshalTable(builtinTable)
	if slots == nil {
		b.Fatal("no table")
	}
	var w [Words]uint64
	for i := 0; i < Words; i++ {
		w[i] = 0xAAAAAAAAAAAAAAAA
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for ch, ts := range slots[0] {
			for _, tm := range ts {
				wordSim(&w, &tm.Words)
				_ = ch
			}
		}
	}
}

// 基准：完整4槽match查表路径（v8代表模板缓存：每字符仅比较1个代表模板）
func BenchmarkFullMatch(b *testing.B) {
	t := newTable()
	if slots := unmarshalTable(builtinTable); slots != nil {
		t.slots = slots
	}
	t.rebuild()
	var w [Slots][Words]uint64
	for s := range w {
		for i := 0; i < Words; i++ {
			w[s][i] = 0xAAAAAAAAAAAAAAAA
		}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		t.match(&w)
	}
}

// 基准：rank路径（top3候选计算）
func BenchmarkRank(b *testing.B) {
	t := newTable()
	if slots := unmarshalTable(builtinTable); slots != nil {
		t.slots = slots
	}
	var w [Slots][Words]uint64
	for s := range w {
		for i := 0; i < Words; i++ {
			w[s][i] = 0xAAAAAAAAAAAAAAAA
		}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		t.rank(&w)
	}
}

// 基准：imageWords（图像处理全链路）
func BenchmarkImageWords(b *testing.B) {
	sid, img, err := fetchForBench()
	if err != nil {
		b.Skip("network unavailable")
	}
	_ = sid
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		imageWords(img)
	}
}

// 基准：内存分配
func BenchmarkAlloc(b *testing.B) {
	reportAllocs := func() {}
	_ = reportAllocs
	var w1, w2 [Words]uint64
	for i := 0; i < Words; i++ {
		w1[i] = 0xDEAD
		w2[i] = 0xBEEF
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		wordSim(&w1, &w2)
	}
}
