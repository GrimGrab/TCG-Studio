package tcgcc.bridge;

import forge.card.MagicColor;
import forge.game.GameEntityView;
import forge.game.GameLogEntry;
import forge.game.GameView;
import forge.game.card.CardView;
import forge.game.card.CounterType;
import forge.game.combat.CombatView;
import forge.game.player.PlayerView;
import forge.game.spellability.SpellAbilityView;
import forge.game.spellability.StackItemView;
import forge.game.zone.ZoneType;

import java.util.ArrayList;
import java.util.HashMap;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.regex.Matcher;
import java.util.regex.Pattern;

/** GameView → plain maps for JSON. Only what the viewing (human) player may see. */
final class StateWriter {
    private StateWriter() {}

    private static final ZoneType[] PUBLIC_ZONES = { ZoneType.Battlefield, ZoneType.Graveyard, ZoneType.Exile, ZoneType.Command };

    static Map<String, Object> write(GameView gv, PlayerView me) {
        Map<String, Object> s = new LinkedHashMap<>();
        s.put("turn", gv.getTurn());
        s.put("phase", gv.getPhase() == null ? "" : gv.getPhase().name());
        s.put("activePlayer", gv.getPlayerTurn() == null ? -1 : gv.getPlayerTurn().getId());
        s.put("me", me == null ? -1 : me.getId());
        s.put("gameOver", gv.isGameOver());

        List<Object> players = new ArrayList<>();
        for (PlayerView p : gv.getPlayers()) {
            Map<String, Object> pm = new LinkedHashMap<>();
            pm.put("id", p.getId());
            pm.put("name", p.getName());
            pm.put("ai", p.isAI());
            pm.put("life", p.getLife());
            pm.put("priority", p.getHasPriority());
            pm.put("lost", p.getHasLost());
            pm.put("library", p.getZoneSize(ZoneType.Library));
            pm.put("handSize", p.getZoneSize(ZoneType.Hand));
            pm.put("landsPlayed", p.getNumLandThisTurn());
            Map<String, Object> mana = new LinkedHashMap<>();
            putMana(mana, "W", p.getMana(MagicColor.WHITE));
            putMana(mana, "U", p.getMana(MagicColor.BLUE));
            putMana(mana, "B", p.getMana(MagicColor.BLACK));
            putMana(mana, "R", p.getMana(MagicColor.RED));
            putMana(mana, "G", p.getMana(MagicColor.GREEN));
            putMana(mana, "C", p.getMana((byte) MagicColor.COLORLESS));
            pm.put("mana", mana);
            if (me != null && p.getId() == me.getId()) pm.put("hand", cards(p.getHand()));
            for (ZoneType z : PUBLIC_ZONES) pm.put(z.name().toLowerCase(), cards(p.getCards(z)));
            players.add(pm);
        }
        s.put("players", players);

        List<Object> stack = new ArrayList<>();
        for (StackItemView si : gv.getStack()) {
            Map<String, Object> e = new LinkedHashMap<>();
            e.put("text", si.getText());
            e.put("card", si.getSourceCard() == null ? null : card(si.getSourceCard()));
            e.put("player", si.getActivatingPlayer() == null ? -1 : si.getActivatingPlayer().getId());
            List<Integer> targets = new ArrayList<>();
            if (si.getTargetCards() != null) for (CardView c : si.getTargetCards()) targets.add(c.getId());
            e.put("targetCards", targets);
            List<Integer> tp = new ArrayList<>();
            if (si.getTargetPlayers() != null) for (PlayerView p : si.getTargetPlayers()) tp.add(p.getId());
            e.put("targetPlayers", tp);
            stack.add(e);
        }
        s.put("stack", stack);

        List<Object> combat = new ArrayList<>();
        CombatView cv = gv.getCombat();
        if (cv != null) {
            for (CardView a : cv.getAttackers()) {
                Map<String, Object> e = new LinkedHashMap<>();
                e.put("attacker", a.getId());
                GameEntityView d = cv.getDefender(a);
                e.put("defender", d == null ? null : d.getId());
                e.put("defenderIsPlayer", d instanceof PlayerView);
                List<Integer> bl = new ArrayList<>();
                var blockers = cv.getBlockers(a);
                if (blockers != null) for (CardView b : blockers) bl.add(b.getId());
                e.put("blockers", bl);
                combat.add(e);
            }
        }
        s.put("combat", combat);

        List<String> log = new ArrayList<>();
        Map<String, Object> logCards = new LinkedHashMap<>(); // id -> card, for the cards the log lines name (hover previews)
        if (gv.getGameLog() != null) {
            List<GameLogEntry> all = gv.getGameLog().getAllEntries(); // oldest first: send the newest 30, in order
            Map<Integer, CardView> byId = new HashMap<>();
            for (CardView c : allCards(gv)) byId.put(c.getId(), c);
            for (int i = Math.max(0, all.size() - 30); i < all.size(); i++) {
                GameLogEntry e = all.get(i);
                log.add(e.message());
                if (e.sourceCard() != null) {
                    byId.putIfAbsent(e.sourceCard().getId(), e.sourceCard()); // tokens that are gone
                    // Most lines name their card without an id ("X cast Y"): the client matches the source card by name.
                    CardView src = byId.get(e.sourceCard().getId());
                    if (canShow(src, me)) logCards.put(String.valueOf(src.getId()), card(src));
                }
                Matcher m = CARD_REF.matcher(e.message());
                while (m.find()) {
                    CardView c = byId.get(Integer.parseInt(m.group(1)));
                    if (c != null && namedBefore(e.message(), m.start(), c) && canShow(c, me)) logCards.put(String.valueOf(c.getId()), card(c));
                }
            }
        }
        s.put("log", log);
        s.put("logCards", logCards);
        return s;
    }

    /** Forge writes a card into log text as CardView.toString() = "Name (id)". */
    private static final Pattern CARD_REF = Pattern.compile(" \\((\\d+)\\)");

    /** True when the text right before a " (id)" reference is that card's name (so "(3)" in other text isn't taken for a card). */
    private static boolean namedBefore(String msg, int refStart, CardView c) {
        String before = msg.substring(0, refStart);
        CardView.CardStateView st = c.getCurrentState();
        return (c.getName() != null && !c.getName().isEmpty() && before.endsWith(c.getName()))
            || (st != null && st.getName() != null && !st.getName().isEmpty() && before.endsWith(st.getName()));
    }

    /** Log previews never show a hidden face: nothing now in a library, no face-down card the viewer doesn't control. */
    private static boolean canShow(CardView c, PlayerView me) {
        if (c.getZone() == ZoneType.Library) return false;
        if (c.isFaceDown()) return me != null && c.getController() != null && c.getController().getId() == me.getId();
        return true;
    }

    private static void putMana(Map<String, Object> m, String k, int v) { if (v > 0) m.put(k, v); }

    private static List<Object> cards(Iterable<CardView> cs) {
        List<Object> r = new ArrayList<>();
        if (cs != null) for (CardView c : cs) r.add(card(c));
        return r;
    }

    static Map<String, Object> card(CardView c) {
        Map<String, Object> m = new LinkedHashMap<>();
        m.put("id", c.getId());
        CardView.CardStateView st = c.getCurrentState();
        m.put("name", st == null ? c.getName() : st.getName());
        m.put("oracleName", c.getOracleName());
        if (st != null) {
            m.put("set", st.getSetCode());
            m.put("type", st.getType() == null ? "" : st.getType().toString());
            m.put("cost", st.getManaCost() == null ? "" : st.getManaCost().toString());
            m.put("text", st.getOracleText());
            if (st.isCreature()) {
                m.put("power", st.getPower());
                m.put("toughness", st.getToughness());
            }
            if (st.isPlaneswalker()) m.put("loyalty", st.getLoyalty());
            m.put("land", st.isLand());
            m.put("creature", st.isCreature());
        }
        m.put("controller", c.getController() == null ? -1 : c.getController().getId());
        m.put("owner", c.getOwner() == null ? -1 : c.getOwner().getId());
        m.put("zone", c.getZone() == null ? "" : c.getZone().name());
        if (c.isTapped()) m.put("tapped", true);
        if (c.isSick()) m.put("sick", true);
        if (c.isToken()) m.put("token", true);
        if (c.isFaceDown()) m.put("faceDown", true);
        if (c.isAttacking()) m.put("attacking", true);
        if (c.isBlocking()) m.put("blocking", true);
        if (c.getDamage() > 0) m.put("damage", c.getDamage());
        if (c.getAttachedTo() != null) m.put("attachedTo", c.getAttachedTo().getId());
        var counters = c.getCounters();
        if (counters != null && !counters.isEmpty()) {
            Map<String, Object> cm = new LinkedHashMap<>();
            for (CounterType t : counters.elementSet()) cm.put(t.getName(), counters.count(t));
            m.put("counters", cm);
        }
        return m;
    }

    static List<CardView> allCards(GameView gv) {
        List<CardView> r = new ArrayList<>();
        for (PlayerView p : gv.getPlayers())
            for (ZoneType z : ZoneType.values()) {
                var cs = p.getCards(z);
                if (cs != null) for (CardView c : cs) r.add(c);
            }
        for (StackItemView si : gv.getStack()) if (si.getSourceCard() != null) r.add(si.getSourceCard());
        return r;
    }

    static CardView findCard(GameView gv, int id) {
        if (gv == null) return null;
        for (CardView c : allCards(gv)) if (c.getId() == id) return c;
        return null;
    }

    /** The card an option is about (a card, an ability's host card, a stack item's source), or null. */
    static CardView cardViewOf(Object o) {
        if (o instanceof CardView c) return c;
        if (o instanceof SpellAbilityView sa) return sa.getHostCard();
        if (o instanceof StackItemView si) return si.getSourceCard();
        return null;
    }

    static Integer cardId(Object o) {
        if (o instanceof CardView c) return c.getId();
        if (o instanceof SpellAbilityView sa && sa.getHostCard() != null) return sa.getHostCard().getId();
        if (o instanceof StackItemView si && si.getSourceCard() != null) return si.getSourceCard().getId();
        return null;
    }

    static String describe(Object o) {
        if (o == null) return "";
        if (o instanceof CardView c) return c.getName() + " (" + c.getId() + ")";
        if (o instanceof PlayerView p) return p.getName();
        return o.toString();
    }
}
