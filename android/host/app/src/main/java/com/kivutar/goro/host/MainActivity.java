package com.kivutar.goro.host;

import android.app.Activity;
import android.os.Bundle;
import android.view.MotionEvent;
import android.view.SurfaceHolder;
import android.view.SurfaceView;
import android.view.WindowInsets;
import android.view.Gravity;
import android.view.inputmethod.InputMethodManager;
import android.view.inputmethod.EditorInfo;
import android.graphics.Color;
import android.text.Editable;
import android.text.InputType;
import android.text.TextWatcher;
import android.os.Handler;
import android.os.Looper;
import android.widget.EditText;
import android.widget.FrameLayout;
import android.content.res.AssetManager;
import android.content.res.Configuration;
import android.content.BroadcastReceiver;
import android.content.Context;
import android.content.Intent;
import android.content.IntentFilter;
import android.os.Build;
import android.util.Log;
import java.io.FileInputStream;
import java.io.FileOutputStream;
import java.io.InputStream;
import java.io.ByteArrayOutputStream;
import java.io.File;
import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.util.ArrayList;
import org.json.JSONArray;
import org.json.JSONObject;

public final class MainActivity extends Activity {
    static {
        System.loadLibrary("goro_android");
        System.loadLibrary("goro_jni");
    }

    private HostSurfaceView surfaceView;
    private FrameLayout rootView;
    private EditText chatInput;
    private boolean chatInputActive;
    private int chatInputMode;
    private boolean syncingChatInput;
    private final Handler uiHandler = new Handler(Looper.getMainLooper());
    private final Runnable chatInputPoll = new Runnable() {
        @Override public void run() {
            if (isFinishing()) return;
            setChatInputMode(nativeTextInputMode());
            uiHandler.postDelayed(this, 100);
        }
    };
    private final BroadcastReceiver assetPatchReceiver = new BroadcastReceiver() {
        @Override public void onReceive(Context context, Intent intent) {
            if (AssetPatchWorker.ACTION_PATCH_RELEASE_READY.equals(intent.getAction())) {
                String release = intent.getStringExtra("release");
                if (release != null) nativeBeginAssetRelease(release, intent.getIntExtra("overlay_count", 0));
                return;
            }
            if (!AssetPatchWorker.ACTION_PATCH_READY.equals(intent.getAction())) return;
            String name = intent.getStringExtra("name");
            String format = intent.getStringExtra("format");
            String path = intent.getStringExtra("path");
            if (name == null || format == null || path == null) return;
            nativeMountAssetOverlay(name, format, path, intent.getIntExtra("priority", 0));
        }
    };

    @Override protected void onCreate(Bundle state) {
        super.onCreate(state);
        IntentFilter patchFilter = new IntentFilter(AssetPatchWorker.ACTION_PATCH_READY);
        patchFilter.addAction(AssetPatchWorker.ACTION_PATCH_RELEASE_READY);
        if (Build.VERSION.SDK_INT >= 33) {
            registerReceiver(assetPatchReceiver, patchFilter, Context.RECEIVER_NOT_EXPORTED);
        } else {
            registerReceiver(assetPatchReceiver, patchFilter);
        }
        getWindow().setFlags(1024, 1024);
        surfaceView = new HostSurfaceView();
        surfaceView.setOnApplyWindowInsetsListener((view, insets) -> {
            android.graphics.Insets bars = insets.getInsets(WindowInsets.Type.systemBars());
            nativeSurfaceInsetsChanged(bars.left, bars.top, bars.right, bars.bottom);
            return insets;
        });
        surfaceView.post(() -> surfaceView.requestApplyInsets());
        rootView = new FrameLayout(this);
        rootView.addView(surfaceView, new FrameLayout.LayoutParams(
                FrameLayout.LayoutParams.MATCH_PARENT, FrameLayout.LayoutParams.MATCH_PARENT));
        chatInput = new EditText(this);
        chatInput.setSingleLine(true);
        chatInput.setImeOptions(EditorInfo.IME_ACTION_NONE);
        chatInput.setInputType(InputType.TYPE_CLASS_TEXT | InputType.TYPE_TEXT_FLAG_CAP_SENTENCES);
        chatInput.setTextColor(Color.TRANSPARENT);
        chatInput.setHintTextColor(Color.TRANSPARENT);
        chatInput.setBackgroundColor(Color.TRANSPARENT);
        chatInput.setCursorVisible(false);
        chatInput.setTextSize(1.0f);
        chatInput.setPadding(0, 0, 0, 0);
        chatInput.setVisibility(EditText.GONE);
        chatInput.addTextChangedListener(new TextWatcher() {
            @Override public void beforeTextChanged(CharSequence value, int start, int count, int after) { }
            @Override public void onTextChanged(CharSequence value, int start, int before, int count) { }
            @Override public void afterTextChanged(Editable value) {
                if (!syncingChatInput) nativeTextInputChanged(value.toString());
            }
        });
        FrameLayout.LayoutParams chatInputLayout = new FrameLayout.LayoutParams(720, 64, Gravity.BOTTOM | Gravity.CENTER_HORIZONTAL);
        chatInputLayout.bottomMargin = 24;
        rootView.addView(chatInput, chatInputLayout);
        setContentView(rootView);
        uiHandler.post(chatInputPoll);
        File dataRoot = getExternalFilesDir(null);
        File externalRoot = dataRoot == null ? null : new File(dataRoot, "goro-data");
        File fixtureRoot = new File(getFilesDir(), "goro-fixture");
        File selectedRoot = null;
        try {
            if (externalRoot != null && new File(externalRoot, "mobile-assets.json").isFile()) {
                selectedRoot = installMobileAssetsFromFile(externalRoot);
            } else if (externalRoot != null && new File(externalRoot, "data.grf").isFile()) {
                selectedRoot = externalRoot;
            } else if (assetExists("goro-mobile/mobile-assets.json")) {
                selectedRoot = installMobileAssetsFromAssets("goro-mobile");
            }
        } catch (Exception error) {
            Log.e("GoroAndroidHost", "mobile assets install failed", error);
        }
        if (selectedRoot == null) {
            extractRendererFixture(fixtureRoot);
            selectedRoot = fixtureRoot;
        }
        if (!selectedRoot.exists() && externalRoot != null) {
            selectedRoot = externalRoot;
        }
        // The generated base pack may intentionally omit the large offline
        // scenario to keep the pack reusable. Keep the standalone fallback
        // content available when the APK contains it, regardless of whether
        // the resource root came from bundled assets or an external pack.
        installBundledOfflineContent(selectedRoot);
        nativeConfigureResourceRoot(selectedRoot.getAbsolutePath());
        mountPreviouslyInstalledOverlays();
        AssetPatchWorker.enqueue(this);
    }

    private void setChatInputMode(int mode) {
        boolean active = mode != 0;
        if (chatInput == null || (chatInputActive == active && chatInputMode == mode)) return;
        chatInputMode = mode;
        chatInputActive = active;
        if (active) {
            syncingChatInput = true;
            chatInput.setText("");
            syncingChatInput = false;
            chatInput.setVisibility(EditText.VISIBLE);
            chatInput.requestFocus();
            InputMethodManager inputMethod = (InputMethodManager) getSystemService(INPUT_METHOD_SERVICE);
            if (inputMethod != null) inputMethod.showSoftInput(chatInput, InputMethodManager.SHOW_IMPLICIT);
        } else {
            InputMethodManager inputMethod = (InputMethodManager) getSystemService(INPUT_METHOD_SERVICE);
            if (inputMethod != null) inputMethod.hideSoftInputFromWindow(chatInput.getWindowToken(), 0);
            chatInput.clearFocus();
            syncingChatInput = true;
            chatInput.setText("");
            syncingChatInput = false;
            chatInput.setVisibility(EditText.GONE);
        }
    }

    private File installMobileAssetsFromFile(File sourceRoot) throws Exception {
        byte[] manifest = readFileBytes(new File(sourceRoot, "mobile-assets.json"));
        return installMobileAssets(manifest, new File(getFilesDir(), "goro-mobile-installed"), (relative, target) -> {
            copyFile(new File(sourceRoot, relative), target);
        });
    }

    private File installMobileAssetsFromAssets(String assetRoot) throws Exception {
        AssetManager assets = getAssets();
        byte[] manifest = readAssetBytes(assets, assetRoot + "/mobile-assets.json");
        return installMobileAssets(manifest, new File(getFilesDir(), "goro-mobile-installed"), (relative, target) -> {
            copyAsset(assets, assetRoot + "/" + relative, target);
        });
    }

    private void installBundledOfflineContent(File destination) {
        if (destination == null || !assetExists("offline/content.json")) return;
        File content = new File(destination, "offline/content.json");
        if (content.isFile()) return;
        try {
            File parent = content.getParentFile();
            if (parent != null) parent.mkdirs();
            copyAsset(getAssets(), "offline/content.json", content);
            Log.i("GoroAndroidHost", "offline-content path=" + content.getAbsolutePath() + " size=" + content.length());
        } catch (Exception error) {
            Log.e("GoroAndroidHost", "offline content install failed", error);
        }
    }

    private File installMobileAssets(byte[] manifestBytes, File destination, PackCopier copier) throws Exception {
        JSONObject manifest = new JSONObject(new String(manifestBytes, StandardCharsets.UTF_8));
        String profileName = manifest.optString("default_profile", "default");
        JSONArray profiles = manifest.optJSONArray("profiles");
        JSONObject profile = null;
        if (profiles != null) {
            for (int i = 0; i < profiles.length(); i++) {
                JSONObject candidate = profiles.optJSONObject(i);
                if (candidate != null && profileName.equals(candidate.optString("name"))) {
                    profile = candidate;
                    break;
                }
            }
        }
        if (profile == null) {
            throw new IOException("mobile asset profile not found: " + profileName);
        }
        String manifestHash = sha256(manifestBytes);
        File marker = new File(destination, ".installed");
        String markerValue = manifestHash + ":" + profileName;
        if (destination.isDirectory() && marker.isFile() && markerValue.equals(new String(readFileBytes(marker), StandardCharsets.UTF_8))) {
            return destination;
        }

        File parent = destination.getParentFile();
        if (parent != null) parent.mkdirs();
        File temporary = new File(parent, destination.getName() + ".tmp");
        deleteRecursive(temporary);
        temporary.mkdirs();
        JSONArray packs = manifest.optJSONArray("packs");
        JSONArray selectedPacks = profile.optJSONArray("packs");
        if (packs == null || selectedPacks == null || selectedPacks.length() == 0) {
            throw new IOException("mobile asset profile has no packs");
        }
        String preferredFormat = manifest.optString("preferred_format", "pak").toLowerCase();
        if (!preferredFormat.equals("pak") && !preferredFormat.equals("grf") && !preferredFormat.equals("gpf")) {
            throw new IOException("unsupported mobile asset format: " + preferredFormat);
        }
        for (int i = 0; i < selectedPacks.length(); i++) {
            String selectedName = selectedPacks.optString(i, "");
            JSONObject pack = findPack(packs, selectedName);
            if (pack == null) throw new IOException("profile references unknown pack: " + selectedName);
            JSONObject artifact = findArtifact(pack.optJSONArray("files"), preferredFormat);
            if (artifact == null) {
                artifact = findFallbackArtifact(pack.optJSONArray("files"), preferredFormat);
            }
            if (artifact == null && pack.has("file")) {
                artifact = new JSONObject();
                artifact.put("format", "grf");
                artifact.put("file", pack.optString("file", ""));
                artifact.put("sha256", pack.optString("sha256", ""));
            }
            if (artifact == null) throw new IOException("mobile pack has no usable artifact: " + selectedName);
            String relative = artifact.optString("file", "");
            if (!isSafeRelativePath(relative)) throw new IOException("unsafe mobile pack path: " + relative);
            File target = new File(temporary, new File(relative).getName());
            try {
                copier.copy(relative, target);
            } catch (java.io.FileNotFoundException missingPreferred) {
                if (!artifact.optString("format", "").equals(preferredFormat)) throw missingPreferred;
                JSONObject fallback = findFallbackArtifact(pack.optJSONArray("files"), preferredFormat);
                if (fallback == null) throw missingPreferred;
                relative = fallback.optString("file", "");
                if (!isSafeRelativePath(relative)) throw new IOException("unsafe mobile pack path: " + relative);
                target = new File(temporary, new File(relative).getName());
                copier.copy(relative, target);
                artifact = fallback;
            }
            String expectedHash = artifact.optString("sha256", "");
            if (!expectedHash.isEmpty() && !expectedHash.equalsIgnoreCase(sha256(target))) {
                throw new IOException("mobile pack hash mismatch: " + selectedName);
            }
        }
        File manifestTarget = new File(temporary, "mobile-assets.json");
        writeFileBytes(manifestTarget, manifestBytes);
        writeFileBytes(new File(temporary, ".installed"), markerValue.getBytes(StandardCharsets.UTF_8));
        replaceDirectoryAtomically(temporary, destination);
        return destination;
    }

    private void replaceDirectoryAtomically(File temporary, File destination) throws IOException {
        File parent = destination.getParentFile();
        if (parent != null && !parent.exists() && !parent.mkdirs() && !parent.isDirectory()) {
            throw new IOException("cannot create mobile asset install parent");
        }
        File backup = new File(parent, destination.getName() + ".previous");
        deleteRecursive(backup);
        boolean movedPrevious = false;
        if (destination.exists()) {
            if (!destination.renameTo(backup)) throw new IOException("cannot stage previous mobile asset install");
            movedPrevious = true;
        }
        if (!temporary.renameTo(destination)) {
            if (movedPrevious && !backup.renameTo(destination)) {
                throw new IOException("rename mobile asset install failed and previous install could not be restored");
            }
            throw new IOException("rename mobile asset install failed");
        }
        if (movedPrevious) deleteRecursive(backup);
    }

    private JSONObject findPack(JSONArray packs, String name) {
        for (int i = 0; i < packs.length(); i++) {
            JSONObject pack = packs.optJSONObject(i);
            if (pack != null && name.equals(pack.optString("name"))) return pack;
        }
        return null;
    }

    private JSONObject findArtifact(JSONArray artifacts, String format) {
        if (artifacts == null) return null;
        for (int i = 0; i < artifacts.length(); i++) {
            JSONObject artifact = artifacts.optJSONObject(i);
            if (artifact != null && format.equalsIgnoreCase(artifact.optString("format"))) return artifact;
        }
        return null;
    }

    private JSONObject findFallbackArtifact(JSONArray artifacts, String requested) {
        if (artifacts == null) return null;
        String[] order = new String[] {"pak", "grf", "gpf"};
        for (String format : order) {
            if (format.equalsIgnoreCase(requested)) continue;
            JSONObject artifact = findArtifact(artifacts, format);
            if (artifact != null) return artifact;
        }
        return null;
    }

    private void mountPreviouslyInstalledOverlays() {
        try {
            File mobileRoot = new File(getFilesDir(), "goro-mobile");
            File activePointer = new File(mobileRoot, "active-release.json");
            if (!activePointer.isFile()) return;
            JSONObject pointer = new JSONObject(new String(readFileBytes(activePointer), StandardCharsets.UTF_8));
            String releaseName = pointer.optString("release", "");
            if (!releaseName.matches("[A-Za-z0-9_-]+")) throw new IOException("unsafe active release name");
            File release = new File(mobileRoot, "releases/" + releaseName);
            String stateName = pointer.optString("state", "state.json");
            String normalizedStateName = stateName.replace('\\', '/');
            if (!isSafeRelativePath(stateName) || normalizedStateName.contains("/")) throw new IOException("unsafe active release state");
            File stateFile = new File(release, stateName);
            if (!stateFile.isFile()) return;
            JSONObject stateManifest = new JSONObject(new String(readFileBytes(stateFile), StandardCharsets.UTF_8));
            JSONArray statePacks = stateManifest.optJSONArray("packs");
            if (statePacks == null) throw new IOException("active asset release has no packs");
            File overlays = new File(release, "overlays");
            File[] files = overlays.listFiles();
            if (files == null) {
                nativeBeginAssetRelease(releaseName, 0);
                return;
            }
            ArrayList<File> artifacts = new ArrayList<>();
            for (File file : files) {
                if (!file.isFile() || file.getName().startsWith(".") || file.getName().endsWith(".part") || file.getName().endsWith(".meta")) continue;
                artifacts.add(file);
            }
            artifacts.sort((left, right) -> left.getName().compareTo(right.getName()));
            ArrayList<JSONObject> metadataEntries = new ArrayList<>();
            for (File file : artifacts) {
                File metadata = new File(file.getPath() + ".meta");
                if (!metadata.isFile()) throw new IOException("active asset overlay metadata is missing: " + file.getName());
                JSONObject value = new JSONObject(new String(readFileBytes(metadata), StandardCharsets.UTF_8));
                String name = value.optString("name", "");
                String format = value.optString("format", "").toLowerCase(java.util.Locale.ROOT);
                String expectedHash = value.optString("sha256", "");
                JSONObject pack = findPack(statePacks, name);
                JSONObject artifact = pack == null ? null : findArtifact(pack.optJSONArray("artifacts"), format);
                if (pack == null || "base".equalsIgnoreCase(pack.optString("role")) || artifact == null || expectedHash.isEmpty()
                        || !expectedHash.equalsIgnoreCase(artifact.optString("sha256", ""))
                        || !expectedHash.equalsIgnoreCase(sha256(file))) {
                    throw new IOException("active asset overlay failed integrity check: " + file.getName());
                }
                metadataEntries.add(value);
            }
            nativeBeginAssetRelease(releaseName, metadataEntries.size());
            for (int i = 0; i < metadataEntries.size(); i++) {
                JSONObject value = metadataEntries.get(i);
                nativeMountAssetOverlay(value.getString("name"), value.getString("format"), artifacts.get(i).getAbsolutePath(), value.optInt("priority", 0));
            }
        } catch (Exception error) {
            Log.w("GoroAndroidHost", "skipping active asset release", error);
        }
    }

    private boolean isSafeRelativePath(String path) {
        if (path == null || path.isEmpty()) return false;
        String normalized = path.replace('\\', '/');
        return !normalized.startsWith("/") && !normalized.contains("../") && !normalized.equals("..");
    }

    private boolean assetExists(String path) {
        try (InputStream input = getAssets().open(path, AssetManager.ACCESS_STREAMING)) {
            return true;
        } catch (Exception error) {
            return false;
        }
    }

    private byte[] readAssetBytes(AssetManager assets, String path) throws IOException {
        try (InputStream input = assets.open(path, AssetManager.ACCESS_STREAMING)) {
            return readBytes(input);
        }
    }

    private byte[] readFileBytes(File file) throws IOException {
        try (InputStream input = new FileInputStream(file)) {
            return readBytes(input);
        }
    }

    private byte[] readBytes(InputStream input) throws IOException {
        ByteArrayOutputStream output = new ByteArrayOutputStream();
        byte[] buffer = new byte[8192];
        int read;
        while ((read = input.read(buffer)) >= 0) output.write(buffer, 0, read);
        return output.toByteArray();
    }

    private void copyAsset(AssetManager assets, String path, File target) throws IOException {
        try (InputStream input = assets.open(path, AssetManager.ACCESS_STREAMING); FileOutputStream output = new FileOutputStream(target)) {
            copy(input, output);
        }
    }

    private void copyFile(File source, File target) throws IOException {
        try (InputStream input = new FileInputStream(source); FileOutputStream output = new FileOutputStream(target)) {
            copy(input, output);
        }
    }

    private void copy(InputStream input, FileOutputStream output) throws IOException {
        byte[] buffer = new byte[8192];
        int read;
        while ((read = input.read(buffer)) >= 0) output.write(buffer, 0, read);
    }

    private void writeFileBytes(File file, byte[] data) throws IOException {
        File parent = file.getParentFile();
        if (parent != null) parent.mkdirs();
        try (FileOutputStream output = new FileOutputStream(file)) {
            output.write(data);
        }
    }

    private void deleteRecursive(File file) {
        if (!file.exists()) return;
        if (file.isDirectory()) {
            File[] children = file.listFiles();
            if (children != null) for (File child : children) deleteRecursive(child);
        }
        file.delete();
    }

    private String sha256(byte[] data) {
        try {
            MessageDigest digest = MessageDigest.getInstance("SHA-256");
            return hex(digest.digest(data));
        } catch (Exception error) {
            return "unavailable";
        }
    }

    private interface PackCopier { void copy(String relative, File target) throws Exception; }

    private String hex(byte[] data) {
        StringBuilder result = new StringBuilder();
        for (byte value : data) result.append(String.format("%02x", value));
        return result.toString();
    }

    private void extractRendererFixture(File destination) {
        File archive = new File(destination, "data.pak");
        try {
            AssetManager assets = getAssets();
            destination.mkdirs();
            boolean pakInstalled = false;
            if (assetExists("goro-fixture/data.pak")) {
                try {
                    if (!archive.isFile() || archive.length() == 0) {
                        copyAsset(assets, "goro-fixture/data.pak", archive);
                    }
                    pakInstalled = archive.isFile() && archive.length() > 0;
                    if (pakInstalled) {
                        Log.i("GoroAndroidHost", "pak path=" + archive.getAbsolutePath() + " size=" + archive.length());
                    }
                } catch (Exception error) {
                    Log.e("GoroAndroidHost", "bundled pak install failed; trying GRF fallback", error);
                }
            }

            if (!pakInstalled) {
                archive = new File(destination, "renderer-fixture.grf");
                InputStream input = assets.open("goro-fixture/renderer-fixture.grf", AssetManager.ACCESS_STREAMING);
                FileOutputStream output = new FileOutputStream(archive);
                byte[] buffer = new byte[8192];
                int total = 0;
                int read;
                while ((read = input.read(buffer)) >= 0) {
                    output.write(buffer, 0, read);
                    total += read;
                }
                output.close();
                input.close();
                Log.i("GoroAndroidHost", "fixture path=" + archive.getAbsolutePath() + " size=" + total + " sha256=" + sha256(archive));
            }

            if (assetExists("offline/content.json")) {
                File content = new File(destination, "offline/content.json");
                content.getParentFile().mkdirs();
                copyAsset(assets, "offline/content.json", content);
                Log.i("GoroAndroidHost", "offline-content path=" + content.getAbsolutePath() + " size=" + content.length());
            }

            // Keep the fixture-local config beside the extracted resources so
            // the embedded Go config loader can select the requested desktop
            // or mobile presentation mode during test installs.
            if (assetExists("goro-fixture/goro/goro.ini")) {
                File config = new File(destination, "goro/goro.ini");
                if (!config.isFile()) {
                    config.getParentFile().mkdirs();
                    copyAsset(assets, "goro-fixture/goro/goro.ini", config);
                    Log.i("GoroAndroidHost", "fixture-config path=" + config.getAbsolutePath());
                }
            }
        } catch (Exception error) {
            Log.i("GoroAndroidHost", "fixture status=absent reason=" + error.getClass().getSimpleName());
        }
    }

    private String sha256(File file) {
        try {
            MessageDigest digest = MessageDigest.getInstance("SHA-256");
            FileInputStream input = new FileInputStream(file);
            byte[] buffer = new byte[8192];
            int read;
            while ((read = input.read(buffer)) >= 0) digest.update(buffer, 0, read);
            input.close();
            StringBuilder result = new StringBuilder();
            for (byte value : digest.digest()) result.append(String.format("%02x", value));
            return result.toString();
        } catch (Exception error) {
            return "unavailable";
        }
    }

    @Override protected void onDestroy() {
        uiHandler.removeCallbacks(chatInputPoll);
        unregisterReceiver(assetPatchReceiver);
        nativeShutdown();
        super.onDestroy();
    }

    @Override public void onConfigurationChanged(Configuration configuration) {
        super.onConfigurationChanged(configuration);
        if (surfaceView != null) {
            surfaceView.post(() -> surfaceView.requestApplyInsets());
        }
    }

    @Override public void onBackPressed() {
        // The Go presentation owns modal priority. The first back action in
        // profile name entry dismisses the custom keyboard; subsequent backs
        // walk the editor/profile stack instead of terminating the activity.
        nativeBack();
    }

    @Override protected void onPause() {
        nativePause();
        super.onPause();
    }

    @Override protected void onResume() {
        super.onResume();
        nativeResume();
    }

    private native void nativeSurfaceCreated(android.view.Surface surface);
    private native void nativeSurfaceChanged(int width, int height);
    private native void nativeSurfaceInsetsChanged(int left, int top, int right, int bottom);
    private native void nativeSurfaceDestroyed();
    private native void nativeTouch(int action, int pointerId, float x, float y, boolean pressed);
    private native void nativeBack();
    private native int nativeTextInputMode();
    private native void nativeTextInputChanged(String text);
    private native void nativeShutdown();
    private native void nativePause();
    private native void nativeResume();
    private native void nativeConfigureResourceRoot(String path);
    private native void nativeMountAssetOverlay(String name, String format, String path, int priority);
    private native void nativeBeginAssetRelease(String release, int overlayCount);

    private final class HostSurfaceView extends SurfaceView implements SurfaceHolder.Callback {
        HostSurfaceView() { super(MainActivity.this); getHolder().addCallback(this); setFocusable(true); }

        @Override public void surfaceCreated(SurfaceHolder holder) { nativeSurfaceCreated(holder.getSurface()); }
        @Override public void surfaceChanged(SurfaceHolder holder, int format, int width, int height) { nativeSurfaceChanged(width, height); }
        @Override public void surfaceDestroyed(SurfaceHolder holder) { nativeSurfaceDestroyed(); }

        @Override public boolean onTouchEvent(MotionEvent event) {
            final int action = event.getActionMasked();
            if (action == MotionEvent.ACTION_CANCEL) {
                for (int i = 0; i < event.getPointerCount(); i++) {
                    nativeTouch(action, event.getPointerId(i), event.getX(i), event.getY(i), false);
                }
                return true;
            }
            if (action == MotionEvent.ACTION_MOVE) {
                for (int i = 0; i < event.getPointerCount(); i++) {
                    nativeTouch(action, event.getPointerId(i), event.getX(i), event.getY(i), true);
                }
                return true;
            }
            final int index = event.getActionIndex();
            final boolean pressed = action == MotionEvent.ACTION_DOWN || action == MotionEvent.ACTION_POINTER_DOWN;
            final boolean released = action == MotionEvent.ACTION_UP || action == MotionEvent.ACTION_POINTER_UP;
            if (pressed || released) {
                nativeTouch(action, event.getPointerId(index), event.getX(index), event.getY(index), pressed);
            }
            return true;
        }
    }
}
