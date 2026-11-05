using System;
using System.Threading;

namespace TCGCustomCards.Debug
{
    /// <summary>
    /// Watches for a frozen game. Windows closes a game that stops responding without anything in the logs, so: the main
    /// thread stamps every frame (<see cref="Frame"/>, from the hooks runner) and mod code leaves a breadcrumb of what it is
    /// doing (<see cref="Step"/>); a background thread writes one log line when no frame came for <see cref="FrozenSeconds"/>
    /// while the game had focus (an unfocused game may pause its frames), and another when it answers again.
    /// </summary>
    internal static class HangWatch
    {
        private const int FrozenSeconds = 10;

        private static long _lastFrame; // DateTime ticks (UTC)
        private static volatile string _step = "starting";
        private static volatile bool _focused = true;
        private static Thread _thread;

        /// <summary>What mod code is doing now (cheap: one field write).</summary>
        public static void Step(string what) => _step = what;

        public static void Frame() => Interlocked.Exchange(ref _lastFrame, DateTime.UtcNow.Ticks);

        public static void Focus(bool focused)
        {
            _focused = focused;
            Frame();
        }

        public static void Start()
        {
            if (_thread != null) return;
            Frame();
            _thread = new Thread(Watch) { IsBackground = true, Name = "TCGCC hang watch" };
            _thread.Start();
        }

        private static void Watch()
        {
            bool reported = false;
            while (true)
            {
                Thread.Sleep(1000);
                double since = TimeSpan.FromTicks(DateTime.UtcNow.Ticks - Interlocked.Read(ref _lastFrame)).TotalSeconds;
                if (!reported && _focused && since >= FrozenSeconds)
                {
                    reported = true;
                    Plugin.Log.LogError($"Hang watch: the game hasn't drawn a frame for {since:0} s. Last thing the mod did: {_step}. " +
                                        "If Windows closes the game now, send this log.");
                }
                else if (reported && since < 2)
                {
                    reported = false;
                    Plugin.Log.LogWarning($"Hang watch: the game is responding again (last mod step before the freeze: {_step}).");
                }
            }
        }
    }
}
