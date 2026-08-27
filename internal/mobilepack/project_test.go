package mobilepack

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/kivutar/goro/res"
)

func TestBuildProjectCreatesIncrementalPacksWithoutStagingTree(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "offline", "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "data", "texture"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "offline", "content.json"), []byte("offline-content"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "offline", "nested", "extra.json"), []byte("extra"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "data", "texture", "stone.bmp"), []byte("texture"), 0o644); err != nil {
		t.Fatal(err)
	}
	page, err := ListResources(root, "offline/", 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 2 || len(page.Files) != 2 {
		t.Fatalf("resource page = %#v, want two offline files", page)
	}

	project := Project{
		Format: ProjectFormat, Version: ProjectVersion,
		Source:       ProjectSource{Root: root},
		Base:         PackSpec{Name: "base", Presets: []string{"runtime-core"}},
		Packs:        []PackSpec{{Name: "textures", Include: []string{"data/texture/**"}}},
		Profiles:     []ProfileSpec{{Name: "default", Packs: []string{"base", "textures"}}},
		Optimization: OptimizationSpec{Mode: "lossless"},
	}
	output := filepath.Join(t.TempDir(), "mobile-assets")
	result, err := BuildProject(root, project, output)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Manifest.Packs) != 2 {
		t.Fatalf("packs = %d, want 2", len(result.Manifest.Packs))
	}
	if result.Manifest.Version != ArtifactVersion || result.Manifest.PreferredFormat != "pak" {
		t.Fatalf("manifest container metadata = %+v", result.Manifest)
	}
	for _, pack := range result.Manifest.Packs {
		if len(pack.Files) != 2 {
			t.Fatalf("pack %s artifacts = %+v, want PAK and GRF", pack.Name, pack.Files)
		}
	}
	base, err := res.OpenGRF(filepath.Join(output, "packs", "00-base.grf"))
	if err != nil {
		t.Fatal(err)
	}
	if !base.Has("offline/content.json") || base.Has("data/texture/stone.bmp") {
		t.Fatalf("base pack has unexpected contents")
	}
	_ = base.Close()
	textures, err := res.OpenGRF(filepath.Join(output, "packs", "01-textures.grf"))
	if err != nil {
		t.Fatal(err)
	}
	if !textures.Has("data/texture/stone.bmp") || textures.Has("offline/content.json") {
		t.Fatalf("optional pack has unexpected contents")
	}
	_ = textures.Close()
	basePAK, err := res.OpenPAK(filepath.Join(output, "packs", "00-base.pak"))
	if err != nil {
		t.Fatal(err)
	}
	if !basePAK.Has("offline/content.json") || basePAK.Has("data/texture/stone.bmp") {
		t.Fatalf("base PAK has unexpected contents")
	}
	_ = basePAK.Close()
	texturesPAK, err := res.OpenPAK(filepath.Join(output, "packs", "01-textures.pak"))
	if err != nil {
		t.Fatal(err)
	}
	if !texturesPAK.Has("data/texture/stone.bmp") || texturesPAK.Has("offline/content.json") {
		t.Fatalf("optional PAK has unexpected contents")
	}
	_ = texturesPAK.Close()
	if _, err := os.Stat(filepath.Join(output, "mobile-assets.json")); err != nil {
		t.Fatal(err)
	}
}

func TestBuildProjectDeliveryManifestCanBeSigned(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "data"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "data", "value.txt"), []byte("signed delivery"), 0o644); err != nil {
		t.Fatal(err)
	}
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	keyPath := filepath.Join(t.TempDir(), "delivery.key")
	if err := os.WriteFile(keyPath, privateKey.Seed(), 0o600); err != nil {
		t.Fatal(err)
	}
	project := Project{
		Format: ProjectFormat, Version: ProjectVersion,
		Source:       ProjectSource{Root: root},
		Base:         PackSpec{Name: "base", Include: []string{"data/value.txt"}},
		Profiles:     []ProfileSpec{{Name: "default", Packs: []string{"base"}}},
		Delivery:     DeliverySpec{Enabled: true, Formats: []string{"pak"}, Release: "signed", SigningKey: keyPath},
		Optimization: OptimizationSpec{Mode: "lossless"},
	}
	output := filepath.Join(t.TempDir(), "output")
	result, err := BuildProject(root, project, output)
	if err != nil {
		t.Fatal(err)
	}
	manifestBytes, err := os.ReadFile(result.DeliveryManifestPath)
	if err != nil {
		t.Fatal(err)
	}
	signatureText, err := os.ReadFile(result.DeliverySignaturePath)
	if err != nil {
		t.Fatal(err)
	}
	signature, err := base64.StdEncoding.DecodeString(string(bytes.TrimSpace(signatureText)))
	if err != nil || !ed25519.Verify(publicKey, manifestBytes, signature) {
		t.Fatalf("delivery signature invalid: decode=%v", err)
	}
	var manifest DeliveryManifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.PublicKey != base64.StdEncoding.EncodeToString(publicKey) {
		t.Fatalf("public key=%q", manifest.PublicKey)
	}
}

func TestProjectRoundTripAndProfileRequiresBase(t *testing.T) {
	root := t.TempDir()
	project := Project{
		Format: ProjectFormat, Version: ProjectVersion,
		Source:       ProjectSource{Root: root},
		Base:         PackSpec{Name: "base", Presets: []string{"runtime-core"}},
		Profiles:     []ProfileSpec{{Name: "broken", Packs: []string{"other"}}},
		Optimization: OptimizationSpec{Mode: "lossless"},
	}
	path := filepath.Join(t.TempDir(), "project.json")
	if err := SaveProject(path, project); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadProject(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Base.Name != "base" || loaded.Format != ProjectFormat {
		t.Fatalf("loaded project = %#v", loaded)
	}
	_, err = PlanProject(root, loaded)
	if err == nil {
		t.Fatal("PlanProject unexpectedly accepted a profile without base")
	}
}

func TestBuildProjectDeliveryManifestEmitsMultipleLegacyFormats(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "data"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "data", "value.txt"), []byte("delivery"), 0o644); err != nil {
		t.Fatal(err)
	}
	project := Project{
		Format: ProjectFormat, Version: ProjectVersion,
		Source:       ProjectSource{Root: root},
		Base:         PackSpec{Name: "base", Include: []string{"data/value.txt"}},
		Profiles:     []ProfileSpec{{Name: "default", Packs: []string{"base"}}},
		Delivery:     DeliverySpec{Enabled: true, Formats: []string{"pak", "grf", "gpf"}, Release: "r-test"},
		Optimization: OptimizationSpec{Mode: "lossless"},
	}
	output := filepath.Join(t.TempDir(), "output")
	result, err := BuildProject(root, project, output)
	if err != nil {
		t.Fatal(err)
	}
	if result.Delivery == nil || result.Delivery.Version != DeliveryManifestVersion || len(result.Delivery.Packs) != 1 {
		t.Fatalf("delivery manifest=%#v", result.Delivery)
	}
	if len(result.Delivery.Packs[0].Artifacts) != 3 {
		t.Fatalf("delivery artifacts=%#v", result.Delivery.Packs[0].Artifacts)
	}
	for _, artifact := range result.Delivery.Packs[0].Artifacts {
		if _, err := os.Stat(filepath.Join(output, filepath.FromSlash(artifact.URL))); err != nil {
			t.Fatalf("artifact %s missing: %v", artifact.URL, err)
		}
	}
}
