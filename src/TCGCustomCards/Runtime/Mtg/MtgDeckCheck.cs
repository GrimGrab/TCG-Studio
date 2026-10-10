using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Security.Cryptography;
using System.Text;
using Newtonsoft.Json.Linq;

namespace TCGCustomCards.Runtime.Mtg
{
    /// <summary>
    /// Tournament deck legality, decided by Forge (bridge <c>validate</c> → <c>validated</c>): deck-size/copy rules of Forge's
    /// Constructed or Limited format plus a Forge GameFormat limited to the event's set codes. Results are cached per (deck
    /// text, kind, sets); the bridge is started on demand and requests wait until it's ready.
    /// </summary>
    internal static class MtgDeckCheck
    {
        public sealed class Result
        {
            public bool Done;
            public bool Ok;
            /// <summary>Forge's own messages, word for word.</summary>
            public readonly List<string> Problems = new List<string>();
        }

        private static readonly Dictionary<string, Result> Cache = new Dictionary<string, Result>();
        private static readonly Dictionary<int, string> Waiting = new Dictionary<int, string>();
        private static readonly Queue<JObject> Outbox = new Queue<JObject>();
        private static int _nextId = 1;

        /// <summary>Starts (or returns the cached) check of <paramref name="deck"/>; poll <see cref="Result.Done"/>.</summary>
        public static Result Check(MtgDeck deck, IList<string> setCodes, string kind = "constructed")
        {
            string dck = deck.ToDck();
            var codes = setCodes.Select(c => c.ToUpperInvariant()).OrderBy(c => c).ToList();
            string key = Hash(dck + "\n" + kind + "\n" + string.Join(",", codes));
            if (Cache.TryGetValue(key, out var r)) return r;
            r = new Result();
            Cache[key] = r;
            string err = ForgeBridge.EnsureStarted();
            if (err != null) return Fail(key, err);
            string file;
            try
            {
                string dir = Path.Combine(ForgeLauncher.UserDir, "decks", "tcgcc-check");
                Directory.CreateDirectory(dir);
                file = Path.Combine(dir, key + ".dck");
                File.WriteAllText(file, dck, new UTF8Encoding(false));
            }
            catch (System.Exception e)
            {
                return Fail(key, "Couldn't write the deck for Forge: " + e.Message);
            }
            int id = _nextId++;
            Waiting[id] = key;
            Outbox.Enqueue(new JObject { ["t"] = "validate", ["id"] = id, ["deck"] = file, ["kind"] = kind, ["sets"] = new JArray(codes) });
            return r;
        }

        /// <summary>Forge's answer to "can this card be a commander?" (cached per card name; null = still asking).</summary>
        public sealed class CommanderAnswer
        {
            public bool Done, Ok;
            public string Reason;
        }

        private static readonly Dictionary<string, CommanderAnswer> CommanderAnswers = new Dictionary<string, CommanderAnswer>(System.StringComparer.OrdinalIgnoreCase);
        private static readonly Dictionary<int, string> CommanderWaiting = new Dictionary<int, string>();

        /// <summary>Asks Forge (DeckFormat.Commander.isLegalCommander) whether <paramref name="card"/> can be a commander.</summary>
        public static CommanderAnswer CanBeCommander(MtgCard card)
        {
            if (CommanderAnswers.TryGetValue(card.Name, out var a) && !(a.Done && !a.Ok && a.Reason != null && a.Reason.StartsWith("Forge isn't")))
                return a;
            a = new CommanderAnswer();
            CommanderAnswers[card.Name] = a;
            string err = ForgeBridge.EnsureStarted();
            if (err != null) { a.Done = true; a.Reason = err; return a; }
            int id = _nextId++;
            CommanderWaiting[id] = card.Name;
            Outbox.Enqueue(new JObject { ["t"] = "commanderok", ["id"] = id, ["name"] = card.Name, ["set"] = card.SetCode });
            return a;
        }

        public static void OnCommanderOk(JObject m)
        {
            int id = (int?)m["id"] ?? -1;
            if (!CommanderWaiting.TryGetValue(id, out var name)) return;
            CommanderWaiting.Remove(id);
            if (!CommanderAnswers.TryGetValue(name, out var a)) return;
            a.Ok = (bool?)m["ok"] ?? false;
            a.Reason = (string)m["reason"];
            a.Done = true;
        }

        /// <summary>Every frame (MtgSession.Pump): send queued checks once Forge is ready; fail them if Forge stopped.</summary>
        public static void Tick()
        {
            if (Outbox.Count == 0 && Waiting.Count == 0 && CommanderWaiting.Count == 0) return;
            if (!ForgeBridge.Running)
            {
                foreach (var key in Waiting.Values.ToList()) Fail(key, "Forge isn't running (see TCGForge\\userdata\\tcgcc-bridge.log)");
                Waiting.Clear();
                foreach (var name in CommanderWaiting.Values)
                    if (CommanderAnswers.TryGetValue(name, out var ca)) { ca.Done = true; ca.Reason = "Forge isn't running"; }
                CommanderWaiting.Clear();
                Outbox.Clear();
                return;
            }
            while (ForgeBridge.Ready && Outbox.Count > 0) ForgeBridge.Send(Outbox.Dequeue());
        }

        public static void OnValidated(JObject m)
        {
            int id = (int?)m["id"] ?? -1;
            if (!Waiting.TryGetValue(id, out var key)) return;
            Waiting.Remove(id);
            if (!Cache.TryGetValue(key, out var r)) return;
            r.Ok = (bool?)m["ok"] ?? false;
            foreach (var p in (m["problems"] as JArray) ?? new JArray()) r.Problems.Add((string)p);
            r.Done = true;
            Plugin.Log.LogInfo($"MTG deck check: {(r.Ok ? "legal" : string.Join(" | ", r.Problems))}");
        }

        private static Result Fail(string key, string error)
        {
            var r = Cache[key];
            Cache.Remove(key); // try again next time
            r.Problems.Add(error);
            r.Done = true;
            return r;
        }

        private static string Hash(string s)
        {
            using (var sha = SHA1.Create())
                return string.Concat(sha.ComputeHash(Encoding.UTF8.GetBytes(s)).Select(b => b.ToString("x2")));
        }
    }
}
