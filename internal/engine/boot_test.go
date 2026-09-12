package engine

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestBootPlanReference(t *testing.T) {
	path := os.Getenv("TAOOT_BOOT_REFERENCE")
	if path == "" {
		t.Skip("set TAOOT_BOOT_REFERENCE to owned-data boot plans")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var corpus []struct {
		Path string
		Plan BootPlan
	}
	if err = json.Unmarshal(data, &corpus); err != nil {
		t.Fatal(err)
	}
	for _, entry := range corpus {
		data, err := os.ReadFile(entry.Path)
		if err != nil {
			t.Fatal(err)
		}
		got := ReadBootPlan(data)
		if !reflect.DeepEqual(got, entry.Plan) {
			t.Fatalf("%s: expected %+v, got %+v", entry.Path, entry.Plan, got)
		}
	}
}
