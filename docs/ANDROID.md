# Android

The Android package name is `cat.kenny.taoot`. The APK targets ARM64 devices with OpenGL ES 3 support. It uses the same engine, save format, and mods as the desktop player. CI currently produces a development-signed APK for sideloading, not a Play Store release.

Choose a prepared folder containing `cd1` and `cd2` using Android's document picker. The player copies the selected game files into app storage. It does not need broad storage permission. Mods can be imported with the same picker after extracting their ZIP. The save menu imports and exports `.ti` files through Android document access.

Tap objects and dialogue choices directly. Drag a finger to operate draggable puzzles. Tap the joystick's center for Door/Space, tap an edge for one directional step, or drag to steer. Holding a direction waits briefly before repeating. Release it to stop. In portrait, the game and controls sit at the bottom of the screen for easier reach, with the joystick below the picture. In landscape, the joystick sits on the left and Door, Skip, Menu, and Keys sit on the right. Keys opens a keyboard that also works without a physical keyboard. Rotation changes the layout without restarting the game.

Android gamepads use Godot's native input. The left stick or D-pad moves through rooms, dialogue replies and menus. A confirms the selected control or clicks and drags; B goes back or skips speech. L2 and R2 cycle clickable targets. The right stick moves the pointer, L1 slows it, Y opens the keyboard, and Start opens the menu. Save names and text puzzles can be entered with the controller alone. Touch controls hide while a gamepad is connected and return when it disconnects.

App updates keep saves, but uninstalling removes app storage. Export saves before uninstalling. Local and CI APKs use the shared debug key in `packaging/android/debug.keystore`, so they can update one another. Production releases need a separate private signing key.
