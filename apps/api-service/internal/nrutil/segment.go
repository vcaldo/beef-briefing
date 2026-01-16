// Package nrutil provides utilities for New Relic instrumentation.
package nrutil

import (
	"context"

	"github.com/newrelic/go-agent/v3/newrelic"
)

// StartSegment starts a New Relic segment for the given context and returns
// a function that ends the segment. This allows for clean defer usage:
//
//	defer nrutil.StartSegment(ctx, "ServiceName.MethodName")()
//
// If no transaction is found in the context, a no-op function is returned.
// This makes the helper safe to use even when New Relic is not configured.
func StartSegment(ctx context.Context, name string) func() {
	txn := newrelic.FromContext(ctx)
	if txn == nil {
		return func() {}
	}
	segment := txn.StartSegment(name)
	return segment.End
}

// StartDatastoreSegment starts a New Relic datastore segment and returns
// a function that ends the segment. This is useful for tracking database operations:
//
//	defer nrutil.StartDatastoreSegment(ctx, newrelic.DatastorePostgres, "users", "SELECT")()
//
// If no transaction is found in the context, a no-op function is returned.
func StartDatastoreSegment(ctx context.Context, product newrelic.DatastoreProduct, collection, operation string) func() {
	txn := newrelic.FromContext(ctx)
	if txn == nil {
		return func() {}
	}
	segment := newrelic.DatastoreSegment{
		StartTime:  txn.StartSegmentNow(),
		Product:    product,
		Collection: collection,
		Operation:  operation,
	}
	return segment.End
}

// StartExternalSegment starts a New Relic external segment and returns
// a function that ends the segment. This is useful for tracking external HTTP calls:
//
//	defer nrutil.StartExternalSegment(ctx, "https://api.example.com/users")()
//
// If no transaction is found in the context, a no-op function is returned.
func StartExternalSegment(ctx context.Context, url string) func() {
	txn := newrelic.FromContext(ctx)
	if txn == nil {
		return func() {}
	}
	segment := newrelic.ExternalSegment{
		StartTime: txn.StartSegmentNow(),
		URL:       url,
	}
	return segment.End
}

// NoticeError records an error to New Relic if a transaction exists in the context.
// This is a no-op if no transaction is found in the context.
//
//	nrutil.NoticeError(ctx, err)
func NoticeError(ctx context.Context, err error) {
	if err == nil {
		return
	}
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		txn.NoticeError(err)
	}
}
