package tcgcc.bridge;

import forge.gamemodes.match.HostedMatch;
import forge.gui.download.GuiDownloadService;
import forge.gui.interfaces.IGuiBase;
import forge.gui.interfaces.IGuiGame;
import forge.item.PaperCard;
import forge.localinstance.skin.FSkinProp;
import forge.localinstance.skin.ISkinImage;
import forge.sound.IAudioClip;
import forge.sound.IAudioMusic;
import forge.util.FSerializableFunction;
import forge.util.ImageFetcher;
import org.jupnp.UpnpServiceConfiguration;

import java.io.File;
import java.util.ArrayList;
import java.util.Collection;
import java.util.List;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.Future;
import java.util.function.Consumer;

/**
 * Forge's platform layer with no window: a single "EDT" thread stands in for Swing's event thread (player actions from the
 * game run there, like Forge desktop), dialogs never open, no sound or images.
 */
public final class HeadlessGui implements IGuiBase {
    private volatile Thread edtThread;
    private final ExecutorService edt = Executors.newSingleThreadExecutor(r -> {
        Thread t = new Thread(r, "tcgcc-edt");
        t.setDaemon(true);
        edtThread = t;
        return t;
    });

    @Override public boolean isRunningOnDesktop() { return true; }
    @Override public boolean isLibgdxPort() { return false; }
    @Override public String getCurrentVersion() { return "tcgcc-bridge"; }

    @Override public void invokeInEdtNow(Runnable r) { r.run(); }
    @Override public void invokeInEdtLater(Runnable r) { edt.submit(wrap(r)); }
    @Override public void invokeInEdtAndWait(Runnable r) {
        if (isGuiThread()) { r.run(); return; }
        Future<?> f = edt.submit(wrap(r));
        try { f.get(); } catch (Exception e) { Bridge.log("invokeInEdtAndWait: " + e); }
    }
    @Override public void runBackgroundTask(String message, Runnable task) { new Thread(wrap(task), "tcgcc-bg").start(); }
    @Override public boolean isGuiThread() { return Thread.currentThread() == edtThread; }

    private static Runnable wrap(Runnable r) {
        return () -> {
            try { r.run(); } catch (Throwable t) { Bridge.log("EDT task failed: " + t); t.printStackTrace(); }
        };
    }

    @Override public String getAssetsDir() { return ""; } // like GuiDesktop: res/ is under the working directory
    @Override public ImageFetcher getImageFetcher() { return null; }
    @Override public ISkinImage getSkinIcon(FSkinProp skinProp) { return null; }
    @Override public ISkinImage getUnskinnedIcon(String path) { return null; }
    @Override public ISkinImage getCardArt(PaperCard card, boolean backFace) { return null; }
    @Override public ISkinImage createLayeredImage(PaperCard card, FSkinProp background, String overlayFilename, float opacity) { return null; }
    @Override public void clearImageCache() {}
    @Override public String encodeSymbols(String str, boolean formatReminderText) { return str; }
    @Override public int getAvatarCount() { return 1; }
    @Override public int getSleevesCount() { return 1; }
    @Override public float getScreenScale() { return 1f; }
    @Override public void preventSystemSleep(boolean preventSleep) {}
    @Override public void download(GuiDownloadService service, Consumer<Boolean> callback) { if (callback != null) callback.accept(false); }
    @Override public void copyToClipboard(String text) {}
    @Override public void browseToUrl(String url) {}

    @Override public void showCardList(String title, String message, List<PaperCard> list) {}
    @Override public boolean showBoxedProduct(String title, String message, List<PaperCard> list) { return false; }
    @Override public void showBugReportDialog(String title, String text, boolean showExitAppBtn) { Bridge.log("BUG " + title + ": " + text); }
    @Override public void showImageDialog(ISkinImage image, String message, String title) {}
    @Override public int showOptionDialog(String message, String title, FSkinProp icon, List<String> options, int defaultOption) { return defaultOption; }
    @Override public String showInputDialog(String message, String title, FSkinProp icon, String initialInput, List<String> inputOptions, boolean isNumeric) { return initialInput; }
    @Override public String showFileDialog(String title, String defaultDir) { return null; }
    @Override public File getSaveFile(File defaultFile) { return defaultFile; }
    @Override public <T> List<T> order(String title, String top, int remainingObjectsMin, int remainingObjectsMax, List<T> sourceChoices, List<T> destChoices) {
        return new ArrayList<>(sourceChoices);
    }
    @Override public <T> List<T> getChoices(String message, int min, int max, Collection<T> choices, Collection<T> selected, FSerializableFunction<T, String> display) {
        List<T> r = new ArrayList<>();
        for (T t : choices) { if (r.size() >= min) break; r.add(t); }
        return r;
    }
    @Override public PaperCard chooseCard(String title, String message, List<PaperCard> list) { return list.isEmpty() ? null : list.get(0); }

    @Override public boolean isSupportedAudioFormat(File file) { return false; }
    @Override public IAudioClip createAudioClip(String filename) { return null; }
    @Override public IAudioMusic createAudioMusic(String filename) { return null; }
    @Override public void startAltSoundSystem(String filename, boolean isSynchronized) {}

    @Override public void showSpellShop() {}
    @Override public void showBazaar() {}

    @Override public IGuiGame getNewGuiGame() { return new BridgeGuiGame(null); } // spectator only; never used with a human
    @Override public HostedMatch hostMatch() { return new HostedMatch(); }
    @Override public UpnpServiceConfiguration getUpnpPlatformService() { return null; }
    @Override public boolean hasNetGame() { return false; }
}
