using System;
using System.Collections.Concurrent;
using System.Diagnostics;
using System.IO;
using System.Text;
using System.Threading;
using Newtonsoft.Json;
using Newtonsoft.Json.Linq;

namespace TCGCustomCards.Runtime.Mtg
{
    /// <summary>
    /// The headless Forge process (forge-bridge\, GPL): started once per session and kept alive between games (Forge takes ~6 s
    /// to load its card database). Messages are one JSON object per line on stdin/stdout; incoming ones are read on a background
    /// thread and handed to the main thread through <see cref="Poll"/>. Protocol: docs/mtg-forge.md "Bridge protocol".
    /// </summary>
    internal static class ForgeBridge
    {
        private static Process _proc;
        private static Thread _reader;
        private static readonly ConcurrentQueue<JObject> Inbox = new ConcurrentQueue<JObject>();
        private static readonly object SendLock = new object();

        public static bool Ready { get; private set; }
        public static bool Running
        {
            get
            {
                try { return _proc != null && !_proc.HasExited; }
                catch { return false; }
            }
        }

        /// <summary>Starts the bridge unless it's already running. Returns null or an error for the player.</summary>
        public static string EnsureStarted()
        {
            if (Running) return null;
            Ready = false;
            while (Inbox.TryDequeue(out _)) { }
            string user = ForgeLauncher.UserDir;
            if (user == null) return "MTG mode isn't installed - open TCG Studio > Setup > Install MTG mode";
            var psi = ForgeLauncher.BridgeStartInfo(Path.Combine(user, "tcgcc-bridge.log"));
            if (psi == null) return "Forge bridge missing (tcgcc-forge-bridge.jar) - reinstall the mod";
            try
            {
                _proc = Process.Start(psi);
            }
            catch (Exception e)
            {
                Plugin.Log.LogError($"Forge bridge failed to start: {e}");
                return "Forge failed to start (see BepInEx log)";
            }
            var proc = _proc;
            _reader = new Thread(() => ReadLoop(proc)) { IsBackground = true, Name = "tcgcc-forge-reader" };
            _reader.Start();
            Plugin.Log.LogInfo("Forge bridge started");
            return null;
        }

        private static void ReadLoop(Process proc)
        {
            try
            {
                string line;
                while ((line = proc.StandardOutput.ReadLine()) != null)
                {
                    if (line.Length == 0 || line[0] != '{') continue;
                    try { Inbox.Enqueue(JObject.Parse(line)); }
                    catch (Exception e) { Plugin.Log.LogWarning($"Forge bridge: bad line ({e.Message}): {Trim(line)}"); }
                }
            }
            catch (Exception e)
            {
                Plugin.Log.LogWarning($"Forge bridge reader stopped: {e.Message}");
            }
            Inbox.Enqueue(new JObject { ["t"] = "exited" });
        }

        /// <summary>Next message from Forge, or null. Main thread only.</summary>
        public static JObject Poll()
        {
            if (!Inbox.TryDequeue(out var m)) return null;
            string t = (string)m["t"];
            if (t == "ready") Ready = true;
            if (t == "exited") Ready = false;
            return m;
        }

        public static void Send(JObject m)
        {
            if (!Running) return;
            // ASCII-only JSON so the pipe's encoding never matters.
            string s = JsonConvert.SerializeObject(m, new JsonSerializerSettings { StringEscapeHandling = StringEscapeHandling.EscapeNonAscii });
            lock (SendLock)
            {
                try
                {
                    var bytes = Encoding.ASCII.GetBytes(s + "\n");
                    _proc.StandardInput.BaseStream.Write(bytes, 0, bytes.Length);
                    _proc.StandardInput.BaseStream.Flush();
                }
                catch (Exception e)
                {
                    Plugin.Log.LogWarning($"Forge bridge send failed: {e.Message}");
                }
            }
        }

        public static void Act(string action, int? id = null)
        {
            var m = new JObject { ["t"] = "act", ["a"] = action };
            if (id.HasValue) m["id"] = id.Value;
            Send(m);
        }

        public static void Stop()
        {
            if (!Running) return;
            Send(new JObject { ["t"] = "quit" });
            try { if (!_proc.WaitForExit(1500)) _proc.Kill(); } catch { }
            Ready = false;
        }

        private static string Trim(string s) => s.Length > 200 ? s.Substring(0, 200) + "…" : s;
    }
}
