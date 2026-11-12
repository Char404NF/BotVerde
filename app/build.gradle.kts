plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.plugin.compose")
}

android {
    lint {
        // Media3 marca partes de su API como inestables; se usan a propósito con la versión fija del proyecto.
        disable += "UnsafeOptInUsageError"
    }
    namespace = "com.botchat.app"
    compileSdk = 37

    defaultConfig {
        applicationId = "com.botchat.app"
        minSdk = 26
        targetSdk = 36
        versionCode = 10
        versionName = "0.6.0"
        ndk { abiFilters += "arm64-v8a" }
    }
    // Dos variantes del MISMO app: mismo applicationId, mismo versionCode y la misma llave de firma (la de siempre), para poder
    // instalar una encima de la otra en cualquier sentido sin desinstalar (desinstalar borraría la sesión de WhatsApp).
    //  - debug: la de siempre (con símbolos de depuración, sin optimizar).
    //  - rendimiento: no depurable, con R8 (código reducido y optimizado) y perfil base (baseline-prof.txt).
    buildTypes {
        create("rendimiento") {
            initWith(getByName("debug"))
            isDebuggable = false
            isMinifyEnabled = true
            isShrinkResources = true
            proguardFiles(getDefaultProguardFile("proguard-android-optimize.txt"), "proguard-rules.pro")
            signingConfig = signingConfigs.getByName("debug")
            matchingFallbacks += "debug"
        }
    }
    // Con -PpruebasRendimiento las pruebas unitarias corren contra la variante rendimiento (por defecto, contra debug).
    if (project.hasProperty("pruebasRendimiento")) testBuildType = "rendimiento"
    buildFeatures { compose = true }
    packaging { jniLibs { useLegacyPackaging = true } } // comprime la librería nativa: APK mucho más liviano
    testOptions {
        unitTests.isReturnDefaultValues = true
        unitTests.isIncludeAndroidResources = true // pruebas de pantallas con Robolectric
    }
    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }
}

dependencies {
    // Módulo Go (whatsmeow) generado con gomobile bind
    implementation(files("libs/wabot.aar"))

    implementation(platform("androidx.compose:compose-bom:2026.09.00"))
    implementation("androidx.compose.material3:material3")
    implementation("androidx.activity:activity-compose:1.13.0")
    implementation("androidx.lifecycle:lifecycle-viewmodel-compose:2.11.0")
    implementation("androidx.core:core-ktx:1.19.1")
    implementation("androidx.fragment:fragment-ktx:1.9.1")
    implementation("androidx.lifecycle:lifecycle-runtime-compose:2.11.0")
    implementation("androidx.navigation:navigation-compose:2.10.2")
    implementation("androidx.compose.material:material-icons-core:1.7.8")
    implementation("androidx.biometric:biometric:1.1.0")
    // Video y audio descifrados en flujo (Media3: AndroidX, sin servicios de Google)
    implementation("androidx.media3:media3-exoplayer:1.11.1")
    implementation("androidx.media3:media3-ui:1.11.1")

    // Teléfonos y países (código abierto, Java puro, sin Google Play Services)
    implementation("com.googlecode.libphonenumber:libphonenumber:9.0.43")

    testImplementation("junit:junit:4.13.2")
    testImplementation("org.json:json:20240303")
    // Pruebas de pantallas en la JVM (sin teléfono): Robolectric + pruebas de Compose
    testImplementation("org.robolectric:robolectric:4.17")
    testImplementation(platform("androidx.compose:compose-bom:2026.09.00"))
    testImplementation("androidx.compose.ui:ui-test-junit4")
    testImplementation("androidx.test:core:1.7.0")
    testImplementation("androidx.test.ext:junit:1.3.0")
    debugImplementation("androidx.compose.ui:ui-test-manifest")
    // Solo para correr las pruebas de pantallas contra la variante rendimiento (./gradlew -PpruebasRendimiento testRendimientoUnitTest); el APK no la lleva.
    if (project.hasProperty("pruebasRendimiento")) "rendimientoImplementation"("androidx.compose.ui:ui-test-manifest")
}
