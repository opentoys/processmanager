package notifier

import (
	"encoding/json"
	"fmt"
	"processmanager/internal/utils"
	"regexp"
	"strings"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
)

// 自定义函数映射，供 expr 表达式使用
// regex(pattern, text) - 正则匹配
// jsonpath(expr, text) - JSONPath 查询（简易版，支持点号路径和数组索引）
var exprFunctions = map[string]any{
	"regex":    regexMatch,
	"jsonpath": jsonPathQuery,
}

// regexMatch 正则匹配函数
// 用法: regex(pattern, text) - 对 text 进行正则匹配
func regexMatch(pattern, text string) bool {
	matched, err := regexp.MatchString(pattern, text)
	if err != nil {
		return false
	}
	return matched
}

// jsonPathQuery JSONPath 查询函数（简易实现）
// 用法: jsonpath(expr, text) - 对 text 进行 JSON 解析后查询
// 支持: jsonpath("$.key", text), jsonpath("$.key.sub", text), jsonpath("$.arr[0]", text)
func jsonPathQuery(pathExpr, text string) (string, error) {
	var data any
	if err := json.Unmarshal([]byte(text), &data); err != nil {
		return "", fmt.Errorf("invalid json: %w", err)
	}

	res, err := queryJSONPath(data, pathExpr)
	if err != nil {
		return "", err
	}

	switch v := res.(type) {
	case string:
		return v, nil
	default:
		b, _ := json.Marshal(v)
		return string(b), nil
	}
}

// queryJSONPath 简易 JSONPath 查询
// 支持 $.key.sub.arr[0] 格式
func queryJSONPath(data any, pathExpr string) (any, error) {
	// 去除前导 $.
	path := pathExpr
	path = strings.TrimPrefix(path, "$.")
	path = strings.TrimPrefix(path, "$")

	if path == "" || path == "$" {
		return data, nil
	}

	current := data
	parts := strings.Split(path, ".")

	for _, part := range parts {
		if part == "" {
			continue
		}

		// 处理数组索引，如 arr[0]
		if idx := strings.Index(part, "["); idx >= 0 {
			key := part[:idx]
			if key != "" {
				current = getMapValue(current, key)
			}
			// 提取所有索引 [0][1]
			rest := part[idx:]
			for len(rest) > 0 && rest[0] == '[' {
				end := strings.Index(rest, "]")
				if end < 0 {
					break
				}
				indexStr := rest[1:end]
				var index int
				if _, err := fmt.Sscanf(indexStr, "%d", &index); err != nil {
					return nil, fmt.Errorf("invalid array index: %s", indexStr)
				}
				current = getArrayValue(current, index)
				rest = rest[end+1:]
			}
		} else {
			current = getMapValue(current, part)
		}

		if current == nil {
			return nil, fmt.Errorf("path not found: %s", pathExpr)
		}
	}

	return current, nil
}

// getMapValue 从 map 中获取值
func getMapValue(data any, key string) any {
	m, ok := data.(map[string]any)
	if !ok {
		return nil
	}
	return m[key]
}

// getArrayValue 从数组中获取值
func getArrayValue(data any, index int) any {
	arr, ok := data.([]any)
	if !ok {
		return nil
	}
	if index < 0 || index >= len(arr) {
		return nil
	}
	return arr[index]
}

// ruleEntry 编译后的规则条目
type ruleEntry struct {
	name     string
	channels []string
	program  *vm.Program
}

func (r *ruleEntry) match(line string) bool {
	if r.program == nil {
		return true
	}
	env := map[string]any{
		"line": line,
	}
	for k, v := range exprFunctions {
		env[k] = v
	}
	output, err := expr.Run(r.program, env)
	if err != nil {
		return false
	}
	result, ok := output.(bool)
	return ok && result
}

// compileRules 将原始规则配置编译为 ruleEntry
func compileRules(rules map[string]*utils.NoticeRule) map[string]*ruleEntry {
	entries := make(map[string]*ruleEntry, len(rules))
	for name, rule := range rules {
		var program *vm.Program
		if rule.Expr != "" {
			env := map[string]any{"line": ""}
			for k, v := range exprFunctions {
				env[k] = v
			}
			p, err := expr.Compile(rule.Expr, expr.Env(env), expr.AsBool())
			if err != nil {
				continue
			}
			program = p
		}
		entries[name] = &ruleEntry{
			name:     name,
			channels: rule.Channel,
			program:  program,
		}
	}
	return entries
}

// matchLine 判断日志行是否匹配给定的规则列表
// processName 用于匹配通配符 "*"
func matchLine(line string, processName string, rules map[string]*ruleEntry) []*ruleEntry {
	var matched []*ruleEntry
	for name, rule := range rules {
		if name != "*" && name != processName {
			continue
		}
		if rule.match(line) {
			matched = append(matched, rule)
		}
	}
	return matched
}

// parseRules 解析配置中的 notice 规则
func parseRules(raw map[string]utils.NoticeRule) map[string]*utils.NoticeRule {
	if raw == nil {
		return nil
	}
	rules := make(map[string]*utils.NoticeRule, len(raw))
	for name, r := range raw {
		rules[name] = &utils.NoticeRule{
			Expr:    strings.TrimSpace(r.Expr),
			Channel: r.Channel,
		}
	}
	return rules
}
