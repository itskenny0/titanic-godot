extends Reference
# All input returned by this helper remains in the original 512x384 space.
var world = Rect2()
var controls = []
var navigation = {}
var navigation_display = Rect2()
var rail = 72.0
var panels = []
# Rows in both rails: two controls each, then the layout switch on the left
# and the notes button on the right, clear of the touch buttons below.
const UPPER_Y = 24
const LOWER_Y = 110
const SWITCH_Y = 196
var notes_display = Rect2()

func configure(size, metadata, roomy = false):
	rail = 96.0 if roomy else 72.0
	panels = [Rect2(4,16,rail-8,352),Rect2(size.x-rail+4,16,rail-8,352)]
	# Keep every world pixel and click target, filling the viewport behind the UI.
	world = Rect2(Vector2.ZERO, size)
	controls.clear()
	notes_display = Rect2(size.x - rail + 6, SWITCH_Y, rail - 12, 26)
	navigation = {}
	var hand = null
	var help = null
	for original in metadata.get("controls", []):
		var c = original.duplicate()
		if c.get("hand", false):
			hand = c
			continue
		if c.name == "invenhelp":
			help = c
			continue
		if c.name == "navarrow":
			navigation = c
			var arrow_size = Vector2(c.w,c.h)*world_scale()
			navigation_display = Rect2(Vector2((size.x-arrow_size.x)/2,size.y-arrow_size.y),arrow_size)
			continue
		var left = c.name in ["life", "bag"]
		var upper = c.name in ["life", "watch"]
		c.display = Rect2(6 if left else size.x - rail + 6, UPPER_Y if upper else LOWER_Y, rail-12, 80)
		controls.append(c)
	# An item taken out of the bag waits beside the move arrow, as it waited
	# beside the bag in the original, clear of the panels and touch controls.
	if hand != null:
		var x = (size.x - (rail - 12)) / 2
		if not navigation.empty():
			x = navigation_display.end.x + 12
		hand.display = Rect2(x, size.y - 86, rail - 12, 80)
		controls.append(hand)
	# The Help button of a new voyage takes the place of a toolbar item Frank
	# does not have yet; the lower rails belong to the touch controls.
	if help != null:
		var spots = [Rect2(6,LOWER_Y,rail-12,80), Rect2(size.x-rail+6,LOWER_Y,rail-12,80), Rect2(size.x-rail+6,UPPER_Y,rail-12,80)]
		var x = (size.x - (rail - 12)) / 2
		if not navigation.empty():
			x = navigation_display.end.x + 12
		if hand != null:
			x = hand.display.end.x + 8
		spots.append(Rect2(x, size.y - 86, rail - 12, 80))
		for spot in spots:
			var free = true
			for c in controls:
				free = free and not c.display.intersects(spot)
			if free:
				help.display = spot
				break
		if help.has("display"):
			controls.append(help)

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
