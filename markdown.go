package config

import (
	"strings"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	extensionAst "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
	"gitlab.com/tozd/go/errors"
)

// walker is the expected number of columns to find in a table.
const tableColumns = 4

// walker interface provides a Walker to be passed to goldmark.ast.Walk function.
type walker interface {
	Walker(n ast.Node, entering bool) (ast.WalkStatus, error)
}

// chainVisitor is a visitor which runs a series of Moves visitors one after
// the other, starting with the next one when the previous one stops,
// starting on the same node the previous one stopped.
type chainVisitor struct {
	Moves []walker
}

// Walker implements walker interface for chainVisitor.
func (v *chainVisitor) Walker(n ast.Node, entering bool) (ast.WalkStatus, error) {
	if len(v.Moves) == 0 {
		return ast.WalkStop, nil
	}
	w := v.Moves[0]
	status, err := w.Walker(n, entering)
	if err != nil {
		return status, err
	} else if status == ast.WalkStop {
		v.Moves = v.Moves[1:]
		return v.Walker(n, entering)
	}
	return status, nil
}

// findHeadingVisitor is a visitor which stops when it finds the first heading
// with text contents equal to Heading.
type findHeadingVisitor struct {
	Source  []byte
	Heading string
}

// Walker implements walker interface for findHeadingVisitor.
func (v *findHeadingVisitor) Walker(n ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	if n.Kind() == ast.KindHeading && string(n.Text(v.Source)) == v.Heading {
		return ast.WalkStop, nil
	}
	return ast.WalkContinue, nil
}

// findFirstVisitor is a visitor which stops when it finds the first node with
// kind matching Kind.
type findFirstVisitor struct {
	Kind ast.NodeKind
}

// Walker implements walker interface for findFirstVisitor.
func (v *findFirstVisitor) Walker(n ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	if n.Kind() == v.Kind {
		return ast.WalkStop, nil
	}
	return ast.WalkContinue, nil
}

// extractTableVisitor is a visitor which extracts data from a table given
// CheckHeader, Key, and Value functions. Extracted data is available in
// Result field after walking is done. It expects that the node it starts
// walking on is of kind KindTable.
//
// CheckHeader gets a slice of cell values for the header row and it should
// return an error if header is invalid.
//
// Key gets a slice of cell values for the row and it should return a string
// used as the extracted key for the row. If it returns an empty string, the
// row is skipped.
//
// Value gets a slice of cell values for the row and it should return a string
// used as the extracted value for the row.
type extractTableVisitor struct {
	Source      []byte
	CheckHeader func([]string) errors.E
	Key         func([]string) (string, errors.E)
	Value       func([]string) (string, errors.E)
	Result      map[string]string
	currentRow  []string
}

// Walker implements walker interface for extractTableVisitor.
func (v *extractTableVisitor) Walker(n ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering || n.Kind() != extensionAst.KindTable {
		return ast.WalkStop, errors.New("not starting at a table")
	}
	err := ast.Walk(n, v.tableWalker)
	return ast.WalkStop, err
}

// tableWalker is a sub-walker which walks an individual table node only (and its children nodes).
// It does the extraction.
func (v *extractTableVisitor) tableWalker(n ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering && (n.Kind() == extensionAst.KindTableHeader || n.Kind() == extensionAst.KindTableRow) {
		v.currentRow = []string{}
	}
	if entering && n.Kind() == extensionAst.KindTableCell {
		v.currentRow = append(v.currentRow, string(n.Text(v.Source)))
		return ast.WalkSkipChildren, nil
	}
	if !entering && n.Kind() == extensionAst.KindTableHeader {
		err := v.CheckHeader(v.currentRow)
		if err != nil {
			return ast.WalkStop, err
		}
	}
	if !entering && n.Kind() == extensionAst.KindTableRow {
		key, err := v.Key(v.currentRow)
		if err != nil {
			return ast.WalkStop, err
		}
		value, err := v.Value(v.currentRow)
		if err != nil {
			return ast.WalkStop, err
		}
		if key != "" {
			_, ok := v.Result[key]
			if ok {
				return ast.WalkStop, errors.Errorf(`duplicate key "%s"`, key)
			}
			v.Result[key] = value
		}
	}
	return ast.WalkContinue, nil
}

// parseTable is a halper function which parses Markdown input and find the first table after
// the heading, which then converts into a map between fields (attributes) and their descriptions.
//
// keyMapper is used to optionally (when not nil) further transform found fields.
func parseTable(input []byte, heading string, keyMapper func(string) string) (map[string]string, errors.E) {
	p := parser.NewParser(
		parser.WithBlockParsers(parser.DefaultBlockParsers()...),
		parser.WithInlineParsers(parser.DefaultInlineParsers()...),
		parser.WithParagraphTransformers(parser.DefaultParagraphTransformers()...),
		parser.WithParagraphTransformers(
			util.Prioritized(extension.NewTableParagraphTransformer(), 200), //nolint:gomnd
		),
		parser.WithASTTransformers(
			util.Prioritized(extension.NewTableASTTransformer(), 0),
		),
	)
	parsed := p.Parse(text.NewReader(input))
	extractTable := extractTableVisitor{
		Source: input,
		CheckHeader: func(row []string) errors.E {
			expectedHeader := []string{"Attribute", "Type", "Required", "Description"}
			if len(row) != len(expectedHeader) {
				return errors.Errorf("invalid header: %+v", row)
			}
			for i, h := range expectedHeader {
				if row[i] != h {
					return errors.Errorf("invalid header: %+v", row)
				}
			}
			return nil
		},
		Key: func(row []string) (string, errors.E) {
			if len(row) != tableColumns {
				return "", errors.Errorf("invalid row: %+v", row)
			}
			// We skip deprecated fields.
			if strings.Contains(row[3], "(Deprecated") {
				return "", nil
			}
			key := row[0]
			// We do not care for which plan the field is.
			// TODO: Remove other possible plan suffixes, too.
			//       See: https://gitlab.com/gitlab-org/gitlab/-/blob/master/doc/development/documentation/styleguide/index.md#available-product-tier-badges
			key = strings.TrimSuffix(key, " (PREMIUM)")
			if key == "id" {
				// This is a documented parameter for project ID.
				key = ""
			}
			if keyMapper != nil {
				key = keyMapper(key)
			}
			return key, nil
		},
		Value: func(row []string) (string, errors.E) {
			if len(row) != tableColumns {
				return "", errors.Errorf("invalid row: %+v", row)
			}
			description := row[3]
			if len(description) > 0 {
				if !strings.HasSuffix(description, ".") && !strings.HasSuffix(description, ")") {
					description += "."
				}
				description += " "
			}
			return description + "Type: " + row[1], nil
		},
		Result:     map[string]string{},
		currentRow: nil,
	}
	visitor := &chainVisitor{
		Moves: []walker{
			&findHeadingVisitor{
				Source:  input,
				Heading: heading,
			},
			&findFirstVisitor{
				Kind: extensionAst.KindTable,
			},
			&extractTable,
		},
	}
	err := ast.Walk(parsed, visitor.Walker)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return extractTable.Result, nil
}
