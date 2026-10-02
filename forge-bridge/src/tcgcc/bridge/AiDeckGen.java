package tcgcc.bridge;

import forge.card.CardDb;
import forge.card.CardRules;
import forge.card.ColorSet;
import forge.card.MagicColor;
import forge.deck.CardPool;
import forge.deck.Deck;
import forge.deck.DeckFormat;
import forge.deck.generation.DeckGenerator2Color;
import forge.deck.generation.DeckGenerator3Color;
import forge.deck.generation.DeckGenerator4Color;
import forge.deck.generation.DeckGenerator5Color;
import forge.deck.generation.DeckGeneratorBase;
import forge.deck.generation.DeckGeneratorMonoColor;
import forge.gamemodes.limited.CardRanker;
import forge.gamemodes.limited.SealedDeckBuilder;
import forge.item.PaperCard;
import forge.item.SealedTemplate;
import forge.item.generation.UnOpenedProduct;
import forge.model.FModel;
import forge.util.storage.IStorage;

import java.util.ArrayList;
import java.util.Comparator;
import java.util.HashSet;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Random;
import java.util.Set;
import java.util.TreeMap;
import java.util.function.Predicate;

/**
 * Builds the AI's deck with Forge's own generators, limited to the cards of the chosen sets (the game sends their names):
 * "random" = a constructed deck of {@code size} cards in 1-5 colours (DeckGenerator*Color), "sealed" = {@code boosters} boosters
 * of those sets opened and built into a 40-card deck by Forge's sealed AI (SealedDeckBuilder, draft rankings). {@code power}
 * (-1..1) uses Forge's card ratings (CardRanker): below 0 the best-rated cards are left out, above 0 only the better ones are
 * used (Random) or more boosters are opened (Sealed). Anything that fails returns the game's own deck.
 *
 * <p>Forge's generators log nothing usable (DeckGeneratorBase.trace is a no-op DebugTrace, LimitedDeckBuilder.logToConsole is
 * false), so the lines for the player are written here. They never name cards (that would reveal the AI's deck); the full
 * list goes to the bridge log.
 */
final class AiDeckGen {
    private AiDeckGen() {}

    static final String DECK_NAME = "TCGCC Opponent";
    private static final String[] COLORS = { MagicColor.Constant.WHITE, MagicColor.Constant.BLUE, MagicColor.Constant.BLACK,
            MagicColor.Constant.RED, MagicColor.Constant.GREEN };
    /** Last deck built (the same customer keeps it on a rematch: spec {@code reuse}). */
    private static Result last;
    /** Default weights for 1..5 colours (mostly two) when the game sends none. */
    private static final int[] COUNT_WEIGHTS = { 15, 45, 25, 10, 5 };
    private static final Random rng = new Random();

    static final class Result {
        Deck deck;
        String source = "fallback";
        String colors = "";
        final List<String> lines = new ArrayList<>();
        /** The deck for the after-match reveal: {n, name, set, kind: creature|spell|land}. */
        final List<Map<String, Object>> list = new ArrayList<>();
    }

    private static double num(Map<String, Object> m, String k, double def) {
        return m.get(k) instanceof Number n ? n.doubleValue() : def;
    }

    @SuppressWarnings("unchecked")
    static Result build(Map<String, Object> spec, Deck fallback) {
        if (Json.b(spec, "reuse") && last != null) {
            Bridge.log("AI deck: rematch, same deck as last game");
            Result again = new Result();
            again.deck = last.deck;
            again.source = last.source;
            again.colors = last.colors;
            again.lines.add("Same customer, same deck");
            again.lines.addAll(last.lines);
            again.list.addAll(last.list);
            return again;
        }
        Result r = new Result();
        String style = "sealed".equalsIgnoreCase(Json.s(spec, "style")) ? "sealed" : "random";
        int size = Math.max(40, Math.min(100, Json.i(spec, "size", 60)));
        int boosters = Math.max(1, Math.min(24, Json.i(spec, "boosters", 6)));
        double power = Math.max(-1, Math.min(1, num(spec, "power", 0)));
        try {
            Map<String, List<PaperCard>> bySet = resolve(spec.get("pools") instanceof List<?> l ? (List<Object>) l : List.of());
            List<PaperCard> pool = new ArrayList<>();
            bySet.values().forEach(pool::addAll);
            Bridge.log("AI deck (" + style + ", power " + power + "): " + pool.size() + " cards known to Forge, sets " + bySet.keySet());
            Map<String, Object> colors = spec.get("colors") instanceof Map<?, ?> c ? (Map<String, Object>) c : Map.of();
            Deck d;
            if (style.equals("sealed")) {
                int n = power > 0 ? (int) Math.round(boosters * (1 + power * 0.5)) : boosters;
                d = sealed(bySet, n, power < 0 ? -power : 0, r);
            } else {
                List<String> sets = new ArrayList<>(bySet.keySet());
                d = power == 0 ? null : random(byPower(pool, power, r), sets, colors, size, r);
                if (d == null) {
                    if (power != 0) {
                        Bridge.log("AI deck: not enough cards at power " + power + ", using every card");
                        r.lines.clear();
                    }
                    d = random(pool, sets, colors, size, r);
                }
            }
            if (d != null) {
                r.deck = d;
                r.source = "forge";
            }
        } catch (Throwable t) {
            Bridge.log("AI deck generation failed: " + t);
            t.printStackTrace();
        }
        if (r.deck == null) {
            r.deck = fallback;
            r.lines.clear();
            r.lines.add("Forge couldn't build a deck from these sets - using a simple random deck");
        }
        for (Map.Entry<PaperCard, Integer> e : r.deck.getMain()) {
            var t = e.getKey().getRules().getType();
            Map<String, Object> row = new LinkedHashMap<>();
            row.put("n", e.getValue());
            row.put("name", e.getKey().getName());
            row.put("set", e.getKey().getEdition());
            row.put("kind", t.isLand() ? "land" : t.isCreature() ? "creature" : "spell");
            r.list.add(row);
        }
        logDeck(r.deck);
        last = r;
        return r;
    }

    // --- Power: Forge's card ratings -------------------------------------------------------------------------------------

    /** Forge's rating of a card (draft rankings; higher = better). */
    private static double score(PaperCard pc) {
        try {
            return CardRanker.getRawScore(pc);
        } catch (Throwable t) {
            return 0;
        }
    }

    /** Non-land cards, best-rated first. */
    private static List<PaperCard> ranked(List<PaperCard> cards) {
        List<PaperCard> spells = new ArrayList<>();
        for (PaperCard pc : cards) if (!pc.getRules().getType().isLand()) spells.add(pc);
        spells.sort(Comparator.comparingDouble(AiDeckGen::score).reversed());
        return spells;
    }

    /**
     * The pool for a Random deck at {@code power}: below 0 the best-rated |power|*40 % of the spells are left out, above 0 only
     * the best (1 - power*0.5) share is kept. Lands always stay.
     */
    private static List<PaperCard> byPower(List<PaperCard> pool, double power, Result r) {
        List<PaperCard> spells = ranked(pool);
        if (spells.isEmpty()) return pool;
        Bridge.log("AI deck: best rated " + names(spells.subList(0, Math.min(3, spells.size())))
                + ", worst " + names(spells.subList(Math.max(0, spells.size() - 3), spells.size())));
        int n = spells.size();
        List<PaperCard> keep = power < 0
                ? spells.subList((int) Math.round(n * -power * 0.4), n)
                : spells.subList(0, Math.max(1, (int) Math.round(n * (1 - power * 0.5))));
        Set<PaperCard> kept = new HashSet<>(keep);
        List<PaperCard> out = new ArrayList<>();
        for (PaperCard pc : pool) if (pc.getRules().getType().isLand() || kept.contains(pc)) out.add(pc);
        r.lines.add(power < 0 ? "Weaker deck: the best-rated " + (n - keep.size()) + " cards are left out"
                : "Stronger deck: only the best-rated " + keep.size() + " of " + n + " cards");
        return out;
    }

    private static String names(List<PaperCard> cards) {
        List<String> s = new ArrayList<>();
        for (PaperCard pc : cards) s.add(pc.getName());
        return s.toString();
    }

    /**
     * Per set code (the game's {@code pools[{set, names}]}) its cards, printed in that set when Forge has the printing, else any
     * printing (logged). Unknown names are skipped.
     */
    @SuppressWarnings("unchecked")
    private static Map<String, List<PaperCard>> resolve(List<Object> pools) {
        CardDb db = FModel.getMagicDb().getCommonCards();
        Map<String, List<PaperCard>> bySet = new LinkedHashMap<>();
        for (Object o : pools) {
            if (!(o instanceof Map<?, ?> m)) continue;
            String code = Json.s((Map<String, Object>) m, "set");
            List<String> names = m.get("names") instanceof List<?> l ? (List<String>) l : List.of();
            List<PaperCard> cards = new ArrayList<>();
            Set<String> seen = new HashSet<>();
            List<String> otherPrinting = new ArrayList<>(), unknown = new ArrayList<>();
            for (String name : names) {
                if (name == null || !seen.add(name.toLowerCase())) continue;
                PaperCard pc = code == null || code.isEmpty() ? null : db.getCard(name, code);
                if (pc == null) {
                    pc = db.getCard(name);
                    if (pc != null) otherPrinting.add(name);
                }
                if (pc == null) unknown.add(name);
                else cards.add(pc);
            }
            if (!otherPrinting.isEmpty()) Bridge.log("AI deck: not printed in " + code + " for Forge, using another printing: " + otherPrinting);
            if (!unknown.isEmpty()) Bridge.log("AI deck: Forge doesn't know " + unknown);
            if (!cards.isEmpty()) bySet.computeIfAbsent(code == null || code.isEmpty() ? "?" : code, k -> new ArrayList<>()).addAll(cards);
        }
        return bySet;
    }

    // --- Random: Forge's constructed generator -------------------------------------------------------------------------

    /**
     * {@code colors} = {@code {min, max, weights[5]}}: the colour count is rolled between min and max by the weights for 1..5
     * colours (all 0 in range = equal), capped at the colours the pool has.
     */
    private static Deck random(List<PaperCard> pool, List<String> sets, Map<String, Object> colors, int size, Result r) {
        Set<String> names = new HashSet<>();
        int[] weight = new int[5];
        for (PaperCard pc : pool) {
            names.add(pc.getName());
            CardRules rules = pc.getRules();
            if (rules.getType().isLand()) continue;
            ColorSet c = rules.getColor();
            boolean[] has = { c.hasWhite(), c.hasBlue(), c.hasBlack(), c.hasRed(), c.hasGreen() };
            for (int i = 0; i < 5; i++) if (has[i]) weight[i]++;
        }
        int available = 0;
        for (int w : weight) if (w > 0) available++;
        if (available == 0) return null;
        Predicate<PaperCard> inSets = pc -> names.contains(pc.getName());
        CardDb db = FModel.getMagicDb().getCommonCards();

        int min = Math.max(1, Math.min(5, Json.i(colors, "min", 1)));
        int max = Math.max(min, Math.min(5, Json.i(colors, "max", 5)));
        int[] w = COUNT_WEIGHTS.clone();
        if (colors.get("weights") instanceof List<?> l)
            for (int i = 0; i < 5 && i < l.size(); i++) w[i] = l.get(i) instanceof Number n ? Math.max(0, n.intValue()) : w[i];
        Bridge.log("AI deck: colours " + min + "-" + max + ", weights " + java.util.Arrays.toString(w) + ", pool has " + available);

        for (int attempt = 0; attempt < 4; attempt++) {
            // New roll every try; a single allowed count gets two tries, then one colour fewer.
            int wanted = rollCount(min, max, w);
            int count = min == max && attempt >= 2 ? Math.max(1, wanted - 1) : wanted;
            count = Math.min(count, available);
            List<Integer> picked = count == 5 ? List.of(0, 1, 2, 3, 4) : pickColors(weight, count);
            if (picked.isEmpty()) return null;
            DeckGeneratorBase gen = generator(db, inSets, picked);
            gen.setSingleton(false);
            CardPool cards = gen.getDeck(size, true);
            int total = cards.countAll();
            int lands = cards.countAll(pc -> pc.getRules().getType().isLand());
            Bridge.log("AI deck: random try " + (attempt + 1) + " " + colorNames(picked) + " -> " + total + " cards, " + lands + " lands");
            if (total < size - 5 || total - lands < size / 3) continue;

            Deck d = new Deck(DECK_NAME);
            d.getMain().addAll(cards);
            r.colors = colorNames(picked);
            int creatures = cards.countAll(pc -> pc.getRules().getType().isCreature());
            if (wanted > picked.size())
                r.lines.add("Not enough cards for " + wanted + " colours - using " + picked.size());
            r.lines.add("Customer builds " + deckKind(picked) + " from " + String.join(", ", sets));
            r.lines.add((total - lands) + " spells (" + creatures + " creatures), " + lands + " lands");
            r.lines.add("Mana curve: " + curve(cards));
            return d;
        }
        return null;
    }

    /** A colour count in [min, max], weighted by {@code w[count - 1]}; all weights 0 in range = equal chances. */
    private static int rollCount(int min, int max, int[] w) {
        int sum = 0;
        for (int c = min; c <= max; c++) sum += w[c - 1];
        if (sum <= 0) return min + rng.nextInt(max - min + 1);
        int roll = rng.nextInt(sum);
        for (int c = min; c <= max; c++) {
            roll -= w[c - 1];
            if (roll < 0) return c;
        }
        return max;
    }

    /** Forge's generator for that many colours, limited to the player's sets. */
    private static DeckGeneratorBase generator(CardDb db, Predicate<PaperCard> inSets, List<Integer> c) {
        DeckFormat f = DeckFormat.Constructed;
        return switch (c.size()) {
            case 1 -> new DeckGeneratorMonoColor(db, f, inSets, COLORS[c.get(0)]);
            case 2 -> new DeckGenerator2Color(db, f, inSets, COLORS[c.get(0)], COLORS[c.get(1)]);
            case 3 -> new DeckGenerator3Color(db, f, inSets, COLORS[c.get(0)], COLORS[c.get(1)], COLORS[c.get(2)]);
            case 4 -> new DeckGenerator4Color(db, f, inSets, COLORS[c.get(0)], COLORS[c.get(1)], COLORS[c.get(2)], COLORS[c.get(3)]);
            default -> new DeckGenerator5Color(db, f, inSets);
        };
    }

    /** "a mono-Green deck", "a White-Blue-Red deck", "a five-colour deck". */
    private static String deckKind(List<Integer> picked) {
        if (picked.size() >= 5) return "a five-colour deck";
        if (picked.size() == 1) return "a mono-" + colorNames(picked) + " deck";
        return "a " + colorNames(picked) + " deck";
    }

    /** Up to {@code n} different colours, weighted by how many spells use them. */
    private static List<Integer> pickColors(int[] weight, int n) {
        List<Integer> picked = new ArrayList<>();
        for (int k = 0; k < n; k++) {
            int sum = 0;
            for (int i = 0; i < 5; i++) if (!picked.contains(i)) sum += weight[i];
            if (sum <= 0) break;
            int roll = rng.nextInt(sum);
            for (int i = 0; i < 5; i++) {
                if (picked.contains(i)) continue;
                roll -= weight[i];
                if (roll < 0) { picked.add(i); break; }
            }
        }
        picked.sort(null); // WUBRG order for the name ("Blue-Red", not "Red-Blue")
        return picked;
    }

    // --- Sealed: Forge's boosters + sealed AI ----------------------------------------------------------------------------

    /** Lets us read how many cards the sealed AI could use (protected in LimitedDeckBuilder). */
    private static final class Sealed extends SealedDeckBuilder {
        Sealed(List<PaperCard> list) { super(list); }
        int playables() { return aiPlayables.size(); }
    }

    /**
     * {@code boosters} boosters rotating over the sets, each from that set's cards only (custom sets can be trimmed).
     * {@code weaken} (0..1): the best-rated weaken*40 % of the opened spells are taken out before building.
     */
    private static Deck sealed(Map<String, List<PaperCard>> byEdition, int boosters, double weaken, Result r) {
        if (byEdition.isEmpty()) return null;
        IStorage<SealedTemplate> templates = FModel.getMagicDb().getBoosters();

        for (int attempt = 0; attempt < 2; attempt++) {
            List<PaperCard> opened = new ArrayList<>();
            Map<String, Integer> perEdition = new TreeMap<>();
            List<String> editions = new ArrayList<>(byEdition.keySet());
            for (int b = 0; b < boosters; b++) {
                String ed = editions.get(b % editions.size());
                SealedTemplate tpl = templates.contains(ed) ? templates.get(ed) : SealedTemplate.genericDraftBooster;
                List<PaperCard> booster;
                try {
                    booster = new UnOpenedProduct(tpl, byEdition.get(ed)).get();
                } catch (Throwable t) { // template slots the trimmed pool can't fill: plain booster
                    Bridge.log("AI deck: " + ed + " booster template failed (" + t + "), using a generic booster");
                    booster = new UnOpenedProduct(SealedTemplate.genericDraftBooster, byEdition.get(ed)).get();
                }
                opened.addAll(booster);
                perEdition.merge(ed, 1, Integer::sum);
            }
            int openedCount = opened.size();
            int removed = 0;
            if (weaken > 0) {
                List<PaperCard> best = ranked(opened);
                removed = (int) Math.round(best.size() * weaken * 0.4);
                for (PaperCard pc : best.subList(0, removed)) opened.remove(pc);
            }
            Sealed builder = new Sealed(opened);
            int playables = builder.playables();
            Deck d = builder.buildDeck();
            if (d == null) continue;
            CardPool main = d.getMain();
            int total = main.countAll();
            int lands = main.countAll(pc -> pc.getRules().getType().isLand());
            Bridge.log("AI deck: sealed try " + (attempt + 1) + " from " + openedCount + " cards -> " + total + " cards, " + lands + " lands");
            if (total < 40 || total - lands < 15) continue;

            d.setName(DECK_NAME);
            r.colors = colorNames(builder.getColors());
            List<String> parts = new ArrayList<>();
            perEdition.forEach((ed, n) -> parts.add(n + " " + ed));
            int spells = total - lands;
            r.lines.add("Customer opens " + boosters + " boosters (" + String.join(", ", parts) + ") - " + openedCount + " cards");
            if (removed > 0) r.lines.add("Weaker deck: the best-rated " + removed + " cards are left out");
            r.lines.add("Picks " + r.colors + ": " + spells + " spells, " + lands + " lands");
            int notPlayable = Math.max(0, openedCount - removed - playables);
            r.lines.add("Left out " + Math.max(0, openedCount - spells) + " cards"
                    + (notPlayable > 0 ? " (" + notPlayable + " not playable by the AI)" : ""));
            return d;
        }
        return null;
    }

    // --- Text ----------------------------------------------------------------------------------------------------------

    private static String colorNames(List<Integer> picked) {
        List<String> s = new ArrayList<>();
        for (int i : picked) s.add(cap(COLORS[i]));
        return String.join("-", s);
    }

    private static String colorNames(ColorSet c) {
        if (c == null) return "Colorless";
        List<Integer> picked = new ArrayList<>();
        boolean[] has = { c.hasWhite(), c.hasBlue(), c.hasBlack(), c.hasRed(), c.hasGreen() };
        for (int i = 0; i < 5; i++) if (has[i]) picked.add(i);
        return picked.isEmpty() ? "Colorless" : colorNames(picked);
    }

    private static String cap(String s) { return s.isEmpty() ? s : Character.toUpperCase(s.charAt(0)) + s.substring(1); }

    /** "1:4 2:9 3:8 4:6 5+:5" over the non-land cards (0-drops count as 1). */
    private static String curve(CardPool cards) {
        int[] n = new int[6];
        for (Map.Entry<PaperCard, Integer> e : cards) {
            CardRules rules = e.getKey().getRules();
            if (rules.getType().isLand()) continue;
            int cmc = Math.max(1, Math.min(5, rules.getManaCost().getCMC()));
            n[cmc] += e.getValue();
        }
        return "1:" + n[1] + " 2:" + n[2] + " 3:" + n[3] + " 4:" + n[4] + " 5+:" + n[5];
    }

    private static void logDeck(Deck d) {
        if (d == null) return;
        StringBuilder sb = new StringBuilder("AI deck list (" + d.getMain().countAll() + "):");
        for (Map.Entry<PaperCard, Integer> e : d.getMain()) sb.append("\n  ").append(e.getValue()).append(' ').append(e.getKey().getName())
                .append('|').append(e.getKey().getEdition());
        Bridge.log(sb.toString());
    }
}
