extends TouchScreenButton
const Style = preload("res://scripts/ui_style.gd")
var plate_normal = Style.plate(Color("252a2b"), Color("847354"), 7)
var plate_pressed = Style.plate(Color("4b4738"), Color("d4bd84"), 7)
var confirm_mode = false

func _ready():
	connect("pressed", self, "update")
	connect("released", self, "update")

func _draw():
	draw_style_box(plate_pressed if is_pressed() else plate_normal, Rect2(0,0,52,52))
	var ink = Color("e2d5b5")
	if name == "menu":
		for y in [12,18,24]:
			draw_line(Vector2(17,y), Vector2(35,y), ink, 1.5, true)
	elif name == "space":
		if confirm_mode:
			draw_polyline(PoolVector2Array([Vector2(17,18),Vector2(23,24),Vector2(35,11)]),ink,1.5,true)
		else:
			draw_rect(Rect2(19,9,15,20), ink, false, 1.5)
			draw_circle(Vector2(29,20), 1.4, ink)
	elif name == "escape":
		for x in [18,27]:
			draw_polyline(PoolVector2Array([Vector2(x,11),Vector2(x+7,18),Vector2(x,25)]), ink, 1.5, true)
	elif name == "keyboard":
		draw_rect(Rect2(13,10,26,18), ink, false, 1)
		for row in range(2):
			for col in range(5):
				draw_rect(Rect2(16+col*4,13+row*5,2,2), ink)
		draw_line(Vector2(20,24),Vector2(32,24),ink,1)
