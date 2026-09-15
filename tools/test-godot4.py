#!/usr/bin/env python3
"""Run the shared UI or owned-data integration test on the generated frontend."""
from pathlib import Path
import argparse, re, subprocess
p=argparse.ArgumentParser();p.add_argument('--godot',required=True);p.add_argument('--test',choices=['ui','integration','controller','idle','patches','game_files','window_close'],required=True);p.add_argument('--game-data');p.add_argument('--hd-pack');p.add_argument('--resolution',default='640x480');p.add_argument('--touch',action='store_true');p.add_argument('--patches',default='none');p.add_argument('--expect-isos',action='store_true');a=p.parse_args()
root=Path(__file__).resolve().parents[1];project=root/'.build/godot4-project'
s=(root/'tests'/f'{a.test}.gd').read_text()
if a.test == 'game_files':
 s=s.replace('extends SceneTree', 'extends SceneTree\nconst File = preload("res://scripts/file_compat.gd")\nconst Directory = preload("res://scripts/directory_compat.gd")')
 s=s.replace('PoolByteArray', 'PackedByteArray').replace('OS.get_ticks_usec()', 'Time.get_ticks_usec()').replace('.plus_file(', '.path_join(')
s=re.sub(r'JSON.parse\(([^\n]+)\).result',r'JSON.parse_string(\1)',s)
s=s.replace('JSON.print(', 'JSON.stringify(').replace('.instance()', '.instantiate()').replace('player.ready', 'player.game_ready')
s=s.replace('yield(self, "idle_frame")','await process_frame').replace('yield(VisualServer, "frame_post_draw")','await RenderingServer.frame_post_draw').replace('yield(create_timer(0.3), "timeout")','await create_timer(0.3).timeout')
s=re.sub(r'yield\(create_timer\(([\d.]+)\), "timeout"\)',r'await create_timer(\1).timeout',s)
s=s.replace('get_texture().get_data()', 'get_texture().get_image()').replace('OS.window_size', 'DisplayServer.window_get_size()').replace('check_box.pressed', 'check_box.button_pressed')
s=s.replace('VisualServer.force_draw()', 'RenderingServer.force_draw()')
s=re.sub(r'^\t(?:image|screenshot)\.flip_y\(\)\n', '', s, flags=re.M)
s=s.replace('.empty()', '.is_empty()').replace('.has_icon_override(', '.has_theme_icon_override(').replace('.get_icon(', '.get_theme_icon(')
s=s.replace('extends Reference', 'extends RefCounted').replace('.scancode', '.keycode')
s=s.replace('player.get_focus_owner()', 'player.get_viewport().gui_get_focus_owner()')
for old,new in {'JOY_BUTTON_0':'JOY_BUTTON_A','JOY_BUTTON_1':'JOY_BUTTON_B','JOY_BUTTON_3':'JOY_BUTTON_Y','JOY_DPAD_DOWN':'JOY_BUTTON_DPAD_DOWN'}.items():
 s=re.sub(r'\b'+old+r'\b',new,s)
name=f'{a.test}-test.gd';(project/name).write_text(s)
cmd=[a.godot,'--path',str(project),'--audio-driver','Dummy','--resolution',a.resolution,'-s','res://'+name,'--','--integration-test' if a.test=='integration' else '--ui-test','--patches='+a.patches]
if a.game_data:cmd+=['--game-data='+a.game_data]
if a.expect_isos:cmd+=['--expect-isos']
if a.hd_pack:cmd+=['--hd-pack='+a.hd_pack,'--expect-hd']
if a.touch:cmd+=['--touch-test']
try:subprocess.run(cmd,check=True,timeout=100)
finally:(project/name).unlink(missing_ok=True)
