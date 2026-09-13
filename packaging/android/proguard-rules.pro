# Godot looks up Java classes and methods through JNI and plugin metadata.
-keep class org.godotengine.godot.** { *; }
-keep class cat.kenny.taoot.** { *; }
-keepclasseswithmembernames,includedescriptorclasses class * {
    native <methods>;
}
