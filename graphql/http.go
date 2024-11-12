package graphql

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type Executor interface {
	Execute(context.Context, QueryParams) (any, error)
}

// Handler returns an http.Handler that can serve GraphQL requests.
func Handler(e Executor) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var params QueryParams
		var err error
		switch r.Method {
		case http.MethodGet:
			values := r.URL.Query()
			params.Query = values.Get("query")
			params.OperationName = values.Get("operationName")
			if values.Has("variables") {
				err = json.Unmarshal([]byte(values.Get("variables")), &params.Variables)
			}
		case http.MethodPost:
			err = json.NewDecoder(r.Body).Decode(&params)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to parse request: %v", err), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		data, err := e.Execute(r.Context(), params)
		resp := QueryResponse{Data: data, Errors: err}
		err = json.NewEncoder(w).Encode(&resp)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})
}
