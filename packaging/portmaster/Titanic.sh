#!/bin/bash
# PortMaster launcher. Native joypads are disabled in FRT; gptokeyb supplies both sticks.
# Resolve the adjacent game folder before loading firmware helpers, so even
# early startup failures reach the log. PortMaster has no get_ports_location().
launcher_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)" || exit 1
GAMEDIR="$launcher_dir/titanic"
if [[ ! -d "$GAMEDIR" ]]; then
  echo "Missing Titanic game folder beside the launcher: $GAMEDIR" >&2
  exit 1
fi
exec > >(tee "$GAMEDIR/log.txt") 2>&1
XDG_DATA_HOME=${XDG_DATA_HOME:-$HOME/.local/share}
if [[ -d /opt/system/Tools/PortMaster ]]; then
  controlfolder=/opt/system/Tools/PortMaster
elif [[ -d /opt/tools/PortMaster ]]; then
  controlfolder=/opt/tools/PortMaster
elif [[ -d "$XDG_DATA_HOME/PortMaster" ]]; then
  controlfolder="$XDG_DATA_HOME/PortMaster"
else
  controlfolder=/roms/ports/PortMaster
fi
if [[ ! -f "$controlfolder/control.txt" ]]; then
  echo "PortMaster control.txt was not found in $controlfolder"
  exit 1
fi
source "$controlfolder/control.txt" || exit 1
[[ ! -f "$controlfolder/mod_${CFW_NAME}.txt" ]] || source "$controlfolder/mod_${CFW_NAME}.txt"
get_controls
cd "$GAMEDIR" || exit 1
runtime=frt_3.5.2
godot_file="$controlfolder/libs/$runtime.squashfs"
if [[ ! -f "$godot_file" ]]; then
  $ESUDO "$controlfolder/harbourmaster" --quiet --no-check runtime_check "$runtime.squashfs"
fi
[[ -f "$godot_file" ]] || { echo "Missing PortMaster runtime: $godot_file"; exit 1; }
godot_dir="$GAMEDIR/.runtime"
$ESUDO mkdir -p "$godot_dir"
$ESUDO mount -o loop,ro "$godot_file" "$godot_dir" || exit 1
mapper_pid=
cleanup() {
  [[ -z "$mapper_pid" ]] || kill "$mapper_pid" 2>/dev/null || true
  $ESUDO umount "$godot_dir" || true
}
trap cleanup EXIT INT TERM
export FRT_NO_EXIT_SHORTCUTS=FRT_NO_EXIT_SHORTCUTS
export RETANIC_ARCH="${DEVICE_ARCH:-aarch64}"
export RETANIC_NATIVE_DIR="$GAMEDIR/native"
export RETANIC_PATCH_DIR="$GAMEDIR/patches/files"
export XDG_DATA_HOME="$GAMEDIR/saves"
export SDL_GAMECONTROLLERCONFIG="$sdl_controllerconfig"
export PATH="$godot_dir:$PATH"
$ESUDO chmod 666 /dev/uinput
$GPTOKEYB "$runtime" -c "$GAMEDIR/titanic.gptk" &
mapper_pid=$!
if declare -F pm_platform_helper >/dev/null; then
  pm_platform_helper "$godot_dir/$runtime"
fi
# Override the project's desktop window size before SDL creates its surface.
# device_info.txt reports the display dimensions, including firmware rotation.
display_width=${DISPLAY_WIDTH:-640}
display_height=${DISPLAY_HEIGHT:-480}
if [[ ! "$display_width" =~ ^[1-9][0-9]{1,3}$ || ! "$display_height" =~ ^[1-9][0-9]{1,3}$ ]]; then
  display_width=640
  display_height=480
fi
echo "Titanic display: ${display_width}x${display_height}; game frame: 512x384"
"$godot_dir/$runtime" $GODOT_OPTS --resolution "${display_width}x${display_height}" --main-pack "$GAMEDIR/titanic.pck" -- --game-data="$GAMEDIR/gamedata"
