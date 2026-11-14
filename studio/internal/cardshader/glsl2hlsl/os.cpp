// The one function glslang expects from its per-OS layer; nothing to do in WebAssembly.
#include "glslang/OSDependent/osinclude.h"

namespace glslang {
void OS_DumpMemoryCounters() {}
}
