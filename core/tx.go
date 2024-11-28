package core

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/ipld/go-ipld-prime/datamodel"
	"github.com/ipld/go-ipld-prime/node/basicnode"
)

var ErrReadOnlyTx = errors.New("transaction is read only")

type Transaction struct {
	*DB
	readOnly bool
	rootLink datamodel.Link
	rootNode datamodel.Node
}

// ReadDocument returns the document in the given collection with the given unique id.
func (tx *Transaction) ReadDocument(ctx context.Context, collection, id string) (datamodel.Node, error) {
	return tx.GetNode(ctx, DocumentPath(collection, id), tx.rootNode)
}

// CreateDocument creates a new document in the collection with the given name and returns its unique id.
func (tx *Transaction) CreateDocument(ctx context.Context, collection string, node datamodel.Node) (string, error) {
	if tx.readOnly {
		return "", ErrReadOnlyTx
	}
	id, err := uuid.NewRandom()
	if err != nil {
		return "", err
	}
	lnk, err := tx.Store(ctx, node)
	if err != nil {
		return "", err
	}
	rootPath := DocumentPath(collection, id.String())
	rootNode, err := tx.SetNode(ctx, rootPath, tx.rootNode, basicnode.NewLink(lnk))
	if err != nil {
		return "", err
	}
	tx.rootNode = rootNode
	return id.String(), nil
}

// UpdateDocuments updates the document with the given id in the collection with the given name.
func (tx *Transaction) UpdateDocument(ctx context.Context, collection, id string, node datamodel.Node) error {
	if tx.readOnly {
		return ErrReadOnlyTx
	}
	lnk, err := tx.Store(ctx, node)
	if err != nil {
		return err
	}
	rootPath := DocumentPath(collection, id)
	rootNode, err := tx.SetNode(ctx, rootPath, tx.rootNode, basicnode.NewLink(lnk))
	if err != nil {
		return err
	}
	tx.rootNode = rootNode
	return nil
}

// DeleteDocument deletes the document with the given id in the collection with the given name.
func (tx *Transaction) DeleteDocument(ctx context.Context, collection, id string) error {
	if tx.readOnly {
		return ErrReadOnlyTx
	}
	rootPath := DocumentPath(collection, id)
	rootNode, err := tx.SetNode(ctx, rootPath, tx.rootNode, nil)
	if err != nil {
		return err
	}
	tx.rootNode = rootNode
	return nil
}

// DocumentIterator returns a new iterator that can be used to iterate through all documents in a collection.
func (tx *Transaction) DocumentIterator(ctx context.Context, collection string) (*DocumentIterator, error) {
	documentsPath := DocumentsPath(collection)
	documentsNode, err := tx.GetNode(ctx, documentsPath, tx.rootNode)
	if err != nil {
		return nil, err
	}
	iter := documentsNode.MapIterator()
	return &DocumentIterator{
		db: tx.DB,
		it: iter,
	}, nil
}

// Commit finalizes the transaction and updates the store root link.
func (tx *Transaction) Commit(ctx context.Context) error {
	if tx.readOnly {
		return nil
	}
	parentsNode, err := BuildRootParentsNode(tx.DB, tx.rootLink)
	if err != nil {
		return err
	}
	rootPath := datamodel.ParsePath(RootParentsFieldName)
	rootNode, err := tx.SetNode(ctx, rootPath, tx.rootNode, parentsNode)
	if err != nil {
		return err
	}
	rootLink, err := tx.Store(ctx, rootNode)
	if err != nil {
		return err
	}
	return tx.SetRootLink(ctx, rootLink)
}
