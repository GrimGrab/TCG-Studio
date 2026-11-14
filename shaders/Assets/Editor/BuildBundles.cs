using System.IO;
using UnityEditor;
using UnityEngine;

/// <summary>
/// Builds the mod's shader bundle: Assets/Shaders/*.shader → Build/tcgcc_foil (StandaloneWindows64, built-in pipeline).
/// Run headless: Unity.exe -batchmode -quit -projectPath shaders -executeMethod BuildBundles.Build (see tools/build-shaders.ps1).
/// </summary>
public static class BuildBundles
{
    public const string BundleName = "tcgcc_foil";
    /// <summary>The template TCG Studio turns players' card shaders into (Assets/CardShader); not shipped to the game.</summary>
    public const string CardShaderBundle = "tcgcc_cardshader";

    public static void Build()
    {
        foreach (var guid in AssetDatabase.FindAssets("t:Shader", new[] { "Assets/Shaders", "Assets/CardShader" }))
        {
            var path = AssetDatabase.GUIDToAssetPath(guid);
            AssetImporter.GetAtPath(path).assetBundleName = path.StartsWith("Assets/CardShader") ? CardShaderBundle : BundleName;
            var shader = AssetDatabase.LoadAssetAtPath<Shader>(path);
            var messages = ShaderUtil.GetShaderMessages(shader);
            foreach (var m in messages) Debug.Log($"[BuildBundles] {path}: {m.severity} {m.message} (line {m.line})");
            if (ShaderUtil.ShaderHasError(shader)) throw new System.Exception($"Shader has errors: {path}");
        }
        Directory.CreateDirectory("Build");
        BuildPipeline.BuildAssetBundles("Build", BuildAssetBundleOptions.ChunkBasedCompression, BuildTarget.StandaloneWindows64);
        Debug.Log("[BuildBundles] built Build/" + BundleName + " and Build/" + CardShaderBundle);
    }
}
