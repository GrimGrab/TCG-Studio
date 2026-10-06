/* Minimal stand-in for decoders.wasm: only what crn_decomp.h uses, implemented in decoders.cpp (no C library). */
#pragma once
#include <stddef.h>
#ifdef __cplusplus
extern "C" {
#endif
void* malloc(size_t n);
void free(void* p);
void* realloc(void* p, size_t n);
#ifdef __cplusplus
}
#endif
