package graphql

import (
	"context"

	"github.com/nasdf/capy/core"
	"github.com/nasdf/capy/types"

	"github.com/99designs/gqlgen/graphql"
	"github.com/ipld/go-ipld-prime/datamodel"
	"github.com/ipld/go-ipld-prime/node/basicnode"
	"github.com/vektah/gqlparser/v2"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

type contextKey string

var (
	idContextKey   = contextKey("id")
	rootContextKey = contextKey("root")
	linkContextKey = contextKey("link")
)

const (
	createOperationPrefix = "create_"
	listOperationPrefix   = "list_"
	getOperationPrefix    = "get_"
)

// QueryParams contains all of the parameters for a query.
type QueryParams struct {
	Query         string         `json:"query"`
	OperationName string         `json:"operationName"`
	Variables     map[string]any `json:"variables"`
}

// Execute runs the query and returns a node containing the result of the query operation.
func Execute(ctx context.Context, system *types.System, store *core.Store, schema *ast.Schema, params QueryParams) (datamodel.Node, error) {
	nb := basicnode.Prototype.Any.NewBuilder()
	ma, err := nb.BeginMap(2)
	if err != nil {
		return nil, err
	}
	err = execute(ctx, system, store, schema, params, ma)
	if err != nil {
		return nil, err
	}
	err = ma.Finish()
	if err != nil {
		return nil, err
	}
	return nb.Build(), nil
}

func execute(ctx context.Context, system *types.System, store *core.Store, schema *ast.Schema, params QueryParams, na datamodel.MapAssembler) error {
	query, errs := gqlparser.LoadQuery(schema, params.Query)
	if errs != nil {
		return assignErrors(errs, na)
	}
	exe := executionContext{
		schema: schema,
		store:  store,
		system: system,
		params: params,
		query:  query,
	}
	data, err := exe.execute(ctx)
	if err != nil {
		return assignErrors(gqlerror.List{gqlerror.WrapIfUnwrapped(err)}, na)
	}
	va, err := na.AssembleEntry("data")
	if err != nil {
		return err
	}
	return va.AssignNode(data)
}

type executionContext struct {
	schema *ast.Schema
	store  *core.Store
	system *types.System
	query  *ast.QueryDocument
	params QueryParams
}

func (e *executionContext) execute(ctx context.Context) (datamodel.Node, error) {
	var operation *ast.OperationDefinition
	if e.params.OperationName != "" {
		operation = e.query.Operations.ForName(e.params.OperationName)
	} else if len(e.query.Operations) == 1 {
		operation = e.query.Operations[0]
	}
	if operation == nil {
		return nil, gqlerror.Errorf("operation is not defined")
	}
	rootLink, err := e.store.RootLink(ctx)
	if err != nil {
		return nil, err
	}
	res := basicnode.Prototype.Map.NewBuilder()
	ctx = context.WithValue(ctx, rootContextKey, rootLink)
	switch operation.Operation {
	case ast.Mutation:
		err := e.executeMutation(ctx, operation.SelectionSet, res)
		if err != nil {
			return nil, err
		}

	case ast.Query:
		err = e.executeQuery(ctx, operation.SelectionSet, res)
		if err != nil {
			return nil, err
		}

	default:
		return nil, gqlerror.Errorf("unsupported operation %s", operation.Operation)
	}
	return res.Build(), nil
}

func (e *executionContext) collectFields(sel ast.SelectionSet, satisfies ...string) []graphql.CollectedField {
	reqCtx := &graphql.OperationContext{
		RawQuery:  e.params.Query,
		Variables: e.params.Variables,
		Doc:       e.query,
	}
	return graphql.CollectFields(reqCtx, sel, satisfies)
}
