package main

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type contextKey int

const (
	contextKeyBegin contextKey = iota
)

// Hooks satisfies the sqlhook.Hooks interface
type Hooks struct {}

// Before hook will print the query with it's args and return the context with the timestamp
func (h *Hooks) Before(ctx context.Context, query string, args ...interface{}) (context.Context, error) {
	// fmt.Printf("> %s %q", query, args)
	return context.WithValue(ctx, contextKeyBegin, time.Now()), nil
}

// After hook will get the timestamp registered on the Before hook and print the elapsed time
func (h *Hooks) After(ctx context.Context, query string, args ...interface{}) (context.Context, error) {
	begin := ctx.Value(contextKeyBegin).(time.Time)
	sql := fmt.Sprintf(strings.ReplaceAll(query, "?", "%s"), args)
	logger.With("time", time.Since(begin).Milliseconds(), "query", sql).Debugf("SQL Query")
	return ctx, nil
}

