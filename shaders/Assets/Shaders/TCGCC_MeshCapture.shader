// TCG Custom Cards — reads back mesh geometry the CPU can't access (the game's meshes aren't readable and DX11 won't hand
// their vertex buffers to GetData). Used once by the template export (Debug/AccessoryTemplateExport), never drawn in the scene.
//  Pass 0: a geometry shader writes every triangle (object-space positions, normals, UV0) to the UAV _Tris at
//          _TriOffset + SV_PrimitiveID. Needs UAV access outside the pixel shader (D3D 11.1+).
//  Pass 1: fallback — rasterises the mesh in UV space and outputs object-space position (RGB, float target).
//  Pass 2: same, outputs the object-space normal.
Shader "Hidden/TCGCC/MeshCapture"
{
    SubShader
    {
        Cull Off ZWrite Off ZTest Always

        CGINCLUDE
        #include "UnityCG.cginc"
        struct appdata { float4 vertex : POSITION; float3 normal : NORMAL; float2 uv : TEXCOORD0; };
        struct v2f { float4 pos : SV_POSITION; float3 op : TEXCOORD0; float3 on : TEXCOORD1; float2 uv : TEXCOORD2; };

        v2f vertUV(appdata v)
        {
            v2f o;
            float2 p = v.uv * 2 - 1;
            #if UNITY_UV_STARTS_AT_TOP
            p.y = -p.y;
            #endif
            o.pos = float4(p, 0.5, 1);
            o.op = v.vertex.xyz;
            o.on = v.normal;
            o.uv = v.uv;
            return o;
        }
        ENDCG

        Pass
        {
            CGPROGRAM
            #pragma target 5.0
            #pragma vertex vertUV
            #pragma geometry geom
            #pragma fragment frag
            struct Tri { float3 p0; float3 p1; float3 p2; float3 n0; float3 n1; float3 n2; float2 u0; float2 u1; float2 u2; };
            RWStructuredBuffer<Tri> _Tris : register(u1);
            int _TriOffset;

            [maxvertexcount(3)]
            void geom(triangle v2f i[3], uint pid : SV_PrimitiveID, inout TriangleStream<v2f> s)
            {
                Tri t;
                t.p0 = i[0].op; t.p1 = i[1].op; t.p2 = i[2].op;
                t.n0 = i[0].on; t.n1 = i[1].on; t.n2 = i[2].on;
                t.u0 = i[0].uv; t.u1 = i[1].uv; t.u2 = i[2].uv;
                _Tris[_TriOffset + pid] = t;
                s.Append(i[0]); s.Append(i[1]); s.Append(i[2]);
            }
            float4 frag(v2f i) : SV_Target { return 0; }
            ENDCG
        }

        Pass
        {
            CGPROGRAM
            #pragma vertex vertUV
            #pragma fragment frag
            float4 frag(v2f i) : SV_Target { return float4(i.op, 1); }
            ENDCG
        }

        Pass
        {
            CGPROGRAM
            #pragma vertex vertUV
            #pragma fragment frag
            float4 frag(v2f i) : SV_Target { return float4(normalize(i.on), 1); }
            ENDCG
        }
    }
}
