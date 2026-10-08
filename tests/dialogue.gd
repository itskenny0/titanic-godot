extends SceneTree
class PreviewPlayer:
	extends "res://scripts/player.gd"
	var started_save = null
	var classic_device = false
	func classic_format_device(): return classic_device
	func start_runtime(save_path = ""): started_save = save_path
class Recorder:
	extends Reference
	var commands = []
	var metadata = {"eligible":true,"choices":5}
	var surface = {"context":"dialogue","key":"test","targets":[]}
	func query(method): return JSON.print(metadata if method == "dialogue_layout" else surface)
	func execute(method,args = "{}"):
		if method == "command": commands.append(JSON.parse(args).result)
		return ""
var failed = false
func check(ok, label):
	if not ok:
		failed = true
		printerr("FAIL: ",label)
func _init(): call_deferred("begin")
func begin():
	var player = PreviewPlayer.new()
	get_root().add_child(player)
	player.set_process(false)
	player.close_modal()
	player.config.clear()
	player.runtime = Recorder.new()
	player.ready = true
	check(player.needs_dialogue_choice(),"fresh installs get dialogue chooser")
	player.config.set_value("graphics","layout_chosen",true)
	player.config.set_value("graphics","redrawn_ui_chosen",true)
	check(player.needs_dialogue_choice(),"upgrades get chooser independently of previous choices")
	check(not player.wide_dialogue_enabled(),"classic remains default")
	player.show_dialogue_options("voyage.ti",true)
	yield(self, "idle_frame")
	yield(self, "idle_frame")
	var r = player.modal.get_global_rect()
	check(r.position.y >= 0 and r.end.y <= player.layout_size.y,"chooser fits native 4:3 viewport")
	player.controller_back()
	check(player.modal == player.dialogue_choice_dialog,"first choice requires explicit Continue")
	player.toggle_dialogue_option("timed_subtitles")
	player.toggle_dialogue_option("widescreen")
	player.finish_dialogue_options()
	check(player.started_save == "voyage.ti","chooser preserves save selected at boot")
	check(not player.needs_dialogue_choice(),"choice is remembered")
	check(player.wide_dialogue_enabled() and player.config.get_value("dialogue","timed_subtitles"),"options are independent and saved")
	player.config = ConfigFile.new()
	player.config.load("user://settings.cfg")
	check(not player.needs_dialogue_choice(),"opt-in persists across launches")
	player.started_save = null
	player.show_dialogue_options()
	player.toggle_dialogue_option("timed_subtitles")
	player.finish_dialogue_options()
	check(player.started_save == null,"settings do not restart gameplay")
	player.close_modal()
	OS.set_window_size(Vector2(1280,720))
	yield(self, "idle_frame")
	player.has_frame = true
	player.sync_dialogue_layout()
	check(player.dialogue_active,"wide screen enables dialogue")
	for touch in [false,true]:
		player.dialogue.configure(Vector2(683,384),5,touch)
		for i in range(5):
			var point = Vector2(256,276+24*i)
			var screen = player.game_to_display(point)
			check(player.display_to_game(screen).distance_to(point) < 0.01,"answer mapping preserves all five authored targets")
			check(player.dialogue.choices[i].position.x >= (140 if touch else 24),"answer avoids default touch controls")
		check(player.display_to_game(Vector2(340,100)).y < 264,"artwork never clicks an answer")
	player.controller_surface = {"context":"dialogue","key":"test","targets":[{"id":"choice:0:1","aim_x":256,"aim_y":276,"w":512,"h":24}]}
	player.runtime.surface = player.controller_surface
	player.focus_controller_target(0)
	player.runtime.commands.clear()
	player.controller_confirm(true)
	player.controller_confirm(false)
	yield(self, "idle_frame")
	check(not player.runtime.commands.empty() and player.runtime.commands.back().get("kind","") == "release" and player.runtime.commands.back().get("y",0) == 276,"controller confirms through original answer coordinates")
	player.classic_device = true
	player.sync_dialogue_layout()
	check(not player.dialogue_active,"native 4:3 device always keeps Classic")
	player.classic_device = false
	OS.set_window_size(Vector2(480,800))
	yield(self, "idle_frame")
	player.sync_dialogue_layout()
	check(not player.dialogue_active,"portrait falls back to Classic")
	player.config.clear()
	player.config.save("user://settings.cfg")
	print("DIALOGUE UI ","FAIL" if failed else "PASS")
	quit(1 if failed else 0)
