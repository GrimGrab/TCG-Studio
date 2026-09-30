using TCGCustomCards.Core;

namespace TCGCustomCards.Runtime
{
    /// <summary>
    /// A custom furniture piece from the accessory library. Its EObjectType int is assigned by <see cref="Registry.BuildFurniture"/>
    /// (ephemeral, in our own block); saves use the stable key below. The prefab is built by <see cref="FurnitureInjector"/>.
    /// </summary>
    internal class CustomFurniture
    {
        public FurnitureDef Def;
        public EObjectType Object = EObjectType.None;
        /// <summary>Hidden copy of the base prefab with our look and spots; the game instantiates it like any furniture prefab.</summary>
        public InteractableObject Prefab;

        public string Key => $"fur/{Def.Id}";
    }
}
