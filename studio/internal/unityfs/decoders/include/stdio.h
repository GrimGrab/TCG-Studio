/* Minimal stand-in for decoders.wasm (see stdlib.h). */
#pragma once
#include <stdarg.h>
#ifdef __cplusplus
extern "C" {
#endif
int sprintf(char* buf, const char* fmt, ...);
int vsprintf(char* buf, const char* fmt, va_list args);
int puts(const char* s);
int printf(const char* fmt, ...);
#ifdef __cplusplus
}
#endif
