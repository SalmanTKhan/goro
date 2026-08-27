package offlinecontent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildResolvesImportsOverridesAndDeclarations(t *testing.T) {
	root := filepath.Join("testdata", "rathena")
	content, err := Build(Options{RathenaRoot: root, Maps: []string{"prontera", "prt_fild00"}, Profile: "pre-re", Packetver: 20080910})
	if err != nil {
		t.Fatal(err)
	}
	if content.Source.Fingerprint == "" || content.Source.Profile != "pre-re" {
		t.Fatalf("source metadata = %+v", content.Source)
	}
	if content.Items[909].Name != "Refined Jellopy" || !content.Items[501].UseSupported || content.Items[501].UseEffect.HPMin != 45 || content.Items[501].UseEffect.HPMax != 65 {
		t.Fatalf("item import/effect = %+v", content.Items)
	}
	if len(content.Warnings) != 1 || !strings.Contains(content.Warnings[0], "unsupported Script") {
		t.Fatalf("unsupported script warnings = %+v", content.Warnings)
	}
	poring := content.Monsters[1002]
	if poring.HP != 50 || poring.AttackMin != 7 || poring.AttackMax != 10 || len(poring.Drops) != 1 || poring.Drops[0].Rate != 7000 {
		t.Fatalf("monster = %+v", poring)
	}
	if content.Skills[5].SPCost != 8 || content.Skills[5].CooldownMS != 350 {
		t.Fatalf("skill = %+v", content.Skills[5])
	}
	prontera := content.Maps["prontera"]
	if len(prontera.Shops) != 1 || len(prontera.NPCs) != 1 || len(prontera.Warps) != 1 {
		t.Fatalf("prontera declarations = %+v", prontera)
	}
	if prontera.Shops[0].Items[0].Price != 50 {
		t.Fatalf("-1 shop price = %d, want item buy price 50", prontera.Shops[0].Items[0].Price)
	}
	if len(content.Maps["prt_fild00"].Spawns) != 1 || content.Maps["prt_fild00"].Spawns[0].Count != 2 {
		t.Fatalf("field spawns = %+v", content.Maps["prt_fild00"].Spawns)
	}
}

func TestBuildIsDeterministic(t *testing.T) {
	root := filepath.Join("testdata", "rathena")
	first, err := Build(Options{RathenaRoot: root, Maps: []string{"prt_fild00", "prontera"}})
	if err != nil {
		t.Fatal(err)
	}
	second, err := Build(Options{RathenaRoot: root, Maps: []string{"prontera", "prt_fild00"}})
	if err != nil {
		t.Fatal(err)
	}
	firstData, _ := json.Marshal(first)
	secondData, _ := json.Marshal(second)
	if string(firstData) != string(secondData) || first.Source.Fingerprint != second.Source.Fingerprint {
		t.Fatal("identical source produced different content")
	}
}

func TestAddStarterPoringUsesImportedDefinitionAndChangesFingerprint(t *testing.T) {
	content, err := Build(Options{RathenaRoot: filepath.Join("testdata", "rathena"), Maps: []string{"prontera"}})
	if err != nil {
		t.Fatal(err)
	}
	before := content.Source.Fingerprint
	if err := AddStarterPoring(&content, "prontera", 82, 98); err != nil {
		t.Fatal(err)
	}
	if content.Source.Fingerprint == before {
		t.Fatal("starter fixture did not change content fingerprint")
	}
	spawns := content.Maps["prontera"].Spawns
	if len(spawns) != 1 || spawns[0].MonsterID != 1002 || spawns[0].Name != content.Monsters[1002].Name || spawns[0].X != 82 || spawns[0].Y != 98 || spawns[0].Count != 1 {
		t.Fatalf("starter spawn = %+v", spawns)
	}
	if err := AddStarterPoring(&content, "prontera", 82, 98); err != nil {
		t.Fatal(err)
	}
	if len(content.Maps["prontera"].Spawns) != 1 {
		t.Fatalf("duplicate starter spawn was added: %+v", content.Maps["prontera"].Spawns)
	}
}

func TestBuildFailsMalformedCoreRecord(t *testing.T) {
	root := t.TempDir()
	copyFixtureTree(t, filepath.Join("testdata", "rathena"), root)
	path := filepath.Join(root, "db", "pre-re", "mob_db.yml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	data = []byte(strings.Replace(string(data), "Hp: 50", "Hp: broken", 1))
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	result, buildErr := Build(Options{RathenaRoot: root, Maps: []string{"prontera"}})
	if buildErr == nil {
		t.Fatalf("malformed monster record was accepted: %+v", result.Monsters[1002])
	}
}

func copyFixtureTree(t *testing.T, source, destination string) {
	t.Helper()
	err := filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o600)
	})
	if err != nil {
		t.Fatal(err)
	}
}
