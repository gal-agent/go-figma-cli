package xmlscan

import "testing"

const sample = `<page id="0:1" name="Desktop" type="page">
  <frame id="1:10" name="Header" type="frame">
    <text id="1:11" name="Logo" type="text"/>
  </frame>
  <frame id="1:20" name="CardList" type="frame">
    <instance id="1:21" name="Card" type="instance">
      <text id="1:22" name="Title" type="text"/>
    </instance>
  </frame>
</page>`

func TestParseAndChildren(t *testing.T) {
	root, err := Parse(sample)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	page := Root(root)
	if page.ID != "0:1" || page.Name != "Desktop" {
		t.Fatalf("root = %+v", page)
	}
	kids := page.DirectChildren()
	if len(kids) != 2 {
		t.Fatalf("want 2 direct children, got %d", len(kids))
	}
	if kids[0].ID != "1:10" || kids[1].ID != "1:20" {
		t.Fatalf("children ids = %s, %s", kids[0].ID, kids[1].ID)
	}
}

func TestFind(t *testing.T) {
	root, err := Parse(sample)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if n := Root(root).Find("1:22"); n == nil || n.Name != "Title" {
		t.Fatalf("find deep node failed: %+v", n)
	}
	if Root(root).Find("9:99") != nil {
		t.Fatal("unexpected find")
	}
}

func TestFrames(t *testing.T) {
	root, err := Parse(sample)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	// header, cardlist, card-instance = 3 frame-ish nodes (page is not one)
	if got := len(Root(root).Frames()); got != 3 {
		t.Fatalf("frames = %d, want 3", got)
	}
}

func TestParseEmpty(t *testing.T) {
	if _, err := Parse("no xml here at all <b"); err == nil {
		t.Fatal("expected error for non-XML input")
	}
}
