package tcgcc.bridge;

import forge.LobbyPlayer;
import forge.ai.LobbyPlayerAi;
import forge.deck.Deck;
import forge.deck.io.DeckSerializer;
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
    public static final String VERSION = "5";

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
            String profile = null;
            if (m.get("aiDeck") instanceof Map<?, ?> spec0) { // Forge builds the AI deck; the file is the fallback
                @SuppressWarnings("unchecked")
                Map<String, Object> spec = (Map<String, Object>) spec0;
                AiDeckGen.Result r = AiDeckGen.build(spec, ai);
                ai = r.deck;
                profile = Json.s(spec, "profile");
                send(msg("aideck", "style", Json.s(spec, "style"), "source", r.source, "colors", r.colors,
                        "cards", ai.getMain().countAll(), "lines", r.lines, "list", r.list, "profile", profile));
            }
            String name = Json.s(m, "name");
            String aiName = Json.s(m, "opponentName");

            RegisteredPlayer rpHuman = new RegisteredPlayer(human);
            rpHuman.setPlayer(GamePlayerUtil.getGuiPlayer(name == null ? "Player" : name, 0, 0, false));
            RegisteredPlayer rpAi = new RegisteredPlayer(ai);
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
            match.startMatch(GameType.Constructed, null, players, rpHuman, gui);
            log("match started: " + human.getName() + " vs " + ai.getName());
        } catch (Throwable t) {
            log("start failed: " + t);
            t.printStackTrace();
            send(msg("error", "text", "Couldn't start the match: " + t.getMessage()));
        }
    }

    /** Sends a question and blocks the calling (game) thread until the game answers. Null if the bridge is shutting down. */
    static Map<String, Object> ask(Map<String, Object> q) {
        if (quitting) return null;
        int id = nextAsk.getAndIncrement();
        CompletableFuture<Map<String, Object>> f = new CompletableFuture<>();
        pending.put(id, f);
        q.put("t", "ask");
        q.put("id", id);
        if (gui != null) gui.addContext(q); // which spell/ability is asking (Forge doesn't pass it to its dialogs)
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
