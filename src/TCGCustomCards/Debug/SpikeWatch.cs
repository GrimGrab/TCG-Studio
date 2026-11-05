using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.Linq;

namespace TCGCustomCards.Debug
{
    /// <summary>
    /// [Debug] CustomerDebug: a line in the log for every frame that took long, with how much of it each measured part of the mod took and
    /// whether the garbage collector ran, and what just happened (<see cref="Mark"/>): drawing work reaches the graphics card later than its code runs,
    /// so a spike soon after an event can be its doing even with no time measured. Costs two clock reads per measured part.
    /// </summary>
    internal static class SpikeWatch
    {
        private const float SpikeSeconds = 0.04f;
        private const float MarkSeconds = 3f; // spikes this soon after a mark name it
        private const float SettleSeconds = 20f; // loading a scene is one long spike: not what this is for

        private static readonly Dictionary<string, long> Spent = new Dictionary<string, long>();
        private static readonly Dictionary<string, int> Times = new Dictionary<string, int>();
        private static int _collections;
        private static float _since;
        private static string _scene;
        private static string _mark;
        private static float _markAt;

        /// <summary>Something that may cost frames just after it.</summary>
        public static void Mark(string what)
        {
            if (!Plugin.CustomerDebug.Value) return;
            _mark = what;
            _markAt = UnityEngine.Time.unscaledTime;
        }

        public struct Section : IDisposable
        {
            private readonly string _name;
            private readonly long _start;

            public Section(string name)
            {
                _name = name;
                _start = Stopwatch.GetTimestamp();
            }

            public void Dispose()
            {
                if (_name == null) return;
                Spent.TryGetValue(_name, out long had);
                Spent[_name] = had + Stopwatch.GetTimestamp() - _start;
                Times.TryGetValue(_name, out int n);
                Times[_name] = n + 1;
            }
        }

        public static Section Measure(string name) => Plugin.CustomerDebug.Value ? new Section(name) : default;

        /// <summary>Once per frame: looks back at the frame that just ended.</summary>
        public static void Frame()
        {
            if (!Plugin.CustomerDebug.Value) return;
            float took = UnityEngine.Time.unscaledDeltaTime, now = UnityEngine.Time.unscaledTime;
            string scene = UnityEngine.SceneManagement.SceneManager.GetActiveScene().name;
            if (scene != _scene) { _scene = scene; _since = now; }
            int collections = GC.CollectionCount(0);
            if (took >= SpikeSeconds && now - _since >= SettleSeconds)
            {
                double perMs = Stopwatch.Frequency / 1000.0;
                string parts = string.Join(", ", Spent.OrderByDescending(kv => kv.Value).Select(kv => $"{kv.Key} {kv.Value / perMs:0.0} ms ×{Times[kv.Key]}"));
                string after = _mark != null && now - _markAt <= MarkSeconds ? $"; {now - _markAt:0.0} s after {_mark}" : "";
                Plugin.Log.LogInfo($"Spike: {took * 1000f:0} ms{(collections != _collections ? " with garbage collection" : "")}; mod parts: {(parts.Length > 0 ? parts : "none")}{after}");
            }
            _collections = collections;
            Spent.Clear();
            Times.Clear();
        }
    }
}
