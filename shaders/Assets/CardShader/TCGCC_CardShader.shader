// TCG Custom Cards — template for card shaders written by players in TCG Studio (built-in render pipeline, UI).
// Never shown as it is: Studio copies the built bundle and swaps this fragment program for the player's compiled one
// (studio/internal/cardshader). What the two sides agree on, and what a change here must keep in step with the
// prelude there:
//   - the fragment program's inputs: TEXCOORD0..5 below, in this order, SV_POSITION last;
//   - its constants: the one buffer TCGCCCard (register b0), laid out as written (the prelude reads the sixteen value
//     slots as one array _P[16]: the same bytes);
//   - its textures: _MainTex t0/s0, _Tex0.._Tex3 t1..t4, _Mask t5.
// The fragment program reads every one of them, so the binding data Unity stores with the shader covers them all.
// Everything that needs Unity's own constant buffers (time, camera, object matrix) is worked out in the vertex
// program. The mod's Image puts the card's own 0..1 coordinates in uv0, the art's place in its texture in uv1, and what
// it knows about the card in uv2 (two random numbers, when it appeared, when the pointer went onto it) and uv3 (border,
// foil, rarity, grade). A canvas that doesn't pass uv2/uv3 on gives zeros, which read as "nothing special".
// Structure (stencil / ColorMask / clip rect / alpha clip) follows Unity's built-in UI/Default shader (MIT licence).
Shader "TCGCC/CardShader"
{
    Properties
    {
        [PerRendererData] _MainTex ("Artwork", 2D) = "white" {}
        _Color ("Tint", Color) = (1,1,1,1)
        _Tex0 ("Image 0", 2D) = "white" {}
        _Tex1 ("Image 1", 2D) = "white" {}
        _Tex2 ("Image 2", 2D) = "white" {}
        _Tex3 ("Image 3", 2D) = "white" {}
        _Mask ("Card mask", 2D) = "white" {}
        // The value slots are declared one by one, not as an array: only declared properties are carried over when Unity
        // copies a material, which it does for every Image under a UI Mask (the copy would see zeros in an array).
        _P0 ("Value slot 0", Vector) = (0,0,0,0)
        _P1 ("Value slot 1", Vector) = (0,0,0,0)
        _P2 ("Value slot 2", Vector) = (0,0,0,0)
        _P3 ("Value slot 3", Vector) = (0,0,0,0)
        _P4 ("Value slot 4", Vector) = (0,0,0,0)
        _P5 ("Value slot 5", Vector) = (0,0,0,0)
        _P6 ("Value slot 6", Vector) = (0,0,0,0)
        _P7 ("Value slot 7", Vector) = (0,0,0,0)
        _P8 ("Value slot 8", Vector) = (0,0,0,0)
        _P9 ("Value slot 9", Vector) = (0,0,0,0)
        _P10 ("Value slot 10", Vector) = (0,0,0,0)
        _P11 ("Value slot 11", Vector) = (0,0,0,0)
        _P12 ("Value slot 12", Vector) = (0,0,0,0)
        _P13 ("Value slot 13", Vector) = (0,0,0,0)
        _P14 ("Value slot 14", Vector) = (0,0,0,0)
        _P15 ("Value slot 15", Vector) = (0,0,0,0)

        _StencilComp ("Stencil Comparison", Float) = 8
        _Stencil ("Stencil ID", Float) = 0
        _StencilOp ("Stencil Operation", Float) = 0
        _StencilWriteMask ("Stencil Write Mask", Float) = 255
        _StencilReadMask ("Stencil Read Mask", Float) = 255
        _ColorMask ("Color Mask", Float) = 15
        [Toggle(UNITY_UI_ALPHACLIP)] _UseUIAlphaClip ("Use Alpha Clip", Float) = 0
    }

    SubShader
    {
        Tags
        {
            "Queue"="Transparent"
            "IgnoreProjector"="True"
            "RenderType"="Transparent"
            "PreviewType"="Plane"
            "CanUseSpriteAtlas"="True"
        }

        Stencil
        {
            Ref [_Stencil]
            Comp [_StencilComp]
            Pass [_StencilOp]
            ReadMask [_StencilReadMask]
            WriteMask [_StencilWriteMask]
        }

        Cull Off
        Lighting Off
        ZWrite Off
        ZTest [unity_GUIZTestMode]
        Blend One OneMinusSrcAlpha
        ColorMask [_ColorMask]

        Pass
        {
            Name "CardShader"
        CGPROGRAM
            #pragma vertex vert
            #pragma fragment frag
            #pragma target 4.0

            #include "UnityCG.cginc"
            #include "UnityUI.cginc"

            #pragma multi_compile_local _ UNITY_UI_CLIP_RECT
            #pragma multi_compile_local _ UNITY_UI_ALPHACLIP

            struct appdata_t
            {
                float4 vertex    : POSITION;
                float4 color     : COLOR;
                float4 texcoord  : TEXCOORD0; // xy = place on the card, 0..1
                float4 texcoord1 : TEXCOORD1; // the art in its texture: xy = corner, zw = size
                float4 texcoord2 : TEXCOORD2; // x = random per copy on screen, y = random per card, z = time it appeared, w = time the pointer went onto it (0 = it is not on it)
                float4 texcoord3 : TEXCOORD3; // x = border, y = foil, z = rarity (place in the set's list), w = grade (0 = not graded)
            };

            struct v2f
            {
                float4 uvWorld : TEXCOORD0; // xy = place on the card, zw = world position xy (clip rect)
                float4 view    : TEXCOORD1; // xy = view tilt in the card plane, z = facing, w = time
                float4 art     : TEXCOORD2; // the art in its texture: xy = corner, zw = size
                float4 misc    : TEXCOORD3; // x = opacity, yz = art size in pixels
                float4 card    : TEXCOORD4; // x = random per copy, y = random per card, z = seconds since it appeared, w = seconds the pointer has been on it
                float4 facts   : TEXCOORD5; // x = border, y = foil, z = rarity, w = grade
                float4 vertex  : SV_POSITION;
            };

            sampler2D _MainTex;
            sampler2D _Tex0, _Tex1, _Tex2, _Tex3;
            sampler2D _Mask;
            float4 _MainTex_TexelSize;
            fixed4 _Color;
            // Globals set by the mod's card snapshots: a still frame at a fixed moment (shared with TCGCC/HoloFoil).
            float _TCGCC_HoloFreeze, _TCGCC_HoloTime;

            // The fragment program's constants. _P = the value slots a player shader's parameters are given.
            CBUFFER_START(TCGCCCard)
                float4 _TextureSampleAdd;
                float4 _ClipRect;
                float4 _P0;
                float4 _P1;
                float4 _P2;
                float4 _P3;
                float4 _P4;
                float4 _P5;
                float4 _P6;
                float4 _P7;
                float4 _P8;
                float4 _P9;
                float4 _P10;
                float4 _P11;
                float4 _P12;
                float4 _P13;
                float4 _P14;
                float4 _P15;
            CBUFFER_END

            v2f vert(appdata_t v)
            {
                v2f o;
                float4 world = mul(unity_ObjectToWorld, v.vertex);
                o.vertex = UnityObjectToClipPos(v.vertex);
                o.uvWorld = float4(v.texcoord.xy, world.xy);
                o.art = v.texcoord1;
                o.misc = float4(v.color.a * _Color.a, _MainTex_TexelSize.zw * v.texcoord1.zw, 0);
                // The card plane's axes in world space (UI vertices live in the canvas/card plane).
                float3 right = normalize(mul((float3x3)unity_ObjectToWorld, float3(1, 0, 0)));
                float3 up = normalize(mul((float3x3)unity_ObjectToWorld, float3(0, 1, 0)));
                float3 viewDir = normalize(_WorldSpaceCameraPos.xyz - world.xyz);
                float time = _TCGCC_HoloFreeze > 0.5 ? _TCGCC_HoloTime : _Time.y;
                o.view = float4(dot(viewDir, right), dot(viewDir, up), abs(dot(viewDir, normalize(cross(right, up)))), time);
                // A still frame shows the card as it is long after appearing, with no pointer on it.
                bool still = _TCGCC_HoloFreeze > 0.5;
                float shown = still ? 1000.0 : max(0.0, time - v.texcoord2.z);
                float hover = (still || v.texcoord2.w <= 0.0) ? 0.0 : max(0.0, time - v.texcoord2.w);
                o.card = float4(v.texcoord2.xy, shown, hover);
                o.facts = v.texcoord3;
                return o;
            }

            float4 frag(v2f i) : SV_Target
            {
                float2 uv = i.art.xy + saturate(i.uvWorld.xy) * i.art.zw;
                float4 col = tex2D(_MainTex, uv) + _TextureSampleAdd;
                // Read every input once, so none is dropped from the stored bindings.
                float4 slots = _P0 + _P1 + _P2 + _P3 + _P4 + _P5 + _P6 + _P7 + _P8 + _P9 + _P10 + _P11 + _P12 + _P13 + _P14 + _P15;
                float4 images = tex2D(_Tex0, uv) + tex2D(_Tex1, uv) + tex2D(_Tex2, uv) + tex2D(_Tex3, uv) + tex2D(_Mask, uv);
                col += (slots + images + i.view + i.misc + i.card + i.facts) * 0.000001;
                col.a *= i.misc.x;
                col.rgb *= col.a;

                #ifdef UNITY_UI_CLIP_RECT
                col *= UnityGet2DClipping(i.uvWorld.zw, _ClipRect);
                #endif
                #ifdef UNITY_UI_ALPHACLIP
                clip(col.a - 0.001);
                #endif
                return col;
            }
        ENDCG
        }
    }
}
