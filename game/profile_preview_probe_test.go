package game

import (
	"os"
	"testing"
	"time"

	"github.com/kivutar/goro/render"
	"github.com/kivutar/goro/res"
	"github.com/kivutar/goro/session"
)

func TestProfilePreviewProbe(t *testing.T) {
	root := os.Getenv("GORO_DATA_DIR")
	if root == "" {
		t.Skip("set GORO_DATA_DIR to probe profile previews")
	}
	manager, err := res.NewManager(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, archive := range manager.Archives {
		defer archive.Close()
	}
	for _, sex := range []byte{0, 1} {
		for _, hair := range []int{1, 2, 4, 12, 23} {
			view, status := loadPlayerHumanoidSpriteView(manager, session.Character{Job: 0, Hair: int16(hair)}, sex, false)
			if view == nil {
				t.Fatalf("sex=%d hair=%d view missing: %s", sex, hair, status)
			}
			if view.head == nil {
				t.Fatalf("sex=%d hair=%d head missing: %s", sex, hair, status)
			}
			billboard, ok := humanoidBillboardForState(view, spriteState{actionFamily: spriteActionIdle, direction: 4}, time.Now())
			if !ok || billboard == nil || billboard.image == nil {
				t.Fatalf("sex=%d hair=%d billboard missing: %s", sex, hair, status)
			}
			image := render.NewImage(160, 180)
			(&WorldMode{}).drawHumanoidPreviewScaledNearest(image, view, 0, 0, 160, 180, 4.0)
			_, _, _, _, drawn := renderImageOpaqueBounds(image)
			if !drawn {
				t.Fatalf("sex=%d hair=%d mobile preview drew no pixels: %s", sex, hair, status)
			}
			t.Logf("sex=%d hair=%d body=%s head=%s billboard=%dx%d", sex, hair, view.body.source, view.head.source, billboard.image.Bounds().Dx(), billboard.image.Bounds().Dy())
		}
	}
}
