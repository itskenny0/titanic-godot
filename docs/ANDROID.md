# Android

The Android package name is `cat.kenny.taoot`. The APK targets ARM64 devices with OpenGL ES 3 support. It uses the same engine, save format, and mods as the desktop player. CI currently produces a development-signed APK for sideloading, not a Play Store release.

Choose a prepared folder containing `cd1` and `cd2` using Android's document picker. The player copies the selected game files into app storage. It does not need broad storage permission. Mods can be imported with the same picker after extracting their ZIP. The save menu imports and exports `.ti` files through Android document access.

Tap objects and dialogue choices directly. Drag a finger to operate draggable puzzles. Tap the joystick's center for Door/Space, tap an edge for one directional step, or drag to steer. Holding a direction waits briefly before repeating. Release it to stop. In portrait, the game and controls sit at the bottom of the screen for easier reach, with the joystick below the picture. In landscape, the joystick sits on the left and Door, Skip, Menu, and Keys sit on the right. Keys opens a keyboard that also works without a physical keyboard. Rotation changes the layout without restarting the game.

Android gamepads use Godot's native input. The left stick moves, the right stick moves the pointer, A clicks and drags, L1 slows the pointer, and Start opens the menu. Touch controls hide while a gamepad is connected and return when it disconnects.

App updates keep saves, but uninstalling removes app storage. Export saves before uninstalling. Development signing keys are local to a build environment; builds with different keys require uninstalling the previous APK. Configure a persistent signing key before distributing updates to players.
