package forge.gamemodes.limited;

/**
 * TCG Custom Cards bridge helper (GPL-3.0): Forge's DeckColors (the colour commitment its draft AI keeps per seat) has a
 * package-private constructor; this class lives in Forge's package only to create one. Used by tcgcc.bridge.Draft.
 */
public final class TcgDraftColors {
    private TcgDraftColors() {}

    public static DeckColors create() {
        return new DeckColors();
    }
}
