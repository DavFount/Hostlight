package main

import "testing"

func TestApplicationMenuStructure(t *testing.T) {
	root := NewApp().applicationMenu()
	if len(root.Items) != 3 {
		t.Fatal("expected File, View, Help")
	}
	for i, label := range []string{"File", "View", "Help"} {
		if root.Items[i].Label != label || root.Items[i].SubMenu == nil {
			t.Fatal(root.Items[i])
		}
	}
	if root.Items[0].SubMenu.Items[0].Label != "Settings…" || root.Items[2].SubMenu.Items[0].Label != "About Hostlight" {
		t.Fatal("missing navigation")
	}
}
