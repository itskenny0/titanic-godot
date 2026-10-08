extends SceneTree
var failed = false
func check(value, label):
	if not value:
		failed = true
		printerr("FAIL: ", label)
func _init():
	var renderer = preload("res://scripts/svg_artwork.gd")
	var file = File.new()
	check(file.open("res://artwork/ui/manifest.json", File.READ) == OK, "SVG manifest packaged")
	var manifest = JSON.parse(file.get_as_text()).result
	file.close()
	check(manifest.images.size() == 16, "exploration controls and all navigation hint variants present")
	for key in manifest.images:
		var entry = manifest.images[key]
		var path = "res://artwork/ui/images/" + key + ".svg"
		check(file.open(path, File.READ) == OK, "raw SVG packaged")
		var source = file.get_buffer(file.get_len())
		file.close()
		var pixels = renderer.rasterize(source, int(entry.width), int(entry.height), path)
		check(pixels.size() == int(entry.width)*int(entry.height)*4, "SVG decoded: " + key)
		var translucent = false
		var transparent = false
		var opaque = false
		for i in range(3,pixels.size(),4):
			translucent = translucent or (pixels[i] > 0 and pixels[i] < 255)
			transparent = transparent or pixels[i] == 0
			opaque = opaque or pixels[i] == 255
		check(translucent and transparent and opaque, "SVG alpha preserved: " + key)
		check(renderer.rasterize(source,1,1,path).empty(), "wrong dimensions rejected")
	check(renderer.rasterize(PoolByteArray(),2,2).empty(), "empty SVG rejected")
	print("SVG TESTS ", "FAIL" if failed else "PASS")
	quit(1 if failed else 0)
