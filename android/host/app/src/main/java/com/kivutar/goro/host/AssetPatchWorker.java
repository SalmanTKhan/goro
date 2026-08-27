package com.kivutar.goro.host;

import android.app.NotificationChannel;
import android.app.NotificationManager;
import android.content.Context;
import android.content.Intent;
import android.os.Build;
import android.util.Base64;
import androidx.annotation.NonNull;
import androidx.core.app.NotificationCompat;
import androidx.work.Constraints;
import androidx.work.Data;
import androidx.work.ExistingWorkPolicy;
import androidx.work.ExistingPeriodicWorkPolicy;
import androidx.work.NetworkType;
import androidx.work.OneTimeWorkRequest;
import androidx.work.PeriodicWorkRequest;
import androidx.work.WorkManager;
import androidx.work.WorkerParameters;
import java.io.BufferedInputStream;
import java.io.File;
import java.io.FileInputStream;
import java.io.FileOutputStream;
import java.io.IOException;
import java.io.InputStream;
import java.net.HttpURLConnection;
import java.net.URL;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.StandardCopyOption;
import java.security.KeyFactory;
import java.security.MessageDigest;
import java.security.PublicKey;
import java.security.Signature;
import java.security.spec.X509EncodedKeySpec;
import java.util.ArrayList;
import java.util.HashMap;
import java.util.HashSet;
import java.util.Locale;
import java.util.Set;
import org.json.JSONArray;
import org.json.JSONObject;

/** Downloads signed, whole-artifact mobile patches without invoking a patcher. */
public final class AssetPatchWorker extends androidx.work.Worker {
    private static final String WORK_NAME = "goro-mobile-asset-patches";
    private static final String CHANNEL_ID = "goro-mobile-assets";
    private static final String PERIODIC_WORK_NAME = "goro-mobile-asset-poll";
    public static final String ACTION_PATCH_READY = "com.kivutar.goro.host.ASSET_PATCH_READY";
    public static final String ACTION_PATCH_RELEASE_READY = "com.kivutar.goro.host.ASSET_PATCH_RELEASE_READY";
    private static final int NOTIFICATION_ID = 4107;

    private static final class PendingOverlay {
        final String name;
        final String format;
        final String path;
        final int priority;

        PendingOverlay(String name, String format, String path, int priority) {
            this.name = name;
            this.format = format;
            this.path = path;
            this.priority = priority;
        }
    }

    public AssetPatchWorker(@NonNull Context context, @NonNull WorkerParameters parameters) {
        super(context, parameters);
    }

    public static void enqueue(Context context) {
        if (BuildConfig.MOBILE_ASSET_PATCH_MANIFEST_URL == null || BuildConfig.MOBILE_ASSET_PATCH_MANIFEST_URL.trim().isEmpty()) return;
        NetworkType network = "connected".equalsIgnoreCase(BuildConfig.MOBILE_ASSET_PATCH_NETWORK_POLICY)
                ? NetworkType.CONNECTED : NetworkType.UNMETERED;
        Constraints constraints = new Constraints.Builder().setRequiredNetworkType(network).build();
        OneTimeWorkRequest request = new OneTimeWorkRequest.Builder(AssetPatchWorker.class)
                .setConstraints(constraints).build();
        WorkManager manager = WorkManager.getInstance(context.getApplicationContext());
        manager.enqueueUniqueWork(WORK_NAME, ExistingWorkPolicy.KEEP, request);
        PeriodicWorkRequest poll = new PeriodicWorkRequest.Builder(AssetPatchWorker.class, 15, java.util.concurrent.TimeUnit.MINUTES)
                .setConstraints(constraints).build();
        manager.enqueueUniquePeriodicWork(PERIODIC_WORK_NAME, ExistingPeriodicWorkPolicy.KEEP, poll);
    }

    @NonNull @Override public Result doWork() {
        String manifestURL = BuildConfig.MOBILE_ASSET_PATCH_MANIFEST_URL;
        if (manifestURL == null || manifestURL.trim().isEmpty()) return Result.success();
        try {
            createNotificationChannel();
            byte[] manifestBytes = downloadBytes(new URL(manifestURL), 16L * 1024L * 1024L);
            JSONObject manifest = new JSONObject(new String(manifestBytes, StandardCharsets.UTF_8));
            verifyManifest(manifestBytes, manifest, new URL(manifestURL + ".sig"));
            if (!"goro-mobile-delivery".equals(manifest.optString("format")) || manifest.optInt("version", 0) != 1) {
                throw new IOException("unsupported mobile delivery manifest");
            }
            String release = safeName(manifest.optString("release", "local"));
            File releaseDir = new File(getApplicationContext().getFilesDir(), "goro-mobile/releases/" + release);
            File overlayDir = new File(releaseDir, "overlays");
            if (!overlayDir.mkdirs() && !overlayDir.isDirectory()) throw new IOException("cannot create asset overlay directory");
            JSONArray packs = manifest.optJSONArray("packs");
            if (packs == null) throw new IOException("delivery manifest has no packs");
            HashMap<String, JSONObject> byName = new HashMap<>();
            for (int i = 0; i < packs.length(); i++) {
                JSONObject pack = packs.optJSONObject(i);
                if (pack != null) byName.put(pack.optString("name", ""), pack);
            }
            Set<String> done = new HashSet<>();
            Set<String> active = new HashSet<>();
            ArrayList<PendingOverlay> pending = new ArrayList<>();
            for (int i = 0; i < packs.length(); i++) {
                JSONObject pack = packs.optJSONObject(i);
                if (pack != null) downloadPack(pack, byName, manifest, manifestURL, overlayDir, done, active, pending);
            }
            writeAtomic(new File(releaseDir, "state.json"), manifestBytes);
            sendReleaseReady(release, pending.size());
            for (PendingOverlay overlay : pending) sendReady(overlay);
            notifyProgress("Asset patches ready", 100, true);
            return Result.success();
        } catch (Exception error) {
            notifyProgress("Asset patch retry: " + error.getMessage(), 0, false);
            return Result.retry();
        }
    }

    private void downloadPack(JSONObject pack, HashMap<String, JSONObject> byName, JSONObject manifest,
            String manifestURL, File overlayDir, Set<String> done, Set<String> active,
            ArrayList<PendingOverlay> pending) throws Exception {
        String name = pack.optString("name", "");
        if (name.isEmpty() || done.contains(name) || "base".equalsIgnoreCase(pack.optString("role"))) return;
        if (!active.add(name)) throw new IOException("delivery dependency cycle at " + name);
        JSONArray dependencies = pack.optJSONArray("dependencies");
        if (dependencies != null) {
            for (int i = 0; i < dependencies.length(); i++) {
                JSONObject dependency = byName.get(dependencies.optString(i, ""));
                if (dependency == null) throw new IOException("missing delivery dependency " + dependencies.optString(i));
                downloadPack(dependency, byName, manifest, manifestURL, overlayDir, done, active, pending);
            }
        }
        JSONObject artifact = selectArtifact(pack, pack.optJSONArray("artifacts"));
        if (artifact == null) throw new IOException("no usable artifact for " + name);
        String format = artifact.optString("format", "").toLowerCase(Locale.ROOT);
        File target = new File(overlayDir, safeName(name) + "." + safeName(format));
        String expectedHash = artifact.optString("sha256", "");
        if (expectedHash.isEmpty()) throw new IOException("artifact hash is missing for " + name);
        if (!target.isFile() || !expectedHash.equalsIgnoreCase(sha256(target))) {
            File part = new File(target.getPath() + ".part");
            URL artifactURL = resolveURL(manifest, manifestURL, artifact.optString("url", ""));
            downloadResumable(artifactURL, part, expectedHash, artifact.optLong("archive_bytes", 0));
            moveAtomic(part, target);
        }
		JSONObject metadata = new JSONObject();
		metadata.put("name", name);
		metadata.put("format", format);
		metadata.put("sha256", expectedHash);
		metadata.put("archive_bytes", artifact.optLong("archive_bytes", 0));
		metadata.put("priority", pack.optInt("download_priority", 0));
		writeAtomic(new File(target.getPath() + ".meta"), metadata.toString().getBytes(StandardCharsets.UTF_8));
        pending.add(new PendingOverlay(name, format, target.getAbsolutePath(), pack.optInt("download_priority", 0)));
        done.add(name);
        active.remove(name);
        notifyProgress("Downloaded " + name + " (" + format + ")", 0, false);
    }

    private void sendReady(PendingOverlay overlay) {
        // The Go side performs format-specific validation and activation. Do
        // not let a malformed file become visible merely because its hash is
        // correct.
        Intent ready = new Intent(ACTION_PATCH_READY);
        ready.setPackage(getApplicationContext().getPackageName());
        ready.putExtra("name", overlay.name);
        ready.putExtra("format", overlay.format);
        ready.putExtra("path", overlay.path);
        ready.putExtra("priority", overlay.priority);
        getApplicationContext().sendBroadcast(ready);
    }

    private void sendReleaseReady(String release, int overlayCount) {
        Intent ready = new Intent(ACTION_PATCH_RELEASE_READY);
        ready.setPackage(getApplicationContext().getPackageName());
        ready.putExtra("release", release);
        ready.putExtra("overlay_count", overlayCount);
        getApplicationContext().sendBroadcast(ready);
    }

    private JSONObject selectArtifact(JSONObject pack, JSONArray artifacts) throws IOException {
        if (artifacts == null) return null;
        String requested = BuildConfig.MOBILE_ASSET_PATCH_FORMAT.toLowerCase(Locale.ROOT);
        String preferred = "";
        String[] order = new String[]{"pak", "grf", "gpf", "thor", "rgz"};
        // The worker receives the pack preference through the manifest. The
        // caller uses auto when no explicit command-line format was supplied.
        if ("auto".equals(requested)) {
			String packPreference = pack.optString("preferred_format", "").toLowerCase(Locale.ROOT);
			if (!packPreference.isEmpty()) {
				JSONObject preferredArtifact = findArtifact(artifacts, packPreference);
				if (preferredArtifact != null) return preferredArtifact;
			}
            for (int i = 0; i < artifacts.length(); i++) {
                JSONObject artifact = artifacts.optJSONObject(i);
                if (artifact != null && "pak".equalsIgnoreCase(artifact.optString("format"))) return artifact;
            }
            for (String candidate : order) {
                JSONObject artifact = findArtifact(artifacts, candidate);
                if (artifact != null) return artifact;
            }
            return null;
        }
        if (!"pak".equals(requested) && !"grf".equals(requested) && !"gpf".equals(requested)
                && !"thor".equals(requested) && !"rgz".equals(requested)) throw new IOException("unsupported patch format " + requested);
        JSONObject selected = findArtifact(artifacts, requested);
        if (selected != null) return selected;
        return null;
    }

    private JSONObject findArtifact(JSONArray artifacts, String format) {
        for (int i = 0; i < artifacts.length(); i++) {
            JSONObject artifact = artifacts.optJSONObject(i);
            if (artifact != null && format.equalsIgnoreCase(artifact.optString("format"))) return artifact;
        }
        return null;
    }

    private URL resolveURL(JSONObject manifest, String manifestURL, String artifactURL) throws Exception {
        if (artifactURL.startsWith("https://") || artifactURL.startsWith("http://")) return new URL(artifactURL);
        String base = manifest.optString("base_url", "");
        URL parent = base.isEmpty() ? new URL(manifestURL) : new URL(base.endsWith("/") ? base : base + "/");
        return new URL(parent, artifactURL);
    }

    private void verifyManifest(byte[] bytes, JSONObject manifest, URL signatureURL) throws Exception {
        String configuredKey = BuildConfig.MOBILE_ASSET_PATCH_PUBLIC_KEY;
        if (configuredKey == null || configuredKey.trim().isEmpty()) {
            throw new IOException("delivery public key is not configured in the APK");
        }
        String encodedKey = configuredKey;
        byte[] signatureBytes = downloadBytes(signatureURL, 1024 * 1024);
        String signatureText = new String(signatureBytes, StandardCharsets.UTF_8).trim();
        byte[] signature = Base64.decode(signatureText, Base64.DEFAULT);
        byte[] publicKeyBytes = Base64.decode(encodedKey, Base64.DEFAULT);
        PublicKey publicKey = KeyFactory.getInstance("Ed25519").generatePublic(new X509EncodedKeySpec(publicKeyBytes));
        Signature verifier = Signature.getInstance("Ed25519");
        verifier.initVerify(publicKey);
        verifier.update(bytes);
        if (!verifier.verify(signature)) throw new IOException("delivery manifest signature mismatch");
    }

    private byte[] downloadBytes(URL url, long maxBytes) throws Exception {
        HttpURLConnection connection = open(url, 0);
        try (InputStream input = new BufferedInputStream(connection.getInputStream())) {
            java.io.ByteArrayOutputStream output = new java.io.ByteArrayOutputStream();
            byte[] buffer = new byte[8192];
            int total = 0;
            int read;
            while ((read = input.read(buffer)) >= 0) {
                total += read;
                if (maxBytes > 0 && total > maxBytes) throw new IOException("download exceeds limit");
                output.write(buffer, 0, read);
            }
            return output.toByteArray();
        } finally {
            connection.disconnect();
        }
    }

    private void downloadResumable(URL url, File part, String expectedHash, long expectedBytes) throws Exception {
        long existing = part.isFile() ? part.length() : 0;
        HttpURLConnection connection = open(url, existing);
        boolean append = existing > 0 && connection.getResponseCode() == HttpURLConnection.HTTP_PARTIAL;
        if (existing > 0 && !append) {
            connection.disconnect();
            existing = 0;
            connection = open(url, 0);
        }
        try (InputStream input = new BufferedInputStream(connection.getInputStream());
                FileOutputStream output = new FileOutputStream(part, append)) {
            byte[] buffer = new byte[1024 * 1024];
            int read;
            long total = existing;
            while ((read = input.read(buffer)) >= 0) {
                output.write(buffer, 0, read);
                total += read;
                setProgressAsync(new Data.Builder().putString("pack", part.getName()).putLong("bytes", total).build());
            }
            output.getFD().sync();
            if (expectedBytes > 0 && total != expectedBytes) throw new IOException("download length mismatch");
        } finally {
            connection.disconnect();
        }
        if (!expectedHash.isEmpty() && !expectedHash.equalsIgnoreCase(sha256(part))) throw new IOException("artifact hash mismatch");
    }

    private HttpURLConnection open(URL url, long range) throws IOException {
        if (!"https".equalsIgnoreCase(url.getProtocol())) throw new IOException("asset delivery requires HTTPS");
        HttpURLConnection connection = (HttpURLConnection) url.openConnection();
        connection.setConnectTimeout(15000);
        connection.setReadTimeout(60000);
        connection.setInstanceFollowRedirects(false);
        if (range > 0) connection.setRequestProperty("Range", "bytes=" + range + "-");
        int status = connection.getResponseCode();
        if (status < 200 || status >= 300) {
            connection.disconnect();
            throw new IOException("asset server returned HTTP " + status);
        }
        return connection;
    }

    private String sha256(File file) throws Exception {
        MessageDigest digest = MessageDigest.getInstance("SHA-256");
        try (InputStream input = new BufferedInputStream(new FileInputStream(file))) {
            byte[] buffer = new byte[1024 * 1024];
            int read;
            while ((read = input.read(buffer)) >= 0) digest.update(buffer, 0, read);
        }
        StringBuilder result = new StringBuilder();
        for (byte value : digest.digest()) result.append(String.format(Locale.ROOT, "%02x", value));
        return result.toString();
    }

    private String safeName(String value) throws IOException {
        if (value == null || value.trim().isEmpty()) throw new IOException("empty asset name");
        StringBuilder result = new StringBuilder();
        for (int i = 0; i < value.length(); i++) {
            char c = Character.toLowerCase(value.charAt(i));
            if ((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' || c == '_') result.append(c);
            else result.append('-');
        }
        return result.toString();
    }

    private void writeAtomic(File target, byte[] data) throws IOException {
        File temporary = new File(target.getPath() + ".tmp");
        try (FileOutputStream output = new FileOutputStream(temporary)) { output.write(data); output.getFD().sync(); }
        moveAtomic(temporary, target);
    }

    private void moveAtomic(File source, File target) throws IOException {
        try {
            Files.move(source.toPath(), target.toPath(), StandardCopyOption.ATOMIC_MOVE, StandardCopyOption.REPLACE_EXISTING);
        } catch (java.nio.file.AtomicMoveNotSupportedException unsupported) {
            Files.move(source.toPath(), target.toPath(), StandardCopyOption.REPLACE_EXISTING);
        }
    }

    private void createNotificationChannel() {
        if (Build.VERSION.SDK_INT < 26) return;
        NotificationManager manager = (NotificationManager) getApplicationContext().getSystemService(Context.NOTIFICATION_SERVICE);
        if (manager != null) manager.createNotificationChannel(new NotificationChannel(CHANNEL_ID, "Goro asset updates", NotificationManager.IMPORTANCE_LOW));
    }

    private void notifyProgress(String message, int progress, boolean complete) {
        NotificationManager manager = (NotificationManager) getApplicationContext().getSystemService(Context.NOTIFICATION_SERVICE);
        if (manager == null) return;
        NotificationCompat.Builder builder = new NotificationCompat.Builder(getApplicationContext(), CHANNEL_ID)
                .setSmallIcon(android.R.drawable.stat_sys_download).setContentTitle("Goro assets").setContentText(message).setOngoing(!complete);
        if (progress > 0) builder.setProgress(100, progress, false); else builder.setProgress(0, 0, true);
        manager.notify(NOTIFICATION_ID, builder.build());
    }
}
