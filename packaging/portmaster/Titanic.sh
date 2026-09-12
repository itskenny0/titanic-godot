#!/bin/bash
# PortMaster launcher. Native joypads are disabled in FRT; gptokeyb supplies both sticks.
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
source "$controlfolder/control.txt"
get_controls
GAMEDIR="/$(get_ports_location)/titanic"
cd "$GAMEDIR" || exit 1
exec > >(tee -a "$GAMEDIR/log.txt") 2>&1
runtime=frt_3.5.2
godot_file="$controlfolder/libs/$runtime.squashfs"
if [[ ! -f "$godot_file" ]]; then
  $ESUDO "$controlfolder/harbourmaster" --quiet --no-check runtime_check "$runtime.squashfs"
fi
[[ -f "$godot_file" ]] || exit 1
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
export XDG_DATA_HOME="$GAMEDIR/saves"
export SDL_GAMECONTROLLERCONFIG="$sdl_controllerconfig"
export PATH="$godot_dir:$PATH"
$ESUDO chmod 666 /dev/uinput
$GPTOKEYB "$runtime" -c "$GAMEDIR/titanic.gptk" &
mapper_pid=$!
pm_platform_helper "$runtime"
"$godot_dir/$runtime" $GODOT_OPTS --main-pack "$GAMEDIR/titanic.pck" -- --game-data="$GAMEDIR/gamedata"
