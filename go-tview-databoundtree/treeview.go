package tview_databoundtree

import (
	"github.com/gdamore/tcell"
	"github.com/rivo/tview"
)

var (
	KeyCollapseNode = tcell.KeyLeft
	KeyExpandNode   = tcell.KeyRight
	KeyQuitMain     = 'q'
)

func Main(enableMouse bool, data any, showRoot bool) error {
	app := tview.NewApplication().
		EnableMouse(enableMouse).SetRoot(NewTreeView(data, showRoot), true)
	return app.SetInputCapture(func(key *tcell.EventKey) *tcell.EventKey {
		isquitkey := (key.Key() == tcell.Key(KeyQuitMain))
		if KeyQuitMain >= 'a' && KeyQuitMain <= 'z' {
			isquitkey = key.Rune() == rune(KeyQuitMain)
		}
		if isquitkey {
			app.Stop()
			return nil
		}
		return key
	}).Run()
}

func NewTreeView(data any, showRoot bool) *tview.TreeView {
	treenode := newTreeNode(data, nil).Expand()
	treeview := tview.NewTreeView().SetRoot(treenode).SetCurrentNode(treenode)
	treeview.SetInputCapture(treeViewOnInput(treeview))
	if !showRoot {
		treeview.SetTopLevel(1)
	}
	return treeview
}

func newTreeNode(data any, parent DataNode) *tview.TreeNode {
	datanode, is := data.(DataNode)
	if !is {
		datanode = newDataNode(data, parent)
	}
	treenode := tview.NewTreeNode(datanode.String())
	for _, sub := range datanode.Subs() {
		treenode.AddChild(newTreeNode(sub, datanode))
	}
	treenode.Collapse()
	return treenode
}

func treeViewParentNode(ancestorNode *tview.TreeNode, childNode *tview.TreeNode) *tview.TreeNode {
	if ancestorNode != nil && ancestorNode != childNode {
		for _, subnode := range ancestorNode.GetChildren() {
			if subnode == childNode {
				return ancestorNode
			} else if parent := treeViewParentNode(subnode, childNode); parent != nil {
				return parent
			}
		}
	}
	return nil
}

func treeViewOnInput(treeView *tview.TreeView) func(key *tcell.EventKey) *tcell.EventKey {
	return func(key *tcell.EventKey) *tcell.EventKey {
		curnode := treeView.GetCurrentNode()
		if curnode == nil {
			return key
		}
		switch key.Key() {
		case KeyExpandNode:
			if len(curnode.GetChildren()) > 0 && !curnode.IsExpanded() {
				curnode.Expand()
				return nil
			}
		case KeyCollapseNode:
			if len(curnode.GetChildren()) > 0 && curnode.IsExpanded() {
				curnode.Collapse()
				return nil
			} else if parent := treeViewParentNode(treeView.GetRoot(), curnode); parent != nil {
				treeView.SetCurrentNode(parent)
				return nil
			}
		}
		return key
	}
}
