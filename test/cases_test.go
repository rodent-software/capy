package test

import (
	"bytes"
	"context"
	"embed"
	"io/fs"
	"path/filepath"
	"testing"
	"text/template"

	"github.com/nasdf/capy"
	"github.com/nasdf/capy/core"
	"github.com/nasdf/capy/graphql"
	"github.com/nasdf/capy/storage"
	"github.com/nasdf/capy/types"

	"github.com/ipld/go-ipld-prime/codec/json"
	"github.com/ipld/go-ipld-prime/node/bindnode"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

//go:embed cases
var casesFS embed.FS

type TestCase struct {
	// Schema is the GraphQL Schema used to create a Capy instance.
	Schema string
	// Operations is a list of all GraphQL Operations to run in this test case.
	Operations []Operation
}

func (tc TestCase) Run(t *testing.T) {
	ctx := context.Background()
	store := core.Open(storage.NewMemory())

	db, err := capy.New(ctx, store, tc.Schema)
	require.NoError(t, err, "failed to create db")

	for _, op := range tc.Operations {
		rootLink, err := store.RootLink(ctx)
		require.NoError(t, err, "failed to load root link")

		rootNode, err := store.Load(ctx, rootLink, db.Types.Prototype(types.RootTypeName))
		require.NoError(t, err, "failed to load root node")

		rootValue := bindnode.Unwrap(rootNode)
		require.NotNil(t, rootValue)

		query, err := op.QueryTemplate(ctx, store, rootValue)
		require.NoError(t, err, "failed to execute query template")

		node, err := db.Execute(ctx, graphql.QueryParams{Query: query})
		require.NoError(t, err, "failed to execute query")

		var actual bytes.Buffer
		err = json.Encode(node, &actual)
		require.NoError(t, err, "failed to encode results")

		expected, err := op.ResponseTemplate(ctx, store, rootValue)
		require.NoError(t, err, "failed to execute response template")

		assert.JSONEq(t, expected, actual.String())
	}
}

type Operation struct {
	// Query contains the Query document for this operation.
	Query string
	// Response contains the expected GraphQL Response.
	Response string
}

func (o Operation) QueryTemplate(ctx context.Context, store *core.Store, rootValue any) (string, error) {
	tpl, err := template.New("response").Parse(o.Query)
	if err != nil {
		return "", err
	}
	var data bytes.Buffer
	if err := tpl.Execute(&data, rootValue); err != nil {
		return "", nil
	}
	return data.String(), nil
}

func (o Operation) ResponseTemplate(ctx context.Context, store *core.Store, rootValue any) (string, error) {
	tpl, err := template.New("response").Parse(o.Response)
	if err != nil {
		return "", err
	}
	var data bytes.Buffer
	if err := tpl.Execute(&data, rootValue); err != nil {
		return "", nil
	}
	return data.String(), nil
}

func TestAllCases(t *testing.T) {
	fs.WalkDir(casesFS, "cases", func(path string, d fs.DirEntry, err error) error {
		if filepath.Ext(path) != ".yaml" && err == nil {
			return nil
		}
		require.NoError(t, err, "failed to walk cases directory")

		data, err := fs.ReadFile(casesFS, path)
		require.NoError(t, err, "failed to read test case file")

		var testCase TestCase
		err = yaml.Unmarshal(data, &testCase)
		require.NoError(t, err, "failed to parse test case file")

		t.Run(path, testCase.Run)
		return nil
	})
}
