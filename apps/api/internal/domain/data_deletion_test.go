package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReparentAroundDeleted(t *testing.T) {
	parentOf := map[string]string{
		"a": "", "b": "a", "c": "b", "d": "a", // a and b are deleted
		"e": "", "f": "e", "g": "f", "h": "e", // f is deleted
		"x": "foreign", // parent outside this workspace's tree
	}
	deleted := map[string]bool{"a": true, "b": true, "f": true}

	moves := ReparentAroundDeleted(parentOf, deleted)

	assert.Equal(t, map[string]string{
		"c": "", // a and b both go: c becomes a root node
		"d": "",
		"g": "e", // nearest kept ancestor
	}, moves)
}

// A deleted node whose own parent sits outside the workspace (a corrupt or
// cross-workspace link) must not drag a kept child into that other tree.
func TestReparentAroundDeleted_NeverMovesIntoAnotherTree(t *testing.T) {
	parentOf := map[string]string{"d": "foreign", "c": "d"}

	moves := ReparentAroundDeleted(parentOf, map[string]bool{"d": true})

	assert.Equal(t, map[string]string{"c": ""}, moves)
}

func TestReparentAroundDeleted_StopsOnCycles(t *testing.T) {
	parentOf := map[string]string{"a": "b", "b": "a", "c": "a"}

	moves := ReparentAroundDeleted(parentOf, map[string]bool{"a": true, "b": true})

	assert.Equal(t, map[string]string{"c": ""}, moves)
}
