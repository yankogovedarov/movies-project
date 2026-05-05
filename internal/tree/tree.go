package tree

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/yankogovedarov/movie-tracker/internal/db"
)

type Node struct {
	Name     string
	Children []*Node
	Files    []db.Medium
}

func Build(media []db.Medium) *Node {
	root := &Node{}
	for _, m := range media {
		node := root
		if m.FolderRelativePath != "" {
			parts := strings.Split(m.FolderRelativePath, string(filepath.Separator))
			for _, part := range parts {
				node = node.findOrCreate(part)
			}
		}
		node.Files = append(node.Files, m)
	}
	root.sortRecursive()
	return root
}

func (n *Node) findOrCreate(name string) *Node {
	for _, c := range n.Children {
		if c.Name == name {
			return c
		}
	}
	child := &Node{Name: name}
	n.Children = append(n.Children, child)
	return child
}

func (n *Node) sortRecursive() {
	sort.Slice(n.Children, func(i, j int) bool {
		return n.Children[i].Name < n.Children[j].Name
	})
	for _, c := range n.Children {
		c.sortRecursive()
	}
}
