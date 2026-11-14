package cardshader

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"unsafe"
)

// Direct3D's own shader compiler, the DLL every Windows 10/11 has in System32 (the Unity editor ships the same one).
// Called directly: no C toolchain, nothing to bundle.

var (
	d3dOnce        sync.Once
	d3dErr         error
	d3dCompileProc *syscall.Proc
	d3dStripProc   *syscall.Proc
	d3dDisasmProc  *syscall.Proc
)

func d3d() error {
	d3dOnce.Do(func() {
		dll, err := syscall.LoadDLL(filepath.Join(os.Getenv("SystemRoot"), "System32", "d3dcompiler_47.dll"))
		if err != nil {
			d3dErr = fmt.Errorf("the shader compiler of Windows (d3dcompiler_47.dll) could not be loaded: %w", err)
			return
		}
		for name, p := range map[string]**syscall.Proc{"D3DCompile": &d3dCompileProc, "D3DStripShader": &d3dStripProc, "D3DDisassemble": &d3dDisasmProc} {
			if *p, err = dll.FindProc(name); err != nil {
				d3dErr = fmt.Errorf("d3dcompiler_47.dll: %w", err)
				return
			}
		}
	})
	return d3dErr
}

// blobBytes copies an ID3DBlob's content and releases it.
func blobBytes(blob uintptr) []byte {
	if blob == 0 {
		return nil
	}
	vtbl := *(**[5]uintptr)(unsafe.Pointer(blob))
	ptr, _, _ := syscall.SyscallN(vtbl[3], blob)
	size, _, _ := syscall.SyscallN(vtbl[4], blob)
	out := make([]byte, size)
	copy(out, unsafe.Slice((*byte)(unsafe.Pointer(ptr)), size))
	syscall.SyscallN(vtbl[2], blob)
	return out
}

const (
	d3dCompileOptimization3 = 1 << 15
	// Reflection, debug, test and private data: Unity strips the same from what it ships.
	d3dStripAll = 1 | 2 | 4 | 8
)

// compileHLSL compiles one program (target "ps_4_0") and returns stripped DXBC. Compiler messages come back as the
// error text, one "(line,col-col): error X…: …" per line.
func compileHLSL(source, entry, target string, defines []string) ([]byte, error) {
	if err := d3d(); err != nil {
		return nil, err
	}
	type macro struct{ name, def *byte }
	var keep [][]byte
	cstr := func(s string) *byte {
		b := append([]byte(s), 0)
		keep = append(keep, b)
		return &b[0]
	}
	var macros []macro
	for _, d := range defines {
		macros = append(macros, macro{cstr(d), cstr("1")})
	}
	macros = append(macros, macro{})
	src := []byte(source)
	var code, msgs uintptr
	hr, _, _ := d3dCompileProc.Call(
		uintptr(unsafe.Pointer(&src[0])), uintptr(len(src)), 0,
		uintptr(unsafe.Pointer(&macros[0])), 0,
		uintptr(unsafe.Pointer(cstr(entry))), uintptr(unsafe.Pointer(cstr(target))),
		d3dCompileOptimization3, 0,
		uintptr(unsafe.Pointer(&code)), uintptr(unsafe.Pointer(&msgs)))
	runtime.KeepAlive(keep)
	runtime.KeepAlive(macros)
	runtime.KeepAlive(src)
	text := strings.TrimSpace(string(blobBytes(msgs)))
	if int32(hr) < 0 {
		blobBytes(code)
		if text == "" {
			text = fmt.Sprintf("D3DCompile failed (0x%08X)", uint32(hr))
		}
		return nil, fmt.Errorf("%s", text)
	}
	dxbc := blobBytes(code)
	var stripped uintptr
	hr, _, _ = d3dStripProc.Call(uintptr(unsafe.Pointer(&dxbc[0])), uintptr(len(dxbc)), d3dStripAll, uintptr(unsafe.Pointer(&stripped)))
	if int32(hr) < 0 {
		return nil, fmt.Errorf("D3DStripShader failed (0x%08X)", uint32(hr))
	}
	return blobBytes(stripped), nil
}

// disassemble returns the readable listing of a compiled program.
func disassemble(dxbc []byte) (string, error) {
	if err := d3d(); err != nil {
		return "", err
	}
	var out uintptr
	hr, _, _ := d3dDisasmProc.Call(uintptr(unsafe.Pointer(&dxbc[0])), uintptr(len(dxbc)), 0, 0, uintptr(unsafe.Pointer(&out)))
	if int32(hr) < 0 {
		return "", fmt.Errorf("D3DDisassemble failed (0x%08X)", uint32(hr))
	}
	return strings.TrimRight(string(blobBytes(out)), "\x00"), nil
}
