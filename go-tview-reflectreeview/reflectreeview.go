package tview_reflectreeview

import (
	"github.com/gdamore/tcell"
	"github.com/rivo/tview"
)

var MainQuitOnKeyEvents = [][3]int{
	{int(tcell.ModCtrl | tcell.ModAlt), 0, 'q'}, // ctrl+q
	// {0, int(tcell.KeyESC), 0},    // esc
}

func Main(enableMouse bool, data any, showRoot bool) error {
	app := tview.NewApplication()
	app.EnableMouse(enableMouse)
	tree := New(data, showRoot)
	// app.SetInputCapture(func(key *tcell.EventKey) *tcell.EventKey {
	// 	for _, quitter := range MainQuitOnKeyEvents {
	// 		if (quitter[0] < 0 || key.Modifiers() == tcell.ModMask(quitter[0])) &&
	// 			((quitter[1] > 0 && key.Key() == tcell.Key(quitter[1])) || (quitter[2] > 0 && key.Rune() == rune(quitter[2]))) {
	// 			panic("YOY")
	// 			app.Stop()
	// 			return nil
	// 		}
	// 	}
	// 	return key
	// })
	return app.SetRoot(tree, true).Run()
}

func New(data any, showRoot bool) *tview.TreeView {
	tree := tview.NewTreeView()
	if !showRoot {
		tree.SetTopLevel(1)
	}
	node := newNode(data)
	tree.SetRoot(node).SetCurrentNode(node)
	return tree
}

func newNode(data any) *tview.TreeNode {
	node := tview.NewTreeNode("foo")

	return node
}
