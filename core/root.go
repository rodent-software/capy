package core

import (
	"context"

	"github.com/ipld/go-ipld-prime/datamodel"
	"github.com/ipld/go-ipld-prime/fluent/qp"
	"github.com/ipld/go-ipld-prime/node/basicnode"
	"github.com/vektah/gqlparser/v2"
	"github.com/vektah/gqlparser/v2/ast"
)

const (
	RootParentsFieldName         = "Parents"
	RootSchemaFieldName          = "Schema"
	RootCollectionsFieldName     = "Collections"
	CollectionDocumentsFieldName = "Documents"
)

func CollectionPath(collection string) datamodel.Path {
	return datamodel.ParsePath(RootCollectionsFieldName).AppendSegmentString(collection)
}

func DocumentsPath(collection string) datamodel.Path {
	return CollectionPath(collection).AppendSegmentString(CollectionDocumentsFieldName)
}

func DocumentPath(collection string, id string) datamodel.Path {
	return DocumentsPath(collection).AppendSegmentString(id)
}

func BuildRootNode(ctx context.Context, db *DB, schema string) (datamodel.Node, error) {
	s, err := gqlparser.LoadSchema(&ast.Source{Input: schema})
	if err != nil {
		return nil, err
	}
	schemaLink, err := db.Store(ctx, basicnode.NewString(schema))
	if err != nil {
		return nil, err
	}
	collectionsNode, err := BuildRootCollectionsNode(ctx, db, s)
	if err != nil {
		return nil, err
	}
	collectionsLink, err := db.Store(ctx, collectionsNode)
	if err != nil {
		return nil, err
	}
	parentsNode, err := BuildRootParentsNode(db)
	if err != nil {
		return nil, err
	}
	return qp.BuildMap(basicnode.Prototype.Map, 3, func(ma datamodel.MapAssembler) {
		qp.MapEntry(ma, RootSchemaFieldName, qp.Link(schemaLink))
		qp.MapEntry(ma, RootCollectionsFieldName, qp.Link(collectionsLink))
		qp.MapEntry(ma, RootParentsFieldName, qp.Node(parentsNode))
	})
}

func BuildRootParentsNode(db *DB, parents ...datamodel.Link) (datamodel.Node, error) {
	return qp.BuildList(basicnode.Prototype.List, int64(len(parents)), func(la datamodel.ListAssembler) {
		for _, l := range parents {
			qp.ListEntry(la, qp.Link(l))
		}
	})
}

func BuildRootCollectionsNode(ctx context.Context, db *DB, s *ast.Schema) (datamodel.Node, error) {
	fields := make(map[string]datamodel.Link)
	for _, def := range s.Types {
		if def.BuiltIn || def.Kind != ast.Object {
			continue
		}
		node, err := BuildCollectionNode(ctx, db, def.Name)
		if err != nil {
			return nil, err
		}
		lnk, err := db.Store(ctx, node)
		if err != nil {
			return nil, err
		}
		fields[def.Name] = lnk
	}
	return qp.BuildMap(basicnode.Prototype.Map, int64(len(fields)), func(ma datamodel.MapAssembler) {
		for k, v := range fields {
			qp.MapEntry(ma, k, qp.Link(v))
		}
	})
}

func BuildCollectionNode(ctx context.Context, db *DB, name string) (datamodel.Node, error) {
	return qp.BuildMap(basicnode.Prototype.Map, 1, func(ma datamodel.MapAssembler) {
		qp.MapEntry(ma, CollectionDocumentsFieldName, qp.Map(0, func(ma datamodel.MapAssembler) {}))
	})
}

// Dump returns a map containing all collections and document ids.
//
// This function is primarily used for testing.
func Dump(ctx context.Context, db *DB) (map[string][]string, error) {
	rootNode, err := db.RootNode(ctx)
	if err != nil {
		return nil, err
	}
	collectionsLinkNode, err := rootNode.LookupByString(RootCollectionsFieldName)
	if err != nil {
		return nil, err
	}
	collectionsLink, err := collectionsLinkNode.AsLink()
	if err != nil {
		return nil, err
	}
	collectionsNode, err := db.Load(ctx, collectionsLink, basicnode.Prototype.Map)
	if err != nil {
		return nil, err
	}
	docs := make(map[string][]string)
	iter := collectionsNode.MapIterator()
	for !iter.Done() {
		k, v, err := iter.Next()
		if err != nil {
			return nil, err
		}
		collection, err := k.AsString()
		if err != nil {
			return nil, err
		}
		collectionLink, err := v.AsLink()
		if err != nil {
			return nil, err
		}
		collectionNode, err := db.Load(ctx, collectionLink, basicnode.Prototype.Map)
		if err != nil {
			return nil, err
		}
		documentsNode, err := collectionNode.LookupByString(CollectionDocumentsFieldName)
		if err != nil {
			return nil, err
		}
		documentIter := documentsNode.MapIterator()
		for !documentIter.Done() {
			k, _, err := documentIter.Next()
			if err != nil {
				return nil, err
			}
			id, err := k.AsString()
			if err != nil {
				return nil, err
			}
			docs[collection] = append(docs[collection], id)
		}
	}
	return docs, nil
}