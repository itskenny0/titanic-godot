extends Reference
# All input returned by this helper remains in the original 512x384 space.
var world = Rect2()
var controls = []
var navigation = {}
var navigation_display = Rect2()
var rail = 72.0
var panels = []

func configure(size, metadata, roomy = false):
	rail = 96.0 if roomy else 72.0
	panels = [Rect2(4,16,rail-8,352),Rect2(size.x-rail+4,16,rail-8,352)]
	# Keep every world pixel and click target, filling the viewport behind the UI.
	world = Rect2(Vector2.ZERO, size)
	controls.clear()
	navigation = {}
	for original in metadata.get("controls", []):
		var c = original.duplicate()
		if c.name == "navarrow":
			navigation = c
			var arrow_size = Vector2(c.w,c.h)*world_scale()
			navigation_display = Rect2(Vector2((size.x-arrow_size.x)/2,size.y-arrow_size.y),arrow_size)
			continue
		var left = c.name in ["life", "bag"]
		var upper = c.name in ["life", "watch"]
		c.display = Rect2(6 if left else size.x - rail + 6, 32 if upper else 124, rail-12, 80)
		controls.append(c)

func world_scale():
	return world.size / Vector2(512,264)

func screen_to_game(point):
	for c in controls:
		if c.display.has_point(point):
			return Vector2(c.aim_x, c.aim_y)
	if not navigation.empty() and navigation_display.has_point(point):
		return Vector2(navigation.aim_x,navigation.aim_y)
	var local = (point - world.position) / world_scale()
	if Rect2(0,0,512,264).has_point(local):
		return local
	return Vector2(-1,-1)

func game_to_screen(point, target_id = ""):
	for c in controls:
		if target_id == "prop:" + c.name or point.distance_to(Vector2(c.aim_x,c.aim_y)) < 0.1:
			return c.display.position + c.display.size / 2
	if not navigation.empty() and target_id == "prop:navarrow":
		return navigation_display.position + navigation_display.size/2
	return world.position + point * world_scale()

func target_circle(target):
	for c in controls:
		if target.id == "prop:" + c.name:
			return {"center": c.display.position + c.display.size / 2, "radius": 28.0}
	var scale = world.size.x / 512.0
	return {"center": game_to_screen(Vector2(target.aim_x,target.aim_y),target.id), "radius": clamp(min(target.w,target.h) * scale / 2.0 + 5, 12, 30)}
