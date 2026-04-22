# 🚀 终极 Prompt：Go 语言 cfst 自动化调度与 Web 展示系统

**Role:** 你是一位精通 Go 语言并发编程与系统架构的资深后端工程师。

**Task:** 请仔细阅读 https://github.com/XIU2/CloudflareSpeedTest 的源代码，编写一个高性能、生产级别的 Go 程序，用于定时运行main.go中的main 并通过 Web 界面展示其测速结果。CloudflareSpeedTest已经在本地编译为了cfst。

**Core Requirements:**
1. **任务调度逻辑**：
   - 每隔 2 小时执行一次定时任务。
   - **禁止任务堆叠**：不使用简单的 `time.Ticker`，请使用 `time.AfterFunc` 或在循环体末尾重新计时，确保“任务结束后才开始下一轮计秒”。
   - **单次执行保护**：使用 `sync.RWMutex` 或原子操作维护 `IsRunning` 状态，防止手动触发（如有）与定时触发重叠。

2. **外部程序执行 (Process Security)**：
   - **默认参数**：`-tl 200 -dn 20`。
   - **超时强制终止**：使用 `context.WithTimeout` 设置 30 分钟上限。
   - **日志捕获**：捕获标准输出 (stdout) 和标准错误 (stderr) 到状态记录中。

3. **文件 IO 与防冲突 (Atomic Update)**：
   - **绝对规避读写竞争**：禁止直接修改或截断 `result.csv`。
   - **原子替换策略**：测速结果必须先写入临时文件 `result.tmp.csv`。只有当执行成功且文件写入完成后，再调用 `os.Rename` 将其瞬间换为 `result.csv`。

4. **Web 界面与展示 (UI & Safety)**：
   - 监听 `0.0.0.0:8080` 地址和端口。
   - 响应内容：读取 `result.csv` 并解析为 HTML 表格。
   - **状态反馈**：页面需显著展示：当前状态（测速中/待机）、上次成功运行时间、最近一次错误提示（如有）。
   - **语法严谨性**：**严禁**在 `fmt.Fprintf` 参数中直接嵌套匿名函数逻辑。请在渲染前预处理好所有状态变量（如 `statusText`, `statusClass`）。
   - **UI 风格**：集成简单的内联 CSS，提供现代化的表格样式、字体及卡片式布局。

5. **系统健壮性 (Graceful Shutdown)**：
   - 使用 `os/signal` 监听 `SIGINT` 和 `SIGTERM` 信号。
   - 实现优雅关机：接收到退出信号时，停止调度并确保当前的 HTTP Server 请求处理完毕后再退出。

**Output Context:** 请输出单一的、完整的 `main.go` 文件代码，包含必要的注释。
