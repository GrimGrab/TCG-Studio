// TCG Custom Cards — holographic foil for full-image cards (built-in render pipeline, UI).
// Drawn by an Image that uses the card artwork itself as its sprite, on top of the art: outputs only the foil light (additive),
// clipped by the artwork's alpha, so it never darkens the art and rounded corners come for free.
// Structure (stencil / ColorMask / clip rect / alpha clip) follows Unity's built-in UI/Default shader (MIT licence).
// Holo technique after the Cyanilux "Holofoil Card Shader Breakdown" and daniel-ilett/shaders-holo-card (MIT):
// a colour ramp sampled at (uv direction + view tilt + distortion), masked, plus sparkle/etch patterns.
// Motion is time-driven like the game's own foil (bands drift, a light sweep crosses the card); reaction to the camera angle is
// scaled by _ViewAmount (0 = the look doesn't change when the player moves or looks around).
Shader "TCGCC/HoloFoil"
{
    Properties
    {
        [PerRendererData] _MainTex ("Artwork", 2D) = "white" {}
        _Color ("Tint", Color) = (1,1,1,1)

        _Ramp ("Colour ramp (1D, repeat)", 2D) = "white" {}
        _Strength ("Strength", Range(0, 2)) = 0.5
        _Tiling ("Band tiling", Float) = 1.4
        _Parallax ("View parallax", Float) = 1.6
        _Distort ("Distortion", Float) = 0.25
        _Speed ("Drift speed", Float) = 0.03
        _Bands ("Band contrast", Range(0, 1)) = 0.6
        _ArtMask ("Art mask (0 flat, 1 bright, 2 dark)", Float) = 0
        _Pattern ("Pattern (0 none, 1 sparkle, 2 etched, 3 cosmos)", Float) = 0
        _Sparkle ("Sparkle amount", Range(0, 1)) = 0.5
        _PatternScale ("Pattern scale", Float) = 60
        _Edge ("Edge glow", Range(0, 1)) = 0.2
        _ViewAmount ("View-angle reaction", Range(0, 1)) = 0
        _Sweep ("Light sweep", Range(0, 2)) = 0.6
        _SweepSpeed ("Light sweep speed", Float) = 0.15

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
        Blend One One
        ColorMask [_ColorMask]

        Pass
        {
            Name "HoloFoil"
        CGPROGRAM
            #pragma vertex vert
            #pragma fragment frag
            #pragma target 3.0

            #include "UnityCG.cginc"
            #include "UnityUI.cginc"

            #pragma multi_compile_local _ UNITY_UI_CLIP_RECT
            #pragma multi_compile_local _ UNITY_UI_ALPHACLIP

            struct appdata_t
            {
                float4 vertex   : POSITION;
                float4 color    : COLOR;
                float2 texcoord : TEXCOORD0;
                UNITY_VERTEX_INPUT_INSTANCE_ID
            };

            struct v2f
            {
                float4 vertex        : SV_POSITION;
                fixed4 color         : COLOR;
                float2 texcoord      : TEXCOORD0;
                float4 worldPosition : TEXCOORD1;
                float3 right         : TEXCOORD2;
                float3 up            : TEXCOORD3;
                UNITY_VERTEX_OUTPUT_STEREO
            };

            sampler2D _MainTex;
            float4 _MainTex_ST;
            fixed4 _Color;
            fixed4 _TextureSampleAdd;
            float4 _ClipRect;

            sampler2D _Ramp;
            float _Strength, _Tiling, _Parallax, _Distort, _Speed, _Bands;
            float _ArtMask, _Pattern, _Sparkle, _PatternScale, _Edge;
            float _ViewAmount, _Sweep, _SweepSpeed;
            // Globals (Shader.SetGlobalFloat) so every copy of the material — incl. UI mask/stencil copies — sees them. Unset = 0 = live.
            // The mod's card snapshots set _TCGCC_HoloFreeze = 1 for the moment of a picture: a still frame at a fixed moment
            // instead of a random one (between two light sweeps the holo looked absent).
            float _TCGCC_HoloFreeze, _TCGCC_HoloTime;

            float HoloTime() { return _TCGCC_HoloFreeze > 0.5 ? _TCGCC_HoloTime : _Time.y; }
            // Frozen: the moment the light sweep crosses the middle of the card (sweep coordinate 0.4 -> 0.5).
            float SweepTime() { return _TCGCC_HoloFreeze > 0.5 ? (_SweepSpeed > 0.0001 ? 0.9 / _SweepSpeed : 0) : _Time.y; }

            v2f vert(appdata_t v)
            {
                v2f o;
                UNITY_SETUP_INSTANCE_ID(v);
                UNITY_INITIALIZE_VERTEX_OUTPUT_STEREO(o);
                o.worldPosition = mul(unity_ObjectToWorld, v.vertex);
                o.vertex = UnityObjectToClipPos(v.vertex);
                o.texcoord = TRANSFORM_TEX(v.texcoord, _MainTex);
                o.color = v.color * _Color;
                // The card plane's axes in world space (UI vertices live in the canvas/card plane).
                o.right = normalize(mul((float3x3)unity_ObjectToWorld, float3(1, 0, 0)));
                o.up = normalize(mul((float3x3)unity_ObjectToWorld, float3(0, 1, 0)));
                return o;
            }

            float hash21(float2 p)
            {
                p = frac(p * float2(123.34, 456.21));
                p += dot(p, p + 45.32);
                return frac(p.x * p.y);
            }

            float valueNoise(float2 p)
            {
                float2 i = floor(p), f = frac(p);
                float2 u = f * f * (3.0 - 2.0 * f);
                float a = hash21(i), b = hash21(i + float2(1, 0)), c = hash21(i + float2(0, 1)), d = hash21(i + float2(1, 1));
                return lerp(lerp(a, b, u.x), lerp(c, d, u.x), u.y);
            }

            // Twinkling point sparkles in a jittered grid; brightness depends on view tilt so they flash as the card moves.
            float sparkles(float2 uv, float2 tilt, float scale, float amount)
            {
                float2 grid = uv * float2(scale, scale * 1.4);
                float2 cell = floor(grid);
                float2 f = frac(grid);
                float h = hash21(cell);
                float h2 = hash21(cell + 17.13);
                float2 center = float2(0.2 + 0.6 * h, 0.2 + 0.6 * h2);
                float d = length(f - center);
                float star = saturate(1.0 - d * 6.0);
                star = star * star * star;
                float present = step(1.0 - amount * 0.45, hash21(cell + 3.7));
                float phase = h * 43.0 + dot(tilt, float2(21.0, 13.0)) + HoloTime() * (0.6 + h2);
                float twinkle = pow(saturate(sin(phase) * 0.5 + 0.5), 6.0);
                return star * present * twinkle;
            }

            fixed4 frag(v2f i) : SV_Target
            {
                half4 art = (tex2D(_MainTex, i.texcoord) + _TextureSampleAdd) * i.color;

                // View tilt in the card plane (world-space UI); screen-space UI simply gets the time drift.
                float3 viewDir = normalize(_WorldSpaceCameraPos.xyz - i.worldPosition.xyz);
                float2 tilt = float2(dot(viewDir, i.right), dot(viewDir, i.up)) * _ViewAmount;
                float3 normal = normalize(cross(i.right, i.up));
                float facing = abs(dot(viewDir, normal));

                // Holo bands: colour ramp coordinate from uv direction + view tilt + distortion (Cyanilux-style).
                float2 uv = i.texcoord;
                float distortion = valueNoise(uv * 5.0) * 0.6 + valueNoise(uv * 13.0) * 0.4;
                float coord = (uv.x * 0.6 + uv.y) * _Tiling
                            + (tilt.x * 0.9 + tilt.y * 0.6) * _Parallax
                            + distortion * _Distort
                            + HoloTime() * _Speed;
                float3 holo = tex2D(_Ramp, float2(frac(coord), 0.5)).rgb;
                float band = pow(saturate(sin(coord * 6.2831853 * 1.5) * 0.5 + 0.5), 2.0);
                float intensity = lerp(1.0, 0.25 + 0.75 * band, _Bands);

                // Art-aware masks: reverse holo on bright areas, or on dark areas.
                float lum = dot(art.rgb, float3(0.299, 0.587, 0.114));
                float mask = 1.0;
                if (_ArtMask > 0.5 && _ArtMask < 1.5) mask = smoothstep(0.3, 0.85, lum);
                else if (_ArtMask > 1.5) mask = 1.0 - smoothstep(0.15, 0.6, lum);

                float3 col = holo * intensity * mask;

                // Patterns.
                float3 extra = 0;
                if (_Pattern > 0.5 && _Pattern < 1.5)          // sparkle
                {
                    extra += sparkles(uv, tilt, _PatternScale, _Sparkle) * (holo + 0.6);
                }
                else if (_Pattern > 1.5 && _Pattern < 2.5)     // etched diagonal lines that catch the light
                {
                    float lines = frac((uv.x + uv.y * 1.4) * _PatternScale * 0.25);
                    float etch = smoothstep(0.85, 1.0, lines) * (0.4 + 0.6 * band);
                    extra += etch * holo * 1.2;
                    extra += sparkles(uv, tilt, _PatternScale * 0.5, _Sparkle * 0.5) * (holo + 0.5);
                }
                else if (_Pattern > 2.5)                       // cosmos: soft nebula clouds + dense sparkles
                {
                    float cloud = valueNoise(uv * 3.0 + tilt * 0.8) * valueNoise(uv * 7.0 - tilt * 0.5);
                    col *= 0.6 + 1.2 * cloud;
                    extra += sparkles(uv, tilt, _PatternScale, _Sparkle) * (holo + 0.8);
                    extra += sparkles(uv + 0.37, tilt * 1.3, _PatternScale * 1.7, _Sparkle * 0.7) * holo;
                }

                // Edge glow at grazing angles (only with view reaction).
                float edge = pow(1.0 - facing, 2.0) * _Edge * _ViewAmount;

                // Soft light sweep travelling diagonally across the card over time (like the game's drifting glow).
                float sweepPos = frac((uv.x * 0.6 + uv.y) * 0.5 - SweepTime() * _SweepSpeed);
                float sweepD = (sweepPos - 0.5) / 0.07;
                float sweep = exp(-sweepD * sweepD) * _Sweep;
                float3 sweepCol = (holo * 0.7 + 0.3) * sweep * lerp(0.5, 1.0, mask);

                float3 result = (col + extra + holo * edge + sweepCol) * _Strength * art.a;

                #ifdef UNITY_UI_CLIP_RECT
                result *= UnityGet2DClipping(i.worldPosition.xy, _ClipRect);
                #endif
                #ifdef UNITY_UI_ALPHACLIP
                clip(art.a - 0.001);
                #endif

                return fixed4(result, 0);
            }
        ENDCG
        }
    }
}
