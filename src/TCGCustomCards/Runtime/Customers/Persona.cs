using System.Collections.Generic;
using Newtonsoft.Json;
using Newtonsoft.Json.Linq;

namespace TCGCustomCards.Runtime.Customers
{
    /// <summary>
    /// A named customer. Regulars persist (side-car, or the setup item library when made in TCG Studio); walk-ins live
    /// for one visit unless they are promoted when they leave. Saved for every regular in every slot, so it stays small:
    /// defaults are left out of the JSON, and [Customers] MaxRegulars caps how many exist.
    /// </summary>
    [JsonObject(ItemNullValueHandling = NullValueHandling.Ignore)]
    internal class Persona
    {
        public string id;
        public string name;
        /// <summary>Must match the pooled Customer's m_IsFemale: each Customer object has a fixed body.</summary>
        [JsonProperty(DefaultValueHandling = DefaultValueHandling.Ignore)]
        public bool female;
        /// <summary>Character model index (vanilla "Male"/"Female" + index); -1 = pinned to whatever model the first visit had.</summary>
        [JsonProperty(DefaultValueHandling = DefaultValueHandling.Ignore), System.ComponentModel.DefaultValue(-1)]
        public int model = -1;
        /// <summary>Trait ids (<see cref="Traits"/>). Unknown ids are kept and ignored.</summary>
        public List<string> traits = new List<string>();
        /// <summary>Table skill 0-100 (rolled on creation; TCG Studio can set it). -1 = not rolled yet.</summary>
        [JsonProperty(DefaultValueHandling = DefaultValueHandling.Ignore), System.ComponentModel.DefaultValue(-1)]
        public int skill = -1;
        /// <summary>Their talking voice (TCG Studio); null = one worked out from their id.</summary>
        [JsonProperty(NullValueHandling = NullValueHandling.Ignore)]
        public CustomerVoice voice;
        /// <summary>Own lines per situation (TCG Studio), added to the ones they inherit.</summary>
        public Dictionary<string, List<string>> lines;
        /// <summary>Inherited lines (built-in, the setup's, their traits') they never say, per situation (TCG Studio).</summary>
        public Dictionary<string, List<string>> removed;
        /// <summary>Own animations per moment (TCG Studio), added to the ones they inherit.</summary>
        public Dictionary<string, List<string>> anims;
        /// <summary>Inherited animations they never play, per moment (TCG Studio).</summary>
        public Dictionary<string, List<string>> animsOff;
        /// <summary>A saved face from TCG Studio this person rolled as a walk-in. Null for the rest.</summary>
        [JsonProperty(NullValueHandling = NullValueHandling.Ignore)]
        public string rolledFace;
        /// <summary>This session's Chaser returns: the day and how many that day (not saved).</summary>
        [JsonIgnore] public int chaseDay, chases;
        /// <summary>The id their face and portrait are kept under (Runtime/Clothes/Wardrobe).</summary>
        [JsonIgnore] public string LookId => rolledFace ?? id;
        /// <summary>Set in TCG Studio or on the shop whiteboard: what they're known as. They don't earn another.</summary>
        public string nickname;
        /// <summary>Renamed or nicknamed on the whiteboard: this save's name and nickname win over TCG Studio's.</summary>
        [JsonProperty(DefaultValueHandling = DefaultValueHandling.Ignore)]
        public bool edited;
        [JsonProperty(DefaultValueHandling = DefaultValueHandling.Ignore)]
        public bool regular;
        /// <summary>"authored" (setup file), "promoted" (a walk-in who became a regular) or "walk-in".</summary>
        public string origin;

        // Memory of this persona in the current save.
        [JsonProperty(DefaultValueHandling = DefaultValueHandling.Ignore)]
        public int visits;
        [JsonProperty(DefaultValueHandling = DefaultValueHandling.Ignore), System.ComponentModel.DefaultValue(-1)]
        public int firstDay = -1;
        [JsonProperty(DefaultValueHandling = DefaultValueHandling.Ignore), System.ComponentModel.DefaultValue(-1)]
        public int lastDay = -1;
        [JsonProperty(DefaultValueHandling = DefaultValueHandling.Ignore)]
        public float spent;

        /// <summary>Copy that shares no lists with this one.</summary>
        public Persona Clone()
        {
            var p = (Persona)MemberwiseClone();
            p.traits = new List<string>(traits ?? new List<string>());
            if (lines != null) p.lines = new Dictionary<string, List<string>>(lines);
            if (removed != null) p.removed = new Dictionary<string, List<string>>(removed);
            if (anims != null) p.anims = new Dictionary<string, List<string>>(anims);
            if (animsOff != null) p.animsOff = new Dictionary<string, List<string>>(animsOff);
            return p;
        }
    }

    /// <summary>Side-car section "personas".</summary>
    /// <summary>A talking voice. Values not in the file keep these middles.</summary>
    public class CustomerVoice
    {
        /// <summary>How fast the letter recordings are read: 1 = as recorded, 2 = an octave up.</summary>
        public float pitch = 1f;
        /// <summary>0 muffled … 0.5 as recorded … 1 crisp.</summary>
        public float tone = 0.5f;
    }

    internal class PersonaSave
    {
        public int version = 1;
        public List<Persona> regulars = new List<Persona>();
        /// <summary>Persona id per entry of the vanilla saved customer list (CPlayerData.m_CustomerSaveDataList), null = none.</summary>
        public List<string> present = new List<string>();
        /// <summary>Walk-ins inside the shop at save time (their ids appear in <see cref="present"/>).</summary>
        public List<Persona> walkIns = new List<Persona>();
        /// <summary>Removed on the whiteboard; regulars made in TCG Studio stay out of this save.</summary>
        [JsonProperty(NullValueHandling = NullValueHandling.Ignore)]
        public List<string> forgotten;
        /// <summary>Per-module state, written back untouched for modules that aren't loaded.</summary>
        public Dictionary<string, JToken> modules = new Dictionary<string, JToken>();
    }

    /// <summary>The customers list of the setup's item library (&lt;setup&gt;\accessories\accessories.json, TCG Studio's Customers page).</summary>
    internal class CustomersFile
    {
        [JsonProperty("customers")]
        public List<Persona> customers = new List<Persona>();
    }

    /// <summary>The setup's own lines (&lt;setup&gt;\game\customer-lines.json): per situation, lines added to the built-in ones
    /// and built-in lines switched off.</summary>
    internal class CustomerLinesFile
    {
        [JsonProperty("lines")]
        public Dictionary<string, List<string>> lines = null;
        [JsonProperty("removed")]
        public Dictionary<string, List<string>> removed = null;
        /// <summary>Generated names ("female", "male", "last"): added, and built-in ones switched off.</summary>
        [JsonProperty("names")]
        public Dictionary<string, List<string>> names = null;
        [JsonProperty("namesOff")]
        public Dictionary<string, List<string>> namesOff = null;
        /// <summary>Employees' lines: added per situation, and built-in ones switched off.</summary>
        [JsonProperty("employeeLines")]
        public Dictionary<string, List<string>> employeeLines = null;
        [JsonProperty("employeeRemoved")]
        public Dictionary<string, List<string>> employeeRemoved = null;
    }
}
