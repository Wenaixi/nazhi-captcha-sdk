package captchasdk

import "testing"

func TestPopcount(t *testing.T) {
	cases := []struct {
		x    uint64
		want uint64
	}{
		{0, 0},
		{1, 1},
		{0xFFFFFFFFFFFFFFFF, 64},
		{0xFF00FF00FF00FF00, 32},
	}
	for _, c := range cases {
		if got := popcount(c.x); got != c.want {
			t.Errorf("popcount(%x) = %d, want %d", c.x, got, c.want)
		}
	}
}

func TestMarshalRoundtrip(t *testing.T) {
	tbl := newTable()
	var w1 [Words]uint64
	w1[0] = 0xDEADBEEFCAFEBABE
	tbl.slots[0]["7"] = append(tbl.slots[0]["7"], &Tmpl{Words: w1, Count: 3})
	var w2 [Words]uint64
	w2[1] = 0x1234567890ABCDEF
	tbl.slots[3]["x"] = append(tbl.slots[3]["x"], &Tmpl{Words: w2, Count: 1})

	bin := marshalTable(tbl.slots)
	restored := unmarshalTable(bin)
	if restored == nil {
		t.Fatal("unmarshal returned nil")
	}
	if len(restored[0]["7"]) != 1 || restored[0]["7"][0].Count != 3 {
		t.Fatalf("slot0 entry mismatch: %+v", restored[0]["7"])
	}
	if restored[0]["7"][0].Words[0] != w1[0] {
		t.Fatalf("slot0 words mismatch")
	}
	if len(restored[3]["x"]) != 1 || restored[3]["x"][0].Words[1] != w2[1] {
		t.Fatalf("slot3 entry mismatch")
	}
}

func TestBuiltinLoads(t *testing.T) {
	slots := unmarshalTable(builtinTable)
	if slots == nil {
		t.Fatal("内置库解析失败")
	}
	n := 0
	for s := 0; s < Slots; s++ {
		for _, ts := range slots[s] {
			n += len(ts)
		}
	}
	if n < 100 {
		t.Fatalf("内置模板过少: %d", n)
	}
	t.Logf("内置模板 %d 条加载正常", n)
}

func TestIdxToCode(t *testing.T) {
	if IdxToCode(0) != "1111" {
		t.Fatalf("IdxToCode(0) = %s", IdxToCode(0))
	}
	if IdxToCode(Combos-1) != "xxxx" {
		t.Fatalf("IdxToCode(max) = %s", IdxToCode(Combos-1))
	}
}
func TestMatchImage_Boundaries(t *testing.T) {
	// 空输入：返回 (空串, false)
	if code, ok := MatchImage(nil); ok || code != "" {
		t.Fatalf("空输入应返回空串+false，实际 code=%q ok=%v", code, ok)
	}
	// 非法字节（非图片）：返回 (空串, false)，不 panic
	if code, ok := MatchImage([]byte("not-an-image")); ok || code != "" {
		t.Fatalf("非法输入应返回空串+false，实际 code=%q ok=%v", code, ok)
	}
	// 极短 JPEG 字节：不 panic（分割必然失败返回 false）
	jpeg := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46, 0x00, 0x01}
	MatchImage(jpeg) // 不 panic 即可
}

func TestMatchImage_SharedTableWithSolver(t *testing.T) {
	// MatchImage 与 Solver.New() 必须共享同一模板表单例（内存 1 份）
	s := New()
	if s.tbl != sharedTbl {
		t.Fatal("Solver 与 MatchImage 应共享同一模板表")
	}
}

