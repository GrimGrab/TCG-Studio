# Making custom cards that play in the Tetramon card game

This guide covers the play-table card game that comes with the game (Tetramon). It explains the `play` block of a card in
`set.json`: lane attack, element, evolution and the card's **effect** when it is played. In TCG Studio you edit all of this on
a card's detail panel (Sets → your set → Cards → pick a card). Saving the set is enough, because the game reads it directly.

## The play block

```json
"play": {
  "laneAttack": [0, 5, 40, 5],
  "element": "Water",
  "evolvesFrom": "baby-wisp",
  "effect": { ... }
}
```

| Field | What it does |
|---|---|
| `laneAttack` | Attack in each lane, in the order **Fire, Earth, Water, Wind**. At the end of your turn each lane hits the opponent for its top card's attack plus any boosts. These numbers are also printed on the card. |
| `element` | `Fire`, `Earth`, `Water` or `Wind`. Used by searches that only find one element (`FireTypeOnly` and so on) and by the deck editor's counts. It doesn't change damage. |
| `evolvesFrom` | Id of another card **in the same set**. That card must be on top of a lane before this one can be placed on it. Leave it out for a basic card. A lane holds at most 4 cards, so chains can have up to 4 stages. |
| `effect` | Optional. Runs every time the card is placed on a lane. Leave it out for a card with no effect. |

The card's `description` is only text. Nothing checks that it matches the effect, so write the text to say what the effect does.

## Effect basics

An effect is a list of **steps** that run in order:

```json
"effect": {
  "playEffectQueueDataList": [
    { "playEffectType": "DrawCard", "playEffectTypeSecondary": "None", "countList": [2] }
  ]
}
```

This one says "When played: draw 2 cards."

Each step has these fields. Only `playEffectType` and `countList` are required.

| Field | Meaning |
|---|---|
| `playEffectType` | What the step does (see the list below). **Never `None`**: the game would freeze. |
| `playEffectTypeSecondary` | Optional condition or multiplier, `None` by default (see below). |
| `countList` | `[amount]` or `[amount, divisor]`. At least one number. |
| `isTargetEnemy` | Do it to the opponent instead of you. |
| `isTargetBoth` | Do it to both players. |
| `isOptional` | For discards: the player may choose not to. |
| `failIfNotEnoughCard` | For discards: the step fails if there aren't enough cards. |
| `onlyRunIfPreviousQueueSuccess` | Skip this step unless the step before it worked. |
| `onlyRunIfPreviousQueueFailed` | Skip this step unless the step before it failed. |
| `delayStartTime` | Seconds to wait before this step starts (just for pacing). |

Optional effect-level fields, next to `playEffectQueueDataList`:

| Field | Meaning |
|---|---|
| `effectMonsterTypeList` | List of cards that some steps refer to. Use card ids from your set (`"baby-wisp"`) or vanilla monster names (`"PiggyA"`). |
| `searchData` | Settings for `SearchDeck` / `SearchDiscardPile` (see "Searching"). |
| `name` | Defaults to the card's name. |

Write enum values as their names in quotes (`"DrawCard"`, not `3`).

## Step types (`playEffectType`)

"Amount" means `countList[0]`, multiplied by the secondary if there is one.

**Cards**
| Type | Effect |
|---|---|
| `DrawCard` | Draw *amount* cards. |
| `DrawCardUntilXInHand` | Draw until you hold *amount* cards. |
| `DiscardHandCard` | Choose *amount* cards from your hand and discard them. |
| `DiscardAllHandCard` | Discard your whole hand. |
| `DiscardDeckTop` | Put the top *amount* cards of the deck into the discard pile. |
| `ShuffleHandToDeckAndDraw` | Shuffle your hand into the deck, then draw *amount*. |
| `DiscardHandCardSpecificMonsterType` | Discard *amount* cards that are in `effectMonsterTypeList` (needs that list). |
| `SearchDeck`, `SearchDiscardPile` | Look through cards and pick some (needs `searchData`). |

**Attack**
| Type | Effect |
|---|---|
| `BoostCardActiveElement` | +*amount* attack for this card in the lane it was played in. |
| `BoostActiveCardFireElement` / `Earth` / `Water` / `Wind` | +*amount* attack for this card in that lane only. |
| `BoostFireElement` / `Earth` / `Water` / `Wind` | +*amount* attack to that whole lane, not tied to this card. |
| `AbsorbDiscardedCardBoostActiveElement` | This card gains the attack of the card discarded by an earlier step (in the played lane). |
| `AbsorbDiscardedCardBoostFireElement` | Same, but uses the discarded card's Fire attack. |
| `AbsorbBothPlayersDiscardedCardBoostActiveElement` | Gains the attack of the cards both players discarded. |
| `AbsorbSearchedCardBoostActiveElement` | Gains the attack of the card found by an earlier search step. |

Negative amounts work too, for a drawback.

**Field**
| Type | Effect |
|---|---|
| `BounceToHand` | Pick a lane: its cards go back to the hand. With `isTargetEnemy`, an opponent lane. |
| `DestoryCard` (spelled like that) | Pick a lane: its cards go to the discard pile. |
| `DestroyTopFieldCard` | Pick a lane: only its top card is discarded. |
| `AddTamerShield` | Gives you a shield of *amount* points, which absorbs damage before your HP. |
| `DestroyTamerShield` | Removes the shield (yours, or with `isTargetEnemy` the opponent's). |
| `AddFreezeCounter` | Adds *amount* freeze counters. At 8 that player's lanes don't attack. Usually used with `isTargetEnemy`. |
| `RemoveAllFreezeCounter` | Clears the freeze counters. |
| `CannotEvolve` | That player can't evolve for a turn. Usually used with `isTargetEnemy`. |
| `StopPlayEffect` | The next *amount* cards played (by either player) trigger no effect. |

**Evolution**
| Type | Effect |
|---|---|
| `QuickEvolve` | This card can evolve on the same turn it was played. |
| `InstantEvolve` | Choose a card from `effectMonsterTypeList` in your hand and evolve straight into it (needs that list). |

## Secondary (`playEffectTypeSecondary`)

**Multipliers.** Amount = `countList[0]` × (count ÷ `countList[1]`, rounded down). They need **two** numbers in `countList`.

| Secondary | Counts |
|---|---|
| `ForEachCardInHand` / `ForEachCardInHandEnemy` | cards in hand |
| `ForEachCardInDiscardPile` / `ForEachCardInDiscardPileEnemy` | cards in the discard pile |
| `ForEachCardInDeck` | cards left in your deck |
| `ForEachDiscardedCardThisTurn` | cards you discarded this turn |
| `ForEachDiscardedCardLastTrigger` | cards discarded by the earlier steps of this effect |
| `ForEachGuardianCardTakenSelf` | guardian cards you have taken |
| `ForEachFreezeCounterEnemy` | the opponent's freeze counters |
| `ForEachMonsterTypeInFieldAndDiscardPile` | cards from `effectMonsterTypeList` on both fields and in both discard piles |

`"countList": [5, 2]` with `ForEachCardInHand` and 6 cards in hand = 5 × (6 ÷ 2) = 15.

**Conditions.** These decide whether the step counts as "worked". Combine them with `onlyRunIfPreviousQueueSuccess`.

| Secondary | Works when |
|---|---|
| `AmountCheckDiscardPile` / `AmountCheckDiscardPileEnemy` | that discard pile has at least `countList[1]` cards |
| `IsCardOnYourField` | a card from `effectMonsterTypeList` is on your field |
| `IsTamerShieldActive` | you have a shield |
| `DiscardSelfIfFailLastQueue` | Not a check: if the previous step failed, this card is discarded from its lane. |

`SameElementOfDiscardedCard`, `SameElementOfSearchedCard`, `TargetAllFieldCardMonsterType` and `skipPlayEffectIfSecondaryFufill`
exist in the game's data, but the game never uses them, so they have no effect.

## Searching

```json
"searchData": {
  "resolveCardCondition": "EvoOnly",
  "searchLookAtCount": 50,
  "searchPickCount": 1,
  "mustFufillCount": false,
  "isRevealCard": true,
  "isSearchingDiscardPile": false,
  "shuffleDeckFirst": false,
  "shuffleDeckEnd": true,
  "selectedCardQueueType": "ToHand",
  "remainingCardQueueType": "None",
  "specificMonsterTypeList": []
}
```

| Field | Meaning |
|---|---|
| `searchLookAtCount` | How many cards from the top you look at. **0 = none.** Use 50 for the whole deck. |
| `searchPickCount` | How many you may take. |
| `resolveCardCondition` | `Any`, `BasicOnly`, `EvoOnly`, `FireTypeOnly`, `EarthTypeOnly`, `WaterTypeOnly`, `WindTypeOnly`, or `MonsterTypeListOnly` (uses `specificMonsterTypeList`). |
| `mustFufillCount` | Must take the full amount if possible. |
| `isRevealCard` | Show the picked card to the opponent. |
| `isSearchingDiscardPile` | Look in the discard pile instead of the deck. **Set this for `SearchDiscardPile` too**, because the step type alone doesn't switch. |
| `shuffleDeckFirst` / `shuffleDeckAfterRemaining` / `shuffleDeckEnd` | When to shuffle. |
| `selectedCardQueueType` | Where picked cards go: `ToHand`, `ToDiscard`, `ToDeckTop`, `ToDeckBtm`, `ToElementArea`, … |
| `remainingCardQueueType` | Where the cards you looked at but didn't pick go (`None` = they stay). |
| `specificMonsterTypeList` | Card ids or vanilla names for `MonsterTypeListOnly`. |

## Examples

**+10 attack in the lane it's played in**
```json
{ "playEffectQueueDataList": [ { "playEffectType": "BoostCardActiveElement", "countList": [10] } ] }
```

**Discard a card; if you did, draw 2**
```json
{ "playEffectQueueDataList": [
  { "playEffectType": "DiscardHandCard", "countList": [1], "isOptional": true, "failIfNotEnoughCard": true },
  { "playEffectType": "DrawCard", "countList": [2], "onlyRunIfPreviousQueueSuccess": true }
] }
```

**+5 attack for every 2 cards in your discard pile**
```json
{ "playEffectQueueDataList": [
  { "playEffectType": "BoostCardActiveElement", "playEffectTypeSecondary": "ForEachCardInDiscardPile", "countList": [5, 2] }
] }
```

**Freeze the opponent by 3 and break their shield**
```json
{ "playEffectQueueDataList": [
  { "playEffectType": "AddFreezeCounter", "countList": [3], "isTargetEnemy": true },
  { "playEffectType": "DestroyTamerShield", "countList": [0], "isTargetEnemy": true }
] }
```

**Search your deck for an evolution card**
```json
{ "playEffectQueueDataList": [ { "playEffectType": "SearchDeck", "countList": [1] } ],
  "searchData": { "resolveCardCondition": "EvoOnly", "searchLookAtCount": 50, "searchPickCount": 1,
                  "isRevealCard": true, "shuffleDeckEnd": true, "selectedCardQueueType": "ToHand",
                  "remainingCardQueueType": "None" } }
```

**Only if your partner card is on the field: +20 attack**
```json
{ "playEffectQueueDataList": [
  { "playEffectType": "BoostCardActiveElement", "playEffectTypeSecondary": "IsCardOnYourField", "countList": [0] },
  { "playEffectType": "BoostCardActiveElement", "countList": [20], "onlyRunIfPreviousQueueSuccess": true }
],
  "effectMonsterTypeList": ["my-partner-card"] }
```

## When an effect doesn't work

If an effect would freeze or crash the game, the mod refuses it. The card then plays with **no effect**, and the reason is
written to the log. You can see it in Studio under **Game & mod → Debug log** (or in `<game>\BepInEx\LogOutput.log`) as
`Set … card '…': effect ignored — …`. The mod refuses an effect when:

- the step list is empty, or a step's `playEffectType` is `None`;
- a `countList` is missing or empty;
- an `AmountCheck…` / `ForEach…` secondary has fewer than 2 numbers in `countList`;
- `DiscardHandCardSpecificMonsterType`, `InstantEvolve`, `IsCardOnYourField` or `ForEachMonsterTypeInFieldAndDiscardPile`
  has no `effectMonsterTypeList`;
- `SearchDeck` / `SearchDiscardPile` has no `searchData`;
- a card reference is neither a card id from this set nor a vanilla monster name;
- a field name is misspelled (unknown fields are errors).

## Playing your cards

- Decks are exactly 50 cards, with at most 4 copies of a card. Build them at the workbench and add cards from the binder.
  Custom and vanilla cards can be mixed.
- Deck text export/import keeps custom cards as `custom:<set>:<card>`.
- Opponents use the game's own decks for now. Custom cards only show up in your deck.
