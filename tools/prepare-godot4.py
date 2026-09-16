#!/usr/bin/env python3
"""Generate the Windows ARM64 frontend from the shared Godot 3 UI."""
from pathlib import Path
import argparse,shutil,subprocess,re
p=argparse.ArgumentParser();p.add_argument('--godot',required=True);p.add_argument('--output',default='.build/godot4-project');args=p.parse_args()
root=Path(__file__).resolve().parents[1];out=Path(args.output).resolve()
shutil.copytree(root/'godot',out,ignore=shutil.ignore_patterns('.import','native','.godot','integration.gd','*-test.gd'),dirs_exist_ok=True)
(out/'engine.js').unlink(missing_ok=True)
(out/'integration.gd').unlink(missing_ok=True)
(out/'runtime-test.gd').unlink(missing_ok=True)
(out/'ui-test.gd').unlink(missing_ok=True)
# Replace these APIs before conversion so the converter does not lose nested JSON calls.
for path in (out/'scripts').glob('*.gd'):
 s=path.read_text().replace('JSON.parse(', 'parse_json_value(').replace(').result', ')')
 s=re.sub(r'\bFile\.', 'FileCompat.', re.sub(r'\bDirectory\.', 'DirectoryCompat.', s))
 s+='\nfunc parse_json_value(text):\n\treturn JSON.parse_string(text)\n'
 if 'FileCompat.' in s:s='const FileCompat = preload("res://scripts/file_compat.gd")\n'+s
 if 'DirectoryCompat.' in s:s='const DirectoryCompat = preload("res://scripts/directory_compat.gd")\n'+s
 # extends must precede member constants.
 lines=s.splitlines();extends=next(x for x in lines if x.startswith('extends '));lines.remove(extends)
 path.write_text(extends+'\n'+'\n'.join(lines)+'\n')
subprocess.run([args.godot,'--headless','--path',str(out),'--convert-3to4','--max-file-kb','10000','--max-line-length','10000'],check=True)
p=out/'scripts/player.gd';s=p.read_text();a=s.index('\telse:\n\t\tvar library = GDNativeLibrary');b=s.index('\tif runtime == null:',a)
s=s[:a]+'\telse:\n\t\tshow_note("This build needs the Titanic engine module.")\n\t\treturn\n'+s[b:]
a=s.index('\t\tvar font = ');b=s.index('\treturn fonts[key]',a)
s=s[:a]+'''\t\tvar font = load("res://fonts/LiberationMono-Regular.ttf" if "Courier" in key else "res://fonts/LiberationSans-Regular.ttf").duplicate()
\t\tfont.set_meta("size", int(key.split("px")[0]))
\t\tfonts[key] = font
'''+s[b:]
s=s.replace('get_font_for(args.font).get_string_size(args.text).x','get_font_for(args.font).get_string_size(args.text, HORIZONTAL_ALIGNMENT_LEFT, -1, int(args.font.split("px")[0])).x')
s=s.replace('command.text,ink(command.color),max(0,512-command.x))','command.text, HORIZONTAL_ALIGNMENT_LEFT, max(0,512-command.x), int(command.font.split("px")[0]), ink(command.color))')
s=s.replace('command.text,ink(command.color))','command.text, HORIZONTAL_ALIGNMENT_LEFT, -1, int(command.font.split("px")[0]), ink(command.color))')
s=s.replace('command.text, ink(command.color))','command.text, HORIZONTAL_ALIGNMENT_LEFT, -1, int(command.font.split("px")[0]), ink(command.color))')
s=s.replace('.clip(Rect2(', '.intersection(Rect2(')
s=s.replace('frame_image.create_from_data(', 'frame_image = Image.create_from_data(')
s=re.sub(r'frame_texture\.create_from_image\(frame_image\).*', 'frame_texture = ImageTexture.create_from_image(frame_image)', s)
s=s.replace('frame_texture.set_data(frame_image)', 'frame_texture.update(frame_image)')
s=s.replace('adaptive_atlas_image.create_from_data(', 'adaptive_atlas_image = Image.create_from_data(')
s=re.sub(r'adaptive_atlas_texture\.create_from_image\(adaptive_atlas_image\).*', 'adaptive_atlas_texture = ImageTexture.create_from_image(adaptive_atlas_image)', s)
s=s.replace('adaptive_atlas_texture.set_data(adaptive_atlas_image)', 'adaptive_atlas_texture.update(adaptive_atlas_image)')
s=s.replace('c.label,UIStyle.INK)', 'c.label,HORIZONTAL_ALIGNMENT_LEFT,-1,11,UIStyle.INK)')

s=s.replace('cursor_layer.z_index = 100', 'texture_filter = CanvasItem.TEXTURE_FILTER_NEAREST\n\tcursor_layer.z_index = 100')
s=s.replace('for key in "1234567890qwertyuiopasdfghjklzxcvbnm.,-":','for key in "1234567890qwertyuiopasdfghjklzxcvbnm.,-".split(""):')
s=s.replace('var ready =', 'var game_ready =').replace('ready = false','game_ready = false').replace('ready = true','game_ready = true').replace('not ready','not game_ready').replace('if ready and','if game_ready and').replace('game_game_ready','game_ready')
s=s.replace('Label.ALIGNMENT_CENTER', 'HORIZONTAL_ALIGNMENT_CENTER').replace('MainLoop.NOTIFICATION_WM_QUIT_REQUEST','NOTIFICATION_WM_CLOSE_REQUEST').replace('MainLoop.NOTIFICATION_WM_GO_BACK_REQUEST', 'NOTIFICATION_WM_GO_BACK_REQUEST')
s=re.sub(r'(?<![a-z_])update\(\)', 'queue_redraw()', s)
for old,new in {'JOY_R':'JOY_BUTTON_RIGHT_SHOULDER','JOY_AXIS_6':'JOY_AXIS_TRIGGER_LEFT','JOY_AXIS_7':'JOY_AXIS_TRIGGER_RIGHT','JOY_START':'JOY_BUTTON_START','JOY_R3':'JOY_BUTTON_RIGHT_STICK','JOY_L':'JOY_BUTTON_LEFT_SHOULDER','JOY_BUTTON_0':'JOY_BUTTON_A','JOY_BUTTON_1':'JOY_BUTTON_B','JOY_BUTTON_2':'JOY_BUTTON_X','JOY_BUTTON_3':'JOY_BUTTON_Y','JOY_DPAD_UP':'JOY_BUTTON_DPAD_UP','JOY_DPAD_DOWN':'JOY_BUTTON_DPAD_DOWN','JOY_DPAD_LEFT':'JOY_BUTTON_DPAD_LEFT','JOY_DPAD_RIGHT':'JOY_BUTTON_DPAD_RIGHT','JOY_AXIS_0':'JOY_AXIS_LEFT_X','JOY_AXIS_1':'JOY_AXIS_LEFT_Y','JOY_AXIS_2':'JOY_AXIS_RIGHT_X','JOY_AXIS_3':'JOY_AXIS_RIGHT_Y'}.items():
 s=re.sub(r'\b'+old+r'\b',new,s)
s=s.replace('get_tree().set_screen_stretch(SceneTree.STRETCH_MODE_2D, SceneTree.STRETCH_ASPECT_KEEP, layout_size)','get_window().content_scale_size = Vector2i(layout_size)')
s=s.replace('img.create(52, 52, false, Image.FORMAT_RGBA8)','img = Image.create(52, 52, false, Image.FORMAT_RGBA8)')
s=re.sub(r'texture\.create_from_image\(img\).*', 'texture = ImageTexture.create_from_image(img)', s)
s=s.replace('text.align = Label.ALIGNMENT_CENTER','text.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER')
s=s.replace('.valign = Label.VALIGN_CENTER', '.vertical_alignment = VERTICAL_ALIGNMENT_CENTER')
s=s.replace('.autowrap = true', '.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART').replace('.align =', '.horizontal_alignment =').replace('t.normal = texture', 't.texture_normal = texture')
s=s.replace('Engine.target_fps', 'Engine.max_fps')
s=s.replace('scroll.scroll_horizontal_enabled = false', 'scroll.horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED')
s=s.replace('get_tree().connect("files_dropped",', 'get_window().connect("files_dropped",')
s=s.replace('OS.has_touchscreen_ui_hint()', 'DisplayServer.is_touchscreen_available()')
s=s.replace('check.pressed', 'check.button_pressed')
s=s.replace('super.ti', '.ti') # The converter mistakes filename text for a base call.
s=s.replace('patch_boxes[id].pressed', 'patch_boxes[id].button_pressed')
s=s.replace('event.shift}', 'event.shift_pressed}').replace('event.shift)', 'event.shift_pressed)')
s=s.replace('event.button_pressed', 'event.pressed')
a=s.index('func dispatch_pointer(event):');b=s.index('func process_controller(delta):',a)
s=s[:a]+'func dispatch_pointer(event):\n\tget_viewport().push_input(event, true)\n\n'+s[b:]
p.write_text(s)
for path in (out/'scripts').glob('*.gd'):
 text=path.read_text().replace('.plus_file(', '.path_join(').replace('.bind(args)', '.bindv(args)').replace('OS.get_cmdline_args()', '(OS.get_cmdline_args() + OS.get_cmdline_user_args())')
 text=re.sub(r'\bevent\.(control|meta|alt|shift)\b', lambda m: 'event.' + {'control':'ctrl_pressed','meta':'meta_pressed','alt':'alt_pressed','shift':'shift_pressed'}[m[1]], text)
 text=text.replace('self, "update"', 'self, "queue_redraw"')
 text=text.replace('caption,Style.INK)', 'caption,HORIZONTAL_ALIGNMENT_LEFT,-1,11,Style.INK)')
 text=re.sub(r'(?<![a-z_])update\(\)', 'queue_redraw()', text)
 path.write_text(text)
for source in (root/'tools/godot4').glob('*.gd'):shutil.copy2(source,out/'scripts'/source.name)
p=out/'project.godot';s=p.read_text();s+='\nrendering/renderer/rendering_method="gl_compatibility"\n' if False else ''
# Put renderer settings in the rendering section.
s=s.replace('window/stretch/mode="2d"', 'window/stretch/mode="canvas_items"')
s=s.replace('[rendering]', '[rendering]\ntextures/lossless_compression/force_png=true\ntextures/vram_compression/import_etc2_astc=true\nrenderer/rendering_method="gl_compatibility"\nrenderer/rendering_method.mobile="gl_compatibility"')
p.write_text(s)
