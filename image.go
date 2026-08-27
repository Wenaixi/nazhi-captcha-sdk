package captchasdk

import (
	"bytes"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"sync"
)

// gray 复用灰度矩阵（68x33是kaptcha标准尺寸，其他尺寸走普通分配）
type gray struct {
	w, h int
	pix  [][]byte
}

var grayPool = sync.Pool{
	New: func() interface{} { return newGray(68, 33) },
}

func newGray(w, h int) *gray {
	g := &gray{w: w, h: h, pix: make([][]byte, h)}
	for i := range g.pix {
		g.pix[i] = make([]byte, w)
	}
	return g
}

func loadGray(data []byte) (*gray, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	// 标准kaptcha尺寸直接从池取（零分配热路径）；异形尺寸临时分配
	var g *gray
	if w == 68 && h == 33 {
		g = grayPool.Get().(*gray)
		defer grayPool.Put(g)
	} else {
		g = newGray(w, h)
	}
	for y := 0; y < h; y++ {
		row := g.pix[y]
		for x := 0; x < w; x++ {
			r, gg, bl, _ := img.At(b.Min.X+x, b.Min.Y+y).RGBA()
			row[x] = byte(0.299*float64(r>>8) + 0.587*float64(gg>>8) + 0.114*float64(bl>>8))
		}
	}
	return g, nil
}

type box [4]int

// segSlots 列投影分割4槽位
func segSlots(g *gray) ([]box, bool) {
	w, h, pix := g.w, g.h, g.pix
	top0, bot0 := h, -1
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if pix[y][x] < DarkThreshold {
				if y < top0 {
					top0 = y
				}
				if y > bot0 {
					bot0 = y
				}
				break
			}
		}
	}
	if bot0 <= top0 {
		return nil, false
	}
	col := make([]int, w)
	for x := 0; x < w; x++ {
		c := 0
		for y := top0; y <= bot0; y++ {
			if pix[y][x] < DarkThreshold {
				c++
			}
		}
		col[x] = c
	}
	type seg struct{ a, b int }
	var segs []seg
	in, s0 := false, 0
	for x := 0; x < w; x++ {
		if col[x] > 1 && !in {
			in, s0 = true, x
		}
		if (col[x] <= 1 || x == w-1) && in {
			in = false
			if x-s0 >= 3 {
				segs = append(segs, seg{s0, x})
			}
		}
	}
	if len(segs) == 0 {
		return nil, false
	}
	var merged []seg
	for _, s := range segs {
		if len(merged) > 0 && s.a-merged[len(merged)-1].b < 4 {
			merged[len(merged)-1].b = s.b
		} else {
			merged = append(merged, s)
		}
	}
	for len(merged) < Slots {
		mi := 0
		for i := 1; i < len(merged); i++ {
			if merged[i].b-merged[i].a > merged[mi].b-merged[mi].a {
				mi = i
			}
		}
		mid := (merged[mi].a + merged[mi].b) / 2
		a1 := merged[mi]
		merged = append(merged[:mi], merged[mi+1:]...)
		merged = append(merged, seg{a1.a, mid}, seg{mid + 1, a1.b})
		if len(merged) >= Slots {
			break
		}
	}
	for i := 1; i < len(merged); i++ {
		for j := i; j > 0 && merged[j].a < merged[j-1].a; j-- {
			merged[j], merged[j-1] = merged[j-1], merged[j]
		}
	}
	if len(merged) > Slots {
		merged = merged[:Slots]
	}
	if len(merged) < Slots {
		return nil, false
	}
	var out []box
	for _, s := range merged {
		ty0, ty1 := h, -1
		for x := s.a; x <= s.b; x++ {
			for y := 0; y < h; y++ {
				if pix[y][x] < DarkThreshold {
					if y < ty0 {
						ty0 = y
					}
					if y > ty1 {
						ty1 = y
					}
				}
			}
		}
		if ty1 <= ty0 {
			ty0, ty1 = top0, bot0
		}
		out = append(out, box{s.a, s.b, ty0, ty1})
	}
	return out, true
}

// charWords 槽位→320bit模板
func charWords(g *gray, bx box) ([Words]uint64, bool) {
	x1, x2, ty0, ty1 := bx[0], bx[1], bx[2], bx[3]
	cw := x2 - x1 + 1
	ch := ty1 - ty0 + 1
	if cw < 2 || ch < 2 {
		return [Words]uint64{}, false
	}
	var bits [TW * TH]byte
	for yy := 0; yy < TH; yy++ {
		row := g.pix[ty0+(ch*yy)/TH]
		base := yy * TW
		for xx := 0; xx < TW; xx++ {
			px := x1 + (cw*xx)/TW
			if px > x2 {
				px = x2
			}
			if row[px] < DarkThreshold {
				bits[base+xx] = 1
			}
		}
	}
	var out [Words]uint64
	for wi := 0; wi < Words; wi++ {
		var word uint64
		base := wi * 64
		for b := 0; b < 64; b++ {
			if bits[base+b] == 1 {
				word |= 1 << uint(b)
			}
		}
		out[wi] = word
	}
	return out, true
}

// imageWords 整图→4槽模板
func imageWords(data []byte) ([Slots][Words]uint64, bool) {
	g, err := loadGray(data)
	if err != nil {
		return [Slots][Words]uint64{}, false
	}
	boxes, ok := segSlots(g)
	if !ok || len(boxes) < Slots {
		return [Slots][Words]uint64{}, false
	}
	var out [Slots][Words]uint64
	for i := 0; i < Slots; i++ {
		wo, ok := charWords(g, boxes[i])
		if !ok {
			return [Slots][Words]uint64{}, false
		}
		out[i] = wo
	}
	return out, true
}
