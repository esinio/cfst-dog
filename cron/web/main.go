package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
)

// --- 精致设计的单文件 HTML ---
const htmlTemplate = `
<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Cloudflare 优选监控</title>
    <link href="https://cdn.jsdelivr.net/npm/bootstrap@5.3.0/dist/css/bootstrap.min.css" rel="stylesheet">
    <style>
        :root {
            --bg-color: #f0f2f5;
            --card-bg: #ffffff;
            --primary-blue: #007aff;
            --success-green: #34c759;
            --danger-red: #ff3b30;
        }
        body { background-color: var(--bg-color); color: #1d1d1f; font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; padding: 20px; }
        .container { max-width: 1100px; margin: 0 auto; }

        /* 顶部卡片 */
        .header-card {
            background: linear-gradient(135deg, #1e293b 0%, #334155 100%);
            color: white; padding: 25px; border-radius: 16px; margin-bottom: 25px;
            box-shadow: 0 10px 20px rgba(0,0,0,0.1);
        }
        .update-tag { background: rgba(255,255,255,0.1); padding: 5px 12px; border-radius: 20px; font-size: 0.85rem; border: 1px solid rgba(255,255,255,0.2); }

        /* 表格样式 */
        .table-container { background: var(--card-bg); border-radius: 16px; overflow: hidden; box-shadow: 0 4px 6px rgba(0,0,0,0.05); }
        .table { margin-bottom: 0; }
        .table thead { background-color: #f8f9fa; }
        .table th { border-bottom: 1px solid #eee; padding: 16px; font-weight: 600; text-transform: uppercase; font-size: 0.75rem; color: #64748b; }
        .table td { padding: 16px; vertical-align: middle; border-bottom: 1px solid #f1f5f9; font-size: 0.95rem; }

        /* 数据高亮 */
        .ip-code { font-family: "SFMono-Regular", Consolas, monospace; background: #f1f5f9; padding: 4px 8px; border-radius: 6px; color: #475569; font-weight: 500; }
        .location-badge { background: #e0f2fe; color: #0369a1; padding: 4px 10px; border-radius: 6px; font-size: 0.8rem; font-weight: bold; }
        .speed-high { color: var(--success-green); font-weight: 800; font-size: 1.1rem; }
        .speed-low { color: #64748b; }
        .latency-low { color: var(--primary-blue); font-weight: 600; }
        .loss-warn { color: var(--danger-red); font-weight: bold; background: #fee2e2; padding: 2px 6px; border-radius: 4px; }

        tr:last-child td { border-bottom: none; }
        tr:hover { background-color: #f8fafc; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header-card d-flex justify-content-between align-items-center">
            <div>
                <h3 class="mb-1">Cloudflare 优选节点</h3>
                <p class="mb-0 opacity-75 small">数据路径: {{.FilePath}}</p>
            </div>
            <div class="text-end">
                <span class="update-tag">最后同步: {{.LastUpdate}}</span>
            </div>
        </div>

        <div class="table-container">
            <div class="table-responsive">
                <table class="table">
                    <thead>
                        <tr>
                            <th>IP 地址</th>
                            <th>地区</th>
                            <th>丢包</th>
                            <th>平均延迟</th>
                            <th>下载速度</th>
                        </tr>
                    </thead>
                    <tbody>
                        {{range .Results}}
                        <tr>
                            <td><span class="ip-code">{{.IP}}</span></td>
                            <td><span class="location-badge">{{.Location}}</span></td>
                            <td><span class="{{if ne .LossRate "0.00"}}loss-warn{{end}}">{{.LossRate}}%</span></td>
                            <td><span class="latency-low">{{.AvgLatency}} ms</span></td>
                            <td>
                                <span class="{{if IsFast .DownloadSpeed}}speed-high{{else}}speed-low{{end}}">
                                    {{.DownloadSpeed}} <small>MB/s</small>
                                </span>
                            </td>
                        </tr>
                        {{end}}
                    </tbody>
                </table>
            </div>
        </div>
    </div>
</body>
</html>
`

type Result struct {
	IP            string
	LossRate      string
	AvgLatency    string
	DownloadSpeed string
	Location      string
}

type PageData struct {
	LastUpdate string
	FilePath   string
	Results    []Result
}

func main() {
	csvPath := flag.String("file", "result.csv", "CSV文件路径")
	port := flag.Int("port", 8080, "运行端口")
	flag.Parse()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		info, err := os.Stat(*csvPath)
		if err != nil {
			http.Error(w, "找不到 CSV 文件", 404)
			return
		}

		file, err := os.Open(*csvPath)
		if err != nil {
			http.Error(w, "无法读取文件", 500)
			return
		}
		defer file.Close()

		records, _ := csv.NewReader(file).ReadAll()
		var results []Result
		for i, line := range records {
			if i == 0 || len(line) < 7 {
				continue
			}
			results = append(results, Result{
				IP:            line[0],
				LossRate:      line[3],
				AvgLatency:    line[4],
				DownloadSpeed: line[5],
				Location:      line[6],
			})
		}

		// 注册自定义模板函数用于判断速度
		funcMap := template.FuncMap{
			"IsFast": func(speedStr string) bool {
				s, _ := strconv.ParseFloat(strings.TrimSpace(speedStr), 64)
				return s >= 50.0 // 速度超过 50MB/s 定义为快
			},
		}

		tmpl, _ := template.New("index").Funcs(funcMap).Parse(htmlTemplate)
		tmpl.Execute(w, PageData{
			LastUpdate: info.ModTime().Format("15:04:05 (2006-01-02)"),
			FilePath:   *csvPath,
			Results:    results,
		})
	})

	fmt.Printf("服务运行在: http://localhost:%d\n", *port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", *port), nil))
}
