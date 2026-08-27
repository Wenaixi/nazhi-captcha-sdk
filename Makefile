# 纳智验证码破解 SDK 构建与工具链

GO ?= go

.PHONY: build gen train merge hitrate count stress demo test bench clean all

all: build

# 编译全部工具（-s -w 减体积）
build:
	$(GO) build -ldflags="-s -w" -o bin/gen.exe ./cmd/gen
	$(GO) build -ldflags="-s -w" -o bin/train.exe ./cmd/train
	$(GO) build -ldflags="-s -w" -o bin/merge.exe ./cmd/merge
	$(GO) build -ldflags="-s -w" -o bin/hitrate.exe ./cmd/hitrate
	$(GO) build -ldflags="-s -w" -o bin/count.exe ./cmd/count
	$(GO) build -ldflags="-s -w" -o bin/stress.exe ./cmd/stress
	$(GO) build -ldflags="-s -w" -o bin/demo.exe ./cmd/demo

# 从 v6 json 模板生成内置库
gen:
	$(GO) run ./cmd/gen

# 离线预训练（详见 README 模板库训练与升级）
train:
	$(GO) run ./cmd/train -n 900 -c 10 -lib data/builtin.bin

# 固化训练产物为内置库
merge:
	$(GO) run ./cmd/merge

# 命中率验证（质量门禁）
hitrate:
	$(GO) run ./cmd/hitrate -n 200

# 模板库统计
count:
	$(GO) run ./cmd/count data/builtin.bin

# 并发压测（默认 1000 账号）
stress:
	$(GO) run ./cmd/stress

# 接入示例
demo:
	$(GO) run ./cmd/demo

# 单元测试
test:
	$(GO) test -run=. -count=1 ./...

# 性能基准
bench:
	$(GO) test -bench=. -benchmem -run="^$$" -benchtime=2s .

# 清理产物
clean:
	-rm -rf bin
