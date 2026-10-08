extends HBoxContainer

func populate(player, examples):
	for redrawn in [false, true]:
		var panel = PanelContainer.new()
		panel.rect_min_size = Vector2(194,154)
		panel.add_stylebox_override("panel", preload("res://scripts/ui_style.gd").plate(Color("171a1b"),Color("44483f"),4))
		add_child(panel)
		var canvas = Control.new()
		canvas.rect_min_size = Vector2(192,152)
		canvas.mouse_filter = Control.MOUSE_FILTER_IGNORE
		panel.add_child(canvas)
		var title = Label.new()
		title.text = "Redrawn" if redrawn else "Original"
		title.rect_position = Vector2(8,3)
		title.add_font_override("font",player.get_font_for("13px Arial"))
		canvas.add_child(title)
		var shown = 0
		if examples is Array:
			for example in examples:
				if not example.name in ["life","bag","navarrow"]:
					continue
				var width = int(example.width)
				var height = int(example.height)
				if width < 1 or height < 1 or width > 128 or height > 128:
					continue
				var pixels = Marshalls.base64_to_raw(example.pixels)
				if redrawn:
					var file = File.new()
					var path = "res://artwork/ui/images/" + example.key + ".svg"
					if file.open(path,File.READ) != OK:
						continue
					width *= 2
					height *= 2
					pixels = preload("res://scripts/svg_artwork.gd").rasterize(file.get_buffer(file.get_len()),width,height,path)
					file.close()
				if pixels.size() != width*height*4:
					continue
				var img = Image.new()
				img.create_from_data(width,height,false,Image.FORMAT_RGBA8,pixels)
				var texture = ImageTexture.new()
				texture.create_from_image(img,0)
				var picture = TextureRect.new()
				picture.texture = texture
				picture.expand = true
				picture.stretch_mode = TextureRect.STRETCH_KEEP_ASPECT_CENTERED
				picture.mouse_filter = Control.MOUSE_FILTER_IGNORE
				var bounds = {"life": Rect2(4,24,94,94), "bag": Rect2(102,24,84,90), "navarrow": Rect2(48,121,96,26)}[example.name]
				picture.rect_position = bounds.position
				picture.rect_size = bounds.size
				canvas.add_child(picture)
				shown += 1
		if shown == 0:
			var unavailable = Label.new()
			unavailable.text = "Preview unavailable\nfor these game files."
			unavailable.rect_position = Vector2(8,62)
			unavailable.add_font_override("font",player.get_font_for("12px Arial"))
			canvas.add_child(unavailable)
