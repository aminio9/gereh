// Package engine compiles and evaluates bounded CEL policy expressions.
package engine

import (
	"context"
	"fmt"
	"sync"

	"github.com/aminio9/gereh/services/policy-approval/internal/domain"
	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	exprpb "google.golang.org/genproto/googleapis/api/expr/v1alpha1"
)

const (
	maxExpressionBytes = 4096
	maxExpressionNodes = 512
	maxExpressionDepth = 32
	maxEvaluationCost  = uint64(10000)
)

type compiledExpression struct {
	program   cel.Program
	nodeCount int
}

// CEL compiles and evaluates bounded policy expressions.
type CEL struct {
	environment *cel.Env
	cache       sync.Map
}

// NewCEL creates the bounded CEL evaluation environment.
func NewCEL() (*CEL, error) {
	environment, err := cel.NewEnv(
		cel.Variable(
			"subject",
			cel.MapType(cel.StringType, cel.DynType),
		),
		cel.Variable(
			"resource",
			cel.MapType(cel.StringType, cel.DynType),
		),
		cel.Variable(
			"request",
			cel.MapType(cel.StringType, cel.DynType),
		),
		cel.Variable(
			"context",
			cel.MapType(cel.StringType, cel.DynType),
		),
		cel.ParserExpressionSizeLimit(maxExpressionBytes),
		cel.ParserRecursionLimit(maxExpressionDepth),
	)
	if err != nil {
		return nil, fmt.Errorf(
			"create CEL environment: %w",
			err,
		)
	}

	return &CEL{
		environment: environment,
	}, nil
}

// Validate reports whether the expression compiles as a bounded bool.
func (engine *CEL) Validate(expression string) error {
	_, err := engine.compile(expression)
	return err
}

// Evaluate evaluates the expression against bounded runtime cost.
func (engine *CEL) Evaluate(
	ctx context.Context,
	cacheKey string,
	expression string,
	variables map[string]any,
) (bool, error) {
	value, ok := engine.cache.Load(cacheKey)
	if !ok {
		compiled, err := engine.compile(expression)
		if err != nil {
			return false, err
		}

		actual, _ := engine.cache.LoadOrStore(cacheKey, compiled)
		value = actual
	}

	compiled, ok := value.(*compiledExpression)
	if !ok {
		return false, fmt.Errorf(
			"invalid CEL cache value",
		)
	}

	result, _, err := compiled.program.ContextEval(ctx, variables)
	if err != nil {
		return false, fmt.Errorf(
			"%w: %w",
			domain.ErrExpressionCostExceeded,
			err,
		)
	}

	if result == types.True {
		return true, nil
	}

	if result == types.False {
		return false, nil
	}

	return false, fmt.Errorf(
		"%w: CEL expression returned %s instead of bool",
		domain.ErrInvalidExpression,
		result.Type(),
	)
}

func (engine *CEL) compile(
	expression string,
) (*compiledExpression, error) {
	ast, issues := engine.environment.Compile(expression)
	if issues != nil && issues.Err() != nil {
		return nil, fmt.Errorf(
			"%w: %w",
			domain.ErrInvalidExpression,
			issues.Err(),
		)
	}

	if ast.OutputType().String() != "bool" {
		return nil, fmt.Errorf(
			"%w: expression result must be bool",
			domain.ErrInvalidExpression,
		)
	}

	checked, err := cel.AstToCheckedExpr(ast)
	if err != nil {
		return nil, fmt.Errorf(
			"%w: %w",
			domain.ErrInvalidExpression,
			err,
		)
	}

	count := countExpressionNodes(checked.GetExpr())
	if count > maxExpressionNodes {
		return nil, fmt.Errorf(
			"%w: expression exceeds node limit",
			domain.ErrInvalidExpression,
		)
	}

	program, err := engine.environment.Program(
		ast,
		cel.EvalOptions(cel.OptOptimize, cel.OptTrackCost),
		cel.CostLimit(maxEvaluationCost),
	)
	if err != nil {
		return nil, fmt.Errorf(
			"%w: %w",
			domain.ErrInvalidExpression,
			err,
		)
	}

	return &compiledExpression{
		program:   program,
		nodeCount: count,
	}, nil
}

// countExpressionNodes returns the number of expression nodes in the tree.
func countExpressionNodes(expr *exprpb.Expr) int {
	if expr == nil {
		return 0
	}

	count := 1
	switch kind := expr.GetExprKind().(type) {
	case *exprpb.Expr_SelectExpr:
		count += countExpressionNodes(kind.SelectExpr.GetOperand())

	case *exprpb.Expr_ListExpr:
		for _, element := range kind.ListExpr.GetElements() {
			count += countExpressionNodes(element)
		}

	case *exprpb.Expr_StructExpr:
		for _, entry := range kind.StructExpr.GetEntries() {
			count++
			count += countExpressionNodes(entry.GetValue())
		}

	case *exprpb.Expr_CallExpr:
		count += countExpressionNodes(kind.CallExpr.GetTarget())
		for _, argument := range kind.CallExpr.GetArgs() {
			count += countExpressionNodes(argument)
		}

	case *exprpb.Expr_ComprehensionExpr:
		comprehension := kind.ComprehensionExpr
		count += countExpressionNodes(comprehension.GetIterRange())
		count += countExpressionNodes(comprehension.GetAccuInit())
		count += countExpressionNodes(comprehension.GetLoopCondition())
		count += countExpressionNodes(comprehension.GetLoopStep())
		count += countExpressionNodes(comprehension.GetResult())
	}

	return count
}
