package consoletable_test

import (
	"strings"
	"testing"

	"github.com/ErikKalkoken/feedforward/internal/consoletable"
	"github.com/stretchr/testify/assert"
)

func TestConsoleTable(t *testing.T) {
	table := consoletable.New("title", 2)
	out := &strings.Builder{}
	table.Target = out
	table.AddRow([]any{"first", "second"})
	table.AddRow([]any{"alpha", "bravo"})
	table.Print()
	s := out.String()
	assert.Contains(t, s, "first")
	assert.Contains(t, s, "second")
	assert.Contains(t, s, "alpha")
	assert.Contains(t, s, "bravo")
}
