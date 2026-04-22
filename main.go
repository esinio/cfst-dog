package main

import (
	"context"
	"encoding/csv"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

// --- 配置参数 ---
const (
	Port           = ":8080"
	TargetBin      = "./cfst" // 已更新为你的本地编译名称
	ResultFile     = "/data/result.csv"
	TempResultFile = "/data/result.tmp.csv"
	TaskInterval   = 2 * time.Hour    // 定时周期
	MaxRunTime     = 30 * time.Minute // 强制超时
)

// --- 全局状态结构体 ---
type AppStatus struct {
	mu          sync.RWMutex
	IsRunning   bool
	LastRunTime time.Time
	LastError   string
}

var status = &AppStatus{}

func main() {
	// 1. 设置信号监听以实现优雅关机
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// 2. 启动异步调度器 (内部包含防止堆叠逻辑)
	go scheduler(ctx)

	// 3. 设置 Web 路由
	mux := http.NewServeMux()
	mux.HandleFunc("/", handleIndex)

	server := &http.Server{
		Addr:    Port,
		Handler: mux,
	}

	// 4. 在协程中启动服务器
	go func() {
		log.Printf("[系统] Web 界面已就绪: http://localhost%s\n", Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[错误] 服务器启动失败: %v", err)
		}
	}()

	// 5. 等待退出信号
	<-ctx.Done()
	log.Println("[系统] 接收到退出信号，正在清理资源...")

	// 优雅关闭：最多等待 5 秒处理残余请求
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("[错误] 强制关闭服务: %v", err)
	}
	log.Println("[系统] 程序已安全退出")
}

// scheduler 确保任务结束后才开始下一轮计时
func scheduler(ctx context.Context) {
	// 程序启动立即执行一次
	runTask()

	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(TaskInterval):
			// 只有当这一行代码被执行到，才说明上一次 runTask 已经返回
			// 并且已经过去了 TaskInterval 的时间
			runTask()
		}
	}
}

// runTask 执行外部 cfst 进程
func runTask() {
	// 状态保护：防止手动触发或逻辑异常导致的重叠
	status.mu.Lock()
	if status.IsRunning {
		status.mu.Unlock()
		return
	}
	status.IsRunning = true
	status.mu.Unlock()

	// 确保退出时重置状态
	defer func() {
		status.mu.Lock()
		status.IsRunning = false
		status.mu.Unlock()
	}()

	log.Println("[任务] 开始新一轮测速...")

	// 设定带超时的 Context
	taskCtx, cancel := context.WithTimeout(context.Background(), MaxRunTime)
	defer cancel()

	// 构造命令：使用临时文件接收结果，避免读写竞争
	// 参数说明：-tl 200 (平均延迟上限), -dn 20 (下载个数), -o (输出文件)
	cmd := exec.CommandContext(taskCtx, TargetBin, "-tl", "200", "-dn", "20", "-o", TempResultFile)

	// 捕获标准输出和错误以便排查
	out, err := cmd.CombinedOutput()

	status.mu.Lock()
	defer status.mu.Unlock()

	if err != nil {
		status.LastError = fmt.Sprintf("执行异常: %v | 详情: %s", err, string(out))
		log.Printf("[任务] 失败: %s", status.LastError)
		return
	}

	// 原子替换：只有成功生成临时文件后，才进行文件更名
	if _, err := os.Stat(TempResultFile); err == nil {
		if err := os.Rename(TempResultFile, ResultFile); err != nil {
			status.LastError = "文件原子替换失败: " + err.Error()
			return
		}
		status.LastRunTime = time.Now()
		status.LastError = "" // 清空历史错误
		log.Println("[任务] 测速成功，数据已更新")
	} else {
		status.LastError = "测速程序未生成预期的 CSV 文件"
	}
}

// handleIndex 渲染 HTML 界面
func handleIndex(w http.ResponseWriter, r *http.Request) {
	// 1. 预处理数据（不在 Fprintf 内部写逻辑）
	status.mu.RLock()
	isRunning := status.IsRunning
	lastTime := "从未运行"
	if !status.LastRunTime.IsZero() {
		lastTime = status.LastRunTime.Format("2006-01-02 15:04:05")
	}
	lastErr := status.LastError

	statusText := "待机中"
	statusClass := "status-idle"
	if isRunning {
		statusText = "测速进行中..."
		statusClass = "status-running"
	}
	status.mu.RUnlock()

	// 2. 写入 HTML 头部与样式
	fmt.Fprint(w, `<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>CFST 自动化监控</title>
    <style>
        body { font-family: -apple-system, system-ui, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; background: #f0f2f5; padding: 20px; color: #1a1a1a; }
        .container { max-width: 1100px; margin: 0 auto; }
        .card { background: white; border-radius: 12px; box-shadow: 0 4px 6px rgba(0,0,0,0.05); padding: 24px; margin-bottom: 20px; }
        .header { display: flex; justify-content: space-between; align-items: center; }
        .status-idle { color: #52c41a; font-weight: bold; }
        .status-running { color: #faad14; font-weight: bold; animation: fade 1.5s infinite; }
        @keyframes fade { 50% { opacity: 0.5; } }
        .error-msg { color: #ff4d4f; background: #fff2f0; padding: 10px; border-radius: 6px; border: 1px solid #ffccc7; margin-top: 15px; font-size: 0.9em; }
        table { width: 100%; border-collapse: collapse; margin-top: 15px; }
        th { background: #fafafa; text-align: left; padding: 12px; border-bottom: 2px solid #f0f0f0; color: #8c8c8c; font-weight: 500; }
        td { padding: 12px; border-bottom: 1px solid #f0f0f0; font-size: 14px; }
        tr:hover { background: #fafafa; }
        .scroll { overflow-x: auto; }
    </style>
</head>
<body>
    <div class="container">
        <div class="card header">
            <div>
                <h2 style="margin:0">Cloudflare Speed Test 响应速度监控</h2>
                <p style="margin:5px 0 0; color: #8c8c8c;">自动每 2 小时运行一次测试</p>
            </div>`)

	// 3. 动态展示状态
	fmt.Fprintf(w, "<div>状态：<span class=\"%s\">%s</span></div>", statusClass, statusText)
	fmt.Fprint(w, "</div>")

	if lastErr != "" {
		fmt.Fprintf(w, "<div class=\"card error-msg\"><strong>最近运行报错：</strong>%s</div>", html.EscapeString(lastErr))
	}

	fmt.Fprintf(w, "<div class=\"card\"><div class=\"header\"><h3>测试结果列表</h3><small>上次更新：%s</small></div><div class=\"scroll\">", lastTime)

	// 4. 解析并显示表格数据
	renderTable(w)

	fmt.Fprint(w, "</div></div></div></body></html>")
}

// renderTable 处理 CSV 解析
func renderTable(w io.Writer) {
	f, err := os.Open(ResultFile)
	if err != nil {
		fmt.Fprint(w, "<p style='color:#8c8c8c'>暂无历史测速结果，请等待任务完成。</p>")
		return
	}
	defer f.Close()

	reader := csv.NewReader(f)
	rows, err := reader.ReadAll()
	if err != nil {
		fmt.Fprintf(w, "<p>数据读取错误: %v</p>", err)
		return
	}

	if len(rows) == 0 {
		fmt.Fprint(w, "<p>CSV 文件中尚无记录。</p>")
		return
	}

	fmt.Fprint(w, "<table>")
	for i, row := range rows {
		if i == 0 {
			fmt.Fprint(w, "<thead><tr>")
			for _, col := range row {
				fmt.Fprintf(w, "<th>%s</th>", html.EscapeString(col))
			}
			fmt.Fprint(w, "</tr></thead><tbody>")
		} else {
			fmt.Fprint(w, "<tr>")
			for _, col := range row {
				fmt.Fprintf(w, "<td>%s</td>", html.EscapeString(col))
			}
			fmt.Fprint(w, "</tr>")
		}
	}
	fmt.Fprint(w, "</tbody></table>")
}
