# 纳智验证码破解SDK
.PHONY: build gen demo test clean

# 编译SDK二进制
build:
	go build -ldflags="-s -w" -o gen.exe ./cmd/gen
	go build -ldflags="-s -w" -o demo.exe ./cmd/demo

# 从v6训练库重新生成内置模板
gen: build
	./gen.exe ../captcha-cracker-v6/data/table.json data/builtin.bin

# 演示
demo: build
	./demo.exe

# 跑测试
test:
	go test ./... -v

clean:
	rm -f gen.exe demo.exe
