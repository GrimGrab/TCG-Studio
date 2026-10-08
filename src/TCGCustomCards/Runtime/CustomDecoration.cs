using TCGCustomCards.Core;

namespace TCGCustomCards.Runtime
{
    /// <summary>
    /// A custom decoration from the accessory library. Placeable ones (posters, objects) get an EDecoObject int from
    /// <see cref="Registry.BuildDecorations"/>; surfaces (wall/floor/ceiling) get their list index when injected
    /// (<see cref="DecorationInjector"/>). Both are ephemeral; saves use the stable key below.
    /// </summary>
    internal class CustomDecoration
    {
        public DecorationDef Def;
        /// <summary>Placeable: our EDecoObject value (None for surfaces).</summary>
        public EDecoObject Object = EDecoObject.None;
        /// <summary>Surface: index in the game's wall/floor/ceiling list (-1 until injected).</summary>
        public int Index = -1;
        /// <summary>Placeable: hidden copy of the base decoration prefab with our model.</summary>
        public InteractableObject Prefab;
        public DecoData Data;
        public DecoPurchaseData Purchase;
        public ShopDecoData Surface;

        public string Key => $"deco/{Def.Id}";
        public bool Ready => Def.IsSurface ? Index >= 0 : Prefab != null;
    }
}
