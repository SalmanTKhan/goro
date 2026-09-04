//go:build ignore

// Command generate derives the Windows resource objects (icon + VERSIONINFO)
// linked into goro.exe. It writes goro_windows_amd64.syso and
// goro_windows_arm64.syso at the repository root, where `go build .` picks them
// up automatically via GOOS/GOARCH filename filtering.
//
// Version fields come from `git describe`; the tool falls back to a zeroed
// "devel" build when git or tags are unavailable so the generator never blocks a
// build. Regenerate and commit the .syso files when cutting a release tag; the
// release workflow also regenerates them from the exact tag.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// goversioninfo is invoked with `go run` and pinned, mirroring the rsrc usage
// this replaced. No go.mod entry is required.
const goversioninfoPkg = "github.com/josephspurrier/goversioninfo/cmd/goversioninfo@v1.7.0"

type versionParts struct {
	major, minor, patch, build int
}

type fixedVersion struct {
	Major int
	Minor int
	Patch int
	Build int
}

type fixedFileInfo struct {
	FileVersion    fixedVersion
	ProductVersion fixedVersion
	FileFlagsMask  string
	FileFlags      string
	FileOS         string
	FileType       string
	FileSubType    string
}

type stringFileInfo struct {
	CompanyName      string
	FileDescription  string
	FileVersion      string
	InternalName     string
	LegalCopyright   string
	OriginalFilename string
	ProductName      string
	ProductVersion   string
	Comments         string
}

type translation struct {
	LangID    string
	CharsetID string
}

type varFileInfo struct {
	Translation translation
}

type versionInfo struct {
	FixedFileInfo  fixedFileInfo
	StringFileInfo stringFileInfo
	VarFileInfo    varFileInfo
}

func main() {
	describe := gitOutput("describe", "--tags", "--always", "--dirty")
	if describe == "" {
		describe = "devel"
	}
	commit := gitOutput("rev-parse", "--short", "HEAD")
	dirty := strings.HasSuffix(describe, "-dirty")

	parts := parseParts(describe)

	comments := "commit " + commit
	if commit == "" {
		comments = "no git metadata"
	}
	if dirty {
		comments += " (modified working tree)"
	}

	fv := fixedVersion{Major: parts.major, Minor: parts.minor, Patch: parts.patch, Build: parts.build}
	info := versionInfo{
		FixedFileInfo: fixedFileInfo{
			FileVersion:    fv,
			ProductVersion: fv,
			FileFlagsMask:  "3f",
			FileFlags:      "00",
			FileOS:         "040004", // VOS_NT_WINDOWS32
			FileType:       "01",     // VFT_APP
			FileSubType:    "00",
		},
		StringFileInfo: stringFileInfo{
			CompanyName:      "kivutar",
			FileDescription:  "Goro",
			FileVersion:      describe,
			InternalName:     "goro",
			LegalCopyright:   "Copyright © kivutar",
			OriginalFilename: "goro.exe",
			ProductName:      "Goro",
			ProductVersion:   describe,
			Comments:         comments,
		},
		VarFileInfo: varFileInfo{
			Translation: translation{LangID: "0409", CharsetID: "04B0"},
		},
	}

	// go:generate runs this from the repository root (the directive lives in
	// main.go), so every path here is repo-root-relative.
	jsonPath := filepath.Join("packaging", "windows", "versioninfo.json")
	data, err := json.MarshalIndent(info, "", "\t")
	check(err)
	check(os.WriteFile(jsonPath, append(data, '\n'), 0o644))

	run(jsonPath, "goro_windows_amd64.syso", false)
	run(jsonPath, "goro_windows_arm64.syso", true)

	fmt.Printf("wrote goro_windows_{amd64,arm64}.syso for %s\n", describe)
}

var describeRE = regexp.MustCompile(`^v(\d+)\.(\d+)\.(\d+)(?:-(\d+)-g[0-9a-f]+)?`)

func parseParts(describe string) versionParts {
	m := describeRE.FindStringSubmatch(describe)
	if m == nil {
		return versionParts{}
	}
	p := versionParts{
		major: atoi(m[1]),
		minor: atoi(m[2]),
		patch: atoi(m[3]),
	}
	if m[4] != "" {
		p.build = atoi(m[4]) // commits since the tag
	}
	return p
}

// run invokes goversioninfo for a single architecture. arm64 needs both -arm and
// -64; amd64 needs -64 without -arm.
func run(jsonPath, out string, arm64 bool) {
	args := []string{"run", goversioninfoPkg,
		"-icon", filepath.Join("packaging", "windows", "goro.ico"),
		"-o", out,
		"-64",
		fmt.Sprintf("-arm=%t", arm64),
		jsonPath,
	}
	cmd := exec.Command("go", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	// goversioninfo picks the syso machine type from -64/-arm, not GOOS/GOARCH,
	// and its binary must run on the host. Clear any cross-compile target so a
	// release build (which sets GOOS=windows) still runs the tool natively.
	cmd.Env = append(os.Environ(), "GOOS=", "GOARCH=")
	check(cmd.Run())
}

func gitOutput(args ...string) string {
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func atoi(s string) int {
	n, err := strconv.Atoi(s)
	check(err)
	return n
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}
