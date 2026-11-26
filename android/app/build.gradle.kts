import java.time.ZoneId
import java.time.ZonedDateTime
import java.time.format.DateTimeFormatter

plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.android")
}

val buildTime = ZonedDateTime.now(ZoneId.of("Asia/Shanghai"))
val autoVersionCode = (buildTime.year - 2020) * 100_000_000 +
    buildTime.monthValue * 1_000_000 +
    buildTime.dayOfMonth * 10_000 +
    buildTime.hour * 100 +
    buildTime.minute
val autoVersionName = buildTime.format(DateTimeFormatter.ofPattern("yyyy.MM.dd-HHmm"))

android {
    namespace = "com.growthbuddy.child"
    compileSdk = 34

    defaultConfig {
        applicationId = "com.growthbuddy.child"
        minSdk = 24
        targetSdk = 34
        versionCode = autoVersionCode
        versionName = autoVersionName
        setProperty("archivesBaseName", "GrowthBuddyChild-$autoVersionName")
        testInstrumentationRunner = "androidx.test.runner.AndroidJUnitRunner"
        val arm64Only = (project.findProperty("arm64Only") as String?)?.toBoolean() ?: false
        ndk {
            abiFilters += if (arm64Only) listOf("arm64-v8a") else listOf("arm64-v8a", "armeabi-v7a")
        }
    }

    packaging {
        jniLibs {
            useLegacyPackaging = true
        }
    }

    buildTypes {
        release {
            isMinifyEnabled = true
            isShrinkResources = true
            signingConfig = signingConfigs.getByName("debug")
            proguardFiles(
                getDefaultProguardFile("proguard-android-optimize.txt"),
                "proguard-rules.pro"
            )
        }
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }
    kotlinOptions {
        jvmTarget = "17"
    }
    buildFeatures {
        viewBinding = true
        buildConfig = true
    }
    lint {
        checkReleaseBuilds = false
        abortOnError = false
    }
}

dependencies {
    implementation("androidx.core:core-ktx:1.13.1")
    implementation("androidx.appcompat:appcompat:1.7.0")
    implementation("com.google.android.material:material:1.12.0")
    implementation("androidx.activity:activity-ktx:1.9.1")
    implementation("androidx.constraintlayout:constraintlayout:2.1.4")
    implementation("androidx.swiperefreshlayout:swiperefreshlayout:1.1.0")

    // CameraX
    val cameraxVersion = "1.3.4"
    implementation("androidx.camera:camera-core:$cameraxVersion")
    implementation("androidx.camera:camera-camera2:$cameraxVersion")
    implementation("androidx.camera:camera-lifecycle:$cameraxVersion")
    implementation("androidx.camera:camera-view:$cameraxVersion")

    // Google ML Kit Pose Detection (模型内置在 APK 中，离线可用)
    implementation("com.google.mlkit:pose-detection:18.0.0-beta5")

    // ZXing Core (离线生成与解析好友二维码，无 GMS 依赖)
    implementation("com.google.zxing:core:3.5.3")

    testImplementation("junit:junit:4.13.2")
}
