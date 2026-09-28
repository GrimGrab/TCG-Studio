using System.Collections;
using System.Collections.Generic;
using HarmonyLib;
using Newtonsoft.Json.Linq;
using TCGCustomCards.UI;
using UnityEngine;

namespace TCGCustomCards.Runtime.Mtg
{
    /// <summary>
    /// An MTG game played inside a normal play-table session: the vanilla sit (customer, seat, camera) runs, then
    /// <see cref="Run"/> replaces the Tetramon board (PlayTableGame.DelayStart) and finishes with PlayTableGame.ReportWinner, so the
    /// vanilla win/lose screen, win counts, achievement, rematch/leave, end-game gift and customer leave all apply.
    /// </summary>
    internal static class MtgSession
    {
        /// <summary>Set when the player picked "Magic" and the vanilla sit is about to start a game.</summary>
        public static bool Pending;
        /// <summary>True from the first MTG game at a table until the player leaves (rematches stay MTG).</summary>
        public static bool InSession;

        // Latest data from Forge (main thread).
        public static JObject State, Prompt, Ask;
        /// <summary>Which in-game copy each Forge card is drawn as (3D table, dialogs, previews) for the current game.</summary>
        public static MtgCardFaces CardFaces;
        public static readonly List<string> Messages = new List<string>();
        public static string Status = "";
        public static bool Playing;
        public static JObject GameOver;

        private static MtgRunner _runner;

        public static void EnsureRunner()
        {
            if (_runner != null) return;
            var go = new GameObject("TCGCC_Mtg");
            Object.DontDestroyOnLoad(go);
            _runner = go.AddComponent<MtgRunner>();
        }

        public static bool CanPlayInGame => ForgeLauncher.IsInstalled && System.IO.File.Exists(
            System.IO.Path.Combine(Plugin.PluginDir, "tcgcc-forge-bridge.jar"));

        /// <summary>Drains the bridge inbox (every frame).</summary>
        public static void Pump()
        {
            JObject m;
            while ((m = ForgeBridge.Poll()) != null)
            {
                switch ((string)m["t"])
                {
                    case "state": State = m; break;
                    case "prompt": Prompt = m; break;
                    case "ask": Ask = m; break;
                    case "gameover": GameOver = m; break;
                    case "message":
                    case "error":
                        AddMessage(((string)m["title"] is string t && t.Length > 0 ? t + ": " : "") + (string)m["text"]);
                        break;
                    case "flash": AddMessage("Can't do that now"); break;
                    case "exited":
                        if (Playing) AddMessage("Forge stopped unexpectedly (see TCGForge\\userdata\\tcgcc-bridge.log)");
                        break;
                }
            }
        }

        private static void AddMessage(string s)
        {
            Messages.Add(s);
            if (Messages.Count > 6) Messages.RemoveAt(0);
            Plugin.Log.LogInfo($"MTG: {s}");
        }

        public static void Answer(JArray picked)
        {
            if (Ask == null) return;
            ForgeBridge.Send(new JObject { ["t"] = "answer", ["id"] = Ask["id"], ["picked"] = picked });
            Ask = null;
        }

        /// <summary>"amounts" asks: one number per option.</summary>
        public static void AnswerAmounts(IEnumerable<int> amounts)
        {
            if (Ask == null) return;
            ForgeBridge.Send(new JObject { ["t"] = "answer", ["id"] = Ask["id"], ["amounts"] = new JArray(amounts) });
            Ask = null;
        }

        public static void AnswerIndex(int index)
        {
            if (Ask == null) return;
            ForgeBridge.Send(new JObject { ["t"] = "answer", ["id"] = Ask["id"], ["picked"] = index });
            Ask = null;
        }

        /// <summary>Replacement for PlayTableGame.DelayStart (a coroutine on the PlayTableGame).</summary>
        public static IEnumerator Run(PlayTableGame game)
        {
            EnsureRunner();
            State = Prompt = Ask = GameOver = null;
            Messages.Clear();
            _leftMidGame = false;
            Playing = true;
            Status = "Shuffling decks…";
            MtgTableUI.Show();
            CSingleton<LightManager>.Instance.SetCanChangeBGM(canChange: false);
            SoundManager.BlendToMusic("BGM_FightOpening", 0.5f, isLinearBlend: true);
            SoundManager.QueueMusic("BGM_FightOpening", "BGM_FightLoop", 1f);

            string err = MtgMode.BuildDecks(out var player, out var opponent);
            if (err == null) err = ForgeBridge.EnsureStarted();
            if (err != null)
            {
                yield return Abort(game, err);
                yield break;
            }
            string deckFile = ForgeLauncher.WriteDeck(player);
            string oppFile = ForgeLauncher.WriteDeck(opponent);

            Status = "Starting Forge…";
            float until = Time.unscaledTime + 60f;
            while (!ForgeBridge.Ready && ForgeBridge.Running && Time.unscaledTime < until) yield return null;
            if (!ForgeBridge.Ready)
            {
                yield return Abort(game, "Forge didn't start (see TCGForge\\userdata\\tcgcc-bridge.log)");
                yield break;
            }

            Status = "";
            CardFaces = MtgMode.Faces();
            if (Plugin.MtgBoard3D.Value)
            {
                try { MtgTable3d.Begin(game, CardFaces); }
                catch (System.Exception e) { Plugin.Log.LogError($"MTG 3D table failed, using the flat board: {e}"); MtgTable3d.End(); }
            }
            ForgeBridge.Send(new JObject
            {
                ["t"] = "start", ["deck"] = deckFile, ["opponent"] = oppFile,
                ["name"] = string.IsNullOrWhiteSpace(CPlayerData.PlayerName) ? "Player" : CPlayerData.PlayerName,
                ["opponentName"] = "Customer",
            });
            Traverse.Create(game).Field("m_CurrentInteractablePlayTable").GetValue<InteractablePlayTable>()?.StartPlayerCardGame();
            Traverse.Create(game).Field("m_CanExit").SetValue(true); // Esc → vanilla "quit battle?" → concede (MtgQuitBattle)

            while (GameOver == null)
            {
                if (_leftMidGame) yield break; // player left the table (vanilla quit screen); LeaveMidGame cleaned up
                if (!ForgeBridge.Running)
                {
                    yield return Abort(game, "Forge stopped unexpectedly (see TCGForge\\userdata\\tcgcc-bridge.log)");
                    yield break;
                }
                yield return null;
            }

            bool won = (bool?)GameOver["won"] ?? false;
            bool draw = (bool?)GameOver["draw"] ?? false;
            Traverse.Create(game).Field("m_CanExit").SetValue(false);
            Status = draw ? "Draw!" : won ? "You win!" : "You lose!";
            RestoreShopMusic(); // vanilla does this when a player's HP hits 0 (PlayCardSet.TakeDamage)
            yield return new WaitForSecondsRealtime(2.5f);
            if (_leftMidGame) yield break;
            Playing = false;
            MtgTableUI.Hide();
            MtgTable3d.End();
            game.ReportWinner(won, draw); // vanilla win/lose screen → rematch (runs Run again) or leave
        }

        private static IEnumerator Abort(PlayTableGame game, string error)
        {
            Status = error;
            Toast.Show(error);
            Plugin.Log.LogWarning($"MTG game aborted: {error}");
            RestoreShopMusic();
            yield return new WaitForSecondsRealtime(3f);
            Playing = false;
            MtgTableUI.Hide();
            MtgTable3d.End();
            InSession = false;
            game.FinishLeaveGame(false);
        }

        private static bool _leftMidGame;

        /// <summary>Back to the shop's time-of-day music (vanilla: SetCanChangeBGM(true) + RefreshBGM at a duel's end/quit).</summary>
        private static void RestoreShopMusic()
        {
            var light = CSingleton<LightManager>.Instance;
            if (light == null) return;
            light.SetCanChangeBGM(canChange: true);
            light.RefreshBGM();
        }

        /// <summary>
        /// The player left the table while an MTG game was running (Esc → vanilla quit screen calls FinishLeaveGame directly):
        /// concede in Forge, remove the MTG screen/cards, give the music back. Like a vanilla quit, no win/loss is reported.
        /// </summary>
        public static void LeaveMidGame()
        {
            if (!Playing) return;
            _leftMidGame = true;
            if (GameOver == null) ForgeBridge.Act("concede");
            Playing = false;
            MtgTableUI.Hide();
            MtgTable3d.End();
            RestoreShopMusic();
        }
    }

    /// <summary>Pumps bridge messages every frame and stops Forge when the game closes.</summary>
    internal class MtgRunner : MonoBehaviour
    {
        private void Update() => MtgSession.Pump();
        private void OnApplicationQuit() => ForgeBridge.Stop();
    }
}
