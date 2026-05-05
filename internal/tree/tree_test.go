package tree_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yankogovedarov/movie-tracker/internal/db"
	"github.com/yankogovedarov/movie-tracker/internal/tree"
)

func med(filename, folder string) db.Medium {
	return db.Medium{Filename: filename, FolderRelativePath: folder}
}

func TestBuild_GroupsFilesByFolder(t *testing.T) {
	media := []db.Medium{
		med("A.mkv", "Action"),
		med("B.mkv", "Action"),
		med("C.mkv", "Drama"),
	}
	root := tree.Build(media)
	require.Len(t, root.Children, 2)
	assert.Equal(t, "Action", root.Children[0].Name)
	assert.Len(t, root.Children[0].Files, 2)
	assert.Equal(t, "Drama", root.Children[1].Name)
	assert.Len(t, root.Children[1].Files, 1)
}

func TestBuild_NestedFolders(t *testing.T) {
	media := []db.Medium{
		med("Movie.mkv", `DoNotDelete\SciFi`),
	}
	root := tree.Build(media)
	require.Len(t, root.Children, 1)
	assert.Equal(t, "DoNotDelete", root.Children[0].Name)
	require.Len(t, root.Children[0].Children, 1)
	assert.Equal(t, "SciFi", root.Children[0].Children[0].Name)
	assert.Len(t, root.Children[0].Children[0].Files, 1)
}

func TestBuild_SortsChildrenAlphabetically(t *testing.T) {
	media := []db.Medium{
		med("Z.mkv", "Zebra"),
		med("A.mkv", "Alpha"),
		med("M.mkv", "Middle"),
	}
	root := tree.Build(media)
	require.Len(t, root.Children, 3)
	assert.Equal(t, "Alpha", root.Children[0].Name)
	assert.Equal(t, "Middle", root.Children[1].Name)
	assert.Equal(t, "Zebra", root.Children[2].Name)
}

func TestBuild_EmptyMedia(t *testing.T) {
	root := tree.Build(nil)
	assert.Empty(t, root.Children)
	assert.Empty(t, root.Files)
}

func TestBuild_RootLevelFiles(t *testing.T) {
	media := []db.Medium{
		med("Movie.mkv", ""),
	}
	root := tree.Build(media)
	assert.Empty(t, root.Children)
	assert.Len(t, root.Files, 1)
}
