/* Minimal stand-in for decoders.wasm (see stdlib.h). */
#pragma once
#include <stddef.h>
#include <string.h>
#ifdef __cplusplus
extern "C" {
#endif
size_t malloc_usable_size(void* p);
#ifdef __cplusplus
}
#endif
