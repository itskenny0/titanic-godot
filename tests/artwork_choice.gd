extends SceneTree

class PreviewPlayer:
	extends "res://scripts/player.gd"
	var started_save = null
	func start_runtime(save_path = ""):
		started_save = save_path

class Recorder:
	extends Reference
	var examples = []
	func query(_method, _args = "{}"):
		return JSON.print(examples)
	func execute(_method, _args = "{}"):
		return ""

var failed = false
func check(ok, message):
	if not ok:
		failed = true
		printerr("FAIL: ", message)
func _init(): call_deferred("begin")
func begin():
	var player = PreviewPlayer.new()
	get_root().add_child(player)
	player.set_process(false)
	player.close_modal()
	player.config.clear()
	var recorder = Recorder.new()
	var pixels = PoolByteArray()
	pixels.resize(64*17*4)
	recorder.examples = [{"name":"navarrow", "key":"664db3b77a63612c31d91fe32685b42e492f61635d9d0cb3cbe165fb306ca84a", "width":64,"height":17,"pixels":Marshalls.raw_to_base64(pixels)}]
	player.runtime = recorder
	player.game_index = {"1/house.shp":"synthetic"}
	check(player.needs_artwork_choice(), "fresh installs choose artwork")
	player.config.set_value("graphics","layout_chosen",true)
	player.config.set_value("graphics","redrawn_ui",false)
	check(player.needs_artwork_choice(), "upgraders with old settings still see artwork choice")
	for enabled in [false,true]:
		player.started_save = null
		player.show_artwork_choice("saved-voyage.ti",true)
		yield(self, "idle_frame")
		yield(self, "idle_frame")
		var bounds = player.modal.get_global_rect()
		check(bounds.position.x >= 0 and bounds.position.y >= 0 and bounds.end.x <= player.layout_size.x and bounds.end.y <= player.layout_size.y, "comparison fits handheld viewport")
		var preview = player.modal.find_node("artwork_preview",true,false)
		check(preview != null and preview.get_child_count() == 2, "original and redrawn examples displayed side by side")
		for card in preview.get_children():
			check(card.get_child(0).get_child(1) is TextureRect, "both examples contain rendered artwork")
		player.controller_back()
		check(player.modal == player.artwork_choice_dialog, "Back cannot silently dismiss first choice")
		if enabled:
			player.controller_direction("downarrow")
			yield(self, "idle_frame")
		player.controller_confirm(true)
		player.controller_confirm(false)
		yield(self, "idle_frame")
		check(player.started_save == "saved-voyage.ti", "choice preserves the requested startup save")
		check(player.config.get_value("graphics","redrawn_ui") == enabled, "choice controls SVG loading")
		player.config = ConfigFile.new()
		check(player.config.load("user://settings.cfg") == OK, "choice saved to settings")
		check(not player.needs_artwork_choice(), "choice is not repeated on next launch")
		check(player.config.get_value("graphics","redrawn_ui") == enabled, "both opt-in and opt-out persist")
	player.started_save = null
	player.show_artwork_choice()
	player.choose_artwork(false)
	check(player.started_save == null, "settings change does not restart ongoing gameplay")
	check(not player.config.get_value("graphics","redrawn_ui"), "settings can disable redraws later")
	print("ARTWORK CHOICE ","FAIL" if failed else "PASS")
	quit(1 if failed else 0)
