package figmaurl

import "testing"

func TestParse(t *testing.T) {
	cases := []struct {
		in      string
		fileKey string
		nodeID  string
		wantErr bool
	}{
		{in: "https://www.figma.com/design/AbCdEf123/My-File?node-id=12-34", fileKey: "AbCdEf123", nodeID: "12:34"},
		{in: "https://www.figma.com/file/AbCdEf123/t?node-id=12%3A34", fileKey: "AbCdEf123", nodeID: "12:34"},
		{in: "https://figma.com/design/AbCdEf123/t", fileKey: "AbCdEf123"},
		{in: "AbCdEf123", fileKey: "AbCdEf123"},
		{in: "https://www.figma.com/design/AbCdEf123/t?node-id=1-2;3-4", fileKey: "AbCdEf123", nodeID: "1:2;3:4"},
		{in: "", wantErr: true},
		{in: "https://x.example.com/", wantErr: true},
		{in: "not a key!", wantErr: true},
	}
	for _, c := range cases {
		ref, err := Parse(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("Parse(%q): expected error", c.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("Parse(%q): %v", c.in, err)
			continue
		}
		if ref.FileKey != c.fileKey || ref.NodeID != c.nodeID {
			t.Errorf("Parse(%q) = %+v, want key=%s node=%s", c.in, ref, c.fileKey, c.nodeID)
		}
	}
}

func TestNormalizeNodeID(t *testing.T) {
	if got := NormalizeNodeID("12-34"); got != "12:34" {
		t.Fatalf("dash form: %q", got)
	}
	if got := NormalizeNodeID("12:34"); got != "12:34" {
		t.Fatalf("colon form: %q", got)
	}
}
