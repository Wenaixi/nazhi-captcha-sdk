package captchasdk

// 阈值的整数汉明距离形式（免除热路径float运算）
// MatchDiffMax match判定：320bit相似度0.72 → 距离上限89.6取整
// AddDiffMax   add合并：相似度0.85 → 距离上限48
const (
	MatchDiffMax = 90
	AddDiffMax   = 48
)

// charStr 单字符byte→string查表（rank路径零分配）
var charStr = func() [256]string {
	var t [256]string
	for c := 0; c < 256; c++ {
		t[c] = string([]byte{byte(c)})
	}
	return t
}()

// table 运行时模板表
type table struct {
	slots []map[string][]*Tmpl
	reps  []map[string]*Tmpl // 每字符代表模板缓存（add时重建，match零扫描）
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

// add 训练样本（相似合并；同步维护代表模板缓存，保证add后match立即生效）
func (t *table) add(wordsArr [Slots][Words]uint64, code string) {
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
// ponytail: O(全表)重建，自学习入库频率低（每破解成功1次），摊销可忽略
func (t *table) rebuild() {
	t.reps = make([]map[string]*Tmpl, Slots)
	for i := 0; i < Slots; i++ {
		t.reps[i] = make(map[string]*Tmpl, len(t.slots[i]))
		for ch, ts := range t.slots[i] {
			var rep *Tmpl
			for _, tm := range ts {
				if rep == nil || tm.Count > rep.Count {
					rep = tm
				}
			}
			if rep != nil {
				t.reps[i][ch] = rep
			}
		}
	}
}

// match 每槽最相似字符组合
// CPU优化：每个字符只与其Count最大的代表模板比较（~47条/槽 → ~8字符/槽，6倍提速），
// 代表模板缓存由rebuild维护，match内零扫描零装箱
func (t *table) match(wordsArr *[Slots][Words]uint64) (string, bool) {
	if t.reps == nil {
		t.rebuild()
	}
	var code [Slots]byte
	for i := 0; i < Slots; i++ {
		bestCh := byte(0)
		bestDiff := 1 << 30
		for ch, rep := range t.reps[i] {
			if d := wordDiffHamming(&wordsArr[i], &rep.Words); d < bestDiff {
				bestDiff = d
				bestCh = ch[0]
			}
		}
		if bestCh == 0 || bestDiff > MatchDiffMax {
			return "", false
		}
		code[i] = bestCh
	}
	return string(code[:]), true
}

// rank 每槽top3候选（整数距离+固定数组插入排序，零堆分配零装箱）
func (t *table) rank(wordsArr *[Slots][Words]uint64) ([Slots][3]string, bool) {
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
