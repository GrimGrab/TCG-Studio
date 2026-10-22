using HarmonyLib;

namespace TCGCustomCards.Patches
{
    /// <summary>
    /// [Gameplay] StinkyCustomers off: every spawn rolls stinky in ActivateCustomer (Customer.cs:381) only when canSpawnSmelly is
    /// set, so clearing it stops new stinky customers. Saved ones are restored by LoadCustomerSaveData / the play table and stay.
    /// </summary>
    [HarmonyPatch(typeof(Customer), nameof(Customer.ActivateCustomer))]
    internal static class NoStinkyCustomersPatch
    {
        private static void Prefix(ref bool canSpawnSmelly)
        {
            if (!Plugin.StinkyCustomers.Value) canSpawnSmelly = false;
        }
    }
}
