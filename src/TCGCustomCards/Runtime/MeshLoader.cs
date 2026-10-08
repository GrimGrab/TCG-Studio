using System;
using System.Collections.Generic;
using System.Globalization;
using System.IO;
using UnityEngine;
using UnityEngine.Rendering;

namespace TCGCustomCards.Runtime
{
    /// <summary>
    /// Loads the models TCG Studio writes (baked figurines/furniture, furniture paint meshes): OBJ already in Unity space (left-handed,
    /// Y up, UV origin bottom-left), one object. Reads v / vt / vn / f (polygons are fanned into triangles); each "usemtl" starts the
    /// next submesh (material names ignored), groups are ignored. Meshes are cached per path for the session.
    /// </summary>
    internal static class MeshLoader
    {
        /// <summary>Hard limit; TCG Studio keeps models well below this.</summary>
        public const int MaxTriangles = 200000;

        private static readonly Dictionary<string, Mesh> Cache = new Dictionary<string, Mesh>(StringComparer.OrdinalIgnoreCase);

        public static Mesh Get(string path)
        {
            if (Cache.TryGetValue(path, out var cached) && cached != null) return cached;
            Mesh mesh = null;
            try { mesh = LoadObj(path); }
            catch (Exception e) { Plugin.Log.LogError($"Model '{path}' failed to load: {e.Message}"); }
            Cache[path] = mesh;
            return mesh;
        }

        private static Mesh LoadObj(string path)
        {
            var inv = CultureInfo.InvariantCulture;
            var pos = new List<Vector3>();
            var uvs = new List<Vector2>();
            var nrms = new List<Vector3>();
            var outPos = new List<Vector3>();
            var outUv = new List<Vector2>();
            var outN = new List<Vector3>();
            var subs = new List<List<int>> { new List<int>() };
            var tris = subs[0];
            bool sawMaterial = false;
            int triCount = 0;
            var map = new Dictionary<(int, int, int), int>();
            var face = new List<int>();
            bool anyNormal = false;

            int Resolve(int i, int count) => i > 0 ? i - 1 : count + i;

            foreach (var raw in File.ReadLines(path))
            {
                string line = raw.Trim();
                if (line.Length < 2 || line[0] == '#') continue;
                var p = line.Split((char[])null, StringSplitOptions.RemoveEmptyEntries);
                switch (p[0])
                {
                    case "v":
                        pos.Add(new Vector3(float.Parse(p[1], inv), float.Parse(p[2], inv), float.Parse(p[3], inv)));
                        break;
                    case "vt":
                        uvs.Add(new Vector2(float.Parse(p[1], inv), p.Length > 2 ? float.Parse(p[2], inv) : 0f));
                        break;
                    case "usemtl":
                        // Triangles before the first usemtl (if any) keep submesh 0.
                        if (sawMaterial || tris.Count > 0) { tris = new List<int>(); subs.Add(tris); }
                        sawMaterial = true;
                        break;
                    case "vn":
                        nrms.Add(new Vector3(float.Parse(p[1], inv), float.Parse(p[2], inv), float.Parse(p[3], inv)));
                        break;
                    case "f":
                        face.Clear();
                        for (int k = 1; k < p.Length; k++)
                        {
                            var parts = p[k].Split('/');
                            int vi = Resolve(int.Parse(parts[0], inv), pos.Count);
                            int ti = parts.Length > 1 && parts[1].Length > 0 ? Resolve(int.Parse(parts[1], inv), uvs.Count) : -1;
                            int ni = parts.Length > 2 && parts[2].Length > 0 ? Resolve(int.Parse(parts[2], inv), nrms.Count) : -1;
                            var key = (vi, ti, ni);
                            if (!map.TryGetValue(key, out int idx))
                            {
                                idx = outPos.Count;
                                map[key] = idx;
                                outPos.Add(pos[vi]);
                                outUv.Add(ti >= 0 ? uvs[ti] : Vector2.zero);
                                outN.Add(ni >= 0 ? nrms[ni] : Vector3.zero);
                                anyNormal |= ni >= 0;
                            }
                            face.Add(idx);
                        }
                        for (int k = 1; k + 1 < face.Count; k++)
                        {
                            tris.Add(face[0]); tris.Add(face[k]); tris.Add(face[k + 1]);
                            triCount++;
                        }
                        if (triCount > MaxTriangles) throw new InvalidDataException($"more than {MaxTriangles} triangles");
                        break;
                }
            }
            if (triCount == 0) throw new InvalidDataException("no faces");

            var mesh = new Mesh
            {
                name = "TCGCC_" + Path.GetFileNameWithoutExtension(path),
                indexFormat = outPos.Count > 65535 ? IndexFormat.UInt32 : IndexFormat.UInt16,
                hideFlags = HideFlags.DontUnloadUnusedAsset,
            };
            mesh.SetVertices(outPos);
            mesh.SetUVs(0, outUv);
            mesh.subMeshCount = subs.Count;
            for (int i = 0; i < subs.Count; i++) mesh.SetTriangles(subs[i], i);
            if (anyNormal) mesh.SetNormals(outN);
            else mesh.RecalculateNormals();
            mesh.RecalculateBounds();
            // Keep it CPU-readable: outline effects and colliders may read it; figurine meshes are small.
            mesh.UploadMeshData(false);
            Plugin.Log.LogInfo($"Model {Path.GetFileName(path)}: {outPos.Count} vertices, {triCount} triangles, {subs.Count} submesh(es), bounds {mesh.bounds.size}");
            return mesh;
        }
    }
}
