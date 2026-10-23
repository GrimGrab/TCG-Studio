package tcgcc.bridge;

import forge.card.MagicColor;
import forge.game.Game;
import forge.game.GameEntityView;
import forge.game.card.Card;
import forge.game.combat.CombatUtil;
import forge.game.player.Player;
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

    /** {@code game} (may be null) lets Forge's own rules answer questions the views can't, e.g. who may block whom. */
    static Map<String, Object> write(GameView gv, PlayerView me, Game game) {
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
            boolean mine = me != null && p.getId() == me.getId();
            if (mine) pm.put("hand", cards(p.getHand(), me));
            else pm.put("handShown", cards(shownTo(p.getHand(), me), me)); // opponent's hand cards Forge lets me see (Telepathy)
            // Library cards Forge lets me see (Future Sight / Courser reveal the top, Bolas's Citadel lets me look), top first
            pm.put("libraryShown", cards(shownTo(p.getLibrary(), me), me));
            for (ZoneType z : PUBLIC_ZONES) {
                List<Object> zl = cards(visible(p.getCards(z), z), me);
                if (z == ZoneType.Command) addEffectSources(zl, game);
                pm.put(z.name().toLowerCase(), zl);
            }
            List<CardView> cmds = p.getCommanders();
            if (cmds != null && !cmds.isEmpty()) {
                List<Object> cl = new ArrayList<>();
                for (CardView c : cmds) {
                    Map<String, Object> cm = new LinkedHashMap<>();
                    cm.put("id", c.getId());
                    cm.put("name", c.getName());
                    int cast = p.getCommanderCast(c);
                    cm.put("cast", cast);
                    cm.put("tax", 2 * cast); // CostAdjustment adds {2} per earlier cast from the command zone
                    Map<String, Object> dmg = new LinkedHashMap<>();
                    for (PlayerView o : gv.getPlayers()) if (o.getId() != p.getId()) dmg.put(String.valueOf(o.getId()), o.getCommanderDamage(c));
                    cm.put("damageDealt", dmg);
                    cl.add(cm);
                }
                pm.put("commanders", cl);
            }
            players.add(pm);
        }
        s.put("players", players);
        s.put("usable", usable(me, game));

        List<Object> stack = new ArrayList<>();
        for (StackItemView si : gv.getStack()) {
            Map<String, Object> e = new LinkedHashMap<>();
            e.put("text", si.getText());
            e.put("card", si.getSourceCard() == null ? null : card(si.getSourceCard(), me));
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
                // Attack target from Forge's live combat (the view lags while attackers are declared), else the view's.
                Object[] ld = liveDefender(game, a.getId());
                if (ld != null) {
                    e.put("defender", ld[0]);
                    e.put("defenderIsPlayer", ld[1]);
                } else {
                    GameEntityView d = cv.getDefender(a);
                    e.put("defender", d == null ? null : d.getId());
                    e.put("defenderIsPlayer", d instanceof PlayerView);
                }
                // Blockers from Forge's live combat (updated on every assignment while you declare); the view only catches up
                // once blocks are confirmed, so lines/checks on the table lagged a whole step.
                List<Integer> bl = liveBlockers(game, a.getId());
                if (bl == null) {
                    bl = new ArrayList<>();
                    var blockers = cv.getBlockers(a);
                    if (blockers != null) for (CardView b : blockers) bl.add(b.getId());
                }
                e.put("blockers", bl);
                List<Integer> can = canBeBlockedBy(game, me, a.getId());
                if (can != null) e.put("canBeBlockedBy", can);
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
                    if (canShow(src, me)) logCards.put(String.valueOf(src.getId()), card(src, me));
                }
                Matcher m = CARD_REF.matcher(e.message());
                while (m.find()) {
                    CardView c = byId.get(Integer.parseInt(m.group(1)));
                    if (c != null && namedBefore(e.message(), m.start(), c) && canShow(c, me)) logCards.put(String.valueOf(c.getId()), card(c, me));
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

    /** Log previews never show a hidden face: no library card or face-down card Forge doesn't let the viewer see. */
    private static boolean canShow(CardView c, PlayerView me) {
        if (c.getZone() == ZoneType.Library) return me != null && c.canBeShownTo(me);
        if (c.isFaceDown()) return !hiddenFace(c, me);
        return true;
    }

    /**
     * Cards outside my hand Forge offers me to use: its own "Flashback" zone (graveyard flashback, cast from exile, the
     * top of a library…), which Forge keeps up to date in the views on its game thread. That zone also holds cards I may
     * only look at ({@code PlayerZone} filters: {@code mayPlay || mayPlayerLook || …}); those are left out here: no
     * may-play for me and only seen thanks to a look permission (library, hand, face-down). Only a hint for the table;
     * the click itself still goes to Forge. Views only: running Forge's rules from this thread (getAllPossibleAbilities)
     * raced the game thread and crashed it (ConcurrentModificationException in StaticEffects, headless 2026-10-10).
     */
    private static List<Integer> usable(PlayerView me, Game game) {
        List<Integer> ids = new ArrayList<>();
        if (me == null) return ids;
        for (CardView c : me.getCards(ZoneType.Flashback)) {
            if (c == null) continue;
            boolean lookOnly = c.mayPlayerLook(me)
                && (c.getZone() == ZoneType.Library || c.getZone() == ZoneType.Hand || c.isFaceDown())
                && !mayPlay(game, c, me);
            if (!lookOnly) ids.add(c.getId());
        }
        return ids;
    }

    /**
     * Forge 2.0.15 dropped CardView's may-play players (Card.updateMayPlay set them from the play options that grant zone
     * permissions): read those options from the card itself. A read only, no rules run; if it races the game thread the card
     * just counts as usable (the tint is a hint, the click still goes to Forge).
     */
    private static boolean mayPlay(Game game, CardView v, PlayerView me) {
        if (game == null) return true;
        try {
            Card card = game.findById(v.getId());
            Player p = game.getPlayer(me);
            return card == null || p == null || card.mayPlay(p).stream().anyMatch(o -> o.grantsZonePermissions());
        } catch (RuntimeException e) {
            return true;
        }
    }

    /** The cards of a hidden zone that Forge's own rules let the viewer see ({@code CardView.canBeShownTo}), in zone order. */
    private static List<CardView> shownTo(Iterable<CardView> cards, PlayerView me) {
        List<CardView> out = new ArrayList<>();
        if (me == null || cards == null) return out;
        for (CardView c : cards) if (c != null && c.canBeShownTo(me)) out.add(c);
        return out;
    }

    /**
     * A face-down card whose real face the viewer may not see: Forge's {@code canFaceDownBeShownTo} (your own morphs,
     * cards you may look at such as Gonti's exiled card). Without a viewer: the AI's face-down cards.
     */
    private static boolean hiddenFace(CardView c, PlayerView me) {
        if (!c.isFaceDown()) return false;
        if (me == null) return c.getController() == null || c.getController().isAI();
        return !c.canFaceDownBeShownTo(me);
    }

    /**
     * The viewer's creatures that Forge's rules allow to block this attacker ({@code CombatUtil.canBlock(attacker, blocker)}:
     * evasion, "can't block", tapped, …) — the table only shows Forge's answer. Null when it can't be worked out.
     */
    private static List<Integer> canBeBlockedBy(Game game, PlayerView me, int attackerId) {
        if (game == null || me == null) return null;
        try {
            Card attacker = game.findById(attackerId);
            if (attacker == null) return null;
            for (Player p : game.getPlayers()) {
                if (p.getId() != me.getId()) continue;
                List<Integer> ids = new ArrayList<>();
                for (Card c : p.getCreaturesInPlay()) if (CombatUtil.canBlock(attacker, c)) ids.add(c.getId());
                return ids;
            }
        } catch (Throwable t) {
            Bridge.log("canBeBlockedBy failed: " + t);
        }
        return null;
    }

    /** {id, isPlayer} of what this attacker attacks in Forge's live combat, or null when that isn't available. */
    private static Object[] liveDefender(Game game, int attackerId) {
        try {
            if (game == null) return null;
            var combat = game.getCombat();
            Card attacker = game.findById(attackerId);
            if (combat == null || attacker == null) return null;
            var d = combat.getDefenderByAttacker(attacker);
            if (d == null) return null;
            return new Object[] { d.getId(), d instanceof forge.game.player.Player };
        } catch (Throwable t) {
            Bridge.log("liveDefender failed: " + t);
            return null;
        }
    }

    /** Blockers of this attacker in Forge's live combat, or null when that isn't available (then the view's are used). */
    private static List<Integer> liveBlockers(Game game, int attackerId) {
        if (game == null) return null;
        try {
            var combat = game.getCombat();
            Card attacker = game.findById(attackerId);
            if (combat == null || attacker == null) return null;
            List<Integer> ids = new ArrayList<>();
            for (Card b : combat.getBlockers(attacker)) ids.add(b.getId());
            return ids;
        } catch (Throwable t) {
            Bridge.log("liveBlockers failed: " + t);
            return null;
        }
    }

    private static void putMana(Map<String, Object> m, String k, int v) { if (v > 0) m.put(k, v); }

    private static List<Object> cards(Iterable<CardView> cs, PlayerView me) {
        List<Object> r = new ArrayList<>();
        if (cs != null) for (CardView c : cs) r.add(card(c, me));
        return r;
    }

    /**
     * The command zone also holds Forge's "Commander Effect" card (the rules object that lets commanders be cast from there
     * and go back there); it isn't a game object for the player, so it's left out. Emblems and commanders stay.
     */
    private static Iterable<CardView> visible(Iterable<CardView> cards, ZoneType z) {
        if (z != ZoneType.Command || cards == null) return cards;
        List<CardView> out = new ArrayList<>();
        for (CardView c : cards) if (c != null && !"Commander Effect".equals(c.getName())) out.add(c);
        return out;
    }

    /**
     * Command-zone effects and emblems have no picture of their own: adds the card that made them ({@code source}: Channel
     * for "Channel Effect", the planeswalker for its emblem) so the table can show that card's art, {@code emblem}, and
     * {@code manaAbility} (a mana ability to click while paying: Forge's mana payment marks no card selectable).
     * A read only; if it races the game thread the card just goes without.
     */
    @SuppressWarnings("unchecked")
    private static void addEffectSources(List<Object> cards, Game game) {
        if (game == null) return;
        for (Object o : cards) {
            Map<String, Object> m = (Map<String, Object>) o;
            try {
                Card card = game.findById((Integer) m.get("id"));
                if (card == null) continue;
                if (card.isEmblem()) m.put("emblem", true);
                if (!card.getManaAbilities().isEmpty()) m.put("manaAbility", true);
                Card src = card.getEffectSource();
                if (src == null || src == card) continue;
                Map<String, Object> sm = new LinkedHashMap<>();
                sm.put("name", src.getName());
                sm.put("set", src.getSetCode());
                m.put("source", sm);
            } catch (RuntimeException ignored) {
                // changed while reading it: no source this time
            }
        }
    }

    /** {@code me} = the viewer (the human player), whose view of face-down cards Forge decides. */
    static Map<String, Object> card(CardView c, PlayerView me) {
        Map<String, Object> m = new LinkedHashMap<>();
        m.put("id", c.getId());
        CardView.CardStateView st = c.getCurrentState();
        // Face-down cards the viewer may not look at (the AI's morphs, manifests…) are hidden information: no name, set or
        // text that would let the game's previews show the real card. Forge decides (your own, or ones you may look at).
        boolean hidden = hiddenFace(c, me);
        m.put("name", hidden ? "Face-down card" : st == null ? c.getName() : st.getName());
        if (!hidden) m.put("oracleName", c.getOracleName());
        if (st != null) {
            if (!hidden) {
                m.put("set", st.getSetCode());
                m.put("cost", st.getManaCost() == null ? "" : st.getManaCost().toString());
                m.put("text", st.getOracleText());
            }
            m.put("type", st.getType() == null ? "" : st.getType().toString());
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
        if (c.isFaceDown()) {
            m.put("faceDown", true);
            // A face-down card the viewer may look at (own morph/foretell, Gonti's exiled card): its real face, which
            // Forge keeps as the alternate state. Only for previews; the card itself stays face-down.
            CardView.CardStateView real = hidden ? null : c.getAlternateState();
            if (real != null) {
                Map<String, Object> rm = new LinkedHashMap<>();
                rm.put("name", real.getName());
                rm.put("set", real.getSetCode());
                rm.put("cost", real.getManaCost() == null ? "" : real.getManaCost().toString());
                rm.put("text", real.getOracleText());
                rm.put("type", real.getType() == null ? "" : real.getType().toString());
                if (real.isCreature()) {
                    rm.put("power", real.getPower());
                    rm.put("toughness", real.getToughness());
                }
                m.put("real", rm);
            }
        }
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
