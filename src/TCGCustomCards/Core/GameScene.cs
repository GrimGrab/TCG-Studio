using HarmonyLib;
using UnityEngine.SceneManagement;

namespace TCGCustomCards.Core
{
    /// <summary>Which scene the game is in. The shop scene's name comes from the game itself ("Start" until game 1.02,
    /// "StartOptimized" since the Unity 6 update): checks written against a fixed name stopped working when it changed.</summary>
    internal static class GameScene
    {
        /// <summary>The shop scene is the active one.</summary>
        public static bool InShop => SceneManager.GetActiveScene().name == CGameManager.k_StartSceneName;

        /// <summary>In the shop with a save (or new game) fully loaded: for code that runs every frame in every scene (the mod's
        /// runners, OnGUI).</summary>
        public static bool InLoadedShop => GameInstance.m_FinishedSavefileLoading && InShop;

        /// <summary>
        /// The live instance of a game singleton, or null. Unlike <c>CSingleton&lt;T&gt;.Instance</c> this never creates one:
        /// that getter makes an empty DontDestroyOnLoad copy when the scene has none (the title screen), and the game then
        /// keeps using the empty copy in the shop (New Game / loading a save broken, 2026-10-26). Use it from code that can
        /// run outside the shop. It only sees a singleton the game already looked up, which the game does as the shop loads.
        /// </summary>
        public static T Existing<T>() where T : UnityEngine.Component
        {
            var t = Field<T>.Instance();
            return t != null ? t : null; // Unity null: destroyed with its scene
        }

        private static class Field<T> where T : UnityEngine.Component
        {
            public static readonly AccessTools.FieldRef<T> Instance = AccessTools.StaticFieldRefAccess<T>(AccessTools.Field(typeof(CSingleton<T>), "instance"));
        }
    }
}
