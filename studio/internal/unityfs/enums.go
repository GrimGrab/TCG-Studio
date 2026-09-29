package unityfs

import (
	"bytes"
	"debug/pe"
	"encoding/binary"
	"fmt"
	"os"
)

// ReadEnums reads enum members (name → value) from a .NET assembly's metadata (ECMA-335 #~ tables), e.g. the game's
// EItemType from Managed\Assembly-CSharp.dll, so Studio follows the installed game version instead of a copied list.
func ReadEnums(dll string, names ...string) (out map[string]map[string]int32, err error) {
	defer catch(&err, "enums")
	raw, err := os.ReadFile(dll)
	if err != nil {
		return nil, err
	}
	pf, err := pe.NewFile(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	var cliRVA uint32
	switch oh := pf.OptionalHeader.(type) {
	case *pe.OptionalHeader32:
		cliRVA = oh.DataDirectory[14].VirtualAddress
	case *pe.OptionalHeader64:
		cliRVA = oh.DataDirectory[14].VirtualAddress
	}
	at := func(rva uint32) []byte {
		for _, s := range pf.Sections {
			if rva >= s.VirtualAddress && rva < s.VirtualAddress+s.VirtualSize {
				return raw[s.Offset+(rva-s.VirtualAddress):]
			}
		}
		panic(fmt.Errorf("rva %x outside sections", rva))
	}
	if cliRVA == 0 {
		return nil, fmt.Errorf("%s is not a .NET assembly", dll)
	}
	cli := at(cliRVA)
	meta := at(binary.LittleEndian.Uint32(cli[8:]))
	if binary.LittleEndian.Uint32(meta) != 0x424A5342 {
		return nil, fmt.Errorf("no metadata root")
	}
	r := &reader{b: meta}
	r.skip(12)
	r.skip(int(r.u32())) // version string
	r.u16()
	streams := map[string][]byte{}
	for i, n := 0, int(r.u16()); i < n; i++ {
		off, size := r.u32(), r.u32()
		name := r.cstr()
		r.align()
		streams[name] = meta[off : off+size]
	}
	tbl, strs, blob := streams["#~"], streams["#Strings"], streams["#Blob"]
	if tbl == nil || strs == nil {
		return nil, fmt.Errorf("no #~ / #Strings stream")
	}
	t := &reader{b: tbl}
	t.skip(6)
	heap := t.u8()
	t.u8()
	valid := t.u64()
	t.u64()
	var rows [64]int
	for i := 0; i < 64; i++ {
		if valid>>i&1 != 0 {
			rows[i] = int(t.u32())
		}
	}
	strSz, guidSz, blobSz := 2+2*int(heap&1), 2+2*int(heap>>1&1), 2+2*int(heap>>2&1)
	idx := func(table int) int {
		if rows[table] < 1<<16 {
			return 2
		}
		return 4
	}
	coded := func(bits int, tables ...int) int {
		for _, tb := range tables {
			if rows[tb] >= 1<<(16-bits) {
				return 4
			}
		}
		return 2
	}
	typeDefOrRef := coded(2, 2, 1, 0x1B)
	sizes := map[int]int{
		0x00: 2 + strSz + 3*guidSz,
		0x01: coded(2, 0x00, 0x1A, 0x23, 0x01) + 2*strSz,
		0x02: 4 + 2*strSz + typeDefOrRef + idx(0x04) + idx(0x06),
		0x03: idx(0x04),
		0x04: 2 + strSz + blobSz,
		0x05: idx(0x06),
		0x06: 4 + 2 + 2 + strSz + blobSz + idx(0x08),
		0x07: idx(0x08),
		0x08: 2 + 2 + strSz,
		0x09: idx(0x02) + typeDefOrRef,
		0x0A: coded(3, 0x02, 0x01, 0x1A, 0x06, 0x1B) + strSz + blobSz,
		0x0B: 2 + coded(2, 0x04, 0x08, 0x17) + blobSz,
	}
	start := map[int]int{}
	p := t.p
	for i := 0; i <= 0x0B; i++ {
		start[i] = p
		p += rows[i] * sizes[i]
	}
	rd := func(b []byte, n int) int {
		if n == 2 {
			return int(binary.LittleEndian.Uint16(b))
		}
		return int(binary.LittleEndian.Uint32(b))
	}
	str := func(i int) string {
		e := bytes.IndexByte(strs[i:], 0)
		return string(strs[i : i+e])
	}
	row := func(table, i int) []byte { return tbl[start[table]+(i-1)*sizes[table]:] }

	// Field row → constant value.
	consts := map[int]int32{}
	hasConst := coded(2, 0x04, 0x08, 0x17)
	for i := 1; i <= rows[0x0B]; i++ {
		b := row(0x0B, i)
		parent := rd(b[2:], hasConst)
		if parent&3 != 0 { // not a Field
			continue
		}
		v := blob[rd(b[2+hasConst:], blobSz):]
		n := int(v[0])
		if v[0]&0x80 != 0 {
			continue
		}
		v = v[1:]
		switch {
		case n >= 4:
			consts[parent>>2] = int32(binary.LittleEndian.Uint32(v))
		case n == 2:
			consts[parent>>2] = int32(int16(binary.LittleEndian.Uint16(v)))
		case n == 1:
			consts[parent>>2] = int32(int8(v[0]))
		}
	}

	want := map[string]bool{}
	for _, n := range names {
		want[n] = true
	}
	out = map[string]map[string]int32{}
	fieldListOff := 4 + 2*strSz + typeDefOrRef
	for i := 1; i <= rows[0x02]; i++ {
		b := row(0x02, i)
		name := str(rd(b[4:], strSz))
		if !want[name] {
			continue
		}
		first := rd(b[fieldListOff:], idx(0x04))
		last := rows[0x04] + 1
		if i < rows[0x02] {
			last = rd(row(0x02, i+1)[fieldListOff:], idx(0x04))
		}
		members := map[string]int32{}
		for f := first; f < last; f++ {
			fb := row(0x04, f)
			if binary.LittleEndian.Uint16(fb)&0x40 == 0 { // not literal (value__)
				continue
			}
			if v, ok := consts[f]; ok {
				members[str(rd(fb[2:], strSz))] = v
			}
		}
		out[name] = members
	}
	for _, n := range names {
		if out[n] == nil {
			return nil, fmt.Errorf("enum %s not found in %s", n, dll)
		}
	}
	return out, nil
}
