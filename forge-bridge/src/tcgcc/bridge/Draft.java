package tcgcc.bridge;

import forge.card.CardDb;
import forge.deck.CardPool;
import forge.deck.Deck;
import forge.gamemodes.limited.BoosterDeckBuilder;
import forge.gamemodes.limited.CardRanker;
import forge.gamemodes.limited.DeckColors;
import forge.gamemodes.limited.SealedDeckBuilder;
import forge.gamemodes.limited.TcgDraftColors;
import forge.item.PaperCard;
import forge.model.FModel;

import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;

/**
 * Booster draft over packs the game supplies (each seat opens the packs that customer really bought). Every physical card
 * keeps the game's {@code ref} (same-name cards can be different foil/border copies in the game), so the pick loop is ours:
 * packs pass left/right/left by round, one card per seat per pass. Every AI choice is Forge's draft AI: CardRanker
 * .rankCardsInPack with the seat's DeckColors (what LimitedPlayerAI.chooseCard does), and each AI deck is Forge's
 * BoosterDeckBuilder over that seat's own picks (what LimitedPlayerAI.buildDeck does).
 *
 * <p>{@code draft{id, humanSeat (-1 = all AI), landSet, packs[round][seat][{ref, name, set}]}} → for the human seat
 * {@code ask{kind: draftpick, round, rounds, pick, pack[{ref, name, set}], picks[ref]}} → {@code answer{picked:[index]}};
 * finally {@code draftdone{id, seats[{picks[ref], deck[{n, name, set}] (AI seats) | suggested[…] (human seat), rating, colors}], lines[]}} or
 * {@code draftdone{id, aborted: true}}.
 */
final class Draft {
    private record Slot(int ref, PaperCard card) {}

    private Draft() {}

    static void start(Map<String, Object> m) {
        Thread t = new Thread(() -> run(m), "tcgcc-draft");
        t.setDaemon(true);
        t.start();
    }

    @SuppressWarnings("unchecked")
    private static void run(Map<String, Object> m) {
        int id = Json.i(m, "id", -1);
        List<String> lines = new ArrayList<>();
        try {
            int human = Json.i(m, "humanSeat", -1);
            String landSet = Json.s(m, "landSet");
            CardDb db = FModel.getMagicDb().getCommonCards();
            List<List<List<Slot>>> packs = new ArrayList<>(); // [round][seat] -> cards
            List<String> unknown = new ArrayList<>();
            for (Object ro : (List<Object>) m.get("packs")) {
                List<List<Slot>> round = new ArrayList<>();
                for (Object so : (List<Object>) ro) {
                    List<Slot> pack = new ArrayList<>();
                    for (Object co : (List<Object>) so) {
                        Map<String, Object> c = (Map<String, Object>) co;
                        String name = Json.s(c, "name"), set = Json.s(c, "set");
                        PaperCard pc = set == null || set.isEmpty() ? null : db.getCard(name, set);
                        if (pc == null) pc = db.getCard(name);
                        if (pc == null) { unknown.add(name); continue; }
                        pack.add(new Slot(Json.i(c, "ref", -1), pc));
                    }
                    round.add(pack);
                }
                packs.add(round);
            }
            if (!unknown.isEmpty()) lines.add("Forge doesn't know " + unknown.size() + " card(s), left out of the packs: " + unknown);
            int rounds = packs.size(), seats = rounds == 0 ? 0 : packs.get(0).size();
            if (seats < 2) throw new IllegalArgumentException("a draft needs at least 2 seats");

            List<List<Slot>> picks = new ArrayList<>();
            List<DeckColors> colors = new ArrayList<>();
            for (int s = 0; s < seats; s++) {
                picks.add(new ArrayList<>());
                colors.add(TcgDraftColors.create());
            }
            for (int r = 0; r < rounds; r++) {
                List<List<Slot>> inFront = new ArrayList<>();
                for (int s = 0; s < seats; s++) inFront.add(new ArrayList<>(s < packs.get(r).size() ? packs.get(r).get(s) : List.of()));
                int dir = r % 2 == 0 ? 1 : -1; // left, right, left …
                int pick = 0;
                while (inFront.stream().anyMatch(p -> !p.isEmpty())) {
                    pick++;
                    for (int s = 0; s < seats; s++) {
                        List<Slot> pack = inFront.get(s);
                        if (pack.isEmpty()) continue;
                        Slot chosen = s == human ? askHuman(r, rounds, pick, pack, picks.get(s)) : aiPick(pack, picks.get(s), colors.get(s));
                        if (chosen == null) {
                            Bridge.log("draft " + id + " aborted at round " + (r + 1) + " pick " + pick);
                            Bridge.send(Bridge.msg("draftdone", "id", id, "aborted", true));
                            return;
                        }
                        pack.remove(chosen);
                        picks.get(s).add(chosen);
                        if (s != human) addColors(colors.get(s), chosen.card()); // the AI's colour commitment
                    }
                    List<List<Slot>> next = new ArrayList<>();
                    for (int s = 0; s < seats; s++) next.add(null);
                    for (int s = 0; s < seats; s++) next.set(Math.floorMod(s + dir, seats), inFront.get(s));
                    inFront = next;
                }
            }

            List<Object> out = new ArrayList<>();
            for (int s = 0; s < seats; s++) {
                Map<String, Object> seat = new LinkedHashMap<>();
                List<Object> refs = new ArrayList<>();
                List<PaperCard> cards = new ArrayList<>();
                for (Slot sl : picks.get(s)) { refs.add(sl.ref()); cards.add(sl.card()); }
                seat.put("picks", refs);
                {
                    // AI seats: their deck. Human seat: Forge's suggestion, the starting point of the player's own build.
                    DeckColors cols = colors.get(s);
                    if (s == human) for (PaperCard pc : cards) addColors(cols, pc);
                    Deck deck = buildDeck(cards, cols, landSet, lines, s);
                    putDeck(seat, deck, s == human);
                    seat.put("colors", colors.get(s).getChosenColors().toString());
                }
                out.add(seat);
            }
            Bridge.log("draft " + id + " done: " + seats + " seats, " + rounds + " rounds" + (lines.isEmpty() ? "" : "; " + lines));
            Bridge.send(Bridge.msg("draftdone", "id", id, "seats", out, "lines", lines));
        } catch (Throwable t) {
            Bridge.log("draft " + id + " failed: " + t);
            t.printStackTrace();
            Bridge.send(Bridge.msg("draftdone", "id", id, "aborted", true, "error", String.valueOf(t.getMessage())));
        }
    }

    /**
     * DeckColors.addColorsOf works on its lazily built colour set and clears it again, so read the colours before every add
     * (LimitedPlayerAI.chooseCard always reads them first). A fresh or just-updated one would throw a NullPointerException.
     */
    private static void addColors(DeckColors cols, PaperCard pc) {
        cols.getChosenColors();
        cols.addColorsOf(pc);
    }

    /** {@code deck} (AI) or {@code suggested} (human) rows {n, name, set} + {@code rating} = mean Forge rating of the non-lands. */
    private static void putDeck(Map<String, Object> seat, Deck deck, boolean human) {
        List<Object> list = new ArrayList<>();
        double sum = 0;
        int spells = 0;
        for (Map.Entry<PaperCard, Integer> e : deck.getMain()) {
            PaperCard pc = e.getKey();
            Map<String, Object> row = new LinkedHashMap<>();
            row.put("n", e.getValue());
            row.put("name", pc.getName());
            row.put("set", pc.getEdition());
            list.add(row);
            if (!pc.getRules().getType().isLand()) {
                sum += score(pc) * e.getValue();
                spells += e.getValue();
            }
        }
        seat.put(human ? "suggested" : "deck", list);
        seat.put("rating", spells == 0 ? 0.0 : sum / spells);
    }

    /**
     * Sealed: every seat builds from its own opened packs, no picking. {@code sealed{id, humanSeat, landSet,
     * seats[[{ref, name, set}]]}} → {@code draftdone{id, seats[{picks[ref] (the whole pool), deck | suggested, rating, colors}]}}.
     * Decks = Forge's SealedDeckBuilder (it picks the colours itself), as for the Sealed AI deck style.
     */
    static void startSealed(Map<String, Object> m) {
        Thread t = new Thread(() -> runSealed(m), "tcgcc-sealed");
        t.setDaemon(true);
        t.start();
    }

    @SuppressWarnings("unchecked")
    private static void runSealed(Map<String, Object> m) {
        int id = Json.i(m, "id", -1);
        List<String> lines = new ArrayList<>();
        try {
            int human = Json.i(m, "humanSeat", -1);
            String landSet = Json.s(m, "landSet");
            CardDb db = FModel.getMagicDb().getCommonCards();
            List<Object> out = new ArrayList<>();
            int s = 0;
            for (Object so : (List<Object>) m.get("seats")) {
                List<Object> refs = new ArrayList<>();
                List<PaperCard> pool = new ArrayList<>();
                for (Object co : (List<Object>) so) {
                    Map<String, Object> c = (Map<String, Object>) co;
                    String name = Json.s(c, "name"), set = Json.s(c, "set");
                    PaperCard pc = set == null || set.isEmpty() ? null : db.getCard(name, set);
                    if (pc == null) pc = db.getCard(name);
                    if (pc == null) { lines.add("Forge doesn't know " + name); continue; }
                    refs.add(Json.i(c, "ref", -1));
                    pool.add(pc);
                }
                Map<String, Object> seat = new LinkedHashMap<>();
                seat.put("picks", refs);
                Deck deck;
                String colors = "";
                try {
                    SealedDeckBuilder b = new SealedDeckBuilder(pool);
                    deck = landSet != null && !landSet.isEmpty() && FModel.getMagicDb().getEditions().get(landSet) != null
                            ? b.buildDeck(landSet) : b.buildDeck();
                    colors = String.valueOf(b.getColors());
                } catch (Throwable e) {
                    lines.add("seat " + s + ": Forge's sealed builder failed (" + e.getMessage() + "), playing the whole pool");
                    deck = new Deck("Sealed " + s);
                    for (PaperCard pc : pool) deck.getMain().add(pc);
                }
                putDeck(seat, deck, s == human);
                seat.put("colors", colors);
                out.add(seat);
                s++;
            }
            Bridge.log("sealed " + id + " done: " + s + " seats" + (lines.isEmpty() ? "" : "; " + lines));
            Bridge.send(Bridge.msg("draftdone", "id", id, "seats", out, "lines", lines));
        } catch (Throwable t) {
            Bridge.log("sealed " + id + " failed: " + t);
            t.printStackTrace();
            Bridge.send(Bridge.msg("draftdone", "id", id, "aborted", true, "error", String.valueOf(t.getMessage())));
        }
    }

    /** Forge's draft AI pick (LimitedPlayerAI.chooseCard): best card for this seat's picks and colours. */
    private static Slot aiPick(List<Slot> pack, List<Slot> mine, DeckColors cols) {
        List<PaperCard> packCards = new ArrayList<>(), mineCards = new ArrayList<>();
        for (Slot s : pack) packCards.add(s.card());
        for (Slot s : mine) mineCards.add(s.card());
        List<PaperCard> ranked;
        try {
            ranked = CardRanker.rankCardsInPack(packCards, mineCards, cols.getChosenColors(), cols.canChoseMoreColors());
        } catch (Throwable t) {
            Bridge.log("draft: ranking failed (" + t + "), taking the first card");
            ranked = List.of();
        }
        if (!ranked.isEmpty()) {
            PaperCard best = ranked.get(0);
            for (Slot s : pack) if (s.card() == best) return s;
            for (Slot s : pack) if (s.card().equals(best)) return s;
        }
        return pack.get(0);
    }

    private static Slot askHuman(int round, int rounds, int pick, List<Slot> pack, List<Slot> mine) {
        while (true) {
            List<Object> opts = new ArrayList<>();
            for (Slot s : pack) {
                Map<String, Object> o = new LinkedHashMap<>();
                o.put("ref", s.ref());
                o.put("name", s.card().getName());
                o.put("set", s.card().getEdition());
                opts.add(o);
            }
            List<Object> mineRefs = new ArrayList<>();
            for (Slot s : mine) mineRefs.add(s.ref());
            Map<String, Object> q = new LinkedHashMap<>();
            q.put("kind", "draftpick");
            q.put("round", round + 1);
            q.put("rounds", rounds);
            q.put("pick", pick);
            q.put("pack", opts);
            q.put("picks", mineRefs);
            Map<String, Object> a = Bridge.ask(q, false);
            if (a == null) return null;
            List<Integer> picked = Json.ints(a, "picked");
            if (!picked.isEmpty() && picked.get(0) >= 0 && picked.get(0) < pack.size()) return pack.get(picked.get(0));
            Bridge.log("draft: bad pick answer " + a + ", asking again");
        }
    }

    /** Forge's AI deck from one seat's picks (LimitedPlayerAI.buildDeck); basics from {@code landSet} when Forge knows it. */
    private static Deck buildDeck(List<PaperCard> cards, DeckColors cols, String landSet, List<String> lines, int seat) {
        try {
            if (landSet != null && !landSet.isEmpty() && FModel.getMagicDb().getEditions().get(landSet) != null)
                return new BoosterDeckBuilder(cards, cols).buildDeck(landSet);
            return new BoosterDeckBuilder(cards, cols).buildDeck();
        } catch (Throwable t) {
            lines.add("seat " + seat + ": Forge's deck builder failed (" + t.getMessage() + "), playing all picks");
            Deck d = new Deck("Draft " + seat);
            CardPool main = d.getMain();
            for (PaperCard pc : cards) main.add(pc);
            return d;
        }
    }

    private static double score(PaperCard pc) {
        try {
            return CardRanker.getRawScore(pc);
        } catch (Throwable t) {
            return 0;
        }
    }
}
