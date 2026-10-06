package unityfs

import (
	"context"
	_ "embed"
	"fmt"
	"sync"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

// decoders.wasm (built by tools\build-decoders-wasm.ps1 from decoders\) holds Unity's crunch transcoder and bcdec's BC7
// decoder, unchanged. It needs no host functions; instances are pooled because each decodes one texture at a time.
//
//go:embed decoders.wasm
var decodersWasm []byte

type wasmDecoder struct {
	mod                       api.Module
	reset, alloc, crunch, bc7 api.Function
}

var (
	decOnce     sync.Once
	decRuntime  wazero.Runtime
	decCompiled wazero.CompiledModule
	decErr      error
	decPool     sync.Pool
)

func getDecoder(ctx context.Context) (*wasmDecoder, error) {
	decOnce.Do(func() {
		decRuntime = wazero.NewRuntime(context.Background())
		decCompiled, decErr = decRuntime.CompileModule(context.Background(), decodersWasm)
	})
	if decErr != nil {
		return nil, fmt.Errorf("texture decoders: %w", decErr)
	}
	if d, ok := decPool.Get().(*wasmDecoder); ok {
		return d, nil
	}
	mod, err := decRuntime.InstantiateModule(ctx, decCompiled, wazero.NewModuleConfig().WithName(""))
	if err != nil {
		return nil, fmt.Errorf("texture decoders: %w", err)
	}
	return &wasmDecoder{mod: mod, reset: mod.ExportedFunction("dec_reset"), alloc: mod.ExportedFunction("dec_alloc"),
		crunch: mod.ExportedFunction("crunch_unpack"), bc7: mod.ExportedFunction("bc7_decode")}, nil
}

// put writes data into a fresh buffer in the module's memory.
func (d *wasmDecoder) put(ctx context.Context, data []byte) (uint32, error) {
	res, err := d.alloc.Call(ctx, uint64(len(data)))
	if err != nil || res[0] == 0 {
		return 0, fmt.Errorf("out of memory")
	}
	p := uint32(res[0])
	if !d.mod.Memory().Write(p, data) {
		return 0, fmt.Errorf("out of memory")
	}
	return p, nil
}

func (d *wasmDecoder) read(p, n uint32) ([]byte, error) {
	v, ok := d.mod.Memory().Read(p, n)
	if !ok {
		return nil, fmt.Errorf("bad output buffer")
	}
	return append([]byte(nil), v...), nil
}

// unpackCrunch transcodes a crunched texture (Unity DXT1Crunched/DXT5Crunched data) to plain DXT blocks of mip 0.
// blockBytes is 8 (DXT1) or 16 (DXT5).
func unpackCrunch(data []byte) (dxt []byte, w, h, blockBytes int, err error) {
	ctx := context.Background()
	d, err := getDecoder(ctx)
	if err != nil {
		return nil, 0, 0, 0, err
	}
	defer decPool.Put(d)
	if _, err := d.reset.Call(ctx); err != nil {
		return nil, 0, 0, 0, err
	}
	src, err := d.put(ctx, data)
	if err != nil {
		return nil, 0, 0, 0, err
	}
	outs, err := d.put(ctx, make([]byte, 20)) // out, out_len, width, height, block_bytes (uint32 each)
	if err != nil {
		return nil, 0, 0, 0, err
	}
	res, err := d.crunch.Call(ctx, uint64(src), uint64(len(data)), uint64(outs), uint64(outs+4), uint64(outs+8),
		uint64(outs+12), uint64(outs+16))
	if err != nil {
		return nil, 0, 0, 0, err
	}
	if code := int32(res[0]); code != 0 {
		return nil, 0, 0, 0, fmt.Errorf("crunched texture data is damaged (code %d)", code)
	}
	mem := d.mod.Memory()
	u := func(off uint32) uint32 { v, _ := mem.ReadUint32Le(outs + off); return v }
	dxt, err = d.read(u(0), u(4))
	return dxt, int(u(8)), int(u(12)), int(u(16)), err
}

// decodeBC7 decodes BC7 blocks into RGBA rows in stored order, padded to whole blocks (row length 4·ceil(w/4) pixels).
func decodeBC7(data []byte, w, h int) ([]byte, error) {
	bw, bh := (w+3)/4, (h+3)/4
	if len(data) < bw*bh*16 {
		return nil, fmt.Errorf("texture data too short")
	}
	ctx := context.Background()
	d, err := getDecoder(ctx)
	if err != nil {
		return nil, err
	}
	defer decPool.Put(d)
	if _, err := d.reset.Call(ctx); err != nil {
		return nil, err
	}
	src, err := d.put(ctx, data[:bw*bh*16])
	if err != nil {
		return nil, err
	}
	res, err := d.bc7.Call(ctx, uint64(src), uint64(w), uint64(h))
	if err != nil {
		return nil, err
	}
	if res[0] == 0 {
		return nil, fmt.Errorf("out of memory")
	}
	return d.read(uint32(res[0]), uint32(bw*4*bh*4*4))
}
