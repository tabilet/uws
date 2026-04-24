package uws1

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/antchfx/xmlquery"
	"github.com/antchfx/xpath"
)

func (o *Orchestrator) criteriaMatchAll(ctx context.Context, criteria []*Criterion) (bool, error) {
	if len(criteria) == 0 {
		return true, nil
	}
	for _, criterion := range criteria {
		if criterion == nil {
			continue
		}
		ok, err := o.evaluateCriterion(ctx, criterion)
		if err != nil {
			return false, fmt.Errorf("evaluating criterion %q (%s): %w", criterion.Condition, criterion.Type, err)
		}
		if !ok {
			return false, nil
		}
	}
	return true, nil
}

func (o *Orchestrator) evaluateCriterion(ctx context.Context, criterion *Criterion) (bool, error) {
	if criterion == nil {
		return true, nil
	}
	switch criterion.Type {
	case "", CriterionSimple:
		return o.evaluateTruthy(ctx, criterion.Condition)
	case CriterionRegex:
		return o.evaluateRegexCriterion(ctx, criterion)
	case CriterionJSONPath:
		return o.evaluateJSONPathCriterion(ctx, criterion)
	case CriterionXPath:
		return o.evaluateXPathCriterion(ctx, criterion)
	default:
		return false, fmt.Errorf("uws1: criterion type %q is not executable by the core orchestrator", criterion.Type)
	}
}

func (o *Orchestrator) evaluateRegexCriterion(ctx context.Context, criterion *Criterion) (bool, error) {
	source, err := o.Runtime.EvaluateExpression(ctx, criterion.Context)
	if err != nil {
		return false, fmt.Errorf("evaluating regex context %q: %w", criterion.Context, err)
	}
	pattern := strings.TrimSpace(criterion.Condition)
	re, err := regexp.Compile(pattern)
	if err != nil {
		return false, fmt.Errorf("compile regex %q: %w", pattern, err)
	}
	text, err := criterionSourceString(source)
	if err != nil {
		return false, err
	}
	return re.MatchString(text), nil
}

func (o *Orchestrator) evaluateJSONPathCriterion(ctx context.Context, criterion *Criterion) (bool, error) {
	source, err := o.Runtime.EvaluateExpression(ctx, criterion.Context)
	if err != nil {
		return false, fmt.Errorf("evaluating jsonpath context %q: %w", criterion.Context, err)
	}
	target := normalizeCriterionTarget(criterion.Context, criterion.Condition)
	switch {
	case target == "":
		return truthyValue(source), nil
	case strings.HasPrefix(target, "#"):
		value, err := resolveCriterionJSONPointer(source, target)
		if err != nil {
			return false, err
		}
		return truthyValue(value), nil
	case strings.HasPrefix(target, "/"):
		value, err := resolveCriterionJSONPointer(source, "#"+target)
		if err != nil {
			return false, err
		}
		return truthyValue(value), nil
	default:
		value, err := o.Runtime.EvaluateExpression(ctx, criterion.Condition)
		if err != nil {
			return false, fmt.Errorf("evaluating jsonpath condition %q: %w", criterion.Condition, err)
		}
		return truthyValue(value), nil
	}
}

func (o *Orchestrator) evaluateXPathCriterion(ctx context.Context, criterion *Criterion) (bool, error) {
	source, err := o.Runtime.EvaluateExpression(ctx, criterion.Context)
	if err != nil {
		return false, fmt.Errorf("evaluating xpath context %q: %w", criterion.Context, err)
	}
	xmlText, err := criterionSourceString(source)
	if err != nil {
		return false, err
	}
	root, err := xmlquery.Parse(strings.NewReader(strings.TrimSpace(xmlText)))
	if err != nil {
		return false, fmt.Errorf("parse xpath XML: %w", err)
	}
	target := normalizeCriterionTarget(criterion.Context, criterion.Condition)
	if target == "" {
		return truthyValue(xmlText), nil
	}
	compiled, err := xpath.Compile(target)
	if err != nil {
		return false, fmt.Errorf("compile xpath %q: %w", target, err)
	}
	value := compiled.Evaluate(xmlquery.CreateXPathNavigator(root))
	return truthyXPathValue(value), nil
}

func normalizeCriterionTarget(contextExpr, condition string) string {
	contextExpr = strings.TrimSpace(contextExpr)
	condition = strings.TrimSpace(condition)
	if contextExpr != "" && strings.HasPrefix(condition, contextExpr) {
		return strings.TrimSpace(strings.TrimPrefix(condition, contextExpr))
	}
	return condition
}

func criterionSourceString(value any) (string, error) {
	switch typed := value.(type) {
	case nil:
		return "", nil
	case string:
		return typed, nil
	case []byte:
		return string(typed), nil
	default:
		if data, err := json.Marshal(typed); err == nil {
			return string(data), nil
		}
		return fmt.Sprint(typed), nil
	}
}

func truthyValue(value any) bool {
	switch typed := value.(type) {
	case nil:
		return false
	case bool:
		return typed
	case string:
		return typed != ""
	case int:
		return typed != 0
	case int64:
		return typed != 0
	case float64:
		return typed != 0
	case []any:
		return len(typed) > 0
	case map[string]any:
		return len(typed) > 0
	default:
		return true
	}
}

func resolveCriterionJSONPointer(root any, pointer string) (any, error) {
	if pointer == "" || pointer == "#" {
		return root, nil
	}
	if !strings.HasPrefix(pointer, "#") {
		return nil, fmt.Errorf("invalid JSON pointer %q", pointer)
	}
	path := strings.TrimPrefix(pointer, "#")
	if path == "" {
		return root, nil
	}
	if !strings.HasPrefix(path, "/") {
		return nil, fmt.Errorf("invalid JSON pointer %q", pointer)
	}
	current := root
	for _, rawToken := range strings.Split(path[1:], "/") {
		token := strings.ReplaceAll(strings.ReplaceAll(rawToken, "~1", "/"), "~0", "~")
		switch typed := current.(type) {
		case map[string]any:
			current = typed[token]
		case []any:
			index, err := parseCriterionIndex(token)
			if err != nil {
				return nil, err
			}
			if index < 0 || index >= len(typed) {
				return nil, nil
			}
			current = typed[index]
		default:
			return nil, nil
		}
	}
	return current, nil
}

func parseCriterionIndex(token string) (int, error) {
	var index int
	if _, err := fmt.Sscanf(token, "%d", &index); err != nil {
		return 0, fmt.Errorf("invalid array index %q", token)
	}
	return index, nil
}

func truthyXPathValue(value any) bool {
	switch typed := value.(type) {
	case nil:
		return false
	case bool:
		return typed
	case string:
		return typed != ""
	case float64:
		return typed != 0
	case *xpath.NodeIterator:
		if typed == nil {
			return false
		}
		return typed.MoveNext()
	default:
		return truthyValue(value)
	}
}
