extends Reference

# Read the authored SVG at runtime, including in exported packs. The Go image
# cache retains the raster result; no converted artwork is written to storage.
static func rasterize(bytes, width, height, path = ""):
	if bytes.empty() or bytes.size() > 1048576 or width < 1 or height < 1 or width > 1024 or height > 768:
		return PoolByteArray()
	var image = Image.new()
	var error
	if image.has_method("load_svg_from_string"):
		error = image.call("load_svg_from_string", bytes.get_string_from_utf8(), 1.0)
	else:
		# Godot 3/FRT exposes SVG decoding through Image.load, not the buffer API.
		# Raw SVGs are packaged at their 2x output size with the original viewBox.
		if path.empty():
			return PoolByteArray()
		error = image.load(path)
	if error != OK:
		return PoolByteArray()
	if image.get_width() != width or image.get_height() != height:
		return PoolByteArray()
	image.convert(Image.FORMAT_RGBA8)
	return image.get_data()
