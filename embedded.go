package captchasdk

import _ "embed"

// 预训练模板库（二进制格式，由 cmd/gen 生成，随二进制分发）
// 使用者无需训练，New() 即含全部内置模板
//
//go:embed data/builtin.bin
var builtinTable []byte
