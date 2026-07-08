package api

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/Debjit28/sprig-db/sprig"
)

var sqlSelectRe = regexp.MustCompile(`(?is)^SELECT\s+(.+?)\s+FROM\s+([A-Za-z_][A-Za-z0-9_]*)(?:\s+WHERE\s+(.+?))?(?:\s+ORDER\s+BY\s+(.+?))?(?:\s+LIMIT\s+(\d+))?(?:\s+OFFSET\s+(\d+))?\s*;?\s*$`)

type sqlOrder struct {
	field string
	desc  bool
}

// ExecuteSQLQuery provides a richer SQL-like query mode for dashboard use.
func ExecuteSQLQuery(db *sprig.Sprig, owner, raw string) (*sprig.QueryResult, []string, error) {
	raw = strings.TrimSpace(raw)
	matches := sqlSelectRe.FindStringSubmatch(raw)
	if len(matches) != 7 {
		return nil, nil, fmt.Errorf("unsupported SQL syntax. Use: SELECT <fields> FROM <collection> [WHERE ...] [ORDER BY ...] [LIMIT n] [OFFSET n]")
	}

	selectClause := strings.TrimSpace(matches[1])
	collection := strings.TrimSpace(matches[2])
	whereClause := strings.TrimSpace(matches[3])
	orderClause := strings.TrimSpace(matches[4])
	limitClause := strings.TrimSpace(matches[5])
	offsetClause := strings.TrimSpace(matches[6])

	result, err := db.Coll(collection).Find()
	if err != nil {
		return nil, nil, err
	}

	records := result.Data
	filtered := make([]sprig.Map, 0, len(records))
	for _, r := range records {
		if rowOwner, ok := r["_owner"].(string); !ok || rowOwner != owner {
			continue
		}
		ok, err := evalWhereClause(r, whereClause)
		if err != nil {
			return nil, nil, err
		}
		if ok {
			filtered = append(filtered, r)
		}
	}

	orders, err := parseOrderClause(orderClause)
	if err != nil {
		return nil, nil, err
	}
	if len(orders) > 0 {
		sort.SliceStable(filtered, func(i, j int) bool {
			a, b := filtered[i], filtered[j]
			for _, o := range orders {
				av, aok := a[o.field]
				bv, bok := b[o.field]
				if !aok && !bok {
					continue
				}
				if !aok {
					return o.desc
				}
				if !bok {
					return !o.desc
				}
				cmp := compareAny(av, bv)
				if cmp == 0 {
					continue
				}
				if o.desc {
					return cmp > 0
				}
				return cmp < 0
			}
			return false
		})
	}

	limit := 50
	if limitClause != "" {
		parsed, err := strconv.Atoi(limitClause)
		if err != nil || parsed < 0 {
			return nil, nil, fmt.Errorf("invalid LIMIT value")
		}
		limit = parsed
	}

	offset := 0
	if offsetClause != "" {
		parsed, err := strconv.Atoi(offsetClause)
		if err != nil || parsed < 0 {
			return nil, nil, fmt.Errorf("invalid OFFSET value")
		}
		offset = parsed
	}

	total := len(filtered)
	if offset > total {
		offset = total
	}
	window := filtered[offset:]
	if limit > 0 && len(window) > limit {
		window = window[:limit]
	}

	projected, keys, err := projectColumns(window, selectClause)
	if err != nil {
		return nil, nil, err
	}

	return &sprig.QueryResult{
		Data:   projected,
		Total:  total,
		Offset: offset,
		Limit:  limit,
	}, keys, nil
}

func projectColumns(records []sprig.Map, selectClause string) ([]sprig.Map, []string, error) {
	if selectClause == "*" {
		return records, nil, nil
	}
	fields := splitCSV(selectClause)
	if len(fields) == 0 {
		return nil, nil, fmt.Errorf("SELECT list is empty")
	}

	keys := make([]string, 0, len(fields))
	for _, f := range fields {
		field := strings.TrimSpace(f)
		if field == "" {
			return nil, nil, fmt.Errorf("invalid empty field in SELECT")
		}
		keys = append(keys, field)
	}

	out := make([]sprig.Map, 0, len(records))
	for _, rec := range records {
		row := sprig.Map{}
		for _, k := range keys {
			row[k] = rec[k]
		}
		out = append(out, row)
	}
	return out, keys, nil
}

func parseOrderClause(clause string) ([]sqlOrder, error) {
	if clause == "" {
		return nil, nil
	}
	parts := splitCSV(clause)
	orders := make([]sqlOrder, 0, len(parts))
	for _, p := range parts {
		toks := strings.Fields(strings.TrimSpace(p))
		if len(toks) == 0 {
			continue
		}
		order := sqlOrder{field: toks[0]}
		if len(toks) > 1 {
			dir := strings.ToUpper(toks[1])
			if dir == "DESC" {
				order.desc = true
			} else if dir != "ASC" {
				return nil, fmt.Errorf("invalid ORDER BY direction for %q", toks[0])
			}
		}
		orders = append(orders, order)
	}
	return orders, nil
}

func evalWhereClause(rec sprig.Map, clause string) (bool, error) {
	if clause == "" {
		return true, nil
	}
	orParts := splitLogical(clause, "OR")
	for _, orPart := range orParts {
		andParts := splitLogical(orPart, "AND")
		allAnd := true
		for _, cond := range andParts {
			ok, err := evalCondition(rec, strings.TrimSpace(cond))
			if err != nil {
				return false, err
			}
			if !ok {
				allAnd = false
				break
			}
		}
		if allAnd {
			return true, nil
		}
	}
	return false, nil
}

func evalCondition(rec sprig.Map, cond string) (bool, error) {
	re := regexp.MustCompile(`(?is)^([A-Za-z_][A-Za-z0-9_]*)\s*(=|!=|>=|<=|>|<|LIKE|IN)\s*(.+)$`)
	m := re.FindStringSubmatch(strings.TrimSpace(cond))
	if len(m) != 4 {
		return false, fmt.Errorf("invalid WHERE condition: %q", cond)
	}
	field := strings.TrimSpace(m[1])
	op := strings.ToUpper(strings.TrimSpace(m[2]))
	rhs := strings.TrimSpace(m[3])

	lhsVal, ok := rec[field]
	if !ok {
		return false, nil
	}

	switch op {
	case "IN":
		if !strings.HasPrefix(rhs, "(") || !strings.HasSuffix(rhs, ")") {
			return false, fmt.Errorf("IN requires parenthesized list")
		}
		listRaw := strings.TrimSpace(rhs[1 : len(rhs)-1])
		items := splitCSV(listRaw)
		for _, item := range items {
			rv, err := parseLiteral(strings.TrimSpace(item))
			if err != nil {
				return false, err
			}
			if compareAny(lhsVal, rv) == 0 {
				return true, nil
			}
		}
		return false, nil
	case "LIKE":
		rv, err := parseLiteral(rhs)
		if err != nil {
			return false, err
		}
		pattern, ok := rv.(string)
		if !ok {
			return false, fmt.Errorf("LIKE pattern must be string")
		}
		text := fmt.Sprintf("%v", lhsVal)
		return likeMatch(text, pattern), nil
	default:
		rv, err := parseLiteral(rhs)
		if err != nil {
			return false, err
		}
		cmp := compareAny(lhsVal, rv)
		switch op {
		case "=":
			return cmp == 0, nil
		case "!=":
			return cmp != 0, nil
		case ">":
			return cmp > 0, nil
		case "<":
			return cmp < 0, nil
		case ">=":
			return cmp >= 0, nil
		case "<=":
			return cmp <= 0, nil
		}
	}
	return false, fmt.Errorf("unsupported operator %q", op)
}

func likeMatch(text, pattern string) bool {
	escaped := regexp.QuoteMeta(pattern)
	escaped = strings.ReplaceAll(escaped, "%", ".*")
	escaped = strings.ReplaceAll(escaped, "_", ".")
	re, err := regexp.Compile("^" + escaped + "$")
	if err != nil {
		return false
	}
	return re.MatchString(text)
}

func parseLiteral(s string) (any, error) {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && ((s[0] == '\'' && s[len(s)-1] == '\'') || (s[0] == '"' && s[len(s)-1] == '"')) {
		return s[1 : len(s)-1], nil
	}
	if strings.EqualFold(s, "true") {
		return true, nil
	}
	if strings.EqualFold(s, "false") {
		return false, nil
	}
	if n, err := strconv.ParseFloat(s, 64); err == nil {
		return n, nil
	}
	return s, nil
}

func compareAny(a, b any) int {
	if af, aok := toFloat64(a); aok {
		if bf, bok := toFloat64(b); bok {
			switch {
			case af < bf:
				return -1
			case af > bf:
				return 1
			default:
				return 0
			}
		}
	}

	as := strings.ToLower(fmt.Sprintf("%v", a))
	bs := strings.ToLower(fmt.Sprintf("%v", b))
	switch {
	case as < bs:
		return -1
	case as > bs:
		return 1
	default:
		return 0
	}
}

func toFloat64(v any) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case int8:
		return float64(n), true
	case int16:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case uint:
		return float64(n), true
	case uint8:
		return float64(n), true
	case uint16:
		return float64(n), true
	case uint32:
		return float64(n), true
	case uint64:
		return float64(n), true
	case float32:
		return float64(n), true
	case float64:
		return n, true
	default:
		return 0, false
	}
}

func splitLogical(input, keyword string) []string {
	parts := []string{}
	var b strings.Builder
	inQuote := rune(0)
	upperKeyword := " " + keyword + " "
	for i := 0; i < len(input); i++ {
		ch := rune(input[i])
		if inQuote != 0 {
			if ch == inQuote {
				inQuote = 0
			}
			b.WriteByte(input[i])
			continue
		}
		if ch == '\'' || ch == '"' {
			inQuote = ch
			b.WriteByte(input[i])
			continue
		}

		remaining := input[i:]
		if len(remaining) >= len(upperKeyword) && strings.EqualFold(remaining[:len(upperKeyword)], upperKeyword) {
			parts = append(parts, strings.TrimSpace(b.String()))
			b.Reset()
			i += len(upperKeyword) - 1
			continue
		}
		b.WriteByte(input[i])
	}
	parts = append(parts, strings.TrimSpace(b.String()))
	return parts
}

func splitCSV(input string) []string {
	parts := []string{}
	var b strings.Builder
	inQuote := rune(0)
	for i := 0; i < len(input); i++ {
		ch := rune(input[i])
		if inQuote != 0 {
			if ch == inQuote {
				inQuote = 0
			}
			b.WriteByte(input[i])
			continue
		}
		if ch == '\'' || ch == '"' {
			inQuote = ch
			b.WriteByte(input[i])
			continue
		}
		if ch == ',' {
			parts = append(parts, strings.TrimSpace(b.String()))
			b.Reset()
			continue
		}
		b.WriteByte(input[i])
	}
	parts = append(parts, strings.TrimSpace(b.String()))
	return parts
}
