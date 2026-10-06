// decoders.wasm: Unity's crunch transcoder (crn_decomp.h, unity branch, zlib licence) and bcdec's BC7 decoder
// (bcdec.h, used under the Unlicense), both unchanged, behind a tiny C interface for Studio (internal/unityfs/wasmdec.go).
// Built by tools\build-decoders-wasm.ps1. No C library is linked: the few runtime functions the headers need are below.
//
// Every call starts from an empty heap (dec_reset), so memory is never leaked between textures.

#include <stddef.h>
#include <stdint.h>
#include <stdarg.h>

extern "C" {

// ---------------------------------------------------------------- minimal runtime (bump heap)

extern unsigned char __heap_base;
static uintptr_t heap_top;

static uintptr_t align16(uintptr_t v) { return (v + 15) & ~(uintptr_t)15; }

// Each block has a 16-byte header holding its size.
void* malloc(size_t n) {
  uintptr_t p = align16(heap_top ? heap_top : (uintptr_t)&__heap_base);
  uintptr_t end = p + 16 + align16(n);
  size_t have = __builtin_wasm_memory_size(0) * 65536;
  if (end > have) {
    size_t pages = (end - have + 65535) / 65536;
    if (__builtin_wasm_memory_grow(0, pages) == (size_t)-1) return 0;
  }
  *(size_t*)p = n;
  heap_top = end;
  return (void*)(p + 16);
}

void free(void*) {}

size_t malloc_usable_size(void* p) { return p ? *(size_t*)((uintptr_t)p - 16) : 0; }

void* memcpy(void* d, const void* s, size_t n) {
  unsigned char* dd = (unsigned char*)d;
  const unsigned char* ss = (const unsigned char*)s;
  while (n--) *dd++ = *ss++;
  return d;
}

void* memmove(void* d, const void* s, size_t n) {
  unsigned char* dd = (unsigned char*)d;
  const unsigned char* ss = (const unsigned char*)s;
  if (dd < ss) {
    while (n--) *dd++ = *ss++;
  } else {
    while (n--) dd[n] = ss[n];
  }
  return d;
}

void* memset(void* d, int c, size_t n) {
  unsigned char* dd = (unsigned char*)d;
  while (n--) *dd++ = (unsigned char)c;
  return d;
}

int memcmp(const void* a, const void* b, size_t n) {
  const unsigned char *x = (const unsigned char*)a, *y = (const unsigned char*)b;
  for (; n; n--, x++, y++)
    if (*x != *y) return *x - *y;
  return 0;
}

void* realloc(void* p, size_t n) {
  if (!p) return malloc(n);
  size_t old = malloc_usable_size(p);
  if (n <= old) return p;
  void* q = malloc(n);
  if (q) memcpy(q, p, old);
  return q;
}

// crunch only formats text for its assert/trace messages; those are dropped (a failed unpack returns an error).
int sprintf(char* buf, const char*, ...) { if (buf) buf[0] = 0; return 0; }
int vsprintf(char* buf, const char*, va_list) { if (buf) buf[0] = 0; return 0; }
int puts(const char*) { return 0; }
int printf(const char*, ...) { return 0; }

void dec_reset(void) { heap_top = 0; }

// dec_alloc gives the caller (Go) a buffer to write input data into, after dec_reset.
void* dec_alloc(size_t n) { return malloc(n); }

}  // extern "C"

void* operator new(size_t n) { return malloc(n); }
void* operator new[](size_t n) { return malloc(n); }
void operator delete(void*) noexcept {}
void operator delete[](void*) noexcept {}
void operator delete(void*, size_t) noexcept {}
void operator delete[](void*, size_t) noexcept {}

#include "crn_decomp.h"

#define BCDEC_IMPLEMENTATION
#include "bcdec.h"

extern "C" {

// crunch_unpack transcodes a Unity crunched texture (DXT1Crunched / DXT5Crunched data = a .crn file) to the plain DXT
// blocks of mip level 0. Returns 0 on success and sets *out/*out_len/*width/*height/*block_bytes; negative = error.
int crunch_unpack(const void* src, uint32_t len, void** out, uint32_t* out_len, uint32_t* width, uint32_t* height,
                  uint32_t* block_bytes) {
  crnd::crn_texture_info info;
  info.m_struct_size = sizeof(info);
  if (!crnd::crnd_get_texture_info(src, len, &info)) return -1;
  uint32_t bw = (info.m_width + 3) / 4, bh = (info.m_height + 3) / 4;
  uint32_t bpb = crnd::crnd_get_bytes_per_dxt_block(info.m_format);
  uint32_t size = bw * bh * bpb;
  void* dst = malloc(size);
  if (!dst) return -2;
  crnd::crnd_unpack_context ctx = crnd::crnd_unpack_begin(src, len);
  if (!ctx) return -3;
  void* faces[1] = {dst};
  bool ok = crnd::crnd_unpack_level(ctx, faces, size, bw * bpb, 0);
  crnd::crnd_unpack_end(ctx);
  if (!ok) return -4;
  *out = dst;
  *out_len = size;
  *width = info.m_width;
  *height = info.m_height;
  *block_bytes = bpb;
  return 0;
}

// bc7_decode decodes w×h BC7 blocks (Unity's stored row order) into RGBA8, 4·w bytes per row. Returns the buffer.
void* bc7_decode(const void* src, uint32_t w, uint32_t h) {
  uint32_t bw = (w + 3) / 4, bh = (h + 3) / 4;
  uint32_t pw = bw * 4;  // padded width
  unsigned char* dst = (unsigned char*)malloc((size_t)pw * bh * 4 * 4);
  if (!dst) return 0;
  const unsigned char* s = (const unsigned char*)src;
  for (uint32_t by = 0; by < bh; by++)
    for (uint32_t bx = 0; bx < bw; bx++, s += 16)
      bcdec_bc7(s, dst + ((size_t)by * 4 * pw + bx * 4) * 4, (int)pw * 4);
  return dst;
}

}  // extern "C"
