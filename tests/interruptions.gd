extends SceneTree
class Recorder:
	extends Reference
	var commands = []
	var tick_dt = -1
	func execute(method, args = "{}"):
		var data = JSON.parse(args).result
		if method == "command": commands.append(data)
		if method == "tick": tick_dt = data.dt
		return ""
	func buffer(_method): return PoolByteArray()
	func query(method):
		return "[]" if method == "events" else '{"context":"busy","key":"","targets":[]}'
var failed = false
func check(ok, message):
	if not ok:
		failed = true
		printerr("FAIL: ", message)
func _init(): call_deferred("begin")
func begin():
	var player = load("res://Main.tscn").instance()
	get_root().add_child(player)
	player.set_process(false)
	player.close_modal()
	var recorder = Recorder.new()
	player.runtime = recorder
	player.ready = true
	player.manual_pause = false
	player.focused = true
	var sound = AudioStreamPlayer.new()
	var stream = AudioStreamSample.new()
	stream.data = PoolByteArray([0,0,0,0])
	stream.loop_mode = AudioStreamSample.LOOP_FORWARD
	stream.loop_end = 4
	sound.stream = stream
	player.add_child(sound)
	sound.play()
	player.sounds[42] = sound
	player.pointer_pressed = true
	player.controller_held["b:0"] = "confirm"
	player.touch_strip.get_node("joystick").move_stick(Vector2(60,0))
	player._notification(MainLoop.NOTIFICATION_APP_PAUSED)
	check(recorder.commands.back().on and sound.stream_paused, "suspend immediately pauses engine and audio")
	check(not player.pointer_pressed and player.controller_held.empty(), "suspend clears held inputs")
	check(player.touch_strip.get_node("joystick").direction.empty(), "suspend resets touch joystick")
	player._notification(MainLoop.NOTIFICATION_WM_FOCUS_OUT)
	player._notification(MainLoop.NOTIFICATION_WM_FOCUS_IN)
	check(recorder.commands.back().on, "focus alone cannot undo suspension")
	player.handle_event({"type":"audio_pause", "on":false})
	check(sound.stream_paused, "late audio event cannot unpause background audio")
	var before = recorder.commands.size()
	player.mouse_button(true)
	player.process_controller(1.0)
	check(recorder.commands.size() == before, "no controller input while backgrounded")
	player._notification(MainLoop.NOTIFICATION_APP_RESUMED)
	check(not recorder.commands.back().on and not sound.stream_paused, "resume restores active gameplay")
	player._process(60.0)
	check(recorder.tick_dt == 0, "resume discards elapsed time")
	player.manual_pause = true
	player._notification(MainLoop.NOTIFICATION_APP_PAUSED)
	player._notification(MainLoop.NOTIFICATION_APP_RESUMED)
	check(recorder.commands.back().on and sound.stream_paused, "manual menu pause survives suspend")
	player.manual_pause = false
	player._notification(MainLoop.NOTIFICATION_WM_FOCUS_OUT)
	player._notification(MainLoop.NOTIFICATION_APP_PAUSED)
	player._notification(MainLoop.NOTIFICATION_APP_RESUMED)
	check(recorder.commands.back().on, "resume waits for window focus")
	player._notification(MainLoop.NOTIFICATION_WM_FOCUS_IN)
	check(not recorder.commands.back().on, "reverse notification order resumes once both ready")
	yield(create_timer(2.1), "timeout")
	player.resume_frame = false
	player.last_process_ms = OS.get_ticks_msec() - 2050
	player._process(5.0)
	check(recorder.tick_dt == 0, "handheld suspend without notification discards gap")
	var mouse = InputEventMouseButton.new()
	check(player.interaction_radius(mouse) == 0, "physical mouse remains precise")
	mouse.device = -1
	check(player.interaction_radius(mouse) == 14, "touch receives target tolerance")
	mouse.device = 0
	player.controller_pointer_event = true
	check(player.interaction_radius(mouse) == 10, "controller receives target tolerance")
	player.controller_pointer_event = false
	player.sounds.clear()
	sound.queue_free()
	print("INTERRUPTIONS ", "FAIL" if failed else "PASS")
	quit(1 if failed else 0)
