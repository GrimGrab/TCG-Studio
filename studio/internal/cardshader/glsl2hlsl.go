package cardshader

import (
	"context"
	_ "embed"
	"fmt"
	"strings"
	"sync"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

// glsl2hlsl.wasm (built by tools\build-glsl2hlsl-wasm.ps1 from glsl2hlsl\) holds glslang and SPIRV-Cross, unchanged:
// GLSL -> SPIR-V -> HLSL. Every conversion runs in a fresh instance: the converters allocate freely and stop the
// instance when they give up on a shader, and an instance costs a few milliseconds.
//
//go:embed glsl2hlsl.wasm
var glsl2hlslWasm []byte

var (
	convOnce     sync.Once
	convRuntime  wazero.Runtime
	convCompiled wazero.CompiledModule
	convErr      error
)

// SourceError is what the shader's author got wrong: the compiler's messages, one per line.
type SourceError struct{ Messages string }

func (e *SourceError) Error() string { return e.Messages }

// toHLSL converts one complete GLSL fragment shader. Mistakes in the shader come back as a *SourceError.
func toHLSL(glsl string) (string, error) {
	ctx := context.Background()
	convOnce.Do(func() {
		convRuntime = wazero.NewRuntime(ctx)
		if _, convErr = wasi_snapshot_preview1.Instantiate(ctx, convRuntime); convErr != nil {
			return
		}
		convCompiled, convErr = convRuntime.CompileModule(ctx, glsl2hlslWasm)
	})
	if convErr != nil {
		return "", fmt.Errorf("shader converter: %w", convErr)
	}
	mod, err := convRuntime.InstantiateModule(ctx, convCompiled, wazero.NewModuleConfig().WithName("").WithStartFunctions("_initialize"))
	if err != nil {
		return "", fmt.Errorf("shader converter: %w", err)
	}
	defer mod.Close(ctx)

	res, err := mod.ExportedFunction("cs_alloc").Call(ctx, uint64(len(glsl)))
	if err != nil || res[0] == 0 || !mod.Memory().Write(uint32(res[0]), []byte(glsl)) {
		return "", fmt.Errorf("shader converter: out of memory")
	}
	ok, err := mod.ExportedFunction("cs_convert").Call(ctx, res[0], uint64(len(glsl)))
	if err != nil {
		// The converters stopped: SPIR-V they cannot express in HLSL (their message is not kept in this build).
		return "", &SourceError{Messages: "ERROR: this shader uses something that cannot be converted for the game (Direct3D)"}
	}
	ptr, _ := mod.ExportedFunction("cs_out").Call(ctx)
	size, _ := mod.ExportedFunction("cs_out_len").Call(ctx)
	out, read := mod.Memory().Read(uint32(ptr[0]), uint32(size[0]))
	if !read {
		return "", fmt.Errorf("shader converter: bad output")
	}
	if ok[0] == 0 {
		return "", &SourceError{Messages: strings.TrimSpace(string(out))}
	}
	return string(out), nil
}
