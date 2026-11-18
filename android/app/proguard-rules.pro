-keepattributes *Annotation*,InnerClasses,Signature,EnclosingMethod,SourceFile,LineNumberTable

# Keep all BotHub app classes & members intact while allowing R8 to strip ~9MB of unused Material Icons & dead library code
-keep class uk.starryheavens.vesper.bothub.** { *; }

# Keep AndroidX Security Crypto + Google Tink (reflection-instantiated primitives in EncryptedSharedPreferences)
-keep class androidx.security.crypto.** { *; }
-keep class com.google.crypto.tink.** { *; }
-dontwarn com.google.crypto.tink.**
-dontwarn javax.annotation.**
-dontwarn com.google.errorprone.annotations.**

# Keep AndroidX Credentials / Passkey / Play Services Auth / Biometric / WorkManager entrypoints
-keep class androidx.credentials.** { *; }
-keep class androidx.biometric.** { *; }
-keep class * extends androidx.work.ListenableWorker { *; }

# OkHttp / Okio optional platform providers
-dontwarn okhttp3.**
-dontwarn okio.**
-dontwarn org.conscrypt.**
-dontwarn org.bouncycastle.**
-dontwarn org.openjsse.**
