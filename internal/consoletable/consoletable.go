package consoletable

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
)

const (
	defaultMargin      = 2
	defaultIndentation = 4
)

// ConsoleTable prints formatted tables on the console.
type ConsoleTable struct {
	// Margin between columns
	Margin int
	// Indentation of the first column
	Indentation int

	// Target for output
	Target io.Writer

	cells   [][]any
	columns int
	title   string
}

// New returns a new console table.
func New(title string, columns int) ConsoleTable {
	st := ConsoleTable{
		Margin:      defaultMargin,
		Indentation: defaultIndentation,
		Target:      os.Stdout,
		columns:     columns,
		cells:       make([][]any, 0),
		title:       title,
	}
	return st
}

// AddRow adds a row to table. First row will be threated as header.
func (t *ConsoleTable) AddRow(r []any) {
	if len(r) != t.columns {
		panic(fmt.Sprintf("Added rows need to have %d columns", t.columns))
	}
	t.cells = append(t.cells, r)
}

// Print prints the table on the console.
func (t *ConsoleTable) Print() {
	fmt.Fprintf(t.Target, "%s:\n\n", t.title)
	cols := make([]int, len(t.cells[0]))
	for _, row := range t.cells {
		for i, v := range row {
			cols[i] = max(cols[i], len(renderCell(v)))
		}
	}
	printRow := func(row []any) {
		fmt.Fprint(t.Target, strings.Repeat(" ", t.Indentation))
		margin := strings.Repeat(" ", t.Margin)
		for i, v := range row {
			_, ok := v.(int)
			if ok {
				fmt.Fprintf(t.Target, "%*s%s", cols[i], renderCell(v), margin)
			} else {
				fmt.Fprintf(t.Target, "%-*s%s", cols[i], renderCell(v), margin)
			}
		}
		fmt.Fprintln(t.Target)
	}
	printRow(t.cells[0])
	h := make([]any, len(t.cells[0]))
	for i := range len(h) {
		h[i] = strings.Repeat("-", cols[i])
	}
	printRow(h)
	for _, r := range t.cells[1:] {
		printRow(r)
	}
}

func renderCell(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case int:
		return humanize.Comma(int64(x))
	case time.Time:
		if x.IsZero() {
			return "-"
		}
		return humanize.Time(x)
	case []string:
		return strings.Join(x, ", ")
	case bool:
		if x {
			return "yes"
		}
		return "no"
	default:
		return fmt.Sprint(v)
	}
}
