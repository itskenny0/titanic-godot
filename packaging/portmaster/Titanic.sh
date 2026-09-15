#!/bin/bash
# PortMaster launcher. Native joypads are disabled in FRT; gptokeyb supplies both sticks.
# Resolve the adjacent game folder before loading firmware helpers, so even
# early startup failures reach the log. PortMaster has no get_ports_location().
launcher_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)" || exit 1
# Prefer the release spelling; accept folders renamed on another filesystem.
resolve_directory() {
  local parent=$1 name=$2 candidate base
  if [[ -d "$parent/$name" ]]; then
    printf '%s\n' "$parent/$name"
    return
  fi
  for candidate in "$parent"/*; do
    [[ -d "$candidate" ]] || continue
    base=${candidate##*/}
    if [[ "${base,,}" == "${name,,}" ]]; then
      printf '%s\n' "$candidate"
      return
    fi
  done
  printf '%s\n' "$parent/$name"
}
GAMEDIR="$(resolve_directory "$launcher_dir" titanic)"
game_data="$(resolve_directory "$GAMEDIR" gamedata)"
log_file="$GAMEDIR/log.txt"
[[ -d "$GAMEDIR" ]] || log_file="$launcher_dir/Titanic-log.txt"
[[ ! -f "$log_file" ]] || cp -f -- "$log_file" "${log_file%.txt}.previous.txt"
if ! : > "$log_file"; then
  echo "Cannot write Titanic startup log: $log_file" >&2
  exit 1
fi
exec > >(tee -a "$log_file") 2>&1
mapper_pid=
runtime_mounted=false
# Invoked by the EXIT trap.
# shellcheck disable=SC2317
cleanup() {
  local status=$?
  trap - EXIT ERR
  echo "Titanic launcher exit status: $status"
  [[ -z "$mapper_pid" ]] || kill "$mapper_pid" 2>/dev/null || true
  if [[ "$runtime_mounted" == true ]]; then
    $ESUDO umount "$godot_dir" || echo "Could not unmount runtime: $godot_dir"
  fi
  echo "Log saved to: $log_file"
  exit "$status"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
set -E
trap 'status=$?; printf "Launcher command failed (status %s, line %s): %s\n" "$status" "$LINENO" "$BASH_COMMAND"' ERR
printf 'Titanic PortMaster startup: %s\n' "$(date -u '+%Y-%m-%d %H:%M:%S UTC')"
printf 'Launcher: %s\nGame directory: %s\n' "$launcher_dir" "$GAMEDIR"
if [[ -r "$GAMEDIR/BUILD-INFO.json" ]]; then
  echo "Package build:"
  cat "$GAMEDIR/BUILD-INFO.json"
fi
printf 'Logging: verbose; bash=%s; initial directory=%s\n' "$BASH_VERSION" "$PWD"
uname -srm
id
if command -v getconf >/dev/null; then getconf GNU_LIBC_VERSION; fi
printf 'Memory available at startup:\n'
if [[ -r /proc/meminfo ]]; then
  while IFS= read -r line; do
    case "$line" in MemTotal:*|MemAvailable:*|SwapTotal:*|SwapFree:*) printf '%s\n' "$line" ;; esac
  done < /proc/meminfo
fi
[[ ! -r /etc/os-release ]] || cat /etc/os-release
if [[ -r /proc/device-tree/model ]]; then
  printf 'Device: '
  tr -d '\0' < /proc/device-tree/model
  printf '\n'
fi
if [[ ! -d "$GAMEDIR" ]]; then
  echo "Missing Titanic game folder beside the launcher: $GAMEDIR (no matching folder, ignoring case)"
  exit 1
fi
printf 'Storage and permissions:\n'
df -h "$GAMEDIR"
ls -ldn -- "$launcher_dir" "$GAMEDIR"
printf 'Selected game-data path: %s\n' "$game_data"
# Only report layout and identifying filenames, never game or save contents.
log_game_layout() (
  shopt -s nullglob nocaseglob
  local folder asset
  for folder in "$GAMEDIR" "$game_data" "$game_data"/[L]OCAL "$game_data"/cd[12] "$game_data"/*/LOCAL; do
    [[ -d "$folder" ]] || { printf 'Missing game-data folder: %s\n' "$folder"; continue; }
    printf 'Game-data folder: %s\n' "$folder"
    ls -ldn -- "$folder"
    local entries=("$folder"/*)
    printf 'Immediate entries: %s; readable=%s searchable=%s\n' "${#entries[@]}" "$([[ -r "$folder" ]] && echo yes || echo no)" "$([[ -x "$folder" ]] && echo yes || echo no)"
    for asset in "$folder"/[b]ootfile "$folder"/[b]edsit1.set "$folder"/*cd[12]*.iso; do
      [[ ! -f "$asset" ]] || ls -ln -- "$asset"
    done
  done
)
log_game_layout
for asset in "$GAMEDIR/titanic.pck" "$GAMEDIR/titanic.gptk" "$GAMEDIR"/native/libtitanic.*.so; do
  if [[ -f "$asset" ]]; then ls -ln -- "$asset"; else printf 'Missing packaged file: %s\n' "$asset"; fi
done
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
echo "PortMaster directory: $controlfolder"
# shellcheck source=/dev/null
source "$controlfolder/control.txt" || { echo "Failed to load control.txt"; exit 1; }
if [[ -f "$controlfolder/mod_${CFW_NAME}.txt" ]]; then
  echo "Loading firmware helper: $controlfolder/mod_${CFW_NAME}.txt"
  # shellcheck source=/dev/null
  source "$controlfolder/mod_${CFW_NAME}.txt"
else
  echo "No firmware-specific helper: $controlfolder/mod_${CFW_NAME}.txt"
fi
echo "Loading controller configuration"
get_controls
printf 'Firmware: %s; device: %s; architecture: %s\n' "${CFW_NAME:-unknown}" "${DEVICE_NAME:-unknown}" "${DEVICE_ARCH:-aarch64}"
printf 'Display backend: DISPLAY=%s WAYLAND_DISPLAY=%s SDL_VIDEODRIVER=%s\n' "${DISPLAY:-}" "${WAYLAND_DISPLAY:-}" "${SDL_VIDEODRIVER:-}"
printf 'Runtime options: %s\nController mapper: %s\n' "${GODOT_OPTS:-}" "${GPTOKEYB:-unset}"
cd "$GAMEDIR" || exit 1
runtime=frt_3.5.2
runtime_arch=${DEVICE_ARCH:-$(uname -m)}
case "$runtime_arch" in
  aarch64|arm64) runtime_arch=aarch64 ;;
  armhf|armv7l|armv6l) runtime_arch=armhf ;;
  *) echo "Unsupported handheld architecture: $runtime_arch"; exit 1 ;;
esac
godot_file="$GAMEDIR/runtime/$runtime.$runtime_arch.squashfs"
if [[ -f "$godot_file" ]]; then
  echo "Using bundled $runtime_arch runtime: $godot_file"
else
  godot_file="$controlfolder/libs/$runtime.squashfs"
  if [[ ! -f "$godot_file" ]]; then
    echo "Missing bundled $runtime_arch runtime. Extract the complete Titanic ZIP again, including titanic/runtime."
    echo "No runtime download is needed. An existing PortMaster runtime also works: $godot_file"
    exit 1
  fi
  echo "Using existing PortMaster runtime: $godot_file"
fi
ls -ln -- "$godot_file"
godot_dir="$GAMEDIR/.runtime"
echo "Mounting runtime: $godot_file -> $godot_dir"
$ESUDO mkdir -p "$godot_dir" || exit 1
$ESUDO mount -o loop,ro "$godot_file" "$godot_dir" || { echo "Runtime mount failed"; exit 1; }
runtime_mounted=true
if [[ ! -x "$godot_dir/$runtime" ]]; then
  echo "Runtime executable missing or not executable: $godot_dir/$runtime"
  exit 1
fi
if command -v file >/dev/null; then
  file -L "$godot_dir/$runtime" "$GAMEDIR"/native/libtitanic.*.so
fi
# Dependency checks are diagnostic only; unavailable tools must not block play.
if command -v ldd >/dev/null && command -v timeout >/dev/null; then
  for binary in "$godot_dir/$runtime" "$GAMEDIR"/native/libtitanic.*.so; do
    [[ -f "$binary" ]] || continue
    echo "Shared-library dependencies: $binary"
    timeout 5 ldd "$binary" || echo "Dependency inspection unavailable or failed for $binary"
  done
fi
export FRT_NO_EXIT_SHORTCUTS=FRT_NO_EXIT_SHORTCUTS
export RETANIC_ARCH="$runtime_arch"
export RETANIC_NATIVE_DIR="$GAMEDIR/native"
export RETANIC_PATCH_DIR="$GAMEDIR/patches/files"
export RETANIC_GAME_DIR="$GAMEDIR"
export XDG_DATA_HOME="$GAMEDIR/saves"
export SDL_GAMECONTROLLERCONFIG="${sdl_controllerconfig:-}"
export PATH="$godot_dir:$PATH"
printf 'Engine paths: native=%s patches=%s saves=%s\n' "$RETANIC_NATIVE_DIR" "$RETANIC_PATCH_DIR" "$XDG_DATA_HOME"
printf 'SDL controller mapping: %s\n' "$SDL_GAMECONTROLLERCONFIG"
if [[ -d /dev/input ]]; then ls -ln /dev/input; fi
if [[ -d /dev/dri ]]; then ls -ln /dev/dri; fi
$ESUDO chmod 666 /dev/uinput
echo "Starting controller mapper"
$GPTOKEYB "$runtime" -c "$GAMEDIR/titanic.gptk" &
mapper_pid=$!
printf 'Controller mapper PID: %s\n' "$mapper_pid"
if declare -F pm_platform_helper >/dev/null; then
  echo "Applying PortMaster platform helper"
  pm_platform_helper "$godot_dir/$runtime"
else
  echo "No pm_platform_helper function; using runtime defaults"
fi
printf 'After platform helper: SDL_VIDEODRIVER=%s SDL_AUDIODRIVER=%s LD_LIBRARY_PATH=%s GODOT_OPTS=%s\n' "${SDL_VIDEODRIVER:-}" "${SDL_AUDIODRIVER:-}" "${LD_LIBRARY_PATH:-}" "${GODOT_OPTS:-}"
if kill -0 "$mapper_pid" 2>/dev/null; then
  echo "Controller mapper is running"
else
  echo "Controller mapper exited before runtime startup"
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
# PortMaster supplies GODOT_OPTS as a shell word list.
# shellcheck disable=SC2206
runtime_command=("$godot_dir/$runtime" --verbose $GODOT_OPTS --resolution "${display_width}x${display_height}" --main-pack "$GAMEDIR/titanic.pck" -- --game-data="$game_data")
printf 'Runtime command:'
printf ' %q' "${runtime_command[@]}"
printf '\n'
"${runtime_command[@]}"
runtime_status=$?
echo "FRT exit status: $runtime_status (elapsed launcher time: ${SECONDS}s)"
case "$runtime_status" in
  126) echo "Runtime could not execute; check architecture and filesystem mount permissions" ;;
  127) echo "Runtime or dynamic loader/library was not found; see dependency diagnostics above" ;;
  137) echo "Runtime was killed with SIGKILL; an out-of-memory kill is one possible cause" ;;
  139) echo "Runtime crashed with SIGSEGV" ;;
esac
exit "$runtime_status"
