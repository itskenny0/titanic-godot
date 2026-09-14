#!/usr/bin/env python3
"""Exercise the real launcher with firmware helpers, without mounting a runtime."""
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile

root = Path(__file__).resolve().parents[1]

def executable(path, text):
    path.write_text(text)
    path.chmod(0o755)

for modern, arch, dimensions, resolution in [
    (False, 'aarch64', ('640', '480'), '640x480'),
    (True, 'armhf', ('1280', '720'), '1280x720'),
    (True, 'aarch64', ('', ''), '640x480'),
    (False, 'aarch64', ('invalid', '0'), '640x480'),
]:
    with tempfile.TemporaryDirectory(prefix='titanic launcher ') as temporary:
        work = Path(temporary)
        ports = work/'ports'
        game = ports/'titanic'
        (game/'gamedata/LOCAL').mkdir(parents=True)
        (game/'gamedata/LOCAL/BOOTFILE').write_text('fixture')
        (game/'gamedata/LOCAL/BEDSIT1.SET').write_text('fixture')
        (ports/'Titanic').mkdir() # Exact lowercase wins when both exist.
        control = work/'config/PortMaster'
        (control/'libs').mkdir(parents=True)
        (control/'libs/frt_3.5.2.squashfs').touch()
        launcher = ports/'Titanic.sh'
        shutil.copy2(root/'packaging/portmaster/Titanic.sh', launcher)
        executable(work/'system', '''#!/usr/bin/env python3
import os, pathlib, subprocess, sys
args = sys.argv[1:]
if args[0] == 'mkdir':
    subprocess.run(['/bin/mkdir', *args[1:]], check=True)
elif args[0] == 'mount':
    if os.environ.get('TEST_MOUNT_FAIL'): raise SystemExit(32)
    (pathlib.Path(args[-1])/'frt_3.5.2').symlink_to(os.environ['TEST_RUNTIME'])
elif args[0] == 'umount':
    (pathlib.Path(args[-1])/'frt_3.5.2').unlink(missing_ok=True)
elif args[0] != 'chmod':
    raise SystemExit('Unexpected system operation: '+str(args))
''')
        executable(work/'mapper', '#!/bin/sh\nexec sleep 30\n')
        executable(work/'runtime', '''#!/usr/bin/env python3
import json, os, pathlib, sys
pathlib.Path(os.environ['TEST_RESULT']).write_text(json.dumps({
    'args': sys.argv[1:], 'cwd': os.getcwd(),
    'arch': os.environ['RETANIC_ARCH'], 'saves': os.environ['XDG_DATA_HOME'],
    'native': os.environ['RETANIC_NATIVE_DIR'], 'game': os.environ['RETANIC_GAME_DIR'],
    'helper': os.environ.get('TEST_PLATFORM_HELPER', '')}))
print('Runtime stdout fixture')
print('Runtime stderr fixture', file=sys.stderr)
raise SystemExit(int(os.environ.get('TEST_RUNTIME_EXIT', '0')))
''')
        # Paths containing spaces are safe for the launcher and game directory.
        # PortMaster's ESUDO/GPTOKEYB variables are deliberately shell word lists.
        bin_dir = Path(tempfile.mkdtemp(prefix='titanic-launch-bin-'))
        try:
            for name in ['system', 'mapper']:
                (bin_dir/name).symlink_to(work/name)
            (control/'control.txt').write_text(
                f'ESUDO="{bin_dir}/system"\nGPTOKEYB="{bin_dir}/mapper"\n'
                f'CFW_NAME=test\nget_controls() {{ export DEVICE_ARCH={arch}; }}\n')
            if modern:
                (control/'mod_test.txt').write_text('pm_platform_helper() { export TEST_PLATFORM_HELPER="$1"; }\n')
            env = dict(os.environ, XDG_DATA_HOME=str(work/'config'),
                       DISPLAY_WIDTH=dimensions[0], DISPLAY_HEIGHT=dimensions[1],
                       GODOT_OPTS='--audio-driver Dummy --resolution 320x240',
                       TEST_RUNTIME=str(work/'runtime'), TEST_RESULT=str(work/'result.json'))
            result = subprocess.run(['bash', str(launcher)], cwd='/', env=env,
                                    capture_output=True, text=True, timeout=15)
            assert result.returncode == 0, result.stdout+result.stderr
            state = json.loads((work/'result.json').read_text())
            assert state['cwd'] == str(game)
            assert state['arch'] == arch
            assert state['native'] == str(game/'native')
            assert state['game'] == str(game)
            assert state['saves'] == str(game/'saves')
            assert state['args'] == ['--verbose', '--audio-driver', 'Dummy', '--resolution', '320x240',
                                     '--resolution', resolution, '--main-pack', str(game/'titanic.pck'),
                                     '--', '--game-data='+str(game/'gamedata')]
            assert state['helper'] == (str(game/'.runtime/frt_3.5.2') if modern else '')
            log = (game/'log.txt').read_text()
            for expected in ['Logging: verbose', 'Storage and permissions:', 'Selected game-data path:', 'Controller mapper PID:', 'Firmware: test', 'architecture: '+arch, 'gamedata/LOCAL',
                             'BOOTFILE', 'BEDSIT1.SET', 'Runtime command:',
                             'Runtime stdout fixture', 'Runtime stderr fixture',
                             'FRT exit status: 0', 'Titanic launcher exit status: 0']:
                assert expected in log, (expected, log)
            # Startup failures keep their status, diagnostics and previous run.
            if modern and arch == 'aarch64':
                failed = subprocess.run(['bash', str(launcher)], cwd='/',
                                        env=dict(env, TEST_RUNTIME_EXIT='139'),
                                        capture_output=True, text=True, timeout=15)
                assert failed.returncode == 139, failed.stdout+failed.stderr
                assert (game/'log.previous.txt').read_text() == log
                assert 'FRT exit status: 139' in (game/'log.txt').read_text()
                failed = subprocess.run(['bash', str(launcher)], cwd='/',
                                        env=dict(env, TEST_MOUNT_FAIL='1'),
                                        capture_output=True, text=True, timeout=15)
                assert failed.returncode == 1, failed.stdout+failed.stderr
                assert 'Runtime mount failed' in (game/'log.txt').read_text()
                (ports/'Titanic').rmdir()
                for spelling in ['Titanic', 'TITANIC', 'tItAnIc']:
                    renamed = ports/spelling
                    game.rename(renamed)
                    (renamed/'gamedata').rename(renamed/'GameData')
                    launched = subprocess.run(['bash', str(launcher)], cwd='/', env=env,
                                              capture_output=True, text=True, timeout=15)
                    assert launched.returncode == 0, launched.stdout+launched.stderr
                    state = json.loads((work/'result.json').read_text())
                    assert state['cwd'] == str(renamed)
                    assert state['saves'] == str(renamed/'saves')
                    assert state['args'][-1] == '--game-data='+str(renamed/'GameData')
                    assert (renamed/'log.txt').exists()
                    (renamed/'GameData').rename(renamed/'gamedata')
                    renamed.rename(game)
                (control/'control.txt').unlink()
                failed = subprocess.run(['bash', str(launcher)], cwd='/', env=env,
                                        capture_output=True, text=True, timeout=15)
                assert failed.returncode == 1
                assert 'control.txt was not found' in (game/'log.txt').read_text()
                game.rename(ports/'unrelated')
                failed = subprocess.run(['bash', str(launcher)], cwd='/', env=env,
                                        capture_output=True, text=True, timeout=15)
                assert failed.returncode == 1
                assert 'no matching folder' in (ports/'Titanic-log.txt').read_text()

        finally:
            shutil.rmtree(bin_dir)
print('PORTMASTER LAUNCHER PASS (simulated firmware helpers, both architectures)')
