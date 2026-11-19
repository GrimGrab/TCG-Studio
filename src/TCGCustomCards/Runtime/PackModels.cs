using System;
using System.Collections.Generic;
using System.IO;
using HarmonyLib;
using Newtonsoft.Json;
using TCGCustomCards.Core;
using UnityEngine;

namespace TCGCustomCards.Runtime
{
    /// <summary>
    /// The pack models TCG Studio builds from the game files (studio-paths.json "packModels", a folder):
    /// <list type="bullet">
    /// <item><c>&lt;shelf mesh&gt;.fixed.obj</c>: the game's shelf pack with one continuous back. The game's own model reads part
    /// of the back's texture twice (beside and under the seam fin). Custom packs use it (<see cref="Shelf"/>, from
    /// ItemInjector) because TCG Studio paints their art for it. Vanilla packs keep the game's model, whose art was painted for it.</item>
    /// <item><c>packopen.json</c> / <c>packopen.fixed.json</c> ([Visuals] ShelfPackInOpening): the pack in the pack opening
    /// (<c>CardOpeningSequence.m_CardPackMesh</c>, the skinned <c>CardPack_Mesh</c> that <c>PackOpenAnim</c> folds shut and tears
    /// open). It is the same model with the same bones and weights, its outside cut along the vanilla / fixed shelf pack's faces
    /// and given their UVs, so a pack opens looking as it does on the shelf. Picked per pack as it is readied. Only the renderer's
    /// mesh changes; the animation, materials and property blocks are the game's. With the setting off, the game's mesh is back.</item>
    /// </list>
    /// </summary>
    internal static class PackModels
    {
        private const string Feature = "Studio's pack models";
        private const int OpenFormatVersion = 1;
        private const string FixedSuffix = ".fixed.obj";
        private const string OpenFile = "packopen.json", OpenFixedFile = "packopen.fixed.json";

        private static bool _shelfFixed; // custom packs got the fixed shelf pack (this game start)
        private static bool _shelfMissingLogged;
        private static SkinnedMeshRenderer _renderer; // the current scene's opening pack (destroyed on a scene reload)
        private static Mesh _original;
        private static readonly Dictionary<string, Opening> Openings = new Dictionary<string, Opening>(); // file → built (null = failed)

        private class Opening
        {
            public Mesh Mesh;
            public string[] Bones;
            public string Source;
        }

        private class FileData
        {
            [JsonProperty("version")] public int Version { get; set; }
            [JsonProperty("mesh")] public string Mesh { get; set; }
            [JsonProperty("bones")] public string[] Bones { get; set; }
            [JsonProperty("bindPoses")] public float[] BindPoses { get; set; }
            [JsonProperty("positions")] public float[] Positions { get; set; }
            [JsonProperty("normals")] public float[] Normals { get; set; }
            [JsonProperty("uvs")] public float[] Uvs { get; set; }
            [JsonProperty("boneIndices")] public int[] BoneIndices { get; set; }
            [JsonProperty("boneWeights")] public float[] BoneWeights { get; set; }
            [JsonProperty("triangles")] public int[] Triangles { get; set; }
        }

        /// <summary>The fixed shelf pack made from <paramref name="vanilla"/>, or null (no file from TCG Studio: the game's model stays).</summary>
        internal static Mesh Shelf(Mesh vanilla)
        {
            if (vanilla == null || StudioPaths.PackModels == null || !Compat.Ok(Feature)) return null;
            string file = Path.Combine(StudioPaths.PackModels, vanilla.name + FixedSuffix);
            if (!File.Exists(file))
            {
                if (!_shelfMissingLogged)
                    Plugin.Log.LogInfo($"Pack models: no fixed shelf pack for '{vanilla.name}' from TCG Studio yet; custom packs use the game's model");
                _shelfMissingLogged = true;
                return null;
            }
            var mesh = MeshLoader.Get(file);
            if (mesh == null) return null;
            if (!_shelfFixed) Plugin.Log.LogInfo($"Pack models: custom packs use the shelf pack with a fixed back ({mesh.vertexCount} vertices)");
            _shelfFixed = true;
            return mesh;
        }

        [HarmonyPatch(typeof(CardOpeningSequence), "Start")]
        private static class CardOpeningSequenceStartPatch
        {
            private static void Postfix(CardOpeningSequence __instance)
            {
                var r = __instance.m_CardPackMesh;
                if (r == null) return;
                _renderer = r;
                _original = r.sharedMesh;
            }
        }

        // The pack about to be opened: the opening pack made from the same shelf model as its own.
        [HarmonyPatch(typeof(CardOpeningSequence), nameof(CardOpeningSequence.ReadyingCardPack))]
        private static class ReadyingCardPackPatch
        {
            private static void Prefix(Item item)
            {
                if (_renderer == null || item == null) return;
                Mesh want = null;
                if (Plugin.ShelfPackInOpening.Value && Compat.Ok(Feature))
                    want = Open(_shelfFixed && Registry.IsCustomPackItem(item.GetItemType()) ? OpenFixedFile : OpenFile);
                var mesh = want != null ? want : _original;
                if (mesh != null && _renderer.sharedMesh != mesh) _renderer.sharedMesh = mesh;
            }
        }

        /// <summary>The opening pack in <paramref name="file"/> when it fits this game's opening pack, else null (reported once).</summary>
        private static Mesh Open(string file)
        {
            if (!Openings.TryGetValue(file, out var o))
            {
                o = Build(file);
                Openings[file] = o;
            }
            return o != null && Fits(o) ? o.Mesh : null;
        }

        /// <summary>The renderer has the bones the file was made for, in the same order (names), and the mesh it was made from.</summary>
        private static bool Fits(Opening o)
        {
            var bones = _renderer.bones;
            if (bones == null || bones.Length != o.Bones.Length)
            {
                Compat.Report(Feature, $"the opening pack has {bones?.Length ?? 0} bones, TCG Studio's pack {o.Bones.Length}");
                return false;
            }
            for (int i = 0; i < bones.Length; i++)
                if (bones[i] == null || bones[i].name != o.Bones[i])
                {
                    Compat.Report(Feature, $"the opening pack's bone {i} is '{(bones[i] != null ? bones[i].name : "none")}', TCG Studio's pack has '{o.Bones[i]}'");
                    return false;
                }
            if (_original != null && _original.name != o.Source)
            {
                Compat.Report(Feature, $"the opening pack's model is '{_original.name}', TCG Studio's pack was made from '{o.Source}'");
                return false;
            }
            return true;
        }

        private static Opening Build(string name)
        {
            if (StudioPaths.PackModels == null)
            {
                Plugin.Log.LogInfo("Pack opening: no pack models from TCG Studio yet (open TCG Studio once); the game's own opening pack is used");
                return null;
            }
            string file = Path.Combine(StudioPaths.PackModels, name);
            if (!File.Exists(file))
            {
                Plugin.Log.LogInfo($"Pack opening: {name} is missing (open TCG Studio once); the game's own opening pack is used");
                return null;
            }
            try
            {
                var d = JsonConvert.DeserializeObject<FileData>(File.ReadAllText(file));
                if (d == null || d.Version != OpenFormatVersion)
                {
                    Compat.Report(Feature, $"{name} is version {d?.Version}, this mod reads {OpenFormatVersion} (update TCG Studio and the mod together)");
                    return null;
                }
                int n = d.Positions.Length / 3;
                if (d.Bones == null || d.BindPoses.Length != 16 * d.Bones.Length || d.Normals.Length != 3 * n || d.Uvs.Length != 2 * n ||
                    d.BoneIndices.Length != 4 * n || d.BoneWeights.Length != 4 * n || d.Triangles.Length % 3 != 0)
                {
                    Compat.Report(Feature, $"{name} is incomplete");
                    return null;
                }
                foreach (int t in d.Triangles)
                    if (t < 0 || t >= n)
                    {
                        Compat.Report(Feature, $"{name} has a triangle outside its vertices");
                        return null;
                    }
                var pos = new Vector3[n];
                var nrm = new Vector3[n];
                var uv = new Vector2[n];
                var bw = new BoneWeight[n];
                for (int i = 0; i < n; i++)
                {
                    pos[i] = new Vector3(d.Positions[3 * i], d.Positions[3 * i + 1], d.Positions[3 * i + 2]);
                    nrm[i] = new Vector3(d.Normals[3 * i], d.Normals[3 * i + 1], d.Normals[3 * i + 2]);
                    uv[i] = new Vector2(d.Uvs[2 * i], d.Uvs[2 * i + 1]);
                    bw[i] = new BoneWeight
                    {
                        boneIndex0 = d.BoneIndices[4 * i], weight0 = d.BoneWeights[4 * i],
                        boneIndex1 = d.BoneIndices[4 * i + 1], weight1 = d.BoneWeights[4 * i + 1],
                        boneIndex2 = d.BoneIndices[4 * i + 2], weight2 = d.BoneWeights[4 * i + 2],
                        boneIndex3 = d.BoneIndices[4 * i + 3], weight3 = d.BoneWeights[4 * i + 3],
                    };
                }
                var binds = new Matrix4x4[d.Bones.Length];
                for (int b = 0; b < binds.Length; b++)
                {
                    var m = new Matrix4x4();
                    for (int row = 0; row < 4; row++)
                        m.SetRow(row, new Vector4(d.BindPoses[16 * b + 4 * row], d.BindPoses[16 * b + 4 * row + 1],
                            d.BindPoses[16 * b + 4 * row + 2], d.BindPoses[16 * b + 4 * row + 3]));
                    binds[b] = m;
                }
                var mesh = new Mesh { name = "TCGCC_" + Path.GetFileNameWithoutExtension(name) };
                if (n > 65535) mesh.indexFormat = UnityEngine.Rendering.IndexFormat.UInt32;
                mesh.vertices = pos;
                mesh.normals = nrm;
                mesh.uv = uv;
                mesh.boneWeights = bw;
                mesh.bindposes = binds;
                mesh.triangles = d.Triangles;
                mesh.RecalculateBounds();
                mesh.UploadMeshData(true);
                Plugin.Log.LogInfo($"Pack opening: {name} loaded ({n} vertices, {d.Triangles.Length / 3} triangles, from {d.Mesh})");
                return new Opening { Mesh = mesh, Bones = d.Bones, Source = d.Mesh };
            }
            catch (Exception e)
            {
                Compat.Report(Feature, $"couldn't read {name}: {e.Message}");
                return null;
            }
        }
    }
}
