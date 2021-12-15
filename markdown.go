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

type walker interface {
	Walker(n ast.Node, entering bool) (ast.WalkStatus, error)
}

type chainVisitor struct {
	Moves []walker
}

func (v *chainVisitor) Walker(n ast.Node, entering bool) (ast.WalkStatus, error) {
	if len(v.Moves) == 0 {
		return ast.WalkStop, nil
	}
	walker := v.Moves[0]
	status, err := walker.Walker(n, entering)
	if err != nil {
		return status, err
	} else if status == ast.WalkStop {
		v.Moves = v.Moves[1:]
		return v.Walker(n, entering)
	}
	return status, nil
}

type findHeadingVisitor struct {
	Source  []byte
	Heading string
}

func (v *findHeadingVisitor) Walker(n ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	if n.Kind() == ast.KindHeading && string(n.Text(v.Source)) == v.Heading {
		return ast.WalkStop, nil
	}
	return ast.WalkContinue, nil
}

type findFirstVisitor struct {
	Kind ast.NodeKind
}

func (v *findFirstVisitor) Walker(n ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	if n.Kind() == v.Kind {
		return ast.WalkStop, nil
	}
	return ast.WalkContinue, nil
}

type extractTableVisitor struct {
	Source      []byte
	CheckHeader func([]string) errors.E
	Key         func([]string) (string, errors.E)
	Value       func([]string) (string, errors.E)
	Result      map[string]string
	currentRow  []string
}

func (v *extractTableVisitor) Walker(n ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering || n.Kind() != extensionAst.KindTable {
		return ast.WalkStop, errors.New("not starting at a table")
	}
	err := ast.Walk(n, v.tableWalker)
	return ast.WalkStop, err
}

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

func parseTable(input []byte, heading string, keyMapper func(string) string) (map[string]string, errors.E) {
	p := parser.NewParser(
		parser.WithBlockParsers(parser.DefaultBlockParsers()...),
		parser.WithInlineParsers(parser.DefaultInlineParsers()...),
		parser.WithParagraphTransformers(parser.DefaultParagraphTransformers()...),
		parser.WithParagraphTransformers(
			util.Prioritized(extension.NewTableParagraphTransformer(), 200),
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
			if len(row) != 4 {
				return "", errors.Errorf("invalid row: %+v", row)
			}
			if strings.Contains(row[3], "(Deprecated") {
				return "", nil
			}
			key := row[0]
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
			if len(row) != 4 {
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
		Result: map[string]string{},
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

func parseProjectTable(input []byte) (map[string]string, errors.E) {
	return parseTable(input, "Edit project", func(key string) string {
		switch key {
		case "public_builds":
			// "public_jobs" is used in get,
			// while "public_builds" is used in edit.
			// See: https://gitlab.com/gitlab-org/gitlab/-/issues/329725
			return "public_jobs"
		case "container_expiration_policy_attributes":
			// "container_expiration_policy" is used in get,
			// while "container_expiration_policy_attributes" is used in edit.
			return "container_expiration_policy"
		case "requirements_access_level":
			// Currently it does not work.
			// See: https://gitlab.com/gitlab-org/gitlab/-/issues/323886
			return ""
		case "show_default_award_emojis":
			// Currently it does not work.
			// See: https://gitlab.com/gitlab-org/gitlab/-/issues/348365
			return ""
		default:
			return key
		}
	})
}

func parseShareTable(input []byte) (map[string]string, errors.E) {
	return parseTable(input, "Share project with group", nil)
}

func parseLabelsTable(input []byte) (map[string]string, errors.E) {
	newDescriptions, err := parseTable(input, "Create a new label", nil)
	if err != nil {
		return nil, err
	}
	editDescriptions, err := parseTable(input, "Edit an existing label", nil)
	if err != nil {
		return nil, err
	}
	// We want to preserve label IDs so we copy edit description for it.
	newDescriptions["id"] = editDescriptions["label_id"]
	return newDescriptions, nil
}
