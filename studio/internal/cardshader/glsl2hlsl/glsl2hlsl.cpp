// GLSL fragment shader -> HLSL, for TCG Studio's card shaders: glslang (GLSL -> SPIR-V) and SPIRV-Cross (SPIR-V ->
// HLSL), both unchanged, behind three calls. Built to glsl2hlsl.wasm by tools\build-glsl2hlsl-wasm.ps1.
//
//   cs_alloc(n)            memory for the source text
//   cs_convert(src, len)   1 = converted, 0 = the shader has errors; the text is then at cs_out() / cs_out_len()
//                          (the HLSL, or glslang's messages: "ERROR: 0:<line>: ...")
//
// The source's "layout(binding = N)" numbers become the HLSL registers (tN/sN for a sampler2D, bN for a uniform
// block) and "layout(location = N)" inputs become TEXCOORDN: that is how the result lines up with the template shader.
#include <cstdlib>
#include <string>
#include <vector>

#include "glslang/Public/ResourceLimits.h"
#include "glslang/Public/ShaderLang.h"
#include "SPIRV/GlslangToSpv.h"
#include "spirv_hlsl.hpp"

static std::string g_out;
static bool g_ready = false;

extern "C" {

__attribute__((export_name("cs_alloc"))) void *cs_alloc(int n) { return malloc(n > 0 ? n : 1); }

__attribute__((export_name("cs_out"))) const char *cs_out() { return g_out.data(); }

__attribute__((export_name("cs_out_len"))) int cs_out_len() { return (int)g_out.size(); }

__attribute__((export_name("cs_convert"))) int cs_convert(const char *src, int len)
{
    if (!g_ready)
    {
        glslang::InitializeProcess();
        g_ready = true;
    }
    g_out.clear();

    glslang::TShader shader(EShLangFragment);
    shader.setStringsWithLengths(&src, &len, 1);
    shader.setEnvInput(glslang::EShSourceGlsl, EShLangFragment, glslang::EShClientOpenGL, 100);
    shader.setEnvClient(glslang::EShClientOpenGL, glslang::EShTargetOpenGL_450);
    shader.setEnvTarget(glslang::EShTargetSpv, glslang::EShTargetSpv_1_0);
    if (!shader.parse(GetDefaultResources(), 300, false, EShMsgDefault))
    {
        g_out = shader.getInfoLog();
        return 0;
    }
    glslang::TProgram program;
    program.addShader(&shader);
    if (!program.link(EShMsgDefault))
    {
        g_out = program.getInfoLog();
        return 0;
    }
    std::vector<unsigned int> spirv;
    glslang::GlslangToSpv(*program.getIntermediate(EShLangFragment), spirv);

    spirv_cross::CompilerHLSL hlsl(std::move(spirv));
    spirv_cross::CompilerHLSL::Options options;
    options.shader_model = 40;
    hlsl.set_hlsl_options(options);
    g_out = hlsl.compile();
    return 1;
}

}
