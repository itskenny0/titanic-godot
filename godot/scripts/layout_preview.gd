extends Control
var player
const Style = preload("res://scripts/ui_style.gd")

func _ready():
	rect_min_size = Vector2(396,100)
	mouse_filter = Control.MOUSE_FILTER_IGNORE

func _draw():
	var chosen = 0 if not player.adaptive_enabled() else (2 if player.adaptive_roomy() else 1)
	for index in range(3):
		var card = Rect2(index*132+2,2,128,96)
		draw_style_box(Style.plate(Color("151b1d"),Style.BRASS if chosen == index else Color("44483f"),4),card)
		var screen = Rect2(card.position+Vector2(4,8),Vector2(120,68))
		var world
		if index == 0:
			var frame = Rect2(screen.position+Vector2(15,0),Vector2(90,67.5))
			world = Rect2(frame.position,Vector2(90,46.4))
			draw_rect(frame,Color("494435"))
			for x in [23,38,53,68]:
				draw_circle(frame.position+Vector2(x,57),5,Style.BRASS)
		else:
			world = screen
		if player.has_frame:
			var scale = player.frame_image.get_width()/512.0
			draw_texture_rect_region(player.frame_texture,world,Rect2(0,0,512*scale,264*scale))
		else:
			# A small deck illustration avoids requiring installed game artwork.
			draw_rect(world,Color("344f58"))
			draw_rect(Rect2(world.position+Vector2(0,world.size.y*0.6),Vector2(world.size.x,world.size.y*0.4)),Color("746951"))
			for x in range(1,6):
				draw_line(world.position+Vector2(world.size.x*x/6,world.size.y*0.44),world.position+Vector2(world.size.x*x/6,world.size.y*0.8),Style.INK,1)
			draw_line(world.position+Vector2(0,world.size.y*0.44),world.position+Vector2(world.size.x,world.size.y*0.44),Style.INK,1)
		if index > 0:
			var rail = 16 if index == 1 else 21
			for x in [0,120-rail+2]:
				draw_rect(Rect2(screen.position+Vector2(x,0),Vector2(rail-2,68)),Color("24292980"))
				for y in [14,34]:
					draw_circle(screen.position+Vector2(x+(rail-2)/2,y),4 if index == 1 else 6,Style.BRASS)
		var caption = ["Classic", "Compact", "Roomy"][index]
		draw_string(player.get_font_for("11px Arial"),card.position+Vector2(8,89),caption,Style.INK)
