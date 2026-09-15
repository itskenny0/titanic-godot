extends SceneTree
var player
var failed = false
func check(ok, text):
	if not ok:
		failed = true
		printerr("FAIL: ", text)
func _init():
	call_deferred("begin")
func begin():
	player = load("res://Main.tscn").instance()
	get_root().add_child(player)
	yield(create_timer(0.3), "timeout")
	player.close_modal()
	player.show_menu()
	yield(self, "idle_frame")
	yield(self, "idle_frame")
	var bounds = player.modal.get_global_rect()
	check(bounds.position.x >= 0 and bounds.position.y >= 0 and bounds.end.x <= player.layout_size.x and bounds.end.y <= player.layout_size.y, "menu fits viewport")
	player.close_modal()
	player.menu = null
	player.show_keyboard(null)
	yield(self, "idle_frame")
	yield(self, "idle_frame")
	bounds = player.modal.get_global_rect()
	check(bounds.end.x <= player.layout_size.x and bounds.end.y <= player.layout_size.y, "keyboard fits viewport")
	player.close_keyboard()
	player.show_patch_picker(true)
	yield(self, "idle_frame")
	yield(self, "idle_frame")
	bounds = player.modal.get_global_rect()
	check(bounds.position.x >= 0 and bounds.position.y >= 0 and bounds.end.x <= player.layout_size.x and bounds.end.y <= player.layout_size.y, "patch chooser fits viewport")
	check(player.patch_boxes.size() == 6, "six patch groups shown")
	player.patch_select_all(true)
	for check_box in player.patch_boxes.values():
		check(check_box.pressed, "select all")
	player.patch_select_all(false)
	player.hd_pack_path = "synthetic-hd-pack"
	player.show_setup()
	yield(self, "idle_frame")
	yield(self, "idle_frame")
	bounds = player.modal.get_global_rect()
	check(bounds.end.x <= player.layout_size.x and bounds.end.y <= player.layout_size.y, "setup fits viewport")
	if player.touch_enabled:
		if OS.window_size.y > OS.window_size.x:
			check(abs(player.layout_size.y - (player.game_origin.y + 536) - 12) < 1, "portrait controls stay at the bottom")
		var picture = Rect2(player.game_origin, Vector2(512,384))
		for child in player.touch_strip.get_children():
			var button = Rect2(child.position, Vector2(52,52))
			if child.name == "joystick":
				button = Rect2(child.position - Vector2.ONE * child.radius, Vector2.ONE * child.radius * 2)
			check(not picture.intersects(button), "touch button stays outside game image: " + child.name)
			check(button.end.x <= player.layout_size.x and button.end.y <= player.layout_size.y, "touch button fits: " + child.name)
	VisualServer.force_draw()
	var image = get_root().get_texture().get_data()
	image.flip_y()
	var variant = "portrait" if OS.window_size.y > OS.window_size.x else "landscape"
	image.save_png("user://ui-" + variant + ".png")
	print("UI TEST ", "FAIL" if failed else "PASS", " ", OS.window_size, " logical=", player.layout_size)
	quit(1 if failed else 0)
