package main

import (
	"bytes"
	"fmt"
	"golang.org/x/text/encoding/simplifiedchinese"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"time"
)

// 参考 PowerShell 文档
// $Begin = Get-Date -Date '1/17/2019 08:00:00'
// $End = Get-Date -Date '1/17/2019 17:00:00'
// Get-EventLog -LogName System -EntryType Error -After $Begin -Before $End
// Get-EventLog -LogName Security | Where-Object {$_.EventID -eq 4624} | Select-Object -Property *
// Get-WinEvent @{logname='Security';starttime=[datetime]::today } -MaxEvents 2 | Select-Object -Property *
// Get-WinEvent -FilterHashtable @{LogName='System';StartTime=$StartTime;EndTime=$EndTime}

const (
	// ExitSuccess RC
	ExitSuccess = 0
	// ExitFailure RC
	ExitFailure = 1

	// TimeLayout 时间格式。
	TimeLayout = "01/02/2006 15:04:05"

	// BestVersion 最好支持PowerShell5.1或以上。
	BestVersion = "5.1"

	// CmdVersion PowerShell 版本信息获取命令。
	CmdVersion = "$PSVersionTable.PSVersion"
	// CmdEventLog 获取 Windows 安全相关日志。
	CmdEventLog = "Get-EventLog -LogName Security | Where-Object {$_.TimeGenerated -ge '%s' -and $_.TimeGenerated -lt '%s'} | Select-Object -Property *\n"

	// CmdVars 初始化参数命令。
	// Begin: 11/20/2019 08:59:30; End: 11/20/2019 09:00:00。
	CmdVars = "$Begin = Get-Date -Date '%s'\n$End = Get-Date -Date '%s'\n"
	// CmdWinEvent 获取 Windows 安全相关日志。
	CmdWinEvent = "Get-WinEvent -FilterHashtable @{LogName='Security';StartTime=$Begin;EndTime=$End} | Select-Object -Property *\n"
	// CmdExit 退出命令。
	CmdExit = "exit\n"
)

// StartTime 日志起始时间。
var StartTime string

// EndTime 日志结束时间。
var EndTime string

func init() {
	initConfig()
	initLogger()
}

func main() {
	switch runtime.GOOS {
	case "windows":
		log.Println("Windows System")
	default:
		fmt.Println("Only supports windows operating system!")
		return
	}

	// 系统PowerShell信息收集。
	cmd := exec.Command("powershell", "$PSVersionTable.PSVersion")
	var out bytes.Buffer
	var eout bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &eout
	if err := cmd.Run(); err != nil {
		panic(fmt.Errorf("PowerShell version get: [%w]", err))
	}
	errBytes, err := simplifiedchinese.GBK.NewDecoder().Bytes(eout.Bytes())
	if err != nil {
		panic(fmt.Errorf("Decode error bytes: [%w]", err))
	}
	errStr := string(errBytes)
	if strings.TrimSpace(errStr) != "" {
		fmt.Println("Command line execution error:", errStr)
		return
	}
	outBytes, err := simplifiedchinese.GBK.NewDecoder().Bytes(out.Bytes())
	if err != nil {
		panic(fmt.Errorf("Decode output bytes: [%w]", err))
	}

	ver, err := ParsePSVersion(string(outBytes), "\r\n", "\r\n\r\n\r\n")
	if err != nil {
		panic(fmt.Errorf("Parse version: [%w]", err))
	}
	log.Printf("PowerShell Version %s\n", ver)

	// 日志查询起始时间初始化。
	StartTime = time.Now().Format(TimeLayout)
	ticker := time.NewTicker(time.Second * time.Duration(cfg.Duration))
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, os.Kill)
	go func() {
		<-c
		ticker.Stop()
		os.Exit(ExitSuccess)
	}()
	for _ = range ticker.C {
		// 结束时间为当前时间。
		EndTime = time.Now().Format(TimeLayout)
		fmt.Println()
		fmt.Println(StartTime, " - - - ", EndTime)
		if ver >= BestVersion {
			// Get-WinEvent
			GetWinEvent(StartTime, EndTime)
		} else {
			// Get-EventLog
			GetEventLog(StartTime, EndTime)
		}
		// 下次查询的起始时间为这次的结束时间。
		StartTime = EndTime
	}
}

// ParsePSVersion 解析从命令行输出的PowerShell命令信息。
func ParsePSVersion(data string, bes string, bee string) (ver string, err error) {
	if data != "" {
		bsi := strings.Index(data, bes)
		bei := strings.Index(data, bee)
		if bsi >= 0 && bei >= 0 && bei >= bsi {
			data = data[bsi+len(bes) : bei]
		}
		if data == "" {
			return
		}
		lines := strings.Split(data, "\r\n")
		if len(lines) > 2 {
			vl := lines[2]
			dvs := strings.Split(strings.TrimSpace(vl), " ")
			dl := len(dvs)
			nums := make([]string, 0)
			for i := 0; i < dl; i++ {
				n := dvs[i]
				if strings.TrimSpace(n) != "" {
					nums = append(nums, n)
				}
			}
			if len(nums) > 1 {
				ver = nums[0] + "." + nums[1]
			}
		}
	}
	return
}
