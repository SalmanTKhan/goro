package res

import (
	"bytes"
	"testing"

	"golang.org/x/text/encoding/korean"
	"golang.org/x/text/transform"
)

func TestParseRSM(t *testing.T) {
	var buf bytes.Buffer
	buf.WriteString("GRSM")
	buf.WriteByte(1)
	buf.WriteByte(5)
	writeI32(&buf, 120)
	writeI32(&buf, 2)
	buf.WriteByte(204)

	buf.Write(make([]byte, 16))
	writeI32(&buf, 1)
	writeFixedRSWString(&buf, "stone.bmp", 40)
	writeFixedRSWString(&buf, "root", 40)

	writeI32(&buf, 1)
	writeFixedRSWString(&buf, "root", 40)
	writeFixedRSWString(&buf, "", 40)

	writeI32(&buf, 1)
	writeI32(&buf, 0)

	for i := 0; i < 9; i++ {
		if i%4 == 0 {
			writeF32(&buf, 1)
		} else {
			writeF32(&buf, 0)
		}
	}
	writeVector3(&buf, 1, 2, 3)
	writeVector3(&buf, 4, 5, 6)
	writeF32(&buf, 45)
	writeVector3(&buf, 0, 1, 0)
	writeVector3(&buf, 1, 1, 1)

	writeI32(&buf, 3)
	writeVector3(&buf, 0, 0, 0)
	writeVector3(&buf, 10, 0, 0)
	writeVector3(&buf, 0, 10, 0)

	writeI32(&buf, 3)
	writeRSMTextureVertex(&buf, 255, 0, 0, 255, 0, 0)
	writeRSMTextureVertex(&buf, 0, 255, 0, 255, 1, 0)
	writeRSMTextureVertex(&buf, 0, 0, 255, 255, 0, 1)

	writeI32(&buf, 1)
	writeU16(&buf, 0)
	writeU16(&buf, 1)
	writeU16(&buf, 2)
	writeU16(&buf, 0)
	writeU16(&buf, 1)
	writeU16(&buf, 2)
	writeU16(&buf, 0)
	writeU16(&buf, 0)
	writeI32(&buf, 1)
	writeI32(&buf, 7)

	writeI32(&buf, 1)
	writeI32(&buf, 0)
	writeF32(&buf, 0)
	writeF32(&buf, 0)
	writeF32(&buf, 0)
	writeF32(&buf, 1)

	writeI32(&buf, 1)
	writeI32(&buf, 5)
	writeVector3(&buf, 1, 2, 3)
	writeF32(&buf, 4)

	writeI32(&buf, 1)
	writeVector3(&buf, 5, 6, 7)
	writeVector3(&buf, 8, 9, 10)
	writeVector3(&buf, 11, 12, 13)
	writeI32(&buf, 14)

	rsm, err := ParseRSM(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if rsm.VersionMajor != 1 || rsm.VersionMinor != 5 || rsm.AnimLength != 120 || rsm.ShadeType != 2 {
		t.Fatalf("unexpected header: %+v", rsm)
	}
	if len(rsm.Textures) != 1 || rsm.Textures[0] != "stone.bmp" || rsm.MainNodeName != "root" {
		t.Fatalf("unexpected textures/root: %#v root=%q", rsm.Textures, rsm.MainNodeName)
	}
	if len(rsm.Nodes) != 1 {
		t.Fatalf("unexpected nodes: %d", len(rsm.Nodes))
	}
	node := rsm.Nodes[0]
	if node.Name != "root" || len(node.Vertices) != 3 || len(node.TextureVertices) != 3 || len(node.Faces) != 1 {
		t.Fatalf("unexpected node: %+v", node)
	}
	if node.TextureRefs[0].Index != 0 || node.Faces[0].SmoothGroup != 7 || node.RotationKeyframes[0].Quaternion[3] != 1 {
		t.Fatalf("unexpected face/anim data: %+v", node)
	}
	if len(rsm.PositionKeyframes) != 1 || rsm.PositionKeyframes[0].Data != 4 {
		t.Fatalf("unexpected pos keyframes: %+v", rsm.PositionKeyframes)
	}
	if len(rsm.VolumeBoxes) != 1 || rsm.VolumeBoxes[0].Flag != 14 {
		t.Fatalf("unexpected volume boxes: %+v", rsm.VolumeBoxes)
	}
}

func writeRSMTextureVertex(buf *bytes.Buffer, r, g, b, a byte, u, v float32) {
	buf.WriteByte(r)
	buf.WriteByte(g)
	buf.WriteByte(b)
	buf.WriteByte(a)
	writeF32(buf, u)
	writeF32(buf, v)
}

func TestDecodeRSMResourceStringDecodesLegacyEUCKR(t *testing.T) {
	want := `프론테라\pron-newcastle.bmp`
	raw, _, err := transform.String(korean.EUCKR.NewEncoder(), want)
	if err != nil {
		t.Fatal(err)
	}
	if got := decodeRSMResourceString([]byte(raw)); got != want {
		t.Fatalf("decoded RSM resource string = %q, want %q", got, want)
	}
}
