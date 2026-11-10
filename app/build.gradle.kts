plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.plugin.compose")
}

android {
    namespace = "com.botchat.app"
    compileSdk = 37

    defaultConfig {
        applicationId = "com.botchat.app"
        minSdk = 26
        targetSdk = 36
        versionCode = 2
        versionName = "0.1.1-etapa1"
        ndk { abiFilters += "arm64-v8a" }
    }
    buildFeatures { compose = true }
    packaging { jniLibs { useLegacyPackaging = true } } // comprime la librería nativa: APK mucho más liviano
    testOptions { unitTests.isReturnDefaultValues = true }
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

    // Teléfonos y países (código abierto, Java puro, sin Google Play Services)
    implementation("com.googlecode.libphonenumber:libphonenumber:9.0.43")

    testImplementation("junit:junit:4.13.2")
}
