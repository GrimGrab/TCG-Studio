using System.Collections.Generic;
using Newtonsoft.Json.Linq;

namespace TCGCustomCards.Runtime.Customers
{
    /// <summary>
    /// One feature on top of the cast (dialogue, wishlists, relationships, …). A module subscribes to Cast / hook events in
    /// <see cref="Init"/> and checks <see cref="Enabled"/> in its handlers, so turning its setting off stops it live. Its saved
    /// state is its own section of the side-car's personas.modules; sections of modules that aren't loaded are kept untouched.
    /// </summary>
    internal interface IPersonaModule
    {
        /// <summary>Side-car section name; never change it.</summary>
        string Id { get; }
        bool Enabled { get; }
        void Init();
        /// <summary>Restores state (null = new game or nothing saved).</summary>
        void Load(JToken state);
        /// <summary>
        /// State to save, only for personas in <paramref name="keep"/> (regulars and the walk-ins being saved). Must stay small:
        /// see the save size budget in the design doc. Null = nothing to save.
        /// </summary>
        JToken Save(ICollection<string> keep);
        /// <summary>Short text for the regulars list, or null.</summary>
        string Summary(Persona p);
    }

    /// <summary>
    /// A module's per-persona state: a dictionary by persona id, saved for kept ids only. Default values are left out of the
    /// JSON, so every field of <typeparamref name="T"/> must start at its type default (0, false, null).
    /// </summary>
    internal class PersonaTable<T> where T : class, new()
    {
        private Dictionary<string, T> _rows = new Dictionary<string, T>();

        /// <summary>Compact: nulls and default values are left out of the save.</summary>
        private static readonly Newtonsoft.Json.JsonSerializer Compact = new Newtonsoft.Json.JsonSerializer
        {
            NullValueHandling = Newtonsoft.Json.NullValueHandling.Ignore,
            DefaultValueHandling = Newtonsoft.Json.DefaultValueHandling.Ignore,
        };

        public IEnumerable<KeyValuePair<string, T>> Rows => _rows;

        public T Get(Persona p, bool create = true)
        {
            if (p == null) return null;
            if (_rows.TryGetValue(p.id, out var row)) return row;
            if (!create) return null;
            return _rows[p.id] = new T();
        }

        public void Remove(string id) => _rows.Remove(id);

        public void Load(JToken state) =>
            _rows = state?.ToObject<Dictionary<string, T>>() ?? new Dictionary<string, T>();

        public JToken Save(ICollection<string> keep)
        {
            var kept = new Dictionary<string, T>();
            foreach (var kv in _rows) if (keep.Contains(kv.Key)) kept[kv.Key] = kv.Value;
            _rows = kept; // walk-ins who left are forgotten here too
            return kept.Count == 0 ? null : JToken.FromObject(kept, Compact);
        }
    }
}
