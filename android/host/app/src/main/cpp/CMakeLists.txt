cmake_minimum_required(VERSION 3.22.1)
project(goro_jni C)

set(GORO_ANDROID_DIR "${CMAKE_SOURCE_DIR}/../jniLibs/${ANDROID_ABI}")

add_library(goro_jni SHARED native_bridge.c)
target_include_directories(goro_jni PRIVATE "${CMAKE_SOURCE_DIR}/../../../../go")
target_link_options(goro_jni PRIVATE "-L${GORO_ANDROID_DIR}" "-Wl,-l:libgoro_android.so")
find_library(log-lib log)
find_library(android-lib android)
target_link_libraries(goro_jni ${android-lib} ${log-lib})
