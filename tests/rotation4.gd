extends SceneTree

func _initialize():
	call_deferred("check_rotation")

func check_rotation():
	var player = load("res://Main.tscn").instantiate()
	root.add_child(player)
	player.hide()
	var square = ColorRect.new()
	square.color = Color.RED
	square.position = Vector2(20, 20)
	square.size = Vector2(100, 100)
	root.add_child(square)
	for dimensions in [Vector2i(480, 800), Vector2i(800, 480), Vector2i(480, 800)]:
		root.size = dimensions
		await create_timer(0.3).timeout
		await RenderingServer.frame_post_draw
		var screenshot = root.get_texture().get_image()
		var low = Vector2i(screenshot.get_width(), screenshot.get_height())
		var high = Vector2i.ZERO
		for y in screenshot.get_height():
			for x in screenshot.get_width():
				var pixel = screenshot.get_pixel(x, y)
				if pixel.r > 0.99 and pixel.g < 0.01 and pixel.b < 0.01:
					low = low.min(Vector2i(x, y))
					high = high.max(Vector2i(x, y))
		var extent = high - low + Vector2i.ONE
		if extent.x < 10 or absi(extent.x - extent.y) > 2:
			printerr("ROTATION FAIL: ", dimensions, " square became ", extent)
			quit(1)
			return
		print("Rotation ", dimensions, ": square ", extent)
	print("ROTATION PASS")
	quit()
