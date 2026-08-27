package captchasdk

import "sync"

// MatchDiffMax match判定：320bit相似度0.72 → 距离上限89.6取整（免除热路径float运算）
// AddDiffMax   训练合并阈值：仅高度相似(0.91+)才合并，变体独立成条——多样性优先提高覆盖率
const (
	MatchDiffMax = 90
	AddDiffMax   = 30
)

// charStr 单字符byte→string查表（rank路径零分配）
var charStr = func() [256]string {
	var t [256]string
	for c := 0; c < 256; c++ {
		t[c] = string([]byte{byte(c)})
	}
	return t
}()

// table 运行时模板表（并发安全：match/rank读锁共享，LoadTable写锁互斥）
type table struct {
	mu    sync.RWMutex
	slots []map[string][]*Tmpl
	reps  []map[string]*Tmpl   // 每字符首选代表模板（match主路径）
	reps2 []map[string][]*Tmpl // 每字符前2代表（提高fast命中率，零训练场景关键）
}

func newTable() *table {
	t := &table{slots: make([]map[string][]*Tmpl, Slots)}
	for i := 0; i < Slots; i++ {
		t.slots[i] = make(map[string][]*Tmpl)
	}
	return t
}

func (t *table) total() int {
	n := 0
	for s := 0; s < Slots; s++ {
		for _, ts := range t.slots[s] {
			n += len(ts)
		}
	}
	return n
}

// add 训练样本入库（仅 cmd/train 离线训练路径调用，运行时零训练）
// 相似合并：与现有模板距离<=AddDiffMax则计数+1，否则新条目
func (t *table) add(wordsArr [Slots][Words]uint64, code string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for i := 0; i < Slots; i++ {
		ch := string(code[i])
		lst := t.slots[i][ch]
		added := false
		for _, tm := range lst {
			if wordDiffHamming(&wordsArr[i], &tm.Words) <= AddDiffMax {
				tm.Count++
				added = true
				break
			}
		}
		if !added {
			t.slots[i][ch] = append(lst, &Tmpl{Words: wordsArr[i], Count: 1})
		}
	}
	t.rebuild()
}

// rebuild 重建代表模板缓存（按Count选各字符最具代表性的模板）
// 调用时机：初始化/LoadTable（离线固化库变更时），运行时零重建
func (t *table) rebuild() {
	// 前置条件：调用方持写锁或处于初始化阶段（无并发）
	t.reps = make([]map[string]*Tmpl, Slots)
	t.reps2 = make([]map[string][]*Tmpl, Slots)
	for i := 0; i < Slots; i++ {
		t.reps[i] = make(map[string]*Tmpl, len(t.slots[i]))
		t.reps2[i] = make(map[string][]*Tmpl, len(t.slots[i]))
		for ch, ts := range t.slots[i] {
			// Count前2代表：最常见变体+次常见变体，双倍覆盖字形差异
			top1, top2 := (*Tmpl)(nil), (*Tmpl)(nil)
			for _, tm := range ts {
				switch {
				case top1 == nil || tm.Count > top1.Count:
					top2, top1 = top1, tm
				case top2 == nil || tm.Count > top2.Count:
					top2 = tm
				}
			}
			if top1 != nil {
				t.reps[i][ch] = top1
				if top2 != nil {
					t.reps2[i][ch] = []*Tmpl{top1, top2}
				} else {
					t.reps2[i][ch] = []*Tmpl{top1}
				}
			}
		}
	}
}

// match 每槽最相似字符组合
// CPU优化：每个字符只与其Count最大的代表模板比较（~47条/槽 → ~8字符/槽，6倍提速），
// 代表模板缓存由rebuild维护，match内零扫描零装箱
// match 每槽最相似字符：全表线性扫描（5364条/槽×5×POPCNT≈54µs——相对rank的81次网络RTT
// 快4个数量级；全表扫描100%利用训练库，命中率=probe全表距离判定，这是O(1)化的关键）
func (t *table) match(wordsArr *[Slots][Words]uint64) (string, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	var code [Slots]byte
	for i := 0; i < Slots; i++ {
		bestCh := byte(0)
		bestDiff := 1 << 30
		for ch, ts := range t.slots[i] {
			for _, tm := range ts {
				if d := wordDiffHamming(&wordsArr[i], &tm.Words); d < bestDiff {
					bestDiff = d
					bestCh = ch[0]
				}
			}
		}
		if bestCh == 0 || bestDiff > MatchDiffMax {
			return "", false
		}
		code[i] = bestCh
	}
	return string(code[:]), true
}

// MatchPublic 导出match（供离线命中率统计工具）
func (t *table) MatchPublic(wordsArr *[Slots][Words]uint64) (string, bool) { return t.match(wordsArr) }

// ProbeSlotDists 导出诊断：每槽与全表所有模板的最小距离（供cmd/probe定位miss根因）
func (t *table) probeSlotDists(wordsArr *[Slots][Words]uint64) [Slots]int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	var out [Slots]int
	for i := 0; i < Slots; i++ {
		minD := 1 << 30
		for _, ts := range t.slots[i] {
			for _, tm := range ts {
				if d := wordDiffHamming(&wordsArr[i], &tm.Words); d < minD {
					minD = d
				}
			}
		}
		out[i] = minD
	}
	return out
}

// rank 每槽top3候选（整数距离+固定数组插入排序，零堆分配零装箱）
func (t *table) rank(wordsArr *[Slots][Words]uint64) ([Slots][3]string, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	var out [Slots][3]string
	type cand struct {
		ch byte
		d  int
	}
	for i := 0; i < Slots; i++ {
		var best [8]cand // 字符池8字符，栈上数组
		n := 0
		for ch, ts := range t.slots[i] {
			minD := 1 << 30
			for _, tm := range ts {
				if d := wordDiffHamming(&wordsArr[i], &tm.Words); d < minD {
					minD = d
				}
			}
			// 插入排序维护top3
			c := cand{ch[0], minD}
			pos := n
			if pos > 3 {
				pos = 3
			}
			for pos > 0 && best[pos-1].d > c.d {
				if pos < 3 {
					best[pos] = best[pos-1]
				}
				pos--
			}
			if pos < 3 {
				best[pos] = c
				if n < 3 {
					n++
				}
			}
			if n < 4 {
				n++
			}
		}
		for k := 0; k < 3 && k < n; k++ {
			out[i][k] = charStr[best[k].ch]
		}
	}
	return out, true
}
