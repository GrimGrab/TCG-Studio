using System;
using System.Collections.Generic;
using System.Linq;
using HarmonyLib;
using UnityEngine;

namespace TCGCustomCards.Runtime
{
    /// <summary>
    /// [Visuals - Game look] CreditCardImages: the cards customers pay with. Every card model (in each customer's hand,
    /// <c>InteractableCustomerCash.m_CardModel</c>, and the ones at the cashier counter's card machines) is mesh credit_card_001
    /// with two materials: white plastic for the edges and back, and card_plastic_textured whose texture credit_card_D (1024×512)
    /// covers the whole printed side. The game has no bump map for it: chip, number, expiry and name are painted into that texture
    /// with their emboss shading. Each design fills a copy of the texture; CreditCardKeepChip / CreditCardKeepPrint paint those
    /// vanilla parts back on top, keeping only their own (grey) pixels, not the blue around them. Each time a customer pays by
    /// card they hand over a random design; the counter's cards show the last one handed over.
    /// </summary>
    internal static class CreditCard
    {
        /// <summary>Width ÷ height of the card's printed side (mesh 85.72 × 53.98 mm; the texture is stretched onto it).</summary>
        internal const float FaceAspect = 85.72f / 53.98f;

        private const string VanillaTexture = "credit_card_D";

        // Areas of credit_card_D (texture pixels, top-left origin, measured from the game files 2026-10-17 plus 4 px for the emboss
        // shadow; Studio gameextract.CardChip/CardPrint have the same values).
        private static readonly RectInt Chip = new RectInt(104, 133, 164, 111);
        private static readonly RectInt[] Print =
        {
            new RectInt(92, 276, 816, 49),  // card number
            new RectInt(96, 333, 508, 71),  // "1234", VALID THRU, MONTH/YEAR, expiry
            new RectInt(91, 430, 324, 41),  // name
        };

        private static readonly PictureSet<Texture2D> Textures = new PictureSet<Texture2D>();
        private static readonly System.Random Random = new System.Random();
        private static Texture _vanilla;

        /// <summary>Card renderers by owner: each customer's hand (one design per payment) and each counter (the last one handed over).</summary>
        private static readonly Dictionary<Component, List<Renderer>> Customers = new Dictionary<Component, List<Renderer>>();
        private static readonly Dictionary<Component, List<Renderer>> Counters = new Dictionary<Component, List<Renderer>>();
        private static readonly Dictionary<Component, string> Shown = new Dictionary<Component, string>(); // owner → design key
        private static string _lastHandedOver;

        [HarmonyPatch(typeof(InteractableCustomerCash), nameof(InteractableCustomerCash.SetIsCard))]
        private static class SetIsCardPatch
        {
            private static void Postfix(InteractableCustomerCash __instance, bool isCard)
            {
                Track(Customers, __instance, __instance.m_CardModel);
                if (!isCard) return;
                var keys = Entries().Select(e => e.key).ToList();
                // A random design for this payment (any of them, repeats too: user's choice).
                string key = keys.Count == 0 ? null : keys[Random.Next(keys.Count)];
                Shown[__instance] = key;
                _lastHandedOver = key;
                Apply(__instance, Customers[__instance], key);
            }
        }

        [HarmonyPatch(typeof(InteractableCashierCounter), "Awake")]
        private static class CounterAwakePatch
        {
            private static void Postfix(InteractableCashierCounter __instance) => Track(Counters, __instance, __instance.gameObject);
        }

        [HarmonyPatch(typeof(InteractableCashierCounter), nameof(InteractableCashierCounter.StartGivingChange))]
        private static class StartGivingChangePatch
        {
            private static void Postfix(InteractableCashierCounter __instance)
            {
                Track(Counters, __instance, __instance.gameObject);
                Apply(__instance, Counters[__instance], _lastHandedOver);
            }
        }

        /// <summary>Remembers the card renderers under <paramref name="root"/> (once per owner; owners from an earlier scene are dropped).</summary>
        private static void Track(Dictionary<Component, List<Renderer>> owners, Component owner, GameObject root)
        {
            foreach (var dead in owners.Keys.Where(k => k == null).ToList()) { owners.Remove(dead); Shown.Remove(dead); }
            if (owners.ContainsKey(owner) || root == null) return;
            owners[owner] = root.GetComponentsInChildren<MeshRenderer>(true).Where(r => Slot(r) >= 0).Cast<Renderer>().ToList();
        }

        /// <summary>The material slot drawing the card's print, or -1.</summary>
        private static int Slot(Renderer r)
        {
            var mats = r.sharedMaterials;
            for (int i = 0; i < mats.Length; i++)
                if (mats[i] != null && mats[i].mainTexture != null && mats[i].mainTexture.name == VanillaTexture) return i;
            return -1;
        }

        /// <summary>Shows design <paramref name="key"/> (null = vanilla) on an owner's cards.</summary>
        private static void Apply(Component owner, List<Renderer> renderers, string key)
        {
            Shown[owner] = key;
            foreach (var r in renderers)
            {
                if (r == null) continue;
                int slot = Slot(r);
                if (slot < 0) continue;
                if (_vanilla == null) _vanilla = r.sharedMaterials[slot].mainTexture; // Unity's null: also a destroyed texture
                ShopPicture.SetTexture(r, key == null ? null : TextureFor(key), slot);
            }
        }

        /// <summary>The designs as keys (path|crop|chip|print: the built texture depends on all four), in the settings' order.</summary>
        private static List<(string key, string path, string crop)> Entries()
        {
            var images = (Plugin.CreditCardImages.Value ?? "").Split(ScreenPictures.ListSeparator);
            var crops = (Plugin.CreditCardCrops.Value ?? "").Split(ScreenPictures.ListSeparator);
            string keep = $"|{Plugin.CreditCardKeepChip.Value}|{Plugin.CreditCardKeepPrint.Value}";
            var list = new List<(string, string, string)>();
            for (int i = 0; i < images.Length; i++)
            {
                string path = ShopPicture.ImagePath(images[i]);
                if (path == null) continue;
                string crop = i < crops.Length ? crops[i] : "";
                list.Add((path + "|" + crop + keep, path, crop));
            }
            return list;
        }

        /// <summary>Settings changed: every card shows a design from the new list (its own when still there).</summary>
        private static void Reapply()
        {
            var entries = Entries();
            var keys = entries.Select(e => e.key).ToList();
            Textures.Keep(new HashSet<string>(keys));
            string Pick(string current) => current != null && keys.Contains(current) ? current : keys.Count == 0 ? null : keys[Random.Next(keys.Count)];
            if (!keys.Contains(_lastHandedOver)) _lastHandedOver = Pick(null);
            foreach (var kv in Customers.Where(k => k.Key != null).ToList())
                Apply(kv.Key, kv.Value, Pick(Shown.TryGetValue(kv.Key, out var k) ? k : null));
            foreach (var kv in Counters.Where(k => k.Key != null).ToList())
                Apply(kv.Key, kv.Value, _lastHandedOver);
        }

        internal static void Init()
        {
            EventHandler apply = (_, __) => Reapply();
            Plugin.CreditCardImages.SettingChanged += apply;
            Plugin.CreditCardCrops.SettingChanged += apply;
            Plugin.CreditCardKeepChip.SettingChanged += apply;
            Plugin.CreditCardKeepPrint.SettingChanged += apply;
        }

        private static Texture2D TextureFor(string key)
        {
            var entry = Entries().FirstOrDefault(e => e.key == key);
            if (entry.path == null) return null;
            bool chip = Plugin.CreditCardKeepChip.Value, print = Plugin.CreditCardKeepPrint.Value;
            return Textures.Get(key, () =>
            {
                if (_vanilla == null) return null;
                var src = ShopPicture.Load(entry.path, "Credit card");
                if (src == null) return null;
                var keep = new List<RectInt>();
                if (chip) keep.Add(Chip);
                if (print) keep.AddRange(Print);
                var tex = ShopPicture.Compose(_vanilla, "TCGCC_CreditCard", (px, w, h) =>
                {
                    var vanilla = (Color32[])px.Clone(); // the mask reads the print from the untouched pixels
                    ShopPicture.Paint(px, w, h, src, ShopPicture.Crop(entry.crop, "CreditCardCrops", src, FaceAspect), new Rect(0, 0, 1, 1),
                        (x, y) => 1f - Keep(vanilla[y * w + x], x, h - 1 - y, keep));
                });
                UnityEngine.Object.Destroy(src);
                if (tex != null) Plugin.Log.LogInfo($"Credit card: using {entry.path}");
                return tex;
            });
        }

        /// <summary>How much of a vanilla pixel stays (0..1): inside a kept area, the low-colour print (silver letters, their dark
        /// shadow, the chip) stays and the saturated blue card around it goes. x, y = texture pixel, top-left origin.</summary>
        internal static float Keep(Color32 c, int x, int y, List<RectInt> areas)
        {
            foreach (var a in areas)
            {
                if (x < a.xMin || x >= a.xMax || y < a.yMin || y >= a.yMax) continue;
                int chroma = Math.Max(c.r, Math.Max(c.g, c.b)) - Math.Min(c.r, Math.Min(c.g, c.b));
                return Mathf.Clamp01((70f - chroma) / 30f);
            }
            return 0f;
        }
    }
}
