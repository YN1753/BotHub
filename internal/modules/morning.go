package modules

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/YN1753/vesper/bothub/internal/pushplus"
	"github.com/YN1753/vesper/bothub/internal/store"
)

type MorningModule struct {
	store *store.Store
	push  *pushplus.Client
	http  *http.Client
}

func NewMorningModule(st *store.Store, push *pushplus.Client) *MorningModule {
	return &MorningModule{
		store: st,
		push:  push,
		http:  NewHTTPClient(15 * time.Second),
	}
}

func (m *MorningModule) ID() string   { return "morning" }
func (m *MorningModule) Name() string { return "每日早报小秘书" }
func (m *MorningModule) Description() string {
	return "定时汇总目标城市天气与穿衣带伞建议、每日格言及 Vesper 节点脉搏"
}

type weatherInfo struct {
	City        string
	Condition   string
	CurrentTemp float64
	MinTemp     float64
	MaxTemp     float64
	PrecipProb  int
	WindSpeed   float64
	Source      string
}

func (m *MorningModule) Run(ctx context.Context) (RunResult, error) {
	city, _ := m.store.GetConfig(store.KeyMorningCity)
	if strings.TrimSpace(city) == "" {
		city = "Shanghai"
	}
	nezhaURL, _ := m.store.GetConfig(store.KeyNezhaURL)

	now := time.Now().In(ShanghaiLocation())
	dateStr := now.Format("2006-01-02")
	weekday := weekdayCN(now.Weekday())

	w, wErr := m.fetchWeather(ctx, city)
	quote := dailyQuote(now)
	nodePulse := m.fetchNodePulse(ctx, nezhaURL)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("### Vesper 晨间简报 · %s (%s)\n\n", dateStr, weekday))

	if wErr == nil {
		sb.WriteString("#### 1. 气象与出行建议\n")
		sb.WriteString(fmt.Sprintf("- **城市**：%s（数据源：%s）\n", w.City, w.Source))
		sb.WriteString(fmt.Sprintf("- **天气状况**：%s\n", w.Condition))
		sb.WriteString(fmt.Sprintf("- **实时气温**：%.1f°C（今日区间 %.1f°C ~ %.1f°C）\n", w.CurrentTemp, w.MinTemp, w.MaxTemp))
		sb.WriteString(fmt.Sprintf("- **降水概率**：%d%% · **风速**：%.1f km/h\n", w.PrecipProb, w.WindSpeed))
		sb.WriteString(fmt.Sprintf("- **穿衣与带伞**：%s\n\n", buildAdvice(w)))
	} else {
		sb.WriteString("#### 1. 气象与出行建议\n")
		sb.WriteString(fmt.Sprintf("- **城市**：%s\n", city))
		sb.WriteString(fmt.Sprintf("- **状态**：气象接口暂不可用（%v）\n\n", wErr))
	}

	sb.WriteString("#### 2. 节点脉搏摘要\n")
	sb.WriteString(nodePulse)
	sb.WriteString("\n\n")

	sb.WriteString("#### 3. 每日短句\n")
	sb.WriteString(fmt.Sprintf("> %s\n\n", quote))
	sb.WriteString("---\n")
	sb.WriteString("*来自 Vesper BotHub · bot.starryheavens.uk*")

	title := fmt.Sprintf("Vesper 早报 · %s %s", dateStr, city)
	if wErr == nil {
		title = fmt.Sprintf("Vesper 早报 · %s %s %.0f~%.0f°C", dateStr, w.City, w.MinTemp, w.MaxTemp)
	}

	if err := m.push.SendMarkdown(ctx, m.ID(), title, sb.String()); err != nil {
		return RunResult{Summary: "推送失败: " + err.Error()}, err
	}
	return RunResult{
		Summary:     fmt.Sprintf("已推送 %s 早报（%s）", city, dateStr),
		ItemsPushed: 1,
	}, nil
}

func (m *MorningModule) fetchWeather(ctx context.Context, city string) (weatherInfo, error) {
	if w, err := m.fetchOpenMeteo(ctx, city); err == nil {
		return w, nil
	}
	return m.fetchWttr(ctx, city)
}

func (m *MorningModule) fetchOpenMeteo(ctx context.Context, city string) (weatherInfo, error) {
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
	if err := json.NewDecoder(resp.Body).Decode(&geo); err != nil {
		return weatherInfo{}, err
	}
	if len(geo.Results) == 0 {
		return weatherInfo{}, fmt.Errorf("city not found: %s", city)
	}
	loc := geo.Results[0]

	forecastURL := fmt.Sprintf(
		"https://api.open-meteo.com/v1/forecast?latitude=%.4f&longitude=%.4f&current=temperature_2m,weather_code,wind_speed_10m&daily=temperature_2m_max,temperature_2m_min,precipitation_probability_max,weather_code&timezone=Asia%%2FShanghai&forecast_days=1",
		loc.Latitude, loc.Longitude,
	)
	fReq, err := http.NewRequestWithContext(ctx, http.MethodGet, forecastURL, nil)
	if err != nil {
		return weatherInfo{}, err
	}
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
	if err := json.NewDecoder(fResp.Body).Decode(&fc); err != nil {
		return weatherInfo{}, err
	}
	if len(fc.Daily.MaxTemp) == 0 || len(fc.Daily.MinTemp) == 0 {
		return weatherInfo{}, fmt.Errorf("empty open-meteo daily data")
	}

	precip := 0
	if len(fc.Daily.PrecipProb) > 0 {
		precip = fc.Daily.PrecipProb[0]
	}
	displayCity := loc.Name
	if displayCity == "" {
		displayCity = city
	}

	return weatherInfo{
		City:        displayCity,
		Condition:   wmoCodeCN(fc.Current.Code),
		CurrentTemp: fc.Current.Temp,
		MinTemp:     fc.Daily.MinTemp[0],
		MaxTemp:     fc.Daily.MaxTemp[0],
		PrecipProb:  precip,
		WindSpeed:   fc.Current.WindSpeed,
		Source:      "Open-Meteo",
	}, nil
}

func (m *MorningModule) fetchWttr(ctx context.Context, city string) (weatherInfo, error) {
	u := fmt.Sprintf("https://wttr.in/%s?format=j1", url.PathEscape(city))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return weatherInfo{}, err
	}
	req.Header.Set("User-Agent", "Vesper-BotHub/1.0")
	resp, err := m.http.Do(req)
	if err != nil {
		return weatherInfo{}, err
	}
	defer resp.Body.Close()

	var data struct {
		CurrentCondition []struct {
			TempC         string `json:"temp_C"`
			WindspeedKmph string `json:"windspeedKmph"`
			WeatherDesc   []struct {
				Value string `json:"value"`
			} `json:"weatherDesc"`
		} `json:"current_condition"`
		Weather []struct {
			MaxtempC string `json:"maxtempC"`
			MintempC string `json:"mintempC"`
			Hourly   []struct {
				ChanceOfRain string `json:"chanceofrain"`
			} `json:"hourly"`
		} `json:"weather"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return weatherInfo{}, err
	}
	if len(data.CurrentCondition) == 0 || len(data.Weather) == 0 {
		return weatherInfo{}, fmt.Errorf("empty wttr.in payload")
	}

	var cur, minT, maxT, wind float64
	fmt.Sscanf(data.CurrentCondition[0].TempC, "%f", &cur)
	fmt.Sscanf(data.CurrentCondition[0].WindspeedKmph, "%f", &wind)
	fmt.Sscanf(data.Weather[0].MintempC, "%f", &minT)
	fmt.Sscanf(data.Weather[0].MaxtempC, "%f", &maxT)

	maxRain := 0
	for _, h := range data.Weather[0].Hourly {
		var p int
		fmt.Sscanf(h.ChanceOfRain, "%d", &p)
		if p > maxRain {
			maxRain = p
		}
	}
	cond := "晴/多云"
	if len(data.CurrentCondition[0].WeatherDesc) > 0 && data.CurrentCondition[0].WeatherDesc[0].Value != "" {
		cond = data.CurrentCondition[0].WeatherDesc[0].Value
	}

	return weatherInfo{
		City:        city,
		Condition:   cond,
		CurrentTemp: cur,
		MinTemp:     minT,
		MaxTemp:     maxT,
		PrecipProb:  maxRain,
		WindSpeed:   wind,
		Source:      "wttr.in",
	}, nil
}

func (m *MorningModule) fetchNodePulse(ctx context.Context, nezhaURL string) string {
	base := strings.TrimRight(strings.TrimSpace(nezhaURL), "/")
	if base == "" {
		return "- **监控地址**：未配置哪吒探针地址"
	}

	// Try Nezha public server list API first, then fall back to HTTP reachability check.
	apiURL := base + "/api/v1/server/list"
	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err == nil {
		req.Header.Set("User-Agent", "Vesper-BotHub/1.0")
		if resp, err := m.http.Do(req); err == nil {
			defer resp.Body.Close()
			latency := time.Since(start).Milliseconds()
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 65536))
			var parsed struct {
				Result []struct {
					Name       string `json:"name"`
					LastActive int64  `json:"last_active"`
				} `json:"result"`
				Data []struct {
					Name       string `json:"name"`
					LastActive int64  `json:"last_active"`
				} `json:"data"`
			}
			if resp.StatusCode == http.StatusOK && json.Unmarshal(body, &parsed) == nil {
				nodes := parsed.Result
				if len(nodes) == 0 {
					nodes = parsed.Data
				}
				if len(nodes) > 0 {
					nowUnix := time.Now().Unix()
					online := 0
					for _, n := range nodes {
						if nowUnix-n.LastActive <= 120 {
							online++
						}
					}
					return fmt.Sprintf("- **哪吒探针**：在线 %d / %d 节点（接口响应 %d ms）\n- **面板入口**：%s", online, len(nodes), latency, base)
				}
			}
		}
	}

	// Fallback: probe base URL reachability.
	probeStart := time.Now()
	pReq, err := http.NewRequestWithContext(ctx, http.MethodGet, base, nil)
	if err != nil {
		return fmt.Sprintf("- **探针状态**：探测构造失败（%v）", err)
	}
	pReq.Header.Set("User-Agent", "Vesper-BotHub/1.0")
	pResp, err := m.http.Do(pReq)
	if err != nil {
		return fmt.Sprintf("- **探针状态**：暂不可达（%s，原因：%v）", base, err)
	}
	defer pResp.Body.Close()
	latency := time.Since(probeStart).Milliseconds()
	return fmt.Sprintf("- **探针入口**：%s 可达（HTTP %d · 延迟 %d ms）", base, pResp.StatusCode, latency)
}

func buildAdvice(w weatherInfo) string {
	var parts []string
	switch {
	case w.MaxTemp < 10:
		parts = append(parts, "气温偏冷，建议穿着羽绒服或厚大衣，注意防风保暖")
	case w.MaxTemp < 18:
		parts = append(parts, "微凉舒适，建议长袖打底配风衣、夹克或薄针织")
	case w.MaxTemp < 26:
		parts = append(parts, "体感温和，适合衬衫、卫衣或轻薄外套")
	default:
		parts = append(parts, "气温较高，建议短袖透气衣物并注意防晒补水")
	}

	if w.MaxTemp-w.MinTemp >= 8 {
		parts = append(parts, "昼夜温差较大，早晚注意添衣")
	}
	if w.PrecipProb >= 40 || strings.Contains(w.Condition, "雨") {
		parts = append(parts, fmt.Sprintf("降水概率 %d%%，出门请随身携带雨具", w.PrecipProb))
	} else {
		parts = append(parts, "今日降水概率较低，无需刻意带伞")
	}
	return strings.Join(parts, "；") + "。"
}

func wmoCodeCN(code int) string {
	switch code {
	case 0:
		return "晴朗"
	case 1, 2:
		return "少云"
	case 3:
		return "阴天"
	case 45, 48:
		return "有雾"
	case 51, 53, 55, 56, 57:
		return "毛毛雨"
	case 61, 63, 65, 66, 67:
		return "降雨"
	case 71, 73, 75, 77:
		return "降雪"
	case 80, 81, 82:
		return "阵雨"
	case 85, 86:
		return "阵雪"
	case 95, 96, 99:
		return "雷暴"
	default:
		return "多云"
	}
}

func weekdayCN(w time.Weekday) string {
	switch w {
	case time.Sunday:
		return "周日"
	case time.Monday:
		return "周一"
	case time.Tuesday:
		return "周二"
	case time.Wednesday:
		return "周三"
	case time.Thursday:
		return "周四"
	case time.Friday:
		return "周五"
	case time.Saturday:
		return "周六"
	default:
		return ""
	}
}

func dailyQuote(t time.Time) string {
	quotes := []string{
		"迟暮之光，星穹破晓；在安静的系统里保持秩序与好奇。",
		"Simplicity is prerequisite for reliability. —— Edsger W. Dijkstra",
		"星海横流，岁月成碑；把复杂留给底层，把澄明留给日常。",
		"Make it work, make it right, make it fast. —— Kent Beck",
		"夜色深处自有微光，慢下来反而能看清系统的每一道脉络。",
		"Programs must be written for people to read, and only incidentally for machines to execute.",
		"风起于青萍之末，浪成于微澜之间；今日宜稳步推进。",
		"Premature optimization is the root of all evil. —— Donald Knuth",
		"山川异域，风月同天；愿代码无 bug，服务器长稳。",
		"Perfection is achieved not when there is nothing more to add, but when there is nothing left to take away.",
	}
	idx := t.YearDay() % len(quotes)
	return quotes[idx]
}
