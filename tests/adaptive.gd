extends SceneTree
class Recorder:
	extends Reference
	var commands = []
	var metadata = {"eligible":true,"revision":1,"controls":[]}
	var surface = {"context":"room","key":"test","targets":[]}
	func execute(method,args="{}"):
		if method == "command": commands.append(JSON.parse(args).result)
		return ""
	func query(method):
		if method in ["controls","targets"]: return JSON.print(surface)
		return JSON.print(metadata)
	func buffer(_method):
		var data = PoolByteArray()
		data.resize(640*128*4)
		return data
var player
var failed = false
func check(ok,message):
	if not ok:
		failed = true
		printerr("FAIL: ",message)
func _init(): call_deferred("begin")
func screen_touch(point, pressed):
	# Exercise Godot's actual mouse emulation and viewport scaling.
	var touch = InputEventScreenTouch.new()
	touch.index = 0
	touch.position = point*Vector2(OS.window_size)/player.layout_size
	touch.pressed = pressed
	Input.parse_input_event(touch)
	Input.flush_buffered_events()

func screen_drag(point):
	var drag = InputEventScreenDrag.new()
	drag.index = 0
	drag.position = point*Vector2(OS.window_size)/player.layout_size
	Input.parse_input_event(drag)
	Input.flush_buffered_events()

func pointer_command_count(recorder):
	var count = 0
	for command in recorder.commands:
		if command.action == "pointer":
			count += 1
	return count
func fit(message):
	var bounds = player.modal.get_global_rect()
	check(bounds.position.x >= 0 and bounds.position.y >= 0 and bounds.end.x <= player.layout_size.x+1 and bounds.end.y <= player.layout_size.y+1,message)
func begin():
	player = load("res://Main.tscn").instance()
	get_root().add_child(player)
	player.set_process(false)
	player.close_modal()
	var recorder = Recorder.new()
	player.runtime = recorder
	player.ready = true
	player.has_frame = true
	player.config.clear()
	check(not player.adaptive_enabled(),"classic remains default for existing settings")
	check(not player.config.get_value("graphics","layout_chosen",false),"older settings have not dismissed the new chooser")
	if "--classic-device" in OS.get_cmdline_args():
		player.config.set_value("graphics","adaptive_exploration",true)
		player.sync_adaptive_layout()
		check(player.classic_format_device() and not player.adaptive_active,"native 4:3 display always uses Classic")
		check(not player.layout_switch.visible,"4:3 hides quick switch")
		print("CLASSIC DEVICE ","FAIL" if failed else "PASS")
		quit(1 if failed else 0)
		return
	var names = ["life","bag","watch","map","navarrow"]
	for i in range(5):
		recorder.metadata.controls.append({"name":names[i],"label":names[i],"x":100+i*30,"y":280,"w":20,"h":20,"aim_x":110+i*30,"aim_y":290,"slot":i})
	player.config.set_value("graphics","adaptive_exploration",true)
	for dimensions in [Vector2(640,480),Vector2(720,480),Vector2(480,800),Vector2(1280,720)]:
		OS.set_window_size(dimensions)
		yield(create_timer(0.3),"timeout")
		player.update_touch_layout()
		player.sync_adaptive_layout()
		check(player.adaptive_active == (dimensions.x/dimensions.y >= 1.6),"adaptive is strictly widescreen only " + str(dimensions))
		check(player.classic_format(dimensions) == (max(dimensions.x,dimensions.y)/min(dimensions.x,dimensions.y)<1.6),"phone orientation distinguished from classic device")
	check(player.adaptive_active,"wide exploration enabled")
	player.overlays = [{"op":"text","text":"A Deck","font":"12px Arial","x":20,"y":24,"color":"#fff"},{"op":"rect","x":0,"y":0,"w":512,"h":384,"color":"rgba(0,0,0,0.5)"}]
	player.sync_adaptive_layout()
	check(player.adaptive_active,"location labels and navigation fades preserve side panels")
	yield(VisualServer, "frame_post_draw")
	player.overlays = []
	for roomy in [false,true]:
		player.config.set_value("graphics","adaptive_roomy",roomy)
		player.sync_adaptive_layout()
		player.update_touch_layout()
		var world = player.adaptive.world
		check(player.touch_strip.get_node("joystick").radius == 58.0,"adaptive joystick retains original thumb-sized radius")
		check(not player.touch_control_bounds("joystick").intersects(player.touch_control_bounds("menu")),"default menu does not overlap enlarged joystick")
		for point in [Vector2(0,0),Vector2(255,132),Vector2(511,200)]:
			check(player.display_to_game(player.game_to_display(point)).distance_to(point)<0.01,"world click roundtrip")
		for control in player.adaptive.controls:
			check(world.encloses(control.display),"translucent game controls overlay world")
			var aim = player.display_to_game(control.display.position+Vector2(2,2))
			check(aim == Vector2(control.aim_x,control.aim_y),"separated control maps to safe original hit")
		for control in ["joystick","menu","space","escape","keyboard"]:
			check(world.encloses(player.touch_control_bounds(control)),"touch control overlays world: " + control)
		check(world.position == Vector2.ZERO and world.size == player.layout_size,"artwork fills viewport without borders")
		check(player.pointer_in_picture(Vector2(1,1)),"top-left artwork remains clickable")
		var arrow = player.adaptive.navigation_display
		check(world.encloses(arrow),"navigation arrow stays fully visible")
		check(player.display_to_game(arrow.position+arrow.size/2) == Vector2(230,290),"navigation arrow retains engine aim")
	var stick = player.touch_strip.get_node("joystick")
	for appearance in ["full","hidden"]:
		stick.appearance = appearance
		recorder.commands.clear()
		screen_touch(stick.position+Vector2(stick.radius*0.8,0),true)
		check(Input.is_action_pressed("touch_right"),"joystick owns native touch over world")
		screen_drag(Vector2(player.layout_size.x/2,200))
		screen_touch(Vector2(player.layout_size.x/2,200),false)
		check(pointer_command_count(recorder)==0,"joystick suppresses mouse copies through drag and release: "+appearance)
		check(not Input.is_action_pressed("touch_right") and not player.touch_mouse_blocked,"release clears joystick and gesture capture")
	stick.appearance = "full"
	recorder.commands.clear()
	screen_touch(stick.position,true)
	screen_touch(stick.position,false)
	check(pointer_command_count(recorder)==0 and recorder.commands.size()==1 and recorder.commands[0].key==" ","center tap opens door without clicking behind joystick")
	for control in ["space","escape","keyboard","menu"]:
		recorder.commands.clear()
		var bounds = player.touch_control_bounds(control)
		screen_touch(bounds.position+bounds.size/2,true)
		check(Input.is_action_pressed("touch_"+control),"native touch button still activates: "+control)
		player.process_touch(0.01)
		screen_touch(Vector2(player.layout_size.x/2,200),false)
		check(pointer_command_count(recorder)==0,"touch button does not click through: "+control)
		if player.virtual_keyboard != null:
			player.close_keyboard()
		player.close_modal()
		player.sync_adaptive_layout()
		player.update_touch_layout()
		yield(self, "idle_frame")
	recorder.commands.clear()
	var free_point = Vector2(player.layout_size.x/2,200)
	screen_touch(free_point,true)
	screen_drag(stick.position)
	screen_touch(stick.position,false)
	check(pointer_command_count(recorder)==3 and not player.pointer_pressed,"world drag may cross controls and still release normally")
	recorder.metadata.eligible = false
	recorder.surface = {"context":"dialogue","key":"replies","targets":[
		{"id":"first","aim_x":256,"aim_y":276,"w":512,"h":24},
		{"id":"second","aim_x":256,"aim_y":300,"w":512,"h":24}]}
	player.sync_adaptive_layout()
	recorder.commands.clear()
	screen_touch(stick.position+Vector2(0,stick.radius*0.8),true)
	screen_touch(stick.position+Vector2(0,stick.radius*0.8),false)
	check(player.controller_selection==1,"touch joystick down selects the next dialogue reply")
	check(stick.confirm_mode and player.touch_strip.get_node("space").get_child(0).text=="Select","touch controls indicate dialogue confirmation")
	for command in recorder.commands:
		check(command.action!="key","dialogue joystick never sends exploration keys")
	recorder.commands.clear()
	screen_touch(stick.position,true)
	screen_touch(stick.position,false)
	check(recorder.commands.size()==2 and recorder.commands[0].kind=="press" and recorder.commands[1].kind=="release" and recorder.commands[1].y==300,"center tap confirms the selected dialogue reply exactly once")
	check(player.touch_surround_style.bg_color==Color(0,0,0) and player.touch_surround_style.border_color==Color(0,0,0),"Classic touch surround is solid black")
	recorder.metadata.eligible = true
	recorder.surface = {"context":"room","key":"test","targets":[]}
	player.refresh_controller_surface()
	player.sync_adaptive_layout()
	check(not stick.confirm_mode,"joystick center returns to Door in exploration")
	player.set_controller_surface({"context":"room","key":"test","targets":[{"id":"prop:bag","aim_x":140,"aim_y":290,"w":20,"h":20},{"id":"prop:light","aim_x":180,"aim_y":280,"w":251,"h":120}]})
	check(player.controller_surface.targets.size()==2,"decorative toolbar removed and quick layout switch reachable")
	player.focus_controller_target(1)
	player.controller_confirm(true)
	check(not player.adaptive_active and player.layout_switch.visible,"controller quick switch opens Classic")
	player.toggle_exploration_layout()
	check(player.adaptive_active,"quick switch returns to side panels")
	player.focus_controller_target(0)
	var target = player.controller_surface.targets[0]
	check(player.selection_circle(target).center==player.display_pointer,"selection ring follows moved control")
	player.mouse_button(true)
	check(player.pointer_pressed and recorder.commands.back().x==140,"controller activates mapped toolbar")
	player.mouse_button(false)
	player.set_controller_surface({"context":"room","key":"behind-panel","targets":[{"id":"painting:door","aim_x":30,"aim_y":45,"w":20,"h":20}]})
	player.focus_controller_target(0)
	check(player.display_to_game(player.display_pointer) != player.pointer,"test target lies behind toolbar")
	player.mouse_button(true)
	check(recorder.commands.back().x==30 and recorder.commands.back().y==45,"controller reaches world target behind overlay")
	recorder.metadata.eligible = false
	player.sync_adaptive_layout()
	check(not player.adaptive_active and not player.pointer_pressed,"unsafe screen restores classic and clears held click")
	check(recorder.commands.back().kind=="release" and recorder.commands.back().x==-1,"layout change cannot drop item onto new UI")
	recorder.metadata.eligible = true
	player.sync_adaptive_layout()
	player.edit_touch_positions()
	yield(self, "idle_frame")
	check(player.touch_editing and player.touch_strip.visible,"edit mode exposes locked-away controls")
	var original = player.touch_strip.get_node("joystick").position
	var touch = InputEventScreenTouch.new()
	touch.index=3
	touch.position=original
	touch.pressed=true
	player.touch_editor._input(touch)
	var drag = InputEventScreenDrag.new()
	drag.index=3
	drag.position=Vector2(160,250)
	player.touch_editor._input(drag)
	check(player.touch_strip.get_node("joystick").position==drag.position,"editor drags joystick")
	var before = recorder.commands.size()
	player.touch_strip.get_node("joystick")._input(touch)
	player.process_touch(1)
	check(recorder.commands.size()==before,"editing sends no gameplay input")
	touch.pressed=false
	touch.position=drag.position
	player.touch_editor._input(touch)
	var handle = player.touch_editor.resize_handle()
	touch.position = handle.position+handle.size/2
	touch.pressed = true
	player.touch_editor._input(touch)
	drag.position = touch.position+Vector2(24,-24)
	drag.index = 4
	player.touch_editor._input(drag)
	check(player.touch_strip.get_node("joystick").radius==58,"other finger cannot resize the joystick")
	drag.index = 3
	player.touch_editor._input(drag)
	check(player.touch_strip.get_node("joystick").radius==70,"corner handle resizes joystick")
	var resized_position = player.touch_strip.get_node("joystick").position
	touch.position = drag.position
	touch.pressed = false
	player.touch_editor._input(touch)
	check(recorder.commands.size()==before,"resizing sends no gameplay input")
	player.controller_action("back",true)
	yield(self, "idle_frame")
	check(not player.touch_editing and not player.touch_strip.get_node("joystick").editing,"controller Back exits the editor and locks inputs in place")
	player.close_modal()
	player.sync_adaptive_layout()
	player.update_touch_layout()
	check(player.touch_strip.get_node("joystick").position.distance_to(resized_position)<0.01,"custom position restored")
	check(player.touch_strip.get_node("joystick").radius==70,"custom size restored")
	player.resize_touch_joystick(90,Vector2(20,300))
	check(player.touch_strip.get_node("joystick").radius==70,"joystick size locked outside editing")
	player.config.set_value("touch","joystick_style","knob")
	player.apply_touch_preferences()
	check(player.touch_strip.get_node("joystick").appearance=="knob" and player.touch_strip.get_node("joystick").modulate.a==0.5,"knob-only style has translucent default")
	# Rotating back must neither lose the custom landscape layout nor reuse it in portrait.
	OS.set_window_size(Vector2(480,800))
	yield(create_timer(0.3),"timeout")
	player.update_touch_layout()
	check(not player.adaptive_active,"rotation immediately restores classic")
	check(player.touch_strip.get_node("joystick").radius==58,"portrait keeps independent joystick size")
	check(player.touch_strip.get_node("joystick").position!=drag.position,"portrait has independent positions")
	OS.set_window_size(Vector2(640,480))
	yield(create_timer(0.3),"timeout")
	player.update_touch_layout()
	player.show_control_options()
	yield(self, "idle_frame")
	yield(self, "idle_frame")
	fit("display settings and preview fit 640x480")
	player.show_touch_help()
	yield(self, "idle_frame")
	yield(self, "idle_frame")
	fit("touch settings fit 640x480")
	player.show_first_layout()
	yield(self, "idle_frame")
	yield(self, "idle_frame")
	fit("first-start illustrated chooser fits viewport")
	print("ADAPTIVE ","FAIL" if failed else "PASS")
	quit(1 if failed else 0)
