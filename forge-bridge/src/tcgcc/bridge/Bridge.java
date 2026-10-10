package tcgcc.bridge;

import forge.LobbyPlayer;
import forge.ai.LobbyPlayerAi;
import forge.deck.Deck;
import forge.deck.DeckFormat;
import forge.deck.io.DeckSerializer;
import forge.game.GameFormat;
import forge.game.GameType;
import forge.game.player.RegisteredPlayer;
import forge.gamemodes.match.HostedMatch;
import forge.gui.GuiBase;
import forge.localinstance.properties.ForgePreferences;
import forge.localinstance.properties.ForgePreferences.FPref;
import forge.model.FModel;
import forge.player.GamePlayerUtil;

import java.io.BufferedReader;
import java.io.File;
import java.io.FileDescriptor;
import java.io.FileOutputStream;
import java.io.InputStreamReader;
import java.io.PrintStream;
import java.nio.charset.StandardCharsets;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.concurrent.CompletableFuture;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.atomic.AtomicInteger;

/**
 * TCG Custom Cards ↔ Forge bridge (GPL-3.0, links against Forge). Runs Forge with no window; the game talks to it over
 * stdin/stdout, one JSON object per line. Protocol: docs/mtg-forge.md "Bridge protocol".
 *
 * <p>Usage: {@code java -cp forge-gui-desktop.jar;tcgcc-forge-bridge.jar tcgcc.bridge.Bridge <logFile>} with the Forge program
 * folder as working directory (its forge.profile.properties points Forge at our user dir).
 */
public final class Bridge {
    public static final String VERSION = "7";

    private static PrintStream proto;
    private static PrintStream logStream;
    private static final AtomicInteger nextAsk = new AtomicInteger(1);
    private static final Map<Integer, CompletableFuture<Map<String, Object>>> pending = new ConcurrentHashMap<>();
    private static volatile boolean quitting;
    static HeadlessGui platform;
    static BridgeGuiGame gui;

    public static void main(String[] args) throws Exception {
        proto = new PrintStream(new FileOutputStream(FileDescriptor.out), true, StandardCharsets.UTF_8);
        String logFile = args.length > 0 ? args[0] : "tcgcc-bridge.log";
        logStream = new PrintStream(new FileOutputStream(logFile, false), true, StandardCharsets.UTF_8);
        System.setOut(logStream); // Forge prints a lot; keep stdout for the protocol
        System.setErr(logStream);
        System.setProperty("java.util.Arrays.useLegacyMergeSort", "true"); // same hack as forge.view.Main

        try {
            platform = new HeadlessGui();
            GuiBase.setInterface(platform);
            FModel.initialize(null, null);
            ForgePreferences prefs = FModel.getPreferences();
            prefs.setPref(FPref.UI_ENABLE_SOUNDS, false);
            prefs.setPref(FPref.UI_ENABLE_MUSIC, false);
            prefs.setPref(FPref.UI_MATCHES_PER_GAME, "1"); // a table duel is one game
            // Forge's desktop picks cards in libraries, graveyards, exile, other hands... by clicking them in its pop-up zone
            // windows. The table has no such windows, so ask with a dialog instead (in memory only: never saved to the
            // player's preferences). On-board picking then stays for the battlefield and your own hand.
            prefs.setPref(FPref.UI_SELECT_FROM_CARD_DISPLAYS, false);
            if (prefs.getPref(FPref.PLAYER_NAME).isBlank()) prefs.setPref(FPref.PLAYER_NAME, "Player");
        } catch (Throwable t) {
            log("init failed: " + t);
            t.printStackTrace();
            send(msg("error", "text", "Forge failed to start: " + t));
            System.exit(2);
        }
        send(msg("ready", "version", VERSION));

        BufferedReader in = new BufferedReader(new InputStreamReader(System.in, StandardCharsets.UTF_8));
        String line;
        while ((line = in.readLine()) != null) {
            if (line.isBlank()) continue;
            try {
                handle(Json.obj(line));
            } catch (Throwable t) {
                log("bad message " + line + ": " + t);
                t.printStackTrace();
            }
        }
        quit(); // game closed the pipe
    }

    private static void handle(Map<String, Object> m) {
        String t = Json.s(m, "t");
        if (t == null) return;
        switch (t) {
            case "start" -> startMatch(m);
            case "validate" -> validate(m);
            case "commanderok" -> commanderOk(m);
            case "draft" -> Draft.start(m);
            case "sealed" -> Draft.startSealed(m);
            case "draftquit" -> cancelAsks(); // the player left the pick screen: the waiting draft pick gets null and aborts
            case "answer" -> {
                CompletableFuture<Map<String, Object>> f = pending.remove(Json.i(m, "id", -1));
                if (f != null) f.complete(m);
            }
            case "act" -> { if (gui != null) platform.invokeInEdtLater(() -> gui.act(m)); }
            case "state" -> { if (gui != null) gui.sendState(); }
            case "quit" -> quit();
            default -> log("unknown message " + t);
        }
    }

    private static void startMatch(Map<String, Object> m) {
        try {
            Deck human = DeckSerializer.fromFile(new File(Json.s(m, "deck")));
            Deck ai = DeckSerializer.fromFile(new File(Json.s(m, "opponent")));
            if (human == null || ai == null) throw new IllegalArgumentException("deck file unreadable");
            boolean commander = "commander".equals(Json.s(m, "variant"));
            String profile = null;
            if (m.get("aiDeck") instanceof Map<?, ?> spec0) { // Forge builds the AI deck; the file is the fallback
                @SuppressWarnings("unchecked")
                Map<String, Object> spec = (Map<String, Object>) spec0;
                AiDeckGen.Result r = AiDeckGen.build(spec, ai);
                if (r.deck == null) throw new IllegalArgumentException(String.join("; ", r.lines));
                ai = r.deck;
                profile = Json.s(spec, "profile");
                send(msg("aideck", "style", Json.s(spec, "style"), "source", r.source, "colors", r.colors,
                        "cards", ai.getMain().countAll() + ai.getCommanders().size(), "lines", r.lines, "list", r.list,
                        "profile", profile, "commander", r.commander));
            }
            if (commander && (human.getCommanders().isEmpty() || ai.getCommanders().isEmpty()))
                throw new IllegalArgumentException(human.getCommanders().isEmpty() ? "your deck has no commander" : "the customer's deck has no commander");
            String name = Json.s(m, "name");
            String aiName = Json.s(m, "opponentName");

            // Commander: forCommander puts the decks' commanders in the command zone (Player.initVariantsZones); the rules
            // (21 commander damage, commander effect) come from the applied variant below, not from the GameType.
            RegisteredPlayer rpHuman = commander ? RegisteredPlayer.forCommander(human) : new RegisteredPlayer(human);
            rpHuman.setPlayer(GamePlayerUtil.getGuiPlayer(name == null ? "Player" : name, 0, 0, false));
            RegisteredPlayer rpAi = commander ? RegisteredPlayer.forCommander(ai) : new RegisteredPlayer(ai);
            LobbyPlayer aiPlayer = GamePlayerUtil.createAiPlayer(aiName == null ? "Opponent" : aiName, 1);
            if (profile != null && !profile.isEmpty() && aiPlayer instanceof LobbyPlayerAi lpa) lpa.setAiProfile(profile); // res/ai/<profile>.ai
            rpAi.setPlayer(aiPlayer);
            int lifeYou = Json.i(m, "lifeYou", 20), lifeAi = Json.i(m, "lifeCustomer", 20);
            rpHuman.setStartingLife(Math.max(1, lifeYou));
            rpAi.setStartingLife(Math.max(1, lifeAi));
            List<RegisteredPlayer> players = new ArrayList<>(List.of(rpHuman, rpAi));

            gui = new BridgeGuiGame(null);
            HostedMatch match = new HostedMatch();
            gui.setMatch(match);
            if (commander) match.startMatch(GameType.Commander, java.util.EnumSet.of(GameType.Commander), players, rpHuman, gui);
            else match.startMatch(GameType.Constructed, null, players, rpHuman, gui);
            log("match started: " + human.getName() + " vs " + ai.getName());
        } catch (Throwable t) {
            log("start failed: " + t);
            t.printStackTrace();
            send(msg("error", "text", "Couldn't start the match: " + t.getMessage()));
        }
    }

    /**
     * Tournament deck check, decided by Forge only: {@code validate{id, deck (.dck path), kind: constructed|limited, sets[]}} →
     * {@code validated{id, ok, problems[]}}. Problems = Forge's own messages: the deck-size/copy rules of DeckFormat.Constructed
     * or Limited, then a GameFormat limited to the event's set codes (a card is legal when that name was printed in one of them).
     */
    /**
     * {@code commanderok{id, name, set}} → {@code commanderok{id, ok, reason}}: Forge's DeckFormat.Commander.isLegalCommander
     * (legendary creature or "can be your commander", Commander ban list), asked when the player picks a commander.
     */
    private static void commanderOk(Map<String, Object> m) {
        int id = Json.i(m, "id", -1);
        boolean ok = false;
        String reason = null;
        try {
            String name = Json.s(m, "name"), set = Json.s(m, "set");
            var db = FModel.getMagicDb().getCommonCards();
            var pc = set == null || set.isEmpty() ? null : db.getCard(name, set);
            if (pc == null) pc = db.getCard(name);
            if (pc == null) reason = "Forge doesn't know " + name;
            else {
                ok = DeckFormat.Commander.isLegalCommander(pc.getRules());
                if (!ok) reason = name + " can't be a commander (Forge: legendary creatures and cards that say they can be your commander; banned cards can't)";
            }
        } catch (Throwable t) {
            reason = "Forge couldn't check: " + t.getMessage();
        }
        send(msg("commanderok", "id", id, "ok", ok, "reason", reason));
    }

    private static void validate(Map<String, Object> m) {
        int id = Json.i(m, "id", -1);
        List<String> problems = new ArrayList<>();
        try {
            Deck deck = DeckSerializer.fromFile(new File(Json.s(m, "deck")));
            if (deck == null) throw new IllegalArgumentException("deck file unreadable");
            String kind = Json.s(m, "kind");
            DeckFormat rules = "limited".equals(kind) ? DeckFormat.Limited : "commander".equals(kind) ? DeckFormat.Commander : DeckFormat.Constructed;
            String p = rules.getDeckConformanceProblem(deck);
            if (p != null) problems.add(p);
            List<String> sets = new ArrayList<>();
            if (m.get("sets") instanceof List<?> l) for (Object o : l) if (o != null) sets.add(String.valueOf(o).toUpperCase());
            if (!sets.isEmpty()) {
                String q = new GameFormat("Tournament", sets, new ArrayList<>()).getDeckConformanceProblem(deck);
                if (q != null) problems.add(q);
            }
            log("validate " + deck.getName() + " sets " + sets + ": " + (problems.isEmpty() ? "ok" : problems));
        } catch (Throwable t) {
            log("validate failed: " + t);
            problems.add("Forge couldn't check the deck: " + t.getMessage());
        }
        send(msg("validated", "id", id, "ok", problems.isEmpty(), "problems", problems));
    }

    /** Sends a question and blocks the calling (game) thread until the game answers. Null if the bridge is shutting down. */
    static Map<String, Object> ask(Map<String, Object> q) {
        return ask(q, true);
    }

    /** Completes every waiting question with null (its asker gives up). */
    static void cancelAsks() {
        for (Integer id : new ArrayList<>(pending.keySet())) {
            CompletableFuture<Map<String, Object>> f = pending.remove(id);
            if (f != null) f.complete(null);
        }
    }

    /** {@code withContext}: add the resolving spell/ability (match questions); drafts have none. */
    static Map<String, Object> ask(Map<String, Object> q, boolean withContext) {
        if (quitting) return null;
        int id = nextAsk.getAndIncrement();
        CompletableFuture<Map<String, Object>> f = new CompletableFuture<>();
        pending.put(id, f);
        q.put("t", "ask");
        q.put("id", id);
        if (withContext && gui != null) gui.addContext(q); // which spell/ability is asking (Forge doesn't pass it to its dialogs)
        send(q);
        try {
            return f.get();
        } catch (Exception e) {
            return null;
        }
    }

    static void send(Map<String, Object> m) {
        String s = Json.write(m);
        synchronized (Bridge.class) {
            proto.println(s);
        }
    }

    static Map<String, Object> msg(String type, Object... kv) {
        Map<String, Object> m = new LinkedHashMap<>();
        m.put("t", type);
        for (int i = 0; i + 1 < kv.length; i += 2) m.put(String.valueOf(kv[i]), kv[i + 1]);
        return m;
    }

    static void log(String s) {
        if (logStream != null) logStream.println("[bridge] " + s);
    }

    private static void quit() {
        quitting = true;
        for (CompletableFuture<Map<String, Object>> f : pending.values()) f.complete(null);
        log("quit");
        logStream.flush();
        System.exit(0);
    }
}
