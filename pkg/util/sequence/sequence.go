// Copyright 2020 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

package sequence

import (
	"github.com/cockroachdb/cockroach/pkg/sql/pgwire/pgcode"
	"github.com/cockroachdb/cockroach/pkg/sql/pgwire/pgerror"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/builtins"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
	"github.com/cockroachdb/cockroach/pkg/sql/sessiondata"
)

// SeqIdentifier wraps together different ways of identifying a sequence.
// The sequence can either be identified via either its name, or its ID.
type SeqIdentifier struct {
	SeqName string
	SeqID   int64
}

// IsByID indicates whether the SeqIdentifier is identifying
// the sequence by its ID or by its name.
func (si *SeqIdentifier) IsByID() bool {
	return len(si.SeqName) == 0
}

// GetSequenceFromFunc extracts a sequence identifier from a FuncExpr if the function
// takes a sequence identifier as an arg (a sequence identifier can either be
// a sequence name or an ID), wrapped in the SeqIdentifier type.
// Returns the identifier of the sequence or nil if no sequence was found.
func GetSequenceFromFunc(funcExpr *tree.FuncExpr) (*SeqIdentifier, error) {
	searchPath := sessiondata.SearchPath{}

	// Resolve doesn't use the searchPath for resolving FunctionDefinitions
	// so we can pass in an empty SearchPath.
	def, err := funcExpr.Func.Resolve(searchPath)
	if err != nil {
		return nil, err
	}

	fnProps, overloads := builtins.GetBuiltinProperties(def.Name)
	if fnProps != nil && fnProps.HasSequenceArguments {
		found := false
		for _, overload := range overloads {
			// Find the overload that matches funcExpr.
			if funcExpr.ResolvedOverload().Types.Match(overload.Types.Types()) {
				found = true
				argTypes, ok := overload.Types.(tree.ArgTypes)
				if !ok {
					panic(pgerror.Newf(
						pgcode.InvalidFunctionDefinition,
						"%s has invalid argument types", funcExpr.Func.String(),
					))
				}
				for i := 0; i < overload.Types.Length(); i++ {
					// Find the sequence name arg.
					argName := argTypes[i].Name
					if argName == builtins.SequenceNameArg {
						arg := funcExpr.Exprs[i]
						switch a := arg.(type) {
						case *tree.DString:
							seqName := string(*a)
							return &SeqIdentifier{
								SeqName: seqName,
							}, nil
						case *tree.DOid:
							id := int64(a.DInt)
							return &SeqIdentifier{
								SeqID: id,
							}, nil
						}
					}
				}
			}
		}
		if !found {
			panic(pgerror.New(
				pgcode.DatatypeMismatch,
				"could not find matching function overload for given arguments",
			))
		}
	}
	return nil, nil
}

// GetUsedSequences returns the identifier of the sequence passed to
// a call to sequence function in the given expression or nil if no sequence
// identifiers are found. The identifier is wrapped in a SeqIdentifier.
// e.g. nextval('foo') => "foo"; <some other expression> => nil
func GetUsedSequences(defaultExpr tree.TypedExpr) ([]SeqIdentifier, error) {
	var seqIdentifiers []SeqIdentifier
	_, err := tree.SimpleVisit(
		defaultExpr,
		func(expr tree.Expr) (recurse bool, newExpr tree.Expr, err error) {
			switch t := expr.(type) {
			case *tree.FuncExpr:
				identifier, err := GetSequenceFromFunc(t)
				if err != nil {
					return false, nil, err
				}
				if identifier != nil {
					seqIdentifiers = append(seqIdentifiers, *identifier)
				}
			}
			return true, expr, nil
		},
	)
	if err != nil {
		return nil, err
	}
	return seqIdentifiers, nil
}
