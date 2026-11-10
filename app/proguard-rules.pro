# Reglas de R8 para la variante «rendimiento».

# gomobile: el código nativo de Go llama a estas clases de Java por su nombre (JNI FindClass/GetMethodID) y Java llama a los
# métodos nativos por su nombre. Nada de esto se puede renombrar, quitar ni reducir.
-keep class go.** { *; }
-keep class wabot.** { *; }
-keepclasseswithmembernames class * { native <methods>; }
-keepclassmembers class * implements go.Seq$Proxy { *; }

# Interfaces que implementa la app para que Go las llame (observador de cambios).
-keep class * implements wabot.Observador { *; }

# Media3 (ExoPlayer): trae sus propias reglas; estas son por si alguna clase se carga por nombre.
-keep class androidx.media3.** { *; }
-dontwarn androidx.media3.**

# libphonenumber carga sus metadatos como recursos del jar por nombre de paquete.
-keep class com.google.i18n.phonenumbers.** { *; }
-keepclassmembers class com.google.i18n.phonenumbers.** { *; }
-dontwarn com.google.i18n.phonenumbers.**

# Servicios y receptores de la app (los declara el manifiesto, pero que queden con su nombre en los reportes de errores).
-keepnames class com.botchat.app.** { *; }

# Útil para leer errores: números de línea.
-keepattributes SourceFile,LineNumberTable
-renamesourcefileattribute SourceFile
