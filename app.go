package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"sort"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

var (
	user32                  = syscall.NewLazyDLL("user32.dll")
	kernel32                = syscall.NewLazyDLL("kernel32.dll")
	procSetWindowsHookEx    = user32.NewProc("SetWindowsHookExW")
	procCallNextHookEx      = user32.NewProc("CallNextHookEx")
	procUnhookWindowsHookEx = user32.NewProc("UnhookWindowsHookEx")
	procGetMessage          = user32.NewProc("GetMessageW")
	procGetCurrentThreadId  = kernel32.NewProc("GetCurrentThreadId")
)

const (
	WH_KEYBOARD_LL = 13
	WM_KEYDOWN     = 0x0100
	WM_SYSKEYDOWN  = 0x0104
)

type POINT struct {
	X, Y int32
}

type MSG struct {
	HWND    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      POINT
}

type KBDLLHOOKSTRUCT struct {
	VkCode      uint32
	ScanCode    uint32
	Flags       uint32
	Time        uint32
	DwExtraInfo uintptr
}

type KeystrokeData struct {
	Date      string `json:"date"`
	Count     int    `json:"count"`
	StartTime int64  `json:"start_time"`
	EndTime   int64  `json:"end_time"`
}

type DailyStats struct {
	Date            string  `json:"date"`
	TotalKeystrokes int     `json:"total_keystrokes"`
	AvgPerMinute    float64 `json:"avg_per_minute"`
	ActiveMinutes   int     `json:"active_minutes"`
}

type KeyTracker struct {
	mu          sync.RWMutex
	dailyData   map[string]*KeystrokeData
	dataFile    string
	lastKeytime time.Time
	hookHandle  uintptr
}

func NewKeyTracker(dataFile string) *KeyTracker {
	kt := &KeyTracker{
		dailyData: make(map[string]*KeystrokeData),
		dataFile:  dataFile,
	}
	kt.loadData()
	return kt
}

func (kt *KeyTracker) loadData() {
	kt.mu.Lock()
	defer kt.mu.Unlock()
	if data, err := os.ReadFile(kt.dataFile); err == nil {
		json.Unmarshal(data, &kt.dailyData)
	}
}

func (kt *KeyTracker) saveData() {
	kt.mu.RLock()
	defer kt.mu.RUnlock()

	if data, err := json.MarshalIndent(kt.dailyData, "", "  "); err == nil {
		os.WriteFile(kt.dataFile, data, 0644)
	} else {
		log.Println("Error marshalling data for saving:", err)
	}
}

func (kt *KeyTracker) recordKeystroke() {
	kt.mu.Lock()
	defer kt.mu.Unlock()

	now := time.Now()
	today := now.Format("2006-01-02")

	if _, exists := kt.dailyData[today]; !exists {
		kt.dailyData[today] = &KeystrokeData{
			Date:      today,
			Count:     0,
			StartTime: now.Unix(),
			EndTime:   now.Unix(),
		}
	}

	kt.dailyData[today].Count++
	kt.dailyData[today].EndTime = now.Unix()
	kt.lastKeytime = now
}

func (kt *KeyTracker) getDailyStats() []DailyStats {
	kt.mu.RLock()
	defer kt.mu.RUnlock()

	var stats []DailyStats
	for _, data := range kt.dailyData {
		durationSeconds := data.EndTime - data.StartTime
		activeMinutes := int(durationSeconds / 60)
		if activeMinutes == 0 {
			activeMinutes = 1
		}
		avgPerMinute := float64(data.Count) / float64(activeMinutes)
		stats = append(stats, DailyStats{
			Date:            data.Date,
			TotalKeystrokes: data.Count,
			AvgPerMinute:    avgPerMinute,
			ActiveMinutes:   activeMinutes,
		})
	}

	sort.Slice(stats, func(i, j int) bool {
		return stats[i].Date < stats[j].Date
	})

	return stats
}

var globalTracker *KeyTracker

func lowLevelKeyboardProc(nCode int, wParam uintptr, lParam uintptr) uintptr {
	if nCode >= 0 && (wParam == WM_KEYDOWN || wParam == WM_SYSKEYDOWN) {
		if globalTracker != nil {
			globalTracker.recordKeystroke()
		}
	}
	ret, _, _ := procCallNextHookEx.Call(0, uintptr(nCode), wParam, lParam)
	return ret
}

func (kt *KeyTracker) startKeyListener() {
	globalTracker = kt
	go func() {
		fmt.Println("Key Hook installed - Monitoring keystrokes.")
		hookHandle, _, err := procSetWindowsHookEx.Call(
			WH_KEYBOARD_LL,
			syscall.NewCallback(lowLevelKeyboardProc),
			0, 0,
		)
		if hookHandle == 0 {
			log.Fatal("Failed to install hook:", err)
		}
		kt.hookHandle = hookHandle
		fmt.Printf("Hook Handle: %x\n", hookHandle)
		var msg MSG
		for {
			ret, _, _ := procGetMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
			if ret == 0 {
				break
			}
		}
		fmt.Println("Key listener message loop ended.")
	}()

	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			kt.saveData()
		}
	}()
}

type PageData struct {
	StatsJSONForInitialRender template.JS
	TotalToday                int
	AvgToday                  float64
	TotalDays                 int
	TotalKeys                 int
	InitialStatsForTable      []DailyStats
}

type APIResponseData struct {
	TotalToday int          `json:"total_today"`
	AvgToday   float64      `json:"avg_today"`
	TotalDays  int          `json:"total_days"`
	TotalKeys  int          `json:"total_keys"`
	Stats      []DailyStats `json:"stats"`
}

const htmlTemplate = `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>ChronoType</title>
    <script src="https://cdn.tailwindcss.com"></script>
    <script src="https://cdnjs.cloudflare.com/ajax/libs/Chart.js/3.9.1/chart.min.js"></script>
    <link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700;900&display=swap" rel="stylesheet">
    <script>
        tailwind.config = {
            darkMode: 'class',
            theme: { extend: { fontFamily: { sans: ['Inter', 'sans-serif'] } } }
        }
        if (localStorage.getItem('theme') === 'dark' || (!('theme' in localStorage) && window.matchMedia('(prefers-color-scheme: dark)').matches)) {
            document.documentElement.classList.add('dark');
        } else {
            document.documentElement.classList.remove('dark');
        }
        function toggleTheme() {
            document.documentElement.classList.toggle('dark');
            localStorage.setItem('theme', document.documentElement.classList.contains('dark') ? 'dark' : 'light');
            renderCharts(); 
        }
    </script>
    <style>
        body { font-family: 'Inter', sans-serif; }
        .chart-container canvas { max-height: 350px; }
        @keyframes pulse-once { 0%, 100% { transform: scale(1); } 50% { transform: scale(1.05); } }
        .animate-pulse-once { animation: pulse-once 0.7s ease-out; }
    </style>
</head>
<body class="bg-white dark:bg-black text-gray-900 dark:text-gray-100 transition-colors duration-300">
    <div class="fixed top-2 right-2 z-50">
        <button onclick="toggleTheme()" class="p-2 rounded-md bg-gray-200 dark:bg-gray-700 hover:bg-gray-300 dark:hover:bg-gray-600 transition-colors">
            <svg id="theme-icon-light" class="w-5 h-5 text-gray-700 dark:hidden" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 3v1m0 16v1m9-9h-1M4 12H3m15.364 6.364l-.707-.707M6.343 6.343l-.707-.707m12.728 0l-.707.707M6.343 17.657l-.707.707M16 12a4 4 0 11-8 0 4 4 0 018 0z"></path></svg>
            <svg id="theme-icon-dark" class="w-5 h-5 text-gray-300 hidden dark:inline" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M20.354 15.354A9 9 0 018.646 3.646 9.003 9.003 0 0012 21a9.003 9.003 0 008.354-5.646z"></path></svg>
        </button>
    </div>
    <div class="fixed top-2 left-2 z-50 px-3 py-1.5 rounded-md text-xs font-semibold bg-green-100 dark:bg-green-800 border border-green-400 dark:border-green-600 text-green-700 dark:text-green-300">
        ACTIVE MONITORING
    </div>

    <div class="container mx-auto max-w-5xl p-4 md:p-6">
        <header class="text-center mb-8 md:mb-10">
            <h1 class="text-3xl sm:text-4xl font-bold text-blue-600 dark:text-blue-400 mb-1 sm:mb-2">ChronoType</h1>
            <p class="text-sm sm:text-base text-gray-600 dark:text-gray-400">Monitoring your daily keyboard usage.</p>
        </header>

        <div class="grid grid-cols-2 sm:grid-cols-2 md:grid-cols-4 gap-4 mb-8 md:mb-10">
            <div class="stat-card bg-gray-50 dark:bg-gray-800 p-4 rounded-lg shadow-md text-center">
                <span id="totalTodayStat" class="stat-number text-2xl sm:text-3xl font-bold text-blue-600 dark:text-blue-400 block">{{.TotalToday}}</span>
                <div class="stat-label text-xs sm:text-sm text-gray-500 dark:text-gray-300 mt-1">Today's Keystrokes</div>
            </div>
            <div class="stat-card bg-gray-50 dark:bg-gray-800 p-4 rounded-lg shadow-md text-center">
                <span id="avgTodayStat" class="stat-number text-2xl sm:text-3xl font-bold text-blue-600 dark:text-blue-400 block">{{printf "%.1f" .AvgToday}}</span>
                <div class="stat-label text-xs sm:text-sm text-gray-500 dark:text-gray-300 mt-1">Avg/Min (Today)</div>
            </div>
            <div class="stat-card bg-gray-50 dark:bg-gray-800 p-4 rounded-lg shadow-md text-center">
                <span id="totalDaysStat" class="stat-number text-2xl sm:text-3xl font-bold text-blue-600 dark:text-blue-400 block">{{.TotalDays}}</span>
                <div class="stat-label text-xs sm:text-sm text-gray-500 dark:text-gray-300 mt-1">Tracked Days</div>
            </div>
            <div class="stat-card bg-gray-50 dark:bg-gray-800 p-4 rounded-lg shadow-md text-center">
                <span id="totalKeysStat" class="stat-number text-2xl sm:text-3xl font-bold text-blue-600 dark:text-blue-400 block">{{.TotalKeys}}</span>
                <div class="stat-label text-xs sm:text-sm text-gray-500 dark:text-gray-300 mt-1">Total Keystrokes</div>
            </div>
        </div>

        <div class="grid md:grid-cols-2 gap-6 mb-6 md:mb-8">
            <div class="chart-container bg-gray-50 dark:bg-gray-800 p-4 sm:p-5 rounded-lg shadow-md">
                <h2 class="text-lg sm:text-xl font-semibold text-center mb-3 text-gray-700 dark:text-gray-200">Daily Keystrokes</h2>
                <canvas id="dailyChart"></canvas>
            </div>
            <div class="chart-container bg-gray-50 dark:bg-gray-800 p-4 sm:p-5 rounded-lg shadow-md">
                <h2 class="text-lg sm:text-xl font-semibold text-center mb-3 text-gray-700 dark:text-gray-200">Average Keystrokes/Minute</h2>
                <canvas id="avgChart"></canvas>
            </div>
        </div>
        
        <div class="data-table-container bg-gray-50 dark:bg-gray-800 p-0 sm:p-2 rounded-lg shadow-md overflow-x-auto">
            <h2 class="text-lg sm:text-xl font-semibold text-center my-3 text-gray-700 dark:text-gray-200">Detailed Daily Log</h2>
            <table class="min-w-full text-sm text-left">
                <thead class="bg-gray-100 dark:bg-gray-700">
                    <tr>
                        <th class="p-3 font-semibold text-gray-600 dark:text-gray-300">Date</th>
                        <th class="p-3 font-semibold text-gray-600 dark:text-gray-300">Total Keystrokes</th>
                        <th class="p-3 font-semibold text-gray-600 dark:text-gray-300">Avg/Min</th>
                        <th class="p-3 font-semibold text-gray-600 dark:text-gray-300">Active Mins</th>
                        <th class="p-3 font-semibold text-gray-600 dark:text-gray-300">Activity Level</th>
                    </tr>
                </thead>
                <tbody id="statsTableBody">
                    {{range .InitialStatsForTable}}
                    <tr class="border-b border-gray-200 dark:border-gray-700 hover:bg-gray-100 dark:hover:bg-gray-700/50 transition-colors">
                        <td class="p-3 whitespace-nowrap">{{.Date}}</td>
                        <td class="p-3 whitespace-nowrap">{{.TotalKeystrokes}}</td>
                        <td class="p-3 whitespace-nowrap">{{printf "%.2f" .AvgPerMinute}}</td>
                        <td class="p-3 whitespace-nowrap">{{.ActiveMinutes}}</td>
                        <td class="p-3 whitespace-nowrap font-medium
                            {{if gt .AvgPerMinute 100.0}}text-red-500 dark:text-red-400{{else if gt .AvgPerMinute 50.0}}text-yellow-500 dark:text-yellow-400{{else if gt .AvgPerMinute 20.0}}text-green-500 dark:text-green-400{{else}}text-blue-500 dark:text-blue-400{{end}}">
                            {{if gt .AvgPerMinute 100.0}}Very High{{else if gt .AvgPerMinute 50.0}}High{{else if gt .AvgPerMinute 20.0}}Moderate{{else}}Low{{end}}
                        </td>
                    </tr>
                    {{end}}
                </tbody>
            </table>
        </div>
    </div>
    
    <script>
        let statsData = {{.StatsJSONForInitialRender}}; 
        let dailyChartInstance, avgChartInstance;

        function getChartColors() {
            const isDarkMode = document.documentElement.classList.contains('dark');
            return {
                textColor: isDarkMode ? '#E5E7EB' : '#374151', gridColor: isDarkMode ? '#4B5563' : '#D1D5DB',
                borderColor: isDarkMode ? '#60A5FA' : '#3B82F6', backgroundColor: isDarkMode ? 'rgba(96, 165, 250, 0.3)' : 'rgba(59, 130, 246, 0.3)',
                pointBackgroundColor: isDarkMode ? '#60A5FA' : '#3B82F6', pointBorderColor: isDarkMode ? '#1F2937' : '#FFFFFF',
                barColors: { 
                    low: isDarkMode ? 'rgba(96, 165, 250, 0.7)' : 'rgba(59, 130, 246, 0.7)',      
                    moderate: isDarkMode ? 'rgba(74, 222, 128, 0.7)' : 'rgba(34, 197, 94, 0.7)',
                    high: isDarkMode ? 'rgba(250, 204, 21, 0.7)' : 'rgba(234, 179, 8, 0.7)',   
                    veryHigh: isDarkMode ? 'rgba(248, 113, 113, 0.7)' : 'rgba(239, 68, 68, 0.7)'
                }
            };
        }

        function renderCharts() {
            const colors = getChartColors();
            Chart.defaults.color = colors.textColor; Chart.defaults.borderColor = colors.gridColor; Chart.defaults.font.family = 'Inter, sans-serif';
            if (dailyChartInstance) dailyChartInstance.destroy(); if (avgChartInstance) avgChartInstance.destroy();
            
            const dailyCtx = document.getElementById('dailyChart').getContext('2d');
            dailyChartInstance = new Chart(dailyCtx, {
                type: 'line', 
                data: { 
                    labels: statsData.map(s => s.date), 
                    datasets: [{ 
                        label: 'Daily Keystrokes', data: statsData.map(s => s.total_keystrokes), 
                        borderColor: colors.borderColor, backgroundColor: colors.backgroundColor, 
                        borderWidth: 2, fill: true, tension: 0.3, pointRadius: 3, pointHoverRadius: 5, 
                        pointBackgroundColor: colors.pointBackgroundColor, pointBorderColor: colors.pointBorderColor, pointBorderWidth: 1 
                    }] 
                },
                options: { 
                    responsive: true, 
                    maintainAspectRatio: true,
                    plugins: { legend: { display: false } }, 
                    scales: { x: { ticks: { font: { size: 10 } } }, y: { ticks: { font: { size: 10 } } } } 
                }
            });
            
            const avgCtx = document.getElementById('avgChart').getContext('2d');
            avgChartInstance = new Chart(avgCtx, {
                type: 'bar', 
                data: { 
                    labels: statsData.map(s => s.date), 
                    datasets: [{ 
                        label: 'Keys Per Minute', data: statsData.map(s => s.avg_per_minute), 
                        backgroundColor: statsData.map(s => { 
                            if (s.avg_per_minute > 100) return colors.barColors.veryHigh; 
                            if (s.avg_per_minute > 50) return colors.barColors.high; 
                            if (s.avg_per_minute > 20) return colors.barColors.moderate; 
                            return colors.barColors.low; 
                        }), 
                        borderRadius: 4, 
                    }] 
                },
                options: { 
                    responsive: true, 
                    maintainAspectRatio: true,
                    plugins: { legend: { display: false } }, 
                    scales: { x: { ticks: { font: { size: 10 } } }, y: { ticks: { font: { size: 10 } }, beginAtZero: true } } 
                }
            });
        }
        
        function updateTable(newStats) {
            const tbody = document.getElementById('statsTableBody');
            tbody.innerHTML = ''; 
            newStats.forEach(stat => {
                const row = tbody.insertRow(); 
                row.className = 'border-b border-gray-200 dark:border-gray-700 hover:bg-gray-100 dark:hover:bg-gray-700/50 transition-colors';
                let cell;
                cell = row.insertCell(); cell.className = 'p-3 whitespace-nowrap'; cell.textContent = stat.date;
                cell = row.insertCell(); cell.className = 'p-3 whitespace-nowrap'; cell.textContent = stat.total_keystrokes;
                cell = row.insertCell(); cell.className = 'p-3 whitespace-nowrap'; cell.textContent = stat.avg_per_minute.toFixed(2);
                cell = row.insertCell(); cell.className = 'p-3 whitespace-nowrap'; cell.textContent = stat.active_minutes;
                let levelText = 'Low'; let levelClass = 'text-blue-500 dark:text-blue-400';
                if (stat.avg_per_minute > 100) { levelText = 'Very High'; levelClass = 'text-red-500 dark:text-red-400'; }
                else if (stat.avg_per_minute > 50) { levelText = 'High'; levelClass = 'text-yellow-500 dark:text-yellow-400'; }
                else if (stat.avg_per_minute > 20) { levelText = 'Moderate'; levelClass = 'text-green-500 dark:text-green-400'; }
                cell = row.insertCell(); cell.className = 'p-3 whitespace-nowrap font-medium ' + levelClass; cell.textContent = levelText;
            });
        }

        async function updateDashboardData() {
            try {
                const response = await fetch('/api/all-stats');
                if (!response.ok) {
                    console.error('Failed to fetch stats:', response.status);
                    return;
                }
                const data = await response.json();

                const totalTodayEl = document.getElementById('totalTodayStat');
                if (totalTodayEl.textContent !== data.total_today.toString()) { 
                    totalTodayEl.textContent = data.total_today;
                    totalTodayEl.classList.add('animate-pulse-once'); 
                    setTimeout(() => totalTodayEl.classList.remove('animate-pulse-once'), 700);
                }
                document.getElementById('avgTodayStat').textContent = data.avg_today.toFixed(1);
                document.getElementById('totalDaysStat').textContent = data.total_days;
                document.getElementById('totalKeysStat').textContent = data.total_keys;

                statsData = data.stats; 
                renderCharts();
                updateTable(data.stats);

            } catch (error) {
                console.error('Error updating dashboard data:', error);
            }
        }
        
        renderCharts(); 
        setInterval(updateDashboardData, 10000);

    </script>
</body>
</html>
`

func main() {
	tracker := NewKeyTracker("keystroke_data.json")
	tracker.startKeyListener()
	time.Sleep(500 * time.Millisecond)

	tmpl, err := template.New("index").Parse(htmlTemplate)
	if err != nil {
		log.Fatal("Failed to parse HTML template:", err)
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		allDailyStats := tracker.getDailyStats()

		var totalToday, totalKeys int
		var avgToday float64
		todayDate := time.Now().Format("2006-01-02")

		for _, stat := range allDailyStats {
			totalKeys += stat.TotalKeystrokes
			if stat.Date == todayDate {
				totalToday = stat.TotalKeystrokes
				avgToday = stat.AvgPerMinute
			}
		}

		statsJSONBytes, _ := json.Marshal(allDailyStats)

		pageRenderData := PageData{
			StatsJSONForInitialRender: template.JS(statsJSONBytes),
			TotalToday:                totalToday,
			AvgToday:                  avgToday,
			TotalDays:                 len(allDailyStats),
			TotalKeys:                 totalKeys,
			InitialStatsForTable:      allDailyStats,
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		err := tmpl.Execute(w, pageRenderData)
		if err != nil {
			log.Println("Error executing template:", err)
		}
	})

	http.HandleFunc("/api/all-stats", func(w http.ResponseWriter, r *http.Request) {
		allDailyStats := tracker.getDailyStats()
		var totalToday, totalKeys int
		var avgToday float64
		todayDate := time.Now().Format("2006-01-02")

		for _, stat := range allDailyStats {
			totalKeys += stat.TotalKeystrokes
			if stat.Date == todayDate {
				totalToday = stat.TotalKeystrokes
				avgToday = stat.AvgPerMinute
			}
		}
		response := APIResponseData{
			TotalToday: totalToday,
			AvgToday:   avgToday,
			TotalDays:  len(allDailyStats),
			TotalKeys:  totalKeys,
			Stats:      allDailyStats,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	})

	fmt.Println("ChronoType server active on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
