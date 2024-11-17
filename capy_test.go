package capy

import (
	"bytes"
	"context"
	"testing"

	"github.com/nasdf/capy/core"
	"github.com/nasdf/capy/storage"
	"github.com/nasdf/capy/test"

	"github.com/ipld/go-ipld-prime/codec/json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCapy(t *testing.T) {
	paths, err := test.TestCasePaths()
	require.NoError(t, err, "failed to walk test cases dir")

	for _, path := range paths {
		testCase, err := test.LoadTestCase(path)
		require.NoError(t, err, "failed to load test case %s", path)

		t.Run(path, func(st *testing.T) {
			// TODO st.Parallel()

			ctx := context.Background()
			store := core.Open(storage.NewMemory())

			db, err := New(ctx, store, testCase.Schema)
			require.NoError(st, err, "failed to create db")

			for _, op := range testCase.Operations {
				node, err := db.Execute(ctx, op.Params)
				require.NoError(st, err)

				var actual bytes.Buffer
				err = json.Encode(node, &actual)
				require.NoError(st, err)

				assert.JSONEq(st, string(op.Response), actual.String())
			}
		})
	}
}
