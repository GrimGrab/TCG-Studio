package tcgcc.bridge;

import forge.LobbyPlayer;
import forge.deck.CardPool;
import forge.game.GameEntityView;
import forge.game.GameState;
import forge.game.GameView;
import forge.game.card.CardView;
import forge.game.event.GameEvent;
import forge.game.phase.PhaseType;
import forge.game.player.DelayedReveal;
import forge.game.player.IHasIcon;
import forge.game.player.PlayerView;
import forge.game.spellability.SpellAbilityView;
import forge.game.spellability.StackItemView;
import forge.game.zone.ZoneType;
import forge.gamemodes.match.AbstractGuiGame;
import forge.gamemodes.match.HostedMatch;
import forge.gamemodes.match.NextGameDecision;
import forge.interfaces.IGameController;
import forge.item.PaperCard;
import forge.localinstance.skin.FSkinProp;
import forge.player.PlayerZoneUpdate;
import forge.player.PlayerZoneUpdates;
import forge.util.FSerializableFunction;
import forge.util.ITriggerEvent;

import java.util.ArrayList;
import java.util.Collection;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;

/**
 * The human player's "screen": Forge calls it for every prompt and state change; we forward those to the game as JSON and
 * turn the game's answers back into Forge calls. Modelled on forge-gui's RemoteClientGuiGame (network play).
 */
public final class BridgeGuiGame extends AbstractGuiGame {
    private HostedMatch match;
    private volatile boolean dirty;
    private volatile boolean gameOverSent;
    private String promptText = "";
    private Integer promptCard;
    private String okLabel = "OK", cancelLabel = "Cancel";
    private boolean okEnabled, cancelEnabled;

    public BridgeGuiGame(Object unused) {
        Thread t = new Thread(this::stateLoop, "tcgcc-state");
        t.setDaemon(true);
        t.start();
    }

    void setMatch(HostedMatch m) { match = m; }

    // ------------------------------------------------------------------ state

    /** Coalesces the many update calls into at most ~15 state messages per second. */
    private void stateLoop() {
        while (true) {
            try {
                Thread.sleep(66);
                if (dirty) sendState();
            } catch (InterruptedException e) {
                return;
            }
        }
    }

    private void markDirty() { dirty = true; }

    synchronized void sendState() {
        GameView gv = getGameView();
        if (gv == null) return;
        dirty = false;
        try {
            Map<String, Object> s = StateWriter.write(gv, humanView(), match == null ? null : match.getGame());
            s.put("t", "state");
            Bridge.send(s);
        } catch (RuntimeException e) {
            dirty = true; // the game thread changed a collection mid-walk: try again next tick
        }
        if (gv.isGameOver() && !gameOverSent) {
            gameOverSent = true;
            PlayerView me = humanView();
            String winner = gv.getWinningPlayerName();
            boolean draw = winner == null || winner.isEmpty();
            boolean won = !draw && me != null && winner.equals(me.getName());
            Bridge.send(Bridge.msg("gameover", "won", won, "draw", draw, "winner", winner == null ? "" : winner));
        }
    }

    PlayerView humanView() {
        GameView gv = getGameView();
        if (gv == null) return null;
        for (PlayerView p : gv.getPlayers()) if (!p.isAI()) return p;
        return null;
    }

    private void sendPrompt() {
        sendState();
        Bridge.send(Bridge.msg("prompt", "text", promptText, "card", promptCard,
                "ok", okEnabled ? okLabel : null, "cancel", cancelEnabled ? cancelLabel : null,
                "selectable", selectableIds(), "picked", pickedIds(), "min", getSelectionMin(), "max", getSelectionMax(),
                "selectableCards", offTableSelectables(), "pickedPlayers", pickedPlayerIds(), "cardInfo", promptCardInfo()));
    }

    /** The prompt's card in full: it may be a card the state doesn't carry (single-card scry/surveil ask about a library card). */
    private Map<String, Object> promptCardInfo() {
        GameView gv = getGameView();
        CardView c = promptCard == null ? null : StateWriter.findCard(gv, promptCard);
        if (c == null) return null;
        try {
            return StateWriter.card(c);
        } catch (RuntimeException e) {
            return null;
        }
    }

    /** Players Forge highlights (e.g. the current attack target while declaring attackers); ids can collide with card ids in picked. */
    private List<Integer> pickedPlayerIds() {
        List<Integer> ids = new ArrayList<>();
        GameView gv = getGameView();
        if (gv == null) return ids;
        for (PlayerView p : gv.getPlayers()) if (isHighlighted(p)) ids.add(p.getId());
        return ids;
    }

    /**
     * Selectable cards the table doesn't show (anywhere but the battlefield, my hand and the stack: graveyards, exile,
     * libraries, command zone, other hands...), with full details so the game can show them in a strip. Forge picks them
     * on the board for some costs/targets whatever the preferences (e.g. "exile a card from your graveyard").
     */
    private List<Map<String, Object>> offTableSelectables() {
        List<Map<String, Object>> r = new ArrayList<>();
        GameView gv = getGameView();
        if (gv == null || !isSelecting()) return r;
        PlayerView me = humanView();
        for (CardView c : StateWriter.allCards(gv)) {
            if (!isSelectable(c) || c.getZone() == null) continue;
            ZoneType z = c.getZone();
            if (z == ZoneType.Battlefield || z == ZoneType.Stack) continue;
            if (z == ZoneType.Hand && me != null && c.getController() != null && c.getController().getId() == me.getId()) continue;
            r.add(StateWriter.card(c));
        }
        return r;
    }

    /**
     * Cards already chosen in the current multi-select (Forge highlights them; clicking again deselects). Players go in
     * pickedPlayers: player and card ids overlap (player 1 vs card 1), so one list framed the wrong card.
     */
    private List<Integer> pickedIds() {
        List<Integer> ids = new ArrayList<>();
        GameView gv = getGameView();
        if (gv == null) return ids;
        for (CardView c : StateWriter.allCards(gv)) if (isHighlighted(c)) ids.add(c.getId());
        return ids;
    }

    private List<Integer> selectableIds() {
        List<Integer> ids = new ArrayList<>();
        GameView gv = getGameView();
        if (gv == null || !isSelecting()) return ids;
        for (CardView c : StateWriter.allCards(gv)) if (isSelectable(c)) ids.add(c.getId());
        return ids;
    }

    /**
     * Context for a question: while a spell/ability resolves it's the top of the stack, so questions like "Choose a source"
     * (Healing Grace) can say who is asking. Nothing is added when the stack is empty (e.g. choices made while casting).
     */
    void addContext(Map<String, Object> q) {
        GameView gv = getGameView();
        if (gv == null || gv.getStack() == null || gv.getStack().isEmpty()) return;
        try {
            StackItemView top = gv.getStack().iterator().next();
            Map<String, Object> ctx = new LinkedHashMap<>();
            ctx.put("text", top.getText());
            if (top.getSourceCard() != null) ctx.put("card", StateWriter.card(top.getSourceCard()));
            q.put("context", ctx);
        } catch (RuntimeException ignored) {
            // stack changed while reading it: no context this time
        }
    }

    // ------------------------------------------------------------------ player actions (from the game, on our EDT thread)

    void act(Map<String, Object> m) {
        IGameController c = getGameController();
        if (c == null) return;
        String a = Json.s(m, "a");
        if (a == null) return;
        switch (a) {
            case "card" -> {
                CardView cv = StateWriter.findCard(getGameView(), Json.i(m, "id", -1));
                if (cv != null) c.selectCard(cv, null, null);
            }
            case "player" -> {
                for (PlayerView p : getGameView().getPlayers())
                    if (p.getId() == Json.i(m, "id", -1)) c.selectPlayer(p, null);
            }
            case "ok" -> c.selectButtonOk();
            case "cancel" -> c.selectButtonCancel();
            case "pass" -> c.passPriority();
            case "mana" -> c.useMana((byte) Json.i(m, "color", 0));
            case "alphaStrike" -> c.alphaStrike();
            case "undo" -> c.undoLastAction();
            case "concede" -> c.concede();
            default -> Bridge.log("unknown action " + a);
        }
    }

    // ------------------------------------------------------------------ prompts

    @Override public void showPromptMessage(PlayerView playerView, String message, CardView card) {
        promptText = message == null ? "" : message;
        promptCard = card == null ? null : card.getId();
        sendPrompt();
    }

    @Override public void updateButtons(PlayerView owner, String label1, String label2, boolean enable1, boolean enable2, boolean focus1) {
        okLabel = label1;
        cancelLabel = label2;
        okEnabled = enable1;
        cancelEnabled = enable2;
        sendPrompt();
    }

    @Override public void setSelectables(Iterable<CardView> cards, int min, int max) {
        super.setSelectables(cards, min, max);
        sendPrompt();
    }

    @Override public void setHighlighted(Iterable<GameEntityView> entities, boolean b) {
        super.setHighlighted(entities, b);
        sendPrompt();
    }

    @Override public void clearSelectables() {
        super.clearSelectables();
        // Forge clears a finished selection (e.g. a graveyard target) after the next input already sent its prompt: resend,
        // or the game keeps offering the old cards (stale off-table strip).
        sendPrompt();
    }

    // ------------------------------------------------------------------ questions (block the game thread until answered)

    /** One option per entry: display text + card id when the entry is (or has) a card. */
    private static List<Map<String, Object>> options(List<?> items, FSerializableFunction<Object, String> display) {
        List<Map<String, Object>> r = new ArrayList<>();
        for (Object o : items) {
            Map<String, Object> e = new LinkedHashMap<>();
            String text = display != null ? display.apply(o) : StateWriter.describe(o);
            e.put("text", text);
            if (o instanceof CardView cv) {
                // Forge puts section dividers into some lists as fake cards named "--…--" (e.g. choose a source:
                // "--Permanents--"); they can't be chosen (Forge re-asks), so mark them as headers.
                if (cv.getName() != null && cv.getName().startsWith("--")) {
                    e.put("header", true);
                    e.put("text", cv.getName().replace("-", " ").trim());
                } else {
                    // The option IS a card: full details so the game can show it (library/hidden cards aren't in its state)
                    e.put("card", cv.getId());
                    e.put("cardInfo", StateWriter.card(cv));
                }
            } else {
                // An ability/mode/stack item: the text is what matters; its card is only context for the hover preview
                CardView src = StateWriter.cardViewOf(o);
                if (src != null) e.put("sourceInfo", StateWriter.card(src));
            }
            r.add(e);
        }
        return r;
    }

    /** Asks the player to pick between min and max of items; returns the chosen indexes (null = no answer/cancel). */
    private List<Integer> choose(String kind, String title, List<?> items, int min, int max, FSerializableFunction<Object, String> display) {
        sendState();
        Map<String, Object> q = new LinkedHashMap<>();
        q.put("kind", kind);
        q.put("title", title == null ? "" : title);
        q.put("min", min);
        q.put("max", max);
        q.put("options", options(items, display));
        Map<String, Object> a = Bridge.ask(q);
        if (a == null) return null;
        List<Integer> picked = new ArrayList<>();
        for (int i : Json.ints(a, "picked")) if (i >= 0 && i < items.size() && !picked.contains(i)) picked.add(i);
        if (picked.size() < min) {
            for (int i = 0; picked.size() < min && i < items.size(); i++) if (!picked.contains(i)) picked.add(i);
        }
        while (picked.size() > max) picked.remove(picked.size() - 1);
        return picked;
    }

    private <T> List<T> pick(List<T> items, List<Integer> idx) {
        List<T> r = new ArrayList<>();
        if (idx != null) for (int i : idx) r.add(items.get(i));
        return r;
    }

    @SuppressWarnings("unchecked")
    @Override public <T> List<T> getChoices(String message, int min, int max, List<T> choices, List<T> selected, FSerializableFunction<T, String> display) {
        if (choices == null || choices.isEmpty()) return new ArrayList<>();
        return pick(choices, choose("choose", message, choices, min, max, (FSerializableFunction<Object, String>) (FSerializableFunction<?, String>) display));
    }

    @Override public <T> OrderResult<T> order(String title, String top, int remainingObjectsMin, int remainingObjectsMax, List<T> sourceChoices,
                                             List<T> destChoices, CardView referenceCard, boolean sideboardingMode, boolean showRememberCheckbox) {
        if (remainingObjectsMin == 0 && remainingObjectsMax == 0 && (destChoices == null || destChoices.isEmpty())) {
            // Order everything (triggers, cards on top of the library...): one reorder list.
            if (sourceChoices.size() < 2) return new OrderResult<>(new ArrayList<>(sourceChoices), false);
            List<Integer> perm = reorder((title == null ? "Order" : title) + (top == null || top.isEmpty() ? "" : " - " + top), sourceChoices, -1);
            List<T> r = new ArrayList<>();
            if (perm == null) r.addAll(sourceChoices);
            else for (int i : perm) r.add(sourceChoices.get(i));
            return new OrderResult<>(r, false);
        }
        // Partial ordering (some may stay unordered): pick one by one ("choose first", then next ...).
        List<T> left = new ArrayList<>(sourceChoices);
        List<T> ordered = new ArrayList<>(destChoices == null ? List.of() : destChoices);
        int keepMax = remainingObjectsMax;
        while (left.size() > 1 && left.size() > remainingObjectsMin) {
            List<Integer> p = choose("order", (title == null ? "Order" : title) + " — choose next (" + (ordered.size() + 1) + ")",
                    left, keepMax > 0 ? 0 : 1, 1, null);
            if (p == null || p.isEmpty()) break;
            ordered.add(left.remove((int) p.get(0)));
        }
        if (remainingObjectsMax == 0) ordered.addAll(left);
        return new OrderResult<>(ordered, false);
    }

    @Override public boolean confirm(CardView c, String question, boolean defaultIsYes, List<String> options) {
        List<String> opts = options == null || options.size() < 2 ? List.of("Yes", "No") : options;
        Map<String, Object> q = new LinkedHashMap<>();
        q.put("kind", "confirm");
        q.put("title", question == null ? "" : question);
        q.put("card", c == null ? null : c.getId());
        if (c != null) q.put("cardInfo", StateWriter.card(c)); // may be a hidden card the state doesn't carry
        q.put("options", opts);
        q.put("default", defaultIsYes ? 0 : 1);
        sendState();
        Map<String, Object> a = Bridge.ask(q);
        if (a == null) return defaultIsYes;
        return Json.i(a, "picked", defaultIsYes ? 0 : 1) == 0;
    }

    @Override public boolean showConfirmDialog(String message, String title, String yesButtonText, String noButtonText, boolean defaultYes) {
        return confirm(null, (title == null || title.isEmpty() ? "" : title + ": ") + message, defaultYes,
                List.of(yesButtonText == null ? "Yes" : yesButtonText, noButtonText == null ? "No" : noButtonText));
    }

    @Override public int showOptionDialog(String message, String title, FSkinProp icon, List<String> options, int defaultOption) {
        if (options == null || options.isEmpty()) return defaultOption;
        List<Integer> p = choose("choose", (title == null ? "" : title + ": ") + message, options, 1, 1, null);
        return p == null || p.isEmpty() ? defaultOption : p.get(0);
    }

    @Override public String showInputDialog(String message, String title, FSkinProp icon, String initialInput, List<String> inputOptions, boolean isNumeric) {
        if (inputOptions != null && !inputOptions.isEmpty()) {
            List<Integer> p = choose("choose", message, inputOptions, 1, 1, null);
            return p == null || p.isEmpty() ? initialInput : inputOptions.get(p.get(0));
        }
        // Free text / a number (e.g. "Other..." in a number list): Forge checks the answer and asks again when it's invalid.
        Map<String, Object> q = new LinkedHashMap<>();
        q.put("kind", "input");
        q.put("title", (title == null || title.isEmpty() ? "" : title + ": ") + (message == null ? "" : message));
        q.put("numeric", isNumeric);
        q.put("initial", initialInput == null ? "" : initialInput);
        sendState();
        Map<String, Object> a = Bridge.ask(q);
        if (a == null || Json.b(a, "cancel")) return null; // shutting down / Cancel: Forge treats null as cancel
        String text = Json.s(a, "text");
        return text == null ? initialInput : text;
    }

    @Override public SpellAbilityView getAbilityToPlay(CardView hostCard, List<SpellAbilityView> abilities, ITriggerEvent triggerEvent) {
        if (abilities == null || abilities.isEmpty()) return null;
        if (abilities.size() == 1) return abilities.get(0);
        List<Integer> p = choose("ability", hostCard == null ? "Choose ability" : hostCard.getName(), abilities, 0, 1, null);
        return p == null || p.isEmpty() ? null : abilities.get(p.get(0));
    }

    @Override public GameEntityView chooseSingleEntityForEffect(String title, List<? extends GameEntityView> optionList, DelayedReveal delayedReveal, boolean isOptional) {
        if (optionList == null || optionList.isEmpty()) return null;
        if (delayedReveal != null) reveal(delayedReveal.getMessagePrefix(), new ArrayList<>(delayedReveal.getCards()));
        List<Integer> p = choose("choose", title, optionList, isOptional ? 0 : 1, 1, null);
        return p == null || p.isEmpty() ? null : optionList.get(p.get(0));
    }

    @Override public List<GameEntityView> chooseEntitiesForEffect(String title, List<? extends GameEntityView> optionList, int min, int max, DelayedReveal delayedReveal) {
        if (optionList == null || optionList.isEmpty()) return new ArrayList<>();
        if (delayedReveal != null) reveal(delayedReveal.getMessagePrefix(), new ArrayList<>(delayedReveal.getCards()));
        return new ArrayList<>(pick(optionList, choose("choose", title, optionList, min, max, null)));
    }

    @Override public <T> void reveal(String message, List<T> items) {
        if (items == null || items.isEmpty()) return;
        choose("reveal", message, items, 0, 0, null);
    }

    /** Top/bottom arrangement ("put on top and/or bottom in any order"): one reorder list with a "rest of library" divider. */
    @Override public List<CardView> manipulateCardList(String title, Iterable<CardView> cards, Iterable<CardView> manipulable, boolean toTop, boolean toBottom, boolean toAnywhere) {
        List<CardView> all = new ArrayList<>();
        for (CardView c : cards) all.add(c);
        List<Object> items = new ArrayList<>(all);
        int divider = -1;
        if (toBottom && toTop) {
            divider = items.size();
            items.add("-- rest of library (cards below go to the bottom) --");
        } else if (toBottom) {
            divider = 0;
            items.add(0, "-- rest of library --");
        }
        List<Integer> perm = reorder(title == null ? "Arrange cards" : title, items, divider);
        if (perm == null) return all;
        List<CardView> r = new ArrayList<>();
        for (int i : perm) if (i != divider) r.add((CardView) items.get(i));
        return r;
    }

    /** Asks for a full ordering of items (first = top/first); returns the index permutation, or null. divider = a non-card separator index or -1. */
    private List<Integer> reorder(String title, List<?> items, int divider) {
        sendState();
        Map<String, Object> q = new LinkedHashMap<>();
        q.put("kind", "reorder");
        q.put("title", title == null ? "" : title);
        q.put("options", options(items, null));
        q.put("divider", divider);
        Map<String, Object> a = Bridge.ask(q);
        if (a == null) return null;
        List<Integer> perm = Json.ints(a, "picked");
        // must be a permutation of 0..n-1, else keep the original order
        if (perm.size() != items.size() || new java.util.HashSet<>(perm).size() != items.size()) return null;
        for (int i : perm) if (i < 0 || i >= items.size()) return null;
        return perm;
    }

    /**
     * Asks for one number per option summing to total (within mins/maxs; suggest = what the client's Auto button restores).
     * extra = additional per-option fields for the dialog (e.g. combat's "lethal" / "afterLethal"). Null = no valid answer.
     */
    private int[] amounts(String title, List<?> items, List<String> labels, int total, int[] mins, int[] maxs, int[] suggest, String unit,
                          List<Map<String, Object>> extra) {
        sendState();
        List<Map<String, Object>> opts = options(items, null);
        for (int i = 0; i < opts.size(); i++) {
            if (extra != null && extra.get(i) != null) opts.get(i).putAll(extra.get(i));
            if (labels != null && labels.get(i) != null) opts.get(i).put("text", labels.get(i));
            opts.get(i).put("min", mins[i]);
            opts.get(i).put("max", maxs[i]);
            opts.get(i).put("suggest", suggest[i]);
        }
        Map<String, Object> q = new LinkedHashMap<>();
        q.put("kind", "amounts");
        q.put("title", title == null ? "" : title);
        q.put("total", total);
        q.put("unit", unit == null ? "" : unit);
        q.put("options", opts);
        Map<String, Object> a = Bridge.ask(q);
        if (a == null) return null;
        List<Integer> got = Json.ints(a, "amounts");
        if (got.size() != items.size()) return null;
        int sum = 0;
        int[] r = new int[items.size()];
        for (int i = 0; i < r.length; i++) {
            r[i] = got.get(i);
            if (r[i] < mins[i] || r[i] > maxs[i]) return null;
            sum += r[i];
        }
        return sum == total ? r : null;
    }

    @Override public Map<CardView, Integer> assignCombatDamage(CardView attacker, List<CardView> blockers, int damage, GameEntityView defender, boolean overrideOrder, boolean maySkip) {
        // Same rules as Forge's own dialog (forge-gui-desktop VAssignCombatDamage): the defending player/planeswalker is only an
        // option with trample, or with "divide damage as you choose" when Forge allows it (overrideOrder). Trample must give
        // every blocker lethal damage first; divide-as-you-choose may split freely.
        boolean trample = defender != null && attacker != null && attacker.getCurrentState().hasTrample();
        boolean divide = attacker != null && attacker.getCurrentState().hasDivideDamage();
        boolean toDefender = defender != null && (trample || (divide && overrideOrder));
        boolean lethalFirst = trample && !divide;
        List<Object> items = new ArrayList<>(blockers);
        List<String> labels = new ArrayList<>();
        List<Map<String, Object>> extra = new ArrayList<>();
        for (CardView b : blockers) {
            int lethal = Math.max(0, b.getLethalDamage());
            labels.add(b.getName() + " (" + b.getId() + ")");
            extra.add(new LinkedHashMap<>(Map.of("lethal", lethal)));
        }
        if (toDefender) {
            items.add(defender);
            labels.add(StateWriter.describe(defender) + (trample ? " (trample)" : ""));
            // Combat rule: trample damage reaches the player/planeswalker only after every blocker has lethal damage.
            extra.add(lethalFirst ? new LinkedHashMap<>(Map.of("afterLethal", true)) : null);
        }
        int n = items.size();
        int[] mins = new int[n], maxs = new int[n], auto = new int[n];
        java.util.Arrays.fill(maxs, damage);
        // Default (the old automatic split): lethal to each blocker in order, the rest to the defender (trample) or the last blocker.
        int left = damage;
        for (int i = 0; i < blockers.size(); i++) {
            int d = Math.min(left, Math.max(0, blockers.get(i).getLethalDamage()));
            auto[i] = d;
            left -= d;
        }
        if (left > 0) auto[toDefender ? n - 1 : Math.max(0, blockers.size() - 1)] += left;

        int[] got = n <= 1 ? null : amounts("Assign " + damage + " combat damage from " + (attacker == null ? "attacker" : attacker.getName()),
                items, labels, damage, mins, maxs, auto, "damage", extra);
        if (got != null && lethalFirst && got[n - 1] > 0) {
            // Same rule as the dialog enforces ("afterLethal"): refuse a split that breaks it.
            for (int i = 0; i < blockers.size(); i++)
                if (got[i] < Math.max(0, blockers.get(i).getLethalDamage())) { got = null; break; }
        }
        if (got == null) got = auto;
        Map<CardView, Integer> r = new LinkedHashMap<>();
        for (int i = 0; i < blockers.size(); i++) r.put(blockers.get(i), got[i]);
        if (toDefender && got[n - 1] > 0) r.put(null, got[n - 1]); // null key = defender (PlayerControllerHuman)
        return r;
    }

    @Override public Map<Object, Integer> assignGenericAmount(CardView effectSource, Map<Object, Integer> target, int amount, boolean atLeastOne, String amountLabel) {
        // Divided damage/counters/shields (atLeastOne) and "any combination of colours" mana; map values are per-option maximums.
        List<Object> keys = new ArrayList<>(target.keySet());
        Map<Object, Integer> r = new LinkedHashMap<>();
        if (keys.isEmpty()) return r;
        int n = keys.size();
        int[] mins = new int[n], maxs = new int[n], auto = new int[n];
        for (int i = 0; i < n; i++) {
            Integer cap = target.get(keys.get(i));
            maxs[i] = cap == null || cap <= 0 ? amount : Math.min(cap, amount);
            mins[i] = atLeastOne ? 1 : 0;
        }
        // Default: minimums first, then spread evenly within the caps.
        int left = amount;
        for (int i = 0; i < n; i++) { auto[i] = Math.min(mins[i], left); left -= auto[i]; }
        for (int guard = 0; left > 0 && guard < amount * n + 1; guard++) {
            int i = guard % n;
            if (auto[i] < maxs[i]) { auto[i]++; left--; }
        }
        String unit = amountLabel == null ? "" : amountLabel;
        int[] got = amounts("Divide " + amount + " " + unit + (effectSource == null ? "" : " (" + effectSource.getName() + ")"),
                keys, null, amount, mins, maxs, auto, unit, null);
        if (got == null) got = auto;
        for (int i = 0; i < n; i++) r.put(keys.get(i), got[i]);
        return r;
    }

    @Override public List<PaperCard> sideboard(CardPool sideboard, CardPool main, String message) { return new ArrayList<>(); }

    @Override public void message(String message, String title) {
        Bridge.send(Bridge.msg("message", "title", title, "text", message));
    }

    @Override public void showErrorDialog(String message, String title) {
        Bridge.send(Bridge.msg("message", "title", title, "text", message, "error", true));
    }

    // ------------------------------------------------------------------ phase stops

    @Override public boolean isUiSetToSkipPhase(PlayerView playerTurn, PhaseType phase) {
        // Forge's own phase stops (desktop: the phase indicator toggles, initialised from these preferences). Defaults stop on
        // your Main 1 / after blockers / Main 2, and on the AI's beginning of combat / attackers / blockers / end step.
        String step = switch (phase) {
            case UPKEEP -> "UPKEEP";
            case DRAW -> "DRAW";
            case MAIN1 -> "MAIN1";
            case COMBAT_BEGIN -> "BEGINCOMBAT";
            case COMBAT_DECLARE_ATTACKERS -> "DECLAREATTACKERS";
            case COMBAT_DECLARE_BLOCKERS -> "DECLAREBLOCKERS";
            case COMBAT_FIRST_STRIKE_DAMAGE -> "FIRSTSTRIKE";
            case COMBAT_DAMAGE -> "COMBATDAMAGE";
            case COMBAT_END -> "ENDCOMBAT";
            case MAIN2 -> "MAIN2";
            case END_OF_TURN -> "EOT";
            case CLEANUP -> "CLEANUP";
            default -> null;
        };
        if (step == null || playerTurn == null) return false; // same as desktop: no toggle for this step = don't skip
        PlayerView me = humanView();
        boolean mine = me != null && playerTurn.getId() == me.getId();
        try {
            var pref = forge.localinstance.properties.ForgePreferences.FPref.valueOf("PHASE_" + (mine ? "HUMAN" : "AI") + "_" + step);
            return !forge.model.FModel.getPreferences().getPrefBoolean(pref);
        } catch (IllegalArgumentException e) {
            return false;
        }
    }

    // ------------------------------------------------------------------ view updates → state

    @Override protected void updateCurrentPlayer(PlayerView player) { markDirty(); }
    @Override public void setGameView(GameView gameView) { super.setGameView(gameView); markDirty(); }
    @Override public void openView(forge.trackable.TrackableCollection<PlayerView> myPlayers) { markDirty(); }
    @Override public void afterGameEnd() { super.afterGameEnd(); sendState(); }
    @Override public void showCombat() { markDirty(); }
    @Override public void flashIncorrectAction() { Bridge.send(Bridge.msg("flash")); }
    @Override public void alertUser() {}
    @Override public void updatePhase(boolean saveState) { super.updatePhase(saveState); markDirty(); }
    @Override public void updateTurn(PlayerView player) { super.updateTurn(player); markDirty(); }
    @Override public void updatePlayerControl() { super.updatePlayerControl(); markDirty(); }
    @Override public void enableOverlay() {}
    @Override public void disableOverlay() {}
    @Override public void finishGame() {
        sendState();
        // One game per match: tell Forge we're done so the match thread can end.
        IGameController c = getGameController();
        if (c != null) c.nextGameDecision(NextGameDecision.QUIT);
    }
    @Override public void showManaPool(PlayerView player) { markDirty(); }
    @Override public void hideManaPool(PlayerView player) { markDirty(); }
    @Override public void updateStack() { super.updateStack(); markDirty(); }
    @Override public void handleGameEvent(GameEvent event) { super.handleGameEvent(event); markDirty(); }
    @Override public Iterable<PlayerZoneUpdate> tempShowZones(PlayerView controller, Iterable<PlayerZoneUpdate> zonesToUpdate) { markDirty(); return zonesToUpdate; }
    @Override public void hideZones(PlayerView controller, Iterable<PlayerZoneUpdate> zonesToUpdate) { markDirty(); }
    @Override public void updateZones(Iterable<PlayerZoneUpdate> zonesToUpdate) { super.updateZones(zonesToUpdate); markDirty(); }
    @Override public void updateCards(Iterable<CardView> cards) { super.updateCards(cards); markDirty(); }
    @Override public void refreshField() { super.refreshField(); markDirty(); }
    @Override public GameState getGamestate() { return null; }
    @Override public void updateManaPool(Iterable<PlayerView> manaPoolUpdate) { super.updateManaPool(manaPoolUpdate); markDirty(); }
    @Override public void updateLives(Iterable<PlayerView> livesUpdate) { super.updateLives(livesUpdate); markDirty(); }
    @Override public void updateShards(Iterable<PlayerView> shardsUpdate) { markDirty(); }
    @Override public void setPanelSelection(CardView hostCard) {}
    @Override public void setCard(CardView card) {} // detail panel; the prompt that follows carries the card (cardInfo)
    @Override public void setPlayerAvatar(LobbyPlayer player, IHasIcon ihi) {}
    @Override public PlayerZoneUpdates openZones(PlayerView controller, Collection<ZoneType> zones, Map<PlayerView, Object> players, boolean backupLastZones) {
        markDirty();
        return new PlayerZoneUpdates();
    }
    @Override public void restoreOldZones(PlayerView playerView, PlayerZoneUpdates playerZoneUpdates) { markDirty(); }
}
