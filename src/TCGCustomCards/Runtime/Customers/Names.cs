using System.Collections.Generic;

namespace TCGCustomCards.Runtime.Customers
{
    /// <summary>Generated customer names: a first and a last name from <see cref="Traits.NameList"/>, the last as an initial with [Customers] LastNameInitialOnly.</summary>
    internal static class Names
    {
        /// <summary>A name not in <paramref name="taken"/> while one can be found; after that it may repeat.</summary>
        public static string Make(bool female, System.Random rng, ICollection<string> taken)
        {
            var first = Traits.NameList(female ? "female" : "male");
            var last = Traits.NameList("last");
            if (first.Count == 0) return "Customer";
            string name = first[0];
            for (int i = 0; i < 50; i++)
            {
                name = first[rng.Next(first.Count)];
                if (last.Count > 0)
                {
                    string surname = last[rng.Next(last.Count)];
                    name += " " + (Plugin.CustomerLastNameInitialOnly.Value ? surname.Substring(0, 1) + "." : surname);
                }
                if (!taken.Contains(name)) break;
            }
            return name;
        }
    }
}
