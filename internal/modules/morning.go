package modules

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/YN1753/vesper/bothub/internal/notifier"
	"github.com/YN1753/vesper/bothub/internal/store"
)

type MorningModule struct {
	store *store.Store
	push  *notifier.Client
	http  *http.Client
}

func NewMorningModule(st *store.Store, push *notifier.Client) *MorningModule {
	return &MorningModule{store: st, push: push, http: NewHTTPClient(15 * time.Second)}
}

func (m *MorningModule) ID() string          { return "morning" }
func (m *MorningModule) Name() string        { return "每日早报小秘书" }
func (m *MorningModule) Description() string { return "汇总目标城市气象建议、每日格言、节点脉搏与临期资产" }

type weatherInfo struct {
	City        string
	Condition   string
	CurrentTemp float64
	MinTemp     float64
	MaxTemp     float64
	PrecipProb  int
	WindSpeed   float64
}

func (m *MorningModule) Run(ctx context.Context) (RunResult, error) {
	return m.RunWithTask(ctx, store.Task{
		ID:   "morning",
		Name: "每日早报小秘书",
		Type: "morning",
	})
}

func (m *MorningModule) RunWithTask(ctx context.Context, task store.Task) (RunResult, error) {
	var params map[string]any
	_ = json.Unmarshal([]byte(task.Params), &params)

	city := ""
	if params != nil {
		if c, ok := params["city"].(string); ok {
			city = strings.TrimSpace(c)
		}
	}
	if city == "" {
		city, _ = m.store.GetConfig(store.KeyMorningCity)
	}
	if strings.TrimSpace(city) == "" {
		city = "Shanghai"
	}

	nezhaURL := ""
	if params != nil {
		if n, ok := params["nezha_url"].(string); ok {
			nezhaURL = strings.TrimSpace(n)
		}
	}
	if nezhaURL == "" {
		nezhaURL, _ = m.store.GetConfig(store.KeyNezhaURL)
	}

	taskName := task.Name
	if taskName == "" {
		taskName = "Vesper 晨间简报"
	}

	now := time.Now().In(ShanghaiLocation())
	dateStr := now.Format("2006-01-02")
	weekday := []string{"周日", "周一", "周二", "周三", "周四", "周五", "周六"}[now.Weekday()]

	w, wErr := m.fetchWeatherWithFallback(ctx, city)
	quote := dailyQuotes[now.YearDay()%len(dailyQuotes)]
	nodePulse := m.fetchNodePulse(ctx, nezhaURL)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("### %s · %s (%s)\n\n", taskName, dateStr, weekday))

	sb.WriteString("#### 1. 气象与出行建议\n")
	if wErr == nil {
		sb.WriteString(fmt.Sprintf("- **城市**：%s（Open-Meteo）\n", w.City))
		sb.WriteString(fmt.Sprintf("- **天气状况**：%s\n", w.Condition))
		sb.WriteString(fmt.Sprintf("- **实时气温**：%.1f°C（今日 %.1f°C ~ %.1f°C）\n", w.CurrentTemp, w.MinTemp, w.MaxTemp))
		sb.WriteString(fmt.Sprintf("- **降水概率**：%d%% · **风速**：%.1f km/h\n", w.PrecipProb, w.WindSpeed))
		sb.WriteString(fmt.Sprintf("- **穿衣建议**：%s\n\n", buildAdvice(w)))
	} else {
		sb.WriteString(fmt.Sprintf("- **城市**：%s（气象接口暂不可用: %v）\n\n", city, wErr))
	}

	sb.WriteString("#### 2. 节点脉搏摘要\n")
	sb.WriteString(nodePulse)
	sb.WriteString("\n\n")

	sb.WriteString("#### 3. 每日格言\n")
	sb.WriteString(fmt.Sprintf("> %s\n\n", quote))

	if openTodos, err := m.store.ListTodos("open", "", 15); err == nil && len(openTodos) > 0 {
		sb.WriteString("#### 4. 今日个人待办提醒\n")
		shown := 0
		for _, td := range openTodos {
			if shown >= 8 {
				sb.WriteString(fmt.Sprintf("- ... 另有 %d 项待办请前往个人行动台查看\n", len(openTodos)-8))
				break
			}
			dueNote := ""
			if strings.TrimSpace(td.DueAt) != "" {
				dueDay := td.DueAt
				if len(dueDay) >= 10 {
					dueDay = dueDay[:10]
				}
				if dueDay < dateStr {
					dueNote = fmt.Sprintf(" [已逾期 %s]", td.DueAt)
				} else if dueDay == dateStr {
					dueNote = fmt.Sprintf(" [今日截止 %s]", td.DueAt)
				} else {
					dueNote = fmt.Sprintf(" [截止 %s]", td.DueAt)
				}
			}
			sb.WriteString(fmt.Sprintf("- **[%s]** %s%s\n", td.Priority, td.Title, dueNote))
			shown++
		}
		sb.WriteString("\n")
	}

	if expiring, _ := m.store.GetExpiringAssets(7); len(expiring) > 0 {
		sb.WriteString("#### 5. 数字资产临期提醒\n")
		for _, a := range expiring {
			sb.WriteString(fmt.Sprintf("- **临期资产**：%s（%s · 剩余 %d 天）\n", a.Name, a.Spec, a.DaysLeft))
		}
		sb.WriteString("\n")
	}

	// Optional AI synthesis when DeepSeek API Key is configured
	sysPrompt := "你是 Vesper BotHub 的晨间参谋。请基于提供的今日气象、哪吒节点脉搏、个人待办清单与临期资产原始数据，用不超过 160 字输出「今日执更要点与行动优先级建议」（分 2-3 条简明要点）。严禁输出任何 Emoji 或特殊装饰符号。"
	if aiInsight, aiErr := SummarizeWithDeepSeek(ctx, m.store, sysPrompt, sb.String()); aiErr == nil && aiInsight != "" {
		sb.WriteString("#### AI 执更参谋锐评\n")
		sb.WriteString(aiInsight)
		sb.WriteString("\n\n")
	}

	sb.WriteString("---\n*来自 Vesper BotHub 自动化汇报*")

	title := fmt.Sprintf("%s · %s %s", taskName, dateStr, city)
	if wErr == nil {
		title = fmt.Sprintf("%s · %s %s %.0f~%.0f°C", taskName, dateStr, w.City, w.MinTemp, w.MaxTemp)
	}

	if err := m.push.SendMarkdown(ctx, task.ID, title, sb.String()); err != nil {
		return RunResult{Summary: "推送失败: " + err.Error()}, err
	}
	return RunResult{Summary: fmt.Sprintf("已推送 %s（城市: %s, 日期: %s）", taskName, city, dateStr), ItemsPushed: 1}, nil
}

func (m *MorningModule) fetchWeather(ctx context.Context, city string) (weatherInfo, error) {
	// 1. Geocoding
	geoURL := fmt.Sprintf("https://geocoding-api.open-meteo.com/v1/search?name=%s&count=1&language=zh&format=json", url.QueryEscape(city))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, geoURL, nil)
	if err != nil {
		return weatherInfo{}, err
	}
	resp, err := m.http.Do(req)
	if err != nil {
		return weatherInfo{}, err
	}
	defer resp.Body.Close()

	var geo struct {
		Results []struct {
			Name      string  `json:"name"`
			Latitude  float64 `json:"latitude"`
			Longitude float64 `json:"longitude"`
		} `json:"results"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&geo)
	if len(geo.Results) == 0 {
		return weatherInfo{}, fmt.Errorf("未找到城市: %s", city)
	}
	loc := geo.Results[0]

	// 2. Forecast
	fURL := fmt.Sprintf(
		"https://api.open-meteo.com/v1/forecast?latitude=%.4f&longitude=%.4f&current=temperature_2m,weather_code,wind_speed_10m&daily=temperature_2m_max,temperature_2m_min,precipitation_probability_max&timezone=Asia%%2FShanghai&forecast_days=1",
		loc.Latitude, loc.Longitude,
	)
	fReq, _ := http.NewRequestWithContext(ctx, http.MethodGet, fURL, nil)
	fResp, err := m.http.Do(fReq)
	if err != nil {
		return weatherInfo{}, err
	}
	defer fResp.Body.Close()

	var fc struct {
		Current struct {
			Temp      float64 `json:"temperature_2m"`
			Code      int     `json:"weather_code"`
			WindSpeed float64 `json:"wind_speed_10m"`
		} `json:"current"`
		Daily struct {
			MaxTemp    []float64 `json:"temperature_2m_max"`
			MinTemp    []float64 `json:"temperature_2m_min"`
			PrecipProb []int     `json:"precipitation_probability_max"`
		} `json:"daily"`
	}
	if err := json.NewDecoder(fResp.Body).Decode(&fc); err != nil || len(fc.Daily.MaxTemp) == 0 {
		return weatherInfo{}, fmt.Errorf("解析气象预报失败")
	}

	precip := 0
	if len(fc.Daily.PrecipProb) > 0 {
		precip = fc.Daily.PrecipProb[0]
	}
	name := loc.Name
	if name == "" {
		name = city
	}

	return weatherInfo{
		City:        name,
		Condition:   wmoCodeCN(fc.Current.Code),
		CurrentTemp: fc.Current.Temp,
		MinTemp:     fc.Daily.MinTemp[0],
		MaxTemp:     fc.Daily.MaxTemp[0],
		PrecipProb:  precip,
		WindSpeed:   fc.Current.WindSpeed,
	}, nil
}

func (m *MorningModule) fetchWeatherWithFallback(ctx context.Context, city string) (weatherInfo, error) {
	// 1. Primary source: Open-Meteo
	w, err := m.fetchWeather(ctx, city)
	if err == nil {
		return w, nil
	}
	// 2. Secondary fallback source: wttr.in (zero key, resilient)
	wFallback, fbErr := m.fetchWeatherWttrIn(ctx, city)
	if fbErr == nil {
		return wFallback, nil
	}
	return weatherInfo{}, fmt.Errorf("主备气象接口均不可用 (open-meteo: %v; wttr.in: %v)", err, fbErr)
}

func (m *MorningModule) fetchWeatherWttrIn(ctx context.Context, city string) (weatherInfo, error) {
	u := fmt.Sprintf("https://wttr.in/%s?format=j1", url.QueryEscape(city))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return weatherInfo{}, err
	}
	req.Header.Set("User-Agent", "curl/7.68.0")
	resp, err := m.http.Do(req)
	if err != nil {
		return weatherInfo{}, err
	}
	defer resp.Body.Close()

	var data struct {
		Current []struct {
			TempC        string `json:"temp_C"`
			Windspeed    string `json:"windspeedKmph"`
			WeatherDesc  []struct{ Value string `json:"value"` } `json:"weatherDesc"`
		} `json:"current_condition"`
		Weather []struct {
			MaxTempC string `json:"maxtempC"`
			MinTempC string `json:"mintempC"`
			Hourly   []struct {
				ChanceOfRain string `json:"chanceofrain"`
			} `json:"hourly"`
		} `json:"weather"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil || len(data.Current) == 0 || len(data.Weather) == 0 {
		return weatherInfo{}, fmt.Errorf("解析 wttr.in 失败")
	}

	curr := data.Current[0]
	today := data.Weather[0]
	cTemp, _ := strconv.ParseFloat(curr.TempC, 64)
	mxTemp, _ := strconv.ParseFloat(today.MaxTempC, 64)
	mnTemp, _ := strconv.ParseFloat(today.MinTempC, 64)
	wSpeed, _ := strconv.ParseFloat(curr.Windspeed, 64)

	precip := 0
	if len(today.Hourly) > 0 {
		precip, _ = strconv.Atoi(today.Hourly[0].ChanceOfRain)
	}
	cond := "晴朗/多云"
	if len(curr.WeatherDesc) > 0 && curr.WeatherDesc[0].Value != "" {
		cond = curr.WeatherDesc[0].Value
	}

	return weatherInfo{
		City:        city,
		Condition:   cond,
		CurrentTemp: cTemp,
		MinTemp:     mnTemp,
		MaxTemp:     mxTemp,
		PrecipProb:  precip,
		WindSpeed:   wSpeed,
	}, nil
}

func (m *MorningModule) fetchNodePulse(ctx context.Context, nezhaURL string) string {
	base := strings.TrimRight(strings.TrimSpace(nezhaURL), "/")
	if base == "" {
		return "- **监控地址**：未配置探针地址"
	}
	start := time.Now()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, base+"/api/v1/server/list", nil)
	req.Header.Set("User-Agent", "Vesper-BotHub/1.0")
	if resp, err := m.http.Do(req); err == nil {
		defer resp.Body.Close()
		var res struct {
			Result []struct {
				LastActive int64 `json:"last_active"`
			} `json:"result"`
		}
		if json.NewDecoder(resp.Body).Decode(&res) == nil && len(res.Result) > 0 {
			online := 0
			nowUnix := time.Now().Unix()
			for _, n := range res.Result {
				if nowUnix-n.LastActive <= 120 {
					online++
				}
			}
			return fmt.Sprintf("- **哪吒探针**：在线 %d / %d 节点（延迟 %d ms）\n- **面板入口**：%s", online, len(res.Result), time.Since(start).Milliseconds(), base)
		}
	}
	return fmt.Sprintf("- **探针入口**：%s（状态检测中）", base)
}

func buildAdvice(w weatherInfo) string {
	var parts []string
	switch {
	case w.MaxTemp < 12:
		parts = append(parts, "气温偏冷，建议厚大衣保暖")
	case w.MaxTemp < 20:
		parts = append(parts, "气温微凉舒适，适合风衣、夹克或针织外搭")
	case w.MaxTemp < 27:
		parts = append(parts, "体感温和，适合轻便长短袖")
	default:
		parts = append(parts, "气温偏高，注意防晒补水")
	}
	if w.WindSpeed >= 30 {
		parts = append(parts, fmt.Sprintf("风力偏大（%.0f km/h），注意防风保暖", w.WindSpeed))
	}
	if w.PrecipProb >= 40 {
		parts = append(parts, fmt.Sprintf("降水概率 %d%%，出门请带伞", w.PrecipProb))
	} else {
		parts = append(parts, "降水概率低，无需带伞")
	}
	return strings.Join(parts, "；") + "。"
}

func wmoCodeCN(code int) string {
	switch {
	case code == 0:
		return "晴朗"
	case code <= 3:
		return "多云/阴天"
	case code <= 48:
		return "有雾"
	case code <= 67:
		return "降雨"
	case code <= 77:
		return "降雪"
	case code <= 82:
		return "阵雨"
	default:
		return "阴"
	}
}

var dailyQuotes = []string{
	"迟暮之光，星穹破晓；在安静的系统里保持秩序与澄明。",
	"Simplicity is prerequisite for reliability. —— Edsger W. Dijkstra",
	"星海横流，岁月成碑；把复杂留给底层，把清爽留给日常。",
	"Make it work, make it right, make it fast. —— Kent Beck",
	"夜色深处自有微光，慢下来反而能看清系统的每一道脉络。",
	"Programs must be written for people to read, and only incidentally for machines to execute.",
	"风起于青萍之末，浪成于微澜之间；今日宜稳步推进。",
	"Premature optimization is the root of all evil. —— Donald Knuth",
	"秩序产生于自律，优雅来自于克制；心有山海，静水深流。",
	"Any fool can write code that a computer can understand. Good programmers write code that humans can understand.",
	"山海辽阔，群星闪烁；每一个稳定的守护都源自细微的打磨。",
	"First, solve the problem. Then, write the code. —— John Johnson",
	"在不确定的世界里，保持确定性的节奏与专注。",
	"The best way to predict the future is to invent it. —— Alan Kay",
	"行至水穷处，坐看云起时；系统与心灵皆需常驻静气。",
	"There are only two hard things in Computer Science: cache invalidation and naming things.",
	"删繁就简三秋树，领异标新二月花；轻量方可恒久。",
	"Good design adds value faster than it adds cost. —— Thomas C. Gale",
	"凡是过往，皆为序章；星轨交替，从容前行。",
	"Talk is cheap. Show me the code. —— Linus Torvalds",
	"万物皆有裂隙，那是光照进来的地方。",
	"Computers are good at following instructions, but not at reading your mind. —— Donald Knuth",
	"暮色深沉处，方见星光之纯粹。",
	"The function of good software is to make the complex appear simple. —— Grady Booch",
	"日日行，不怕千万里；常常做，不怕千万事。",
	"Debugging is twice as hard as writing the code in the first place. —— Brian Kernighan",
	"博观而约取，厚积而薄发；今日亦有新光景。",
	"Truth can only be found in one place: the code. —— Robert C. Martin",
	"海不辞水，故能成其大；山不辞土，故能成其高。",
	"Software is a great combination between artistry and engineering. —— Bill Gates",
	"夜尽天明，星河入怀；保持热爱，奔赴山海。",
}
