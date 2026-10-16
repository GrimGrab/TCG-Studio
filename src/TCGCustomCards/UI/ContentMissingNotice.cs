using System.Collections;
using TCGCustomCards.Core;
using UnityEngine;
using UnityEngine.SceneManagement;

namespace TCGCustomCards.UI
{
    /// <summary>
    /// Tells the player once per game start, in the shop, that custom content didn't load (StudioPaths.Problem): the log alone is
    /// easy to miss, and every custom set, item and placed piece is gone from the shop until it's fixed. Saving stays safe.
    /// </summary>
    internal static class ContentMissingNotice
    {
        public static IEnumerator ShowWhenInShop()
        {
            while (SceneManager.GetActiveScene().name != "Start") yield return new WaitForSeconds(1f);
            yield return new WaitForSeconds(3f); // let the shop finish loading
            while (ChoicePopup.IsOpen) yield return new WaitForSeconds(1f);
            ChoicePopup.Show(
                "TCG Custom Cards: your custom sets and items didn't load (" + StudioPaths.Problem + ").\n\n" +
                "Close the game, open TCG Studio once, then start the game again. Your saves keep your custom things in the meantime.",
                "Quit game", Application.Quit, "Keep playing", null);
        }
    }
}
