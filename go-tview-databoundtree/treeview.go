package tview_databoundtree

import (
	"github.com/gdamore/tcell"
	"github.com/rivo/tview"
)

func Main(enableMouse bool, quitOnKey int, data any, showRoot bool) error {
	app := tview.NewApplication().
		EnableMouse(enableMouse).SetRoot(NewTreeView(data, showRoot), true)
	return app.SetInputCapture(func(key *tcell.EventKey) *tcell.EventKey {
		isquitkey := key.Key() == tcell.Key(quitOnKey)
		if quitOnKey >= 'a' && quitOnKey <= 'z' {
			isquitkey = key.Rune() == rune(quitOnKey)
		}
		if isquitkey {
			app.Stop()
			return nil
		}
		return key
	}).Run()
}

func NewTreeView(data any, showRoot bool) *tview.TreeView {
	treeview := tview.NewTreeView()
	if !showRoot {
		treeview.SetTopLevel(1)
	}
	treenode := NewTreeNode(data)
	treeview.SetRoot(treenode).SetCurrentNode(treenode)
	return treeview
}

func NewTreeNode(data any) *tview.TreeNode {
	datanode := NewDataNode(data)
	treenode := tview.NewTreeNode(datanode.String())
	for _, sub := range datanode.Subs() {
		treenode.AddChild(NewTreeNode(sub))
	}
	return treenode
}
