package main

import (
	clusterstorage "bilibili-ticket-golang/cluster/storage"
	"bilibili-ticket-golang/cmd/gui/bws_service"
	"bilibili-ticket-golang/cmd/gui/cluster_service"
	"bilibili-ticket-golang/cmd/gui/i18n"
	"bilibili-ticket-golang/cmd/gui/notify_service"
	"bilibili-ticket-golang/cmd/gui/store/configuration"
	"bilibili-ticket-golang/cmd/gui/store/cookiejar"
	"bilibili-ticket-golang/lib/biliutils"
	"bilibili-ticket-golang/lib/global"
	"bilibili-ticket-golang/lib/logfile"
	"bilibili-ticket-golang/lib/notify"
	"bilibili-ticket-golang/lib/reporting"
	"bilibili-ticket-golang/lib/tasklog"
	"bilibili-ticket-golang/lib/terminal"
	"bilibili-ticket-golang/process"
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	gc "bilibili-ticket-golang/captcha-solver"

	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS

func init() {
	application.RegisterEvent[tasklog.LogEntry]("ticket:log")
}

func main() {
	relaunched, terminalErr := terminal.Ensure()
	if terminalErr != nil {
		fmt.Fprintf(os.Stderr, "[terminal] %v\n", terminalErr)
		return
	}
	if relaunched {
		return
	}

	consoleOut := os.Stdout
	consoleErr := os.Stderr
	terminalAttached := terminal.Attached()

	_, err := terminal.ConfirmOnce(terminal.ConfirmationOptions{
		MarkerPath:     "data/.privacy-tos-v1.accepted",
		RequiredText:   "我已阅读并同意",
		Prompt:         privacyTOSPrompt,
		RetryMessage:   "未确认隐私与遥测条款。若不同意，请关闭终端退出；若同意，请输入「我已阅读并同意」。",
		SuccessMessage: "隐私与遥测条款已确认。接下来进入使用规范确认。",
		Output:         consoleOut,
	})
	if err != nil {
		fmt.Fprintf(consoleErr, "[main] privacy ToS confirmation failed: %v\n", err)
		return
	}

	_, err = terminal.ConfirmOnce(terminal.ConfirmationOptions{
		MarkerPath:     "data/.verified",
		RequiredText:   "黄牛死全家",
		Prompt:         "本工具仅供个人学习交流使用，严禁倒卖。\n请输入「黄牛死全家」后按回车继续：",
		RetryMessage:   "输入内容不正确，请重新输入。",
		SuccessMessage: "验证完成，正在启动图形界面……",
		Output:         consoleOut,
	})
	if err != nil {
		fmt.Fprintf(consoleErr, "[main] terminal verification failed: %v\n", err)
		return
	}

	// Start local logging only after both terminal confirmations have completed.
	// This keeps all consent prompts and input outside logs/main.log.
	logFile, archivedLog, logErr := logfile.OpenRotating("logs/main.log")
	if logErr == nil {
		defer logFile.Close()
		tw := &timestampWriter{w: logFile}
		if terminalAttached {
			log.SetOutput(io.MultiWriter(tw, consoleErr))
		} else {
			log.SetOutput(tw)
		}
		log.SetFlags(0) // timestampWriter handles the timestamp prefix
		if archivedLog != "" {
			log.Printf("[main] previous log archived to %s", archivedLog)
		}

		// Redirect os.Stdout / os.Stderr through pipes so that println and
		// third-party libraries writing to stdout/stderr are captured.
		rOut, wOut, pipeOutErr := os.Pipe()
		rErr, wErr, pipeErrErr := os.Pipe()
		if pipeOutErr != nil || pipeErrErr != nil {
			for _, stream := range []*os.File{rOut, wOut, rErr, wErr} {
				if stream != nil {
					_ = stream.Close()
				}
			}
			log.Printf("[main] failed to create stdout/stderr pipes: out=%v err=%v", pipeOutErr, pipeErrErr)
		} else {
			os.Stdout = wOut
			os.Stderr = wErr
			stdoutTarget := io.Writer(tw)
			stderrTarget := io.Writer(tw)
			if terminalAttached {
				stdoutTarget = io.MultiWriter(tw, consoleOut)
				stderrTarget = io.MultiWriter(tw, consoleErr)
			}
			var wg sync.WaitGroup
			wg.Add(2)
			go func() {
				defer wg.Done()
				_, _ = io.Copy(stdoutTarget, rOut)
			}()
			go func() {
				defer wg.Done()
				_, _ = io.Copy(stderrTarget, rErr)
			}()
			defer func() {
				_ = wOut.Close()
				_ = wErr.Close()
				wg.Wait()
				_ = tw.Flush()
			}()
		}
	} else {
		log.SetOutput(consoleErr)
		log.Printf("[main] failed to initialise log rotation: %v", logErr)
	}

	// Remote reporting starts only after both terminal confirmations. Nothing
	// above this point is sent to the reporting server.
	reporting.SetDefault(process.NewConfiguredReportClient(
		global.ReportDSN,
		global.ReportSalt,
		global.ReportTimeout,
		global.ReportSkipSSLCheck,
	))
	reporting.ReportAction(reporting.ActionAppStart)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = reporting.Flush(ctx)
	}()

	// Create an instance of the app structure
	//app := NewApp()
	store := configuration.NewDataStorage()
	err = store.Load()
	if err != nil {
		fault := global.NewFault("加载配置文件 data/store.bin", err, "删除 data/store.bin 以重置配置，或检查文件权限")
		log.Fatalf("[main] %v", fault)
	}
	clusterRepository, err := clusterstorage.Open("data/employer.db")
	if err != nil {
		fault := global.NewFault("打开集群数据库 data/employer.db", err, "检查文件权限；若存在 data/employer.db-wal 残留文件，删除后重试")
		log.Fatalf("[main] %v", fault)
	}
	if err = clusterRepository.MigrateLegacy(context.Background(), store); err != nil {
		fault := global.NewFault("迁移旧版数据到集群数据库", err, "数据库可能已损坏，尝试删除 data/employer.db 后重新配置")
		log.Fatalf("[main] %v", fault)
	}
	clusterSvc := cluster_service.NewClusterService(clusterRepository)

	// Restore saved locale or leave empty for first-startup detection
	if store.Locale != "" {
		i18n.SetLocale(store.Locale)
	}
	jar := cookiejar.New(&cookiejar.Options{
		DefaultCookies: store.Cookies,
	})
	c, err := biliutils.NewBiliClientWithCookiejar(jar)
	if err != nil {
		reporting.ReportErrorOp(reporting.CodeBiliClientInitFailed, "catalog.client.initialize", err)
		flushCtx, cancelFlush := context.WithTimeout(context.Background(), 2*time.Second)
		_ = reporting.Flush(flushCtx)
		cancelFlush()
		fault := global.NewFault("创建 Bilibili 客户端", err, "检查网络连接和 Cookie 有效性")
		log.Fatalf("[main] %v", fault)
	}
	clusterSvc.SetCatalogClient(c)

	// Wire up cookie persistence: called from frontend after login & on exit
	c.SetCookieSaveCallback(func() {
		store.Cookies = jar.AllPersistentEntries()
		store.RefreshToken = c.GetRefreshToken()
		if saveErr := store.Save(); saveErr != nil {
			log.Printf("[main] Failed to persist cookies: %v", saveErr)
		}
		if syncErr := clusterSvc.SyncMainAccount(); syncErr != nil {
			log.Printf("[main] Failed to sync main account into pool: %v", syncErr)
		}
	})

	// Restore refresh token from previous session
	c.SetRefreshToken(store.RefreshToken)
	if syncErr := clusterSvc.SyncMainAccount(); syncErr != nil {
		log.Printf("[main] Main account is not available for pool sync: %v", syncErr)
	}

	// Log broker for real-time task log streaming to the frontend
	logStorage := tasklog.NewLogStorage()
	if err := logStorage.Load(); err != nil {
		log.Printf("[main] Failed to load persisted logs: %v", err)
	}

	logBroker := tasklog.NewLogBroker(logStorage)
	logBroker.AddSink(func(entry tasklog.LogEntry) {
		reporting.ReportTaskLog(entry, 0)
	})
	logService := NewTaskLogService(logBroker)

	// Build MultiNotifier from persisted notification channels
	notifier := notify.NewMultiNotifier()
	for _, ch := range store.NotifyChData.GetAll() {
		n, err := ch.ToNotifier()
		if err == nil {
			notifier.Add(n)
		}
	}
	clusterSvc.SetNotifier(func(message string) { notifier.Notify(message) })
	if err := clusterSvc.Start(context.Background()); err != nil {
		// The Start method already wraps its internal errors with Fault; if
		// the error chain already contains a Fault, it will be rendered with
		// file:line info via the custom MarshalError.
		log.Fatalf("[main] 启动集群服务失败: %v", err)
	}

	bwsSvc := bws_service.New(c, logBroker, store.BWSData, notifier, store)
	notifySvc := notify_service.New(notifier, store.NotifyChData, store)

	// App instance for frontend verification & misc utilities
	app := NewAppWithClientAndStore(c, store)

	defer func() {
		reporting.ReportAction(reporting.ActionAppStop)
		bwsSvc.Close()
		clusterSvc.Close()
		c.PersistCookies()
		logBroker.FlushLogs()
	}()

	// Recover persisted BWS reservations on startup.
	bwsSvc.ReloadBWSTasks()

	// Keep tickets persisted on change
	store.TicketData.SetChangeCallback(func(_ configuration.TicketEntry) {
		if saveErr := store.Save(); saveErr != nil {
			log.Printf("[main] Failed to persist tickets: %v", saveErr)
		}
	})

	var solverFunc = func(gt string, challenge string) (string, error) {
		return gc.Solve(gt, challenge)
	}

	// 初始化 captcha DLL
	os.MkdirAll("./libs", 0755)
	if err := gc.Init("./libs"); err != nil {
		log.Printf("[main] captcha DLL init: %v", err)
		app.setCaptchaStatus(CaptchaStatus{Loaded: false, Error: err.Error()})
	} else {
		status := CaptchaStatus{Loaded: true}
		if v, err := gc.Version(); err != nil {
			status.Error = err.Error()
			log.Printf("[main] captcha DLL loaded but version query failed: %v", err)
		} else if v != nil {
			status.Version = v.Version
			status.GitCommit = v.GitCommit
			log.Printf("[main] captcha DLL loaded (version=%s, commit=%s)", v.Version, v.GitCommit)
		}
		app.setCaptchaStatus(status)

		// 预热验证码模块，加载 ONNX 模型，避免首次请求耗时过长
		if err := gc.Warmup(); err != nil {
			log.Printf("[main] captcha warmup failed (non-fatal): %v", err)
		}

		// Wire the solver into BiliClient so voucher errors are auto-resolved.
		c.SetCaptchaSolver(solverFunc)
		app.SetCaptchaSolver(solverFunc)
		clusterSvc.SetLocalWorkerSolver(solverFunc, makeCaptchaTester(solverFunc))
		log.Printf("[main] captcha solver installed — vouchers will be auto-resolved")
	}

	// Create Wails v3 application.
	wailsApp := application.New(application.Options{
		Name: "bilibili-ticket-golang",
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		// Custom error marshalling: when a bound method returns an error, the
		// Wails CallError.Cause field will contain structured JSON with the
		// source file, line number, operation name, error message and a
		// human-readable hint — instead of the default opaque "0".
		MarshalError: global.MarshalError,
	})
	logBroker.SetEmitter(func(event string, data any) { wailsApp.Event.Emit(event, data) })
	app.SetApp(wailsApp)
	clusterSvc.SetApp(wailsApp)

	// Register all services exposed to the frontend as Wails v3 bindings.
	wailsApp.RegisterService(application.NewService(app))
	wailsApp.RegisterService(application.NewService(clusterSvc))
	wailsApp.RegisterService(application.NewService(c))
	wailsApp.RegisterService(application.NewService(logService))
	wailsApp.RegisterService(application.NewService(bwsSvc))
	wailsApp.RegisterService(application.NewService(notifySvc))

	wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "bilibili-ticket-golang",
		Width:            1024,
		Height:           768,
		BackgroundColour: application.RGBA{Red: 27, Green: 38, Blue: 54, Alpha: 255},
		URL:              "/",
	})

	if err = wailsApp.Run(); err != nil {
		log.Printf("[main] Error: %v", err)
	}
}

const privacyTOSPrompt = `
================ 隐私与遥测说明（ToS v1） ================

本程序启用云控与远程诊断。只有在你确认本条款后，远程上报才会初始化。

【会远程上报的内容】
1. 设备标识：由系统 UUID、主板/BIOS/硬盘等可用硬件标识计算出的稳定机器码。
   服务器收到的是派生后的机器码，不是原始序列号、原始 MAC 地址或完整硬件清单。
2. 程序信息：程序版本、Git Commit、事件时间；上报服务器在网络层也可能看到来源 IP。
3. 操作事件：启动/退出、登录方式、账号/购买人、任务、BWS、Worker、设置和通知等
   固定 ACTION 名称。ACTION 不携带按钮参数、账号名、手机号、项目号或 Worker ID。
4. 登录事件：登录成功后的 Bilibili UID，以及是否为已存在账号的重新登录。
5. 错误诊断：错误码、业务操作、分类、上游状态码、重试属性、指纹，以及经过脱敏和
   长度限制的错误消息/原因链；不上传源码文件路径和行号。
6. 任务日志：任务 ID、日志级别、时间和日志消息。日志消息可能包含项目/活动信息、
   票号、接口状态和上游返回文本，因此任务日志不应被视为匿名数据。

【不会作为遥测字段主动上报的内容】
1. Bilibili 登录密码、短信验证码、图形验证码答案。
2. Cookie、SESSDATA、bili_jct、refresh/access token、OAuth code 等登录凭据。
3. 原始系统 UUID、主板/BIOS/硬盘序列号、原始 MAC 地址和完整显卡信息。
4. 本地配置文件、数据库、Cookie 仓库或日志文件的完整副本。
5. 购买人身份证、手机号和姓名不会作为独立遥测字段上传；但若第三方接口把这些内容
   写入错误文本或任务日志，程序会尽力脱敏，仍不能保证所有非标准文本均可识别。

【本地保存】
配置、账号凭据、数据库和运行日志会保存在本机 data/、logs/ 等目录，用于程序运行。

如果不同意，请直接关闭终端退出。
如已阅读并同意以上内容，请输入「我已阅读并同意」后按回车：`

// makeCaptchaTester wraps the solver into a CaptchaTester that fetches a live
// captcha from Bilibili and tests the solver. Used by the local worker manager.
func makeCaptchaTester(solver func(gt, challenge string) (string, error)) func() (elapsed, validate, captchaType string, err error) {
	return func() (elapsed, validate, captchaType string, err error) {
		req, _ := http.NewRequest("GET",
			"https://passport.bilibili.com/x/passport-login/captcha?source=main_web", nil)
		req.Header.Set("User-Agent",
			"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36")

		resp, httpErr := (&http.Client{Timeout: 15 * time.Second}).Do(req)
		if httpErr != nil {
			err = fmt.Errorf("HTTP: %w", httpErr)
			return
		}
		defer resp.Body.Close()

		body, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			err = fmt.Errorf("read: %w", readErr)
			return
		}

		var r struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
			Data    struct {
				Type    string `json:"type"`
				Geetest struct {
					Gt        string `json:"gt"`
					Challenge string `json:"challenge"`
				} `json:"geetest"`
			} `json:"data"`
		}
		if jsonErr := json.Unmarshal(body, &r); jsonErr != nil {
			err = fmt.Errorf("parse: %w", jsonErr)
			return
		}
		if r.Code != 0 {
			err = fmt.Errorf("API code=%d: %s", r.Code, r.Message)
			return
		}

		captchaType = r.Data.Type
		start := time.Now()
		validate, err = solver(r.Data.Geetest.Gt, r.Data.Geetest.Challenge)
		elapsed = time.Since(start).String()
		return
	}
}

// timestampWriter writes output to an io.Writer, prepending a timestamp
// to each line and syncing (if the underlying writer is an *os.File) after
// every newline so that crash logs are never lost.
type timestampWriter struct {
	w   io.Writer
	mu  sync.Mutex
	buf bytes.Buffer
}

const timeLayout = "2006-01-02 15:04:05 "

func (t *timestampWriter) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	consumed := 0
	for _, b := range p {
		consumed++
		if b == '\n' {
			line := append([]byte(time.Now().Format(timeLayout)), t.buf.Bytes()...)
			line = append(line, '\n')
			if _, err := t.w.Write(line); err != nil {
				return consumed, err
			}
			t.buf.Reset()
			if f, ok := t.w.(*os.File); ok {
				f.Sync()
			}
		} else {
			t.buf.WriteByte(b)
		}
	}
	return len(p), nil
}

// Flush writes any remaining buffered partial line to the underlying writer.
func (t *timestampWriter) Flush() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.buf.Len() == 0 {
		return nil
	}
	line := append([]byte(time.Now().Format(timeLayout)), t.buf.Bytes()...)
	line = append(line, '\n')
	_, err := t.w.Write(line)
	t.buf.Reset()
	return err
}
