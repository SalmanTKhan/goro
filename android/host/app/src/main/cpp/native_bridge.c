#include <android/native_window_jni.h>
#include <android/log.h>
#include <jni.h>

// Exported by the Go host library; keep this declaration local because the
// generated c-shared header is not part of the Android source tree.
extern void GoroAndroidMountAssetOverlay(char *name, char *format, char *path, int priority);
extern void GoroAndroidBeginAssetRelease(char *release, int overlay_count);
extern void GoroAndroidBack(void);
extern int GoroAndroidTextInputMode(void);
extern void GoroAndroidTextInput(char *text);
#include <stdint.h>

#include "goro_android.h"

#define TAG "GoroAndroidHost"

JNIEXPORT void JNICALL Java_com_kivutar_goro_host_MainActivity_nativeSurfaceCreated(
    JNIEnv *env, jobject self, jobject surface) {
    (void)self;
    ANativeWindow *window = ANativeWindow_fromSurface(env, surface);
    if (window == NULL) {
        __android_log_print(ANDROID_LOG_ERROR, TAG, "ANativeWindow_fromSurface returned null");
        return;
    }
    __android_log_print(ANDROID_LOG_INFO, TAG, "surface created width=%d height=%d; entering Go bridge",
                        ANativeWindow_getWidth(window), ANativeWindow_getHeight(window));
    GoroAndroidSurfaceCreated((uintptr_t)window, ANativeWindow_getWidth(window), ANativeWindow_getHeight(window));
    __android_log_print(ANDROID_LOG_INFO, TAG, "surface created; Go bridge returned");
}

JNIEXPORT void JNICALL Java_com_kivutar_goro_host_MainActivity_nativeSurfaceChanged(
    JNIEnv *env, jobject self, jint width, jint height) {
    (void)env; (void)self;
    __android_log_print(ANDROID_LOG_INFO, TAG, "surface changed width=%d height=%d", (int)width, (int)height);
    GoroAndroidSurfaceChanged((int)width, (int)height);
}

JNIEXPORT void JNICALL Java_com_kivutar_goro_host_MainActivity_nativeSurfaceInsetsChanged(
    JNIEnv *env, jobject self, jint left, jint top, jint right, jint bottom) {
    (void)env; (void)self;
    GoroAndroidSurfaceInsetsChanged((int)left, (int)top, (int)right, (int)bottom);
}

JNIEXPORT void JNICALL Java_com_kivutar_goro_host_MainActivity_nativeSurfaceDestroyed(
    JNIEnv *env, jobject self) {
    (void)env; (void)self;
    __android_log_print(ANDROID_LOG_INFO, TAG, "surface destroyed; entering Go bridge");
    GoroAndroidSurfaceDestroyed();
    __android_log_print(ANDROID_LOG_INFO, TAG, "surface destroyed; Go released WGPU surface");
}

JNIEXPORT void JNICALL Java_com_kivutar_goro_host_MainActivity_nativeBack(
    JNIEnv *env, jobject self) {
    (void)env; (void)self;
    GoroAndroidBack();
}

JNIEXPORT void JNICALL Java_com_kivutar_goro_host_MainActivity_nativeTouch(
    JNIEnv *env, jobject self, jint action, jint pointer_id, jfloat x, jfloat y, jboolean pressed) {
    (void)env; (void)self;
    GoroAndroidTouch((int)action, (int)pointer_id, (int)(x + 0.5f), (int)(y + 0.5f), pressed ? 1 : 0);
}

JNIEXPORT jint JNICALL Java_com_kivutar_goro_host_MainActivity_nativeTextInputMode(
    JNIEnv *env, jobject self) {
    (void)env; (void)self;
    return (jint)GoroAndroidTextInputMode();
}

JNIEXPORT void JNICALL Java_com_kivutar_goro_host_MainActivity_nativeTextInputChanged(
    JNIEnv *env, jobject self, jstring text) {
    (void)self;
    if (text == NULL) return;
    const char *value = (*env)->GetStringUTFChars(env, text, NULL);
    if (value == NULL) return;
    GoroAndroidTextInput((char *)value);
    (*env)->ReleaseStringUTFChars(env, text, value);
}

JNIEXPORT void JNICALL Java_com_kivutar_goro_host_MainActivity_nativeShutdown(
    JNIEnv *env, jobject self) {
    (void)env; (void)self;
    GoroAndroidShutdown();
}

JNIEXPORT void JNICALL Java_com_kivutar_goro_host_MainActivity_nativePause(
    JNIEnv *env, jobject self) {
    (void)env; (void)self;
    GoroAndroidPause();
}

JNIEXPORT void JNICALL Java_com_kivutar_goro_host_MainActivity_nativeResume(
    JNIEnv *env, jobject self) {
    (void)env; (void)self;
    GoroAndroidResume();
}

JNIEXPORT void JNICALL Java_com_kivutar_goro_host_MainActivity_nativeConfigureResourceRoot(
    JNIEnv *env, jobject self, jstring path) {
    (void)self;
    if (path == NULL) {
        return;
    }
    const char *value = (*env)->GetStringUTFChars(env, path, NULL);
    if (value == NULL) {
        return;
    }
    GoroAndroidSetResourceRoot((char *)value);
    (*env)->ReleaseStringUTFChars(env, path, value);
}

JNIEXPORT void JNICALL Java_com_kivutar_goro_host_MainActivity_nativeMountAssetOverlay(
    JNIEnv *env, jobject self, jstring name, jstring format, jstring path, jint priority) {
    (void)self;
    if (name == NULL || format == NULL || path == NULL) return;
    const char *name_value = (*env)->GetStringUTFChars(env, name, NULL);
    const char *format_value = (*env)->GetStringUTFChars(env, format, NULL);
    const char *path_value = (*env)->GetStringUTFChars(env, path, NULL);
    if (name_value == NULL || format_value == NULL || path_value == NULL) {
        if (name_value != NULL) (*env)->ReleaseStringUTFChars(env, name, name_value);
        if (format_value != NULL) (*env)->ReleaseStringUTFChars(env, format, format_value);
        if (path_value != NULL) (*env)->ReleaseStringUTFChars(env, path, path_value);
        return;
    }
    GoroAndroidMountAssetOverlay((char *)name_value, (char *)format_value, (char *)path_value, (int)priority);
    (*env)->ReleaseStringUTFChars(env, name, name_value);
    (*env)->ReleaseStringUTFChars(env, format, format_value);
    (*env)->ReleaseStringUTFChars(env, path, path_value);
}

JNIEXPORT void JNICALL Java_com_kivutar_goro_host_MainActivity_nativeBeginAssetRelease(
    JNIEnv *env, jobject self, jstring release, jint overlay_count) {
    (void)self;
    if (release == NULL || overlay_count < 0) return;
    const char *release_value = (*env)->GetStringUTFChars(env, release, NULL);
    if (release_value == NULL) return;
    GoroAndroidBeginAssetRelease((char *)release_value, (int)overlay_count);
    (*env)->ReleaseStringUTFChars(env, release, release_value);
}
