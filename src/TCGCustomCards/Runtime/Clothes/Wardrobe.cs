using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using CC;
using Newtonsoft.Json;
using TCGCustomCards.Core;
using TCGCustomCards.Hooks;
using UnityEngine;

namespace TCGCustomCards.Runtime.Clothes
{
    /// <summary>
    /// Clothes made in TCG Studio (accessories.json "clothes", Game look → Clothes): new looks of the game's customer clothing
    /// pieces. When vanilla puts a piece on a customer (CustomerLooks.PieceOn), a look of that piece may restyle it — the
    /// regular's outfit, a look replacing that vanilla colour variant, or a random look by its "worn" chance — by setting the
    /// colours and print on the piece's own material instances, as vanilla's colour variants do. Regulars' outfits
    /// (accessories.json customers[].outfit) change pieces through vanilla's setApparel once their persona is known.
    /// Nothing is saved: a customer is dressed again on each visit and load.
    /// </summary>
    internal static class Wardrobe
    {
        private class Part
        {
            public string tint, tintR, tintG, tintB, print;
            [JsonIgnore] public Texture2D Print;
            [JsonIgnore] public bool Loaded;
        }

        private class Look
        {
            public string id, name, body, piece, replaces;
            public List<Part> parts;
            public int worn;
        }

        private class OutfitSlot
        {
            public int slot;
            public string piece, variant, clothing;
        }

        /// <summary>A regular's own face (Studio's face editor): what a character preset sets, applied the same way.</summary>
        public class Face
        {
            public Dictionary<string, float> shapes, floats;
            public List<string> hair;
            /// <summary>Saved before anyone edited it: the character as it is.</summary>
            public bool IsEmpty => shapes == null && floats == null && hair == null && hairColor == null && textures == null && colors == null;
            public string hairColor;
            public Dictionary<string, string> textures, colors;
        }

        private class Owner
        {
            public string id, faceId;
            public List<OutfitSlot> outfit;
            public Face face;
            /// <summary>Their portrait rendered by Studio from their face ("assets/...png").</summary>
            public string portrait;
        }

        /// <summary>A face made in TCG Studio (accessories.json "faces"), in the pool walk-ins roll from.</summary>
        public class SavedFace
        {
            public string id, body, portrait;
            public int model;
            public bool noWalkIns; // kept for you to give out: walk-ins never roll it
            public Face face;
        }

        private class Library
        {
            public List<SavedFace> faces;
            public List<Look> clothes;
            public List<Owner> customers;
            public List<Owner> employees;
        }

        private static readonly Dictionary<string, List<Look>> ByPiece = new Dictionary<string, List<Look>>(); // body|piece
        private static readonly Dictionary<string, Look> ById = new Dictionary<string, Look>();
        private static readonly Dictionary<string, List<OutfitSlot>> Outfits = new Dictionary<string, List<OutfitSlot>>();
        private static readonly Dictionary<string, Face> Faces = new Dictionary<string, Face>();
        private static readonly Dictionary<string, string> PortraitFiles = new Dictionary<string, string>();
        private static readonly Dictionary<string, Sprite> Portraits = new Dictionary<string, Sprite>();
        /// <summary>Per character: slot → the look its regular's outfit puts there ("" = the piece's own colours).</summary>
        private static readonly Dictionary<CharacterCustomization, Dictionary<int, string>> Forced =
            new Dictionary<CharacterCustomization, Dictionary<int, string>>();
        private static readonly Dictionary<CharacterCustomization, Customer> Owners = new Dictionary<CharacterCustomization, Customer>();
        private static readonly HashSet<CharacterCustomization> NotCustomers = new HashSet<CharacterCustomization>();
        private static readonly System.Random Rng = new System.Random();
        private static string _folder;
        private static Texture2D _transparent;

        private static readonly List<SavedFace> Saved = new List<SavedFace>();

        private static readonly HashSet<string> Assigned = new HashSet<string>();

        /// <summary>The saved faces of this body walk-ins may roll: not switched off, and no regular or employee in TCG Studio has been given them.</summary>
        public static List<SavedFace> UnassignedFaces(bool female) => Saved.FindAll(f => f.body == (female ? "female" : "male") && !f.noWalkIns && !Assigned.Contains(f.id));

        /// <summary>This regular or employee has a face of their own.</summary>
        public static bool HasFace(string id) => id != null && Faces.ContainsKey(id);

        public static void Init(string setupDir)
        {
            if (setupDir == null) return;
            _folder = Path.Combine(setupDir, "accessories");
            string file = Path.Combine(_folder, "accessories.json");
            if (!File.Exists(file)) return;
            Library lib;
            try { lib = JsonConvert.DeserializeObject<Library>(File.ReadAllText(file)); }
            catch (Exception e)
            {
                Plugin.Log.LogError($"Could not read {file}: {e.Message}. The clothes made in TCG Studio are skipped.");
                return;
            }
            foreach (var l in lib?.clothes ?? new List<Look>())
            {
                if (string.IsNullOrEmpty(l.id) || string.IsNullOrEmpty(l.piece) || (l.body != "male" && l.body != "female") || l.parts == null || l.parts.Count == 0)
                {
                    Plugin.Log.LogWarning($"Clothes made in TCG Studio: \"{l.name}\" is incomplete and skipped");
                    continue;
                }
                ById[l.id] = l;
                string key = l.body + "|" + l.piece;
                if (!ByPiece.TryGetValue(key, out var list)) ByPiece[key] = list = new List<Look>();
                list.Add(l);
            }
            foreach (var o in (lib?.customers ?? new List<Owner>()).Concat(lib?.employees ?? new List<Owner>()))
            {
                if (string.IsNullOrEmpty(o.id)) continue;
                if (!string.IsNullOrEmpty(o.faceId)) Assigned.Add(o.faceId);
                if (o.outfit != null && o.outfit.Count > 0) Outfits[o.id] = o.outfit;
                if (o.face != null && !o.face.IsEmpty) Faces[o.id] = o.face;
                if (!string.IsNullOrEmpty(o.portrait)) PortraitFiles[o.id] = o.portrait;
            }
            foreach (var f in lib?.faces ?? new List<SavedFace>())
            {
                if (string.IsNullOrEmpty(f.id) || f.face == null || f.face.IsEmpty) continue;
                Saved.Add(f);
                Faces[f.id] = f.face;
                if (!string.IsNullOrEmpty(f.portrait)) PortraitFiles[f.id] = f.portrait;
            }
            if (PortraitFiles.Count > 0) CustomerLooks.Portrait = PortraitFor;
            if (ById.Count == 0 && Outfits.Count == 0 && Faces.Count == 0) return;
            Plugin.Log.LogInfo($"Clothes: {ById.Count} look(s) made in TCG Studio, {Outfits.Count} regular(s) with an outfit, {Faces.Count} with their own face");
            CustomerLooks.PieceOn += OnPieceOn;
            CustomerLooks.Dressed += OnDressed;
            CustomerLife.Leaving += c =>
            {
                if (c.m_CharacterCustom != null) Forced.Remove(c.m_CharacterCustom);
                FaceHeads.Clear(c);
            };
            Customers.Cast.VisitStarted += (c, p, loaded) => Dress(c);
            if (Faces.Count > 0) CustomerLife.SlowTick.Add(HoldFace); // LOD groups only take it once the customer is active
        }

        /// <summary>The customer a character belongs to; null for a worker (same customizer).</summary>
        private static Customer OwnerOf(CharacterCustomization cc)
        {
            if (Owners.TryGetValue(cc, out var c))
            {
                if (c != null || NotCustomers.Contains(cc)) return c;
            }
            else if (NotCustomers.Contains(cc)) return null;
            // Customers are pooled objects made with the scene; a new scene (a loaded save) brings new ones.
            Owners.Clear();
            NotCustomers.RemoveWhere(x => x == null);
            foreach (var cu in UnityEngine.Object.FindObjectsOfType<Customer>(true))
                if (cu.m_CharacterCustom != null) Owners[cu.m_CharacterCustom] = cu;
            if (Owners.TryGetValue(cc, out c)) return c;
            NotCustomers.Add(cc);
            return null;
        }

        /// <summary>The preset was applied again (a new look for the visitor): the regular's outfit goes back on.</summary>
        private static void OnDressed(CharacterCustomization cc)
        {
            var c = OwnerOf(cc);
            if (c != null) Dress(c);
            else if (Employees.Roster.Of(cc) is Employees.Roster.Custom e && cc.m_HasInit) DressCharacter(cc, e.Def.Id, e.Def.Id, e.Def.Name);
        }

        /// <summary>
        /// Puts a regular's own outfit and face on (setApparel per named slot, PieceOn styles each piece; vanilla setters per
        /// face setting). The outfit goes first: setApparel shapes a new piece by the preset's stored blend shapes, so the
        /// face's shapes (body_fat…) must come after to reach the clothes too, or body and clothes no longer fit (a see-through
        /// ring at the waist). Their character's preset is applied before every time (ApplyCharacterVars), and every preset
        /// sets all of what a face sets, so the next visitor in this pooled customer gets their own face back.
        /// </summary>
        private static void Dress(Customer c)
        {
            var cc = c?.m_CharacterCustom;
            // Not set up yet (a customer restored from a save before their character's Initialize): its hair and apparel
            // object lists are still empty, and setHair/setApparel index them (ArgumentOutOfRange). Initialize ends with
            // LoadFromJSON → ApplyCharacterVars → CustomerLooks.Dressed → here again, set up.
            if (cc == null || !cc.m_HasInit) return;
            var p = Customers.Cast.Of(c);
            DressCharacter(cc, p?.id, p?.LookId, p?.name);
            FaceHeads.Clear(c);
            HoldFace(c);
        }

        private static void DressCharacter(CharacterCustomization cc, string id, string faceId, string name)
        {
            ClearExtraShapes(cc);
            if (id != null && Outfits.TryGetValue(id, out var outfit))
            {
                var forced = new Dictionary<int, string>();
                Forced[cc] = forced;
                foreach (var o in outfit)
                {
                    if (o.slot < 0 || o.slot >= cc.ApparelTables.Count) continue;
                    var items = cc.ApparelTables[o.slot].Items;
                    int sel = items.FindIndex(it => it.Name == o.piece);
                    if (sel < 0) continue; // a piece of the other body
                    int variant = Math.Max(0, items[sel].Materials.FindIndex(m => m.Label == o.variant));
                    forced[o.slot] = o.clothing ?? "";
                    cc.setApparel(sel, o.slot, variant);
                }
            }
            else Forced.Remove(cc);
            if (faceId != null && Faces.TryGetValue(faceId, out var face))
            {
                PutOn(cc, face);
                RememberExtraShapes(cc, face);
                if (Plugin.CustomerDebug.Value) LogFace(cc, name, face);
            }
        }

        private static void HoldFace(Customer c)
        {
            var p = Customers.Cast.Of(c);
            if (p != null && Faces.ContainsKey(p.LookId)) FaceHeads.Hold(c);
        }

        /// <summary>
        /// Shapes a face sets that the game's presets don't (expressions, body_curvy…): a preset resets only its own (mod_*,
        /// shp_*, body_fat), so these go back to 0 before the pooled character is dressed for its next visitor.
        /// </summary>
        private static readonly Dictionary<CharacterCustomization, List<string>> ExtraShapes =
            new Dictionary<CharacterCustomization, List<string>>();

        private static bool IsPresetShape(string k) => k.StartsWith("mod_") || k.StartsWith("shp_") || k == "body_fat";

        private static void RememberExtraShapes(CharacterCustomization cc, Face f)
        {
            if (f.shapes == null) return;
            List<string> list = null;
            foreach (var kv in f.shapes)
            {
                if (kv.Value == 0f || IsPresetShape(kv.Key)) continue;
                if (list == null) list = new List<string>();
                list.Add(kv.Key);
            }
            if (list != null) ExtraShapes[cc] = list;
        }

        private static void ClearExtraShapes(CharacterCustomization cc)
        {
            if (!ExtraShapes.TryGetValue(cc, out var list)) return;
            ExtraShapes.Remove(cc);
            foreach (var k in list) cc.setBlendshapeByName(k, 0f, false);
            if (ExtraShapes.Count > 0) // characters destroyed with their scene (a loaded save)
                foreach (var dead in ExtraShapes.Keys.Where(x => x == null).ToList()) ExtraShapes.Remove(dead);
        }

        /// <summary>[Debug] CustomerDebug: the face settings put on a regular (what the game's customizer was given).</summary>
        private static void LogFace(CharacterCustomization cc, string name, Face f)
        {
            string Get(Dictionary<string, string> d, string k) => d != null && d.TryGetValue(k, out var v) ? v : "-";
            float GetF(string k) => f.floats != null && f.floats.TryGetValue(k, out var v) ? v : 0f;
            Plugin.Log.LogInfo($"Clothes: face put on {name}: {f.shapes?.Count ?? 0} shape(s), skin {Get(f.textures, "_Skin_Texture")} " +
                $"tint {Get(f.colors, "_Skin_Tint")} x{GetF("_Skin_Tint_Alpha"):0.00}, shine {GetF("_Skin_Smoothness_Offset"):0.00}, " +
                $"height {GetF("Height"):0.00}, width {GetF("Width"):0.00}");
        }

        /// <summary>
        /// The portrait of the person wearing this character now (while someone with a face of their own visits, Cast keeps others off
        /// their character); null when it isn't a regular with a portrait of their own.
        /// </summary>
        private static Sprite PortraitFor(int model, bool female)
        {
            var cm = CSingleton<CustomerManager>.Instance;
            if (cm == null) return null;
            foreach (var list in new[] { cm.GetCustomerList(), cm.GetTournamentCustomerList() })
                foreach (var c in list)
                {
                    if (c == null || !c.m_IsActive || c.m_IsFemale != female || c.GetCustomerModelIndex() != model) continue;
                    var p = Customers.Cast.Of(c);
                    if (p == null || !PortraitFiles.TryGetValue(p.LookId, out var file)) continue;
                    if (!Portraits.TryGetValue(p.LookId, out var sprite))
                    {
                        string path = StudioPaths.ItemFile(_folder, file);
                        var tex = path == null ? null : ShopPicture.Load(path, "portrait of " + p.name);
                        sprite = tex == null ? null : Sprite.Create(tex, new Rect(0, 0, tex.width, tex.height), new Vector2(0.5f, 0.5f));
                        Portraits[p.LookId] = sprite;
                    }
                    return sprite;
                }
            return null;
        }

        private static void PutOn(CharacterCustomization cc, Face f)
        {
            if (f.shapes != null)
                foreach (var kv in f.shapes) cc.setBlendshapeByName(kv.Key, kv.Value, false);
            if (f.hair != null)
                for (int slot = 0; slot < f.hair.Count && slot < cc.HairTables.Count; slot++)
                    if (!string.IsNullOrEmpty(f.hair[slot])) cc.setHairByName(f.hair[slot], slot);
            if (!string.IsNullOrEmpty(f.hairColor) && ColorUtility.TryParseHtmlString(f.hairColor, out var hc))
                for (int slot = 0; slot < cc.HairTables.Count; slot++)
                    cc.setHairColor(new CC_Property { propertyName = "_Hair_Color" }, hc, slot);
            if (f.floats != null)
                foreach (var kv in f.floats) cc.setFloatProperty(new CC_Property { propertyName = kv.Key, floatValue = kv.Value });
            if (f.textures != null)
                foreach (var kv in f.textures)
                    if (!string.IsNullOrEmpty(kv.Value)) cc.setTextureProperty(new CC_Property { propertyName = kv.Key, stringValue = kv.Value });
            if (f.colors != null)
                foreach (var kv in f.colors)
                    if (ColorUtility.TryParseHtmlString(kv.Value, out var col)) cc.setColorProperty(new CC_Property { propertyName = kv.Key }, col);
        }

        private static void OnPieceOn(CharacterCustomization cc, int slot, scrObj_Apparel.Apparel piece, int variant, GameObject obj)
        {
            var c = OwnerOf(cc);
            var employee = c == null ? Employees.Roster.Of(cc) : null;
            if (c == null && employee == null) return; // one of the game's workers
            bool female = c != null ? c.m_IsFemale : employee.Def.Female;
            Look look = null;
            if (Forced.TryGetValue(cc, out var forced) && forced.TryGetValue(slot, out var id))
            {
                if (!string.IsNullOrEmpty(id) && ById.TryGetValue(id, out var l) && l.piece == piece.Name) look = l;
                if (look == null) return; // the outfit asks for the piece's own colours
            }
            else if (ByPiece.TryGetValue((female ? "female" : "male") + "|" + piece.Name, out var looks))
            {
                string label = variant >= 0 && variant < piece.Materials.Count ? piece.Materials[variant].Label : null;
                look = looks.Find(l => !string.IsNullOrEmpty(l.replaces) && l.replaces == label);
                if (look == null)
                {
                    int start = Rng.Next(looks.Count);
                    for (int i = 0; i < looks.Count && look == null; i++)
                    {
                        var l = looks[(start + i) % looks.Count];
                        if (l.worn > 0 && Rng.Next(100) < l.worn) look = l;
                    }
                }
            }
            if (look != null) Apply(look, obj);
        }

        private static void Apply(Look look, GameObject obj)
        {
            foreach (var r in obj.GetComponentsInChildren<SkinnedMeshRenderer>(true))
            {
                var mats = r.materials;
                for (int j = 0; j < mats.Length && j < look.parts.Count; j++)
                {
                    var part = look.parts[j];
                    SetColor(mats[j], "_Tint", part.tint);
                    SetColor(mats[j], "_Tint_R", part.tintR);
                    SetColor(mats[j], "_Tint_G", part.tintG);
                    SetColor(mats[j], "_Tint_B", part.tintB);
                    mats[j].SetTexture("_Print", PrintOf(look, part));
                }
            }
        }

        private static void SetColor(Material m, string prop, string hex)
        {
            if (string.IsNullOrEmpty(hex) || !ColorUtility.TryParseHtmlString(hex, out var col)) return;
            m.SetColor(prop, col);
        }

        private static Texture2D PrintOf(Look look, Part part)
        {
            if (!part.Loaded)
            {
                part.Loaded = true;
                string path = StudioPaths.ItemFile(_folder, part.print);
                if (path != null) part.Print = ShopPicture.Load(path, "clothes print " + look.name, mipmaps: true);
            }
            if (part.Print != null) return part.Print;
            if (_transparent == null) _transparent = Resources.Load<Texture2D>("T_Transparent"); // vanilla's "no print"
            return _transparent;
        }
    }
}
