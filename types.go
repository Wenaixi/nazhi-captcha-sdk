// Package captchasdk 纳智验证码破解SDK —— 一行接入，内置预训练库
//
// 用法：
//
//	solver := captchasdk.New()
//	code := solver.Solve(imageBytes) // 内部自动获取SESSION并破解
//
// 或带已有SESSION：
//
//	code := solver.SolveWithSession(sid, imageBytes)
package captchasdk

// 常量：字符池与模板参数
// DefaultBase 默认目标站点（可通过 Solver.SetBase 覆盖）
const (
	DefaultBase   = "https://www.nazhisoft.com"
	Pool          = "13478kwx"
	PoolLen       = len(Pool)
	Combos        = PoolLen * PoolLen * PoolLen * PoolLen
	Slots         = 4
	TW            = 16
	TH            = 20
	Words         = (TW * TH) / 64 // 320bit → 5×uint64
	DarkThreshold = 170
	MatchThresh   = 0.72
	AddThresh     = 0.85
)

// Tmpl 单字符模板
type Tmpl struct {
	Words [Words]uint64
	Count int
}

// codeTable 预计算全部组合（包初始化时1024次，运行时查表零分配）
var codeTable = func() [Combos]string {
	var t [Combos]string
	for i := 0; i < Combos; i++ {
		t[i] = string([]byte{
			Pool[(i/(PoolLen*PoolLen*PoolLen))%PoolLen],
			Pool[(i/(PoolLen*PoolLen))%PoolLen],
			Pool[(i/PoolLen)%PoolLen],
			Pool[i%PoolLen],
		})
	}
	return t
}()

// IdxToCode 索引→4字符（查表，零分配）
func IdxToCode(i int) string { return codeTable[i] }

// ---- 二进制序列化格式（紧凑：40B/模板 vs JSON 124B）----
// header: magic "NKPT" + version + slotCount(1B) + per-slot entryCount(2B×4)
// entry:  char(1B) + count(2B) + words(5×8B=40B)

var magic = [4]byte{'N', 'K', 'P', 'T'}

// MarshalBinary 模板表→二进制
func marshalTable(slots []map[string][]*Tmpl) []byte {
	total := 4 + 1 + Slots*2
	for s := 0; s < Slots; s++ {
		for _, ts := range slots[s] {
			total += len(ts) * 43 // 1+2+40
		}
	}
	buf := make([]byte, 0, total)
	buf = append(buf, magic[:]...)
	buf = append(buf, byte(Slots))
	for s := 0; s < Slots; s++ {
		// 先算条目数
		n := 0
		for _, ts := range slots[s] {
			n += len(ts)
		}
		buf = append(buf, byte(n>>8), byte(n))
	}
	for s := 0; s < Slots; s++ {
		for ch, ts := range slots[s] {
			for _, tm := range ts {
				buf = append(buf, ch[0])
				buf = append(buf, byte(tm.Count>>8), byte(tm.Count))
				for w := 0; w < Words; w++ {
					v := tm.Words[w]
					for b := 0; b < 8; b++ {
						buf = append(buf, byte(v>>(uint(b)*8)))
					}
				}
			}
		}
	}
	return buf
}

// unmarshalTable 二进制→模板表
func unmarshalTable(data []byte) []map[string][]*Tmpl {
	if len(data) < 5 || data[0] != magic[0] || data[1] != magic[1] || data[2] != magic[2] || data[3] != magic[3] {
		return nil
	}
	pos := 5
	counts := make([]int, Slots)
	for s := 0; s < Slots; s++ {
		counts[s] = int(data[pos])<<8 | int(data[pos+1])
		pos += 2
	}
	slots := make([]map[string][]*Tmpl, Slots)
	for s := 0; s < Slots; s++ {
		slots[s] = make(map[string][]*Tmpl)
		for i := 0; i < counts[s]; i++ {
			if pos+43 > len(data) {
				return nil
			}
			ch := string(data[pos : pos+1])
			count := int(data[pos+1])<<8 | int(data[pos+2])
			pos += 3
			var words [Words]uint64
			for w := 0; w < Words; w++ {
				var v uint64
				for b := 0; b < 8; b++ {
					v |= uint64(data[pos]) << uint(b*8)
					pos++
				}
				words[w] = v
			}
			slots[s][ch] = append(slots[s][ch], &Tmpl{Words: words, Count: count})
		}
	}
	return slots
}
