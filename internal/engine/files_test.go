package engine

import "testing"

func TestFilesDiscOverridesAndFallback(t *testing.T) {
	reads := map[string]int{}
	f := NewFiles(map[string]string{"1/room.set": "disc1-room", "2/room.set": "disc2-room", "1/main.stg": "shared"}, func(path string) ([]byte, error) { reads[path]++; return []byte(path), nil })
	check := func(name, want string) {
		t.Helper()
		got, err := f.Provide(name)
		if err != nil || string(got) != want {
			t.Fatalf("%s: %q, %v", name, got, err)
		}
	}
	check(`C:\SETS\ROOM.SET`, "disc1-room")
	f.SetDisc(2)
	check("room.set", "disc2-room")
	check("Disc:Stages:MAIN.STG", "shared")
	check("main.stg", "shared")
	if reads["shared"] != 1 || !f.Has("MAIN.STG") {
		t.Fatal("disc-specific cache failed")
	}
	if n := f.Evict("main.stg"); n != len("shared") {
		t.Fatal(n)
	}
	check("main.stg", "shared")
	if reads["shared"] != 2 {
		t.Fatal("eviction did not reload")
	}
	f.SetDisc(1)
	check("room.set", "disc1-room")
	if reads["disc1-room"] != 1 {
		t.Fatal("other disc's cache was lost")
	}
	check("missing.mov", "")
}
