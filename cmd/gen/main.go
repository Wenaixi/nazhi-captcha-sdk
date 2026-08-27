// gen：table.json(v6格式) → builtin.bin 二进制模板库
package main

import (
	"encoding/json"
	"fmt"
	"os"
)

const (
	Slots = 4
	Words = 5
)

type tmpl struct {
	Words [Words]uint64 `json:"words"`
	Count int           `json:"count"`
}

type srcTable struct {
	Slots []map[string][]*tmpl `json:"slots"`
}

func main() {
	if len(os.Args) < 3 {
		fmt.Println("usage: gen <table.json> <builtin.bin>")
		return
	}
	b, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Println("read err:", err)
		return
	}
	var src srcTable
	if err := json.Unmarshal(b, &src); err != nil || len(src.Slots) != Slots {
		fmt.Println("parse err:", err)
		return
	}
	// 二进制格式：magic + slotCount + per-slot counts + entries
	out := []byte{'N', 'K', 'P', 'T', byte(Slots)}
	for s := 0; s < Slots; s++ {
		n := 0
		for _, ts := range src.Slots[s] {
			n += len(ts)
		}
		out = append(out, byte(n>>8), byte(n))
	}
	for s := 0; s < Slots; s++ {
		for ch, ts := range src.Slots[s] {
			for _, tm := range ts {
				out = append(out, ch[0])
				out = append(out, byte(tm.Count>>8), byte(tm.Count))
				for w := 0; w < Words; w++ {
					v := tm.Words[w]
					for b := 0; b < 8; b++ {
						out = append(out, byte(v>>(uint(b)*8)))
					}
				}
			}
		}
	}
	if err := os.WriteFile(os.Args[2], out, 0644); err != nil {
		fmt.Println("write err:", err)
		return
	}
	fmt.Printf("OK %s -> %s (%d bytes, %d templates)\n", os.Args[1], os.Args[2], len(out), func() int {
		n := 0
		for s := 0; s < Slots; s++ {
			for _, ts := range src.Slots[s] {
				n += len(ts)
			}
		}
		return n
	}())
}
