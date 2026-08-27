package mobilepack

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kivutar/goro/res"
)

func TestBuildCompletePreservesLooseResourcePrecedence(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "data"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "data", "loose.txt"), []byte("loose"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "data", "duplicate.txt"), []byte("loose-wins"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "client.exe"), []byte("not-resource-data"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "extracted data"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "extracted data", "working.txt"), []byte("not-runtime-data"), 0o644); err != nil {
		t.Fatal(err)
	}

	sourceTree := t.TempDir()
	if err := os.MkdirAll(filepath.Join(sourceTree, "data"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourceTree, "data", "archive.txt"), []byte("archive"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourceTree, "data", "duplicate.txt"), []byte("archive-loses"), 0o644); err != nil {
		t.Fatal(err)
	}
	archivePath := filepath.Join(root, "data.grf")
	if _, err := res.PackGRF(archivePath, sourceTree); err != nil {
		t.Fatal(err)
	}

	output := filepath.Join(t.TempDir(), "complete.grf")
	result, err := BuildComplete(root, output)
	if err != nil {
		t.Fatal(err)
	}
	if result.Scope != "complete" || result.Root != "all" {
		t.Fatalf("result scope/root = %q/%q, want complete/all", result.Scope, result.Root)
	}
	if result.IncludedFiles != 3 {
		t.Fatalf("included files = %d, want 3", result.IncludedFiles)
	}

	archive, err := res.OpenGRF(output)
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	for _, name := range []string{"data/loose.txt", "data/archive.txt", "data/duplicate.txt"} {
		if !archive.Has(name) {
			t.Fatalf("complete pack missing %s", name)
		}
	}
	data, err := archive.ReadFile("data/duplicate.txt")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "loose-wins" {
		t.Fatalf("duplicate content = %q, want loose-wins", data)
	}
}

func TestInspectFindsLooseAndArchivedMaps(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "data"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"prontera.gnd", "Izlude.GND"} {
		if err := os.WriteFile(filepath.Join(root, "data", name), []byte("fixture"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	sourceTree := t.TempDir()
	if err := os.MkdirAll(filepath.Join(sourceTree, "data"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourceTree, "data", "Payon.gnd"), []byte("fixture"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := res.PackGRF(filepath.Join(root, "maps.gpf"), sourceTree); err != nil {
		t.Fatal(err)
	}

	inventory, err := Inspect(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(inventory.Archives) != 1 {
		t.Fatalf("archives = %d, want 1", len(inventory.Archives))
	}
	if len(inventory.Maps) != 3 {
		t.Fatalf("maps = %d, want 3", len(inventory.Maps))
	}
	for _, expected := range []string{"izlude", "payon", "prontera"} {
		found := false
		for _, mapInfo := range inventory.Maps {
			if mapInfo.Name == expected {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("inventory missing map %q", expected)
		}
	}
}

func TestBuildMapsRejectsEmptySelection(t *testing.T) {
	_, err := BuildMaps(t.TempDir(), nil, filepath.Join(t.TempDir(), "selection.grf"))
	if err == nil || !strings.Contains(err.Error(), "contains no maps") {
		t.Fatalf("error = %v, want empty-selection error", err)
	}
}

func TestIncludeOfflineNPCResourcesUsesContentSpriteIDs(t *testing.T) {
	root := t.TempDir()
	sourceTree := t.TempDir()
	if err := os.MkdirAll(filepath.Join(sourceTree, "data", "sprite", "NPC"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, extension := range []string{"act", "spr"} {
		path := filepath.Join(sourceTree, "data", "sprite", "NPC", "1_M_01."+extension)
		if err := os.WriteFile(path, []byte(extension), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := res.PackGRF(filepath.Join(root, "data.grf"), sourceTree); err != nil {
		t.Fatal(err)
	}
	manager, err := res.NewManager(root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		for _, archive := range manager.Archives {
			_ = archive.Close()
		}
	}()

	b := newBuilder(manager, "prontera", "map")
	contentPath := filepath.Join(t.TempDir(), "content.json")
	content := `{"maps":{"prontera":{"npcs":[{"sprite":47}]}}}`
	if err := os.WriteFile(contentPath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	b.includeOfflineNPCResources(contentPath)
	for _, extension := range []string{"act", "spr"} {
		canonical := "data/sprite/NPC/1_M_01." + extension
		if _, ok := b.files[canonical]; !ok {
			t.Fatalf("offline NPC %s resource was not included; files=%v", extension, b.files)
		}
	}
}

func TestIncludeOfflineMonsterResourcesUsesContentSpawnIDs(t *testing.T) {
	root := t.TempDir()
	sourceTree := t.TempDir()
	if err := os.MkdirAll(filepath.Join(sourceTree, "data", "sprite", "monster"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, extension := range []string{"act", "spr"} {
		path := filepath.Join(sourceTree, "data", "sprite", "monster", "Poring."+extension)
		if err := os.WriteFile(path, []byte(extension), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := res.PackGRF(filepath.Join(root, "data.grf"), sourceTree); err != nil {
		t.Fatal(err)
	}
	manager, err := res.NewManager(root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		for _, archive := range manager.Archives {
			_ = archive.Close()
		}
	}()

	b := newBuilder(manager, "selection", "map")
	contentPath := filepath.Join(t.TempDir(), "content.json")
	content := `{"maps":{"prt_fild05":{"spawns":[{"monster_id":1002}]}}}`
	if err := os.WriteFile(contentPath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	b.includeOfflineMonsterResources(contentPath)
	for _, extension := range []string{"act", "spr"} {
		canonical := "data/sprite/monster/Poring." + extension
		if _, ok := b.files[canonical]; !ok {
			t.Fatalf("offline monster %s resource was not included; files=%v", extension, b.files)
		}
	}
	if len(b.missing) != 0 {
		t.Fatalf("monster resources reported missing dependencies: %v", b.missing)
	}
}

func TestIncludeOfflineCombatFeedbackResources(t *testing.T) {
	root := t.TempDir()
	sourceTree := t.TempDir()
	if err := os.MkdirAll(filepath.Join(sourceTree, "data", "sprite", "이팩트"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"숫자.act", "숫자.spr", "msg.act", "msg.spr"} {
		if err := os.WriteFile(filepath.Join(sourceTree, "data", "sprite", "이팩트", name), []byte(name), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := res.PackGRF(filepath.Join(root, "data.grf"), sourceTree); err != nil {
		t.Fatal(err)
	}
	manager, err := res.NewManager(root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		for _, archive := range manager.Archives {
			_ = archive.Close()
		}
	}()

	b := newBuilder(manager, "prontera", "map")
	b.includeOfflineCombatFeedbackResources()
	for _, name := range []string{"숫자.act", "숫자.spr", "msg.act", "msg.spr"} {
		canonical := "data/sprite/이팩트/" + name
		if _, ok := b.files[canonical]; !ok {
			t.Fatalf("offline combat feedback resource %s was not included; files=%v", name, b.files)
		}
	}
	if len(b.missing) != 0 {
		t.Fatalf("combat feedback resources reported missing dependencies: %v", b.missing)
	}
}

func TestIncludeOfflineItemResourcesUsesStarterAndContentItems(t *testing.T) {
	root := t.TempDir()
	sourceTree := t.TempDir()
	if err := os.MkdirAll(filepath.Join(sourceTree, "data", "sprite", "아이템"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(sourceTree, "data", "texture", "유저인터페이스", "item"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, item := range []string{"starter_potion", "starter_jellopy", "starter_knife"} {
		for _, extension := range []string{"act", "spr"} {
			name := filepath.Join(sourceTree, "data", "sprite", "아이템", item+"."+extension)
			if err := os.WriteFile(name, []byte(item+extension), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		name := filepath.Join(sourceTree, "data", "texture", "유저인터페이스", "item", item+".bmp")
		if err := os.WriteFile(name, []byte(item+"icon"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(sourceTree, "idnum2itemresnametable.txt"), []byte("501#starter_potion#\n909#starter_jellopy#\n1201#starter_knife#\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := res.PackGRF(filepath.Join(root, "data.grf"), sourceTree); err != nil {
		t.Fatal(err)
	}
	manager, err := res.NewManager(root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		for _, archive := range manager.Archives {
			_ = archive.Close()
		}
	}()

	b := newBuilder(manager, "prontera", "map")
	b.maps = []string{"prontera"}
	contentPath := filepath.Join(t.TempDir(), "content.json")
	content := `{"maps":{"prontera":{"spawns":[{"monster_id":1002}],"shops":[{"items":[{"item_id":501}]}]}},"monsters":{"1002":{"drops":[{"item_id":909}]}}}`
	if err := os.WriteFile(contentPath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	b.includeOfflineItemResources(contentPath)
	for _, item := range []string{"starter_potion", "starter_jellopy", "starter_knife"} {
		for _, extension := range []string{"act", "spr"} {
			canonical := "data/sprite/아이템/" + item + "." + extension
			if _, ok := b.files[canonical]; !ok {
				t.Fatalf("offline item %s resource was not included; files=%v", canonical, b.files)
			}
		}
		canonical := "data/texture/유저인터페이스/item/" + item + ".bmp"
		if _, ok := b.files[canonical]; !ok {
			t.Fatalf("offline item icon was not included: %s", canonical)
		}
	}
	if len(b.missing) != 0 {
		t.Fatalf("item resources reported missing dependencies: %v", b.missing)
	}
}

func TestIncludeOfflineAudioResourcesUsesAuthoredFiles(t *testing.T) {
	root := t.TempDir()
	sourceTree := t.TempDir()
	sounds := []string{
		"BGM/01.mp3",
		"data/mp3nametable.txt",
		"data/wav/effect/ef_bash.wav",
		"data/wav/levelup.wav",
		"data/wav/_enemy_hit_normal1.wav",
		"data/wav/_enemy_hit_normal2.wav",
		"data/wav/_enemy_hit_normal3.wav",
		"data/wav/_enemy_hit_normal4.wav",
		"data/wav/attack_short_sword.wav",
		"data/wav/attack_short_sword_.wav",
		"data/wav/_hit_sword.wav",
	}
	for _, sound := range sounds {
		name := filepath.Join(sourceTree, filepath.FromSlash(sound))
		if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(name, []byte(sound), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := res.PackGRF(filepath.Join(root, "data.grf"), sourceTree); err != nil {
		t.Fatal(err)
	}
	manager, err := res.NewManager(root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		for _, archive := range manager.Archives {
			_ = archive.Close()
		}
	}()

	b := newBuilder(manager, "prontera", "map")
	b.includeOfflineAudioResources()
	for _, name := range []string{
		"BGM/01.mp3",
		"data/mp3nametable.txt",
		"data/wav/effect/ef_bash.wav",
		"data/wav/levelup.wav",
		"data/wav/_enemy_hit_normal1.wav",
		"data/wav/_enemy_hit_normal2.wav",
		"data/wav/_enemy_hit_normal3.wav",
		"data/wav/_enemy_hit_normal4.wav",
		"data/wav/attack_short_sword.wav",
		"data/wav/attack_short_sword_.wav",
		"data/wav/_hit_sword.wav",
	} {
		if _, ok := b.files[name]; !ok {
			t.Fatalf("offline audio resource was not included: %s", name)
		}
	}
	if len(b.missing) != 0 {
		t.Fatalf("audio resources reported missing dependencies: %v", b.missing)
	}
}

func TestBuildCompleteEmbedsOfflineContent(t *testing.T) {
	root := t.TempDir()
	contentPath := filepath.Join(t.TempDir(), "content.json")
	if err := os.WriteFile(contentPath, []byte(`{"format":"goro-offline-content"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "complete.grf")
	result, err := BuildCompleteWithOptions(root, output, BuildOptions{OfflineContent: contentPath})
	if err != nil {
		t.Fatal(err)
	}
	if !result.OfflineContentIncluded || result.OfflineContentSHA256 == "" {
		t.Fatalf("offline content result = %+v", result)
	}
	archive, err := res.OpenGRF(output)
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	data, err := archive.ReadFile("offline/content.json")
	if err != nil || string(data) != `{"format":"goro-offline-content"}` {
		t.Fatalf("embedded content = %q, err=%v", data, err)
	}
}
