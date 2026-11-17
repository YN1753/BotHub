package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/YN1753/vesper/bothub/internal/store"
	"github.com/robfig/cron/v3"
)

func calculateEvalToolSpec() ToolSpec {
	return ToolSpec{
		Definition: ToolDefinition{
			Type: "function",
			Function: ToolFunction{
				Name:        "calculate_eval",
				Description: "高精度数学与财务表达式计算器，支持四则运算、乘方(^)、括号、百分比(如 15%) 及 round/abs/min/max/sqrt/floor/ceil 函数，避免估算误差。",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"expression": map[string]any{
							"type":        "string",
							"description": "数学表达式，如 (198.5 * 12) * 0.85 或 round(100 / 3, 2)",
						},
					},
					"required": []string{"expression"},
				},
			},
		},
		ReadOnly:       true,
		StatusLabel:    "正在精确核算数值...",
		DomainKeywords: []string{"计算", "算一下", "多少", "公式", "税后", "利率", "百分比"},
		Execute: func(ctx context.Context, e *Engine, cleanArgs string) (string, error) {
			return e.toolCalculateEval(ctx, cleanArgs)
		},
	}
}

func datetimeCalcToolSpec() ToolSpec {
	return ToolSpec{
		Definition: ToolDefinition{
			Type: "function",
			Function: ToolFunction{
				Name:        "datetime_calc",
				Description: "精确获取或推算当前上海时间、星期、相对日期偏移（如 30 分钟后 / 7 天后可用于设定 run_at 提醒）以及预测 Cron 下 5 次执行时刻。",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"action": map[string]any{
							"type":        "string",
							"enum":        []string{"now", "add", "diff", "cron_next"},
							"description": "操作类型：now | add | diff | cron_next",
						},
						"base_time": map[string]any{
							"type":        "string",
							"description": "可选：基准时间 YYYY-MM-DD HH:MM（缺省为当前时间）",
						},
						"target_time": map[string]any{
							"type":        "string",
							"description": "当 action=diff 时必填：目标时间 YYYY-MM-DD HH:MM 或 YYYY-MM-DD",
						},
						"delta_minutes": map[string]any{
							"type":        "integer",
							"description": "分钟偏移量（可正可负）",
						},
						"delta_hours": map[string]any{
							"type":        "integer",
							"description": "小时偏移量（可正可负）",
						},
						"delta_days": map[string]any{
							"type":        "integer",
							"description": "天数偏移量（可正可负）",
						},
						"cron_expr": map[string]any{
							"type":        "string",
							"description": "当 action=cron_next 时必填的 5 段 Cron 表达式",
						},
					},
				},
			},
		},
		ReadOnly:       true,
		StatusLabel:    "正在推算日期与排期...",
		DomainKeywords: []string{"几天", "倒计时", "工作日", "下周", "星期", "时区", "utc"},
		Execute: func(ctx context.Context, e *Engine, cleanArgs string) (string, error) {
			return e.toolDatetimeCalc(ctx, cleanArgs)
		},
	}
}

// Safe recursive-descent math expression evaluator (no external shells or eval).
type mathParser struct {
	src string
	pos int
}

func (p *mathParser) skipSpace() {
	for p.pos < len(p.src) && unicode.IsSpace(rune(p.src[p.pos])) {
		p.pos++
	}
}

func (p *mathParser) peek() byte {
	p.skipSpace()
	if p.pos >= len(p.src) {
		return 0
	}
	return p.src[p.pos]
}

func (p *mathParser) parseExpr() (float64, error) {
	v, err := p.parseTerm()
	if err != nil {
		return 0, err
	}
	for {
		ch := p.peek()
		if ch != '+' && ch != '-' {
			break
		}
		p.pos++
		rhs, err := p.parseTerm()
		if err != nil {
			return 0, err
		}
		if ch == '+' {
			v += rhs
		} else {
			v -= rhs
		}
	}
	return v, nil
}

func (p *mathParser) parseTerm() (float64, error) {
	v, err := p.parsePower()
	if err != nil {
		return 0, err
	}
	for {
		ch := p.peek()
		if ch != '*' && ch != '/' && ch != '%' {
			break
		}
		p.pos++
		rhs, err := p.parsePower()
		if err != nil {
			return 0, err
		}
		switch ch {
		case '*':
			v *= rhs
		case '/':
			if rhs == 0 {
				return 0, fmt.Errorf("除数不能为 0")
			}
			v /= rhs
		case '%':
			if rhs == 0 {
				return 0, fmt.Errorf("取模除数不能为 0")
			}
			v = math.Mod(v, rhs)
		}
	}
	return v, nil
}

func (p *mathParser) parsePower() (float64, error) {
	base, err := p.parseUnary()
	if err != nil {
		return 0, err
	}
	if p.peek() == '^' {
		p.pos++
		exp, err := p.parsePower()
		if err != nil {
			return 0, err
		}
		return math.Pow(base, exp), nil
	}
	return base, nil
}

func (p *mathParser) parseUnary() (float64, error) {
	ch := p.peek()
	if ch == '+' {
		p.pos++
		return p.parseUnary()
	}
	if ch == '-' {
		p.pos++
		v, err := p.parseUnary()
		return -v, err
	}
	return p.parsePrimary()
}

func (p *mathParser) parsePrimary() (float64, error) {
	ch := p.peek()
	if ch == '(' {
		p.pos++
		v, err := p.parseExpr()
		if err != nil {
			return 0, err
		}
		if p.peek() != ')' {
			return 0, fmt.Errorf("括号未闭合")
		}
		p.pos++
		if p.peek() == '%' {
			p.pos++
			v /= 100
		}
		return v, nil
	}
	if unicode.IsLetter(rune(ch)) {
		start := p.pos
		for p.pos < len(p.src) && (unicode.IsLetter(rune(p.src[p.pos])) || unicode.IsDigit(rune(p.src[p.pos])) || p.src[p.pos] == '_') {
			p.pos++
		}
		ident := strings.ToLower(p.src[start:p.pos])
		if p.peek() != '(' {
			switch ident {
			case "pi":
				return math.Pi, nil
			case "e":
				return math.E, nil
			default:
				return 0, fmt.Errorf("未知常量: %s", ident)
			}
		}
		p.pos++
		var args []float64
		if p.peek() != ')' {
			for {
				a, err := p.parseExpr()
				if err != nil {
					return 0, err
				}
				args = append(args, a)
				if p.peek() == ',' {
					p.pos++
					continue
				}
				break
			}
		}
		if p.peek() != ')' {
			return 0, fmt.Errorf("函数 %s 缺少右括号", ident)
		}
		p.pos++
		return evalMathFunc(ident, args)
	}
	if (ch >= '0' && ch <= '9') || ch == '.' {
		start := p.pos
		dotSeen := false
		for p.pos < len(p.src) {
			c := p.src[p.pos]
			if c >= '0' && c <= '9' {
				p.pos++
			} else if c == '.' && !dotSeen {
				dotSeen = true
				p.pos++
			} else {
				break
			}
		}
		val, err := strconv.ParseFloat(p.src[start:p.pos], 64)
		if err != nil {
			return 0, fmt.Errorf("无效数字: %s", p.src[start:p.pos])
		}
		if p.peek() == '%' && (p.pos+1 >= len(p.src) || !(p.src[p.pos+1] >= '0' && p.src[p.pos+1] <= '9')) {
			after := p.pos + 1
			for after < len(p.src) && unicode.IsSpace(rune(p.src[after])) {
				after++
			}
			if after >= len(p.src) || strings.ContainsRune("+-*/^),", rune(p.src[after])) {
				p.pos++
				val /= 100
			}
		}
		return val, nil
	}
	return 0, fmt.Errorf("无法解析表达式位置 %d", p.pos)
}

func evalMathFunc(name string, args []float64) (float64, error) {
	switch name {
	case "abs":
		if len(args) != 1 {
			return 0, fmt.Errorf("abs 需要 1 个参数")
		}
		return math.Abs(args[0]), nil
	case "sqrt":
		if len(args) != 1 || args[0] < 0 {
			return 0, fmt.Errorf("sqrt 需要 1 个非负参数")
		}
		return math.Sqrt(args[0]), nil
	case "floor":
		if len(args) != 1 {
			return 0, fmt.Errorf("floor 需要 1 个参数")
		}
		return math.Floor(args[0]), nil
	case "ceil":
		if len(args) != 1 {
			return 0, fmt.Errorf("ceil 需要 1 个参数")
		}
		return math.Ceil(args[0]), nil
	case "round":
		if len(args) < 1 || len(args) > 2 {
			return 0, fmt.Errorf("round 需要 1 或 2 个参数")
		}
		digits := 0
		if len(args) == 2 {
			digits = int(args[1])
		}
		factor := math.Pow(10, float64(digits))
		return math.Round(args[0]*factor) / factor, nil
	case "min":
		if len(args) < 1 {
			return 0, fmt.Errorf("min 至少需要 1 个参数")
		}
		m := args[0]
		for _, v := range args[1:] {
			if v < m {
				m = v
			}
		}
		return m, nil
	case "max":
		if len(args) < 1 {
			return 0, fmt.Errorf("max 至少需要 1 个参数")
		}
		m := args[0]
		for _, v := range args[1:] {
			if v > m {
				m = v
			}
		}
		return m, nil
	default:
		return 0, fmt.Errorf("不支持的函数: %s", name)
	}
}

func (e *Engine) toolCalculateEval(_ context.Context, argsJSON string) (string, error) {
	var args struct {
		Expression string `json:"expression"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("解析 calculate_eval 参数失败: %w", err)
	}
	expr := strings.TrimSpace(args.Expression)
	if expr == "" {
		return "", fmt.Errorf("expression 不能为空")
	}
	p := &mathParser{src: expr}
	val, err := p.parseExpr()
	if err != nil {
		return "", err
	}
	if ch := p.peek(); ch != 0 {
		return "", fmt.Errorf("表达式末尾存在多余字符: %q", string(ch))
	}
	if math.IsNaN(val) || math.IsInf(val, 0) {
		return "", fmt.Errorf("计算结果超出有限实数范围")
	}
	rounded := math.Round(val*1e8) / 1e8
	out, _ := json.Marshal(map[string]any{
		"ok":         true,
		"expression": expr,
		"result":     rounded,
		"formatted":  strconv.FormatFloat(rounded, 'f', -1, 64),
	})
	return string(out), nil
}

func weekdayCN(w time.Weekday) string {
	names := [...]string{"周日", "周一", "周二", "周三", "周四", "周五", "周六"}
	return names[int(w)%7]
}

func parseDatetimeFlex(raw string, loc *time.Location) (time.Time, error) {
	for _, layout := range []string{"2006-01-02 15:04:05", "2006-01-02 15:04", "2006-01-02"} {
		if t, err := time.ParseInLocation(layout, raw, loc); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("无法识别时间 %q（请使用 YYYY-MM-DD HH:MM 或 YYYY-MM-DD）", raw)
}

func (e *Engine) toolDatetimeCalc(_ context.Context, argsJSON string) (string, error) {
	var args struct {
		Action       string `json:"action"`
		BaseTime     string `json:"base_time"`
		TargetTime   string `json:"target_time"`
		DeltaMinutes int    `json:"delta_minutes"`
		DeltaHours   int    `json:"delta_hours"`
		DeltaDays    int    `json:"delta_days"`
		CronExpr     string `json:"cron_expr"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("解析 datetime_calc 参数失败: %w", err)
	}

	loc := store.ShanghaiLocation()
	base := time.Now().In(loc)
	if raw := strings.TrimSpace(args.BaseTime); raw != "" {
		t, err := parseDatetimeFlex(raw, loc)
		if err != nil {
			return "", err
		}
		base = t
	}

	action := strings.ToLower(strings.TrimSpace(args.Action))
	if action == "" {
		action = "now"
	}

	switch action {
	case "now", "add":
		target := base.AddDate(0, 0, args.DeltaDays).
			Add(time.Duration(args.DeltaHours)*time.Hour + time.Duration(args.DeltaMinutes)*time.Minute)
		out, _ := json.Marshal(map[string]any{
			"base":          base.Format("2006-01-02 15:04:05"),
			"result":        target.Format("2006-01-02 15:04:05"),
			"run_at_format": target.Format("2006-01-02 15:04"),
			"date":          target.Format("2006-01-02"),
			"weekday":       weekdayCN(target.Weekday()),
			"unix":          target.Unix(),
		})
		return string(out), nil

	case "diff", "diff_days":
		rawTarget := strings.TrimSpace(args.TargetTime)
		if rawTarget == "" {
			return "", fmt.Errorf("diff 操作必须传入 target_time")
		}
		target, err := parseDatetimeFlex(rawTarget, loc)
		if err != nil {
			return "", err
		}
		delta := target.Sub(base)
		direction := "same"
		if delta > 0 {
			direction = "future"
		} else if delta < 0 {
			direction = "past"
		}
		absSecs := math.Abs(delta.Seconds())
		totalHours := math.Round((delta.Hours())*100) / 100
		totalDays := math.Round((delta.Hours()/24.0)*100) / 100
		out, _ := json.Marshal(map[string]any{
			"base":           base.Format("2006-01-02 15:04:05"),
			"base_weekday":   weekdayCN(base.Weekday()),
			"target":         target.Format("2006-01-02 15:04:05"),
			"target_weekday": weekdayCN(target.Weekday()),
			"direction":      direction,
			"diff_days":      totalDays,
			"diff_hours":     totalHours,
			"diff_minutes":   int(math.Round(delta.Minutes())),
			"abs_whole_days": int(absSecs / 86400),
		})
		return string(out), nil

	case "cron_next":
		expr := strings.TrimSpace(args.CronExpr)
		if expr == "" {
			return "", fmt.Errorf("cron_next 必须传入 cron_expr")
		}
		sched, err := cron.ParseStandard(expr)
		if err != nil {
			return "", fmt.Errorf("无效 Cron 表达式: %w", err)
		}
		runs := make([]string, 0, 5)
		cur := base
		for i := 0; i < 5; i++ {
			next := sched.Next(cur).In(loc)
			runs = append(runs, fmt.Sprintf("%s (%s)", next.Format("2006-01-02 15:04"), weekdayCN(next.Weekday())))
			cur = next
		}
		out, _ := json.Marshal(map[string]any{
			"cron_expr": expr,
			"next_runs": runs,
		})
		return string(out), nil

	default:
		return "", fmt.Errorf("不支持的 datetime_calc action: %s", action)
	}
}
