package captchasdk

import "sort"

// table 运行时模板表
type table struct {
	slots []map[string][]*Tmpl
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

// add 训练样本（相似合并）
func (t *table) add(wordsArr [Slots][Words]uint64, code string) {
	for i := 0; i < Slots; i++ {
		ch := string(code[i])
		lst := t.slots[i][ch]
		added := false
		for _, tm := range lst {
			if wordSim(&wordsArr[i], &tm.Words) >= AddThresh {
				tm.Count++
				added = true
				break
			}
		}
		if !added {
			t.slots[i][ch] = append(lst, &Tmpl{Words: wordsArr[i], Count: 1})
		}
	}
}

// match 每槽最相似字符组合（CPU优化：每个字符只与其Count最大的代表模板比较，
// 减少比较次数 ~47条/槽 → ~8字符/槽，即 6倍提速；代表模板由add时维护）
func (t *table) match(wordsArr *[Slots][Words]uint64) (string, bool) {
	var code [Slots]byte
	for i := 0; i < Slots; i++ {
		bestCh := ""
		bestSim := 0.0
		for ch, ts := range t.slots[i] {
			// 找该字符Count最大（最具代表性）的模板快速比较
			var rep *Tmpl
			for _, tm := range ts {
				if rep == nil || tm.Count > rep.Count {
					rep = tm
				}
			}
			if rep == nil {
				continue
			}
			if s := wordSim(&wordsArr[i], &rep.Words); s > bestSim {
				bestSim = s
				bestCh = ch
			}
		}
		if bestCh == "" || bestSim < MatchThresh {
			return "", false
		}
		code[i] = bestCh[0]
	}
	return string(code[:]), true
}

// rank 每槽top3候选
func (t *table) rank(wordsArr *[Slots][Words]uint64) ([Slots][3]string, bool) {
	var out [Slots][3]string
	for i := 0; i < Slots; i++ {
		type cand struct {
			ch  string
			sim float64
		}
		var best []cand
		for ch, ts := range t.slots[i] {
			maxSim := 0.0
			for _, tm := range ts {
				if s := wordSim(&wordsArr[i], &tm.Words); s > maxSim {
					maxSim = s
				}
			}
			best = append(best, cand{ch, maxSim})
		}
		sort.Slice(best, func(a, b int) bool { return best[a].sim > best[b].sim })
		n := 3
		if len(best) < n {
			n = len(best)
		}
		for k := 0; k < n; k++ {
			out[i][k] = best[k].ch
		}
	}
	return out, true
}
